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
	LegacySyncUsers(string) error
	LegacySyncGroups(string) error
	Sync(string, string) error
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
	cacheMetrics     map[string]*CacheMetrics
}

// OperationType handle patch operations for add/remove
type CacheOperationType string

const (
	// PreCache is adding a user/group as part of the precaching
	PreCache CacheOperationType = "precache"
	// CacheHit means a user was successfully found in the cache and prepresents an api called avoided
	CacheHit CacheOperationType = "hit"
	// CacheMiss means a user was not found in the cache so an api call was required
	CacheMiss CacheOperationType = "miss"
)

// User represents a User in AWS SSO
type CacheMetrics struct {
	Precaches  int
	Hits       int
	Misses     int
	TotalUsers int
}

// operations
type OperationAction int

const (
	Equal OperationAction = iota
	Create
	AdoptLocal
	AdoptForeign
	Update
	UpdateForced
	Delete
	Recreate
	Retain
)

var operationName = map[OperationAction]string{
	Equal:        "no action required.",
	Create:       "create new user",
	AdoptLocal:   "adopt local (manual) user",
	AdoptForeign: "adopt user managed by previous external IdP",
	Update:       "update user attributes",
	UpdateForced: "force updated; user's externalId has changed",
	Delete:       "delete; managed user has unmatched.",
	Recreate:     "delete & add; user's externalId has changed, delete existing user and create new user to prevent permission inheritence.",
	Retain:       "retain, unmatched user (manual or created by previous external IdP)",
}

func (operationAction OperationAction) String() string {
	return operationName[operationAction]
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

func (s *syncGSuite) recordCacheOp(orgPath string, operation CacheOperationType) {
	if !s.cfg.CacheMetrics {
		return
	}
	// if this is the first use of s.cachStats make it
	if s.cacheMetrics == nil {
		s.cacheMetrics = make(map[string]*CacheMetrics, 0)
	}
	// have we seen this orgPath before, if not create an stats for it
	if _, exists := s.cacheMetrics[orgPath]; !exists {
		s.cacheMetrics[orgPath] = &CacheMetrics{}
	}
	// update the stat for the operation
	if entry, exists := s.cacheMetrics[orgPath]; exists {
		switch operation {
		case PreCache:
			entry.Precaches++
		case CacheHit:
			entry.Hits++
		case CacheMiss:
			entry.Misses++
		default:
			log.WithFields(
				log.Fields{
					"orgPath":   orgPath,
					"operation": operation}).Error("unknown operation")
		}
	}

	// If existing paths are parent or orgPath, update their stats
	for path, entry := range s.cacheMetrics {
		if !strings.Contains(orgPath, path) {
			continue
		}
		switch operation {
		case PreCache:
			entry.Precaches++
		case CacheHit:
			entry.Hits++
		case CacheMiss:
			entry.Misses++
		}
	}
}

func (s *syncGSuite) outputCacheMetrics() {
	if !s.cfg.CacheMetrics {
		return
	}

	for orgUnitPath, entry := range s.cacheMetrics {
		var query string
		if strings.ContainsRune(orgUnitPath, ' ') {
			query = "OrgUnitPath='" + orgUnitPath + "'"
		} else {
			query = "OrgUnitPath=" + orgUnitPath
		}
		googleUsers, err := s.google.GetUsers(query, s.cfg.UserFilter)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Failed retrieving usercache count")
		} else {
			entry.TotalUsers = len(googleUsers)
		}
	}
	log.WithFields(log.Fields{"Cache Stats": s.cacheMetrics}).Info("Caching Performance, orgPaths with a high Miss rate compared with the TotalUsers consider precaching")
}

// LegacySyncUsers will Sync Google Users to AWS SSO SCIM
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
func (s *syncGSuite) LegacySyncUsers(query string) error {
	log.Debug("get deleted users")
	deletedUsers, err := s.google.GetDeletedUsers()
	if err != nil {
		log.Warn("Error Getting Deleted Users")
		return err
	}

	activeGoogleUsers, err := s.google.GetUsers(query, s.cfg.UserFilter)
	if err != nil {
		return err
	}

	var newDeletedUsers = make([]*admin.User, 0)
	for _, deleteUser := range deletedUsers {
		isDeletedUserActive := false
		log.WithFields(log.Fields{
			"email": deleteUser.PrimaryEmail,
		}).Debug("Inspecting deleted user email")
		for _, activeGoogleUser := range activeGoogleUsers {
			if deleteUser.PrimaryEmail == activeGoogleUser.PrimaryEmail {
				isDeletedUserActive = true
				log.WithFields(log.Fields{
					"email": deleteUser.PrimaryEmail,
				}).Debug("User is active again! Breaking loop...")
				break
			}
		}
		if !isDeletedUserActive {
			log.WithFields(log.Fields{
				"email": deleteUser.PrimaryEmail,
			}).Debug("Inactive user email")
			newDeletedUsers = append(newDeletedUsers, deleteUser)
		}
	}

	for _, u := range newDeletedUsers {
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

	// Filter out ignored google users before correlating with AWS.
	filteredGoogleUsers := make([]*admin.User, 0, len(googleUsers))
	for _, u := range googleUsers {
		if s.ignoreUser(u.PrimaryEmail) {
			continue
		}
		filteredGoogleUsers = append(filteredGoogleUsers, u)
	}

	// Source the existing AWS users from the Identity Store so that their
	// ExternalId is populated. This allows matching on ExternalId (the Google
	// user id) rather than email address alone, mirroring getUserOperations.
	log.Debug("get existing aws users")
	awsUsers, err := s.GetUsers()
	if err != nil {
		log.Error("error getting aws users")
		return err
	}

	// ConvertSdkUserObjToNative does not populate Active; fetch it via SCIM.
	for _, awsUser := range awsUsers {
		scimUser, err := s.aws.FindUserByEmail(awsUser.Username)
		if err != nil {
			log.WithField("user", awsUser.Username).Error("error getting active status for user")
			return err
		}
		awsUser.Active = scimUser.Active
	}

	// Build lookup maps by ExternalId (Google id) and by username (email).
	awsUsersByExtId := make(map[string]*interfaces.User)
	awsUsersByUsername := make(map[string]*interfaces.User)
	for _, awsUser := range awsUsers {
		if awsUser.ExternalId.Id != "" {
			awsUsersByExtId[awsUser.ExternalId.Id] = awsUser
		}
		awsUsersByUsername[awsUser.Username] = awsUser
	}

	// setUser upserts the synced user into s.users so LegacySyncGroups can use it
	// for membership sync. It re-fetches by email to obtain the SCIM ID.
	setUser := func(username string) error {
		uu, err := s.aws.FindUserByEmail(username)
		if err != nil {
			return err
		}
		s.users[uu.Username] = uu
		return nil
	}

	for _, u := range filteredGoogleUsers {
		ll := log.WithFields(log.Fields{
			"email": u.PrimaryEmail,
		})

		if awsUser, found := awsUsersByExtId[u.Id]; found {
			// Matched by ExternalId (Google id). Update if any attribute differs.
			if awsUser.Active == u.Suspended ||
				awsUser.Username != u.PrimaryEmail ||
				awsUser.Name.GivenName != u.Name.GivenName ||
				awsUser.Name.FamilyName != u.Name.FamilyName {
				ll.Debug("update")
				if _, err := s.aws.UpdateUser(aws.UpdateUser(
					awsUser.ID, u.Name.GivenName, u.Name.FamilyName, u.PrimaryEmail, !u.Suspended, u.Id)); err != nil {
					return err
				}
			} else {
				ll.Debug("equals")
			}
		} else if awsUser, found := awsUsersByUsername[u.PrimaryEmail]; found {
			// Matched by email but not by ExternalId.
			if len(awsUser.ExternalId.Id) > 0 {
				if s.cfg.ForceExternalIdUpdate {
					ll.WithField("awsUser", awsUser).Warn("New google user with the same primary email address but a different id, Force option set, so retaining the existing user and updating its ExternalId. Caution this may lead to inherited privileges.")
					if _, err := s.aws.UpdateUser(aws.UpdateUser(
						awsUser.ID, u.Name.GivenName, u.Name.FamilyName, u.PrimaryEmail, !u.Suspended, u.Id)); err != nil {
						return err
					}
				} else {
					ll.WithField("awsUser", awsUser).Warn("New google user with the same primary email address but a different id, delete AWS user and create a new AWS user.")
					if err := s.aws.DeleteUser(aws.UpdateUser(
						awsUser.ID, awsUser.Name.GivenName, awsUser.Name.FamilyName, awsUser.Username, awsUser.Active, awsUser.ExternalId.Id)); err != nil {
						return err
					}
					if _, err := s.aws.CreateUser(aws.NewUser(
						u.Name.GivenName, u.Name.FamilyName, u.PrimaryEmail, !u.Suspended, u.Id)); err != nil {
						return err
					}
				}
			} else {
				// Existing user with no ExternalId: adopt it and sync in place.
				ll.Debug("A local user that matches the primary email address of a google user has been found, adopting to be synced.")
				if _, err := s.aws.UpdateUser(aws.UpdateUser(
					awsUser.ID, u.Name.GivenName, u.Name.FamilyName, u.PrimaryEmail, !u.Suspended, u.Id)); err != nil {
					return err
				}
			}
		} else {
			ll.Info("creating user")
			if _, err := s.aws.CreateUser(aws.NewUser(
				u.Name.GivenName, u.Name.FamilyName, u.PrimaryEmail, !u.Suspended, u.Id)); err != nil {
				return err
			}
		}

		if err := setUser(u.PrimaryEmail); err != nil {
			return err
		}
	}

	// Look for AWS users not found in Google and delete them, unless the
	// RetainUnmatched option is set.
	googleUsersById := make(map[string]struct{}, len(filteredGoogleUsers))
	googleUsersByEmail := make(map[string]struct{}, len(filteredGoogleUsers))
	for _, u := range filteredGoogleUsers {
		googleUsersById[u.Id] = struct{}{}
		googleUsersByEmail[u.PrimaryEmail] = struct{}{}
	}

	for _, awsUser := range awsUsers {
		if _, found := googleUsersById[awsUser.ExternalId.Id]; found {
			continue
		}
		if _, found := googleUsersByEmail[awsUser.Username]; found {
			continue
		}
		if s.cfg.RetainUnmatched {
			log.WithField("awsUser", awsUser).Warn("Retained: deletion suppressed")
			continue
		}
		log.WithField("awsUser", awsUser).Info("deleting user")
		if err := s.aws.DeleteUser(aws.UpdateUser(
			awsUser.ID, awsUser.Name.GivenName, awsUser.Name.FamilyName, awsUser.Username, awsUser.Active, awsUser.ExternalId.Id)); err != nil {
			return err
		}
	}

	return nil
}

// LegacySyncGroups will sync groups from Google -> AWS SSO
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
func (s *syncGSuite) LegacySyncGroups(query string) error {
	log.WithField("query", query).Debug("get google groups")
	googleGroups, err := s.google.GetGroups(query)
	if err != nil {
		return err
	}

	// Source the existing AWS groups from the Identity Store so that their
	// ExternalId is populated. This allows matching on ExternalId (the Google
	// group id) rather than display name alone, mirroring getGroupOperations.
	log.Debug("get existing aws groups")
	awsGroups, err := s.GetGroups()
	if err != nil {
		log.Error("error getting aws groups")
		return err
	}

	awsGroupsByExtId := make(map[string]*interfaces.Group)
	awsGroupsByDisplayName := make(map[string]*interfaces.Group)
	for _, awsGroup := range awsGroups {
		if awsGroup.ExternalId.Id != "" {
			awsGroupsByExtId[awsGroup.ExternalId.Id] = awsGroup
		}
		awsGroupsByDisplayName[awsGroup.DisplayName] = awsGroup
	}

	// Track the display names of the google groups actually synced so that
	// unmatched AWS groups can be reconciled (deleted/retained) afterwards.
	syncedGroupNames := make(map[string]struct{})

	correlatedGroups := make(map[string]*interfaces.Group)

	for _, g := range googleGroups {
		if s.ignoreGroup(g.Email) || !s.includeGroup(g.Email) {
			continue
		}

		syncedGroupNames[g.Email] = struct{}{}

		log := log.WithFields(log.Fields{
			"group": g.Email,
		})

		log.Debug("Check group")
		var group *interfaces.Group

		if awsGroup, found := awsGroupsByExtId[g.Id]; found {
			// Matched by ExternalId (Google id). Update if the display name differs.
			if awsGroup.DisplayName != g.Email {
				log.Debug("update")
				if _, err := s.aws.UpdateGroup(aws.UpdateGroup(awsGroup.ID, g.Email, g.Id)); err != nil {
					return err
				}
				awsGroup.DisplayName = g.Email
			} else {
				log.Debug("equals")
			}
			group = awsGroup
		} else if awsGroup, found := awsGroupsByDisplayName[g.Email]; found {
			// Matched by display name but not by ExternalId: update to backfill
			// (or correct) the ExternalId.
			log.Debug("update")
			if _, err := s.aws.UpdateGroup(aws.UpdateGroup(awsGroup.ID, g.Email, g.Id)); err != nil {
				return err
			}
			awsGroup.ExternalId = interfaces.ExternalId{Id: g.Id}
			group = awsGroup
		} else {
			log.Info("Creating group in AWS")
			newGroup := aws.NewGroup(g.Email, g.Id)
			createdGroup, err := s.aws.CreateGroup(newGroup)
			if err != nil {
				return err
			}
			group = createdGroup
		}

		correlatedGroups[group.DisplayName] = group

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

			// Dry-run users have no ID; the Identity Store rejects an empty
			// memberId.userId, and a user that does not exist is not a member.
			var b *bool
			if u.ID == "" {
				notAMember := false
				b = &notAMember
			} else {
				var err error
				b, err = identitystore.IsMemberInGroups(context.Background(), s.identityStore, &s.cfg.IdentityStoreID, []string{group.ID}, &u.ID)
				if err != nil {
					return err
				}
			}

			if _, ok := memberList[u.Username]; ok {
				if !*b {
					log.WithField("user", u.Username).Info("Adding user to group")
					err := s.aws.AddUserToGroup(u, group)
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

	// Look for AWS groups not matched to a synced Google group and delete
	// them, unless the RetainUnmatched option is set.
	googleGroupsById := make(map[string]struct{})
	for _, g := range googleGroups {
		if s.ignoreGroup(g.Email) || !s.includeGroup(g.Email) {
			continue
		}
		googleGroupsById[g.Id] = struct{}{}
	}

	for _, awsGroup := range awsGroups {
		if _, found := googleGroupsById[awsGroup.ExternalId.Id]; found {
			continue
		}
		if _, found := syncedGroupNames[awsGroup.DisplayName]; found {
			continue
		}
		if s.cfg.RetainUnmatched {
			log.WithField("awsGroup", awsGroup).Warn("Retained: deletion suppressed")
			continue
		}
		log.WithField("awsGroup", awsGroup).Info("deleting group")
		if err := s.aws.DeleteGroup(aws.UpdateGroup(awsGroup.ID, awsGroup.DisplayName, awsGroup.ExternalId.Id)); err != nil {
			return err
		}
	}

	return nil
}

// Sync will sync groups and its members from Google -> AWS SSO SCIM
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
func (s *syncGSuite) Sync(queryGroups string, queryUsers string) error {

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
	addAWSUsers, delAWSUsers, updateAWSUsers, _ := getUserOperations(awsUsers, googleUsers, s.cfg.ForceExternalIdUpdate, s.cfg.RetainUnmatched, s.cfg.IssuerArn)
	addAWSGroups, delAWSGroups, updateAWSGroups, unchangedAWSGroups := getGroupOperations(awsGroups, googleGroups, s.cfg.ForceExternalIdUpdate, s.cfg.RetainUnmatched, s.cfg.IssuerArn)

	// update aws users (updated in google)
	log.Debug("updating aws users updated in google")
	for _, awsUser := range updateAWSUsers {

		log := log.WithFields(log.Fields{"user": awsUser.Username})

		log.Warn("updating user")
		_, err = s.aws.UpdateUser(aws.UpdateUser(
			awsUser.ID,
			awsUser.Name.GivenName,
			awsUser.Name.FamilyName,
			awsUser.Username,
			awsUser.Active,
			awsUser.ExternalId.Id))
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
	addUsersToGroup, deleteUsersFromGroup, unchangedUsersInGroup, err := s.getGroupUsersOperations(googleGroupsUsers, awsGroupsUsers)
	if err != nil {
		return err
	}

	// validate groups members are equal in aws and google
	log.Debug("validating groups members, equals in aws and google")
	for _, awsGroup := range unchangedAWSGroups {
		// add members of the new group
		log.WithFields(
			log.Fields{
				"group":     awsGroup.DisplayName,
				"add":       len(addUsersToGroup[awsGroup.DisplayName]),
				"remove":    len(deleteUsersFromGroup[awsGroup.DisplayName]),
				"unchanged": len(unchangedUsersInGroup[awsGroup.DisplayName])}).Info("Change Summary: Group Members")

		for _, awsUser := range addUsersToGroup[awsGroup.DisplayName] {
			log.WithField("user", awsUser.Username).Info("adding user to group")
			err = s.aws.AddUserToGroup(awsUser, awsGroup)
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
	s.outputCacheMetrics()
	log.Info("sync completed")

	return nil
}

// getGoogleGroupsAndUsers return a list of google users members of googleGroups
// and a map of google groups and its users' list
func (s *syncGSuite) getGoogleGroupsAndUsers(queryGroups string, queryUsers string) ([]*admin.Group, []*admin.User, map[string][]*admin.User, error) {
	funcName := "getGoogleGroupsAndUsers"
	gUsers := make([]*admin.User, 0)
	gGroupsUsers := make(map[string][]*admin.User)
	gUserDetailCache := make(map[string]*admin.User)
	gGroupDetailCache := make(map[string]*admin.Group)
	gUniqUsers := make(map[string]*admin.User)
	gGroups := make([]*admin.Group, 0)

	log.WithFields(log.Fields{
		"func":        funcName,
		"queryGroups": queryGroups,
		"queryUsers":  queryUsers,
	}).Debug("getGoogleGroupsAndUsers()")

	// For larger directories this will reduce execution time and avoid throttling limits
	// however if you have directory with 10,000+ users you may want to down scope
	// this to a specific OU path or disable by leaving empty.
	if s.cfg.PrecacheOrgUnits == nil {
		log.WithFields(log.Fields{
			"func": funcName,
		}).Info("Precaching DISABLED, caching groups on the fly")
	} else {
		log.WithFields(log.Fields{
			"func": funcName,
		}).Info("Precache all Groups from Google")

		googleGroups, err := s.google.GetGroups("*")
		if err != nil {
			log.WithFields(log.Fields{
				"func":  funcName,
				"error": err,
			}).Error("failed precaching groups from Google")
			return nil, nil, nil, err
		}
		for _, g := range googleGroups {
			gGroupDetailCache[g.Email] = g
		}
	}

	// For larger directories this will reduce execution time and avoid throttling limits
	// however if you have directory with 10,000+ users you may want to down scope
	// this to a specific OU path or disable by leaving empty.
	if s.cfg.PrecacheOrgUnits == nil {
		log.WithFields(log.Fields{
			"func":         funcName,
			"OrgUnitPaths": s.cfg.PrecacheOrgUnits,
		}).Info("Precaching DISABLED, caching users on the fly")
	} else {
		precacheQueries := ""
		log.WithFields(log.Fields{
			"func":         funcName,
			"OrgUnitPaths": s.cfg.PrecacheOrgUnits,
		}).Info("Precache users from these paths")

		for _, orgUnitPath := range s.cfg.PrecacheOrgUnits {
			log.WithFields(log.Fields{
				"func":        funcName,
				"orgUnitPath": orgUnitPath,
			}).Debug("format into query string")

			orgUnitPath = strings.TrimSpace(orgUnitPath)
			if orgUnitPath != "/" {
				orgUnitPath = strings.TrimSuffix(orgUnitPath, "/")
			}
			if strings.ContainsRune(orgUnitPath, ' ') {
				precacheQueries = precacheQueries + ",OrgUnitPath='" + orgUnitPath + "'"
			} else {
				precacheQueries = precacheQueries + ",OrgUnitPath=" + orgUnitPath
			}
		}
		log.WithFields(log.Fields{
			"func":            funcName,
			"precacheQueries": precacheQueries,
			"queryFilters":    s.cfg.UserFilter,
		}).Info("Precaching users")

		googleUsers, err := s.google.GetUsers(precacheQueries, s.cfg.UserFilter)
		if err != nil {
			log.WithFields(log.Fields{
				"func":  funcName,
				"error": err,
			}).Error("Precaching failed, caching on the fly")

		} else if len(googleUsers) == 0 {
			log.WithFields(log.Fields{
				"func": funcName,
			}).Warn("Precaching return no users? Switching to caching on the fly")

		} else {
			for _, u := range googleUsers {
				log.WithFields(log.Fields{
					"func": funcName,
					"user": u,
				}).Debug("process user")

				if _, found := gUniqUsers[u.PrimaryEmail]; !found {
					log.WithFields(log.Fields{
						"func":    funcName,
						"user.Id": u.Id,
					}).Debug("Cache user")
					s.recordCacheOp(u.OrgUnitPath, PreCache)

					gUserDetailCache[u.PrimaryEmail] = u
					continue
				} else {
					log.WithFields(log.Fields{
						"func":    funcName,
						"user.Id": u.Id,
					}).Debug("already in cache")
					s.recordCacheOp(u.OrgUnitPath, CacheHit)
					continue
				}
			}
		}
	}

	// Fetch Users
	if queryUsers == "" {
		log.WithFields(log.Fields{
			"func": funcName,
		}).Info("Skipping fetch for userMatch")
	} else {
		log.WithFields(log.Fields{
			"func":         funcName,
			"queryUsers":   queryUsers,
			"queryFilters": s.cfg.UserFilter,
		}).Info("fetching userMatch")

		googleUsers, err := s.google.GetUsers(queryUsers, s.cfg.UserFilter)
		if err != nil {
			log.WithFields(log.Fields{
				"func":  funcName,
				"error": err,
			}).Error("failed fetching userMatch from Google")
			return nil, nil, nil, err
		}

		log.WithFields(log.Fields{
			"func": funcName,
		}).Debug("process users from google, filtering as required")

		for _, u := range googleUsers {
			log.WithFields(log.Fields{
				"func": funcName,
				"user": u,
			}).Debug("process user")

			// Remove any users that should be ignored
			if s.ignoreUser(u.PrimaryEmail) {
				log.WithFields(log.Fields{
					"func":    funcName,
					"user.Id": u.Id,
				}).Info("ignoring user")
				continue
			}

			if _, found := gUniqUsers[u.PrimaryEmail]; !found {
				log.WithFields(log.Fields{
					"func":    funcName,
					"user.Id": u.Id,
				}).Debug("adding user")
				gUserDetailCache[u.PrimaryEmail] = u
				s.recordCacheOp(u.OrgUnitPath, CacheMiss)
				gUniqUsers[u.PrimaryEmail] = gUserDetailCache[u.PrimaryEmail]
				continue
			} else {
				log.WithFields(log.Fields{
					"func":    funcName,
					"user.Id": u.Id,
				}).Debug("already existing")
				s.recordCacheOp(u.OrgUnitPath, CacheHit)
				continue
			}
		}
	}

	// Fetch Users
	if queryGroups == "" {
		log.WithFields(log.Fields{
			"func": funcName,
		}).Info("Skipping fetch for groupMatch")
	} else {
		log.WithFields(log.Fields{
			"func":        funcName,
			"queryGroups": queryGroups,
		}).Info("fetching groups")

		googleGroups, err := s.google.GetGroups(queryGroups)
		if err != nil {
			log.WithFields(log.Fields{
				"func":  funcName,
				"error": err,
			}).Error("failed fetching groups")
			return nil, nil, nil, err
		}

		filteredGoogleGroups := []*admin.Group{}
		log.WithFields(log.Fields{
			"func": funcName,
		}).Info("filter groups by ignoreList")

		for _, g := range googleGroups {
			log.WithFields(log.Fields{
				"func":  funcName,
				"group": g,
			}).Debug("processing group")

			if s.ignoreGroup(g.Email) {
				log.WithFields(log.Fields{
					"func":     funcName,
					"group.Id": g.Id,
				}).Info("ignoring group")
				continue
			}
			filteredGoogleGroups = append(filteredGoogleGroups, g)
		}
		gGroups = filteredGoogleGroups

		log.WithField("func", funcName).Info("fetch group memberships")
		for _, g := range gGroups {
			log.WithFields(log.Fields{
				"func":  funcName,
				"group": g,
			}).Debug("processing group")

			if s.ignoreGroup(g.Email) {
				log.WithFields(log.Fields{
					"func":     funcName,
					"group.Id": g.Id,
				}).Info("skipping group")
				continue
			}

			log.WithField("func", funcName).Debug("fetch membership")
			membersUsers, err := s.getGoogleUsersInGroup(g, gUserDetailCache, gGroupDetailCache)
			if err != nil {
				return nil, nil, nil, err
			}

			// If we've not seen the user email address before add it to the list of unique users
			// also, we need to deduplicate the list of members.
			log.WithFields(log.Fields{
				"func":         funcName,
				"group.Id":     g.Id,
				"membersUsers": membersUsers,
			}).Debug("Process group membership")

			gUniqMembers := make(map[string]*admin.User)
			for _, m := range membersUsers {
				log.WithFields(log.Fields{
					"func":     funcName,
					"group.Id": g.Id,
					"member":   m,
				}).Debug("Processing member")

				if m == nil {
					log.WithFields(log.Fields{
						"func":      funcName,
						"group.Id":  g.Id,
						"member.Id": m.Id,
					}).Error("nil user")
					continue
				}
				if _, found := gUniqUsers[m.PrimaryEmail]; !found {
					log.WithFields(log.Fields{
						"func":     funcName,
						"group.Id": g.Id,
						"member":   m.Id,
					}).Debug("adding user to UniqueUsers")
					gUniqUsers[m.PrimaryEmail] = gUserDetailCache[m.PrimaryEmail]
				}

				if _, found := gUniqMembers[m.PrimaryEmail]; !found {
					log.WithFields(log.Fields{
						"func":     funcName,
						"group.Id": g.Id,
						"member":   m.Id,
					}).Debug("adding user to group")
					gUniqMembers[m.PrimaryEmail] = gUserDetailCache[m.PrimaryEmail]
				}
			}

			log.WithFields(log.Fields{
				"func":     funcName,
				"group.Id": g.Id,
			}).Debug("create gMembers")
			gMembers := make([]*admin.User, 0)
			for _, member := range gUniqMembers {
				gMembers = append(gMembers, member)
			}
			log.WithFields(log.Fields{
				"func":     funcName,
				"group.Id": g.Id,
				"gMembers": gMembers,
			}).Debug("Finished group membership")
			gGroupsUsers[g.Name] = gMembers
		}
	}

	log.WithField("func", funcName).Debug("create gUsers")
	for _, user := range gUniqUsers {
		gUsers = append(gUsers, user)
	}

	log.WithField("func", funcName).Debug("Return")
	return gGroups, gUsers, gGroupsUsers, nil
}

// IssuerCheckMsg is the DEBUG message logged for every user/group whose
// ExternalId Issuer is compared with the IssuerArn ssosync expects (issue #383).
const IssuerCheckMsg = "ExternalId issuer check"

// ExpectedIssuerMsg is the DEBUG message logged once per user/group operations
// pass with the IssuerArn ssosync expects on records it manages.
const ExpectedIssuerMsg = "ExternalId issuer expected"

// ExternalIdsReadMsg is the DEBUG message logged for each Identity Store record
// read with every ExternalId it carries.
const ExternalIdsReadMsg = "ExternalIds read from Identity Store"

// issuerMatches reports whether a record's ExternalId Issuer identifies this
// ssosync SCIM provisioning tenant, i.e. whether ssosync manages the record.
// An empty expected issuer disables the check (everything matches).
//
// The comparison is case-insensitive ON PURPOSE (issue #383). IAM Identity
// Center does not stamp the provisioning tenant id with a consistent case: in a
// live tenant every USER provisioned through SCIM carried
//
//	arn:aws:identitystore::<account>:identitystore/d-xxxxxxxxxx/provisioningtenant/Hw523407fb9-...
//
// (the tenant id as it appears in the SCIM endpoint URL, capital "H"), while
// every GROUP provisioned through the same SCIM tenant carried
//
//	arn:aws:identitystore::<account>:identitystore/d-xxxxxxxxxx/provisioningtenant/hw523407fb9-...
//
// (lowercased). No single spelling of the expected IssuerArn matches both with
// an exact comparison: the old lowercased ARN matched groups and missed users,
// and the case-preserving ARN matches users and misses groups, so either way
// one kind was classified AdoptForeign and rewritten on every run. Two issuers
// that differ only in letter case name the same tenant, so ignoring case is
// safe; the account id (digits) and identity store id still have to match.
// The expected IssuerArn itself is still built with its original case
// (cmd/root.go buildIssuerArn) so logs show the real value, and the DEBUG
// issuer-check line records whether the match was exact.
func issuerMatches(issuer, expectedIssuer string) bool {
	return expectedIssuer == "" || strings.EqualFold(issuer, expectedIssuer)
}

// logIssuerCheck emits the DEBUG line describing how a record's ExternalId is
// compared with the expected issuer. issuerMatch is the result the operation
// decision uses (issuerMatches: case-insensitive, and an empty expected issuer
// disables the check). issuerExactMatch records whether the case also matched,
// so the inconsistent casing IAM Identity Center uses for users and groups
// stays visible. accountMatch/storeAndTenantMatch split the ARN so a wrong
// account id (delegated administrator) is distinguishable from a wrong tenant.
func logIssuerCheck(kind, name, awsId string, ext interfaces.ExternalId, all []interfaces.ExternalId, selectedBy, googleId, expectedIssuer string) {
	fields := log.Fields{
		"externalIdCount":      len(all),
		"externalIdSelectedBy": selectedBy,
		"kind":                 kind,
		"name":                 name,
		"awsId":                awsId,
		"externalId":           ext.Id,
		"issuer":               ext.Issuer,
		"expectedIssuer":       expectedIssuer,
		"googleId":             googleId,
		"externalIdMatch":      googleId != "" && ext.Id == googleId,
		"issuerCheckDisabled":  expectedIssuer == "",
		"issuerMatch":          issuerMatches(ext.Issuer, expectedIssuer),
		"issuerExactMatch":     ext.Issuer == expectedIssuer,
		"caseInsensitiveMatch": strings.EqualFold(ext.Issuer, expectedIssuer),
	}
	gotAcct, gotRest, gotOk := splitIssuerArn(ext.Issuer)
	wantAcct, wantRest, wantOk := splitIssuerArn(expectedIssuer)
	if gotOk && wantOk {
		fields["issuerAccount"] = gotAcct
		fields["expectedAccount"] = wantAcct
		fields["accountMatch"] = gotAcct == wantAcct
		fields["storeAndTenantMatch"] = gotRest == wantRest
	}
	log.WithFields(fields).Debug(IssuerCheckMsg)
}

// splitIssuerArn splits an identitystore provisioning-tenant ARN
// (arn:aws:identitystore::<account>:identitystore/<store>/provisioningtenant/<tenant>)
// into the account id and the remainder after it.
func splitIssuerArn(issuer string) (account, rest string, ok bool) {
	const prefix = "arn:aws:identitystore::"
	after, has := strings.CutPrefix(issuer, prefix)
	if !has {
		return "", "", false
	}
	account, rest, found := strings.Cut(after, ":")
	if !found {
		return "", "", false
	}
	return account, rest, true
}

// logExternalIdsRead emits the DEBUG line listing every ExternalId of a record
// read from the Identity Store. When there are several, the user/group
// operations match on the one issued by the expected issuer (selectExternalId).
func logExternalIdsRead(kind, name, awsId string, externalIds []identitystore_types.ExternalId) {
	ids := make([]map[string]string, 0, len(externalIds))
	for _, e := range externalIds {
		ids = append(ids, map[string]string{"id": derefString(e.Id), "issuer": derefString(e.Issuer)})
	}
	log.WithFields(log.Fields{
		"kind":        kind,
		"name":        name,
		"awsId":       awsId,
		"externalIds": ids,
		"count":       len(externalIds),
	}).Debug(ExternalIdsReadMsg)
}

// getGroupOperation returns the action to perform for this group and the desired group state
func getGroupOperation(awsGroup *interfaces.Group, googleGroup *admin.Group, forceExternalIdUpdate bool, retainUnmatched bool, issuerArn string) (action OperationAction, group *interfaces.Group) {
	log.Debug("getGroupOperation()")

	if awsGroup == nil {
		// Group doesn't exist
		return Create, aws.NewGroup(googleGroup.Name, googleGroup.Id)
	}

	// Match on the ExternalId issued by this SCIM tenant when the group has
	// several (see selectExternalId).
	selectedBy := useManagedGroupExternalId(awsGroup, issuerArn)

	if awsGroup.ExternalId.Id == "" {
		// local (manually created) group, it has no ExternalId
		if googleGroup == nil {
			if retainUnmatched {
				// group not found in Google but retain flag enabled
				return Retain, aws.UpdateGroup(awsGroup.ID, awsGroup.DisplayName, awsGroup.ExternalId.Id)
			}
			// group not found in Google
			return Delete, awsGroup
		}
		// matching Google group found, adopt the local group
		return AdoptLocal, aws.UpdateGroup(awsGroup.ID, googleGroup.Name, googleGroup.Id)
	}

	googleGroupId := ""
	if googleGroup != nil {
		googleGroupId = googleGroup.Id
	}
	logIssuerCheck("group", awsGroup.DisplayName, awsGroup.ID, awsGroup.ExternalId, awsGroup.ExternalIds, selectedBy, googleGroupId, issuerArn)

	if issuerMatches(awsGroup.ExternalId.Issuer, issuerArn) {
		// ssosync managed group (issuer compared case-insensitively, see issuerMatches)
		if googleGroup == nil {
			// group not found in Google
			return Delete, awsGroup
		}
		if awsGroup.ExternalId.Id == googleGroup.Id {
			// group matches on ExternalId
			if awsGroup.DisplayName == googleGroup.Name {
				// no action required
				return Equal, awsGroup
			}
			// group needs updating
			return Update, aws.UpdateGroup(awsGroup.ID, googleGroup.Name, googleGroup.Id)
		}
		// both groups exist but ExternalId doesn't match
		// have they been deleted and recreated?
		if forceExternalIdUpdate {
			// Force Retention of existing group and update External Id to match
			// this may lead to privilege inheritence
			return UpdateForced, aws.UpdateGroup(awsGroup.ID, googleGroup.Name, googleGroup.Id)
		}
		// Do the safe thing and delete the existing group and create a new one to avoid privilege inheritence
		return Recreate, aws.NewGroup(googleGroup.Name, googleGroup.Id)
	}

	// There must be an ExternalId setup, its come from a different SCIM provisioning instance/external IdP
	if googleGroup == nil {
		if retainUnmatched {
			// group not found in Google but retain flag enabled
			return Retain, aws.UpdateGroup(awsGroup.ID, awsGroup.DisplayName, awsGroup.ExternalId.Id)
		}
		// group not found in Google
		return Delete, awsGroup
	}
	return AdoptForeign, aws.UpdateGroup(awsGroup.ID, googleGroup.Name, googleGroup.Id)
}

// getGroupOperations returns the groups of AWS that must be added, deleted, updated, are equals and retained
func getGroupOperations(awsGroups []*interfaces.Group, googleGroups []*admin.Group, forceExternalIdUpdate bool, retainUnmatched bool, issuerArn string) (add []*interfaces.Group, delete []*interfaces.Group, update []*interfaces.Group, equals []*interfaces.Group) {

	log.Debug("getGroupOperations()")
	log.WithFields(log.Fields{"kind": "group", "expectedIssuer": issuerArn, "issuerCheckDisabled": issuerArn == ""}).Debug(ExpectedIssuerMsg)

	stats := struct {
		Equal, Create, AdoptLocal, AdoptForeign, Update, UpdateForced, Delete, Recreate, Retain int
	}{}

	awsMap := make(map[string]*interfaces.Group)
	awsMapExtId := make(map[string]*interfaces.Group)
	googleMap := make(map[string]*admin.Group)
	googleMapId := make(map[string]*admin.Group)

	for _, awsGroup := range awsGroups {
		// Index (and later match) on the ExternalId issued by this SCIM tenant.
		useManagedGroupExternalId(awsGroup, issuerArn)
		awsMap[strings.ToLower(awsGroup.DisplayName)] = awsGroup
		if awsGroup.ExternalId.Id != "" {
			awsMapExtId[awsGroup.ExternalId.Id] = awsGroup
		}
	}

	for _, gGroup := range googleGroups {
		googleMap[strings.ToLower(gGroup.Name)] = gGroup
		googleMapId[gGroup.Id] = gGroup
	}

	// Look for Google groups in AWS
	for _, gGroup := range googleGroups {
		var awsGroup *interfaces.Group

		if match, found := awsMapExtId[gGroup.Id]; found {
			// Found by ExternalId.Id
			awsGroup = match
		} else if match, found := awsMap[strings.ToLower(gGroup.Name)]; found {
			// Found by DisplayName
			awsGroup = match
		}

		// awsGroup is nil when the Google group was not found in AWS at all
		action, group := getGroupOperation(awsGroup, gGroup, forceExternalIdUpdate, retainUnmatched, issuerArn)

		log.WithField("group", group).Info(action)

		switch action {
		case Equal:
			stats.Equal++
			equals = append(equals, group)
		case Create:
			stats.Create++
			add = append(add, group)
		case Delete:
			stats.Delete++
			delete = append(delete, awsGroup)
		case Recreate:
			stats.Recreate++
			log.WithField("group", group).Warn(action)
			delete = append(delete, awsGroup)
			add = append(add, group)
		case UpdateForced:
			stats.UpdateForced++
			log.WithField("group", group).Warn(action)
			update = append(update, group)
		case Update:
			stats.Update++
			update = append(update, group)
		case AdoptLocal:
			stats.AdoptLocal++
			update = append(update, group)
		case AdoptForeign:
			stats.AdoptForeign++
			update = append(update, group)
		case Retain:
			stats.Retain++
			update = append(update, group)
		}
	}

	// Look for AWS groups not found in Google
	for _, awsGroup := range awsGroups {
		if _, found := googleMapId[awsGroup.ExternalId.Id]; found {
			continue
		}
		if _, found := googleMap[strings.ToLower(awsGroup.DisplayName)]; found {
			continue
		}

		action, group := getGroupOperation(awsGroup, nil, forceExternalIdUpdate, retainUnmatched, issuerArn)

		log.WithField("group", group).Info(action)

		switch action {
		case Delete:
			stats.Delete++
			delete = append(delete, group)
		case Retain:
			stats.Retain++
			update = append(update, group)
		}
	}

	log.WithFields(
		log.Fields{
			"equal":        stats.Equal,
			"create":       stats.Create,
			"adoptLocal":   stats.AdoptLocal,
			"adoptForeign": stats.AdoptForeign,
			"update":       stats.Update,
			"updateForced": stats.UpdateForced,
			"delete":       stats.Delete,
			"recreate":     stats.Recreate,
			"retain":       stats.Retain}).Info("Change Summary: Groups")

	return add, delete, update, equals
}

// getUserOperation returns the action to perform for this user and the desired user state
func getUserOperation(awsUser *interfaces.User, googleUser *admin.User, forceExternalIdUpdate bool, retainUnmatched bool, issuerArn string) (action OperationAction, user *interfaces.User) {
	log.Debug("getUserOperation()")

	if awsUser == nil {
		// User doesn't exist
		return Create, aws.NewUser(googleUser.Name.GivenName, googleUser.Name.FamilyName, googleUser.PrimaryEmail, !googleUser.Suspended, googleUser.Id)
	}

	// Match on the ExternalId issued by this SCIM tenant when the user has
	// several (see selectExternalId).
	selectedBy := useManagedUserExternalId(awsUser, issuerArn)

	if awsUser.ExternalId.Id == "" {
		// local (manually created) user, it has no ExternalId
		if googleUser == nil {
			if retainUnmatched {
				// user not found in Google but retain flag enabled
				return Retain, aws.UpdateUser(awsUser.ID, awsUser.Name.GivenName, awsUser.Name.FamilyName, awsUser.Username, false, awsUser.ExternalId.Id)
			}
			// user not found in Google
			return Delete, awsUser
		}
		// matching Google user found, adopt the local user
		return AdoptLocal, aws.UpdateUser(awsUser.ID, googleUser.Name.GivenName, googleUser.Name.FamilyName, googleUser.PrimaryEmail, !googleUser.Suspended, googleUser.Id)
	}

	googleUserId := ""
	if googleUser != nil {
		googleUserId = googleUser.Id
	}
	logIssuerCheck("user", awsUser.Username, awsUser.ID, awsUser.ExternalId, awsUser.ExternalIds, selectedBy, googleUserId, issuerArn)

	if issuerMatches(awsUser.ExternalId.Issuer, issuerArn) {
		// ssosync managed user (issuer compared case-insensitively, see issuerMatches)
		if googleUser == nil {
			// user not found in Google
			return Delete, awsUser
		}
		if awsUser.ExternalId.Id == googleUser.Id {
			// user matches on ExternalId
			if awsUser.Active == !googleUser.Suspended &&
				awsUser.Username == googleUser.PrimaryEmail &&
				awsUser.Name.GivenName == googleUser.Name.GivenName &&
				awsUser.Name.FamilyName == googleUser.Name.FamilyName {
				// no action required
				return Equal, awsUser
			}
			// user needs updating
			return Update, aws.UpdateUser(awsUser.ID, googleUser.Name.GivenName, googleUser.Name.FamilyName, googleUser.PrimaryEmail, !googleUser.Suspended, googleUser.Id)
		}
		// both users exist but ExternalId doesn't match
		// have they been deleted and recreated?
		if forceExternalIdUpdate {
			// Force Retention of existing user and update External Id to match
			// this may lead to privilege inheritence
			return UpdateForced, aws.UpdateUser(awsUser.ID, googleUser.Name.GivenName, googleUser.Name.FamilyName, googleUser.PrimaryEmail, !googleUser.Suspended, googleUser.Id)
		}
		// Do the safe thing and delete the existing user and create a new one to avoid privilege inheritence
		return Recreate, aws.UpdateUser(awsUser.ID, googleUser.Name.GivenName, googleUser.Name.FamilyName, googleUser.PrimaryEmail, !googleUser.Suspended, googleUser.Id)
	}

	// There must be an ExternalId setup, its come from a different SCIM provisioning instance/external IdP
	if googleUser == nil {
		if retainUnmatched {
			// user not found in Google but retain flag enabled
			return Retain, aws.UpdateUser(awsUser.ID, awsUser.Name.GivenName, awsUser.Name.FamilyName, awsUser.Username, false, awsUser.ExternalId.Id)
		}
		// user not found in Google
		return Delete, awsUser
	}
	return AdoptForeign, aws.UpdateUser(awsUser.ID, googleUser.Name.GivenName, googleUser.Name.FamilyName, googleUser.PrimaryEmail, !googleUser.Suspended, googleUser.Id)
}

// getUserOperations returns the users of AWS that must be added, deleted, updated, are equals and retained
func getUserOperations(awsUsers []*interfaces.User, googleUsers []*admin.User, forceExternalIdUpdate bool, retainUnmatched bool, issuerArn string) (add []*interfaces.User, delete []*interfaces.User, update []*interfaces.User, equals []*interfaces.User) {

	log.Debug("getUserOperations()")
	log.WithFields(log.Fields{"kind": "user", "expectedIssuer": issuerArn, "issuerCheckDisabled": issuerArn == ""}).Debug(ExpectedIssuerMsg)

	stats := struct {
		Equal, Create, AdoptLocal, AdoptForeign, Update, UpdateForced, Delete, Recreate, Retain int
	}{}

	awsMapUsername := make(map[string]*interfaces.User)
	awsMapExtId := make(map[string]*interfaces.User)
	googleMapEmail := make(map[string]*admin.User)
	googleMapId := make(map[string]*admin.User)

	for _, awsUser := range awsUsers {
		// Index (and later match) on the ExternalId issued by this SCIM tenant.
		useManagedUserExternalId(awsUser, issuerArn)
		awsMapUsername[strings.ToLower(awsUser.Username)] = awsUser
		if awsUser.ExternalId.Id != "" {
			awsMapExtId[awsUser.ExternalId.Id] = awsUser
		}
	}

	for _, gUser := range googleUsers {
		googleMapId[gUser.Id] = gUser
		googleMapEmail[strings.ToLower(gUser.PrimaryEmail)] = gUser
	}

	// Look for Google users in AWS
	for _, gUser := range googleUsers {
		var awsUser *interfaces.User

		if match, found := awsMapExtId[gUser.Id]; found {
			// Found by ExternalId.Id
			awsUser = match
		} else if match, found := awsMapUsername[strings.ToLower(gUser.PrimaryEmail)]; found {
			// Found by PrimaryEmail/username
			awsUser = match
		}

		// awsUser is nil when the Google user was not found in AWS at all
		action, user := getUserOperation(awsUser, gUser, forceExternalIdUpdate, retainUnmatched, issuerArn)

		log.WithField("user", user).Info(action)

		switch action {
		case Equal:
			stats.Equal++
			equals = append(equals, user)
		case Create:
			stats.Create++
			add = append(add, user)
		case Delete:
			stats.Delete++
			delete = append(delete, awsUser)
		case Recreate:
			stats.Recreate++
			log.WithField("user", user).Warn(action)
			delete = append(delete, awsUser)
			add = append(add, user)
		case UpdateForced:
			stats.UpdateForced++
			log.WithField("user", user).Warn(action)
			update = append(update, user)
		case Update:
			stats.Update++
			update = append(update, user)
		case AdoptLocal:
			stats.AdoptLocal++
			update = append(update, user)
		case AdoptForeign:
			stats.AdoptForeign++
			update = append(update, user)
		case Retain:
			stats.Retain++
			update = append(update, user)
		}
	}

	// Look for AWS users not found in Google
	for _, awsUser := range awsUsers {
		if _, found := googleMapId[awsUser.ExternalId.Id]; found {
			continue
		}
		if _, found := googleMapEmail[strings.ToLower(awsUser.Username)]; found {
			continue
		}

		action, user := getUserOperation(awsUser, nil, forceExternalIdUpdate, retainUnmatched, issuerArn)

		log.WithField("user", user).Info(action)

		switch action {
		case Delete:
			stats.Delete++
			delete = append(delete, user)
		case Retain:
			stats.Retain++
			update = append(update, user)
		}
	}

	log.WithFields(
		log.Fields{
			"equal":        stats.Equal,
			"create":       stats.Create,
			"adoptLocal":   stats.AdoptLocal,
			"adoptForeign": stats.AdoptForeign,
			"update":       stats.Update,
			"updateForced": stats.UpdateForced,
			"delete":       stats.Delete,
			"recreate":     stats.Recreate,
			"retain":       stats.Retain}).Info("Change Summary: Users")

	return add, delete, update, equals
}

// groupUsersOperations returns the groups and its users of AWS that must be delete from these groups and what are equals
func (s *syncGSuite) getGroupUsersOperations(gGroupsUsers map[string][]*admin.User, awsGroupsUsers map[string][]*interfaces.User) (add map[string][]*interfaces.User, delete map[string][]*interfaces.User, equals map[string][]*interfaces.User, err error) {
	log.Debug("getGroupUsersOperations()")
	mbG := make(map[string]map[string]struct{})
	mbA := make(map[string]map[string]struct{})

	for gGroupName, gGroupUsers := range gGroupsUsers {
		mbG[gGroupName] = make(map[string]struct{})
		for _, gUser := range gGroupUsers {
			mbG[gGroupName][gUser.PrimaryEmail] = struct{}{}
		}
	}

	for awsGroupName, awsGroupUsers := range awsGroupsUsers {
		mbA[awsGroupName] = make(map[string]struct{})
		for _, awsUser := range awsGroupUsers {
			mbA[awsGroupName][awsUser.Username] = struct{}{}
		}
	}

	add = make(map[string][]*interfaces.User)
	// get user in google groups that are in aws groups and
	// users in aws groups that aren't in google groups
	for gGroupName, gGroupUsers := range gGroupsUsers {
		for _, gUser := range gGroupUsers {
			if _, found := mbA[gGroupName][gUser.PrimaryEmail]; !found {
				log.Debug("finding user")
				awsUserFull, err := s.aws.FindUserByEmail(gUser.PrimaryEmail)
				if err != nil {
					return nil, nil, nil, err
				}
				add[gGroupName] = append(add[gGroupName], awsUserFull)
			}
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

	return add, delete, equals, nil
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

	googleClient, err := google.NewClient(ctx, cfg.GoogleAdmin, cfg.CustomerID, creds)
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
	log.WithField("Groups", response).Info("Test call to Identity Store successful")

	// Initialize sync client with
	// 1. SCIM API client
	// 2. Google Directory API client
	// 3. Identity Store Public API client
	c := New(cfg, awsScimClient, googleClient, finalIdentityStoreClient)

	log.WithField("sync_method", cfg.SyncMethod).Info("syncing")
	if cfg.SyncMethod == config.DefaultSyncMethod {
		err = c.Sync(cfg.GroupMatch, cfg.UserMatch)
		if err != nil {
			return err
		}
	} else {
		if cfg.UserMatch != "" {
			err = c.LegacySyncUsers(cfg.UserMatch)
			if err != nil {
				return err
			}
		}

		if cfg.GroupMatch != "" {
			err = c.LegacySyncGroups(cfg.GroupMatch)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *syncGSuite) ignoreUser(name string) bool {
	if s.cfg.IgnoreUsers == nil {
		return false
	}

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
	if s.cfg.IgnoreGroups == nil {
		return false
	}

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
	if s.cfg.IncludeGroups == nil {
		return false
	}

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
	log.Debug("ConvertIdentityStoreGroupToAWSGroup()")
	if group.GroupId == nil {
		log.WithField("group", group).Warn("ConvertIdentityStoreGroupToAWSGroup() Group has no GroupId")
		return nil
	}
	if group.DisplayName == nil {
		log.WithField("group", group).Warn("ConvertIdentityStoreGroupToAWSGroup() Group has no DisplayName")
		return nil
	}
	log.WithField("groupId", *group.GroupId).WithField("displayName", *group.DisplayName).Debug("ConvertIdentityStoreGroupToAWSGroup() Group converted")
	logExternalIdsRead("group", *group.DisplayName, *group.GroupId, group.ExternalIds)
	return &interfaces.Group{
		ID:          *group.GroupId,
		ExternalId:  getExternalId(group.ExternalIds),
		ExternalIds: getExternalIds(group.ExternalIds),
		Schemas:     []string{constants.SCIMSchemaGroup},
		DisplayName: *group.DisplayName,
		Members:     []string{},
	}
}

func (s *syncGSuite) GetGroups() ([]*interfaces.Group, error) {
	log.Debug("GetGroups()")
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
	log.Debug("GetUsers()")
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
	log.Debug("ConvertSdkUserObjToNative()")
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
		addressType := ""
		if address.Type != nil {
			addressType = *address.Type
		}
		userAddresses = append(userAddresses, interfaces.UserAddress{
			Type: addressType,
		})
	}

	logExternalIdsRead("user", derefString(user.UserName), derefString(user.UserId), user.ExternalIds)

	// Safely dereference optional pointer fields; the Identity Store SDK leaves
	// these as nil when a value is not set (e.g. users created outside ssosync).
	familyName := ""
	givenName := ""
	if user.Name != nil {
		if user.Name.FamilyName != nil {
			familyName = *user.Name.FamilyName
		}
		if user.Name.GivenName != nil {
			givenName = *user.Name.GivenName
		}
	}

	return &interfaces.User{
		ID:          derefString(user.UserId),
		ExternalId:  getExternalId(user.ExternalIds),
		ExternalIds: getExternalIds(user.ExternalIds),
		Schemas:     []string{constants.SCIMSchemaUser},
		Username:    derefString(user.UserName),
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{
			FamilyName: familyName,
			GivenName:  givenName,
		},
		DisplayName: derefString(user.DisplayName),
		Emails:      userEmails,
		Addresses:   userAddresses,
	}
}

// derefString safely dereferences a string pointer, returning an empty string
// when the pointer is nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// getExternalId extracts the external identifier from the identity store
// ExternalIds slice, consuming the full structure (both Id and Issuer).
// Returns a zero-value interfaces.ExternalId if no external IDs are present.
// The Id and Issuer pointers are dereferenced nil-safely.
func getExternalId(externalIds []identitystore_types.ExternalId) interfaces.ExternalId {
	if len(externalIds) > 0 {
		return interfaces.ExternalId{
			Id:     derefString(externalIds[0].Id),
			Issuer: derefString(externalIds[0].Issuer),
		}
	}
	return interfaces.ExternalId{}
}

// getExternalIds converts every Identity Store ExternalId, nil-safely.
func getExternalIds(externalIds []identitystore_types.ExternalId) []interfaces.ExternalId {
	if len(externalIds) == 0 {
		return nil
	}
	out := make([]interfaces.ExternalId, 0, len(externalIds))
	for _, e := range externalIds {
		out = append(out, interfaces.ExternalId{Id: derefString(e.Id), Issuer: derefString(e.Issuer)})
	}
	return out
}

// How selectExternalId chose the ExternalId used for matching (logged).
const (
	externalIdSelectedNone        = "none"        // the record has no ExternalId
	externalIdSelectedOnly        = "only"        // exactly one ExternalId
	externalIdSelectedIssuerMatch = "issuerMatch" // the entry issued by the expected issuer
	externalIdSelectedFirst       = "first"       // several, none from the expected issuer
)

// selectExternalId picks the ExternalId a user or group is matched on. An
// Identity Store record can carry more than one ExternalId; ssosync manages the
// one issued by its own SCIM provisioning tenant (expectedIssuer), which need
// not be the first. When no entry has the expected issuer (or the issuer check
// is disabled) the current ExternalId (the first entry) is kept, which the
// operation then classifies as foreign/local as before.
func selectExternalId(current interfaces.ExternalId, all []interfaces.ExternalId, expectedIssuer string) (interfaces.ExternalId, string) {
	switch {
	case len(all) == 0:
		if current.Id == "" && current.Issuer == "" {
			return current, externalIdSelectedNone
		}
		return current, externalIdSelectedOnly
	case len(all) == 1:
		return all[0], externalIdSelectedOnly
	}
	if expectedIssuer != "" {
		for _, e := range all {
			// Same rule as the managed/foreign decision: case-insensitive,
			// because IAM Identity Center's casing of the tenant id differs
			// between users and groups (see issuerMatches).
			if issuerMatches(e.Issuer, expectedIssuer) {
				return e, externalIdSelectedIssuerMatch
			}
		}
	}
	return all[0], externalIdSelectedFirst
}

// useManagedUserExternalId sets u.ExternalId to the entry selected by
// selectExternalId and returns how it was chosen. Idempotent.
func useManagedUserExternalId(u *interfaces.User, expectedIssuer string) string {
	ext, by := selectExternalId(u.ExternalId, u.ExternalIds, expectedIssuer)
	u.ExternalId = ext
	return by
}

// useManagedGroupExternalId is useManagedUserExternalId for groups.
func useManagedGroupExternalId(g *interfaces.Group, expectedIssuer string) string {
	ext, by := selectExternalId(g.ExternalId, g.ExternalIds, expectedIssuer)
	g.ExternalId = ext
	return by
}

// CreateUserIDtoUserObjMap
// Create User ID for user object map
func CreateUserIDtoUserObjMap(awsUsers []*interfaces.User) map[string]*interfaces.User {
	log.Debug("CreateUserIDtoUserObjMap()")
	awsUsersMap := make(map[string]*interfaces.User)

	for _, awsUser := range awsUsers {
		awsUsersMap[awsUser.ID] = awsUser
	}

	return awsUsersMap
}

// ListGroupMembershipPagesCallbackFn
// Handler for Paginated Group Membership List
func (s *syncGSuite) GetGroupMembershipsLists(awsGroups []*interfaces.Group, awsUsersMap map[string]*interfaces.User) (map[string][]*interfaces.User, error) {
	log.Debug("GetGroupMembershipsLists()")
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
	log.Debug("GetGroupMembershipsLists() Return")
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

func (s *syncGSuite) getGoogleUsersInGroup(group *admin.Group, userCache map[string]*admin.User, groupCache map[string]*admin.Group) ([]*admin.User, error) {
	funcName := "getGoogleUsersInGroup"
	log.WithFields(log.Fields{
		"func":  funcName,
		"Group": group,
	}).Debug("getGoogleUsersInGroup()")

	// retrieve the members of the group
	groupMembers, err := s.google.GetGroupMembers(group)
	if err != nil {
		log.WithFields(log.Fields{
			"func":    funcName,
			"GroupId": group.Id,
			"Error":   err,
		}).Error("failed retrieving membership")
		return nil, err
	}
	membersUsers := make([]*admin.User, 0)

	log.WithFields(log.Fields{
		"func":      funcName,
		"GroupId":   group.Id,
		"# Members": len(groupMembers),
		"Members":   groupMembers,
	}).Debug("Group membership")

	// process the members of the group
	for memberIndex, m := range groupMembers {
		log.WithFields(log.Fields{
			"func":    funcName,
			"GroupId": group.Id,
			"Member#": memberIndex,
		}).Debug("Parsing member")

		if len(m.Email) == 0 {
			log.WithFields(log.Fields{
				"func":    funcName,
				"GroupId": group.Id,
				"Member#": memberIndex,
			}).Info("skip: no email address")
			continue
		}

		if m.Type != "USER" {
			log.WithFields(log.Fields{
				"func":    funcName,
				"GroupId": group.Id,
				"Member#": memberIndex,
			}).Info("skip: non-USER member")
			continue
		}

		// Ignore any external members, since they don't have users
		// that can be synced
		if len(m.Status) == 0 {
			log.WithFields(log.Fields{
				"func":    funcName,
				"GroupId": group.Id,
				"Member#": memberIndex,
			}).Info("skip: external user")
			continue
		}

		// Ignore any external members, since they don't have users
		// that can be synced
		if m.Status == "SUSPENDED" && !s.cfg.SyncSuspended {
			log.WithFields(log.Fields{
				"func":    funcName,
				"GroupId": group.Id,
				"Member#": memberIndex,
			}).Info("skip: suspended user")
			continue
		}

		// Remove any users that should be ignored
		if s.ignoreUser(m.Email) {
			log.WithFields(log.Fields{
				"func":    funcName,
				"GroupId": group.Id,
				"Member#": memberIndex,
			}).Info("skip: ignore list")
			continue
		}

		// Ignore any users that don't have a valid status
		allowedStatus := map[string]bool{"ACTIVE": true, "SUSPENDED": true}
		if !allowedStatus[m.Status] {
			log.WithFields(log.Fields{
				"func":    funcName,
				"GroupId": group.Id,
				"Member#": memberIndex,
			}).Info("skip: !ACTIVE")
			continue
		}

		// This is a member that should be synced to AWS
		log.WithFields(log.Fields{
			"func":    funcName,
			"GroupId": group.Id,
			"Member#": memberIndex,
		}).Debug("valid member")
		var memberEmail string
		// Find the group member in the cache of UserDetails
		if _, found := userCache[m.Email]; !found {
			log.WithFields(log.Fields{
				"func":    funcName,
				"GroupId": group.Id,
				"Member#": memberIndex,
			}).Debug("Cache: user not found")
			// Looking up the user based on the member email address,
			// it might be an alias
			googleUsers, err := s.google.GetUsers("email="+m.Email, s.cfg.UserFilter)
			if err != nil {
				log.WithFields(log.Fields{
					"func":         funcName,
					"error":        err,
					"Member.Email": m.Email,
				}).Error("Fetching user")
				return nil, err
			}
			// Add user to the cache
			for _, u := range googleUsers {
				if _, found := userCache[u.PrimaryEmail]; !found {
					log.WithFields(log.Fields{
						"func":    funcName,
						"GroupId": group.Id,
						"Member#": memberIndex,
					}).Debug("Cache user")
					userCache[u.PrimaryEmail] = u
					s.recordCacheOp(u.OrgUnitPath, CacheMiss)
				}
				memberEmail = u.PrimaryEmail
			}
			// Check whether the member was based on an alias
			if memberEmail != m.Email {
				log.WithFields(log.Fields{
					"func":    funcName,
					"GroupId": group.Id,
					"Member#": memberIndex,
				}).Debug("Member listed using an alias")
			}
		} else {
			// member (email address) found in user cache
			memberEmail = m.Email
		}

		// Add user based on user cache
		if userCache[memberEmail] == nil {
			log.WithFields(log.Fields{
				"func":        funcName,
				"memberEmail": memberEmail,
			}).Error("Can't retrieve user")
			continue
		} else {
			log.WithFields(log.Fields{
				"func":    funcName,
				"GroupId": group.Id,
				"Member#": memberIndex,
			}).Debug("Add member")
		}
		s.recordCacheOp(userCache[memberEmail].OrgUnitPath, CacheHit)
		membersUsers = append(membersUsers, userCache[memberEmail])

	}
	log.WithFields(log.Fields{
		"func":         funcName,
		"GroupId":      group.Id,
		"# Members":    len(membersUsers),
		"membersUsers": membersUsers,
	}).Debug("Return")
	return membersUsers, nil
}
