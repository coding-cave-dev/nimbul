package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/coding-cave-dev/nimbul/internal/auth"
	"github.com/coding-cave-dev/nimbul/internal/configs"
	"github.com/coding-cave-dev/nimbul/internal/credentials"
	"github.com/coding-cave-dev/nimbul/internal/db"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
	"github.com/google/go-github/v81/github"
)

type HealthCheckResponse struct {
	Body struct {
		Message string `json:"message"`
	}
}

type RegisterRequest struct {
	Body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
}

type RegisterResponse struct {
	Body struct {
		Token string            `json:"token"`
		User  auth.UserResponse `json:"user"`
	}
}

type LoginRequest struct {
	Body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
}

type LoginResponse struct {
	Body struct {
		Token string            `json:"token"`
		User  auth.UserResponse `json:"user"`
	}
}

type MeRequest struct {
	AuthResolver
}

type MeResponse struct {
	Body auth.UserResponse `json:"body"`
}

type StoreCredentialRequest struct {
	AuthResolver
	Body struct {
		Provider  string    `json:"provider"`
		TokenType string    `json:"token_type"`
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
}

type StoreCredentialResponse struct {
	Body struct {
		CredentialID int64 `json:"credential_id"`
	}
}

type GetProvidersRequest struct {
	AuthResolver
}

type GetProvidersResponse struct {
	Body struct {
		Providers []string `json:"providers"`
	}
}

type CreateConfigRequest struct {
	AuthResolver
	Body struct {
		Provider       string `json:"provider"`
		RepoOwner      string `json:"repo_owner"`
		RepoName       string `json:"repo_name"`
		RepoFullName   string `json:"repo_full_name"`
		RepoCloneURL   string `json:"repo_clone_url"`
		DockerfilePath string `json:"dockerfile_path"`
		WebhookSecret  string `json:"webhook_secret"`
	}
}

type CreateConfigResponse struct {
	Body struct {
		ConfigID string `json:"config_id"`
	}
}

type GitHubWebhookRequest struct {
	ID              string `path:"id"`
	SignatureHeader string `header:"X-Hub-Signature"`
	HookId          int64  `header:"X-GitHub-Hook-ID"`
	EventType       string `header:"X-GitHub-Event"`
	Body            json.RawMessage
	RawBody         []byte
}

type UpdateConfigWebhookRequest struct {
	AuthResolver
	ID   string `path:"id"`
	Body struct {
		WebhookID int64 `json:"webhook_id"`
	}
}

type UpdateConfigWebhookResponse struct {
	Body struct {
		Success bool `json:"success"`
	}
}

type GetGitHubTokenRequest struct {
	AuthResolver
}

type GetGitHubTokenResponse struct {
	Body struct {
		Token string `json:"token"`
	}
}

func NewRouter(queries *db.Queries) *fiber.App {
	app := fiber.New()

	api := humafiber.New(app, huma.DefaultConfig("Nimbul API", "1.0.0"))

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-secret-change-in-production"
	}

	authService := auth.NewService(queries, jwtSecret)

	// Initialize credentials service
	credentialsService, err := credentials.NewService(queries)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize credentials service: %v", err))
	}

	// Initialize configs service
	configsService := configs.NewService(queries)

	huma.Get(api, "/health", func(ctx context.Context, input *struct{}) (*HealthCheckResponse, error) {
		resp := &HealthCheckResponse{}
		resp.Body.Message = "Nimbul API is up and running"
		return resp, nil
	})

	huma.Post(api, "/register", func(ctx context.Context, input *RegisterRequest) (*RegisterResponse, error) {
		result, err := authService.Register(ctx, input.Body.Email, input.Body.Password)
		if err != nil {
			fmt.Println("Error registering:", err)
			return nil, mapAuthError(err)
		}

		resp := &RegisterResponse{}
		resp.Body.Token = result.Token
		resp.Body.User = result.User
		return resp, nil
	})

	huma.Post(api, "/login", func(ctx context.Context, input *LoginRequest) (*LoginResponse, error) {
		result, err := authService.Login(ctx, input.Body.Email, input.Body.Password)
		if err != nil {
			fmt.Println("Error logging in:", err)
			return nil, mapAuthError(err)
		}

		resp := &LoginResponse{}
		resp.Body.Token = result.Token
		resp.Body.User = result.User
		return resp, nil
	})

	huma.Get(api, "/me", func(ctx context.Context, input *MeRequest) (*MeResponse, error) {
		// Validate authentication using middleware
		var err error
		ctx, err = ValidateAuth(ctx, input.AuthResolver.Authorization, authService)
		if err != nil {
			fmt.Println("Error validating auth:", err)
			return nil, err
		}

		// Get user ID from context
		userID := GetUserID(ctx)
		if userID == "" {
			return nil, huma.Error401Unauthorized("User ID not found in context")
		}

		// Get user from database
		user, err := authService.GetUserByID(ctx, userID)
		if err != nil {
			if err.Error() == "user not found" {
				return nil, huma.Error404NotFound("User not found")
			}
			return nil, huma.Error500InternalServerError("Internal server error", err)
		}

		resp := &MeResponse{}
		resp.Body = *user
		return resp, nil
	})

	huma.Post(api, "/credentials", func(ctx context.Context, input *StoreCredentialRequest) (*StoreCredentialResponse, error) {
		// Validate authentication using middleware
		var err error
		ctx, err = ValidateAuth(ctx, input.AuthResolver.Authorization, authService)
		if err != nil {
			return nil, err
		}

		// Get user ID from context
		userID := GetUserID(ctx)
		if userID == "" {
			return nil, huma.Error401Unauthorized("User ID not found in context")
		}

		// Validate input
		if input.Body.Provider == "" {
			return nil, huma.Error400BadRequest("provider is required")
		}
		if input.Body.TokenType == "" {
			return nil, huma.Error400BadRequest("token_type is required")
		}
		if input.Body.Token == "" {
			return nil, huma.Error400BadRequest("token is required")
		}

		// Store credential
		result, err := credentialsService.StoreCredential(ctx, credentials.StoreCredentialParams{
			OwnerID:   userID,
			Provider:  input.Body.Provider,
			TokenType: input.Body.TokenType,
			Token:     input.Body.Token,
			ExpiresAt: input.Body.ExpiresAt,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to store credential", err)
		}

		resp := &StoreCredentialResponse{}
		resp.Body.CredentialID = result.CredentialID
		return resp, nil
	})

	huma.Get(api, "/providers", func(ctx context.Context, input *GetProvidersRequest) (*GetProvidersResponse, error) {
		// Validate authentication using middleware
		var err error
		ctx, err = ValidateAuth(ctx, input.AuthResolver.Authorization, authService)
		if err != nil {
			return nil, err
		}

		// Get user ID from context
		userID := GetUserID(ctx)
		if userID == "" {
			return nil, huma.Error401Unauthorized("User ID not found in context")
		}

		// Get unique providers from credentials
		providers, err := queries.GetUniqueProvidersByOwnerID(ctx, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to get providers", err)
		}

		resp := &GetProvidersResponse{}
		resp.Body.Providers = providers
		return resp, nil
	})

	huma.Post(api, "/configs", func(ctx context.Context, input *CreateConfigRequest) (*CreateConfigResponse, error) {
		// Validate authentication using middleware
		var err error
		ctx, err = ValidateAuth(ctx, input.AuthResolver.Authorization, authService)
		if err != nil {
			return nil, err
		}

		// Get user ID from context
		userID := GetUserID(ctx)
		if userID == "" {
			return nil, huma.Error401Unauthorized("User ID not found in context")
		}

		// Validate input
		if input.Body.Provider == "" {
			return nil, huma.Error400BadRequest("provider is required")
		}
		if input.Body.RepoOwner == "" {
			return nil, huma.Error400BadRequest("repo_owner is required")
		}
		if input.Body.RepoName == "" {
			return nil, huma.Error400BadRequest("repo_name is required")
		}
		if input.Body.RepoFullName == "" {
			return nil, huma.Error400BadRequest("repo_full_name is required")
		}
		if input.Body.RepoCloneURL == "" {
			return nil, huma.Error400BadRequest("repo_clone_url is required")
		}
		if input.Body.DockerfilePath == "" {
			return nil, huma.Error400BadRequest("dockerfile_path is required")
		}
		if input.Body.WebhookSecret == "" {
			return nil, huma.Error400BadRequest("webhook_secret is required")
		}

		// Create config
		result, err := configsService.CreateConfig(ctx, configs.CreateConfigParams{
			OwnerID:        userID,
			Provider:       input.Body.Provider,
			RepoOwner:      input.Body.RepoOwner,
			RepoName:       input.Body.RepoName,
			RepoFullName:   input.Body.RepoFullName,
			RepoCloneURL:   input.Body.RepoCloneURL,
			DockerfilePath: input.Body.DockerfilePath,
			WebhookSecret:  input.Body.WebhookSecret,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to create config", err)
		}

		resp := &CreateConfigResponse{}
		resp.Body.ConfigID = result.ConfigID
		return resp, nil
	})

	huma.Get(api, "/credentials/github/token", func(ctx context.Context, input *GetGitHubTokenRequest) (*GetGitHubTokenResponse, error) {
		// Validate authentication using middleware
		var err error
		ctx, err = ValidateAuth(ctx, input.AuthResolver.Authorization, authService)
		if err != nil {
			return nil, err
		}

		// Get user ID from context
		userID := GetUserID(ctx)
		if userID == "" {
			return nil, huma.Error401Unauthorized("User ID not found in context")
		}

		// Get decrypted GitHub access token
		token, err := credentialsService.GetDecryptedToken(ctx, userID, "github", "oauth_access")
		if err != nil {
			fmt.Println("Error getting GitHub access token:", err)
			// Check if token is expired
			if errors.Is(err, credentials.ErrTokenExpired) {
				// Get refresh token
				refreshToken, refreshErr := credentialsService.GetDecryptedToken(ctx, userID, "github", "oauth_refresh")
				if refreshErr != nil {
					if errors.Is(refreshErr, credentials.ErrRefreshTokenExpired) || errors.Is(refreshErr, credentials.ErrTokenExpired) {
						return nil, huma.Error401Unauthorized("GitHub tokens expired. Please reconnect your GitHub account")
					}
					return nil, huma.Error404NotFound("GitHub refresh token not found")
				}

				// Refresh the tokens
				refreshResult, refreshErr := credentialsService.RefreshGitHubToken(ctx, refreshToken)
				if refreshErr != nil {
					if errors.Is(refreshErr, credentials.ErrRefreshTokenExpired) {
						return nil, huma.Error401Unauthorized("GitHub refresh token expired. Please reconnect your GitHub account")
					}
					return nil, huma.Error500InternalServerError("Failed to refresh GitHub token", refreshErr)
				}

				// Calculate expiry times
				accessExpiry := time.Now().Add(time.Duration(refreshResult.ExpiresIn) * time.Second)
				refreshExpiry := time.Now().Add(6 * 30 * 24 * time.Hour) // 6 months (180 days)

				// Update access token
				updateErr := credentialsService.UpdateCredential(ctx, credentials.UpdateCredentialParams{
					OwnerID:   userID,
					Provider:  "github",
					TokenType: "oauth_access",
					Token:     refreshResult.AccessToken,
					ExpiresAt: accessExpiry,
				})
				if updateErr != nil {
					return nil, huma.Error500InternalServerError("Failed to update access token", updateErr)
				}

				// Update refresh token if a new one was provided
				if refreshResult.RefreshToken != "" {
					updateErr = credentialsService.UpdateCredential(ctx, credentials.UpdateCredentialParams{
						OwnerID:   userID,
						Provider:  "github",
						TokenType: "oauth_refresh",
						Token:     refreshResult.RefreshToken,
						ExpiresAt: refreshExpiry,
					})
					if updateErr != nil {
						return nil, huma.Error500InternalServerError("Failed to update refresh token", updateErr)
					}
				}

				// Return the new access token
				resp := &GetGitHubTokenResponse{}
				resp.Body.Token = refreshResult.AccessToken
				return resp, nil
			}
			return nil, huma.Error404NotFound("GitHub access token not found")
		}

		resp := &GetGitHubTokenResponse{}
		resp.Body.Token = token
		return resp, nil
	})

	huma.Patch(api, "/configs/{id}/webhook", func(ctx context.Context, input *UpdateConfigWebhookRequest) (*UpdateConfigWebhookResponse, error) {
		// Validate authentication using middleware
		var err error
		ctx, err = ValidateAuth(ctx, input.AuthResolver.Authorization, authService)
		if err != nil {
			return nil, err
		}

		// Get user ID from context
		userID := GetUserID(ctx)
		if userID == "" {
			return nil, huma.Error401Unauthorized("User ID not found in context")
		}

		// Verify config belongs to user
		config, err := configsService.GetConfigByID(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("Config not found")
		}

		if config.OwnerID != userID {
			return nil, huma.Error403Forbidden("You don't have permission to update this config")
		}

		// Update webhook ID
		err = configsService.UpdateWebhookID(ctx, input.ID, input.Body.WebhookID)
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to update webhook ID", err)
		}

		resp := &UpdateConfigWebhookResponse{}
		resp.Body.Success = true
		return resp, nil
	})

	huma.Post(api, "/webhooks/github/{id}", func(ctx context.Context, input *GitHubWebhookRequest) (*struct{}, error) {
		// Get config by ID
		config, err := configsService.GetConfigByWebhookID(ctx, input.HookId)
		if err != nil {
			fmt.Println("Error getting config by webhook ID:", err)
			return nil, huma.Error404NotFound("Config not found")
		}

		err = github.ValidateSignature(input.SignatureHeader, input.RawBody, []byte(config.WebhookSecret))
		if err != nil {
			fmt.Println("Error validating webhook signature:", err)
			return nil, huma.Error400BadRequest("Invalid webhook signature")
		}

		event, err := github.ParseWebHook(input.EventType, input.RawBody)
		if err != nil {
			return nil, huma.Error400BadRequest("Invalid webhook payload")
		}

		switch event := event.(type) {
		case *github.PingEvent:
			fmt.Printf("Ping event received: %s\n", *event.Zen)
			return &struct{}{}, nil
		case *github.PushEvent:
			fmt.Printf("Push event received: %+v\n", input.HookId)
			return &struct{}{}, nil
		}

		return &struct{}{}, nil
	})

	generateOpenApi := os.Getenv("GENERATE_OPENAPI_SPEC")
	if generateOpenApi == "true" {
		spec, err := api.OpenAPI().DowngradeYAML()
		if err != nil {
			panic(err)
		}
		os.WriteFile("openapi.yaml", []byte(spec), 0644)
	}

	return app
}

func mapAuthError(err error) error {
	switch err {
	case auth.ErrInvalidCredentials:
		return huma.Error401Unauthorized("Invalid email or password")
	case auth.ErrEmailExists:
		return huma.Error409Conflict("Email already exists")
	case auth.ErrInvalidEmail:
		return huma.Error400BadRequest("Invalid email format")
	case auth.ErrInvalidPassword:
		return huma.Error400BadRequest("Password must be at least 8 characters long")
	default:
		return huma.Error500InternalServerError(fmt.Sprintf("Internal server error: %v", err), err)
	}
}
