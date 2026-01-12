package httpserver

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/coding-cave-dev/nimbul/internal/auth"
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
	Authorization string `header:"Authorization"`
}

type MeResponse struct {
	Body auth.UserResponse `json:"body"`
}

func NewRouter(queries *db.Queries) *fiber.App {
	app := fiber.New()

	api := humafiber.New(app, huma.DefaultConfig("Nimbul API", "1.0.0"))

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-secret-change-in-production"
	}

	authService := auth.NewService(queries, jwtSecret)

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
		// Extract Authorization header
		authHeader := input.Authorization
		if authHeader == "" {
			return nil, huma.Error401Unauthorized("Missing Authorization header")
		}

		// Extract Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return nil, huma.Error401Unauthorized("Invalid Authorization header format")
		}

		token := parts[1]

		// Validate token and get user ID
		userID, _, err := authService.ValidateToken(token)
		if err != nil {
			return nil, huma.Error401Unauthorized("Invalid or expired token")
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
