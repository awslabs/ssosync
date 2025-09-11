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
	assert.Equal(cfg.SyncMethod, DefaultSyncMethod)
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

func TestConfigValidate_MissingRegion(t *testing.T) {
	cfg := &Config{
		GoogleAdmin:     "admin@example.com",
		SCIMEndpoint:    "https://scim.example.com",
		SCIMAccessToken: "token123",
		IdentityStoreID: "d-123456789",
		SyncMethod:      "groups",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS region is required")
}

func TestConfigValidate_MissingIdentityStoreID(t *testing.T) {
	cfg := &Config{
		GoogleAdmin:     "admin@example.com",
		SCIMEndpoint:    "https://scim.example.com",
		SCIMAccessToken: "token123",
		Region:          "us-east-1",
		SyncMethod:      "groups",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "identity store ID is required")
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
	assert.Equal(t, "groups", DefaultSyncMethod)
	assert.Equal(t, "/", DefaultPrecacheOrgUnits)
}
