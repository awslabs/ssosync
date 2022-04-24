package config

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// GoogleAdminEmail ...
func GoogleAdminEmail(svc *ssm.SSM) (string, error) {

	input := &ssm.GetParameterInput{
		Name:           aws.String(SSMParameterNameGoogleAdmin),
		WithDecryption: aws.Bool(true),
	}

	output, err := svc.GetParameter(input)
	if err != nil {
		return "", err
	}

	return *output.Parameter.Value, err

}

// SCIMAccessToken ...
func SCIMAccessToken(svc *ssm.SSM) (string, error) {
	input := &ssm.GetParameterInput{
		Name:           aws.String(SSMParameterNameScimAccessToken),
		WithDecryption: aws.Bool(true),
	}

	output, err := svc.GetParameter(input)
	if err != nil {
		return "", err
	}

	return *output.Parameter.Value, err
}

// SCIMEndpointUrl ...
func SCIMEndpointUrl(svc *ssm.SSM) (string, error) {
	input := &ssm.GetParameterInput{
		Name:           aws.String(SSMParameterNameScimEndpointUrl),
		WithDecryption: aws.Bool(true),
	}

	output, err := svc.GetParameter(input)
	if err != nil {
		return "", err
	}

	return *output.Parameter.Value, err
}

// GoogleCredentials ...
func GoogleCredentials(svc *ssm.SSM) (string, error) {
	input := &ssm.GetParameterInput{
		Name:           aws.String(SSMParameterNameGoogleCredentials),
		WithDecryption: aws.Bool(true),
	}

	output, err := svc.GetParameter(input)
	if err != nil {
		return "", err
	}

	return *output.Parameter.Value, err
}
