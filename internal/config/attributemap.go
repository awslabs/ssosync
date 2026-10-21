// Package config ...
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// The supported* sets below are the SINGLE SOURCE OF TRUTH for what the
// attribute-map validator accepts. They are declared as package-level vars (not
// constants, since Go has no const maps) so a later phase that widens the
// supported surface edits exactly one place here and both the validator and any
// error messages pick the change up automatically.
//
// A map[string]struct{} is used as a set: presence of the key is the only thing
// that matters, and struct{} carries no per-entry payload.
var (
	// supportedUserTargetFields are the interfaces.User TARGET field paths the
	// validator accepts as attribute-map keys for the user block this phase.
	supportedUserTargetFields = map[string]struct{}{
		"Name.GivenName":  {},
		"Name.FamilyName": {},
		"Username":        {},
		"Active":          {},
	}

	// supportedUserSourceAttrs are the admin.User SOURCE attribute paths the
	// validator accepts as attribute-map values for the user block this phase.
	supportedUserSourceAttrs = map[string]struct{}{
		"name.givenName":  {},
		"name.familyName": {},
		"primaryEmail":    {},
		"suspended":       {},
	}

	// supportedGroupTargetFields are the interfaces.Group TARGET field paths the
	// validator accepts as attribute-map keys for the group block this phase.
	supportedGroupTargetFields = map[string]struct{}{
		"DisplayName": {},
	}

	// supportedGroupSourceAttrs are the admin.Group SOURCE attribute paths the
	// validator accepts as attribute-map values for the group block this phase.
	supportedGroupSourceAttrs = map[string]struct{}{
		"name": {},
	}
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

	if err := validateAttributeMap("user", result.User, supportedUserTargetFields, supportedUserSourceAttrs); err != nil {
		return AttributeMaps{}, err
	}
	if err := validateAttributeMap("group", result.Group, supportedGroupTargetFields, supportedGroupSourceAttrs); err != nil {
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

// validateAttributeMap hard-fails the attribute-map load on any unsupported or
// empty entry for one block (user or group). It is intentionally strict: the
// mappers that consume a loaded map (mapFromGoogleUser / mapFromGoogleGroup) may
// assume every entry is a known target<-source pair, so an unknown entry is
// caught here at load time rather than silently dropping a field at sync time.
//
// Rules enforced per entry (KEY = interface target field, VALUE = Google source
// attribute):
//   - neither key nor value may be empty/blank;
//   - the key must be in supportedTargets for this block;
//   - the value must be in supportedSources for this block.
//
// ALL violations across the block are collected and reported in a single error
// so a user fixing a map sees every problem at once instead of one per run. Each
// message names the block, the offending token verbatim, WHY it is invalid, and
// the sorted list of currently-supported values for that block, e.g.:
//
//	attribute map (user block): unsupported target field "Name.Givenname" - valid user target fields are: Active, Name.FamilyName, Name.GivenName, Username
func validateAttributeMap(name string, m AttributeMap, supportedTargets, supportedSources map[string]struct{}) error {
	var violations []string

	for k, v := range m {
		keyEmpty := strings.TrimSpace(k) == ""
		valEmpty := strings.TrimSpace(v) == ""

		if keyEmpty {
			violations = append(violations,
				fmt.Sprintf("empty target field - a %s attribute map key must not be empty", name))
		} else if _, ok := supportedTargets[k]; !ok {
			violations = append(violations,
				fmt.Sprintf("unsupported target field %q - valid %s target fields are: %s",
					k, name, sortedKeys(supportedTargets)))
		}

		if valEmpty {
			// Reference the key where we have one so the user can locate the entry.
			if keyEmpty {
				violations = append(violations,
					fmt.Sprintf("empty source attribute - a %s attribute map value must not be empty", name))
			} else {
				violations = append(violations,
					fmt.Sprintf("empty source attribute for target field %q - a %s attribute map value must not be empty", k, name))
			}
		} else if _, ok := supportedSources[v]; !ok {
			violations = append(violations,
				fmt.Sprintf("unsupported source attribute %q - valid %s source attributes are: %s",
					v, name, sortedKeys(supportedSources)))
		}
	}

	if len(violations) == 0 {
		return nil
	}

	// Sort so the aggregated message is deterministic regardless of map
	// iteration order.
	sort.Strings(violations)
	return fmt.Errorf("attribute map (%s block): %s", name, strings.Join(violations, "; "))
}

// sortedKeys returns the keys of a string-set in ascending order, for stable,
// human-readable error messages.
func sortedKeys(set map[string]struct{}) string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
