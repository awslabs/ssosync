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

// Package cmd ...
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/awslabs/ssosync/internal"
	"github.com/awslabs/ssosync/internal/config"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Version: "dev",
	Use:     "ssosync",
	Short:   "SSO Sync, making AWS SSO be populated automagically",
	Long: `A command line tool to enable you to synchronise your Google
Apps (G-Suite) users to AWS Single Sign-on (AWS SSO)
Complete documentation is available at https://github.com/awslabs/ssosync`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := internal.DoSync(ctx, cfg)
		if err != nil {
			return err
		}

		return nil
	},
}

// Execute is the entry point of the command. If we are
// running inside of AWS Lambda, we use the Lambda
// execution path.
func Execute() {
	if cfg.IsLambda {
		lambda.Start(rootCmd.Execute)
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	// init config
	cfg = config.New()
	cfg.IsLambda = len(os.Getenv("_LAMBDA_SERVER_PORT")) > 0

	// initialize cobra
	cobra.OnInitialize(initConfig)
	addFlags(rootCmd, cfg)

	rootCmd.SetVersionTemplate(fmt.Sprintf("%s, commit %s, built at %s by %s\n", version, commit, date, builtBy))

	// silence on the root cmd
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// allow to read in from environment
	viper.SetEnvPrefix("ssosync")
	viper.AutomaticEnv()

	appEnvVars := []string{
		"google_admin",
		"google_credentials",
		"scim_access_token",
		"scim_endpoint",
		"log_level",
		"log_format",
		"ignore_users",
		"ignore_groups",
		"include_groups",
		"user_match",
		"group_match",
		"sync_method",
		"dry_run",
	}

	for _, e := range appEnvVars {
		if err := viper.BindEnv(e); err != nil {
			log.Fatalf(errors.Wrap(err, "cannot bind environment variable").Error())
		}
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf(errors.Wrap(err, "cannot unmarshal config").Error())
	}

	// config logger
	logConfig(cfg)
	log.Debug(cfg)

	if cfg.IsLambda {
		configLambda()
	}
}

func configLambda() {
	s := session.Must(session.NewSession())
	svc := secretsmanager.New(s)
	secrets := config.NewSecrets(svc)

	var skey string

	if skey = cfg.GoogleAdmin; skey == "" {
		skey = "SSOSyncGoogleAdminEmail"
	}
	unwrap, err := secrets.GetSecret(skey)
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.GoogleAdmin = unwrap

	if skey = cfg.GoogleCredentials; skey == "" {
		skey = "SSOSyncGoogleCrednetials"
	}
	unwrap, err = secrets.GetSecret(skey)
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.GoogleCredentials = unwrap

	if skey = cfg.SCIMAccessToken; skey == "" {
		skey = "SSOSyncAccessToken"
	}
	unwrap, err = secrets.GetSecret(skey)
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.SCIMAccessToken = unwrap

	if skey = cfg.SCIMEndpoint; skey == "" {
		skey = "SSOSyncSCMEndPoint"
	}
	unwrap, err = secrets.GetSecret(skey)
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.SCIMEndpoint = unwrap
}

func addFlags(cmd *cobra.Command, cfg *config.Config) {
	rootCmd.PersistentFlags().StringVarP(&cfg.GoogleCredentials, "google-admin", "a", config.DefaultGoogleCredentials, "path to find credentials file for Google")
	rootCmd.PersistentFlags().BoolVarP(&cfg.Debug, "debug", "d", config.DefaultDebug, "enable verbose / debug logging")
	rootCmd.PersistentFlags().StringVarP(&cfg.LogFormat, "log-format", "", config.DefaultLogFormat, "log format")
	rootCmd.PersistentFlags().StringVarP(&cfg.LogLevel, "log-level", "", config.DefaultLogLevel, "log level")
	rootCmd.PersistentFlags().BoolVarP(&cfg.DryRun, "dry-run", "", config.DefaultDryRun, "dry run")
	rootCmd.Flags().StringVarP(&cfg.SCIMAccessToken, "access-token", "t", "", "AWS SCIM Access Token")
	rootCmd.Flags().StringVarP(&cfg.SCIMEndpoint, "endpoint", "e", "", "AWS SCIM Endpoint")
	rootCmd.Flags().StringVarP(&cfg.GoogleCredentials, "google-credentials", "c", config.DefaultGoogleCredentials, "path to find credentials file for Google")
	rootCmd.Flags().StringVarP(&cfg.GoogleAdmin, "google-admin", "u", "", "Google admin user email")
	rootCmd.Flags().StringSliceVar(&cfg.IgnoreUsers, "ignore-users", []string{}, "ignores these Google users")
	rootCmd.Flags().StringSliceVar(&cfg.IgnoreGroups, "ignore-groups", []string{}, "ignores these Google groups")
	rootCmd.Flags().StringSliceVar(&cfg.IncludeGroups, "include-groups", []string{}, "include only these Google groups")
	rootCmd.Flags().StringVarP(&cfg.UserMatch, "user-match", "m", "", "Google users query parameter, example: 'name:John* email:admin*', see: https://developers.google.com/admin-sdk/directory/v1/guides/search-users")
	rootCmd.Flags().StringVarP(&cfg.GroupMatch, "group-match", "g", "", "Google groups query parameter, example: 'name:Admin* email:aws-*', see: https://developers.google.com/admin-sdk/directory/v1/guides/search-groups")
	rootCmd.Flags().StringVarP(&cfg.SyncMethod, "sync-method", "s", config.DefaultSyncMethod, "Select the sync method to use (users_groups|groups)")
}

func logConfig(cfg *config.Config) {
	// reset log format
	if cfg.LogFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	if cfg.Debug {
		cfg.LogLevel = "debug"
	}

	// set the configured log level
	if level, err := log.ParseLevel(cfg.LogLevel); err == nil {
		log.SetLevel(level)
	}
}
