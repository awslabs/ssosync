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
	SyncUsers([]string) error
	SyncGroups([]string) error
	SyncGroupsUsers([]string) error
}

// SyncGSuite is an object type that will synchronize real users and groups
type syncGSuite struct {
	aws    aws.Client
	google google.Client
	cfg    *config.Config

	users map[string]*aws.User
}

// New will create a new SyncGSuite object
func New(cfg *config.Config, a aws.Client, g google.Client) SyncGSuite {
	return &syncGSuite{
		aws:    a,
		google: g,
		cfg:    cfg,
		users:  make(map[string]*aws.User),
	}
}

// SyncUsers will Sync Google Users to AWS SSO SCIM
// References:
// * https://developers.google.com/admin-sdk/directory/v1/guides/search-users
// query possible values:
// '' --> empty or not defined
//  name:'Jane'
//  email:admin*
//  isAdmin=true
//  manager='janesmith@example.com'
//  orgName=Engineering orgTitle:Manager
//  EmploymentData.projects:'GeneGnomes'
func (s *syncGSuite) SyncUsers(queries []string) error {
	activeUsers := make(map[string]*admin.User)

	googleUsers, err := s.getUsers(queries)
	if err != nil {
		return err
	}

	// Required to ensure potential re-add of recently deleted users
	for _, u := range googleUsers {
		activeUsers[u.PrimaryEmail] = u
	}

	log.Debug("get deleted users")
	deletedUsers, err := s.google.GetDeletedUsers()
	if err != nil {
		log.Warn("Error Getting Deleted Users")
		return err
	}

	for _, u := range deletedUsers {
		ll := log.WithFields(log.Fields{
			"email": u.PrimaryEmail,
		})

		// If user is still active, it was re-added (instead of recovered)
		if _, found := activeUsers[u.PrimaryEmail]; found {
			ll.Info("found deleted google user that was re-added")

			continue
		}

		ll.Info("deleting google user")

		uu, err := s.aws.FindUserByEmail(u.PrimaryEmail)
		if err != aws.ErrUserNotFound && err != nil {
			ll.Warn("Error deleting google user")
			return err
		}

		if err == aws.ErrUserNotFound {
			ll.Debug("User already deleted")
			continue
		}

		if err := s.aws.DeleteUser(uu); err != nil {
			ll.Warn("Error deleting user")
			return err
		}

		ll.Info("successfully deleted")
	}

	log.Debug("iterating over non-deleted google users")
	for _, u := range googleUsers {
		if s.ignoreUser(u.PrimaryEmail) {
			continue
		}

		ll := log.WithFields(log.Fields{
			"email": u.PrimaryEmail,
		})

		ll.Debug("finding user")
		uu, _ := s.aws.FindUserByEmail(u.PrimaryEmail)
		if uu != nil {
			s.users[uu.Username] = uu
			// Update the user when suspended state is changed
			if uu.Active == u.Suspended {
				log.Debug("Mismatch active/suspended, updating user")
				// create new user object and update the user
				_, err := s.aws.UpdateUser(aws.UpdateUser(
					uu.ID,
					u.Name.GivenName,
					u.Name.FamilyName,
					u.PrimaryEmail,
					!u.Suspended))
				if err != nil {
					return err
				}
			}
			continue
		}

		ll.Info("creating user")
		uu, err := s.aws.CreateUser(aws.NewUser(
			u.Name.GivenName,
			u.Name.FamilyName,
			u.PrimaryEmail,
			!u.Suspended))
		if err != nil {
			return err
		}

		s.users[uu.Username] = uu
	}

	return nil
}

// SyncGroups will sync groups from Google -> AWS SSO
// References:
// * https://developers.google.com/admin-sdk/directory/v1/guides/search-groups
// query possible values:
// '' --> empty or not defined
//  name='contact'
//  email:admin*
//  memberKey=user@company.com
//  name:contact* email:contact*
//  name:Admin* email:aws-*
//  email:aws-*
func (s *syncGSuite) SyncGroups(queries []string) error {

	googleGroups, err := s.getGroups(queries)
	if err != nil {
		return err
	}

	correlatedGroups := make(map[string]*aws.Group)

	for _, g := range googleGroups {
		ll := log.WithFields(log.Fields{
			"group": g.Name,
		})

		if s.ignoreGroup(g.Name) || !s.includeGroup(g.Name) {
			ll.Info("ignored group")
			continue
		}

		ll.Debug("Check group")
		var group *aws.Group

		gg, err := s.aws.FindGroupByDisplayName(g.Name)
		if err != nil && err != aws.ErrGroupNotFound {
			return err
		}

		if gg != nil {
			ll.Debug("Found group")
			correlatedGroups[gg.DisplayName] = gg
			group = gg
		} else {
			ll.Info("Creating group in AWS")
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

		ll.Info("Start group user sync")

		for _, m := range groupMembers {
			if _, ok := s.users[m.Email]; ok {
				memberList[m.Email] = m
			}
		}

		for _, u := range s.users {
			ll := ll.WithField("user", u.Username)

			ll.Debug("Checking user is in group already")
			b, err := s.aws.IsUserInGroup(u, group)
			if err != nil {
				return err
			}

			if _, ok := memberList[u.Username]; ok {
				if !b {
					ll.Info("Adding user to group")
					err := s.aws.AddUserToGroup(u, group)
					if err != nil {
						return err
					}
				}
			} else {
				if b {
					ll.Warn("Removing user from group")
					err := s.aws.RemoveUserFromGroup(u, group)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// SyncGroupsUsers will sync groups and its members from Google -> AWS SSO SCIM
// allowing filter groups base on google api filter query parameter
// References:
// * https://developers.google.com/admin-sdk/directory/v1/guides/search-groups
// query possible values:
// '' --> empty or not defined
//  name='contact'
//  email:admin*
//  memberKey=user@company.com
//  name:contact* email:contact*
//  name:Admin* email:aws-*
//  email:aws-*
// process workflow:
//  1) delete users in aws, these were deleted in google
//  2) update users in aws, these were updated in google
//  3) add users in aws, these were added in google
//  4) add groups in aws and add its members, these were added in google
//  5) validate equals aws an google groups members
//  6) delete groups in aws, these were deleted in google
func (s *syncGSuite) SyncGroupsUsers(queries []string) error {

	googleGroups, err := s.getGroups(queries)
	if err != nil {
		return err
	}

	// Filter groups
	filteredGoogleGroups := []*admin.Group{}
	for _, g := range googleGroups {

		// because is in flag --ignore-groups
		if s.ignoreGroup(g.Email) {
			log.WithField("group", g.Email).Warn("ignoring group, using --ignore-groups")
			continue
		}

		filteredGoogleGroups = append(filteredGoogleGroups, g)
	}
	googleGroups = filteredGoogleGroups

	log.Debug("preparing list of google users and then google groups and their members")
	googleUsers, googleGroupsUsers, err := s.getGoogleGroupsAndUsers(googleGroups)
	if err != nil {
		return err
	}

	log.Info("get existing aws groups")
	awsGroups, err := s.aws.GetGroups()
	if err != nil {
		log.Error("error getting aws groups")
		return err
	}

	log.Info("get existing aws users")
	awsUsers, err := s.aws.GetUsers()
	if err != nil {
		return err
	}

	log.Debug("preparing list of aws groups and their members")
	awsGroupsUsers, err := s.getAWSGroupsAndUsers(awsGroups, awsUsers)
	if err != nil {
		return err
	}

	// create list of changes by operations
	addAWSUsers, delAWSUsers, updateAWSUsers, _ := getUserOperations(awsUsers, googleUsers)
	addAWSGroups, delAWSGroups, equalAWSGroups := getGroupOperations(awsGroups, googleGroups)

	log.Info("syncing changes")
	// delete aws users (deleted in google)
	log.Debug("deleting aws users deleted in google")
	for _, awsUser := range delAWSUsers {

		log := log.WithFields(log.Fields{"user": awsUser.Username})

		log.Debug("finding user")
		awsUserFull, err := s.aws.FindUserByEmail(awsUser.Username)
		if err != nil {
			return err
		}

		log.Warn("deleting user")
		if err := s.aws.DeleteUser(awsUserFull); err != nil {
			log.Error("error deleting user")
			return err
		}
	}

	// update aws users (updated in google)
	log.Debug("updating aws users updated in google")
	for _, awsUser := range updateAWSUsers {

		log := log.WithFields(log.Fields{"user": awsUser.Username})

		log.Debug("finding user")
		awsUserFull, err := s.aws.FindUserByEmail(awsUser.Username)
		if err != nil {
			return err
		}

		log.Warn("updating user")
		_, err = s.aws.UpdateUser(awsUserFull)
		if err != nil {
			log.Error("error updating user")
			return err
		}
	}

	// add aws users (added in google)
	log.Debug("creating aws users added in google")
	for _, awsUser := range addAWSUsers {
		// Due to limits in users listing, the user may already exists
		// see https://docs.aws.amazon.com/singlesignon/latest/developerguide/listusers.html
		user, _ := s.aws.FindUserByEmail(awsUser.Username)
		if user == nil {
			log := log.WithFields(log.Fields{"user": awsUser.Username})

			log.Info("creating user")
			_, err := s.aws.CreateUser(awsUser)
			if err != nil {
				log.Error("error creating user")
				return err
			}
		}
	}

	// add aws groups (added in google)
	log.Debug("creating aws groups added in google")
	for _, awsGroup := range addAWSGroups {

		log := log.WithFields(log.Fields{"group": awsGroup.DisplayName})

		log.Info("creating group")
		awsGroupFull, err := s.aws.CreateGroup(awsGroup)
		if err != nil {
			log.Error("creating group")
			return err
		}

		// add members of the new group
		for _, googleUser := range googleGroupsUsers[awsGroup.DisplayName] {

			// equivalent aws user of google user on the fly
			log.Debug("finding user")
			awsUserFull, err := s.aws.FindUserByEmail(googleUser.PrimaryEmail)
			if err != nil {
				return err
			}

			log.WithField("user", awsUserFull.Username).Info("adding user to group")
			err = s.aws.AddUserToGroup(awsUserFull, awsGroupFull)
			if err != nil {
				return err
			}
		}
	}

	// list of users to to be removed in aws groups
	deleteUsersFromGroup, _ := getGroupUsersOperations(googleGroupsUsers, awsGroupsUsers)

	// validate groups members are equal in aws and google
	log.Debug("validating groups members, equals in aws and google")
	for _, awsGroup := range equalAWSGroups {

		// add members of the new group
		log := log.WithFields(log.Fields{"group": awsGroup.DisplayName})

		for _, googleUser := range googleGroupsUsers[awsGroup.DisplayName] {

			log.WithField("user", googleUser.PrimaryEmail).Debug("finding user")
			awsUserFull, err := s.aws.FindUserByEmail(googleUser.PrimaryEmail)
			if err != nil {
				return err
			}

			log.WithField("user", awsUserFull.Username).Debug("checking user is in group already")
			b, err := s.aws.IsUserInGroup(awsUserFull, awsGroup)
			if err != nil {
				return err
			}

			if !b {
				log.WithField("user", awsUserFull.Username).Info("adding user to group")
				err := s.aws.AddUserToGroup(awsUserFull, awsGroup)
				if err != nil {
					return err
				}
			}
		}

		for _, awsUser := range deleteUsersFromGroup[awsGroup.DisplayName] {
			log.WithField("user", awsUser.Username).Warn("removing user from group")
			err := s.aws.RemoveUserFromGroup(awsUser, awsGroup)
			if err != nil {
				return err
			}
		}
	}

	// delete aws groups (deleted in google)
	log.Debug("delete aws groups deleted in google")
	for _, awsGroup := range delAWSGroups {

		log := log.WithFields(log.Fields{"group": awsGroup.DisplayName})

		log.Debug("finding group")
		awsGroupFull, err := s.aws.FindGroupByDisplayName(awsGroup.DisplayName)
		if err != nil {
			return err
		}

		log.Warn("deleting group")
		err = s.aws.DeleteGroup(awsGroupFull)
		if err != nil {
			log.Error("deleting group")
			return err
		}
	}

	log.Info("sync completed")

	return nil
}

// getGroups returns Google Groups from multiple queries.
func (s *syncGSuite) getGroups(queries []string) ([]*admin.Group, error) {
	uniqueGroups := map[string]*admin.Group{}

	for _, query := range queries {
		log.WithField("query", query).Debug("get google groups")
		googleGroups, err := s.google.GetGroups(query)
		if err != nil {
			return nil, err
		}

		for _, group := range googleGroups {
			uniqueGroups[group.Id] = group
		}
	}

	groups := make([]*admin.Group, len(uniqueGroups))
	var i int
	for _, group := range uniqueGroups {
		groups[i] = group
		i++
	}

	return groups, nil
}

// getUsers returns Google Users from multiple queries.
func (s *syncGSuite) getUsers(queries []string) ([]*admin.User, error) {
	uniqueUsers := map[string]*admin.User{}

	for _, query := range queries {
		log.WithField("query", query).Debug("get google users")
		googleUsers, err := s.google.GetUsers(query)
		if err != nil {
			return nil, err
		}

		for _, user := range googleUsers {
			uniqueUsers[user.Id] = user
		}
	}

	users := make([]*admin.User, len(uniqueUsers))
	var i int
	for _, user := range uniqueUsers {
		users[i] = user
		i++
	}

	return users, nil
}

// getGoogleGroupsAndUsers return a list of google users members of googleGroups
// and a map of google groups and its users' list
func (s *syncGSuite) getGoogleGroupsAndUsers(googleGroups []*admin.Group) ([]*admin.User, map[string][]*admin.User, error) {
	gUsers := make([]*admin.User, 0)
	gGroupsUsers := make(map[string][]*admin.User)

	gUniqUsers := make(map[string]*admin.User)

	for _, g := range googleGroups {

		log := log.WithFields(log.Fields{"group": g.Name})

		if s.ignoreGroup(g.Email) {
			log.Debug("ignoring group")
			continue
		}

		log.Debug("get group members from google")
		groupMembers, err := s.google.GetGroupMembers(g)
		if err != nil {
			return nil, nil, err
		}

		log.Debug("get users")
		membersUsers := make([]*admin.User, 0)

		for _, m := range groupMembers {

			if s.ignoreUser(m.Email) {
				log.WithField("id", m.Email).Debug("ignoring user")
				continue
			}

			if m.Type == "GROUP" {
				log.WithField("id", m.Email).Debug("ignoring group address")
				continue
			}

			log.WithField("id", m.Email).Debug("get user")
			q := fmt.Sprintf("email:%s", m.Email)
			u, err := s.google.GetUsers(q) // TODO: implement GetUser(m.Email)
			if err != nil {
				return nil, nil, err
			}

			if len(u) == 0 {
				log.WithField("email", m.Email).Warn("ignoring unknown user")
				continue
			}

			membersUsers = append(membersUsers, u[0])

			_, ok := gUniqUsers[m.Email]
			if !ok {
				gUniqUsers[m.Email] = u[0]
			}
		}
		gGroupsUsers[g.Name] = membersUsers
	}

	for _, user := range gUniqUsers {
		gUsers = append(gUsers, user)
	}

	return gUsers, gGroupsUsers, nil
}

// getAWSGroupsAndUsers return a list of google users members of googleGroups
// and a map of google groups and its users' list
func (s *syncGSuite) getAWSGroupsAndUsers(awsGroups []*aws.Group, awsUsers []*aws.User) (map[string][]*aws.User, error) {
	awsGroupsUsers := make(map[string][]*aws.User)

	for _, awsGroup := range awsGroups {

		users := make([]*aws.User, 0)
		log := log.WithFields(log.Fields{"group": awsGroup.DisplayName})

		log.Debug("get group members from aws")
		// NOTE: AWS has not implemented yet some method to get the groups members https://docs.aws.amazon.com/singlesignon/latest/developerguide/listgroups.html
		// so, we need to check each user in each group which are too many unnecessary API calls
		for _, user := range awsUsers {

			log.Debug("checking if user is member of")
			found, err := s.aws.IsUserInGroup(user, awsGroup)
			if err != nil {
				return nil, err
			}
			if found {
				users = append(users, user)
			}
		}

		awsGroupsUsers[awsGroup.DisplayName] = users
	}
	return awsGroupsUsers, nil
}

// getGroupOperations returns the groups of AWS that must be added, deleted and are equals
func getGroupOperations(awsGroups []*aws.Group, googleGroups []*admin.Group) (add []*aws.Group, delete []*aws.Group, equals []*aws.Group) {

	awsMap := make(map[string]*aws.Group)
	googleMap := make(map[string]struct{})

	for _, awsGroup := range awsGroups {
		awsMap[awsGroup.DisplayName] = awsGroup
	}

	for _, gGroup := range googleGroups {
		googleMap[gGroup.Name] = struct{}{}
	}

	// AWS Groups found and not found in google
	for _, gGroup := range googleGroups {
		if _, found := awsMap[gGroup.Name]; found {
			equals = append(equals, awsMap[gGroup.Name])
		} else {
			add = append(add, aws.NewGroup(gGroup.Name))
		}
	}

	// Google Groups founds and not in aws
	for _, awsGroup := range awsGroups {
		if _, found := googleMap[awsGroup.DisplayName]; !found {
			delete = append(delete, aws.NewGroup(awsGroup.DisplayName))
		}
	}

	return add, delete, equals
}

// getUserOperations returns the users of AWS that must be added, deleted, updated and are equals
func getUserOperations(awsUsers []*aws.User, googleUsers []*admin.User) (add []*aws.User, delete []*aws.User, update []*aws.User, equals []*aws.User) {

	awsMap := make(map[string]*aws.User)
	googleMap := make(map[string]struct{})

	for _, awsUser := range awsUsers {
		awsMap[awsUser.Username] = awsUser
	}

	for _, gUser := range googleUsers {
		googleMap[gUser.PrimaryEmail] = struct{}{}
	}

	// AWS Users found and not found in google
	for _, gUser := range googleUsers {
		if awsUser, found := awsMap[gUser.PrimaryEmail]; found {
			if awsUser.Active == gUser.Suspended ||
				awsUser.Name.GivenName != gUser.Name.GivenName ||
				awsUser.Name.FamilyName != gUser.Name.FamilyName {
				log.WithFields(log.Fields{
					"aws.username":        awsUser.Username,
					"aws.active":          awsUser.Active,
					"aws.givenName":       awsUser.Name.GivenName,
					"aws.familyName":      awsUser.Name.FamilyName,
					"google.primaryEmail": gUser.PrimaryEmail,
					"google.suspended":    gUser.Suspended,
					"google.givenName":    gUser.Name.GivenName,
					"google.familyName":   gUser.Name.FamilyName,
				}).Warn("updated user values")

				update = append(update, aws.NewUser(gUser.Name.GivenName, gUser.Name.FamilyName, gUser.PrimaryEmail, !gUser.Suspended))
			} else {
				equals = append(equals, awsUser)
			}
		} else {
			add = append(add, aws.NewUser(gUser.Name.GivenName, gUser.Name.FamilyName, gUser.PrimaryEmail, !gUser.Suspended))
		}
	}

	// Google Users founds and not in aws
	for _, awsUser := range awsUsers {
		if _, found := googleMap[awsUser.Username]; !found {
			delete = append(delete, aws.NewUser(awsUser.Name.GivenName, awsUser.Name.FamilyName, awsUser.Username, awsUser.Active))
		}
	}

	return add, delete, update, equals
}

// groupUsersOperations returns the groups and its users of AWS that must be delete from these groups and what are equals
func getGroupUsersOperations(gGroupsUsers map[string][]*admin.User, awsGroupsUsers map[string][]*aws.User) (delete map[string][]*aws.User, equals map[string][]*aws.User) {

	mbG := make(map[string]map[string]struct{})

	// get user in google groups that are in aws groups and
	// users in aws groups that aren't in google groups
	for gGroupName, gGroupUsers := range gGroupsUsers {
		mbG[gGroupName] = make(map[string]struct{})
		for _, gUser := range gGroupUsers {
			mbG[gGroupName][gUser.PrimaryEmail] = struct{}{}
		}
	}

	delete = make(map[string][]*aws.User)
	equals = make(map[string][]*aws.User)
	for awsGroupName, awsGroupUsers := range awsGroupsUsers {
		for _, awsUser := range awsGroupUsers {
			// users that exist in aws groups but doesn't in google groups
			if _, found := mbG[awsGroupName][awsUser.Username]; found {
				equals[awsGroupName] = append(equals[awsGroupName], awsUser)
			} else {
				delete[awsGroupName] = append(delete[awsGroupName], awsUser)
			}
		}
	}

	return
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

	googleClient, err := google.NewClient(ctx, cfg.GoogleAdmin, creds, cfg.GoogleCustomerId)
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

	log.WithField("sync_method", cfg.SyncMethod).Info("syncing")
	switch cfg.SyncMethod {
	case "groups":
		err = c.SyncGroupsUsers(cfg.GroupMatch)
		if err != nil {
			return err
		}
		break

	case "users_groups":
		err = c.SyncUsers(cfg.UserMatch)
		if err != nil {
			return err
		}

		err = c.SyncGroups(cfg.GroupMatch)
		if err != nil {
			return err
		}
		break
	}

	return nil
}

func (s *syncGSuite) ignoreUser(name string) bool {
	for _, u := range s.cfg.IgnoreUsers {
		if u == name {
			return true
		}
	}

	return false
}

func (s *syncGSuite) ignoreGroup(name string) bool {
	for _, g := range s.cfg.IgnoreGroups {
		if g == name {
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
			return true
		}
	}

	return false
}
