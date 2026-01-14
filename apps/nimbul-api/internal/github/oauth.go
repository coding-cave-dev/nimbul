package github

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	oauth2github "golang.org/x/oauth2/github"
)

// OAuthConfig holds OAuth configuration for GitHub
type OAuthConfig struct {
	config *oauth2.Config
}

// NewOAuthConfig creates a new OAuth configuration for GitHub
func NewOAuthConfig() (*OAuthConfig, error) {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_ID environment variable is not set")
	}

	config := &oauth2.Config{
		ClientID: clientID,
		Scopes:   []string{"admin:repo_hook", "repo"},
		Endpoint: oauth2.Endpoint{
			AuthURL:       oauth2github.Endpoint.AuthURL,
			TokenURL:      oauth2github.Endpoint.TokenURL,
			DeviceAuthURL: oauth2github.Endpoint.DeviceAuthURL,
		},
	}

	return &OAuthConfig{config: config}, nil
}

// StartDeviceAuth initiates the device authorization flow
func (o *OAuthConfig) StartDeviceAuth(ctx context.Context) (*oauth2.DeviceAuthResponse, error) {
	device, err := o.config.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start device auth: %w", err)
	}
	return device, nil
}

// PollForToken polls for the access token using the device code
func (o *OAuthConfig) PollForToken(ctx context.Context, device *oauth2.DeviceAuthResponse) (*oauth2.Token, error) {
	token, err := o.config.DeviceAccessToken(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("failed to get device access token: %w", err)
	}
	return token, nil
}
