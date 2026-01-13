package httpserver

import (
	"context"
	"strings"

	"github.com/coding-cave-dev/nimbul/internal/auth"
	"github.com/danielgtaylor/huma/v2"
)

type contextKey string

const (
	userIDKey contextKey = "userID"
	emailKey  contextKey = "email"
)

// AuthResolver is a reusable resolver that extracts and validates JWT tokens
// from the Authorization header. It can be embedded in request structs.
type AuthResolver struct {
	Authorization string `header:"Authorization"`
}

// ValidateAuth validates the JWT token from the Authorization header and injects
// user information into the context. Returns an error if the token is missing or invalid.
func ValidateAuth(ctx context.Context, authHeader string, authService *auth.Service) (context.Context, error) {
	if authHeader == "" {
		return ctx, huma.Error401Unauthorized("Missing Authorization header")
	}

	// Extract Bearer token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ctx, huma.Error401Unauthorized("Invalid Authorization header format")
	}

	token := parts[1]

	// Validate token and get user ID
	userID, email, err := authService.ValidateToken(token)
	if err != nil {
		return ctx, huma.Error401Unauthorized("Invalid or expired token")
	}

	// Inject user info into context
	ctx = context.WithValue(ctx, userIDKey, userID)
	ctx = context.WithValue(ctx, emailKey, email)

	return ctx, nil
}

// GetUserID extracts the user ID from the context set by ValidateAuth.
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(userIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetUserEmail extracts the email from the context set by ValidateAuth.
func GetUserEmail(ctx context.Context) string {
	if email, ok := ctx.Value(emailKey).(string); ok {
		return email
	}
	return ""
}
