package internal

import (
	"os"
	"path/filepath"
	"testing"

	"ssosync/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testKeyJSON = `{"type":"service_account","client_email":"sa@p.iam.gserviceaccount.com"}`
	testWIFJSON = `{"type":"external_account","audience":"//iam.googleapis.com/projects/1/locations/global/workloadIdentityPools/p/providers/aws"}`
	testTarget  = "ssosync@project.iam.gserviceaccount.com"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

// CLI, key file (the 2.x default): the key file is read; no impersonation.
func TestGoogleAuthFromConfig_CLIKeyFile(t *testing.T) {
	cfg := &config.Config{GoogleCredentials: writeFile(t, "credentials.json", testKeyJSON)}
	auth, err := googleAuthFromConfig(cfg)
	require.NoError(t, err)
	assert.False(t, auth.Keyless())
	assert.Equal(t, testKeyJSON, string(auth.ServiceAccountKey))
	assert.Empty(t, auth.WIFConfig)
}

// CLI, keyless: the WIF configuration file is read and the key file is not
// read at all (cmd/root.go clears the "credentials.json" default).
func TestGoogleAuthFromConfig_CLIKeylessWithWIFFile(t *testing.T) {
	cfg := &config.Config{
		GoogleImpersonateServiceAccount: testTarget,
		GoogleWIFConfig:                 writeFile(t, "wif.json", testWIFJSON),
	}
	auth, err := googleAuthFromConfig(cfg)
	require.NoError(t, err)
	assert.True(t, auth.Keyless())
	assert.Equal(t, testTarget, auth.ImpersonateServiceAccount)
	assert.Equal(t, testWIFJSON, string(auth.WIFConfig))
	assert.Empty(t, auth.ServiceAccountKey)
}

// CLI, keyless with Application Default Credentials: no file needed at all.
func TestGoogleAuthFromConfig_CLIKeylessADC(t *testing.T) {
	auth, err := googleAuthFromConfig(&config.Config{GoogleImpersonateServiceAccount: testTarget})
	require.NoError(t, err)
	assert.True(t, auth.Keyless())
	assert.Empty(t, auth.WIFConfig)
}

// CLI, keyless while a key file was also given explicitly: rejected.
func TestGoogleAuthFromConfig_CLIBothMethods(t *testing.T) {
	_, err := googleAuthFromConfig(&config.Config{
		GoogleCredentials:               writeFile(t, "credentials.json", testKeyJSON),
		GoogleImpersonateServiceAccount: testTarget,
	})
	assert.ErrorContains(t, err, "both a service-account key and a service account to impersonate")
}

func TestGoogleAuthFromConfig_CLIMissingFiles(t *testing.T) {
	_, err := googleAuthFromConfig(&config.Config{GoogleCredentials: filepath.Join(t.TempDir(), "absent.json")})
	assert.ErrorContains(t, err, "cannot read the Google credentials file")

	_, err = googleAuthFromConfig(&config.Config{
		GoogleImpersonateServiceAccount: testTarget,
		GoogleWIFConfig:                 filepath.Join(t.TempDir(), "absent.json"),
	})
	assert.ErrorContains(t, err, "cannot read the Workload Identity Federation configuration file")
}

// A WIF configuration without a service account to impersonate is rejected,
// rather than silently ignored in favour of the key.
func TestGoogleAuthFromConfig_WIFWithoutImpersonation(t *testing.T) {
	_, err := googleAuthFromConfig(&config.Config{
		GoogleCredentials: writeFile(t, "credentials.json", testKeyJSON),
		GoogleWIFConfig:   writeFile(t, "wif.json", testWIFJSON),
	})
	assert.ErrorContains(t, err, "without a service account to impersonate")
}

// Lambda: the values already hold the contents (resolved by cmd/root.go from
// the environment, Secrets Manager or SSM), so nothing is read from disk.
func TestGoogleAuthFromConfig_Lambda(t *testing.T) {
	auth, err := googleAuthFromConfig(&config.Config{IsLambda: true, GoogleCredentials: testKeyJSON})
	require.NoError(t, err)
	assert.Equal(t, testKeyJSON, string(auth.ServiceAccountKey))

	auth, err = googleAuthFromConfig(&config.Config{
		IsLambda:                        true,
		GoogleImpersonateServiceAccount: testTarget,
		GoogleWIFConfig:                 testWIFJSON,
	})
	require.NoError(t, err)
	assert.True(t, auth.Keyless())
	assert.Equal(t, testWIFJSON, string(auth.WIFConfig))
	assert.Empty(t, auth.ServiceAccountKey)
}
