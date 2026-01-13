package credentials

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coding-cave-dev/nimbul/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrTokenExpired        = errors.New("token expired")
	ErrRefreshTokenExpired = errors.New("refresh token expired")
)

type Service struct {
	queries   *db.Queries
	masterKey []byte
}

func NewService(queries *db.Queries) (*Service, error) {
	masterKeyStr := os.Getenv("MASTER_ENCRYPTION_KEY")
	if masterKeyStr == "" {
		return nil, fmt.Errorf("MASTER_ENCRYPTION_KEY environment variable is not set")
	}

	masterKey, err := hex.DecodeString(masterKeyStr)
	if err != nil {
		return nil, fmt.Errorf("MASTER_ENCRYPTION_KEY must be a valid hex string: %w", err)
	}

	if len(masterKey) != 32 {
		return nil, fmt.Errorf("MASTER_ENCRYPTION_KEY must be exactly 32 bytes (256 bits) for AES-256")
	}

	return &Service{
		queries:   queries,
		masterKey: masterKey,
	}, nil
}

type StoreCredentialParams struct {
	OwnerID   string
	Provider  string
	TokenType string
	Token     string // Plaintext token to encrypt
	ExpiresAt time.Time
}

type StoreCredentialResult struct {
	CredentialID int64
}

// StoreCredential encrypts and stores a credential using envelope encryption:
// 1. Generates a random DEK (Data Encryption Key)
// 2. Encrypts the token with the DEK using AES-GCM
// 3. Wraps the DEK with the master key using AES-GCM
// 4. Stores everything in the database
func (s *Service) StoreCredential(ctx context.Context, params StoreCredentialParams) (*StoreCredentialResult, error) {
	// Generate random 32-byte DEK for AES-256
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Generate random 12-byte nonce for token encryption
	tokenNonce := make([]byte, 12)
	if _, err := rand.Read(tokenNonce); err != nil {
		return nil, fmt.Errorf("failed to generate token nonce: %w", err)
	}

	// Generate random 12-byte nonce for DEK wrapping
	dekNonce := make([]byte, 12)
	if _, err := rand.Read(dekNonce); err != nil {
		return nil, fmt.Errorf("failed to generate DEK nonce: %w", err)
	}

	// Encrypt token with DEK using AES-GCM
	tokenCiphertext, err := s.encryptWithGCM(dek, tokenNonce, []byte(params.Token))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Wrap DEK with master key using AES-GCM
	wrappedDEK, err := s.encryptWithGCM(s.masterKey, dekNonce, dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Store credential in database
	credential, err := s.queries.CreateCredential(ctx, db.CreateCredentialParams{
		OwnerID:    params.OwnerID,
		Provider:   params.Provider,
		TokenType:  params.TokenType,
		Ciphertext: tokenCiphertext,
		TokenNonce: tokenNonce,
		WrappedDek: wrappedDEK,
		DekNonce:   dekNonce,
		ExpiresAt:  pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store credential: %w", err)
	}

	return &StoreCredentialResult{
		CredentialID: credential.ID,
	}, nil
}

type UpdateCredentialParams struct {
	OwnerID   string
	Provider  string
	TokenType string
	Token     string // Plaintext token to encrypt
	ExpiresAt time.Time
}

// UpdateCredential updates an existing credential with a new token value
// It generates a new DEK for security (key rotation)
func (s *Service) UpdateCredential(ctx context.Context, params UpdateCredentialParams) error {
	// Generate random 32-byte DEK for AES-256
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Generate random 12-byte nonce for token encryption
	tokenNonce := make([]byte, 12)
	if _, err := rand.Read(tokenNonce); err != nil {
		return fmt.Errorf("failed to generate token nonce: %w", err)
	}

	// Generate random 12-byte nonce for DEK wrapping
	dekNonce := make([]byte, 12)
	if _, err := rand.Read(dekNonce); err != nil {
		return fmt.Errorf("failed to generate DEK nonce: %w", err)
	}

	// Encrypt token with DEK using AES-GCM
	tokenCiphertext, err := s.encryptWithGCM(dek, tokenNonce, []byte(params.Token))
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Wrap DEK with master key using AES-GCM
	wrappedDEK, err := s.encryptWithGCM(s.masterKey, dekNonce, dek)
	if err != nil {
		return fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Update credential in database
	_, err = s.queries.UpdateCredential(ctx, db.UpdateCredentialParams{
		OwnerID:    params.OwnerID,
		Provider:   params.Provider,
		TokenType:  params.TokenType,
		Ciphertext: tokenCiphertext,
		TokenNonce: tokenNonce,
		WrappedDek: wrappedDEK,
		DekNonce:   dekNonce,
		ExpiresAt:  pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}

	return nil
}

// GetDecryptedToken retrieves and decrypts a credential token
// Returns ErrTokenExpired if the token has expired
func (s *Service) GetDecryptedToken(ctx context.Context, ownerID, provider, tokenType string) (string, error) {
	// Get credential from database
	credential, err := s.queries.GetCredentialByOwnerIDAndTokenType(ctx, db.GetCredentialByOwnerIDAndTokenTypeParams{
		OwnerID:   ownerID,
		TokenType: tokenType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get credential: %w", err)
	}

	// Verify provider matches
	if credential.Provider != provider {
		return "", fmt.Errorf("provider mismatch: expected %s, got %s", provider, credential.Provider)
	}

	// Check if token is expired
	if credential.ExpiresAt.Valid {
		if time.Now().After(credential.ExpiresAt.Time) {
			return "", ErrTokenExpired
		}
	}

	// Decrypt wrapped DEK with master key
	dek, err := s.decryptWithGCM(s.masterKey, credential.DekNonce, credential.WrappedDek)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt DEK: %w", err)
	}

	// Decrypt token with DEK
	token, err := s.decryptWithGCM(dek, credential.TokenNonce, credential.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %w", err)
	}

	return string(token), nil
}

type RefreshTokenResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // seconds until expiration
}

// RefreshGitHubToken refreshes a GitHub OAuth access token using the refresh token
func (s *Service) RefreshGitHubToken(ctx context.Context, refreshToken string) (*RefreshTokenResult, error) {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_ID environment variable is not set")
	}

	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if clientSecret == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_SECRET environment variable is not set")
	}

	// Prepare form data
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Make request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.Unmarshal(body, &errorResp); err == nil {
			if errorResp.Error == "invalid_grant" || errorResp.Error == "invalid_request" {
				return nil, ErrRefreshTokenExpired
			}
			return nil, fmt.Errorf("GitHub API error: %s - %s", errorResp.Error, errorResp.ErrorDescription)
		}
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("GitHub API did not return an access token")
	}

	// Default expires_in to 8 hours if not provided
	expiresIn := tokenResp.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 8 * 60 * 60 // 8 hours in seconds
	}

	return &RefreshTokenResult{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

// encryptWithGCM encrypts plaintext using AES-GCM with the given key and nonce
func (s *Service) encryptWithGCM(key, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptWithGCM decrypts ciphertext using AES-GCM with the given key and nonce
func (s *Service) decryptWithGCM(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
