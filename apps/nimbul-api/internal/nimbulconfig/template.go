package nimbulconfig

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// TemplateContext holds variables available for template rendering
type TemplateContext struct {
	COMMIT_SHA   string
	COMMIT_SHORT string
	BRANCH       string
	REPO         string
	TIMESTAMP    string
	BUILD_TAGS   []string // Available for deploy steps
}

// NewTemplateContext creates a new template context with the provided values
func NewTemplateContext(commitSHA, branch, repo string) *TemplateContext {
	commitShort := commitSHA
	if len(commitSHA) > 12 {
		commitShort = commitSHA[:12]
	}

	return &TemplateContext{
		COMMIT_SHA:   commitSHA,
		COMMIT_SHORT: commitShort,
		BRANCH:       branch,
		REPO:         repo,
		TIMESTAMP:    strconv.FormatInt(time.Now().Unix(), 10),
		BUILD_TAGS:   []string{},
	}
}

// RenderString renders a template string with the given context
func RenderString(tmpl string, ctx *TemplateContext) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		// No template syntax, return as-is
		return tmpl, nil
	}

	// Transform BUILD_TAG[n] syntax to use custom function
	// {{ .BUILD_TAG[1] }} -> {{ tag 1 }}
	tmpl = transformBuildTagSyntax(tmpl)

	// Create template with custom functions
	t, err := template.New("nimbul").Funcs(template.FuncMap{
		"tag": func(index int) (string, error) {
			if index < 0 || index >= len(ctx.BUILD_TAGS) {
				return "", fmt.Errorf("BUILD_TAG index %d out of range (available: %d tags)", index, len(ctx.BUILD_TAGS))
			}
			return ctx.BUILD_TAGS[index], nil
		},
	}).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// transformBuildTagSyntax transforms {{ .BUILD_TAG[n] }} to {{ tag n }}
func transformBuildTagSyntax(tmpl string) string {
	// Simple regex-like replacement for {{ .BUILD_TAG[n] }}
	// This handles the common case, more complex cases would need proper parsing
	result := tmpl
	for {
		start := strings.Index(result, "{{ .BUILD_TAG[")
		if start == -1 {
			break
		}

		// Find the closing bracket
		bracketStart := start + len("{{ .BUILD_TAG[")
		bracketEnd := strings.Index(result[bracketStart:], "]")
		if bracketEnd == -1 {
			break
		}
		bracketEnd += bracketStart

		// Find the closing }}
		closeStart := bracketEnd + 1
		closeEnd := strings.Index(result[closeStart:], "}}")
		if closeEnd == -1 {
			break
		}
		closeEnd += closeStart + 2

		// Extract index
		indexStr := result[bracketStart:bracketEnd]
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			// Invalid index, skip this replacement
			result = result[closeEnd:]
			continue
		}

		// Replace {{ .BUILD_TAG[n] }} with {{ tag n }}
		before := result[:start]
		after := result[closeEnd:]
		result = before + fmt.Sprintf("{{ tag %d }}", index) + after
	}

	return result
}

// RenderConfig renders all template strings in a config and returns a new config
func RenderConfig(config *NimbulConfig, ctx *TemplateContext) (*NimbulConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// Create a deep copy to avoid modifying the original
	rendered := &NimbulConfig{
		Version: config.Version,
		Build:   make([]BuildConfig, len(config.Build)),
		Deploy:  make([]DeployConfig, len(config.Deploy)),
	}

	// Render build configs first
	for i, build := range config.Build {
		renderedBuild := BuildConfig{
			Name:       build.Name,
			Dockerfile: build.Dockerfile,
			Context:    build.Context,
			Tags:       make([]string, len(build.Tags)),
		}

		// Render tags
		for j, tag := range build.Tags {
			renderedTag, err := RenderString(tag, ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to render build[%d].tags[%d]: %w", i, j, err)
			}
			renderedBuild.Tags[j] = renderedTag
		}

		rendered.Build[i] = renderedBuild
	}

	// Render deploy configs
	for i, deploy := range config.Deploy {
		// Find the linked build to get its tags
		var linkedBuild *BuildConfig
		for _, build := range rendered.Build {
			if build.Name == deploy.BuildID {
				linkedBuild = &build
				break
			}
		}
		if linkedBuild == nil {
			return nil, fmt.Errorf("deploy[%d]: buildId '%s' not found", i, deploy.BuildID)
		}

		// Create context with BUILD_TAGS from linked build
		deployCtx := *ctx
		deployCtx.BUILD_TAGS = linkedBuild.Tags

		renderedDeploy := DeployConfig{
			Name:      deploy.Name,
			BuildID:   deploy.BuildID,
			Manifests: make([]ManifestConfig, len(deploy.Manifests)),
		}

		// Render manifests
		for j, manifest := range deploy.Manifests {
			renderedManifest := ManifestConfig{
				Path:      manifest.Path,
				Overrides: make([]OverrideConfig, len(manifest.Overrides)),
			}

			// Render overrides
			for k, override := range manifest.Overrides {
				renderedOverride := OverrideConfig{
					Path:  override.Path,
					Match: override.Match,
				}

				// Render override value
				renderedValue, err := RenderString(override.Value, &deployCtx)
				if err != nil {
					return nil, fmt.Errorf("failed to render deploy[%d].manifest[%d].override[%d].value: %w", i, j, k, err)
				}
				renderedOverride.Value = renderedValue

				renderedManifest.Overrides[k] = renderedOverride
			}

			renderedDeploy.Manifests[j] = renderedManifest
		}

		rendered.Deploy[i] = renderedDeploy
	}

	return rendered, nil
}
