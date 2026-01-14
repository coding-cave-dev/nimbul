package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v81/github"
)

const DefaultAppSlug = "nimbul-coding-cave"

// InstallationInfo contains information about a GitHub App installation
type InstallationInfo struct {
	Installed      bool
	InstallationID int64
	InstallURL     string
}

// CheckAppInstallation checks if the specified GitHub App is installed for the user
func CheckAppInstallation(ctx context.Context, client *github.Client, appSlug string) (*InstallationInfo, error) {
	installations, _, err := client.Apps.ListUserInstallations(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list app installations: %w", err)
	}

	info := &InstallationInfo{
		Installed:  false,
		InstallURL: fmt.Sprintf("https://github.com/apps/%s/installations/new", appSlug),
	}

	for _, installation := range installations {
		if installation.GetAppSlug() == appSlug {
			info.Installed = true
			info.InstallationID = installation.GetID()
			break
		}
	}

	return info, nil
}

// VerifyAppInstallation verifies that the app is installed and returns the installation ID
func VerifyAppInstallation(ctx context.Context, client *github.Client, appSlug string) (int64, error) {
	info, err := CheckAppInstallation(ctx, client, appSlug)
	if err != nil {
		return 0, err
	}

	if !info.Installed {
		return 0, fmt.Errorf("app '%s' is not installed", appSlug)
	}

	return info.InstallationID, nil
}

// TestInstallationAuth tests that installation authentication works by listing webhooks
func TestInstallationAuth(ctx context.Context, installClient *github.Client, userClient *github.Client) error {
	// Get user's repos to find one to test webhooks on
	repos, _, err := userClient.Repositories.ListByAuthenticatedUser(ctx, &github.RepositoryListByAuthenticatedUserOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if err != nil {
		return fmt.Errorf("failed to list repos for testing: %w", err)
	}

	if len(repos) == 0 {
		return fmt.Errorf("no repositories found to test webhooks")
	}

	// Try to list webhooks using installation auth
	testRepo := repos[0]
	_, _, err = installClient.Repositories.ListHooks(ctx, testRepo.GetOwner().GetLogin(), testRepo.GetName(), nil)
	if err != nil {
		return fmt.Errorf("failed to list webhooks with installation auth: %w", err)
	}

	return nil
}
