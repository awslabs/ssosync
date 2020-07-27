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

	assert.Equal(cfg.LogLevel, DefaultLogLevel)
	assert.Equal(cfg.LogFormat, DefaultLogFormat)
	assert.Equal(cfg.Debug, DefaultDebug)
	assert.Equal(cfg.GoogleCredentials, DefaultGoogleCredentials)
}
