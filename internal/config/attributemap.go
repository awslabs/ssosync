// Package config ...
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ParseAttributeMaps is the single source of truth for the attribute map wire
// format. Both the CLI file loader and the Lambda env/secret loader delegate to
// it so file contents and resolved env/secret contents are parsed identically.
//
// Wire format: a JSON object with two optional sub-objects,
// {"user": {<target>:<source>...}, "group": {<target>:<source>...}}. An omitted
// sub-object falls back to its corresponding default map, so a partial document
// is valid. This applies to the groups sync method only.
func ParseAttributeMaps(data []byte) (AttributeMaps, error) {
	var parsed AttributeMaps

	// Start from the defaults so any omitted sub-object keeps its default map.
	result := AttributeMaps{
		User:  DefaultAttributeMap.User,
		Group: DefaultAttributeMap.Group,
	}

	if err := json.Unmarshal(data, &parsed); err != nil {
		return AttributeMaps{}, fmt.Errorf("invalid attribute map JSON: %w", err)
	}

	if parsed.User != nil {
		result.User = parsed.User
	}
	if parsed.Group != nil {
		result.Group = parsed.Group
	}

	if err := validateAttributeMap("user", result.User); err != nil {
		return AttributeMaps{}, err
	}
	if err := validateAttributeMap("group", result.Group); err != nil {
		return AttributeMaps{}, err
	}

	return result, nil
}

// LoadAttributeMapsFromString parses an attribute map from a raw string (used by
// the Lambda path, where ATTRIBUTE_MAP resolves to inline JSON or the contents
// of a Secrets Manager / SSM entry). An empty/blank string falls back to
// DefaultAttributeMap, matching the credentials unset-value behaviour.
func LoadAttributeMapsFromString(s string) (AttributeMaps, error) {
	if strings.TrimSpace(s) == "" {
		return DefaultAttributeMap, nil
	}
	return ParseAttributeMaps([]byte(s))
}

// LoadAttributeMapsFromFile reads and parses an attribute map from a JSON file
// (used by the CLI --attribute-map-file flag). An empty path falls back to
// DefaultAttributeMap, matching the credentials unset-value behaviour; a missing
// or unreadable file returns a clear error.
func LoadAttributeMapsFromFile(path string) (AttributeMaps, error) {
	if path == "" {
		return DefaultAttributeMap, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return AttributeMaps{}, fmt.Errorf("cannot read attribute map file %q: %w", path, err)
	}

	return ParseAttributeMaps(data)
}

// validateAttributeMap ensures every key and value in the map is a non-empty
// string, returning a clear error identifying the offending entry.
func validateAttributeMap(name string, m AttributeMap) error {
	for k, v := range m {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("invalid attribute map: %s map contains an empty key", name)
		}
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("invalid attribute map: %s map key %q has an empty value", name, k)
		}
	}
	return nil
}
