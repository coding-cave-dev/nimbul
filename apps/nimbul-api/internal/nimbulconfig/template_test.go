package nimbulconfig

import (
	"strings"
	"testing"
)

func TestRenderString(t *testing.T) {
	ctx := NewTemplateContext("abc123def456789", "main", "owner/repo")

	tests := []struct {
		name     string
		template string
		expected string
		wantErr  bool
	}{
		{
			name:     "no template",
			template: "simple string",
			expected: "simple string",
			wantErr:  false,
		},
		{
			name:     "COMMIT_SHA",
			template: "{{ .COMMIT_SHA }}",
			expected: "abc123def456789",
			wantErr:  false,
		},
		{
			name:     "COMMIT_SHORT",
			template: "{{ .COMMIT_SHORT }}",
			expected: "abc123def456",
			wantErr:  false,
		},
		{
			name:     "BRANCH",
			template: "{{ .BRANCH }}",
			expected: "main",
			wantErr:  false,
		},
		{
			name:     "REPO",
			template: "{{ .REPO }}",
			expected: "owner/repo",
			wantErr:  false,
		},
		{
			name:     "TIMESTAMP",
			template: "{{ .TIMESTAMP }}",
			expected: "", // Will be set dynamically
			wantErr:  false,
		},
		{
			name:     "multiple variables",
			template: "{{ .REPO }}:{{ .COMMIT_SHORT }}",
			expected: "owner/repo:abc123def456",
			wantErr:  false,
		},
		{
			name:     "BUILD_TAG with index",
			template: "{{ .BUILD_TAG[0] }}",
			expected: "test:latest",
			wantErr:  false,
		},
		{
			name:     "BUILD_TAG invalid index",
			template: "{{ .BUILD_TAG[10] }}",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set BUILD_TAGS for tests that need it
			if tt.name == "BUILD_TAG with index" || tt.name == "BUILD_TAG invalid index" {
				ctx.BUILD_TAGS = []string{"test:latest", "test:v1.0"}
			} else {
				ctx.BUILD_TAGS = []string{}
			}

			result, err := RenderString(tt.template, ctx)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// For TIMESTAMP, just check it's not empty
			if tt.name == "TIMESTAMP" {
				if result == "" {
					t.Errorf("Expected non-empty timestamp, got empty")
				}
				return
			}

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestRenderConfig(t *testing.T) {
	ctx := NewTemplateContext("abc123def456789", "main", "owner/repo")

	config := &NimbulConfig{
		Version: "1",
		Build: []BuildConfig{
			{
				Name:       "build-1",
				Dockerfile: "Dockerfile",
				Context:    ".",
				Tags: []string{
					"image:latest",
					"image:{{ .COMMIT_SHORT }}",
				},
			},
		},
		Deploy: []DeployConfig{
			{
				Name:    "deploy-1",
				BuildID: "build-1",
				Manifests: []ManifestConfig{
					{
						Path: "k8s/deploy.yaml",
						Overrides: []OverrideConfig{
							{
								Path:  "spec.template.spec.containers[0].image",
								Value: "{{ .BUILD_TAG[1] }}",
							},
						},
					},
				},
			},
		},
	}

	rendered, err := RenderConfig(config, ctx)
	if err != nil {
		t.Fatalf("Failed to render config: %v", err)
	}

	// Check build tags were rendered
	if len(rendered.Build[0].Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(rendered.Build[0].Tags))
	}
	if rendered.Build[0].Tags[0] != "image:latest" {
		t.Errorf("Expected first tag 'image:latest', got '%s'", rendered.Build[0].Tags[0])
	}
	if rendered.Build[0].Tags[1] != "image:abc123def456" {
		t.Errorf("Expected second tag 'image:abc123def456', got '%s'", rendered.Build[0].Tags[1])
	}

	// Check deploy override was rendered with BUILD_TAG
	if len(rendered.Deploy[0].Manifests[0].Overrides) != 1 {
		t.Errorf("Expected 1 override, got %d", len(rendered.Deploy[0].Manifests[0].Overrides))
	}
	expectedValue := "image:abc123def456"
	if rendered.Deploy[0].Manifests[0].Overrides[0].Value != expectedValue {
		t.Errorf("Expected override value '%s', got '%s'", expectedValue, rendered.Deploy[0].Manifests[0].Overrides[0].Value)
	}
}

func TestRenderConfigInvalidBuildID(t *testing.T) {
	ctx := NewTemplateContext("abc123", "main", "owner/repo")

	config := &NimbulConfig{
		Version: "1",
		Build: []BuildConfig{
			{Name: "build-1", Dockerfile: "Dockerfile", Tags: []string{"tag1"}},
		},
		Deploy: []DeployConfig{
			{
				Name:    "deploy-1",
				BuildID: "build-nonexistent",
				Manifests: []ManifestConfig{
					{Path: "k8s/deploy.yaml"},
				},
			},
		},
	}

	_, err := RenderConfig(config, ctx)
	if err == nil {
		t.Error("Expected error for invalid buildId, got none")
	}
	if err != nil && !contains(err.Error(), "not found") {
		t.Errorf("Expected error about buildId not found, got: %v", err)
	}
}

func TestTransformBuildTagSyntax(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple index",
			input:    "{{ .BUILD_TAG[0] }}",
			expected: "{{ tag 0 }}",
		},
		{
			name:     "index 1",
			input:    "{{ .BUILD_TAG[1] }}",
			expected: "{{ tag 1 }}",
		},
		{
			name:     "with text",
			input:    "image:{{ .BUILD_TAG[1] }}",
			expected: "image:{{ tag 1 }}",
		},
		{
			name:     "multiple tags",
			input:    "{{ .BUILD_TAG[0] }} and {{ .BUILD_TAG[1] }}",
			expected: "{{ tag 0 }} and {{ tag 1 }}",
		},
		{
			name:     "no template",
			input:    "simple string",
			expected: "simple string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformBuildTagSyntax(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || strings.Contains(s, substr))
}
