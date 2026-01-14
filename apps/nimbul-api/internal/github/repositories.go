package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v81/github"
)

// Repository represents a GitHub repository
type Repository struct {
	Owner    string
	Name     string
	FullName string
	CloneURL string
}

// ListRepositories lists all repositories accessible to the authenticated user
func ListRepositories(ctx context.Context, client *github.Client, perPage int) ([]Repository, error) {
	opts := &github.RepositoryListOptions{
		Type:        "all",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: perPage},
	}

	repos, _, err := client.Repositories.List(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	result := make([]Repository, 0, len(repos))
	for _, repo := range repos {
		result = append(result, Repository{
			Owner:    repo.GetOwner().GetLogin(),
			Name:     repo.GetName(),
			FullName: repo.GetFullName(),
			CloneURL: repo.GetCloneURL(),
		})
	}

	return result, nil
}

// ListRepositoriesByAuthenticatedUser lists repositories for the authenticated user
func ListRepositoriesByAuthenticatedUser(ctx context.Context, client *github.Client, perPage int) ([]string, error) {
	opts := &github.RepositoryListByAuthenticatedUserOptions{
		Type:        "all",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: perPage},
	}

	repos, _, err := client.Repositories.ListByAuthenticatedUser(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	result := make([]string, 0, len(repos))
	for _, repo := range repos {
		result = append(result, repo.GetFullName())
	}

	return result, nil
}
