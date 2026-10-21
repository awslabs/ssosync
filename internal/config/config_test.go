package config_test

import (
	"testing"

	. "ssosync/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	assert := assert.New(t)

	cfg := New()

	assert.NotNil(cfg)

	assert.Equal(cfg.LogLevel, DefaultLogLevel)
	assert.Equal(cfg.LogFormat, DefaultLogFormat)
	assert.Equal(cfg.Debug, DefaultDebug)
	assert.Equal(cfg.GoogleCredentials, DefaultGoogleCredentials)
	assert.Equal(cfg.AttributeMapFile, DefaultAttributeMapFile)
	assert.Equal(cfg.AttributeMap, DefaultAttributeMap)
	assert.Equal(cfg.SyncMethod, DefaultSyncMethod)
}

func TestDefaultAttributeMap(t *testing.T) {
	assert := assert.New(t)

	// DefaultAttributeMap only references the existing maps; it does not
	// redefine the mappings.
	assert.Equal(DefaultUserAttributeMap, DefaultAttributeMap.User)
	assert.Equal(DefaultGroupAttributeMap, DefaultAttributeMap.Group)
}

func TestDefaultUserAttributeMap_SafeSetEntries(t *testing.T) {
	assert := assert.New(t)

	// The default user map is exactly the four behaviour-preserving scalars plus
	// the four safe-set enrichment fields.
	assert.Equal(AttributeMap{
		"Name.GivenName":          "name.givenName",
		"Name.FamilyName":         "name.familyName",
		"Username":                "primaryEmail",
		"Active":                  "suspended",
		"Title":                   "organizations[primary].title",
		"Enterprise.Department":   "organizations[primary].department",
		"Enterprise.Organization": "organizations[primary].name",
		"Enterprise.CostCenter":   "organizations[primary].costCenter",
	}, DefaultUserAttributeMap)

	// Opt-in-only targets must NOT be in the default map.
	_, hasManager := DefaultUserAttributeMap["Enterprise.Manager"]
	assert.False(hasManager, "Enterprise.Manager must not be defaulted")
	for _, optIn := range []string{"Enterprise.Division", "Enterprise.EmployeeNumber", "ProfileURL", "Emails", "Addresses", "PhoneNumbers"} {
		_, has := DefaultUserAttributeMap[optIn]
		assert.False(has, "%s must not be defaulted", optIn)
	}
}

func TestDefaultGroupAttributeMap_Unchanged(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(AttributeMap{"DisplayName": "name"}, DefaultGroupAttributeMap)
}

func TestConfigValidate_Success(t *testing.T) {
	cfg := &Config{
		GoogleAdmin:     "admin@example.com",
		SCIMEndpoint:    "https://scim.example.com",
		SCIMAccessToken: "token123",
		Region:          "us-east-1",
		IdentityStoreID: "d-123456789",
		SyncMethod:      "groups",
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfigValidate_MissingGoogleAdmin(t *testing.T) {
	cfg := &Config{
		SCIMEndpoint:    "https://scim.example.com",
		SCIMAccessToken: "token123",
		Region:          "us-east-1",
		IdentityStoreID: "d-123456789",
		SyncMethod:      "groups",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "google admin email is required")
}

func TestConfigValidate_MissingSCIMEndpoint(t *testing.T) {
	cfg := &Config{
		GoogleAdmin:     "admin@example.com",
		SCIMAccessToken: "token123",
		Region:          "us-east-1",
		IdentityStoreID: "d-123456789",
		SyncMethod:      "groups",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SCIM endpoint is required")
}

func TestConfigValidate_MissingSCIMAccessToken(t *testing.T) {
	cfg := &Config{
		GoogleAdmin:     "admin@example.com",
		SCIMEndpoint:    "https://scim.example.com",
		Region:          "us-east-1",
		IdentityStoreID: "d-123456789",
		SyncMethod:      "groups",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SCIM access token is required")
}

func TestConfigValidate_InvalidSyncMethod(t *testing.T) {
	cfg := &Config{
		GoogleAdmin:     "admin@example.com",
		SCIMEndpoint:    "https://scim.example.com",
		SCIMAccessToken: "token123",
		Region:          "us-east-1",
		IdentityStoreID: "d-123456789",
		SyncMethod:      "invalid_method",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sync method must be either 'groups' or 'users_groups'")
}

func TestConfigValidate_ValidSyncMethods(t *testing.T) {
	validMethods := []string{"groups", "users_groups"}

	for _, method := range validMethods {
		cfg := &Config{
			GoogleAdmin:     "admin@example.com",
			SCIMEndpoint:    "https://scim.example.com",
			SCIMAccessToken: "token123",
			Region:          "us-east-1",
			IdentityStoreID: "d-123456789",
			SyncMethod:      method,
		}

		err := cfg.Validate()
		assert.NoError(t, err, "Method %s should be valid", method)
	}
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "info", DefaultLogLevel)
	assert.Equal(t, "text", DefaultLogFormat)
	assert.Equal(t, false, DefaultDebug)
	assert.Equal(t, "credentials.json", DefaultGoogleCredentials)
	assert.Equal(t, "", DefaultAttributeMapFile)
	assert.Equal(t, "groups", DefaultSyncMethod)
}
