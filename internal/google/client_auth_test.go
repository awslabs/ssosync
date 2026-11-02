package google

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// awsWIFConfig is a Workload Identity Federation credential configuration of
// the shape `gcloud iam workload-identity-pools create-cred-config --aws`
// writes. Building a token source from it needs no network: tokens are only
// fetched on first use.
const awsWIFConfig = `{
  "type": "external_account",
  "audience": "//iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/ssosync/providers/aws",
  "subject_token_type": "urn:ietf:params:aws:token-type:aws4_request",
  "token_url": "https://sts.googleapis.com/v1/token",
  "credential_source": {
    "environment_id": "aws1",
    "region_url": "http://169.254.169.254/latest/meta-data/placement/availability-zone",
    "url": "http://169.254.169.254/latest/meta-data/iam/security-credentials",
    "regional_cred_verification_url": "https://sts.{region}.amazonaws.com?Action=GetCallerIdentity&Version=2011-06-15"
  }
}`

// serviceAccountKeyJSON returns a service-account key: valid credentials, but
// not an external_account configuration, so it must not be accepted as the
// WIF configuration. The RSA key is generated per run, so no key is stored in
// the repository.
func serviceAccountKeyJSON(t *testing.T) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(k)
	require.NoError(t, err)
	b, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "p",
		"private_key_id": "k",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   "sa@p.iam.gserviceaccount.com",
		"client_id":      "1",
		"token_uri":      "https://oauth2.googleapis.com/token",
	})
	require.NoError(t, err)
	return string(b)
}

const (
	targetSA     = "ssosync@project.iam.gserviceaccount.com"
	testCustomer = "C01234abc"
)

func TestAuth_Validate(t *testing.T) {
	key := []byte(serviceAccountKeyJSON(t))
	wif := []byte(awsWIFConfig)
	tests := []struct {
		name    string
		auth    Auth
		keyless bool
		wantErr string
	}{
		{"key file (2.x default)", Auth{ServiceAccountKey: key}, false, ""},
		{"keyless with ADC", Auth{ImpersonateServiceAccount: targetSA}, true, ""},
		{"keyless with WIF configuration", Auth{ImpersonateServiceAccount: targetSA, WIFConfig: wif}, true, ""},
		{"both methods", Auth{ServiceAccountKey: key, ImpersonateServiceAccount: targetSA}, true, "both a service-account key and a service account to impersonate"},
		{"neither method", Auth{}, false, "no service-account key and no service account to impersonate"},
		{"WIF without impersonation", Auth{ServiceAccountKey: key, WIFConfig: wif}, false, "without a service account to impersonate"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.keyless, tc.auth.Keyless())
			err := tc.auth.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestNewClient_KeylessWithWIFConfig(t *testing.T) {
	client, err := NewClient(context.Background(), "", testCustomer,
		Auth{ImpersonateServiceAccount: targetSA, WIFConfig: []byte(awsWIFConfig)})
	require.NoError(t, err)
	assert.NotNil(t, client)
}

// Only an external_account (Workload Identity Federation) configuration is
// accepted as the WIF configuration; a service-account key passed there by
// mistake is rejected instead of silently used.
func TestNewClient_KeylessRejectsNonWIFCredentials(t *testing.T) {
	client, err := NewClient(context.Background(), "", testCustomer,
		Auth{ImpersonateServiceAccount: targetSA, WIFConfig: []byte(serviceAccountKeyJSON(t))})
	assert.ErrorContains(t, err, `must be of type "external_account", got "service_account"`)
	assert.Nil(t, client)

	_, err = NewClient(context.Background(), "", testCustomer,
		Auth{ImpersonateServiceAccount: targetSA, WIFConfig: []byte("not json")})
	assert.ErrorContains(t, err, "not valid JSON")
}

func TestNewClient_RejectsInvalidAuth(t *testing.T) {
	client, err := NewClient(context.Background(), "admin@example.com", "my_customer", Auth{})
	assert.Error(t, err)
	assert.Nil(t, client)
}

// The key path is unchanged: a service-account key builds a client offline.
func TestNewClient_ServiceAccountKey(t *testing.T) {
	client, err := NewClient(context.Background(), "admin@example.com", "my_customer",
		Auth{ServiceAccountKey: []byte(serviceAccountKeyJSON(t))})
	require.NoError(t, err)
	assert.NotNil(t, client)
}

// Keyless authentication always needs the customer ID (not my_customer), with
// or without an admin; the key needs it only without an admin (issue #192).
func TestNewClient_CustomerIDRules(t *testing.T) {
	keyless := Auth{ImpersonateServiceAccount: targetSA, WIFConfig: []byte(awsWIFConfig)}
	key := Auth{ServiceAccountKey: []byte(serviceAccountKeyJSON(t))}
	tests := []struct {
		name            string
		admin, customer string
		auth            Auth
		wantErr         string
	}{
		{"keyless, my_customer", "", "my_customer", keyless, "keyless authentication (Workload Identity Federation) requires the Google Workspace customer ID"},
		{"keyless, admin, my_customer", "admin@example.com", "my_customer", keyless, "keyless authentication (Workload Identity Federation) requires the Google Workspace customer ID"},
		{"keyless, empty customer", "", "", keyless, "requires the Google Workspace customer ID"},
		{"keyless, customer ID", "", testCustomer, keyless, ""},
		{"key, admin, my_customer (2.x default)", "admin@example.com", "my_customer", key, ""},
		{"key, no admin, my_customer", "", "my_customer", key, "without a Google admin to act on behalf of"},
		{"key, no admin, customer ID", "", testCustomer, key, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewClient(context.Background(), tc.admin, tc.customer, tc.auth)
			if tc.wantErr == "" {
				require.NoError(t, err)
				assert.NotNil(t, c)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Nil(t, c)
			}
		})
	}
}
