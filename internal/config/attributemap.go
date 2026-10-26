// Package config ...
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
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
	// validator accepts as attribute-map keys for the user block. The SAFE SET
	// (defaulted in DefaultUserAttributeMap) and the OPT-IN-ONLY targets
	// (supported here but deliberately not defaulted, see attrmap-design §5) are
	// both listed; see the inline markers.
	supportedUserTargetFields = map[string]struct{}{
		// Existing / safe set.
		"Name.GivenName":          {},
		"Name.FamilyName":         {},
		"Username":                {},
		"Active":                  {},
		"Title":                   {}, // safe set
		"Enterprise.Department":   {}, // safe set
		"Enterprise.Organization": {}, // safe set
		"Enterprise.CostCenter":   {}, // safe set
		// Opt-in only (supported, NOT defaulted).
		"Enterprise.Manager":        {}, // opt-in (design §5.7)
		"Enterprise.Division":       {}, // opt-in (design §5.2)
		"Enterprise.EmployeeNumber": {}, // opt-in (design §5.3)
		"ProfileURL":                {}, // opt-in (design §5.4)
		"Emails":                    {}, // opt-in (design §5.5)
		"Addresses":                 {}, // opt-in (design §5.5)
		"PhoneNumbers":              {}, // opt-in (design §5.5)
	}

	// supportedUserSourceScalars are the scalar admin.User SOURCE paths the
	// validator accepts as-is (exact string match) for the user block.
	supportedUserSourceScalars = map[string]struct{}{
		"name.givenName":  {},
		"name.familyName": {},
		"primaryEmail":    {},
		"suspended":       {},
	}

	// supportedUserMultiValuedSubfields is the CLOSED (field -> {subfield}) table
	// the validator accepts for type-qualified multi-valued source paths of the
	// form <field>[<selector>].<subfield> (design §5.2). The base path (field)
	// and sub-field are validated exactly against this table; the SELECTOR value
	// (the type string) is validated structurally only (any non-empty type=<v>,
	// or primary), since the Google type labels are tenant data.
	//
	// The address sub-fields other than "formatted"
	// (streetAddress/locality/region/postalCode/country) are deliberately NOT
	// listed, so e.g. addresses[type=work].streetAddress is REJECTED this phase.
	// They ARE AWS-accepted (live Limitations marks addresses "Yes, single value
	// only" with all sub-fields "Yes"; the CreateUser example includes them —
	// https://docs.aws.amazon.com/singlesignon/latest/developerguide/createuser.html
	// and https://docs.aws.amazon.com/singlesignon/latest/developerguide/limitations.html)
	// but are DEFERRED: composing five Google source tokens into one UserAddress
	// element needs multi-source->single-target composition logic that is out of
	// scope this phase. "formatted" (a single scalar) is the one opened address
	// source.
	supportedUserMultiValuedSubfields = map[string]map[string]struct{}{
		"organizations": {"title": {}, "department": {}, "name": {}, "costCenter": {}, "domain": {}},
		"relations":     {"value": {}},
		"websites":      {"value": {}},
		"emails":        {"address": {}},
		"phones":        {"value": {}},
		"addresses":     {"formatted": {}},
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

	if err := validateAttributeMap("user", result.User, supportedUserTargetFields, validateUserSource); err != nil {
		return AttributeMaps{}, err
	}
	if err := validateAttributeMap("group", result.Group, supportedGroupTargetFields, exactSourceValidator(supportedGroupSourceAttrs)); err != nil {
		return AttributeMaps{}, err
	}

	result.User = dropMultiValuedCustomSources(result.User)

	return result, nil
}

// MultiValuedCustomSourceIgnoredMsg is the WARN logged when the attribute map
// selects an element of a multi-valued custom-schema field. Exported so the
// tests can match it.
const MultiValuedCustomSourceIgnoredMsg = "attribute map: multi-valued custom-schema attributes are not supported; mapping ignored"

// dropMultiValuedCustomSources removes (with a WARN) every user entry whose
// source selects an element of a custom-schema field with a [type=<value>]
// selector. Google custom fields cannot be typed, and multi-valued custom
// attributes are not supported yet, so such a mapping is ignored rather than
// failing the load. The target is then simply unmapped (cleared under
// clear-to-consistent). It returns a new map and never modifies m, which may be
// the shared DefaultAttributeMap.User.
func dropMultiValuedCustomSources(m AttributeMap) AttributeMap {
	out := make(AttributeMap, len(m))
	for target, source := range m {
		if strings.HasPrefix(source, "customSchemas.") && strings.Contains(source, "[") {
			log.WithFields(log.Fields{
				"target": target,
				"source": source,
				"reason": "customSourceSelector",
			}).Warn(MultiValuedCustomSourceIgnoredMsg)
			continue
		}
		out[target] = source
	}
	return out
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
//   - the value must be accepted by validateSource for this block. The group
//     block uses an exact closed set (exactSourceValidator); the user block uses
//     the structural classifier (validateUserSource) that accepts scalars, any
//     well-formed customSchemas.<schema>.<field> path, and type-qualified
//     multi-valued paths with any non-empty type= selector.
//
// ALL violations across the block are collected and reported in a single error
// so a user fixing a map sees every problem at once instead of one per run. Each
// message names the block, the offending token verbatim, WHY it is invalid, and
// the sorted list of currently-supported values for that block, e.g.:
//
//	attribute map (user block): unsupported target field "Name.Givenname" - valid user target fields are: Active, Name.FamilyName, Name.GivenName, Username
func validateAttributeMap(name string, m AttributeMap, supportedTargets map[string]struct{}, validateSource func(name, source string) string) error {
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
		} else if msg := validateSource(name, v); msg != "" {
			violations = append(violations, msg)
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

// exactSourceValidator returns a source validator that accepts only the exact
// strings in the given set (used by the group block, whose sources are a fixed
// closed list). It returns "" for an accepted source, else a Step-A error
// fragment naming the offending token and the sorted accepted set.
func exactSourceValidator(supportedSources map[string]struct{}) func(name, source string) string {
	return func(name, source string) string {
		if _, ok := supportedSources[source]; ok {
			return ""
		}
		return fmt.Sprintf("unsupported source attribute %q - valid %s source attributes are: %s",
			source, name, sortedKeys(supportedSources))
	}
}

// validateUserSource validates a user-block SOURCE value per the design §5.2/§5.3
// classifier: scalar (exact set), custom (customSchemas. prefix, structural
// only), or multi-valued (<field>[<selector>].<subfield>, structural base-path +
// sub-field closed table with any non-empty type= selector or primary). It
// returns "" when accepted, else a Step-A error fragment naming the offending
// token, WHY it is invalid, and the accepted forms.
func validateUserSource(name, source string) string {
	// Scalar fast-path.
	if _, ok := supportedUserSourceScalars[source]; ok {
		return ""
	}

	// Custom-schema source: structural-only acceptance (design §5.3).
	if strings.HasPrefix(source, "customSchemas.") {
		return validateCustomSource(name, source)
	}

	// Multi-valued: <field>[<selector>].<subfield>.
	open := strings.IndexByte(source, '[')
	if open <= 0 {
		return unsupportedUserSourceMsg(name, source)
	}
	close := strings.IndexByte(source[open:], ']')
	if close < 0 {
		return unsupportedUserSourceMsg(name, source)
	}
	close += open
	field := source[:open]
	selector := source[open+1 : close]
	rest := source[close+1:]
	if !strings.HasPrefix(rest, ".") || len(rest) < 2 {
		return unsupportedUserSourceMsg(name, source)
	}
	subfield := rest[1:]

	subs, ok := supportedUserMultiValuedSubfields[field]
	if !ok {
		return fmt.Sprintf("unsupported source attribute %q - valid %s source base paths are: %s",
			source, name, sortedKeys(mapKeysAsSet(supportedUserMultiValuedSubfields)))
	}
	if _, ok := subs[subfield]; !ok {
		return fmt.Sprintf("unsupported source attribute %q - valid %s sub-fields for %q are: %s",
			source, name, field, sortedKeys(subs))
	}
	if !validSelector(selector) {
		return fmt.Sprintf("malformed selector in source attribute %q - valid selectors are: [primary], [type=<value>]",
			source)
	}
	return ""
}

// validateCustomSource validates a customSchemas.-prefixed user source
// structurally (design §5.3): it must parse into a non-empty schema and
// non-empty field via the first-dot rule. A well-formed trailing
// [type=<non-empty value>] selector is accepted here only so that
// dropMultiValuedCustomSources can warn about it and ignore the entry
// (multi-valued custom attributes are not supported). [primary] is REJECTED for
// a custom source (Google custom multi-valued entries carry no primary flag).
func validateCustomSource(name, source string) string {
	path := source
	selector := ""
	hasSelector := false
	if open := strings.IndexByte(source, '['); open >= 0 {
		close := strings.IndexByte(source[open:], ']')
		if close < 0 {
			return customSourceMsg(name, source)
		}
		close += open
		// Anything after the closing ] is malformed for a custom source.
		if close != len(source)-1 {
			return customSourceMsg(name, source)
		}
		selector = source[open+1 : close]
		path = source[:open]
		hasSelector = true
	}

	schema, field, ok := parseCustomSchemaPathForValidation(path)
	if !ok || schema == "" || field == "" {
		return customSourceMsg(name, source)
	}

	if hasSelector {
		v, isType := strings.CutPrefix(selector, "type=")
		if !isType || v == "" {
			return customSourceMsg(name, source)
		}
	}
	return ""
}

// parseCustomSchemaPathForValidation mirrors the resolver's first-dot parse rule
// for a selector-free customSchemas.<schema>.<field> path.
func parseCustomSchemaPathForValidation(path string) (schema, field string, ok bool) {
	remainder, has := strings.CutPrefix(path, "customSchemas.")
	if !has {
		return "", "", false
	}
	parts := strings.SplitN(remainder, ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// validSelector reports whether a standard multi-valued selector is accepted:
// "primary" or "type=<non-empty value>".
func validSelector(selector string) bool {
	if selector == "primary" {
		return true
	}
	if v, ok := strings.CutPrefix(selector, "type="); ok {
		return v != ""
	}
	return false
}

func unsupportedUserSourceMsg(name, source string) string {
	return fmt.Sprintf("unsupported source attribute %q - valid %s source attributes are scalars (%s), multi-valued <field>[<selector>].<subfield> over base paths (%s), or customSchemas.<SchemaName>.<fieldName>",
		source, name, sortedKeys(supportedUserSourceScalars), sortedKeys(mapKeysAsSet(supportedUserMultiValuedSubfields)))
}

func customSourceMsg(name, source string) string {
	return fmt.Sprintf("malformed custom-schema source %q - a %s custom source must be customSchemas.<SchemaName>.<fieldName> (single-valued custom fields only; [primary] is not valid for custom sources)",
		source, name)
}

// mapKeysAsSet returns the keys of a (field -> subfields) table as a string-set
// so sortedKeys can render them.
func mapKeysAsSet(m map[string]map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
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
