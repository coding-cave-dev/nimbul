package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrInvalidPassword    = errors.New("password must be at least 8 characters long")
	ErrInvalidToken       = errors.New("invalid or expired token")
)
