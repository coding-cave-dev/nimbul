package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v81/github"
)

// FileExists checks if a file exists at the specified path in a GitHub repository
// If ref is provided, checks at that specific ref (branch, tag, or commit SHA)
func FileExists(ctx context.Context, client *github.Client, owner, repo, path, ref string) (bool, error) {
	opts := &github.RepositoryContentGetOptions{}
	if ref != "" {
		opts.Ref = ref
	}

	_, _, resp, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
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
