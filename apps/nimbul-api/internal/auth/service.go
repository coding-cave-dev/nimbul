package auth

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/coding-cave-dev/nimbul/internal/db"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	queries   *db.Queries
	jwtSecret string
}

func NewService(queries *db.Queries, jwtSecret string) *Service {
	return &Service{
		queries:   queries,
		jwtSecret: jwtSecret,
	}
}

type RegisterResult struct {
	User  UserResponse
	Token string
}

type LoginResult struct {
	User  UserResponse
	Token string
}

type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (s *Service) Register(ctx context.Context, email, password string) (*RegisterResult, error) {
	// Validate email format
	if !isValidEmail(email) {
		return nil, ErrInvalidEmail
	}

	// Validate password
	if len(password) < 8 {
		return nil, ErrInvalidPassword
	}

	// Check if user already exists
	_, err := s.queries.GetUserByEmail(ctx, email)
	if err == nil {
		return nil, ErrEmailExists
	}
	// If error is not "no rows", it's a different error
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Generate ULID for user ID
	userID := ulid.Make().String()

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create user
	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		ID:           userID,
		Email:        email,
		PasswordHash: string(passwordHash),
	})
	if err != nil {
		return nil, err
	}

	// Generate JWT token
	token, err := s.generateToken(userID, email)
	if err != nil {
		return nil, err
	}

	return &RegisterResult{
		User: UserResponse{
			ID:    user.ID,
			Email: user.Email,
		},
		Token: token,
	}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	// Get user by email
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Compare password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := s.generateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		User: UserResponse{
			ID:    user.ID,
			Email: user.Email,
		},
		Token: token,
	}, nil
}

func (s *Service) generateToken(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *Service) ValidateToken(tokenString string) (string, string, error) {
	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return "", "", ErrInvalidToken
	}

	// Validate token
	if !token.Valid {
		return "", "", ErrInvalidToken
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", ErrInvalidToken
	}

	// Get user_id and email from claims
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return "", "", ErrInvalidToken
	}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return "", "", ErrInvalidToken
	}

	return userID, email, nil
}

func (s *Service) GetUserByID(ctx context.Context, userID string) (*UserResponse, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &UserResponse{
		ID:    user.ID,
		Email: user.Email,
	}, nil
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
