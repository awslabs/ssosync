package config

import (
	"encoding/base64"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

// Secrets ...
type Secrets struct {
	svc *secretsmanager.SecretsManager
}

// NewSecrets ...
func NewSecrets(svc *secretsmanager.SecretsManager) *Secrets {
	return &Secrets{
		svc: svc,
	}
}

// GoogleAdminEmail ...
func (s *Secrets) GoogleAdminEmail(secretArn string) (string, error) {
     if len([]rune(secretArn)) == 0 {
        return s.getSecret("SSOSyncGoogleAdminEmail")
     } 
     return s.getSecret(secretArn)
}

// SCIMAccessToken ...
func (s *Secrets) SCIMAccessToken(secretArn string) (string, error) {
     if len([]rune(secretArn)) == 0 {
        return s.getSecret("SSOSyncSCIMAccessToken")
     }
     return s.getSecret(secretArn)
}

// SCIMEndpointURL ...
func (s *Secrets) SCIMEndpointURL(secretArn string) (string, error) {
     if len([]rune(secretArn)) == 0 {
        return s.getSecret("SSOSyncSCIMEndpointURL")
     }
     return s.getSecret(secretArn)
}

// GoogleCredentials ...
func (s *Secrets) GoogleCredentials(secretArn string) (string, error) {
     if len([]rune(secretArn)) == 0 {
        return s.getSecret("SSOSyncGoogleCredentials")
     }
     return s.getSecret(secretArn)
}

// Region ...
func (s *Secrets) Region(secretArn string) (string, error) {
     if len([]rune(secretArn)) == 0 {
        return s.getSecret("SSOSyncRegion")
     }
     return s.getSecret(secretArn)
}

// IdentityStoreID ...
func (s *Secrets) IdentityStoreID(secretArn string) (string, error) {
     if len([]rune(secretArn)) == 0 {
        return s.getSecret("IdentityStoreID")
     }
     return s.getSecret(secretArn)
}

func (s *Secrets) getSecret(secretKey string) (string, error) {
	r, err := s.svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretKey),
		VersionStage: aws.String("AWSCURRENT"),
	})

	if err != nil {
		return "", err
	}

	var secretString string

	if r.SecretString != nil {
		secretString = *r.SecretString
	} else {
		decodedBinarySecretBytes := make([]byte, base64.StdEncoding.DecodedLen(len(r.SecretBinary)))
		l, err := base64.StdEncoding.Decode(decodedBinarySecretBytes, r.SecretBinary)
		if err != nil {
			return "", err
		}
		secretString = string(decodedBinarySecretBytes[:l])
	}

	return secretString, nil
}



