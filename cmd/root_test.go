package cmd

import (
	"testing"

	"ssosync/internal/config"

	"github.com/stretchr/testify/assert"
)

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
