package config

// Config contains a configuration for Autobot
type Config struct {
	// Verbose toggles the verbosity
	Debug bool
	// LogLevel is the level with with to log for this config
	LogLevel string `mapstructure:"log_level"`
	// LogFormat is the format that is used for logging
	LogFormat string `mapstructure:"log_format"`
	// GoogleCredentialsPath is the path to the credentials
	GoogleCredentialsPath string `mapstructure:"google_credentials"`
	// GoogleTokenPath is the path to the token
	GoogleTokenPath string `mapstructure:"google_token"`
	// SCIMConfig is the path to the AWS SSO SCIM Config
	SCIMConfig string `mapstructure:"aws_toml"`
}

const (
	// DefaultLogLevel is the default logging level.
	DefaultLogLevel = "warn"
	// DefaultLogFormat is the default format of the logger
	DefaultLogFormat = "text"
	// DefaultDebug is the default debug status.
	DefaultDebug = false
	// DefaultGoogleCredentialsPath is the default credentials path
	DefaultGoogleCredentialsPath = "credentials.json"
	// DefaultGoogleTokenPath is the default token path
	DefaultGoogleTokenPath = "token.json"
	// DefaultSCIMConfig is the default for the AWS SSO SCIM Configuraiton
	DefaultSCIMConfig = "aws.toml"
)

// New returns a new Config
func New() *Config {
	return &Config{
		Debug:                 DefaultDebug,
		LogLevel:              DefaultLogLevel,
		LogFormat:             DefaultLogFormat,
		GoogleCredentialsPath: DefaultGoogleCredentialsPath,
		GoogleTokenPath:       DefaultGoogleTokenPath,
		SCIMConfig:            DefaultSCIMConfig,
	}
}
