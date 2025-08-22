// Package config ...
package config

import "errors"

// Config ...
type Config struct {
	// Verbose toggles the verbosity
	Debug bool
	// LogLevel is the level with with to log for this config
	LogLevel string `mapstructure:"log_level"`
	// LogFormat is the format that is used for logging
	LogFormat string `mapstructure:"log_format"`
	// GoogleCredentials ...
	GoogleCredentials string `mapstructure:"google_credentials"`
	// GoogleAdmin ...
	GoogleAdmin string `mapstructure:"google_admin"`
	// UserMatch ...
	UserMatch string `mapstructure:"user_match"`
	// GroupFilter ...
	GroupMatch string `mapstructure:"group_match"`
	// SCIMEndpoint ....
	SCIMEndpoint string `mapstructure:"scim_endpoint"`
	// SCIMAccessToken ...
	SCIMAccessToken string `mapstructure:"scim_access_token"`
	// IsLambda ...
	IsLambda bool
	// IsLambdaRunningInCodePipeline ...
	IsLambdaRunningInCodePipeline bool
	// Ignore users ...
	IgnoreUsers []string `mapstructure:"ignore_users"`
	// Ignore groups ...
	IgnoreGroups []string `mapstructure:"ignore_groups"`
	// Include groups ...
	IncludeGroups []string `mapstructure:"include_groups"`
	// SyncMethod allow to defined the sync method used to get the user and groups from Google Workspace
	SyncMethod string `mapstructure:"sync_method"`
	// Region is the region that the identity store exists on
	Region string `mapstructure:"region"`
	// IdentityStoreID is the ID of the identity store
	IdentityStoreID string `mapstructure:"identity_store_id"`
	// Precaching queries as a comma separated list of query strings
	PrecacheOrgUnits []string
	// DryRun flag, when set to true, no change will be made in the Identity Store
	DryRun bool
	// sync suspended user, if true suspended user and their group memberships are sync'd into IAM Identity Center
	SyncSuspended bool
	// User filter string
	UserFilter string
}

const (
	// DefaultLogLevel is the default logging level.
	DefaultLogLevel = "info"
	// DefaultLogFormat is the default format of the logger
	DefaultLogFormat = "text"
	// DefaultDebug is the default debug status.
	DefaultDebug = false
	// DefaultGoogleCredentials is the default credentials path
	DefaultGoogleCredentials = "credentials.json"
	// DefaultSyncMethod is the default sync method to use.
	DefaultSyncMethod = "groups"
	// DefaultPrecacheOrgUnits
	DefaultPrecacheOrgUnits = "/"
)

// New returns a new Config
func New() *Config {
	return &Config{
		Debug:             DefaultDebug,
		LogLevel:          DefaultLogLevel,
		LogFormat:         DefaultLogFormat,
		SyncMethod:        DefaultSyncMethod,
		GoogleCredentials: DefaultGoogleCredentials,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.GoogleAdmin == "" {
		return errors.New("google admin email is required")
	}

	if c.SCIMEndpoint == "" {
		return errors.New("SCIM endpoint is required")
	}

	if c.SCIMAccessToken == "" {
		return errors.New("SCIM access token is required")
	}

	if c.Region == "" {
		return errors.New("AWS region is required")
	}

	if c.IdentityStoreID == "" {
		return errors.New("identity store ID is required")
	}

	if c.SyncMethod != "groups" && c.SyncMethod != "users_groups" {
		return errors.New("sync method must be either 'groups' or 'users_groups'")
	}

	return nil
}
