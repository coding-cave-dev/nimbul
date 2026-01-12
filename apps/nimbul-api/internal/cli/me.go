package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/coding-cave-dev/nimbul/internal/sdk"
	"github.com/spf13/cobra"
)

var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Display current user information",
	Long:  `Display information about the currently logged-in user.`,
	RunE:  meExec,
}

func init() {
	rootCmd.AddCommand(meCmd)
}

func meExec(cmd *cobra.Command, args []string) error {
	// Load token
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

	// Make authenticated request
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

	// Display user information with styling
	orangeColor := lipgloss.Color("#FF6B35")
	grayColor := lipgloss.Color("#808080")

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(orangeColor).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(grayColor).
		MarginRight(2)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	fmt.Println(titleStyle.Render("User Information"))
	fmt.Println()
	fmt.Printf("%s%s\n", labelStyle.Render("Email:"), valueStyle.Render(resp.JSON200.Email))
	fmt.Printf("%s%s\n", labelStyle.Render("ID:"), valueStyle.Render(resp.JSON200.Id))

	return nil
}
