package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type loginModel struct {
	email        string
	password     string
	focusedField int // 0 = email, 1 = password
	err          string
	success      bool
	loading      bool
	quitting     bool
}

type loginSuccessMsg struct {
	email string
	token string
}

type loginErrorMsg struct {
	err string
}

func (m loginModel) Init() tea.Cmd {
	return nil
}

func (m loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		case tea.KeyTab, tea.KeyEnter:
			if m.focusedField == 0 {
				// Move to password field
				m.focusedField = 1
				return m, nil
			} else {
				// Submit form
				if m.email == "" || m.password == "" {
					m.err = "Please fill in all fields"
					return m, nil
				}
				m.loading = true
				m.err = ""
				return m, loginUser(m.email, m.password)
			}

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

	case loginSuccessMsg:
		m.loading = false
		m.success = true
		m.quitting = true
		// Save token
		if err := saveToken(msg.token); err != nil {
			return m, func() tea.Msg {
				return loginErrorMsg{err: fmt.Sprintf("Failed to save token: %v", err)}
			}
		}
		return m, tea.Quit

	case loginErrorMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

func (m loginModel) View() string {
	if m.quitting && m.success {
		return successStyle.Render(fmt.Sprintf("✓ Successfully logged in! Email: %s\nToken saved.", m.email)) + "\n"
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Login"))
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
		b.WriteString(loadingStyle.Render("Logging in..."))
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

func loginUser(email, password string) tea.Cmd {
	return func() tea.Msg {
		resp, err := makeAuthRequest("/login", email, password)
		if err != nil {
			return loginErrorMsg{err: err.Error()}
		}
		return loginSuccessMsg{
			email: resp.User.Email,
			token: resp.Token,
		}
	}
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to your account",
	Long:  `Login to your account with email and password. Your session token will be saved.`,
	RunE:  loginExec,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func loginExec(cmd *cobra.Command, args []string) error {
	model := loginModel{
		focusedField: 0,
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
