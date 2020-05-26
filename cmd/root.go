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
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/awslabs/ssosync/internal"
	"github.com/awslabs/ssosync/internal/aws"
	"github.com/awslabs/ssosync/internal/google"
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
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		if logDebug {
			config.Level.SetLevel(zap.DebugLevel)
		} else {
			config.Level.SetLevel(zap.InfoLevel)
		}

		logger, _ := config.Build()
		defer quietLogSync(logger)

		logger.Info("Creating the Google and AWS Clients needed")

		googleAuthClient, err := google.NewAuthClient(logger, googleCredPath, googleTokenPath)
		if err != nil {
			logger.Fatal("Failed to create Google Auth Client", zap.Error(err))
		}

		googleClient, err := google.NewClient(logger, googleAuthClient)
		if err != nil {
			logger.Fatal("Failed to create Google Client", zap.Error(err))
		}

		awsConfig, err := aws.ReadConfigFromFile(scimConfig)
		if err != nil {
			logger.Fatal("Failed to read AWS Config", zap.Error(err))
		}

		awsClient, err := aws.NewClient(
			logger,
			&http.Client{},
			awsConfig)
		if err != nil {
			logger.Fatal("Failed to create awsClient", zap.Error(err))
		}

		c := internal.New(logger, awsClient, googleClient)
		err = c.SyncUsers()
		if err != nil {
			log.Fatal(err)
		}

		err = c.SyncGroups()
		if err != nil {
			log.Fatal(err)
		}
	},
}

// Execute is the entry point of the command
func Execute(v string) {
	rootCmd.SetVersionTemplate(v)
	rootCmd.AddCommand(googleCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&googleCredPath, "googleCredentialsPath", "c", "credentials.json", "set the path to find credentials for Google")
	rootCmd.Flags().StringVarP(&googleTokenPath, "googleTokenPath", "t", "token.json", "set the path to find token for Google")
	rootCmd.Flags().StringVarP(&scimConfig, "scimConfig", "s", "aws.toml", "AWS SSO SCIM Configuration")
	rootCmd.Flags().BoolVarP(&logDebug, "debug", "d", false, "Enable verbose / debug logging")
}

func quietLogSync(l *zap.Logger) {
	err := l.Sync()
	if err != nil {
		return
	}
}
