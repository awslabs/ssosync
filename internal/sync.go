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
	"io/ioutil"
	"net/http"

	"github.com/awslabs/ssosync/internal/aws"
	"github.com/awslabs/ssosync/internal/config"
	"github.com/awslabs/ssosync/internal/google"
	"go.uber.org/zap"

	log "github.com/sirupsen/logrus"
	admin "google.golang.org/api/admin/directory/v1"
)

// ISyncGSuite is the interface for synchronising users/groups
type ISyncGSuite interface {
	SyncUsers() error
	SyncGroups() error
}

// SyncGSuite is an object type that will synchronise real users and groups
type SyncGSuite struct {
	aws    aws.IClient
	google google.Client

	users map[string]*aws.User
}

// New will create a new SyncGSuite object
func New(a aws.IClient, g google.Client) ISyncGSuite {
	return &SyncGSuite{
		aws:    a,
		google: g,
		users:  make(map[string]*aws.User),
	}
}

// SyncUsers will Sync Google Users to AWS SSO SCIM
func (s *SyncGSuite) SyncUsers() error {
	log.Info("Start user sync")
	log.Info("Get AWS Users")

	awsUsers, err := s.aws.GetUsers()
	if err != nil {
		return err
	}

	log.Debug("Get Google Users")
	googleUsers, err := s.google.GetUsers()
	if err != nil {
		return err
	}

	for _, u := range googleUsers {
		log := log.WithFields(log.Fields{
			"email": u.PrimaryEmail,
		})

		log.Debug("Check user")

		if awsUser, ok := (*awsUsers)[u.PrimaryEmail]; ok {
			log.Debug("Found user")
			s.users[awsUser.Username] = &awsUser
		} else {
			log.Info("Create user in AWS")
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

	log.Info("Clean up AWS Users")
	for _, u := range *awsUsers {
		if _, ok := s.users[u.Username]; !ok {
			log.WithField("email", u.Username).Info("Delete User in AWS")

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
	log.Info("Start group sync")

	log.Debug("Get AWS Groups")
	awsGroups, err := s.aws.GetGroups()
	if err != nil {
		return err
	}

	log.Debug("Get Google Groups")
	googleGroups, err := s.google.GetGroups()
	if err != nil {
		return err
	}

	correlatedGroups := make(map[string]*aws.Group)

	for _, g := range googleGroups {
		log := log.WithFields(log.Fields{
			"group": g.Name,
		})

		log.Debug("Check group")

		var group *aws.Group

		if awsGroup, ok := (*awsGroups)[g.Name]; ok {
			log.Debug("Found group")
			correlatedGroups[awsGroup.DisplayName] = &awsGroup
			group = &awsGroup
		} else {
			log.Info("Creating group in AWS")
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

		log.Info("Start group user sync")

		for _, m := range groupMembers {
			if _, ok := s.users[m.Email]; ok {
				memberList[m.Email] = m
			}
		}

		for _, u := range s.users {
			log.WithField("user", u.Username).Debug("Checking user is in group already")
			b, err := s.aws.IsUserInGroup(u, group)
			if err != nil {
				return err
			}

			if _, ok := memberList[u.Username]; ok {
				if !b {
					log.WithField("user", u.Username).Info("Adding user to group")
					err := s.aws.AddUserToGroup(u, group)
					if err != nil {
						return err
					}
				}
			} else {
				if b {
					log.WithField("user", u.Username).Info("Removing user from group")
					err := s.aws.RemoveUserFromGroup(u, group)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	log.Info("Clean up AWS groups")
	for _, g := range *awsGroups {
		if _, ok := correlatedGroups[g.DisplayName]; !ok {
			log.Info("Delete Group in AWS", zap.String("group", g.DisplayName))
			err := s.aws.DeleteGroup(&g)
			if err != nil {
				return err
			}
		}
	}

	log.Info("Done sync groups")

	return nil
}

// DoSync will create a logger and run the sync with the paths
// given to do the sync.
func DoSync(cfg *config.Config) error {
	log.Info("Creating the Google and AWS Clients needed")

	creds := []byte(cfg.GoogleCredentials)

	if !cfg.IsLambda {
		b, err := ioutil.ReadFile(cfg.GoogleCredentials)
		if err != nil {
			return err
		}
		creds = b
	}

	googleClient, err := google.NewClient(cfg.GoogleAdmin, creds)
	if err != nil {
		return err
	}

	awsClient, err := aws.NewClient(
		&http.Client{},
		&aws.Config{
			Endpoint: cfg.SCIMEndpoint,
			Token:    cfg.SCIMAccessToken,
		})
	if err != nil {
		return err
	}

	c := New(awsClient, googleClient)
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
