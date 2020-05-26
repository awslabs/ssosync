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
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/awslabs/ssosync/internal/google"
)

var googleCmd = &cobra.Command{
	Use:   "google",
	Short: "Log in to Google",
	Long:  `Log in to Google - use me to generate the files needed for the main command`,
	Run: func(cmd *cobra.Command, args []string) {
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.Level.SetLevel(zap.DebugLevel)
		logger, _ := config.Build()
		defer quietLogSync(logger)

		credPath, err := cmd.Flags().GetString("path")
		if err != nil {
			logger.Fatal("No path available", zap.Error(err))
		}
		tokenPath, err := cmd.Flags().GetString("tokenPath")
		if err != nil {
			logger.Fatal("No tokenPath available", zap.Error(err))
		}

		g, err := google.NewAuthClient(logger, credPath, tokenPath)
		if err != nil {
			logger.Fatal("Unable to create google auth client", zap.Error(err))
		}

		g.GetTokenFromWeb()
	},
}

func init() {
	googleCmd.Flags().String("path", "credentials.json", "set the path to find credentials")
	googleCmd.Flags().String("tokenPath", "token.json", "set the path to put token.json output into")
}
