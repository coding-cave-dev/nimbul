package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v81/github"
)

// CreateWebhook creates a webhook for a repository using installation authentication
func CreateWebhook(ctx context.Context, client *github.Client, owner, repo, webhookURL, secret string) (int64, error) {
	hook := &github.Hook{
		Name:   github.String("web"),
		Active: github.Bool(true),
		Events: []string{"push"},
		Config: &github.HookConfig{
			URL:         github.String(webhookURL),
			ContentType: github.String("json"),
			Secret:      github.String(secret),
		},
	}

	createdHook, _, err := client.Repositories.CreateHook(ctx, owner, repo, hook)
	if err != nil {
		return 0, fmt.Errorf("failed to create webhook: %w", err)
	}

	return createdHook.GetID(), nil
}
