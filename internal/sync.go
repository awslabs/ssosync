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

// Package internal ...
package internal

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/awslabs/ssosync/internal/aws"
	"github.com/awslabs/ssosync/internal/config"
	"github.com/awslabs/ssosync/internal/google"
	"github.com/hashicorp/go-retryablehttp"

	log "github.com/sirupsen/logrus"
	admin "google.golang.org/api/admin/directory/v1"
)

// SyncGSuite is the interface for synchronizing users/groups
type SyncGSuite interface {
	Sync() error
}

// SyncGSuite is an object type that will synchronize real users and groups
type syncGSuite struct {
	aws    aws.Client
	google google.Client
	cfg    *config.Config
}

// New will create a new SyncGSuite object
func New(cfg *config.Config, a aws.Client, g google.Client) SyncGSuite {
	return &syncGSuite{
		aws:    a,
		google: g,
		cfg:    cfg,
	}
}

// pull a list of users and groups from AWS,

func (s *syncGSuite) planSync(awsUsers map[string]*aws.User, awsGroups map[string]*aws.Group, googleUsers map[string]*admin.User, googleGroups map[string][]string) (syncPlan map[string][][]string, err error) {

	log.Info("creating user and group sync plans")
	syncPlan = make(map[string][][]string)
	syncPlan["users"], err = s.getUserChanges(awsUsers, googleUsers)
	if err != nil {
		return nil, err
	}
	syncPlan["groups"], err = s.getGroupChanges(awsGroups, googleGroups)
	if err != nil {
		return nil, err
	}

	return syncPlan, err
}

func (s *syncGSuite) Sync() (err error) {

	log.Info("starting sync process")

	awsUsers, err := s.getAwsUsers()
	if err != nil {
		return err
	}
	awsGroups, err := s.getAwsGroups(awsUsers)
	if err != nil {
		return err
	}
	googleGroups, err := s.getFilteredGoogleGroups()
	if err != nil {
		return err
	}
	googleUsers, err := s.getFilteredGoogleUsers(googleGroups)
	if err != nil {
		return err
	}
	//googleDeleteUsers, err := s.google.GetDeletedUsers()

	syncPlan, err := s.planSync(awsUsers, awsGroups, googleUsers, googleGroups)
	if err != nil {
		return err
	}

	if log.GetLevel() >= log.InfoLevel {
		log.WithField("plan", "users").Info("user plan:")
		for _, cmd := range syncPlan["users"] {
			log.WithField("plan", "user").Info("\t", cmd[0], "->", cmd[1])
		}
		log.WithField("plan", "groups").Info("group plan:")
		for _, cmd := range syncPlan["groups"] {
			if len(cmd) == 2 {
				log.WithField("plan", "groups").Info("\t", cmd[0], "->", cmd[1])
			} else {
				log.WithField("plan", "groups").Info("\t", cmd[1], ":", cmd[0], "->", cmd[2])
			}
		}
	}

	if s.cfg.DryRun {
		log.Info("running in dry run mode, no changes will be committed")
		return nil
	}

	newUsers, err := s.syncUsers(syncPlan["users"], awsUsers, googleUsers)
	if err != nil {
		return err
	}
	for user, awsUser := range newUsers {
		awsUsers[user] = awsUser
	}
	err = s.syncGroups(syncPlan["groups"], awsUsers, awsGroups, googleGroups)
	if err != nil {
		return err
	}

	return nil
}

func (s *syncGSuite) syncUsers(userPlan [][]string, awsUsers map[string]*aws.User, googleUsers map[string]*admin.User) (newUsers map[string]*aws.User, err error) {

	log.Info("executing user plan")

	newUsers = make(map[string]*aws.User)

	for _, cmd := range userPlan {
		if cmd[0] == "create" {
			log.WithField("user", cmd[1]).Info("adding user")
			gUser := googleUsers[cmd[1]]
			awsUser := aws.NewUser(gUser.Name.GivenName, gUser.Name.FamilyName, cmd[1], !gUser.Suspended)
			_, err := s.aws.CreateUser(awsUser)
			if err != nil {
				return nil, err
			}
			newUsers[cmd[1]] = awsUser
			continue
		}
		if cmd[0] == "delete" {
			log.WithField("user", cmd[1]).Warn("deleting user")
			awsUser := awsUsers[cmd[1]]
			err := s.aws.DeleteUser(awsUser)
			if err != nil {
				return nil, err
			}
			continue
		}

	}

	return newUsers, nil
}

func (s *syncGSuite) syncGroups(groupPlan [][]string, awsUsers map[string]*aws.User, awsGroups map[string]*aws.Group, googleGroups map[string][]string) (err error) {

	log.Info("executing group plan")

	for _, stmt := range groupPlan {
		cmd := stmt[0]
		group := stmt[1]
		if cmd == "create" {
			log.WithField("group", group).Info("creating group")
			newGroup := aws.NewGroup(group)
			awsGroup, err := s.aws.CreateGroup(newGroup)
			if err != nil {
				return err
			}
			awsGroups[group] = awsGroup
		}
		if cmd == "delete" {
			log.WithField("group", group).Warn("deleting group")
			err = s.aws.DeleteGroup(awsGroups[group])
			if err != nil && err != aws.ErrGroupNotFound {
				return err
			}
		}
		if cmd == "remove" {
			user := stmt[2]
			log.WithField("group", group).Warn("removing user from group: ", user)
			err := s.aws.RemoveUserFromGroup(awsUsers[user], awsGroups[group])
			if err != nil {
				return err
			}
		}
		if cmd == "add" {
			user := stmt[2]
			log.WithField("group", group).Info("adding user to group: ", user)
			err := s.aws.AddUserToGroup(awsUsers[user], awsGroups[group])
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *syncGSuite) getAwsUsers() (awsUsers map[string]*aws.User, err error) {

	log.Info("Retreiving list of AWS Users")

	awsUsers = make(map[string]*aws.User)

	users, err := s.aws.GetUsers()
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		awsUsers[u.Username] = u
	}

	log.Debugf("AWS Users: %+v\n", awsUsers)

	return awsUsers, nil

}

func (s *syncGSuite) getAwsGroups(awsUsers map[string]*aws.User) (awsGroups map[string]*aws.Group, err error) {

	log.Info("Retreiving list of AWS Groups")

	awsGroups = make(map[string]*aws.Group)

	groups, err := s.aws.GetGroups()
	if err != nil {
		return nil, err
	}

	for _, g := range groups {
		log.WithField("group", g.DisplayName).Debug("finding users for group")
		for _, u := range awsUsers {
			//log.WithField("user", u.Username).Debug("checking is user is in group.")
			in, err := s.aws.IsUserInGroup(u, g)
			if err != nil {
				return nil, err
			}
			if in {
				log.WithField("user", u.Username).Debug("adding user to group members")
				g.Members = append(g.Members, u.Username)
			}
		}
		awsGroups[g.DisplayName] = g
	}

	log.Debugf("AWS Groups: %+v\n", awsGroups)

	return awsGroups, nil

}

func (s *syncGSuite) getGroupChanges(awsGroups map[string]*aws.Group, googleGroups map[string][]string) (groupPlan [][]string, err error) {

	deleteGroups := make(map[string]string)
	compareGroups := make(map[string]string)

	for _, awsGroup := range awsGroups {
		if _, in := googleGroups[awsGroup.DisplayName]; in {
			compareGroups[awsGroup.DisplayName] = awsGroup.ID
		} else {
			deleteGroups[awsGroup.DisplayName] = awsGroup.ID
			groupPlan = append(groupPlan, []string{"delete", awsGroup.DisplayName})
		}
	}

	for googleGroup, members := range googleGroups {
		if _, in := deleteGroups[googleGroup]; in {
			continue
		}

		if _, in := compareGroups[googleGroup]; in {
			delMembers, addMembers := s.diffLists(awsGroups[googleGroup].Members, members)

			for _, d := range delMembers {
				groupPlan = append(groupPlan, []string{"remove", googleGroup, d})
			}

			for _, a := range addMembers {
				groupPlan = append(groupPlan, []string{"add", googleGroup, a})
			}

			continue
		}

		groupPlan = append(groupPlan, []string{"create", googleGroup})
		for _, m := range members {
			groupPlan = append(groupPlan, []string{"add", googleGroup, m})
		}

	}

	log.Debugf("Group Changes: %+v\n", groupPlan)

	return groupPlan, nil

}

func (s *syncGSuite) getUserChanges(awsUsers map[string]*aws.User, googleUsers map[string]*admin.User) (userPlan [][]string, err error) {

	userPlan = make([][]string, 0)

	for u := range awsUsers {
		if _, in := googleUsers[u]; !in {
			userPlan = append(userPlan, []string{"delete", u})
		}
	}

	for u := range googleUsers {
		if _, in := awsUsers[u]; !in {
			userPlan = append(userPlan, []string{"create", u})
		}
	}

	log.Debugf("User Changes: %+v\n", userPlan)

	return userPlan, err
}

func (s *syncGSuite) getFilteredGoogleGroups() (map[string][]string, error) {

	log.Info("Retreiving list of Google Groups")

	groupsMembers := make(map[string][]string)

	groups, err := s.google.GetGroups(s.cfg.GroupMatch)
	if err != nil {
		return nil, err
	}

	for _, g := range groups {
		if s.ignoreGroup(g.Email) || s.ignoreGroup(g.Name) {
			continue
		}

		if !(s.includeGroup(g.Email) || s.includeGroup(g.Name)) {
			continue
		}

		members, err := s.google.GetGroupMembers(g)
		if err != nil {
			return nil, err
		}

		memberEmails := make([]string, 0)
		for _, m := range members {
			if m.Type == "Group" {
				log.WithField("id", m.Email).Warn("skipping, nested groups are not supported")
				continue
			}
			memberEmails = append(memberEmails, m.Email)
		}

		groupsMembers[g.Name] = memberEmails
	}

	log.Debugf("Google Groups: %+v\n", groupsMembers)

	return groupsMembers, nil

}

func (s *syncGSuite) getFilteredGoogleUsers(groupsMembers map[string][]string) (map[string]*admin.User, error) {

	log.Info("Retrieving list of Google Users")

	filteredUsers := make(map[string]*admin.User)

	if s.cfg.SyncMethod == config.DefaultSyncMethod {
		log.Info("Using default sync method to get users: ", s.cfg.SyncMethod)
		for g, members := range groupsMembers {
			log.Info("Retreiving users for group:", g)
			for _, m := range members {
				if _, in := filteredUsers[m]; in {
					log.WithField("id", m).Debug("skipping, user already in retreived")
					continue
				}
				if s.ignoreUser(m) {
					log.WithField("id", m).Debug("ignoring user")
					continue
				}

				q := fmt.Sprintf("email:%s", m)

				user, err := s.google.GetUsers(q)
				if err != nil {
					return nil, err
				}

				filteredUsers[m] = user[0]
			}
		}
	} else {
		users, err := s.google.GetUsers(s.cfg.UserMatch)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			if s.ignoreUser(u.PrimaryEmail) {
				log.WithField("id", u.PrimaryEmail).Debug("ignoring user")
			}
			filteredUsers[u.PrimaryEmail] = u
		}
	}

	log.Debugf("%+v\n", filteredUsers)

	return filteredUsers, nil

}

// DoSync will create a logger and run the sync with the paths
// given to do the sync.
func DoSync(ctx context.Context, cfg *config.Config) error {
	log.Info("Syncing AWS users and groups from Google Workspace SAML Application")

	creds := []byte(cfg.GoogleCredentials)

	if !cfg.IsLambda {
		b, err := ioutil.ReadFile(cfg.GoogleCredentials)
		if err != nil {
			return err
		}
		creds = b
	}

	// create a http client with retry and backoff capabilities
	retryClient := retryablehttp.NewClient()

	// https://github.com/hashicorp/go-retryablehttp/issues/6
	if cfg.Debug {
		retryClient.Logger = log.StandardLogger()
	} else {
		retryClient.Logger = nil
	}

	httpClient := retryClient.StandardClient()

	googleClient, err := google.NewClient(ctx, cfg.GoogleAdmin, creds)
	if err != nil {
		return err
	}

	awsClient, err := aws.NewClient(
		httpClient,
		&aws.Config{
			Endpoint: cfg.SCIMEndpoint,
			Token:    cfg.SCIMAccessToken,
		})
	if err != nil {
		return err
	}

	c := New(cfg, awsClient, googleClient)

	c.Sync()

	return nil
}

func (s *syncGSuite) ignoreUser(name string) bool {
	for _, u := range s.cfg.IgnoreUsers {
		if u == name {
			log.WithField("user", name).Info("in ignore user list")
			return true
		}
	}

	return false
}

func (s *syncGSuite) diffLists(listA []string, listB []string) (onlyA []string, onlyB []string) {
	onlyA = make([]string, 0)
	onlyB = make([]string, 0)
	union := make(map[string]string)

OUTER:
	for _, a := range listA {
		for _, b := range listB {
			if a == b {
				union[a] = a
				continue OUTER
			}
		}
		onlyA = append(onlyA, a)
	}

	for _, b := range listB {
		if _, in := union[b]; !in {
			onlyB = append(onlyB, b)
		}
	}

	return onlyA, onlyB
}

func (s *syncGSuite) ignoreGroup(name string) bool {
	for _, g := range s.cfg.IgnoreGroups {
		if g == name {
			log.WithField("group", name).Info("in ignore groups list")
			return true
		}
	}

	return false
}

func (s *syncGSuite) includeGroup(name string) bool {
	if len(s.cfg.IncludeGroups) == 0 {
		return true
	}

	for _, g := range s.cfg.IncludeGroups {
		if g == name {
			log.WithField("group", name).Info("in included groups list")
			return true
		}
	}
	return false
}
