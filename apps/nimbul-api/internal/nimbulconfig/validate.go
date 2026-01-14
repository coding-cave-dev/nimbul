package nimbulconfig

import (
	"fmt"
	"strings"
)

// Validate validates a NimbulConfig and returns an error if invalid
func Validate(config *NimbulConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// 1. Check version == "1"
	if config.Version != "1" {
		return fmt.Errorf("unsupported version: %s (expected '1')", config.Version)
	}

	// 2. Validate builds
	buildNames := make(map[string]bool)
	for i, build := range config.Build {
		if err := validateBuild(build, i, buildNames); err != nil {
			return err
		}
		buildNames[build.Name] = true
	}

	// 3. Validate deploys
	deployNames := make(map[string]bool)
	for i, deploy := range config.Deploy {
		if err := validateDeploy(deploy, i, deployNames, buildNames); err != nil {
			return err
		}
		deployNames[deploy.Name] = true
	}

	return nil
}

// validateBuild validates a single BuildConfig
func validateBuild(build BuildConfig, index int, buildNames map[string]bool) error {
	// name is non-empty and unique
	if build.Name == "" {
		return fmt.Errorf("build[%d]: name is required", index)
	}
	if buildNames[build.Name] {
		return fmt.Errorf("build[%d]: duplicate build name '%s'", index, build.Name)
	}

	// dockerfile is non-empty
	if build.Dockerfile == "" {
		return fmt.Errorf("build[%d]: dockerfile is required", index)
	}

	// context defaults to "." if empty (handled during processing, not validation)
	// tags has at least one entry
	if len(build.Tags) == 0 {
		return fmt.Errorf("build[%d]: at least one tag is required", index)
	}

	return nil
}

// validateDeploy validates a single DeployConfig
func validateDeploy(deploy DeployConfig, index int, deployNames map[string]bool, buildNames map[string]bool) error {
	// name is non-empty and unique
	if deploy.Name == "" {
		return fmt.Errorf("deploy[%d]: name is required", index)
	}
	if deployNames[deploy.Name] {
		return fmt.Errorf("deploy[%d]: duplicate deploy name '%s'", index, deploy.Name)
	}

	// buildId references an existing build name
	if deploy.BuildID == "" {
		return fmt.Errorf("deploy[%d]: buildId is required", index)
	}
	if !buildNames[deploy.BuildID] {
		return fmt.Errorf("deploy[%d]: buildId '%s' does not reference an existing build", index, deploy.BuildID)
	}

	// manifests is non-empty
	if len(deploy.Manifests) == 0 {
		return fmt.Errorf("deploy[%d]: at least one manifest is required", index)
	}

	// Validate each manifest
	for i, manifest := range deploy.Manifests {
		if err := validateManifest(manifest, i); err != nil {
			return fmt.Errorf("deploy[%d].manifest[%d]: %w", index, i, err)
		}
	}

	return nil
}

// validateManifest validates a single ManifestConfig
func validateManifest(manifest ManifestConfig, index int) error {
	// path is non-empty
	if manifest.Path == "" {
		return fmt.Errorf("path is required")
	}

	// Validate each override
	for i, override := range manifest.Overrides {
		if err := validateOverride(override, i); err != nil {
			return fmt.Errorf("override[%d]: %w", i, err)
		}
	}

	return nil
}

// validateOverride validates a single OverrideConfig
func validateOverride(override OverrideConfig, index int) error {
	// path is non-empty (JSONPath)
	if override.Path == "" {
		return fmt.Errorf("path is required")
	}

	// value is non-empty
	if override.Value == "" {
		return fmt.Errorf("value is required")
	}

	// Validate match config if provided
	if override.Match.Kind != "" {
		// Kind should be a valid Kubernetes resource type
		validKinds := []string{
			"Deployment", "Service", "ConfigMap", "Secret", "Ingress",
			"StatefulSet", "DaemonSet", "Job", "CronJob", "PersistentVolumeClaim",
		}
		kindValid := false
		for _, validKind := range validKinds {
			if strings.EqualFold(override.Match.Kind, validKind) {
				kindValid = true
				break
			}
		}
		if !kindValid {
			return fmt.Errorf("match.kind '%s' is not a recognized Kubernetes resource type", override.Match.Kind)
		}
	}

	return nil
}
