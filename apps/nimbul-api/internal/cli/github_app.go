package cli

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

// GitHubAppAuth provides authentication for GitHub App installations
type GitHubAppAuth struct {
	appID          int64
	privateKey     *rsa.PrivateKey
	installationID int64
}

// NewGitHubAppAuth creates a new GitHubAppAuth instance from environment variables
func NewGitHubAppAuth(installationID int64) (*GitHubAppAuth, error) {
	// Get app credentials from environment
	appIDStr := os.Getenv("GITHUB_APP_ID")
	if appIDStr == "" {
		return nil, fmt.Errorf("GITHUB_APP_ID environment variable is not set")
	}

	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_ID: %w", err)
	}

	privateKeyPEM := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	if privateKeyPEM == "" {
		return nil, fmt.Errorf("GITHUB_APP_PRIVATE_KEY environment variable is not set")
	}

	// Parse private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse private key PEM")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}

	return &GitHubAppAuth{
		appID:          appID,
		privateKey:     privateKey,
		installationID: installationID,
	}, nil
}

// GetInstallationToken generates a JWT and exchanges it for an installation token
func (g *GitHubAppAuth) GetInstallationToken(ctx context.Context) (string, error) {
	// Generate JWT for app authentication
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(), // Issued at time (60 seconds ago to account for clock skew)
		"exp": now.Add(10 * time.Minute).Unix(),  // Expires in 10 minutes
		"iss": g.appID,                           // Issuer (App ID)
	})

	jwtToken, err := token.SignedString(g.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	// Create GitHub client with app JWT
	appClient := github.NewClient(nil).WithAuthToken(jwtToken)

	// Get installation token
	installationToken, _, err := appClient.Apps.CreateInstallationToken(ctx, g.installationID, &github.InstallationTokenOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create installation token: %w", err)
	}

	return installationToken.GetToken(), nil
}

// GetInstallationClient creates a GitHub client authenticated with the installation token
func (g *GitHubAppAuth) GetInstallationClient(ctx context.Context) (*github.Client, error) {
	token, err := g.GetInstallationToken(ctx)
	if err != nil {
		return nil, err
	}

	return github.NewClient(nil).WithAuthToken(token), nil
}

// GetUserInstallationID finds the installation ID for the nimbul-coding-cave app for a given user token
func GetUserInstallationID(ctx context.Context, userToken string) (int64, error) {
	ghClient := github.NewClient(nil).WithAuthToken(userToken)

	// Get user's app installations
	installations, _, err := ghClient.Apps.ListUserInstallations(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to list app installations: %w", err)
	}

	// Check if nimbul-coding-cave app is installed
	appSlug := "nimbul-coding-cave"
	for _, installation := range installations {
		if installation.GetAppSlug() == appSlug {
			return installation.GetID(), nil
		}
	}

	return 0, fmt.Errorf("app '%s' is not installed", appSlug)
}
