package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type deviceAuthResponse struct {
	Device *oauth2.DeviceAuthResponse
	config *oauth2.Config
	ctx    context.Context
}

type tokenResponse struct {
	Token oauth2.Token
}

type connectModal struct {
	providers        []string
	selectedProvider string
	providerCursor   int
}

type connectGithubModal struct {
	deviceAuthResponse deviceAuthResponse
	isPolling          bool
	hasToken           bool
}

func (m connectGithubModal) Init() tea.Cmd {
	return nil
}

func (m connectGithubModal) startOauthFlow() tea.Msg {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		panic("GITHUB_CLIENT_ID is not set")
	}

	config := &oauth2.Config{
		ClientID: clientID,
		Endpoint: oauth2.Endpoint{
			AuthURL: github.Endpoint.AuthURL,
			TokenURL: github.Endpoint.
				TokenURL, DeviceAuthURL: github.Endpoint.DeviceAuthURL,
		},
	}
	ctx := context.Background()

	device, err := config.DeviceAuth(ctx)
	if err != nil {
		fmt.Printf("error getting device code: %v\n", err)
		panic(err)
	}

	deviceAuthResponse := deviceAuthResponse{
		Device: device,
		config: config,
		ctx:    ctx,
	}

	return deviceAuthResponse
}

func (m connectGithubModal) pollForToken() tea.Msg {
	token, err := m.deviceAuthResponse.config.DeviceAccessToken(m.deviceAuthResponse.ctx, m.deviceAuthResponse.Device)
	if err != nil {
		fmt.Printf("error exchanging device code: %v\n", err)
		panic(err)
	}

	return tokenResponse{Token: *token}
}

func (m connectGithubModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyDown:
			return m, nil
		case tea.KeyUp:
			return m, nil
		case tea.KeyEnter:
			return m, tea.Quit
		}
	case deviceAuthResponse:
		m.deviceAuthResponse = msg
		m.isPolling = true
		return m, m.pollForToken
	case tokenResponse:
		m.isPolling = false
		m.hasToken = true
		return m, nil
	}

	return m, nil
}

func (m connectGithubModal) View() string {
	s := strings.Builder{}

	if m.isPolling {
		s.WriteString(fmt.Sprintf("Go to %s and enter code %s", m.deviceAuthResponse.Device.VerificationURI, m.deviceAuthResponse.Device.UserCode))
	}

	if m.hasToken {
		s.WriteString("Token received, you can close this window and continue with the setup")
	}

	return s.String()
}

func (m connectModal) Init() tea.Cmd {
	return nil
}

func (m connectModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyDown:
			m.providerCursor++
			if m.providerCursor >= len(m.providers) {
				m.providerCursor = 0
			}
		case tea.KeyUp:
			m.providerCursor--
			if m.providerCursor < 0 {
				m.providerCursor = len(m.providers) - 1
			}
		case tea.KeyEnter:
			m.selectedProvider = m.providers[m.providerCursor]
			if m.selectedProvider == "GitHub" {
				modal := connectGithubModal{}
				return modal, modal.startOauthFlow
			}
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m connectModal) View() string {
	s := strings.Builder{}
	s.WriteString("Select a provider:\n")
	for i, provider := range m.providers {
		if i == m.providerCursor {
			s.WriteString(fmt.Sprintf("> %s\n", provider))
		} else {
			s.WriteString(fmt.Sprintf("  %s\n", provider))
		}
	}
	return s.String()
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect your repository to Nimbul",
	RunE:  connectExec,
}

func init() {
	rootCmd.AddCommand(connectCmd)
}

func connectExec(cmd *cobra.Command, args []string) error {
	p := tea.NewProgram(connectModal{
		providers:        []string{"GitHub"},
		selectedProvider: "",
		providerCursor:   0,
	})
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
