package config_test

import (
	"os"
	"path/filepath"
	"testing"

	. "ssosync/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestLoadAttributeMapsFromFile_ValidFull(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "attrmap.json")
	content := `{
		"user": {
			"Name.GivenName": "name.givenName",
			"Username": "primaryEmail"
		},
		"group": {
			"DisplayName": "name"
		}
	}`
	assert.NoError(os.WriteFile(path, []byte(content), 0o600))

	am, err := LoadAttributeMapsFromFile(path)
	assert.NoError(err)
	assert.Equal(AttributeMap{
		"Name.GivenName": "name.givenName",
		"Username":       "primaryEmail",
	}, am.User)
	assert.Equal(AttributeMap{
		"DisplayName": "name",
	}, am.Group)
}

func TestLoadAttributeMapsFromFile_ValidPartial(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "attrmap.json")
	// Only the group sub-object is provided; user must fall back to default.
	content := `{"group": {"DisplayName": "name"}}`
	assert.NoError(os.WriteFile(path, []byte(content), 0o600))

	am, err := LoadAttributeMapsFromFile(path)
	assert.NoError(err)
	assert.Equal(DefaultUserAttributeMap, am.User)
	assert.Equal(AttributeMap{"DisplayName": "name"}, am.Group)
}

func TestLoadAttributeMapsFromFile_EmptyPathFallsBackToDefault(t *testing.T) {
	assert := assert.New(t)

	am, err := LoadAttributeMapsFromFile("")
	assert.NoError(err)
	assert.Equal(DefaultAttributeMap, am)
}

func TestLoadAttributeMapsFromFile_MissingFile(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	_, err := LoadAttributeMapsFromFile(path)
	assert.Error(err)
	assert.Contains(err.Error(), "cannot read attribute map file")
}

func TestLoadAttributeMapsFromString_Empty(t *testing.T) {
	assert := assert.New(t)

	am, err := LoadAttributeMapsFromString("")
	assert.NoError(err)
	assert.Equal(DefaultAttributeMap, am)

	am, err = LoadAttributeMapsFromString("   \n\t")
	assert.NoError(err)
	assert.Equal(DefaultAttributeMap, am)
}

func TestLoadAttributeMapsFromString_ValidJSON(t *testing.T) {
	assert := assert.New(t)

	am, err := LoadAttributeMapsFromString(`{"user":{"Username":"primaryEmail"}}`)
	assert.NoError(err)
	assert.Equal(AttributeMap{"Username": "primaryEmail"}, am.User)
	// group omitted -> default
	assert.Equal(DefaultGroupAttributeMap, am.Group)
}

func TestLoadAttributeMapsFromString_Malformed(t *testing.T) {
	assert := assert.New(t)

	_, err := LoadAttributeMapsFromString(`{not valid json`)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid attribute map JSON")
}

func TestParseAttributeMaps_EmptyValueFailsValidation(t *testing.T) {
	assert := assert.New(t)

	_, err := ParseAttributeMaps([]byte(`{"user":{"Username":""}}`))
	assert.Error(err)
	assert.Contains(err.Error(), "empty source attribute")
	assert.Contains(err.Error(), "user block")
}

func TestParseAttributeMaps_DefaultMapsPassValidation(t *testing.T) {
	assert := assert.New(t)

	am, err := ParseAttributeMaps([]byte(`{}`))
	assert.NoError(err)
	assert.Equal(DefaultUserAttributeMap, am.User)
	assert.Equal(DefaultGroupAttributeMap, am.Group)
}

func TestParseAttributeMaps_UnsupportedTargetField(t *testing.T) {
	assert := assert.New(t)

	// Wrong case on a known field: "Name.Givenname" is not a supported target.
	_, err := ParseAttributeMaps([]byte(`{"user":{"Name.Givenname":"name.givenName"}}`))
	assert.Error(err)
	msg := err.Error()
	assert.Contains(msg, "attribute map (user block)")
	assert.Contains(msg, `unsupported target field "Name.Givenname"`)
	// The valid-set listing names the supported user target fields.
	assert.Contains(msg, "Active, Name.FamilyName, Name.GivenName, Username")
}

func TestParseAttributeMaps_UnsupportedSourceAttr(t *testing.T) {
	assert := assert.New(t)

	// "email" is not a supported group source attribute (only "name" is).
	_, err := ParseAttributeMaps([]byte(`{"group":{"DisplayName":"email"}}`))
	assert.Error(err)
	msg := err.Error()
	assert.Contains(msg, "attribute map (group block)")
	assert.Contains(msg, `unsupported source attribute "email"`)
	assert.Contains(msg, "valid group source attributes are: name")
}

func TestParseAttributeMaps_EmptyKeyFailsValidation(t *testing.T) {
	assert := assert.New(t)

	_, err := ParseAttributeMaps([]byte(`{"user":{"":"primaryEmail"}}`))
	assert.Error(err)
	msg := err.Error()
	assert.Contains(msg, "attribute map (user block)")
	assert.Contains(msg, "empty target field")
}

func TestParseAttributeMaps_MultipleViolationsAggregated(t *testing.T) {
	assert := assert.New(t)

	// Two distinct problems in the user block: an unsupported target field and
	// an unsupported source attribute. Both must be reported in one error.
	_, err := ParseAttributeMaps([]byte(`{"user":{"Bogus":"name.givenName","Username":"notAnAttr"}}`))
	assert.Error(err)
	msg := err.Error()
	assert.Contains(msg, `unsupported target field "Bogus"`)
	assert.Contains(msg, `unsupported source attribute "notAnAttr"`)
}
