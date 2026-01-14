package github

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// GetInstallationIDByRepository gets the installation ID for a specific repository
// Uses app JWT authentication to find the installation that has access to the repo
func GetInstallationIDByRepository(ctx context.Context, owner, repo string) (int64, error) {
	// Create app JWT for authentication
	appIDStr := os.Getenv("GITHUB_APP_ID")
	if appIDStr == "" {
		return 0, fmt.Errorf("GITHUB_APP_ID environment variable is not set")
	}

	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid GITHUB_APP_ID: %w", err)
	}

	privateKeyPEM := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	if privateKeyPEM == "" {
		return 0, fmt.Errorf("GITHUB_APP_PRIVATE_KEY environment variable is not set")
	}

	// Parse private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return 0, fmt.Errorf("failed to parse private key PEM")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return 0, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return 0, fmt.Errorf("private key is not RSA")
		}
	}

	// Generate JWT
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": appID,
	})

	jwtToken, err := token.SignedString(privateKey)
	if err != nil {
		return 0, fmt.Errorf("failed to sign JWT: %w", err)
	}

	// Create GitHub client with app JWT
	appClient := github.NewClient(nil).WithAuthToken(jwtToken)

	// Get repository installation
	installation, _, err := appClient.Apps.FindRepositoryInstallation(ctx, owner, repo)
	if err != nil {
		return 0, fmt.Errorf("failed to find repository installation: %w", err)
	}

	return installation.GetID(), nil
}
