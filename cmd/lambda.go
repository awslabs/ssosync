// Copyright (c) 2020, Amazon.com, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/awslabs/ssosync/internal"
	"github.com/awslabs/ssosync/internal/config"
)

// inLambda detects if we are running Lambda and will
// return true if we are
func inLambda() bool {
	return len(os.Getenv("_LAMBDA_SERVER_PORT")) > 0
}

// writeSecretToFile will write the given secret out to a temporary file
// prefixed with 'name'.
func writeSecretToFile(secretKey string, name string) (string, error) {
	s := session.Must(session.NewSession())
	svc := secretsmanager.New(s)

	r, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
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

	f, err := ioutil.TempFile("", fmt.Sprintf("%s-*", name))

	if err != nil {
		return "", err
	}

	if _, err = f.WriteString(secretString); err != nil {
		return "", err
	}

	if err := f.Close(); err != nil {
		return "", err
	}

	return f.Name(), nil
}

// removeFileSilently handles the fact we want to delete temporary
// files - but we don't care if it fails. So we can use it in
// defer without warnings.
func removeFileSilently(name string) {
	_ = os.Remove(name)
}

// lambdaHandler is the Lambda entry point
func lambdaHandler(cfg *config.Config) func() error {
	return func() error {
		if err := internal.DoSync(cfg); err != nil {
			return err
		}

		return nil
	}
}
