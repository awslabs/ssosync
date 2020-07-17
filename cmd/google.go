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

	"github.com/awslabs/ssosync/internal/google"
)

var googleCmd = &cobra.Command{
	Use:   "google",
	Short: "Log in to Google",
	Long:  `Log in to Google - use me to generate the files needed for the main command`,
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := google.NewAuthClient(cfg.GoogleCredentialsPath, cfg.GoogleTokenPath)
		if err != nil {
			return err
		}

		if _, err := g.GetTokenFromWeb(); err != nil {
			return err
		}

		return nil
	},
}
