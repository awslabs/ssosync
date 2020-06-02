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
	"fmt"
	"log"
	"os"

	"github.com/awslabs/ssosync/internal"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/spf13/cobra"
)

var (
	googleCredPath  string
	googleTokenPath string
	scimConfig      string

	logDebug bool
)

var rootCmd = &cobra.Command{
	Version: "dev",
	Use:     "ssosync",
	Short:   "SSO Sync, making AWS SSO be populated automagically",
	Long: `A command line tool to enable you to synchronise your Google
Apps (G-Suite) users to AWS Single Sign-on (AWS SSO)
Complete documentation is available at https://github.com/awslabs/ssosync`,
	Run: func(cmd *cobra.Command, args []string) {
		err := internal.DoSync(logDebug, googleCredPath, googleTokenPath, scimConfig)
		if err != nil {
			log.Fatal(err)
		}
	},
}

// Execute is the entry point of the command. If we are
// running inside of AWS Lambda, we use the Lambda
// execution path.
func Execute(v string) {
	if !inLambda() {
		rootCmd.SetVersionTemplate(v)
		rootCmd.AddCommand(googleCmd)

		if err := rootCmd.Execute(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		lambda.Start(lambdaHandler)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&googleCredPath, "googleCredentialsPath", "c", "credentials.json", "set the path to find credentials for Google")
	rootCmd.Flags().StringVarP(&googleTokenPath, "googleTokenPath", "t", "token.json", "set the path to find token for Google")
	rootCmd.Flags().StringVarP(&scimConfig, "scimConfig", "s", "aws.toml", "AWS SSO SCIM Configuration")
	rootCmd.Flags().BoolVarP(&logDebug, "debug", "d", false, "Enable verbose / debug logging")
}
