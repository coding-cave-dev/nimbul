package webhooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coding-cave-dev/nimbul/internal/buildkit"
	"github.com/coding-cave-dev/nimbul/internal/configs"
	"github.com/coding-cave-dev/nimbul/internal/github"
	"github.com/coding-cave-dev/nimbul/internal/k8s"
	"github.com/coding-cave-dev/nimbul/internal/nimbulconfig"
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

	// Get commit SHA
	commitSHA := pushEvent.GetHeadCommit().GetID()
	if commitSHA == "" {
		return fmt.Errorf("push event missing head commit SHA")
	}

	// Get installation ID for the repository
	installationID, err := github.GetInstallationIDByRepository(ctx, config.RepoOwner, config.RepoName)
	if err != nil {
		return fmt.Errorf("failed to get installation ID: %w", err)
	}

	// 2. Clone repository to temp directory
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

	// 3. Fetch and parse nimbul.yaml from cloned repo
	nimbulConfigPath := filepath.Join(tempDir, "nimbul.yaml")
	nimbulConfig, err := nimbulconfig.ParseFile(nimbulConfigPath)
	if err != nil {
		return fmt.Errorf("failed to parse nimbul.yaml: %w", err)
	}

	// 4. Validate config
	if err := nimbulconfig.Validate(nimbulConfig); err != nil {
		return fmt.Errorf("invalid nimbul.yaml: %w", err)
	}

	// 5. Create template context
	branch := extractBranch(ref)
	templateCtx := nimbulconfig.NewTemplateContext(commitSHA, branch, config.RepoFullName)

	// 6. Render config with template variables
	renderedConfig, err := nimbulconfig.RenderConfig(nimbulConfig, templateCtx)
	if err != nil {
		return fmt.Errorf("failed to render nimbul.yaml templates: %w", err)
	}

	// 7. Build Docker images for each build config using BuildKit
	builder := buildkit.NewFromEnv()
	for _, build := range renderedConfig.Build {
		// Get full paths relative to cloned repo
		buildContext := filepath.Join(tempDir, build.Context)
		dockerfileFullPath := filepath.Join(tempDir, build.Dockerfile)

		// Calculate Dockerfile path relative to context
		// Both build.Context and build.Dockerfile are relative to repo root
		dockerfileRelPath, err := filepath.Rel(buildContext, dockerfileFullPath)
		if err != nil {
			return fmt.Errorf("failed to calculate Dockerfile path relative to context: %w", err)
		}

		// Build image with each tag
		for _, tag := range build.Tags {
			// Parse image:tag format
			imageName, tagValue := parseImageTag(tag)
			imageRef := fmt.Sprintf("%s:%s", imageName, tagValue)

			buildReq := buildkit.BuildRequest{
				ContextDir: buildContext,
				Dockerfile: dockerfileRelPath,
				ImageRef:   imageRef,
			}

			if err := builder.BuildAndPush(ctx, buildReq); err != nil {
				return fmt.Errorf("failed to build Docker image %s:%s: %w", imageName, tagValue, err)
			}
			fmt.Printf("Successfully built Docker image: %s:%s\n", imageName, tagValue)
		}
	}

	// 8. Process deploy stage for each deploy config
	for _, deploy := range renderedConfig.Deploy {
		for _, manifest := range deploy.Manifests {
			// Get full path to manifest file in cloned repo
			manifestPath := filepath.Join(tempDir, manifest.Path)

			// Parse manifest file
			docs, err := nimbulconfig.ParseManifestFile(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to parse manifest file %s: %w", manifest.Path, err)
			}

			// Apply overrides
			if err := nimbulconfig.ApplyOverrides(docs, manifest.Overrides); err != nil {
				return fmt.Errorf("failed to apply overrides to manifest %s: %w", manifest.Path, err)
			}

			// Serialize manifest
			serialized, err := nimbulconfig.SerializeManifests(docs)
			if err != nil {
				return fmt.Errorf("failed to serialize manifest %s: %w", manifest.Path, err)
			}

			// Apply manifest to cluster
			fmt.Printf("\n=== Applying Manifest: %s ===\n", manifest.Path)
			if err := k8s.ApplyManifests(ctx, []byte(serialized)); err != nil {
				return fmt.Errorf("failed to apply manifest %s: %w", manifest.Path, err)
			}
			fmt.Printf("✓ Successfully applied manifest: %s\n", manifest.Path)
		}
	}

	// 9. Test Kubernetes client connectivity
	fmt.Println("\n=== Testing Kubernetes Client ===")
	k8sClient, err := k8s.GetClient()
	if err != nil {
		return fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	// Get server version to verify connectivity
	version, err := k8sClient.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes server version: %w", err)
	}

	fmt.Printf("✓ Successfully connected to Kubernetes cluster\n")
	fmt.Printf("  Server Version: %s\n", version.String())
	fmt.Println("=== Kubernetes Client Test Complete ===")

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

// extractBranch extracts the branch name from a git ref
// Examples:
//   - "refs/heads/main" -> "main"
//   - "refs/tags/v1.0.0" -> "v1.0.0"
//   - "abc123..." (commit SHA) -> ""
func extractBranch(ref string) string {
	if ref == "" {
		return ""
	}

	// Remove refs/heads/ prefix
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}

	// Remove refs/tags/ prefix
	if strings.HasPrefix(ref, "refs/tags/") {
		return strings.TrimPrefix(ref, "refs/tags/")
	}

	// If it's a commit SHA (40 hex chars), return empty string
	if len(ref) == 40 && isHexString(ref) {
		return ""
	}

	// Otherwise return as-is (might be a branch name without prefix)
	return ref
}

// parseImageTag parses an image:tag string and returns image name and tag separately
// Examples:
//   - "my-image:latest" -> ("my-image", "latest")
//   - "my-image:v1.0.0" -> ("my-image", "v1.0.0")
//   - "registry.io/my-image:tag" -> ("registry.io/my-image", "tag")
func parseImageTag(imageTag string) (imageName, tag string) {
	parts := strings.Split(imageTag, ":")
	if len(parts) == 1 {
		// No tag specified, use "latest"
		return parts[0], "latest"
	}
	// Last part is the tag, everything before is the image name
	tag = parts[len(parts)-1]
	imageName = strings.Join(parts[:len(parts)-1], ":")
	return imageName, tag
}
