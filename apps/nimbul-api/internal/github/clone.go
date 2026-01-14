package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneRepository clones a GitHub repository to the specified destination path
// Uses installation token for authentication (works for both public and private repos)
func CloneRepository(ctx context.Context, installationID int64, owner, repo, ref, destPath string) error {
	// Get installation token
	appAuth, err := NewAppAuth(installationID)
	if err != nil {
		return fmt.Errorf("failed to create app auth: %w", err)
	}

	token, err := appAuth.GetInstallationToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get installation token: %w", err)
	}

	// Format clone URL with token authentication
	cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", token, owner, repo)

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Clone repository (shallow clone for faster operation)
	// First clone, then checkout the specific ref
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", cloneURL, destPath)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Checkout specific ref if provided
	if ref != "" {
		// Normalize ref - remove refs/heads/ or refs/tags/ prefix if present
		checkoutRef := ref
		if strings.HasPrefix(ref, "refs/heads/") {
			checkoutRef = strings.TrimPrefix(ref, "refs/heads/")
		} else if strings.HasPrefix(ref, "refs/tags/") {
			checkoutRef = strings.TrimPrefix(ref, "refs/tags/")
		}

		// Fetch the specific ref if it's not the default branch
		fetchCmd := exec.CommandContext(ctx, "git", "-C", destPath, "fetch", "origin", checkoutRef)
		fetchCmd.Stdout = os.Stdout
		fetchCmd.Stderr = os.Stderr
		if err := fetchCmd.Run(); err != nil {
			// Try fetching by SHA if branch/tag fetch fails
			fetchCmd = exec.CommandContext(ctx, "git", "-C", destPath, "fetch", "origin", ref)
			fetchCmd.Stdout = os.Stdout
			fetchCmd.Stderr = os.Stderr
			if err := fetchCmd.Run(); err != nil {
				return fmt.Errorf("failed to fetch ref %s: %w", ref, err)
			}
			checkoutRef = ref
		}

		// Checkout the ref
		checkoutCmd := exec.CommandContext(ctx, "git", "-C", destPath, "checkout", checkoutRef)
		checkoutCmd.Stdout = os.Stdout
		checkoutCmd.Stderr = os.Stderr
		if err := checkoutCmd.Run(); err != nil {
			return fmt.Errorf("failed to checkout ref %s: %w", checkoutRef, err)
		}
	}

	return nil
}

// CleanupRepository removes the cloned repository directory
func CleanupRepository(destPath string) error {
	return os.RemoveAll(destPath)
}

// GetDockerfileParentDir returns the parent directory of the Dockerfile path
func GetDockerfileParentDir(dockerfilePath string) string {
	return filepath.Dir(dockerfilePath)
}
