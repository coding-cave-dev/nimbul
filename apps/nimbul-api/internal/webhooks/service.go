package webhooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coding-cave-dev/nimbul/internal/configs"
	"github.com/coding-cave-dev/nimbul/internal/github"
	ghub "github.com/google/go-github/v81/github"
)

type Service struct {
	configsService *configs.Service
}

func NewService(configsService *configs.Service) *Service {
	return &Service{
		configsService: configsService,
	}
}

// HandlePushEvent processes a GitHub push event
func (s *Service) HandlePushEvent(ctx context.Context, config *configs.Config, pushEvent *ghub.PushEvent) error {
	// 1. Verify the event repo matches the config repo
	if pushEvent.Repo.GetFullName() != config.RepoFullName {
		return fmt.Errorf("repository mismatch: expected %s, got %s", config.RepoFullName, pushEvent.Repo.GetFullName())
	}

	// Get the ref from the push event (e.g., "refs/heads/main" or commit SHA)
	ref := pushEvent.GetRef()
	if ref == "" {
		// If no ref, use the head commit SHA
		ref = pushEvent.GetHeadCommit().GetSHA()
	}

	// Get installation ID for the repository
	installationID, err := github.GetInstallationIDByRepository(ctx, config.RepoOwner, config.RepoName)
	if err != nil {
		return fmt.Errorf("failed to get installation ID: %w", err)
	}

	// 2. Verify Dockerfile path exists at the specific ref
	appAuth, err := github.NewAppAuth(installationID)
	if err != nil {
		return fmt.Errorf("failed to create app auth: %w", err)
	}

	installClient, err := appAuth.GetInstallationClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get installation client: %w", err)
	}

	// Use commit SHA for checking file existence (more reliable than ref)
	commitSHA := pushEvent.GetHeadCommit().GetSHA()
	if commitSHA == "" {
		return fmt.Errorf("push event missing head commit SHA")
	}

	exists, err := github.FileExists(ctx, installClient, config.RepoOwner, config.RepoName, config.DockerfilePath, commitSHA)
	if err != nil {
		return fmt.Errorf("failed to verify Dockerfile existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("Dockerfile not found at path %s in repository %s at ref %s", config.DockerfilePath, config.RepoFullName, ref)
	}

	// 3. Clone repository to temp directory
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("nimbul-build-%s-*", config.ID))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := github.CleanupRepository(tempDir); err != nil {
			fmt.Printf("Warning: Failed to cleanup temp directory %s: %v\n", tempDir, err)
		}
	}()

	// Clone repository
	if err := github.CloneRepository(ctx, installationID, config.RepoOwner, config.RepoName, ref, tempDir); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// 4. Build Docker image
	// Image name: repo full name (e.g., "owner/repo-name")
	imageName := config.RepoFullName
	// Tag: use the ref (normalized)
	tag := normalizeRefForTag(ref, commitSHA)

	// Get full path to Dockerfile in cloned repo
	dockerfilePath := filepath.Join(tempDir, config.DockerfilePath)

	// Get parent directory of Dockerfile
	buildContext := github.GetDockerfileParentDir(dockerfilePath)

	// Build Docker image
	if err := s.buildDockerImage(ctx, dockerfilePath, buildContext, imageName, tag); err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}

	// 5. Print image name and tag
	fmt.Printf("Successfully built Docker image: %s:%s\n", imageName, tag)

	return nil
}

// buildDockerImage builds a Docker image using the docker command
func (s *Service) buildDockerImage(ctx context.Context, dockerfilePath, buildContext, imageName, tag string) error {
	// Run docker build command
	// docker build -t {imageName}:{tag} -f {dockerfilePath} {buildContext}
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", fmt.Sprintf("%s:%s", imageName, tag), "-f", dockerfilePath, buildContext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	return nil
}

// normalizeRefForTag normalizes a git ref for use as a Docker tag
// Removes refs/heads/ and refs/tags/ prefixes, and uses commit SHA if ref is empty
func normalizeRefForTag(ref, commitSHA string) string {
	if ref == "" {
		// Use first 12 characters of commit SHA as tag
		if len(commitSHA) > 12 {
			return commitSHA[:12]
		}
		return commitSHA
	}

	// Remove refs/heads/ prefix
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}

	// Remove refs/tags/ prefix
	if strings.HasPrefix(ref, "refs/tags/") {
		return strings.TrimPrefix(ref, "refs/tags/")
	}

	// If it's a commit SHA, use first 12 characters
	if len(ref) == 40 && isHexString(ref) {
		return ref[:12]
	}

	return ref
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
