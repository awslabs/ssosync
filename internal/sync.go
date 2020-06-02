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

package internal

import (
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	admin "google.golang.org/api/admin/directory/v1"

	"github.com/awslabs/ssosync/internal/aws"
	"github.com/awslabs/ssosync/internal/google"
)

// ISyncGSuite is the interface for synchronising users/groups
type ISyncGSuite interface {
	SyncUsers() error
	SyncGroups() error
}

// SyncGSuite is an object type that will synchronise real users and groups
type SyncGSuite struct {
	aws    aws.IClient
	google google.IClient
	logger *zap.Logger

	users map[string]*aws.User
}

// New will create a new SyncGSuite object
func New(logger *zap.Logger, a aws.IClient, g google.IClient) ISyncGSuite {
	return &SyncGSuite{
		aws:    a,
		google: g,
		logger: logger,
		users:  make(map[string]*aws.User),
	}
}

// SyncUsers will Sync Google Users to AWS SSO SCIM
func (s *SyncGSuite) SyncUsers() error {
	s.logger.Info("Start user sync")
	s.logger.Debug("Get AWS Users")
	awsUsers, err := s.aws.GetUsers()
	if err != nil {
		return err
	}
	s.logger.Debug("Get Google Users")
	googleUsers, err := s.google.GetUsers()
	if err != nil {
		return err
	}

	for _, u := range googleUsers {
		logUser := []zap.Field{
			zap.String("email", u.PrimaryEmail),
		}

		s.logger.Debug("Check user", logUser...)
		if awsUser, ok := (*awsUsers)[u.PrimaryEmail]; ok {
			s.logger.Debug("Found user", logUser...)
			s.users[awsUser.Username] = &awsUser
		} else {
			s.logger.Info("Create user in AWS", logUser...)
			newUser, err := s.aws.CreateUser(aws.NewUser(
				u.Name.GivenName,
				u.Name.FamilyName,
				u.PrimaryEmail,
			))
			if err != nil {
				return err
			}
			s.users[newUser.Username] = newUser
		}
	}

	s.logger.Info("Clean up AWS Users")
	for _, u := range *awsUsers {
		if _, ok := s.users[u.Username]; !ok {
			s.logger.Info("Delete User in AWS", zap.String("email", u.Username))
			err := s.aws.DeleteUser(&u)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// SyncGroups will sync groups from Google -> AWS SSO
func (s *SyncGSuite) SyncGroups() error {
	s.logger.Info("Start group sync")

	s.logger.Debug("Get AWS Groups")
	awsGroups, err := s.aws.GetGroups()
	if err != nil {
		return err
	}

	s.logger.Debug("Get Google Groups")
	googleGroups, err := s.google.GetGroups()
	if err != nil {
		return err
	}

	correlatedGroups := make(map[string]*aws.Group)

	for _, g := range googleGroups {
		logGroup := []zap.Field{
			zap.String("group", g.Name),
		}

		s.logger.Debug("Check group", logGroup...)

		var group *aws.Group

		if awsGroup, ok := (*awsGroups)[g.Name]; ok {
			s.logger.Debug("Found group", logGroup...)
			correlatedGroups[awsGroup.DisplayName] = &awsGroup
			group = &awsGroup
		} else {
			s.logger.Info("Creating group in AWS", logGroup...)
			newGroup, err := s.aws.CreateGroup(aws.NewGroup(g.Name))
			if err != nil {
				return err
			}
			correlatedGroups[newGroup.DisplayName] = newGroup
			group = newGroup
		}

		groupMembers, err := s.google.GetGroupMembers(g)
		if err != nil {
			return err
		}

		memberList := make(map[string]*admin.Member)

		s.logger.Info("Start group user sync", logGroup...)

		for _, m := range groupMembers {
			if _, ok := s.users[m.Email]; ok {
				memberList[m.Email] = m
			}
		}

		for _, u := range s.users {
			logDetail := append(logGroup, zap.String("user", u.Username))

			s.logger.Debug("Checking user is in group already", logDetail...)
			b, err := s.aws.IsUserInGroup(u, group)
			if err != nil {
				return err
			}

			if _, ok := memberList[u.Username]; ok {
				if !b {
					s.logger.Info("Adding user to group", logDetail...)
					err := s.aws.AddUserToGroup(u, group)
					if err != nil {
						return err
					}
				}
			} else {
				if b {
					s.logger.Info("Removing user from group", logDetail...)
					err := s.aws.RemoveUserFromGroup(u, group)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	s.logger.Info("Clean up AWS groups")
	for _, g := range *awsGroups {
		if _, ok := correlatedGroups[g.DisplayName]; !ok {
			s.logger.Info("Delete Group in AWS", zap.String("group", g.DisplayName))
			err := s.aws.DeleteGroup(&g)
			if err != nil {
				return err
			}
		}
	}

	s.logger.Info("Done sync groups")
	return nil
}

// QuietLogSync will squash logging errors when calling
// sync on the logger.
func QuietLogSync(l *zap.Logger) {
	err := l.Sync()
	if err != nil {
		return
	}
}

// DoSync will create a logger and run the sync with the paths
// given to do the sync.
func DoSync(debug bool, credPath string, tokenPath string, awsTomlPath string) error {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	if debug {
		config.Level.SetLevel(zap.DebugLevel)
	} else {
		config.Level.SetLevel(zap.InfoLevel)
	}

	logger, _ := config.Build()
	defer QuietLogSync(logger)

	logger.Info("Creating the Google and AWS Clients needed")

	googleAuthClient, err := google.NewAuthClient(logger, credPath, tokenPath)
	if err != nil {
		logger.Fatal("Failed to create Google Auth Client", zap.Error(err))
	}

	googleClient, err := google.NewClient(logger, googleAuthClient)
	if err != nil {
		logger.Fatal("Failed to create Google Client", zap.Error(err))
	}

	awsConfig, err := aws.ReadConfigFromFile(awsTomlPath)
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

	c := New(logger, awsClient, googleClient)
	err = c.SyncUsers()
	if err != nil {
		return err
	}

	err = c.SyncGroups()
	if err != nil {
		return err
	}

	return nil
}
