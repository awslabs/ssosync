package aws

import (
	"context"
	"testing"
	"time"

	aws_sdk "github.com/aws/aws-sdk-go-v2/aws"
	aws_config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	sts_types "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAssumeRoleClient struct {
	input *sts.AssumeRoleInput
}

func (c *stubAssumeRoleClient) AssumeRole(_ context.Context, params *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	c.input = params

	return &sts.AssumeRoleOutput{
		Credentials: &sts_types.Credentials{
			AccessKeyId:     aws_sdk.String("assumed-access-key"),
			SecretAccessKey: aws_sdk.String("assumed-secret"),
			SessionToken:    aws_sdk.String("assumed-token"),
			Expiration:      aws_sdk.Time(time.Now().Add(time.Hour)),
		},
	}, nil
}

func TestLoadIdentityStoreConfigWithoutAssumeRole(t *testing.T) {
	originalLoader := loadDefaultAWSConfig
	originalSTSClient := newAssumeRoleClient
	t.Cleanup(func() {
		loadDefaultAWSConfig = originalLoader
		newAssumeRoleClient = originalSTSClient
	})

	baseCfg := aws_sdk.Config{
		Region:      "eu-west-1",
		Credentials: credentials.NewStaticCredentialsProvider("base-access-key", "base-secret", ""),
	}

	loadDefaultAWSConfig = func(_ context.Context, optFns ...func(*aws_config.LoadOptions) error) (aws_sdk.Config, error) {
		var options aws_config.LoadOptions
		for _, optFn := range optFns {
			require.NoError(t, optFn(&options))
		}
		assert.Equal(t, "eu-west-1", options.Region)

		return baseCfg, nil
	}

	stsClientCalled := false
	newAssumeRoleClient = func(cfg aws_sdk.Config) assumeRoleAPI {
		stsClientCalled = true
		return &stubAssumeRoleClient{}
	}

	cfg, err := LoadIdentityStoreConfig(context.Background(), "eu-west-1", "")
	require.NoError(t, err)

	assert.False(t, stsClientCalled)
	assert.Equal(t, baseCfg.Region, cfg.Region)

	creds, err := cfg.Credentials.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "base-access-key", creds.AccessKeyID)
}

func TestLoadIdentityStoreConfigWithAssumeRole(t *testing.T) {
	originalLoader := loadDefaultAWSConfig
	originalSTSClient := newAssumeRoleClient
	t.Cleanup(func() {
		loadDefaultAWSConfig = originalLoader
		newAssumeRoleClient = originalSTSClient
	})

	stsClient := &stubAssumeRoleClient{}
	loadDefaultAWSConfig = func(_ context.Context, optFns ...func(*aws_config.LoadOptions) error) (aws_sdk.Config, error) {
		var options aws_config.LoadOptions
		for _, optFn := range optFns {
			require.NoError(t, optFn(&options))
		}
		assert.Equal(t, "eu-west-1", options.Region)

		return aws_sdk.Config{
			Region:      options.Region,
			Credentials: credentials.NewStaticCredentialsProvider("base-access-key", "base-secret", ""),
		}, nil
	}

	newAssumeRoleClient = func(cfg aws_sdk.Config) assumeRoleAPI {
		assert.Equal(t, "eu-west-1", cfg.Region)
		return stsClient
	}

	cfg, err := LoadIdentityStoreConfig(context.Background(), "eu-west-1", "arn:aws:iam::123456789012:role/ssosync-target")
	require.NoError(t, err)

	assert.Equal(t, "eu-west-1", cfg.Region)

	creds, err := cfg.Credentials.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "assumed-access-key", creds.AccessKeyID)
	assert.Equal(t, "assumed-secret", creds.SecretAccessKey)
	assert.Equal(t, "assumed-token", creds.SessionToken)

	require.NotNil(t, stsClient.input)
	assert.Equal(t, "arn:aws:iam::123456789012:role/ssosync-target", aws_sdk.ToString(stsClient.input.RoleArn))
	assert.Equal(t, assumeRoleSessionName, aws_sdk.ToString(stsClient.input.RoleSessionName))
}
