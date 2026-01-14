package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/coding-cave-dev/nimbul/internal/github"
	"github.com/coding-cave-dev/nimbul/internal/sdk"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Nimbul for your repository",
	Long:  `Initialize Nimbul to watch your repository and build Docker images on commits`,
	RunE:  initExec,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

type initState struct {
	authToken           string
	userID              string
	providers           []string
	currentRepo         *gitRepo
	availableRepos      []githubRepo
	selectedRepo        *githubRepo
	repoSelectionCursor int
	confirmRepoCursor   int // 0 = Yes, 1 = No
	dockerfileInput     string
	dockerfileFocused   bool
	dockerfilePath      string
	webhookSecret       string
	configID            string
	step                string
	err                 error
}

type gitRepo struct {
	owner string
	name  string
	url   string
}

type githubRepo = github.Repository

type providersLoadedMsg struct {
	providers []string
	err       error
}

type gitRepoDetectedMsg struct {
	repo *gitRepo
	err  error
}

type githubReposLoadedMsg struct {
	repos []githubRepo
	err   error
}

type repoSelectedMsg struct {
	repo *githubRepo
}

type confirmRepoMsg struct {
	useCurrent bool
}

type dockerfileSubmittedMsg struct {
	path string
}

type dockerfileValidatedMsg struct {
	path   string
	exists bool
	err    error
}

func initExec(cmd *cobra.Command, args []string) error {
	// Check login
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
		return fmt.Errorf("authentication failed. Please run 'nimbul login' again")
	}

	if resp.JSON200 == nil {
		return fmt.Errorf("empty response body")
	}

	state := &initState{
		authToken: token,
		userID:    resp.JSON200.Id,
		step:      "loading",
	}

	p := tea.NewProgram(initModel{
		state:  state,
		client: client,
	})

	if _, err := p.Run(); err != nil {
		return err
	}

	if state.err != nil {
		return state.err
	}

	return nil
}

type initModel struct {
	state  *initState
	client *sdk.ClientWithResponses
}

func (m initModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadProviders,
		m.detectGitRepo,
	)
}

func (m initModel) loadProviders() tea.Msg {
	ctx := context.Background()
	authHeader := fmt.Sprintf("Bearer %s", m.state.authToken)
	params := &sdk.GetProvidersParams{
		Authorization: &authHeader,
	}

	resp, err := m.client.GetProvidersWithResponse(ctx, params)
	if err != nil {
		return providersLoadedMsg{err: fmt.Errorf("failed to load providers: %w", err)}
	}

	if resp.StatusCode() != 200 {
		var errMsg string
		if resp.ApplicationproblemJSONDefault != nil {
			if resp.ApplicationproblemJSONDefault.Detail != nil {
				errMsg = *resp.ApplicationproblemJSONDefault.Detail
			} else if resp.ApplicationproblemJSONDefault.Title != nil {
				errMsg = *resp.ApplicationproblemJSONDefault.Title
			}
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("status %d", resp.StatusCode())
		}
		return providersLoadedMsg{err: fmt.Errorf("failed to load providers: %s", errMsg)}
	}

	if resp.JSON200 == nil || resp.JSON200.Providers == nil {
		return providersLoadedMsg{err: fmt.Errorf("empty response body")}
	}

	return providersLoadedMsg{providers: *resp.JSON200.Providers}
}

func (m initModel) detectGitRepo() tea.Msg {
	cwd, err := os.Getwd()
	if err != nil {
		return gitRepoDetectedMsg{err: err}
	}

	// Check if current directory is a git repo
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		// Not a git repo or no origin remote
		return gitRepoDetectedMsg{}
	}

	remoteURL := strings.TrimSpace(string(output))
	fmt.Println("remoteURL", remoteURL)

	// Parse git URL to get owner/repo
	// Handle both https://github.com/owner/repo.git and git@github.com:owner/repo.git
	var owner, repo string
	if strings.Contains(remoteURL, "github.com") {
		parts := strings.Split(remoteURL, "github.com")
		if len(parts) > 1 {
			path := strings.Trim(parts[1], "/:")
			path = strings.TrimSuffix(path, ".git")
			pathParts := strings.Split(path, "/")
			if len(pathParts) >= 2 {
				owner = pathParts[0]
				repo = pathParts[1]
			}
		}
	}

	if owner != "" && repo != "" {
		return gitRepoDetectedMsg{
			repo: &gitRepo{
				owner: owner,
				name:  repo,
				url:   remoteURL,
			},
		}
	}

	return gitRepoDetectedMsg{}
}

func (m initModel) loadGitHubRepos() tea.Msg {
	// Get GitHub token from API using SDK
	ctx := context.Background()
	authHeader := fmt.Sprintf("Bearer %s", m.state.authToken)
	params := &sdk.GetCredentialsGithubTokenParams{
		Authorization: &authHeader,
	}

	tokenResp, err := m.client.GetCredentialsGithubTokenWithResponse(ctx, params)
	if err != nil {
		return githubReposLoadedMsg{err: fmt.Errorf("failed to get GitHub token: %w", err)}
	}

	if tokenResp.StatusCode() != 200 {
		var errMsg string
		if tokenResp.ApplicationproblemJSONDefault != nil {
			if tokenResp.ApplicationproblemJSONDefault.Detail != nil {
				errMsg = *tokenResp.ApplicationproblemJSONDefault.Detail
			} else if tokenResp.ApplicationproblemJSONDefault.Title != nil {
				errMsg = *tokenResp.ApplicationproblemJSONDefault.Title
			}
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("status %d", tokenResp.StatusCode())
		}
		return githubReposLoadedMsg{err: fmt.Errorf("failed to get GitHub token: %s", errMsg)}
	}

	if tokenResp.JSON200 == nil {
		return githubReposLoadedMsg{err: fmt.Errorf("empty token response")}
	}

	// Use GitHub package to list repos
	ghClient := github.NewClient(ctx, tokenResp.JSON200.Token)
	repos, err := github.ListRepositories(ctx, ghClient, 100)
	if err != nil {
		return githubReposLoadedMsg{err: err}
	}

	return githubReposLoadedMsg{repos: repos}
}

func (m initModel) validateDockerfile() tea.Cmd {
	return func() tea.Msg {
		// Get GitHub token from API using SDK
		ctx := context.Background()
		authHeader := fmt.Sprintf("Bearer %s", m.state.authToken)
		params := &sdk.GetCredentialsGithubTokenParams{
			Authorization: &authHeader,
		}

		tokenResp, err := m.client.GetCredentialsGithubTokenWithResponse(ctx, params)
		if err != nil {
			return dockerfileValidatedMsg{
				path:   m.state.dockerfilePath,
				exists: false,
				err:    fmt.Errorf("failed to get GitHub token: %w", err),
			}
		}

		if tokenResp.StatusCode() != 200 {
			var errMsg string
			if tokenResp.ApplicationproblemJSONDefault != nil {
				if tokenResp.ApplicationproblemJSONDefault.Detail != nil {
					errMsg = *tokenResp.ApplicationproblemJSONDefault.Detail
				} else if tokenResp.ApplicationproblemJSONDefault.Title != nil {
					errMsg = *tokenResp.ApplicationproblemJSONDefault.Title
				}
			}
			if errMsg == "" {
				errMsg = fmt.Sprintf("status %d", tokenResp.StatusCode())
			}
			return dockerfileValidatedMsg{
				path:   m.state.dockerfilePath,
				exists: false,
				err:    fmt.Errorf("failed to get GitHub token: %s", errMsg),
			}
		}

		if tokenResp.JSON200 == nil {
			return dockerfileValidatedMsg{
				path:   m.state.dockerfilePath,
				exists: false,
				err:    fmt.Errorf("empty token response"),
			}
		}

		// Use GitHub package to check if file exists
		ghClient := github.NewClient(ctx, tokenResp.JSON200.Token)
		exists, err := github.FileExists(ctx, ghClient, m.state.selectedRepo.Owner, m.state.selectedRepo.Name, m.state.dockerfilePath)
		if err != nil {
			return dockerfileValidatedMsg{
				path:   m.state.dockerfilePath,
				exists: false,
				err:    err,
			}
		}

		return dockerfileValidatedMsg{
			path:   m.state.dockerfilePath,
			exists: exists,
			err:    nil,
		}
	}
}

func (m initModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		// Handle keyboard input based on current step
		switch m.state.step {
		case "confirm_repo":
			return m.handleConfirmRepoKeys(msg)
		case "select_repo":
			return m.handleRepoSelectionKeys(msg)
		case "dockerfile":
			return m.handleDockerfileKeys(msg)
		}

		return m, nil

	case providersLoadedMsg:
		if msg.err != nil {
			m.state.err = msg.err
			return m, tea.Quit
		}
		m.state.providers = msg.providers
		return m, m.checkProvidersAndContinue()

	case gitRepoDetectedMsg:
		if msg.err != nil {
			// Ignore errors, just continue without current repo
			return m, nil
		}
		m.state.currentRepo = msg.repo
		return m, nil

	case githubReposLoadedMsg:
		if msg.err != nil {
			m.state.err = msg.err
			return m, tea.Quit
		}
		if len(msg.repos) == 0 {
			m.state.err = fmt.Errorf("no repositories found")
			return m, tea.Quit
		}
		m.state.availableRepos = msg.repos
		m.state.repoSelectionCursor = 0
		m.state.step = "select_repo"
		return m, nil

	case repoSelectedMsg:
		m.state.selectedRepo = msg.repo
		m.state.step = "dockerfile"
		m.state.dockerfileInput = "Dockerfile"
		m.state.dockerfileFocused = true
		return m, nil

	case confirmRepoMsg:
		if msg.useCurrent {
			// Use current repo, convert to githubRepo format
			m.state.selectedRepo = &githubRepo{
				Owner:    m.state.currentRepo.owner,
				Name:     m.state.currentRepo.name,
				FullName: fmt.Sprintf("%s/%s", m.state.currentRepo.owner, m.state.currentRepo.name),
				CloneURL: m.state.currentRepo.url,
			}
			m.state.step = "dockerfile"
			m.state.dockerfileInput = "Dockerfile"
			m.state.dockerfileFocused = true
			return m, nil
		} else {
			// Load repos for selection
			return m, func() tea.Msg {
				return m.loadGitHubRepos()
			}
		}

	case dockerfileSubmittedMsg:
		m.state.dockerfilePath = msg.path
		return m, m.validateDockerfile()

	case dockerfileValidatedMsg:
		if msg.err != nil {
			m.state.err = fmt.Errorf("failed to validate Dockerfile: %w", msg.err)
			return m, tea.Quit
		}
		if !msg.exists {
			m.state.err = fmt.Errorf("Dockerfile not found at path: %s", msg.path)
			return m, tea.Quit
		}
		// File exists, proceed with config creation
		return m, m.createConfig()

	case configCreatedMsg:
		if msg.err != nil {
			m.state.err = msg.err
			return m, tea.Quit
		}
		m.state.configID = msg.configID
		m.state.webhookSecret = msg.webhookSecret
		return m, m.setupWebhook()

	case webhookSetupMsg:
		if msg.err != nil {
			m.state.err = msg.err
			return m, tea.Quit
		}
		m.state.step = "complete"
		return m, tea.Quit
	}

	return m, nil
}

func (m initModel) checkProvidersAndContinue() tea.Cmd {
	// Check if GitHub is connected
	hasGitHub := false
	for _, p := range m.state.providers {
		if p == "github" {
			hasGitHub = true
			break
		}
	}

	if !hasGitHub {
		m.state.err = fmt.Errorf("GitHub not connected. Please run 'nimbul connect' first")
		return tea.Quit
	}

	// If we have a current repo, ask if they want to use it
	if m.state.currentRepo != nil {
		m.state.step = "confirm_repo"
		m.state.confirmRepoCursor = 0 // Default to Yes
		return nil
	}

	// Otherwise, load GitHub repos for selection
	return func() tea.Msg {
		return m.loadGitHubRepos()
	}
}

func (m initModel) handleConfirmRepoKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown:
		m.state.confirmRepoCursor = 1 - m.state.confirmRepoCursor // Toggle between 0 and 1
		return m, nil
	case tea.KeyEnter:
		return m, func() tea.Msg {
			return confirmRepoMsg{useCurrent: m.state.confirmRepoCursor == 0}
		}
	}
	return m, nil
}

func (m initModel) handleRepoSelectionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.state.repoSelectionCursor > 0 {
			m.state.repoSelectionCursor--
		} else {
			m.state.repoSelectionCursor = len(m.state.availableRepos) - 1
		}
		return m, nil
	case tea.KeyDown:
		if m.state.repoSelectionCursor < len(m.state.availableRepos)-1 {
			m.state.repoSelectionCursor++
		} else {
			m.state.repoSelectionCursor = 0
		}
		return m, nil
	case tea.KeyEnter:
		if m.state.repoSelectionCursor < len(m.state.availableRepos) {
			return m, func() tea.Msg {
				return repoSelectedMsg{repo: &m.state.availableRepos[m.state.repoSelectionCursor]}
			}
		}
	}
	return m, nil
}

func (m initModel) handleDockerfileKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.state.dockerfileFocused {
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEnter:
		if m.state.dockerfileInput == "" {
			m.state.dockerfileInput = "Dockerfile"
		}
		return m, func() tea.Msg {
			return dockerfileSubmittedMsg{path: m.state.dockerfileInput}
		}
	case tea.KeyBackspace:
		if len(m.state.dockerfileInput) > 0 {
			m.state.dockerfileInput = m.state.dockerfileInput[:len(m.state.dockerfileInput)-1]
		}
		return m, nil
	case tea.KeyRunes:
		m.state.dockerfileInput += string(msg.Runes)
		return m, nil
	}
	return m, nil
}

type configCreatedMsg struct {
	configID      string
	webhookSecret string
	err           error
}

func (m initModel) createConfig() tea.Cmd {
	return func() tea.Msg {
		// Generate webhook secret
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			return configCreatedMsg{err: fmt.Errorf("failed to generate webhook secret: %w", err)}
		}
		webhookSecret := hex.EncodeToString(secretBytes)

		ctx := context.Background()
		authHeader := fmt.Sprintf("Bearer %s", m.state.authToken)
		params := &sdk.PostConfigsParams{
			Authorization: &authHeader,
		}

		reqBody := sdk.CreateConfigRequestBody{
			Provider:       "github",
			RepoOwner:      m.state.selectedRepo.Owner,
			RepoName:       m.state.selectedRepo.Name,
			RepoFullName:   m.state.selectedRepo.FullName,
			RepoCloneUrl:   m.state.selectedRepo.CloneURL,
			DockerfilePath: m.state.dockerfilePath,
			WebhookSecret:  webhookSecret,
		}

		resp, err := m.client.PostConfigsWithResponse(ctx, params, reqBody)
		if err != nil {
			return configCreatedMsg{err: fmt.Errorf("failed to create config: %w", err)}
		}

		if resp.StatusCode() != 200 {
			var errMsg string
			if resp.ApplicationproblemJSONDefault != nil {
				if resp.ApplicationproblemJSONDefault.Detail != nil {
					errMsg = *resp.ApplicationproblemJSONDefault.Detail
				} else if resp.ApplicationproblemJSONDefault.Title != nil {
					errMsg = *resp.ApplicationproblemJSONDefault.Title
				}
			}
			if errMsg == "" {
				errMsg = fmt.Sprintf("status %d", resp.StatusCode())
			}
			return configCreatedMsg{err: fmt.Errorf("failed to create config: %s", errMsg)}
		}

		if resp.JSON200 == nil {
			return configCreatedMsg{err: fmt.Errorf("empty response body")}
		}

		return configCreatedMsg{
			configID:      resp.JSON200.ConfigId,
			webhookSecret: webhookSecret,
		}
	}
}

type webhookSetupMsg struct {
	webhookID int64
	err       error
}

func (m initModel) setupWebhook() tea.Cmd {
	return func() tea.Msg {
		// Get GitHub token using SDK
		ctx := context.Background()
		authHeader := fmt.Sprintf("Bearer %s", m.state.authToken)
		params := &sdk.GetCredentialsGithubTokenParams{
			Authorization: &authHeader,
		}

		tokenResp, err := m.client.GetCredentialsGithubTokenWithResponse(ctx, params)
		if err != nil {
			return webhookSetupMsg{err: fmt.Errorf("failed to get GitHub token: %w", err)}
		}

		if tokenResp.StatusCode() != 200 {
			var errMsg string
			if tokenResp.ApplicationproblemJSONDefault != nil {
				if tokenResp.ApplicationproblemJSONDefault.Detail != nil {
					errMsg = *tokenResp.ApplicationproblemJSONDefault.Detail
				} else if tokenResp.ApplicationproblemJSONDefault.Title != nil {
					errMsg = *tokenResp.ApplicationproblemJSONDefault.Title
				}
			}
			if errMsg == "" {
				errMsg = fmt.Sprintf("status %d", tokenResp.StatusCode())
			}
			return webhookSetupMsg{err: fmt.Errorf("failed to get GitHub token: %s", errMsg)}
		}

		if tokenResp.JSON200 == nil {
			return webhookSetupMsg{err: fmt.Errorf("empty token response")}
		}

		// Get installation ID using user token
		installationID, err := github.GetUserInstallationID(ctx, tokenResp.JSON200.Token)
		if err != nil {
			return webhookSetupMsg{err: fmt.Errorf("failed to get installation ID: %w", err)}
		}

		// Create GitHub app auth with installation ID
		appAuth, err := github.NewAppAuth(installationID)
		if err != nil {
			return webhookSetupMsg{err: fmt.Errorf("failed to create app auth: %w", err)}
		}

		// Get installation client for creating webhook
		installClient, err := appAuth.GetInstallationClient(ctx)
		if err != nil {
			return webhookSetupMsg{err: fmt.Errorf("failed to get installation client: %w", err)}
		}

		// Get API base URL for webhook URL
		apiBaseURL := getAPIBaseURL()
		webhookURL := fmt.Sprintf("%s/webhooks/github/%s", apiBaseURL, m.state.configID)

		// Setup webhook via GitHub API using app installation auth
		webhookID, err := github.CreateWebhook(ctx, installClient, m.state.selectedRepo.Owner, m.state.selectedRepo.Name, webhookURL, m.state.webhookSecret)
		if err != nil {
			return webhookSetupMsg{err: err}
		}

		// Update config with webhook ID using SDK
		updateParams := &sdk.PatchConfigsByIdWebhookParams{
			Authorization: &authHeader,
		}
		updateBody := sdk.UpdateConfigWebhookRequestBody{
			WebhookId: webhookID,
		}
		updateResp, err := m.client.PatchConfigsByIdWebhookWithResponse(ctx, m.state.configID, updateParams, updateBody)
		if err != nil {
			// Log error but don't fail - webhook was created successfully
			fmt.Fprintf(os.Stderr, "Warning: Failed to update webhook ID: %v\n", err)
		} else if updateResp.StatusCode() != 200 {
			var errMsg string
			if updateResp.ApplicationproblemJSONDefault != nil {
				if updateResp.ApplicationproblemJSONDefault.Detail != nil {
					errMsg = *updateResp.ApplicationproblemJSONDefault.Detail
				} else if updateResp.ApplicationproblemJSONDefault.Title != nil {
					errMsg = *updateResp.ApplicationproblemJSONDefault.Title
				}
			}
			if errMsg == "" {
				errMsg = fmt.Sprintf("status %d", updateResp.StatusCode())
			}
			fmt.Fprintf(os.Stderr, "Warning: Failed to update webhook ID: %s\n", errMsg)
		}

		return webhookSetupMsg{webhookID: webhookID}
	}
}

func (m initModel) View() string {
	var s strings.Builder

	switch m.state.step {
	case "loading":
		s.WriteString(loadingStyle.Render("Loading providers and detecting git repository...\n"))

	case "confirm_repo":
		s.WriteString(titleStyle.Render("Repository Detected\n\n"))
		s.WriteString(fmt.Sprintf("Detected repository: %s/%s\n\n", m.state.currentRepo.owner, m.state.currentRepo.name))
		s.WriteString("Use this repository?\n\n")

		yesStyle := labelStyle
		noStyle := labelStyle
		if m.state.confirmRepoCursor == 0 {
			yesStyle = inputFocusedStyle
		} else {
			noStyle = inputFocusedStyle
		}

		s.WriteString(yesStyle.Render("  → Yes"))
		if m.state.confirmRepoCursor == 0 {
			s.WriteString(" ✓")
		}
		s.WriteString("\n")
		s.WriteString(noStyle.Render("  → No"))
		if m.state.confirmRepoCursor == 1 {
			s.WriteString(" ✓")
		}
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lightGray).Render("Use ↑↓ to select, Enter to confirm"))

	case "select_repo":
		s.WriteString(titleStyle.Render("Select Repository\n\n"))
		if len(m.state.availableRepos) == 0 {
			s.WriteString("No repositories found.\n")
		} else {
			// Show up to 10 repos at a time
			start := 0
			end := len(m.state.availableRepos)
			if end > 10 {
				// Simple pagination - show around cursor
				if m.state.repoSelectionCursor > 5 {
					start = m.state.repoSelectionCursor - 5
				}
				end = start + 10
				if end > len(m.state.availableRepos) {
					end = len(m.state.availableRepos)
					start = end - 10
					if start < 0 {
						start = 0
					}
				}
			}

			for i := start; i < end; i++ {
				repo := m.state.availableRepos[i]
				if i == m.state.repoSelectionCursor {
					s.WriteString(inputFocusedStyle.Render(fmt.Sprintf("  → %s", repo.FullName)))
					s.WriteString(" ✓")
				} else {
					s.WriteString(labelStyle.Render(fmt.Sprintf("    %s", repo.FullName)))
				}
				s.WriteString("\n")
			}
			if len(m.state.availableRepos) > 10 {
				s.WriteString(fmt.Sprintf("\nShowing %d-%d of %d repositories\n", start+1, end, len(m.state.availableRepos)))
			}
		}
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lightGray).Render("Use ↑↓ to navigate, Enter to select"))

	case "dockerfile":
		s.WriteString(titleStyle.Render("Dockerfile Path\n\n"))
		s.WriteString(fmt.Sprintf("Repository: %s\n\n", m.state.selectedRepo.FullName))
		s.WriteString(labelStyle.Render("Dockerfile path:"))
		s.WriteString("\n")

		if m.state.dockerfileFocused {
			if m.state.dockerfileInput == "" {
				s.WriteString(inputFocusedStyle.Render("Dockerfile█"))
			} else {
				s.WriteString(inputFocusedStyle.Render(m.state.dockerfileInput + "█"))
			}
		} else {
			if m.state.dockerfileInput == "" {
				s.WriteString(inputStyle.Render("Dockerfile"))
			} else {
				s.WriteString(inputStyle.Render(m.state.dockerfileInput))
			}
		}
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lightGray).Render("Enter path to Dockerfile, then press Enter"))

	case "complete":
		s.WriteString(successStyle.Render("✓ Nimbul initialized successfully!\n\n"))
		s.WriteString(fmt.Sprintf("Config ID: %s\n", m.state.configID))
		s.WriteString("Webhook has been set up. Commits to your repository will trigger builds.\n")

	default:
		if m.state.err != nil {
			s.WriteString(errorStyle.Render(fmt.Sprintf("✗ Error: %v\n", m.state.err)))
		}
	}

	return s.String()
}
