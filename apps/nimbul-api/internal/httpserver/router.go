package httpserver

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/coding-cave-dev/nimbul/internal/auth"
	"github.com/coding-cave-dev/nimbul/internal/credentials"
	"github.com/coding-cave-dev/nimbul/internal/db"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
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
