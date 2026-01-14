package nimbulconfig

// NimbulConfig represents the root configuration structure
type NimbulConfig struct {
	Version string         `yaml:"version"`
	Build   []BuildConfig  `yaml:"build"`
	Deploy  []DeployConfig `yaml:"deploy"`
}

// BuildConfig defines a Docker build configuration
type BuildConfig struct {
	Name       string   `yaml:"name"`
	Dockerfile string   `yaml:"dockerfile"`
	Context    string   `yaml:"context"`
	Tags       []string `yaml:"tags"`
}

// DeployConfig defines a deployment configuration
type DeployConfig struct {
	Name      string           `yaml:"name"`
	BuildID   string           `yaml:"buildId"`
	Manifests []ManifestConfig `yaml:"manifests"`
}

// ManifestConfig defines a Kubernetes manifest configuration
type ManifestConfig struct {
	Path      string           `yaml:"path"`
	Overrides []OverrideConfig `yaml:"overrides"`
}

// OverrideConfig defines how to override values in a manifest
type OverrideConfig struct {
	Path  string      `yaml:"path"`  // JSONPath-style path
	Match MatchConfig `yaml:"match"` // Filter criteria
	Value string      `yaml:"value"` // Value with template support
}

// MatchConfig defines filter criteria for selecting resources
type MatchConfig struct {
	Kind string `yaml:"kind"` // e.g., "Deployment", "Service"
	Name string `yaml:"name"` // Optional: resource name
}
