package aws

import "github.com/BurntSushi/toml"

// Config specifes the configuration needed for AWS SSO SCIM
type Config struct {
	Endpoint string
	Token    string
}

// ReadConfigFromFile will read a TOML file into the Config Struct
func ReadConfigFromFile(path string) (*Config, error) {
	var c Config
	_, err := toml.DecodeFile(path, &c)
	return &c, err
}
