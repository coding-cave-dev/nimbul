package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	orangeColor = lipgloss.Color("#FF6B35")
	grayColor   = lipgloss.Color("#808080")
	lightGray   = lipgloss.Color("#A0A0A0")
	darkGray    = lipgloss.Color("#505050")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(orangeColor).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			MarginBottom(1)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(darkGray).
			Width(40)

	inputFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(orangeColor).
				Width(40)

	errorStyle = lipgloss.NewStyle().
			Foreground(orangeColor).
			Italic(true).
			MarginTop(1)

	successStyle = lipgloss.NewStyle().
			Foreground(orangeColor).
			Bold(true).
			MarginTop(1)

	loadingStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			MarginTop(1)
)

type registerModel struct {
	email        string
	password     string
	focusedField int // 0 = email, 1 = password
	err          string
	success      bool
	loading      bool
	quitting     bool
}

type registerSuccessMsg struct {
	email string
}

type registerErrorMsg struct {
	err string
}

func (m registerModel) Init() tea.Cmd {
	return nil
}

func (m registerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		// Ignore mouse events (including scrolling)
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyTab:
			if m.focusedField == 0 {
				// Move to password field
				m.focusedField = 1
				return m, nil
			} else {
				m.focusedField = 0
				return m, nil
			}

		case tea.KeyEnter:
			// Submit form
			if m.email == "" || m.password == "" {
				m.err = "Please fill in all fields"
				return m, nil
			}
			m.loading = true
			m.err = ""
			return m, registerUser(m.email, m.password)

		case tea.KeyBackspace:
			if m.focusedField == 0 {
				if len(m.email) > 0 {
					m.email = m.email[:len(m.email)-1]
				}
			} else {
				if len(m.password) > 0 {
					m.password = m.password[:len(m.password)-1]
				}
			}

		case tea.KeyRunes:
			// Only process printable characters
			if m.focusedField == 0 {
				m.email += string(msg.Runes)
			} else {
				m.password += string(msg.Runes)
			}
		}

	case registerSuccessMsg:
		m.loading = false
		m.success = true
		m.quitting = true
		return m, tea.Quit

	case registerErrorMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

func (m registerModel) View() string {
	if m.quitting && m.success {
		return successStyle.Render(fmt.Sprintf("✓ Successfully registered! Email: %s", m.email)) + "\n"
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Register"))
	b.WriteString("\n\n")

	// Email field
	emailLabel := labelStyle.Render("Email:")
	b.WriteString(emailLabel)
	b.WriteString("\n")

	if m.focusedField == 0 {
		b.WriteString(inputFocusedStyle.Render(m.email + "█"))
	} else {
		b.WriteString(inputStyle.Render(m.email))
	}
	b.WriteString("\n\n")

	// Password field
	passwordLabel := labelStyle.Render("Password:")
	b.WriteString(passwordLabel)
	b.WriteString("\n")

	maskedPassword := strings.Repeat("*", len(m.password))
	if m.focusedField == 1 {
		b.WriteString(inputFocusedStyle.Render(maskedPassword + "█"))
	} else {
		b.WriteString(inputStyle.Render(maskedPassword))
	}
	b.WriteString("\n")

	// Loading indicator
	if m.loading {
		b.WriteString(loadingStyle.Render("Registering..."))
		b.WriteString("\n")
	}

	// Error message
	if m.err != "" {
		b.WriteString(errorStyle.Render("✗ " + m.err))
		b.WriteString("\n")
	}

	// Help text
	helpText := lipgloss.NewStyle().
		Foreground(lightGray).
		MarginTop(1).
		Render("Press Tab to switch fields, Enter to submit, Ctrl+C to quit")
	b.WriteString(helpText)

	return b.String()
}

func registerUser(email, password string) tea.Cmd {
	return func() tea.Msg {
		resp, err := makeAuthRequest("/register", email, password)
		if err != nil {
			return registerErrorMsg{err: err.Error()}
		}
		return registerSuccessMsg{email: resp.User.Email}
	}
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new user account",
	Long:  `Register a new user account with email and password.`,
	RunE:  registerExec,
}

func init() {
	rootCmd.AddCommand(registerCmd)
}

func registerExec(cmd *cobra.Command, args []string) error {
	model := registerModel{
		focusedField: 0,
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
