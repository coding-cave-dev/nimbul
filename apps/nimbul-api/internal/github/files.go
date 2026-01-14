package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v81/github"
)

// FileExists checks if a file exists at the specified path in a GitHub repository
func FileExists(ctx context.Context, client *github.Client, owner, repo, path string) (bool, error) {
	_, _, resp, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		// Check if error is 404 (file not found)
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	// File exists
	return true, nil
}
