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
			"DisplayName": "email"
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
		"DisplayName": "email",
	}, am.Group)
}

func TestLoadAttributeMapsFromFile_ValidPartial(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "attrmap.json")
	// Only the group sub-object is provided; user must fall back to default.
	content := `{"group": {"DisplayName": "email"}}`
	assert.NoError(os.WriteFile(path, []byte(content), 0o600))

	am, err := LoadAttributeMapsFromFile(path)
	assert.NoError(err)
	assert.Equal(DefaultUserAttributeMap, am.User)
	assert.Equal(AttributeMap{"DisplayName": "email"}, am.Group)
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
	assert.Contains(err.Error(), "empty value")
}
