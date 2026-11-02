package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"ssosync/internal/config"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAWSExclusionDefaults(t *testing.T) {
	configuration := config.New()
	assert.Empty(t, configuration.ExcludeAWSUserIDs)
	assert.Empty(t, configuration.ExcludeAWSGroupIDs)
	assert.False(t, configuration.RetainUnmatched)
}

func TestAWSExclusionCLIFlags(t *testing.T) {
	previousUsers, previousGroups := cfg.ExcludeAWSUserIDs, cfg.ExcludeAWSGroupIDs
	previousIgnoredUsers, previousIgnoredGroups := cfg.IgnoreUsers, cfg.IgnoreGroups
	userFlag := rootCmd.Flags().Lookup("exclude-aws-user-ids")
	groupFlag := rootCmd.Flags().Lookup("exclude-aws-group-ids")
	ignoredUserFlag := rootCmd.Flags().Lookup("ignore-users")
	ignoredGroupFlag := rootCmd.Flags().Lookup("ignore-groups")
	previousUserChanged, previousGroupChanged := userFlag.Changed, groupFlag.Changed
	previousIgnoredUserChanged, previousIgnoredGroupChanged := ignoredUserFlag.Changed, ignoredGroupFlag.Changed
	t.Cleanup(func() {
		_ = rootCmd.Flags().Set("exclude-aws-user-ids", strings.Join(previousUsers, ","))
		_ = rootCmd.Flags().Set("exclude-aws-group-ids", strings.Join(previousGroups, ","))
		_ = rootCmd.Flags().Set("ignore-users", strings.Join(previousIgnoredUsers, ","))
		_ = rootCmd.Flags().Set("ignore-groups", strings.Join(previousIgnoredGroups, ","))
		userFlag.Changed, groupFlag.Changed = previousUserChanged, previousGroupChanged
		ignoredUserFlag.Changed, ignoredGroupFlag.Changed = previousIgnoredUserChanged, previousIgnoredGroupChanged
		cfg.ExcludeAWSUserIDs, cfg.ExcludeAWSGroupIDs = previousUsers, previousGroups
		cfg.IgnoreUsers, cfg.IgnoreGroups = previousIgnoredUsers, previousIgnoredGroups
		viper.Reset()
	})
	t.Setenv("SSOSYNC_EXCLUDE_AWS_USER_IDS", "other-user")
	t.Setenv("SSOSYNC_EXCLUDE_AWS_GROUP_IDS", "other-group")
	t.Setenv("SSOSYNC_IGNORE_USERS", "other-user")
	t.Setenv("SSOSYNC_IGNORE_GROUPS", "other-group")
	require.NoError(t, rootCmd.Flags().Set("exclude-aws-user-ids", "user-one, user-two"))
	require.NoError(t, rootCmd.Flags().Set("exclude-aws-group-ids", "group-one, group-two"))
	require.NoError(t, rootCmd.Flags().Set("ignore-users", "user-one, user-two"))
	require.NoError(t, rootCmd.Flags().Set("ignore-groups", "group-one, group-two"))
	initConfig()
	assert.Equal(t, cfg.IgnoreUsers, cfg.ExcludeAWSUserIDs)
	assert.Equal(t, cfg.IgnoreGroups, cfg.ExcludeAWSGroupIDs)
}

func TestAWSExclusionCLIEnvironment(t *testing.T) {
	previous := cfg
	cfg = config.New()
	viper.Reset()
	t.Cleanup(func() { cfg = previous; viper.Reset() })
	t.Setenv("SSOSYNC_EXCLUDE_AWS_USER_IDS", "user-one, user-two")
	t.Setenv("SSOSYNC_EXCLUDE_AWS_GROUP_IDS", "group-one, group-two")
	t.Setenv("SSOSYNC_IGNORE_USERS", "user-one, user-two")
	t.Setenv("SSOSYNC_IGNORE_GROUPS", "group-one, group-two")
	initConfig()
	assert.Equal(t, cfg.IgnoreUsers, cfg.ExcludeAWSUserIDs)
	assert.Equal(t, cfg.IgnoreGroups, cfg.ExcludeAWSGroupIDs)
	assert.Len(t, cfg.ExcludeAWSUserIDs, 2)
	assert.Len(t, cfg.ExcludeAWSGroupIDs, 2)
}

func TestAWSExclusionLambdaEnvironment(t *testing.T) {
	previousConfig, previousAWSConfig := cfg, awsConfig
	previousCache, previousPipeline := secretCache, codePipelineClient
	cfg = config.New()
	cfg.IsLambda = true
	viper.Reset()
	t.Cleanup(func() {
		cfg, awsConfig = previousConfig, previousAWSConfig
		secretCache, codePipelineClient = previousCache, previousPipeline
		viper.Reset()
	})
	for key, value := range map[string]string{
		"AWS_REGION": "us-east-1", "AWS_PROFILE": "", "AWS_EC2_METADATA_DISABLED": "true",
		"AWS_CONFIG_FILE": t.TempDir() + "/none", "AWS_SHARED_CREDENTIALS_FILE": t.TempDir() + "/none",
		"GOOGLE_ADMIN": "", "GOOGLE_CREDENTIALS": "{}", "CUSTOMER_ID": "my_customer",
		"SCIM_ENDPOINT": "", "SCIM_ACCESS_TOKEN": "", "ATTRIBUTE_MAP": "",
		"EXCLUDE_AWS_USER_IDS": "user-one, user-two", "EXCLUDE_AWS_GROUP_IDS": "group-one, group-two",
		"IGNORE_USERS": "user-one, user-two", "IGNORE_GROUPS": "group-one, group-two",
	} {
		t.Setenv(key, value)
	}
	initConfig()
	assert.Equal(t, cfg.IgnoreUsers, cfg.ExcludeAWSUserIDs)
	assert.Equal(t, cfg.IgnoreGroups, cfg.ExcludeAWSGroupIDs)
	assert.Len(t, cfg.ExcludeAWSUserIDs, 2)
	assert.Len(t, cfg.ExcludeAWSGroupIDs, 2)
}

func TestAWSExclusionSAMParameters(t *testing.T) {
	content, err := os.ReadFile("../template.yaml")
	require.NoError(t, err)
	var template map[string]any
	require.NoError(t, yaml.Unmarshal(content, &template))
	parameters := template["Parameters"].(map[string]any)
	conditions := template["Conditions"].(map[string]any)
	resources := template["Resources"].(map[string]any)
	function := resources["SSOSyncFunction"].(map[string]any)["Properties"].(map[string]any)
	variables := function["Environment"].(map[string]any)["Variables"].(map[string]any)
	for env, parameter := range map[string]string{
		"EXCLUDE_AWS_USER_IDS": "ExcludeAWSUserIDs", "EXCLUDE_AWS_GROUP_IDS": "ExcludeAWSGroupIDs",
	} {
		assert.Equal(t, "", parameters[parameter].(map[string]any)["Default"])
		assert.Equal(t, []any{[]any{parameter, ""}}, conditions["Set"+parameter])
		assert.Equal(t, []any{"Set" + parameter, parameter, "AWS::NoValue"}, variables[env])
	}
}

// The real Issuer stamped by IAM Identity Center keeps the provisioning tenant
// id's original (mixed) case, and ssosync compares Issuers case-sensitively, so
// the expected IssuerArn must preserve case (issue #383).
func TestBuildIssuerArn_PreservesCase(t *testing.T) {
	got := buildIssuerArn("123456789012", "d-1234567890", "Dbv12345678-abcd-EF01-2345-6789abcdef01")
	assert.Equal(t,
		"arn:aws:identitystore::123456789012:identitystore/d-1234567890/provisioningtenant/Dbv12345678-abcd-EF01-2345-6789abcdef01",
		got)
}

// The IssuerArn account is the instance owner, not the account ssosync runs in
// (a linked or delegated administrator account), and the caller account is not
// even looked up when the owner is known (issue #383).
func TestIssuerAccountId_UsesInstanceOwner(t *testing.T) {
	called := false
	caller := func() string { called = true; return "222222222222" }

	acct, source := issuerAccountId("111111111111", caller)
	assert.Equal(t, "111111111111", acct)
	assert.Equal(t, accountSourceInstanceOwner, source)
	assert.False(t, called, "STS must not be called when the instance owner is known")
}

func TestIssuerAccountId_FallsBackToCaller(t *testing.T) {
	acct, source := issuerAccountId("", func() string { return "222222222222" })
	assert.Equal(t, "222222222222", acct)
	assert.Equal(t, accountSourceCallerIdentity, source)
}

// End to end for the ARN: running in a delegated/linked account, the expected
// IssuerArn equals the real Issuer from the issue report (management account +
// mixed-case tenant).
func TestIssuerArn_DelegatedAccountMatchesRealIssuer(t *testing.T) {
	acct, _ := issuerAccountId("111111111111", func() string { return "222222222222" })
	got := buildIssuerArn(acct, "d-1234567890", "Dbv12345678-abcd")
	assert.Equal(t, "arn:aws:identitystore::111111111111:identitystore/d-1234567890/provisioningtenant/Dbv12345678-abcd", got)
}

func TestGetIssuerId_PreservesCaseFromEndpoint(t *testing.T) {
	prev := cfg
	defer func() { cfg = prev }()
	cfg = config.New()

	for name, tc := range map[string]struct{ endpoint, want string }{
		"mixed case":     {"https://scim.eu-west-1.amazonaws.com/Dbv12345678-abcd-EF01-2345-6789abcdef01/scim/v2", "Dbv12345678-abcd-EF01-2345-6789abcdef01"},
		"trailing slash": {"https://scim.us-east-1.amazonaws.com/Abc-123/scim/v2/", "Abc-123"},
		"no tenant":      {"https://example.com/scim/v2", ""},
	} {
		t.Run(name, func(t *testing.T) {
			cfg.SCIMEndpoint = tc.endpoint
			assert.Equal(t, tc.want, getIssuerId())
		})
	}
}

// The function's own resources (secrets, SSM parameters, CodePipeline) are
// read in the Lambda's region, never in the IAM Identity Center region, even on
// a warm invocation where cfg.Region still holds the Identity Center region
// from the previous run.
func TestLambdaAWSConfig_UsesFunctionRegionNotIdentityCenterRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-2")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/none")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/none")

	prev := cfg
	t.Cleanup(func() { cfg = prev })
	cfg = config.New()

	for _, identityCenterRegion := range []string{"", "eu-west-1"} {
		cfg.Region = identityCenterRegion // "" cold start, "eu-west-1" warm
		awsCfg, err := lambdaAWSConfig(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "us-east-2", awsCfg.Region, "cfg.Region=%q", identityCenterRegion)
	}
}
