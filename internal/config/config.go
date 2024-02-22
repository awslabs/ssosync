// Package config ...
package config

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
	// GoogleCredentialsSecretId ...
	GoogleCredentialsSecretID string `mapstructure:"google_credentials_secret_id"`
	// GoogleAdmin ...
	GoogleAdmin string `mapstructure:"google_admin"`
	// GoogleAdminSecretId ...
	GoogleAdminSecretID string `mapstructure:"google_admin_secret_id"`
	// UserMatch ...
	UserMatch string `mapstructure:"user_match"`
	// GroupFilter ...
	GroupMatch string `mapstructure:"group_match"`
	// SCIMEndpoint ....
	SCIMEndpoint string `mapstructure:"scim_endpoint"`
	// SCIMEndpointSecret_id ....
	SCIMEndpointSecretID string `mapstructure:"scim_endpoint_secret_id"`
	// SCIMAccessToken ...
	SCIMAccessToken string `mapstructure:"scim_access_token"`
	// SCIMAccessTokenSecretId ...
	SCIMAccessTokenSecretID string `mapstructure:"scim_access_token_secret_id"`
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
	// RegionSecretId is the secret storing the region that the identity store exists on
	RegionSecretId string `mapstructure:"region_secret_id"`
	// IdentityStoreID is the ID of the identity store
	IdentityStoreID string `mapstructure:"identity_store_id"`
	// IdentityStoreIDSecretId is the ID of the identity store
	IdentityStoreIDSecretID string `mapstructure:"identity_store_id_secret_id"`
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
