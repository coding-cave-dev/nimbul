package configs

import (
	"context"
	"fmt"

	"github.com/coding-cave-dev/nimbul/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/oklog/ulid/v2"
)

type Service struct {
	queries *db.Queries
}

func NewService(queries *db.Queries) *Service {
	return &Service{
		queries: queries,
	}
}

type CreateConfigParams struct {
	OwnerID        string
	Provider       string
	RepoOwner      string
	RepoName       string
	RepoFullName   string
	RepoCloneURL   string
	DockerfilePath string
	WebhookSecret  string
}

type CreateConfigResult struct {
	ConfigID string
}

type Config struct {
	ID             string
	OwnerID        string
	Provider       string
	RepoOwner      string
	RepoName       string
	RepoFullName   string
	RepoCloneURL   string
	DockerfilePath string
	WebhookSecret  string
	WebhookID      *int64
	CreatedAt      pgtype.Timestamptz
	UpdatedAt      pgtype.Timestamptz
}

// CreateConfig creates a new repo configuration
func (s *Service) CreateConfig(ctx context.Context, params CreateConfigParams) (*CreateConfigResult, error) {
	// Generate ULID for config ID
	configID := ulid.Make().String()

	// Create config in database
	config, err := s.queries.CreateConfig(ctx, db.CreateConfigParams{
		ID:             configID,
		OwnerID:        params.OwnerID,
		Provider:       params.Provider,
		RepoOwner:      params.RepoOwner,
		RepoName:       params.RepoName,
		RepoFullName:   params.RepoFullName,
		RepoCloneUrl:   params.RepoCloneURL,
		DockerfilePath: params.DockerfilePath,
		WebhookSecret:  params.WebhookSecret,
		WebhookID:      pgtype.Int8{Valid: false}, // Will be set after webhook creation
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	return &CreateConfigResult{
		ConfigID: config.ID,
	}, nil
}

// GetConfigByID retrieves a config by its ID
func (s *Service) GetConfigByID(ctx context.Context, id string) (*Config, error) {
	config, err := s.queries.GetConfigByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	return dbConfigToConfig(config), nil
}

// GetConfigByWebhookID retrieves a config by its webhook ID
func (s *Service) GetConfigByWebhookID(ctx context.Context, webhookID int64) (*Config, error) {
	config, err := s.queries.GetConfigByWebhookID(ctx, pgtype.Int8{Int64: webhookID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	return dbConfigToConfig(config), nil
}

// GetConfigsByOwnerID retrieves all configs for a user
func (s *Service) GetConfigsByOwnerID(ctx context.Context, ownerID string) ([]Config, error) {
	configs, err := s.queries.GetConfigsByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get configs: %w", err)
	}

	result := make([]Config, len(configs))
	for i, c := range configs {
		result[i] = *dbConfigToConfig(c)
	}

	return result, nil
}

// UpdateWebhookID updates the webhook ID for a config
func (s *Service) UpdateWebhookID(ctx context.Context, configID string, webhookID int64) error {
	_, err := s.queries.UpdateConfigWebhookID(ctx, db.UpdateConfigWebhookIDParams{
		ID:        configID,
		WebhookID: pgtype.Int8{Int64: webhookID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update webhook ID: %w", err)
	}

	return nil
}

// dbConfigToConfig converts a db.RepoConfig to a configs.Config
func dbConfigToConfig(dbConfig db.RepoConfig) *Config {
	var webhookID *int64
	if dbConfig.WebhookID.Valid {
		webhookID = &dbConfig.WebhookID.Int64
	}

	return &Config{
		ID:             dbConfig.ID,
		OwnerID:        dbConfig.OwnerID,
		Provider:       dbConfig.Provider,
		RepoOwner:      dbConfig.RepoOwner,
		RepoName:       dbConfig.RepoName,
		RepoFullName:   dbConfig.RepoFullName,
		RepoCloneURL:   dbConfig.RepoCloneUrl,
		DockerfilePath: dbConfig.DockerfilePath,
		WebhookSecret:  dbConfig.WebhookSecret,
		WebhookID:      webhookID,
		CreatedAt:      dbConfig.CreatedAt,
		UpdatedAt:      dbConfig.UpdatedAt,
	}
}
