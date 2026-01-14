package nimbulconfig

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseManifestFile parses a Kubernetes manifest file that may contain multiple documents
// separated by `---`. Returns a slice of document maps.
func ParseManifestFile(path string) ([]map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	return ParseManifestBytes(data)
}

// ParseManifestBytes parses Kubernetes manifest bytes that may contain multiple documents
func ParseManifestBytes(data []byte) ([]map[string]interface{}, error) {
	// Split by document separator `---`
	documents := strings.Split(string(data), "---")
	var result []map[string]interface{}

	for i, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var parsedDoc map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &parsedDoc); err != nil {
			return nil, fmt.Errorf("failed to parse document %d: %w", i, err)
		}

		if parsedDoc == nil {
			continue
		}

		result = append(result, parsedDoc)
	}

	return result, nil
}

// ApplyOverrides applies override configurations to matching resources in the documents
func ApplyOverrides(docs []map[string]interface{}, overrides []OverrideConfig) error {
	for _, doc := range docs {
		for _, override := range overrides {
			// Check if this document matches the override criteria
			if !matchesResource(doc, override.Match) {
				continue
			}

			// Apply the override
			if err := setValueAtPath(doc, override.Path, override.Value); err != nil {
				return fmt.Errorf("failed to apply override at path '%s': %w", override.Path, err)
			}
		}
	}

	return nil
}

// SerializeManifests converts documents back to YAML string with `---` separators
func SerializeManifests(docs []map[string]interface{}) (string, error) {
	if len(docs) == 0 {
		return "", nil
	}

	var parts []string
	for _, doc := range docs {
		data, err := yaml.Marshal(doc)
		if err != nil {
			return "", fmt.Errorf("failed to marshal document: %w", err)
		}
		parts = append(parts, strings.TrimSpace(string(data)))
	}

	return strings.Join(parts, "\n---\n"), nil
}

// matchesResource checks if a Kubernetes resource matches the match criteria
func matchesResource(doc map[string]interface{}, match MatchConfig) bool {
	// Check kind
	kind, ok := doc["kind"].(string)
	if !ok || kind != match.Kind {
		return false
	}

	// If name is specified, check it
	if match.Name != "" {
		metadata, ok := doc["metadata"].(map[string]interface{})
		if !ok {
			return false
		}
		name, ok := metadata["name"].(string)
		if !ok || name != match.Name {
			return false
		}
	}

	return true
}

// setValueAtPath sets a value at a JSONPath-style path in a nested map structure
// Path format: "spec.template.spec.containers[0].image"
func setValueAtPath(doc map[string]interface{}, path string, value interface{}) error {
	parts := parsePath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	current := interface{}(doc)
	for i, part := range parts[:len(parts)-1] {
		var err error
		current, err = navigateTo(current, part)
		if err != nil {
			return fmt.Errorf("failed to navigate to path segment '%s' at position %d: %w", part, i, err)
		}
	}

	// Set the final value
	lastPart := parts[len(parts)-1]
	return setValue(current, lastPart, value)
}

// parsePath parses a JSONPath-style path into segments
// Example: "spec.template.spec.containers[0].image" -> ["spec", "template", "spec", "containers", "[0]", "image"]
func parsePath(path string) []string {
	// Use regex to split by dots, but preserve array indices like [0]
	re := regexp.MustCompile(`(\[[\d]+\]|[^.]+)`)
	matches := re.FindAllString(path, -1)
	return matches
}

// navigateTo navigates to a path segment in a nested structure
func navigateTo(current interface{}, segment string) (interface{}, error) {
	// Handle array indexing like [0]
	if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
		indexStr := segment[1 : len(segment)-1]
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			return nil, fmt.Errorf("invalid array index '%s': %w", indexStr, err)
		}

		slice, ok := current.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected array at segment '%s', got %T", segment, current)
		}

		if index < 0 || index >= len(slice) {
			return nil, fmt.Errorf("array index %d out of range (length: %d)", index, len(slice))
		}

		return slice[index], nil
	}

	// Handle map key
	m, ok := current.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map at segment '%s', got %T", segment, current)
	}

	value, exists := m[segment]
	if !exists {
		// Create nested map if it doesn't exist
		newMap := make(map[string]interface{})
		m[segment] = newMap
		return newMap, nil
	}

	return value, nil
}

// setValue sets a value at the final path segment
func setValue(current interface{}, segment string, value interface{}) error {
	// Handle array indexing like [0]
	if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
		indexStr := segment[1 : len(segment)-1]
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			return fmt.Errorf("invalid array index '%s': %w", indexStr, err)
		}

		slice, ok := current.([]interface{})
		if !ok {
			return fmt.Errorf("expected array at segment '%s', got %T", segment, current)
		}

		if index < 0 || index >= len(slice) {
			return fmt.Errorf("array index %d out of range (length: %d)", index, len(slice))
		}

		slice[index] = value
		return nil
	}

	// Handle map key
	m, ok := current.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map at segment '%s', got %T", segment, current)
	}

	m[segment] = value
	return nil
}
