package config

import (
	"encoding/base64"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	log "github.com/sirupsen/logrus"
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
func (s *Secrets) GoogleAdminEmail() (string, error) {
	return s.getSecret("SSOSyncGoogleAdminEmail", true)
}

// SCIMAccessToken ...
func (s *Secrets) SCIMAccessToken() (string, error) {
	return s.getSecret("SSOSyncSCIMAccessToken", false)
}

// SCIMEndpointUrl ...
func (s *Secrets) SCIMEndpointUrl() (string, error) {
	return s.getSecret("SSOSyncSCIMEndpointUrl", false)
}

// GoogleCredentials ...
func (s *Secrets) GoogleCredentials() (string, error) {
	return s.getSecret("SSOSyncGoogleCredentials", false)
}

func (s *Secrets) getSecret(secretKey string, optional bool) (string, error) {
	r, err := s.svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretKey),
		VersionStage: aws.String("AWSCURRENT"),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == secretsmanager.ErrCodeResourceNotFoundException && optional {
				log.WithFields(log.Fields{
					"secretname": secretKey,
				}).Info("Secret not found, ignoring optional secret")

				return "", nil
			}
		}

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
