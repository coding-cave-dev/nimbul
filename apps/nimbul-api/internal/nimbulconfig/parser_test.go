package nimbulconfig

import (
	"strings"
	"testing"
)

func TestParseFile(t *testing.T) {
	// Skip if file doesn't exist (e.g., in CI)
	// This test is mainly for local development
	config, err := ParseFile("../../../nimbul.yaml")
	if err != nil {
		t.Skipf("Skipping test - nimbul.yaml not found: %v", err)
		return
	}

	if config.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", config.Version)
	}

	if len(config.Build) == 0 {
		t.Error("Expected at least one build config")
	}

	if len(config.Deploy) == 0 {
		t.Error("Expected at least one deploy config")
	}
}

func TestParse(t *testing.T) {
	yamlContent := `
version: "1"
build:
  - name: test-build
    dockerfile: Dockerfile
    context: .
    tags:
      - test:latest
deploy:
  - name: test-deploy
    buildId: test-build
    manifests:
      - path: k8s/deployment.yaml
        overrides:
          - path: spec.template.spec.containers[0].image
            match:
              kind: Deployment
            value: test:latest
`

	reader := strings.NewReader(yamlContent)
	config, err := Parse(reader)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if config.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", config.Version)
	}

	if len(config.Build) != 1 {
		t.Errorf("Expected 1 build, got %d", len(config.Build))
	}

	if config.Build[0].Name != "test-build" {
		t.Errorf("Expected build name 'test-build', got '%s'", config.Build[0].Name)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *NimbulConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &NimbulConfig{
				Version: "1",
				Build: []BuildConfig{
					{
						Name:       "build-1",
						Dockerfile: "Dockerfile",
						Context:    ".",
						Tags:       []string{"image:tag"},
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
										Value: "image:tag",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			config: &NimbulConfig{
				Version: "2",
				Build:   []BuildConfig{},
				Deploy:  []DeployConfig{},
			},
			wantErr: true,
			errMsg:  "unsupported version",
		},
		{
			name: "missing build name",
			config: &NimbulConfig{
				Version: "1",
				Build: []BuildConfig{
					{
						Dockerfile: "Dockerfile",
						Tags:       []string{"image:tag"},
					},
				},
				Deploy: []DeployConfig{},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "duplicate build name",
			config: &NimbulConfig{
				Version: "1",
				Build: []BuildConfig{
					{Name: "build-1", Dockerfile: "Dockerfile", Tags: []string{"tag1"}},
					{Name: "build-1", Dockerfile: "Dockerfile", Tags: []string{"tag2"}},
				},
				Deploy: []DeployConfig{},
			},
			wantErr: true,
			errMsg:  "duplicate build name",
		},
		{
			name: "missing dockerfile",
			config: &NimbulConfig{
				Version: "1",
				Build: []BuildConfig{
					{
						Name: "build-1",
						Tags: []string{"image:tag"},
					},
				},
				Deploy: []DeployConfig{},
			},
			wantErr: true,
			errMsg:  "dockerfile is required",
		},
		{
			name: "missing tags",
			config: &NimbulConfig{
				Version: "1",
				Build: []BuildConfig{
					{
						Name:       "build-1",
						Dockerfile: "Dockerfile",
						Tags:       []string{},
					},
				},
				Deploy: []DeployConfig{},
			},
			wantErr: true,
			errMsg:  "at least one tag is required",
		},
		{
			name: "invalid buildId reference",
			config: &NimbulConfig{
				Version: "1",
				Build: []BuildConfig{
					{Name: "build-1", Dockerfile: "Dockerfile", Tags: []string{"tag1"}},
				},
				Deploy: []DeployConfig{
					{
						Name:    "deploy-1",
						BuildID: "build-2", // Doesn't exist
						Manifests: []ManifestConfig{
							{Path: "k8s/deploy.yaml"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "does not reference an existing build",
		},
		{
			name: "missing manifest path",
			config: &NimbulConfig{
				Version: "1",
				Build: []BuildConfig{
					{Name: "build-1", Dockerfile: "Dockerfile", Tags: []string{"tag1"}},
				},
				Deploy: []DeployConfig{
					{
						Name:      "deploy-1",
						BuildID:   "build-1",
						Manifests: []ManifestConfig{{Path: ""}},
					},
				},
			},
			wantErr: true,
			errMsg:  "path is required",
		},
		{
			name: "missing override path",
			config: &NimbulConfig{
				Version: "1",
				Build: []BuildConfig{
					{Name: "build-1", Dockerfile: "Dockerfile", Tags: []string{"tag1"}},
				},
				Deploy: []DeployConfig{
					{
						Name:    "deploy-1",
						BuildID: "build-1",
						Manifests: []ManifestConfig{
							{
								Path: "k8s/deploy.yaml",
								Overrides: []OverrideConfig{
									{Value: "test"},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path is required",
		},
		{
			name: "missing override value",
			config: &NimbulConfig{
				Version: "1",
				Build: []BuildConfig{
					{Name: "build-1", Dockerfile: "Dockerfile", Tags: []string{"tag1"}},
				},
				Deploy: []DeployConfig{
					{
						Name:    "deploy-1",
						BuildID: "build-1",
						Manifests: []ManifestConfig{
							{
								Path: "k8s/deploy.yaml",
								Overrides: []OverrideConfig{
									{Path: "spec.template.spec.containers[0].image"},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "value is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain '%s', got '%v'", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
