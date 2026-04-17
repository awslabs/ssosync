package cmd

import (
	"testing"

	"github.com/awslabs/ssosync/internal/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddFlagsIncludesAssumeRoleArn(t *testing.T) {
	flag := rootCmd.Flags().Lookup("assume-role-arn")
	require.NotNil(t, flag)
	assert.Equal(t, "", flag.DefValue)
}

func TestViperParsesAssumeRoleArn(t *testing.T) {
	t.Setenv("SSOSYNC_ASSUME_ROLE_ARN", "arn:aws:iam::123456789012:role/identity-center-admin")

	v := viper.New()
	v.SetEnvPrefix("ssosync")
	v.AutomaticEnv()
	require.NoError(t, v.BindEnv("assume_role_arn"))

	cfg := config.New()
	require.NoError(t, v.Unmarshal(cfg))
	assert.Equal(t, "arn:aws:iam::123456789012:role/identity-center-admin", cfg.AssumeRoleArn)
}
