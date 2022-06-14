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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-lambda-go/events"
        "github.com/aws/aws-sdk-go/service/codepipeline"
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
Apps (Google Workspace) users to AWS Single Sign-on (AWS SSO)
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
                log.Info("Executing as Lambda")
        	lambda.Start(Handler) 
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func Handler(ctx context.Context, event events.CodePipelineEvent) (string, error) {
    log.Debug(event)
    err := rootCmd.Execute()
    s := session.Must(session.NewSession())
    cpl := codepipeline.New(s)

    cfg.IsLambdaRunningInCodePipeline = len(event.CodePipelineJob.ID) > 0

    if cfg.IsLambdaRunningInCodePipeline {
        log.Info("Lambda has been invoked by CodePipeline")

        if err != nil {
    	    // notify codepipeline and mark its job execution as Failure
    	    log.Fatalf(errors.Wrap(err, "Notifying CodePipeline and mark its job execution as Failure").Error())
    	    jobID := event.CodePipelineJob.ID
    	    if len(jobID) == 0 {
    		panic("CodePipeline Job ID is not set")
    	    }  
    	    // mark the job as Failure.
    	    cplFailure := &codepipeline.PutJobFailureResultInput{
    		JobId: aws.String(jobID),
    		FailureDetails: &codepipeline.FailureDetails{
    			Message: aws.String(err.Error()),
    			Type: aws.String("JobFailed"),
    		},
    	    }
    	    _, cplErr := cpl.PutJobFailureResult(cplFailure)
    	    if cplErr != nil {
                log.Fatalf(errors.Wrap(err, "Failed to update CodePipeline jobID status").Error())
    	    }
    	    return "Failure", err
        } else {
            log.Info("Notifying CodePipeline and mark its job execution as Success")
            jobID := event.CodePipelineJob.ID
            if len(jobID) == 0 {
    	       panic("CodePipeline Job ID is not set")
            }
            // mark the job as Success.
            cplSuccess := &codepipeline.PutJobSuccessResultInput{
    	       JobId: aws.String(jobID),
            }
            _, cplErr := cpl.PutJobSuccessResult(cplSuccess)
            if cplErr != nil {
                log.Fatalf(errors.Wrap(err, "Failed to update CodePipeline jobID status").Error())
            }
            return "Success", nil
        }
    } else {
        if err != nil {
            log.Fatalf(errors.Wrap(err, "Notifying Lambda and mark this execution as Failure").Error())
            return "Failure", err
        } else {
            return "Success", nil
        }
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
		"region",
		"identity_store_id",
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

	if cfg.IsLambda {
		configLambda()
	}
}

func configLambda() {
	s := session.Must(session.NewSession())
	svc := secretsmanager.New(s)
	secrets := config.NewSecrets(svc)

	unwrap, err := secrets.GoogleAdminEmail()
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.GoogleAdmin = unwrap

	unwrap, err = secrets.GoogleCredentials()
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.GoogleCredentials = unwrap

	unwrap, err = secrets.SCIMAccessToken()
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.SCIMAccessToken = unwrap

	unwrap, err = secrets.SCIMEndpointUrl()
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.SCIMEndpoint = unwrap

	unwrap, err = secrets.Region()
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.Region = unwrap

	unwrap, err = secrets.IdentityStoreID()
	if err != nil {
		log.Fatalf(errors.Wrap(err, "cannot read config").Error())
	}
	cfg.IdentityStoreID = unwrap
}

func addFlags(cmd *cobra.Command, cfg *config.Config) {
	rootCmd.PersistentFlags().StringVarP(&cfg.GoogleCredentials, "google-admin", "a", config.DefaultGoogleCredentials, "path to find credentials file for Google Workspace")
	rootCmd.PersistentFlags().BoolVarP(&cfg.Debug, "debug", "d", config.DefaultDebug, "enable verbose / debug logging")
	rootCmd.PersistentFlags().StringVarP(&cfg.LogFormat, "log-format", "", config.DefaultLogFormat, "log format")
	rootCmd.PersistentFlags().StringVarP(&cfg.LogLevel, "log-level", "", config.DefaultLogLevel, "log level")
	rootCmd.Flags().StringVarP(&cfg.SCIMAccessToken, "access-token", "t", "", "AWS SSO SCIM API Access Token")
	rootCmd.Flags().StringVarP(&cfg.SCIMEndpoint, "endpoint", "e", "", "AWS SSO SCIM API Endpoint")
	rootCmd.Flags().StringVarP(&cfg.GoogleCredentials, "google-credentials", "c", config.DefaultGoogleCredentials, "path to Google Workspace credentials file")
	rootCmd.Flags().StringVarP(&cfg.GoogleAdmin, "google-admin", "u", "", "Google Workspace admin user email")
	rootCmd.Flags().StringSliceVar(&cfg.IgnoreUsers, "ignore-users", []string{}, "ignores these Google Workspace users")
	rootCmd.Flags().StringSliceVar(&cfg.IgnoreGroups, "ignore-groups", []string{}, "ignores these Google Workspace groups")
	rootCmd.Flags().StringSliceVar(&cfg.IncludeGroups, "include-groups", []string{}, "include only these Google Workspace groups, NOTE: only works when --sync-method 'users_groups'")
	rootCmd.Flags().StringSliceVarP(&cfg.UserMatch, "user-match", "m", []string{}, "Google Workspace Users filter query parameter, example: 'name:John* email:admin*', see: https://developers.google.com/admin-sdk/directory/v1/guides/search-users")
	rootCmd.Flags().StringSliceVarP(&cfg.GroupMatch, "group-match", "g", []string{}, "Google Workspace Groups filter query parameter, example: 'name:Admin* email:aws-*', see: https://developers.google.com/admin-sdk/directory/v1/guides/search-groups")
	rootCmd.Flags().StringVarP(&cfg.SyncMethod, "sync-method", "s", config.DefaultSyncMethod, "Sync method to use (users_groups|groups)")
	rootCmd.Flags().StringVarP(&cfg.Region, "region", "r", "", "AWS Region where AWS SSO is enabled")
	rootCmd.Flags().StringVarP(&cfg.IdentityStoreID, "identity-store-id", "i", "", "Identifier of Identity Store in AWS SSO")
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
