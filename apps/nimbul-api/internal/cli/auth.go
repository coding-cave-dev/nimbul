package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coding-cave-dev/nimbul/internal/sdk"
	"gopkg.in/yaml.v3"
)

type AuthResponse struct {
	Token string
	User  struct {
		ID    string
		Email string
	}
}

type Config struct {
	APIURL string `yaml:"api_url"`
}

func getAPIBaseURL() string {
	// Check environment variable first
	if apiURL := os.Getenv("NIMBUL_API_URL"); apiURL != "" {
		return apiURL
	}

	// Try to read from config file
	configDir, err := os.UserConfigDir()
	if err == nil {
		configPath := filepath.Join(configDir, "nimbul", "config.yaml")
		if data, err := os.ReadFile(configPath); err == nil {
			var config Config
			if err := yaml.Unmarshal(data, &config); err == nil && config.APIURL != "" {
				return config.APIURL
			}
		}
	}

	// Default fallback
	return "http://localhost:8080"
}

func getTokenPath() string {
	if tokenPath := os.Getenv("NIMBUL_TOKEN_PATH"); tokenPath != "" {
		return tokenPath
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to home directory
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".nimbul", "token")
	}

	return filepath.Join(configDir, "nimbul", "token")
}

func saveToken(token string) error {
	tokenPath := getTokenPath()
	dir := filepath.Dir(tokenPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// Write token with secure permissions
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}

	return nil
}

func loadToken() (string, error) {
	tokenPath := getTokenPath()
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read token: %w", err)
	}
	return string(data), nil
}

func getSDKClient() (*sdk.ClientWithResponses, error) {
	baseURL := getAPIBaseURL()
	client, err := sdk.NewClientWithResponses(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create SDK client: %w", err)
	}
	return client, nil
}

func makeAuthRequest(endpoint, email, password string) (*AuthResponse, error) {
	client, err := getSDKClient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	reqBody := sdk.LoginRequestBody{
		Email:    email,
		Password: password,
	}

	var resp *AuthResponse
	var errMsg string

	switch endpoint {
	case "/login":
		loginResp, err := client.PostLoginWithResponse(ctx, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}

		if loginResp.StatusCode() != 200 {
			if loginResp.ApplicationproblemJSONDefault != nil {
				if loginResp.ApplicationproblemJSONDefault.Detail != nil {
					errMsg = *loginResp.ApplicationproblemJSONDefault.Detail
				} else if loginResp.ApplicationproblemJSONDefault.Title != nil {
					errMsg = *loginResp.ApplicationproblemJSONDefault.Title
				} else {
					errMsg = fmt.Sprintf("request failed with status %d", loginResp.StatusCode())
				}
			} else {
				errMsg = fmt.Sprintf("request failed with status %d", loginResp.StatusCode())
			}
			return nil, fmt.Errorf("%s", errMsg)
		}

		if loginResp.JSON200 == nil {
			return nil, fmt.Errorf("empty response body")
		}

		resp = &AuthResponse{
			Token: loginResp.JSON200.Token,
			User: struct {
				ID    string
				Email string
			}{
				ID:    loginResp.JSON200.User.Id,
				Email: loginResp.JSON200.User.Email,
			},
		}

	case "/register":
		registerResp, err := client.PostRegisterWithResponse(ctx, sdk.RegisterRequestBody{
			Email:    email,
			Password: password,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}

		if registerResp.StatusCode() != 200 {
			if registerResp.ApplicationproblemJSONDefault != nil {
				if registerResp.ApplicationproblemJSONDefault.Detail != nil {
					errMsg = *registerResp.ApplicationproblemJSONDefault.Detail
				} else if registerResp.ApplicationproblemJSONDefault.Title != nil {
					errMsg = *registerResp.ApplicationproblemJSONDefault.Title
				} else {
					errMsg = fmt.Sprintf("request failed with status %d", registerResp.StatusCode())
				}
			} else {
				errMsg = fmt.Sprintf("request failed with status %d", registerResp.StatusCode())
			}
			return nil, fmt.Errorf("%s", errMsg)
		}

		if registerResp.JSON200 == nil {
			return nil, fmt.Errorf("empty response body")
		}

		resp = &AuthResponse{
			Token: registerResp.JSON200.Token,
			User: struct {
				ID    string
				Email string
			}{
				ID:    registerResp.JSON200.User.Id,
				Email: registerResp.JSON200.User.Email,
			},
		}

	default:
		return nil, fmt.Errorf("unknown endpoint: %s", endpoint)
	}

	return resp, nil
}
