package aws

import (
	"context"

	aws_sdk "github.com/aws/aws-sdk-go-v2/aws"
	aws_config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const assumeRoleSessionName = "ssosync"

type assumeRoleAPI interface {
	AssumeRole(ctx context.Context, params *sts.AssumeRoleInput, optFns ...func(*sts.Options)) (*sts.AssumeRoleOutput, error)
}

var loadDefaultAWSConfig = func(ctx context.Context, optFns ...func(*aws_config.LoadOptions) error) (aws_sdk.Config, error) {
	return aws_config.LoadDefaultConfig(ctx, optFns...)
}

var newAssumeRoleClient = func(cfg aws_sdk.Config) assumeRoleAPI {
	return sts.NewFromConfig(cfg)
}

// LoadIdentityStoreConfig returns the AWS config used for Identity Store API calls.
// When assumeRoleArn is set, the returned config uses credentials from STS AssumeRole.
func LoadIdentityStoreConfig(ctx context.Context, region string, assumeRoleArn string) (aws_sdk.Config, error) {
	cfg, err := loadDefaultAWSConfig(ctx, aws_config.WithRegion(region))
	if err != nil {
		return aws_sdk.Config{}, err
	}

	if assumeRoleArn == "" {
		return cfg, nil
	}

	assumedCfg := cfg
	assumedCfg.Credentials = aws_sdk.NewCredentialsCache(stscreds.NewAssumeRoleProvider(
		newAssumeRoleClient(cfg),
		assumeRoleArn,
		func(options *stscreds.AssumeRoleOptions) {
			options.RoleSessionName = assumeRoleSessionName
		},
	))

	return assumedCfg, nil
}
