package config_test

import (
	"testing"

	. "github.com/awslabs/ssosync/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	assert := assert.New(t)

	cfg := New()

	assert.NotNil(cfg)

	assert.Equal(DefaultLogLevel, cfg.LogLevel)
	assert.Equal(DefaultLogFormat, cfg.LogFormat)
	assert.Equal(DefaultDebug, cfg.Debug)
	assert.Equal(DefaultGoogleCredentials, cfg.GoogleCredentials)
}
