package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coding-cave-dev/nimbul/internal/sdk"
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

// saveTokensToAPI saves both access and refresh tokens to the credentials endpoint
// The userID is extracted server-side from the authToken
func saveTokensToAPI(client *sdk.ClientWithResponses, authToken, provider string, oauthToken oauth2.Token) error {
	if client == nil {
		return fmt.Errorf("SDK client is not available")
	}

	ctx := context.Background()
	authHeader := fmt.Sprintf("Bearer %s", authToken)

	// Calculate expiry times
	accessExpiry := time.Now().Add(8 * time.Hour)            // 8 hours
	refreshExpiry := time.Now().Add(6 * 30 * 24 * time.Hour) // 6 months (180 days)

	params := &sdk.PostCredentialsParams{
		Authorization: &authHeader,
	}

	// Save access token
	accessTokenReq := sdk.StoreCredentialRequestBody{
		Provider:  provider,
		TokenType: "oauth_access",
		Token:     oauthToken.AccessToken,
		ExpiresAt: accessExpiry,
	}

	accessResp, err := client.PostCredentialsWithResponse(ctx, params, accessTokenReq)
	if err != nil {
		return fmt.Errorf("failed to save access token: %w", err)
	}

	if accessResp.StatusCode() != 200 {
		var errMsg string
		if accessResp.ApplicationproblemJSONDefault != nil {
			if accessResp.ApplicationproblemJSONDefault.Detail != nil {
				errMsg = *accessResp.ApplicationproblemJSONDefault.Detail
			} else if accessResp.ApplicationproblemJSONDefault.Title != nil {
				errMsg = *accessResp.ApplicationproblemJSONDefault.Title
			}
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("request failed with status %d", accessResp.StatusCode())
		}
		return fmt.Errorf("failed to save access token: %s", errMsg)
	}

	// Save refresh token (only if it exists)
	if oauthToken.RefreshToken != "" {
		refreshTokenReq := sdk.StoreCredentialRequestBody{
			Provider:  provider,
			TokenType: "oauth_refresh",
			Token:     oauthToken.RefreshToken,
			ExpiresAt: refreshExpiry,
		}

		refreshResp, err := client.PostCredentialsWithResponse(ctx, params, refreshTokenReq)
		if err != nil {
			return fmt.Errorf("failed to save refresh token: %w", err)
		}

		if refreshResp.StatusCode() != 200 {
			var errMsg string
			if refreshResp.ApplicationproblemJSONDefault != nil {
				if refreshResp.ApplicationproblemJSONDefault.Detail != nil {
					errMsg = *refreshResp.ApplicationproblemJSONDefault.Detail
				} else if refreshResp.ApplicationproblemJSONDefault.Title != nil {
					errMsg = *refreshResp.ApplicationproblemJSONDefault.Title
				}
			}
			if errMsg == "" {
				errMsg = fmt.Sprintf("request failed with status %d", refreshResp.StatusCode())
			}
			return fmt.Errorf("failed to save refresh token: %s", errMsg)
		}
	}

	return nil
}

type connectModal struct {
	email            string
	userID           string
	authToken        string
	providers        []string
	selectedProvider string
	providerCursor   int
}

type connectGithubModal struct {
	deviceAuthResponse deviceAuthResponse
	isPolling          bool
	hasToken           bool
	token              oauth2.Token
	authToken          string
	userID             string
	client             *sdk.ClientWithResponses
	tokensSaved        bool
	saveError          error
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
		if m.hasToken {
			return m, tea.Quit
		}
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
		m.token = msg.Token

		// Save tokens to API if not already saved
		if !m.tokensSaved {
			err := saveTokensToAPI(m.client, m.authToken, "github", msg.Token)
			if err != nil {
				m.saveError = err
				// Log error but don't fail the flow
				fmt.Printf("Warning: Failed to save tokens to API: %v\n", err)
			} else {
				m.tokensSaved = true
			}
		}

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
		s.WriteString("\n")
		if m.tokensSaved {
			s.WriteString("✓ Tokens saved successfully\n")
		} else if m.saveError != nil {
			s.WriteString(fmt.Sprintf("⚠ Warning: Failed to save tokens: %v\n", m.saveError))
		}
		s.WriteString(fmt.Sprintf("Token: %s", m.token.AccessToken))
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("Refresh Token: %s", m.token.RefreshToken))
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("Expiry: %s", m.token.Expiry))
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("Token Type: %s", m.token.TokenType))
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
				// Get SDK client for API calls
				client, err := getSDKClient()
				if err != nil {
					// If we can't get client, still proceed with OAuth flow
					// but token saving will fail later
					client = nil
				}
				modal := connectGithubModal{
					authToken: m.authToken,
					userID:    m.userID,
					client:    client,
				}
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
	Short: "Connect your GitHub account to Nimbul",
	RunE:  connectExec,
}

func init() {
	rootCmd.AddCommand(connectCmd)
}

func connectExec(cmd *cobra.Command, args []string) error {
	token, err := loadToken()
	if err != nil {
		return fmt.Errorf("failed to load token: %w", err)
	}

	if token == "" {
		return fmt.Errorf("not logged in. Please run 'nimbul login' first")
	}

	// Get SDK client
	client, err := getSDKClient()
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	ctx := context.Background()
	authHeader := fmt.Sprintf("Bearer %s", token)
	params := &sdk.GetMeParams{
		Authorization: &authHeader,
	}

	resp, err := client.GetMeWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode() != 200 {
		if resp.ApplicationproblemJSONDefault != nil {
			detail := ""
			if resp.ApplicationproblemJSONDefault.Detail != nil {
				detail = *resp.ApplicationproblemJSONDefault.Detail
			} else if resp.ApplicationproblemJSONDefault.Title != nil {
				detail = *resp.ApplicationproblemJSONDefault.Title
			}
			if detail != "" {
				return fmt.Errorf("%s", detail)
			}
		}
		return fmt.Errorf("request failed with status %d", resp.StatusCode())
	}

	if resp.JSON200 == nil {
		return fmt.Errorf("empty response body")
	}

	p := tea.NewProgram(connectModal{
		providers:        []string{"GitHub"},
		selectedProvider: "",
		providerCursor:   0,
		userID:           resp.JSON200.Id,
		authToken:        token,
		email:            resp.JSON200.Email,
	})
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
