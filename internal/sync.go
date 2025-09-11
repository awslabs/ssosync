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
	"errors"
	"os"
	"strings"
	"time"

	"ssosync/internal/aws"
	"ssosync/internal/aws/identitystore"
	"ssosync/internal/config"
	"ssosync/internal/constants"
	"ssosync/internal/google"
	"ssosync/internal/interfaces"

	retryablehttp "github.com/hashicorp/go-retryablehttp"

	aws_config "github.com/aws/aws-sdk-go-v2/config"
	aws_identitystore "github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystore_types "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	log "github.com/sirupsen/logrus"
	admin "google.golang.org/api/admin/directory/v1"
)

// SyncGSuite is the interface for synchronizing users/groups
type SyncGSuite interface {
	SyncUsers(string) error
	SyncGroups(string) error
	SyncGroupsUsers(string, string) error
}

// SyncGSuite is an object type that will synchronize real users and groups
type syncGSuite struct {
	aws           aws.Client
	google        google.Client
	cfg           *config.Config
	identityStore interfaces.IdentityStoreAPI

	users            map[string]*interfaces.User
	ignoreUsersSet   map[string]struct{}
	ignoreGroupsSet  map[string]struct{}
	includeGroupsSet map[string]struct{}
}

// New will create a new SyncGSuite object
func New(cfg *config.Config, a aws.Client, g google.Client, ids interfaces.IdentityStoreAPI) SyncGSuite {
	return &syncGSuite{
		aws:           a,
		google:        g,
		cfg:           cfg,
		identityStore: ids,
		users:         make(map[string]*interfaces.User),
	}
}

// SyncUsers will Sync Google Users to AWS SSO SCIM
// References:
// * https://developers.google.com/admin-sdk/directory/v1/guides/search-users
// query possible values:
// ” --> empty or not defined
//
//	name:'Jane'
//	email:admin*
//	isAdmin=true
//	manager='janesmith@example.com'
//	orgName=Engineering orgTitle:Manager
//	EmploymentData.projects:'GeneGnomes'
func (s *syncGSuite) SyncUsers(query string) error {
	log.Debug("get deleted users")
	deletedUsers, err := s.google.GetDeletedUsers()
	if err != nil {
		log.Warn("Error Getting Deleted Users")
		return err
	}

	for _, u := range deletedUsers {
		log.WithFields(log.Fields{
			"email": u.PrimaryEmail,
		}).Info("deleting google user")

		uu, err := s.aws.FindUserByEmail(u.PrimaryEmail)
		if err != aws.ErrUserNotFound && err != nil {
			log.WithFields(log.Fields{
				"email": u.PrimaryEmail,
			}).Warn("Error deleting google user")
			return err
		}

		if err == aws.ErrUserNotFound {
			log.WithFields(log.Fields{
				"email": u.PrimaryEmail,
			}).Debug("User already deleted")
			continue
		}
		err = s.aws.DeleteUser(uu)
		if err != nil {
			log.WithFields(log.Fields{
				"email": u.PrimaryEmail,
			}).Warn("Error deleting user")
			return err
		}
	}

	log.Debug("get active google users")
	googleUsers, err := s.google.GetUsers(query, s.cfg.UserFilter)
	if err != nil {
		return err
	}

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
					!u.Suspended,
					u.Id))
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
			!u.Suspended,
			u.Id))
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
// ” --> empty or not defined
//
//	name='contact'
//	email:admin*
//	memberKey=user@company.com
//	name:contact* email:contact*
//	name:Admin* email:aws-*
//	email:aws-*
func (s *syncGSuite) SyncGroups(query string) error {
	log.WithField("query", query).Debug("get google groups")
	googleGroups, err := s.google.GetGroups(query)
	if err != nil {
		return err
	}

	correlatedGroups := make(map[string]*interfaces.Group)

	for _, g := range googleGroups {
		if s.ignoreGroup(g.Email) || !s.includeGroup(g.Email) {
			continue
		}

		log := log.WithFields(log.Fields{
			"group": g.Email,
		})

		log.Debug("Check group")
		var group *interfaces.Group

		gg, err := s.aws.FindGroupByDisplayName(g.Email)
		if err != nil && err != aws.ErrGroupNotFound {
			return err
		}

		if gg != nil {
			log.Debug("Found group")
			correlatedGroups[gg.DisplayName] = gg
			group = gg
		} else {
			log.Info("Creating group in AWS")
			newGroup := aws.NewGroup(g.Email, g.Id)
			_, err := s.aws.CreateGroup(newGroup)
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
			b, err := identitystore.IsMemberInGroups(context.Background(), s.identityStore, &s.cfg.IdentityStoreID, []string{group.ID}, &u.ID)
			if err != nil {
				return err
			}

			if _, ok := memberList[u.Username]; ok {
				if !*b {
					log.WithField("user", u.Username).Info("Adding user to group")
					err = s.aws.AddUserToGroup(u, group)
					if err != nil {
						return err
					}
				}
			} else {
				if *b {
					log.WithField("user", u.Username).Warn("Removing user from group")
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
// ” --> empty or not defined
//
//	name='contact'
//	email:admin*
//	memberKey=user@company.com
//	name:contact* email:contact*
//	name:Admin* email:aws-*
//	email:aws-*
//
// process workflow:
//  1. delete users in aws, these were deleted in google
//  2. update users in aws, these were updated in google
//  3. add users in aws, these were added in google
//  4. add groups in aws and add its members, these were added in google
//  5. validate equals aws an google groups members
//  6. delete groups in aws, these were deleted in google
func (s *syncGSuite) SyncGroupsUsers(queryGroups string, queryUsers string) error {

	log.WithField("queryGroup", queryGroups).Info("get google groups")
	log.WithField("queryUsers", queryUsers).Info("get google users")

	log.Info("preparing list of google users, groups and their members")
	googleGroups, googleUsers, googleGroupsUsers, err := s.getGoogleGroupsAndUsers(queryGroups, queryUsers)
	if err != nil {
		return err
	}
	log.WithField("googleGroups", googleGroups).Debug("Groups to sync")
	log.WithField("googleUsers", googleUsers).Debug("Users to sync")

	log.Info("get existing aws groups")
	awsGroups, err := s.GetGroups()
	if err != nil {
		log.Error("error getting aws groups")
		return err
	}

	log.Info("get existing aws users")
	awsUsers, err := s.GetUsers()
	if err != nil {
		log.Error("error getting aws users")
		return err
	}

	log.Info("get active status for aws users")
	for _, awsUser := range awsUsers {
		scimUser, err := s.aws.FindUserByEmail(awsUser.Username)

		if err != nil {
			log.Error("error getting active status for user " + awsUser.ID)
			return err
		}

		awsUser.Active = scimUser.Active
	}

	log.Info("preparing map of user id's to user")
	awsUsersMap := CreateUserIDtoUserObjMap(awsUsers)

	log.Debug("preparing list of aws groups and their members")
	awsGroupsUsers, err := s.GetGroupMembershipsLists(awsGroups, awsUsersMap)
	if err != nil {
		return err
	}

	// create list of changes by operations
	addAWSUsers, delAWSUsers, updateAWSUsers, _ := getUserOperations(awsUsers, googleUsers)
	addAWSGroups, delAWSGroups, updateAWSGroups, equalAWSGroups := getGroupOperations(awsGroups, googleGroups)

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
		err = s.aws.DeleteUser(awsUserFull)
		if err != nil {
			log.WithField("user", awsUser).Error("error deleting user")
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
		_, err = s.aws.UpdateUser(aws.UpdateUser(
			awsUserFull.ID,
			awsUser.Name.GivenName,
			awsUser.Name.FamilyName,
			awsUser.Username,
			awsUser.Active,
			awsUser.ExternalId))
		if err != nil {
			log.WithField("user", awsUser).Error("error updating user")
			return err
		}
	}

	// add aws users (added in google)
	log.Debug("creating aws users added in google")
	for _, awsUser := range addAWSUsers {

		log := log.WithFields(log.Fields{"user": awsUser.Username})

		log.Info("creating user")
		_, err := s.aws.CreateUser(awsUser)
		if err != nil {
			errHTTP := new(aws.ErrHTTPNotOK)
			if errors.As(err, &errHTTP) && errHTTP.StatusCode == constants.StatusConflict {
				log.WithField("user", awsUser.Username).Warn("user already exists")
				continue
			}
			log.WithField("user", awsUser).Error("error creating user")
			return err
		}
	}

	// add aws groups (added in google)
	log.Debug("creating aws groups added in google")
	for _, awsGroup := range addAWSGroups {

		log.WithFields(log.Fields{"group": awsGroup}).Debug("Creating")

		log.WithFields(log.Fields{"Name": awsGroup.DisplayName}).Info("creating group")
		awsGroup, err := s.aws.CreateGroup(awsGroup)
		if err != nil {
			log.Error("creating group")
			return err
		}

		// add members of the new group
		for _, googleUser := range googleGroupsUsers[awsGroup.DisplayName] {

			// equivalent aws user of google user on the fly
			log.WithField("email:", googleUser.PrimaryEmail).Debug("aws.FindUserByEmail() finding user")
			awsUserFull, err := s.aws.FindUserByEmail(googleUser.PrimaryEmail)
			if err != nil {
				return err
			}

			log.WithField("user", awsUserFull.Username).Info("adding user to group")
			err = s.aws.AddUserToGroup(awsUserFull, awsGroup)
			if err != nil {
				return err
			}
		}
	}

	// update aws groups (changed in google)
	log.Debug("updating aws groups changed in google")
	for _, awsGroup := range updateAWSGroups {

		log.WithFields(log.Fields{"group": awsGroup}).Debug("Updating")

		log.WithFields(log.Fields{"Name": awsGroup.DisplayName}).Info("updating group")
		awsGroup, err := s.aws.UpdateGroup(awsGroup)
		if err != nil {
			log.Error("upating group")
			return err
		}

		// add members of the new group
		for _, googleUser := range googleGroupsUsers[awsGroup.DisplayName] {

			// equivalent aws user of google user on the fly
			log.WithField("email:", googleUser.PrimaryEmail).Debug("aws.FindUserByEmail() finding user")
			awsUserFull, err := s.aws.FindUserByEmail(googleUser.PrimaryEmail)
			if err != nil {
				return err
			}

			log.WithField("user", awsUserFull.Username).Info("adding user to group")
			err = s.aws.AddUserToGroup(awsUserFull, awsGroup)
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

			log.WithField("user", awsUserFull.Username).Info("adding user to group")
			err = s.aws.AddUserToGroup(awsUserFull, awsGroup)
			if err != nil {
				return err
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

// getGoogleGroupsAndUsers return a list of google users members of googleGroups
// and a map of google groups and its users' list
func (s *syncGSuite) getGoogleGroupsAndUsers(queryGroups string, queryUsers string) ([]*admin.Group, []*admin.User, map[string][]*admin.User, error) {
	gUsers := make([]*admin.User, 0)
	gGroupsUsers := make(map[string][]*admin.User)
	gUserDetailCache := make(map[string]*admin.User)
	gGroupDetailCache := make(map[string]*admin.Group)
	gUniqUsers := make(map[string]*admin.User)

	log.Debug("getGoogleGroupsAndUsers()")

	// Precaching group data, this will speed up processing of nested groups, etc...
	log.Info("Precache all Groups from Google")
	googleGroups, err := s.google.GetGroups("*")
	if err != nil {
		log.WithField("error", err).Error("failed precaching groups from Google")
		return nil, nil, nil, err
	}
	for _, g := range googleGroups {
		gGroupDetailCache[g.Email] = g
	}

	// Fetch Users
	log.WithField("queryUsers", queryUsers).WithField("queryFilters", s.cfg.UserFilter).Info("google.GetUsers() fetching userMatch")

	googleUsers, err := s.google.GetUsers(queryUsers, s.cfg.UserFilter)
	if err != nil {
		log.WithField("error", err).Error("failed fetching userMatch from Google")
		return nil, nil, nil, err
	}

	log.Debug("process users from google, filtering as required")
	for _, u := range googleUsers {

		// Remove any users that should be ignored
		if s.ignoreUser(u.PrimaryEmail) {
			log.WithField("email", u.PrimaryEmail).Debug("ignoring user")
			continue
		}

		if _, found := gUniqUsers[u.PrimaryEmail]; !found {
			log.WithField("email", u.PrimaryEmail).Debug("adding user")
			gUserDetailCache[u.PrimaryEmail] = u
			gUniqUsers[u.PrimaryEmail] = gUserDetailCache[u.PrimaryEmail]
			continue
		} else {
			log.WithField("email", u.PrimaryEmail).Debug("already existing")
			continue
		}
	}

	// For larger directories this will reduce execution time and avoid throttling limits
	// however if you have directory with 10s of 1000s of users you may want to down scope
	// this to a specific OU path or disable by leaving empty.
	if s.cfg.PrecacheOrgUnits[0] != "DISABLED" {
		precacheQueries := ""
		log.WithField("Precache OrgUnitPaths", s.cfg.PrecacheOrgUnits).Info("to be converted to queries")
		for _, orgUnitPath := range s.cfg.PrecacheOrgUnits {
			log.WithField("orgUnitPath", orgUnitPath).Debug("format into query string")
			orgUnitPath = strings.TrimSpace(orgUnitPath)
			orgUnitPath = strings.TrimSuffix(orgUnitPath, "/")
			if strings.ContainsRune(orgUnitPath, ' ') {
				precacheQueries = precacheQueries + ",OrgUnitPath='" + orgUnitPath + "'"
			} else {
				precacheQueries = precacheQueries + ",OrgUnitPath=" + orgUnitPath
			}
		}

		log.WithField("PrecacheOrgUnits", precacheQueries).WithField("queryFilters", s.cfg.UserFilter).Info("google.GetUsers() Precaching users from Google")

		googleUsers, err = s.google.GetUsers(precacheQueries, s.cfg.UserFilter)
		if err != nil {
			log.WithField("error", err).Error("Precaching failed, caching on the fly")
		} else if len(googleUsers) == 0 {
			log.Warn("Precaching return no users? Switching to caching on the fly")
		} else {
			for _, u := range googleUsers {
				if _, found := gUniqUsers[u.PrimaryEmail]; !found {
					log.WithField("email", u.PrimaryEmail).Debug("adding user to cache")
					gUserDetailCache[u.PrimaryEmail] = u
					continue
				} else {
					log.WithField("email", u.PrimaryEmail).Debug("already in cache")
					continue
				}
			}
		}
	} else {
		log.Info("Precaching DISABLED, caching on the fly")
	}

	log.WithField("queryGroups", queryGroups).Info("google.GetGroups() fetching groups from Google")
	gGroups, err := s.google.GetGroups(queryGroups)
	if err != nil {
		log.WithField("error", err).Error("google.GetGroups() failed fetching groups from Google")
		return nil, nil, nil, err
	}

	filteredGoogleGroups := []*admin.Group{}
	for _, g := range gGroups {
		if s.ignoreGroup(g.Email) {
			log.WithField("group", g.Email).Debug("ignoring group")
			continue
		}
		filteredGoogleGroups = append(filteredGoogleGroups, g)
	}
	gGroups = filteredGoogleGroups

	log.Debug("for each group retrieve the group members")
	for _, g := range gGroups {

		if s.ignoreGroup(g.Email) {
			log.WithField("group", g).Info("skipping group, ignore group")
			continue
		}

		log.WithField("group", g.Name).Debug("getGoogleUsersInGroup()")
		membersUsers := s.getGoogleUsersInGroup(g, gUserDetailCache, gGroupDetailCache)

		// If we've not seen the user email address before add it to the list of unique users
		// also, we need to deduplicate the list of members.
		log.WithField("group", g.Name).Debug("Processing group membership")
		gUniqMembers := make(map[string]*admin.User)
		for _, m := range membersUsers {
			log.WithField("user", m).Debug("processing member")
			if _, found := gUniqUsers[m.PrimaryEmail]; !found {
				log.WithField("email", m.PrimaryEmail).Debug("adding user to UniqueUsers")
				gUniqUsers[m.PrimaryEmail] = gUserDetailCache[m.PrimaryEmail]
			}

			if _, found := gUniqMembers[m.PrimaryEmail]; !found {
				log.WithField("email", m.PrimaryEmail).Debug("adding user to group")
				gUniqMembers[m.PrimaryEmail] = gUserDetailCache[m.PrimaryEmail]
			}
		}

		gMembers := make([]*admin.User, 0)
		for _, member := range gUniqMembers {
			gMembers = append(gMembers, member)
		}
		gGroupsUsers[g.Name] = gMembers
		log.WithField("group name:", g.Name).Debug("Finished processing membership")
	}

	log.Debug("returning group memberships and additional unique users")
	for _, user := range gUniqUsers {
		gUsers = append(gUsers, user)
	}

	return gGroups, gUsers, gGroupsUsers, nil
}

// getGroupOperations returns the groups of AWS that must be added, deleted and are equals
func getGroupOperations(awsGroups []*interfaces.Group, googleGroups []*admin.Group) (add []*interfaces.Group, delete []*interfaces.Group, update []*interfaces.Group, equals []*interfaces.Group) {

	log.Debug("getGroupOperations()")
	awsMap := make(map[string]*interfaces.Group)
	awsMapExtId := make(map[string]*interfaces.Group)
	googleMap := make(map[string]struct{})
	googleMapId := make(map[string]struct{})

	for _, awsGroup := range awsGroups {
		awsMap[awsGroup.DisplayName] = awsGroup
		if awsGroup.ExternalId != "" {
			awsMapExtId[awsGroup.ExternalId] = awsGroup
		}
	}

	for _, gGroup := range googleGroups {
		googleMap[gGroup.Name] = struct{}{}
		googleMapId[gGroup.Id] = struct{}{}
	}

	// AWS Groups found and not found in google
	for _, gGroup := range googleGroups {
		if awsGroup, found := awsMapExtId[gGroup.Id]; found {
			if awsGroup.DisplayName != gGroup.Name {
				log.WithField("awsGroup", awsGroup).Debug("update")
				log.WithField("gGroup", gGroup).Debug("update")
				update = append(update, aws.UpdateGroup(awsGroup.ID, gGroup.Name, gGroup.Id))
			} else {
				log.WithField("gGroup", gGroup).Debug("equals")
				equals = append(equals, awsGroup)
			}
		} else if awsGroup, found := awsMap[gGroup.Name]; found {
			log.WithField("awsGroup", awsGroup).Debug("update")
			log.WithField("gGroup", gGroup).Debug("update")
			update = append(update, aws.UpdateGroup(awsGroup.ID, gGroup.Name, gGroup.Id))
		} else {
			log.WithField("gGroup", gGroup).Debug("add")
			add = append(add, aws.NewGroup(gGroup.Name, gGroup.Id))
		}
	}

	// Google Groups founds and not in aws
	for _, awsGroup := range awsGroups {
		if _, found := googleMapId[awsGroup.ExternalId]; !found {
			if _, found := googleMap[awsGroup.DisplayName]; !found {
				log.WithField("awsGroup", awsGroup).Debug("delete")
				delete = append(delete, aws.UpdateGroup(awsGroup.ID, awsGroup.DisplayName, awsGroup.ExternalId))
			}
		}
	}

	return add, delete, update, equals
}

// getUserOperations returns the users of AWS that must be added, deleted, updated and are equals
func getUserOperations(awsUsers []*interfaces.User, googleUsers []*admin.User) (add []*interfaces.User, delete []*interfaces.User, update []*interfaces.User, equals []*interfaces.User) {

	log.Debug("getUserOperations()")
	awsMap := make(map[string]*interfaces.User)
	awsMapExtId := make(map[string]*interfaces.User)
	googleMap := make(map[string]struct{})
	googleMapId := make(map[string]struct{})

	for _, awsUser := range awsUsers {
		awsMapExtId[awsUser.ExternalId] = awsUser
		awsMap[awsUser.Username] = awsUser
	}

	for _, gUser := range googleUsers {
		googleMapId[gUser.Id] = struct{}{}
		googleMap[gUser.PrimaryEmail] = struct{}{}
	}

	// AWS Users found and not found in google
	for _, gUser := range googleUsers {
		if awsUser, found := awsMapExtId[gUser.Id]; found {
			if awsUser.Active == gUser.Suspended ||
				awsUser.Username != gUser.PrimaryEmail ||
				awsUser.Name.GivenName != gUser.Name.GivenName ||
				awsUser.Name.FamilyName != gUser.Name.FamilyName ||
				awsUser.ExternalId != gUser.Id {
				log.WithField("gUser", gUser).Debug("update")
				log.WithField("awsUser", awsUser).Debug("update")
				update = append(update, aws.UpdateUser(awsUser.ID, gUser.Name.GivenName, gUser.Name.FamilyName, gUser.PrimaryEmail, !gUser.Suspended, gUser.Id))
			} else {
				log.WithField("awsUser", awsUser).Debug("equals")
				equals = append(equals, awsUser)
			}
		} else if awsUser, found := awsMap[gUser.PrimaryEmail]; found {
			log.WithField("gUser", gUser).Debug("update")
			log.WithField("awsUser", awsUser).Debug("update")
			update = append(update, aws.UpdateUser(awsUser.ID, gUser.Name.GivenName, gUser.Name.FamilyName, gUser.PrimaryEmail, !gUser.Suspended, gUser.Id))
		} else {
			log.WithField("gUser", gUser).Debug("add")
			add = append(add, aws.NewUser(gUser.Name.GivenName, gUser.Name.FamilyName, gUser.PrimaryEmail, !gUser.Suspended, gUser.Id))
		}
	}

	// Google Users founds and not in aws
	for _, awsUser := range awsUsers {
		if _, found := googleMapId[awsUser.ExternalId]; !found {
			if _, found := googleMap[awsUser.Username]; !found {
				log.WithField("awsUser", awsUser).Debug("delete")
				delete = append(delete, aws.UpdateUser(awsUser.ID, awsUser.Name.GivenName, awsUser.Name.FamilyName, awsUser.Username, awsUser.Active, awsUser.ExternalId))
			}
		}
	}

	return add, delete, update, equals
}

// groupUsersOperations returns the groups and its users of AWS that must be delete from these groups and what are equals
func getGroupUsersOperations(gGroupsUsers map[string][]*admin.User, awsGroupsUsers map[string][]*interfaces.User) (delete map[string][]*interfaces.User, equals map[string][]*interfaces.User) {

	log.Debug("getGroupUsersOperations()")
	mbG := make(map[string]map[string]struct{})

	// get user in google groups that are in aws groups and
	// users in aws groups that aren't in google groups
	for gGroupName, gGroupUsers := range gGroupsUsers {
		mbG[gGroupName] = make(map[string]struct{})
		for _, gUser := range gGroupUsers {
			mbG[gGroupName][gUser.PrimaryEmail] = struct{}{}
		}
	}

	delete = make(map[string][]*interfaces.User)
	equals = make(map[string][]*interfaces.User)
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
		b, err := os.ReadFile(cfg.GoogleCredentials)
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
		log.WithField("error", err).Warn("Problem establising a connection to Google directory")
		return err
	}

	mkAwsScimClient := aws.NewClient
	if cfg.DryRun {
		mkAwsScimClient = aws.NewDryClient
	}

	if cfg.DryRun {
		log.Warn("This is a DRY RUN - actions will *not* be actually performed")
		defer log.Warn("This was a DRY RUN - actions were *not* actually performed")
	}

	awsScimClient, err := mkAwsScimClient(
		httpClient,
		&aws.Config{
			Endpoint: cfg.SCIMEndpoint,
			Token:    cfg.SCIMAccessToken,
		})
	if err != nil {
		log.WithField("error", err).Warn("Problem establising a SCIM connection to AWS IAM Identity Center")
		return err
	}

	aws_cfg, err := aws_config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}

	// Initialize AWS Identity Store Public API Client with session
	identityStoreClient := aws_identitystore.NewFromConfig(aws_cfg, func(o *aws_identitystore.Options) {
		o.Region = cfg.Region
	})

	// Wrap with dry run client if in dry run mode
	var finalIdentityStoreClient interfaces.IdentityStoreAPI = identityStoreClient
	if cfg.DryRun {
		finalIdentityStoreClient = aws.NewDryIdentityStore(identityStoreClient)
	}

	// Perform a lightweight test query to validate connectivity
	testCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	response, err := identitystore.ListGroups(
		testCtx,
		identityStoreClient,
		&cfg.IdentityStoreID,
		func(g identitystore_types.Group) *interfaces.Group {
			return ConvertIdentityStoreGroupToAWSGroup(g)
		},
	)

	if err != nil {
		log.WithField("error", err).Warn("Problem performing test query against Identity Store")
		return err
	}
	log.WithField("Groups", response).Info("Test call for groups successful")

	// Initialize sync client with
	// 1. SCIM API client
	// 2. Google Directory API client
	// 3. Identity Store Public API client
	c := New(cfg, awsScimClient, googleClient, finalIdentityStoreClient)

	log.WithField("sync_method", cfg.SyncMethod).Info("syncing")
	if cfg.SyncMethod == config.DefaultSyncMethod {
		err = c.SyncGroupsUsers(cfg.GroupMatch, cfg.UserMatch)
		if err != nil {
			return err
		}
	} else {
		err = c.SyncUsers(cfg.UserMatch)
		if err != nil {
			return err
		}

		err = c.SyncGroups(cfg.GroupMatch)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *syncGSuite) ignoreUser(name string) bool {
	if s.ignoreUsersSet == nil {
		s.ignoreUsersSet = make(map[string]struct{}, len(s.cfg.IgnoreUsers))
		for _, u := range s.cfg.IgnoreUsers {
			s.ignoreUsersSet[u] = struct{}{}
		}
	}
	_, exists := s.ignoreUsersSet[name]
	return exists
}

func (s *syncGSuite) ignoreGroup(name string) bool {
	if s.ignoreGroupsSet == nil {
		s.ignoreGroupsSet = make(map[string]struct{}, len(s.cfg.IgnoreGroups))
		for _, g := range s.cfg.IgnoreGroups {
			s.ignoreGroupsSet[g] = struct{}{}
		}
	}
	_, exists := s.ignoreGroupsSet[name]
	return exists
}

func (s *syncGSuite) includeGroup(name string) bool {
	if s.includeGroupsSet == nil {
		s.includeGroupsSet = make(map[string]struct{}, len(s.cfg.IncludeGroups))
		for _, g := range s.cfg.IncludeGroups {
			s.includeGroupsSet[g] = struct{}{}
		}
	}
	_, exists := s.includeGroupsSet[name]
	return exists
}

func ConvertIdentityStoreGroupToAWSGroup(group identitystore_types.Group) *interfaces.Group {
	if group.GroupId == nil {
		log.WithField("group", group).Warn("Group has no GroupId")
		return nil
	}
	if group.DisplayName == nil {
		log.WithField("group", group).Warn("Group has no DisplayName")
		return nil
	}
	log.WithField("groupId", group.GroupId).WithField("displayName", group.DisplayName).Debug("Group converted")
	return &interfaces.Group{
		ID:          *group.GroupId,
		Schemas:     []string{constants.SCIMSchemaGroup},
		DisplayName: *group.DisplayName,
		Members:     []string{},
	}
}

func (s *syncGSuite) GetGroups() ([]*interfaces.Group, error) {

	awsGroups, err := identitystore.ListGroupsPager(context.Background(),
		aws_identitystore.NewListGroupsPaginator(
			s.identityStore,
			&aws_identitystore.ListGroupsInput{
				IdentityStoreId: &s.cfg.IdentityStoreID,
			},
		),
		ConvertIdentityStoreGroupToAWSGroup,
	)
	if err != nil {
		return nil, err
	}

	return awsGroups, nil
}

func (s *syncGSuite) GetUsers() ([]*interfaces.User, error) {
	awsUsers, err := identitystore.ListUsersPager(
		context.Background(),
		aws_identitystore.NewListUsersPaginator(
			s.identityStore,
			&aws_identitystore.ListUsersInput{
				IdentityStoreId: &s.cfg.IdentityStoreID,
			},
		),
		ConvertSdkUserObjToNative,
	)

	if err != nil {
		return nil, err
	}

	return awsUsers, nil
}

// ConvertSdkUserObjToNative
// Convert SDK user to native user object
func ConvertSdkUserObjToNative(user identitystore_types.User) *interfaces.User {
	// Convert emails into native Email object
	userEmails := make([]interfaces.UserEmail, 0)

	for _, email := range user.Emails {
		if email.Value == nil || email.Type == nil {
			// This must be a user created by AWS Control Tower
			// Need feature development to make how these users are treated
			// configurable.
			continue
		}
		userEmails = append(userEmails, interfaces.UserEmail{
			Value:   *email.Value,
			Type:    *email.Type,
			Primary: email.Primary,
		})
	}

	// Convert addresses into native Address object
	userAddresses := make([]interfaces.UserAddress, 0)

	for _, address := range user.Addresses {
		userAddresses = append(userAddresses, interfaces.UserAddress{
			Type: *address.Type,
		})
	}

	return &interfaces.User{
		ID:       *user.UserId,
		Schemas:  []string{constants.SCIMSchemaUser},
		Username: *user.UserName,
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{
			FamilyName: *user.Name.FamilyName,
			GivenName:  *user.Name.GivenName,
		},
		DisplayName: *user.DisplayName,
		Emails:      userEmails,
		Addresses:   userAddresses,
	}
}

// CreateUserIDtoUserObjMap
// Create User ID for user object map
func CreateUserIDtoUserObjMap(awsUsers []*interfaces.User) map[string]*interfaces.User {
	awsUsersMap := make(map[string]*interfaces.User)

	for _, awsUser := range awsUsers {
		awsUsersMap[awsUser.ID] = awsUser
	}

	return awsUsersMap
}

// ListGroupMembershipPagesCallbackFn
// Handler for Paginated Group Membership List
func (s *syncGSuite) GetGroupMembershipsLists(awsGroups []*interfaces.Group, awsUsersMap map[string]*interfaces.User) (map[string][]*interfaces.User, error) {
	awsGroupsUsers := make(map[string][]*interfaces.User)
	curGroup := &interfaces.Group{}

	// For every group, get the members and assign in awsGroupsUsers map
	for _, group := range awsGroups {
		curGroup = group
		awsGroupsUsers[curGroup.DisplayName] = make([]*interfaces.User, 0)
		memberships, err := identitystore.ListGroupMembershipsPager(
			context.Background(),
			aws_identitystore.NewListGroupMembershipsPaginator(
				s.identityStore,
				&aws_identitystore.ListGroupMembershipsInput{
					IdentityStoreId: &s.cfg.IdentityStoreID,
					GroupId:         &group.ID,
				},
			),
			func(membership identitystore_types.GroupMembership) *interfaces.User {
				// Extract user ID from membership and return the user
				if membership.MemberId != nil {
					if userMember, ok := membership.MemberId.(*identitystore_types.MemberIdMemberUserId); ok {
						return awsUsersMap[userMember.Value]
					}
				}
				return nil
			},
		)

		if err != nil {
			return nil, err
		}

		for _, user := range memberships { // For every member in the group
			if user != nil {
				// Append new user onto existing list of users
				awsGroupsUsers[curGroup.DisplayName] = append(awsGroupsUsers[curGroup.DisplayName], user)
			}
		}

	}

	return awsGroupsUsers, nil
}

func (s *syncGSuite) IsUserInGroup(user *aws.User, group *aws.Group) (*bool, error) {
	isUserInGroupOutput, err := s.identityStore.IsMemberInGroups(
		context.Background(),
		&aws_identitystore.IsMemberInGroupsInput{
			IdentityStoreId: &s.cfg.IdentityStoreID,
			GroupIds:        []string{group.ID},
			MemberId: &identitystore_types.MemberIdMemberUserId{
				Value: user.ID,
			},
		},
	)

	if err != nil {
		return nil, err
	}

	if len(isUserInGroupOutput.Results) > 0 {
		return &isUserInGroupOutput.Results[0].MembershipExists, nil
	}

	falseValue := false
	return &falseValue, nil
}

func (s *syncGSuite) getGoogleUsersInGroup(group *admin.Group, userCache map[string]*admin.User, groupCache map[string]*admin.Group) []*admin.User {
	log.WithField("Email:", group.Email).Debug("getGoogleUsersInGroup()")

	// retrieve the members of the group
	groupMembers, err := s.google.GetGroupMembers(group)
	if err != nil {
		return nil
	}
	membersUsers := make([]*admin.User, 0)

	// process the members of the group
	for _, m := range groupMembers {
		log.WithField("member", m).Debug("processing group member")

		if m.Type != "USER" {
			log.WithField("id", m).Warn("skipping USER member")
			continue
		}

		// Ignore any external members, since they don't have users
		// that can be synced
		if m.Status != "ACTIVE" && m.Status != "SUSPENDED" {
			log.WithField("id", m.Email).Warn("skipping member: external user")
			continue
		}

		// Ignore any external members, since they don't have users
		// that can be synced
		if m.Status == "SUSPENDED" && !s.cfg.SyncSuspended {
			log.WithField("id", m.Email).Warn("skipping member: suspended user")
			continue
		}

		// Remove any users that should be ignored
		if s.ignoreUser(m.Email) {
			log.WithField("email", m.Email).Debug("skipping member: ignore list")
			continue
		}

		// Find the group member in the cache of UserDetails
		if _, found := userCache[m.Email]; !found {
			log.WithField("email", m.Email).Warn("not found in cache, fetching user")
			googleUsers, err := s.google.GetUsers("email="+m.Email, s.cfg.UserFilter)
			if err != nil {
				log.WithField("error:", err).Error("Fetching user")
				continue
			}
			for _, u := range googleUsers {
				log.WithField("email", u.PrimaryEmail).Debug("caching user")
				userCache[u.PrimaryEmail] = u
			}
		}
		log.WithField("email", m.Email).Debug("adding member")
		membersUsers = append(membersUsers, userCache[m.Email])

	}
	log.WithField("membersUsers", membersUsers).Debug("Return group membership")
	return membersUsers
}
