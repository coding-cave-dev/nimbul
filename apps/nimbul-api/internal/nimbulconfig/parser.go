package nimbulconfig

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseFile parses a nimbul.yaml file from the given file path
func ParseFile(path string) (*NimbulConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	return Parse(file)
}

// Parse parses a nimbul.yaml configuration from an io.Reader
func Parse(reader io.Reader) (*NimbulConfig, error) {
	var config NimbulConfig
	decoder := yaml.NewDecoder(reader)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	return &config, nil
}

// ParseBytes parses a nimbul.yaml configuration from a byte slice
func ParseBytes(data []byte) (*NimbulConfig, error) {
	var config NimbulConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return &config, nil
}
