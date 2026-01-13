package credentials

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/coding-cave-dev/nimbul/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
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
