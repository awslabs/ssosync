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
	"io/ioutil"

	"github.com/awslabs/ssosync/internal/aws"
	"github.com/awslabs/ssosync/internal/config"
	"github.com/awslabs/ssosync/internal/google"
	"github.com/hashicorp/go-retryablehttp"

	aws_sdk "github.com/aws/aws-sdk-go/aws"
	aws_sdk_sess "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/identitystore"
	"github.com/aws/aws-sdk-go/service/identitystore/identitystoreiface"
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
	aws                 aws.Client
	google              google.Client
	cfg                 *config.Config
	identityStoreClient identitystoreiface.IdentityStoreAPI

	users map[string]*aws.User
}

// New will create a new SyncGSuite object
func New(cfg *config.Config, a aws.Client, g google.Client, ids identitystoreiface.IdentityStoreAPI) SyncGSuite {
	return &syncGSuite{
		aws:                 a,
		google:              g,
		cfg:                 cfg,
		identityStoreClient: ids,
		users:               make(map[string]*aws.User),
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
		_, err = s.identityStoreClient.DeleteUser(&identitystore.DeleteUserInput{IdentityStoreId: &s.cfg.IdentityStoreID, UserId: &uu.ID})
		if err != nil {
			log.WithFields(log.Fields{
				"email": u.PrimaryEmail,
			}).Warn("Error deleting user")
			return err
		}
	}

	log.Debug("get active google users")
	googleUsers, err := s.google.GetUsers(query)
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
func (s *syncGSuite) SyncGroups(query string) error {

	log.WithField("query", query).Debug("get google groups")
	googleGroups, err := s.google.GetGroups(query)
	if err != nil {
		return err
	}

	correlatedGroups := make(map[string]*aws.Group)

	for _, g := range googleGroups {
		if s.ignoreGroup(g.Email) || !s.includeGroup(g.Email) {
			continue
		}

		log := log.WithFields(log.Fields{
			"group": g.Email,
		})

		log.Debug("Check group")
		var group *aws.Group

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
			newGroup := aws.NewGroup(g.Email)
			createGroupOutput, err := s.identityStoreClient.CreateGroup(&identitystore.CreateGroupInput{IdentityStoreId: &s.cfg.IdentityStoreID, DisplayName: &g.Email})
			if err != nil {
				return err
			}
			newGroup.ID = *createGroupOutput.GroupId
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
			b, err := s.IsUserInGroup(u, group)
			if err != nil {
				return err
			}

			if _, ok := memberList[u.Username]; ok {
				if !*b {
					log.WithField("user", u.Username).Info("Adding user to group")
					_, err = s.identityStoreClient.CreateGroupMembership(
						&identitystore.CreateGroupMembershipInput{
							IdentityStoreId: &s.cfg.IdentityStoreID,
							GroupId:         &group.ID,
							MemberId:        &identitystore.MemberId{UserId: &u.ID},
						},
					)
					if err != nil {
						return err
					}
				}
			} else {
				if *b {
					log.WithField("user", u.Username).Warn("Removing user from group")
					err := s.RemoveUserFromGroup(&u.ID, &group.ID)
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
func (s *syncGSuite) SyncGroupsUsers(queryGroups string, queryUsers string) error {

	log.WithField("queryGroup", queryGroups).Info("get google groups")
	log.WithField("queryUsers", queryUsers).Info("get google users")

	log.Debug("preparing list of google users, groups and their members")
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
		_, err = s.identityStoreClient.DeleteUser(
			&identitystore.DeleteUserInput{IdentityStoreId: &s.cfg.IdentityStoreID, UserId: &awsUserFull.ID},
		)
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
			awsUser.Active))
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
			errHttp := new(aws.ErrHttpNotOK)
			if errors.As(err, &errHttp) && errHttp.StatusCode == 409 {
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

		log := log.WithFields(log.Fields{"group": awsGroup.DisplayName})

		log.Info("creating group")
		newAwsGroup, err := s.identityStoreClient.CreateGroup(
			&identitystore.CreateGroupInput{IdentityStoreId: &s.cfg.IdentityStoreID, DisplayName: &awsGroup.DisplayName},
		)
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
			_, err = s.identityStoreClient.CreateGroupMembership(
				&identitystore.CreateGroupMembershipInput{
					IdentityStoreId: &s.cfg.IdentityStoreID,
					GroupId:         newAwsGroup.GroupId,
					MemberId:        &identitystore.MemberId{UserId: &awsUserFull.ID},
				},
			)
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
			b, err := s.IsUserInGroup(awsUserFull, awsGroup)
			if err != nil {
				return err
			}

			if !*b {
				log.WithField("user", awsUserFull.Username).Info("adding user to group")
				_, err = s.identityStoreClient.CreateGroupMembership(
					&identitystore.CreateGroupMembershipInput{
						IdentityStoreId: &s.cfg.IdentityStoreID,
						GroupId:         &awsGroup.ID,
						MemberId:        &identitystore.MemberId{UserId: &awsUserFull.ID},
					},
				)
				if err != nil {
					return err
				}
			}
		}

		for _, awsUser := range deleteUsersFromGroup[awsGroup.DisplayName] {
			log.WithField("user", awsUser.Username).Warn("removing user from group")
			err := s.RemoveUserFromGroup(&awsUser.ID, &awsGroup.ID)
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
		_, err = s.identityStoreClient.DeleteGroup(
			&identitystore.DeleteGroupInput{IdentityStoreId: &s.cfg.IdentityStoreID, GroupId: &awsGroupFull.ID},
		)
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

        // For large directories this will reduce execution time and avoid throttling limits
        log.Debug("Fetching ALL users from google, to use as cache")
	googleUsers, err := s.google.GetUsers("*")
        if err != nil {
                return nil, nil, nil, err
        }
        for _, u := range googleUsers {
		log.WithField("email", u).Debug("processing member of gUserDetailCache")
                gUserDetailCache[u.PrimaryEmail] = u
        }

        log.Debug("Fetching ALL groups from google, to use as cache")
	googleGroups, err := s.google.GetGroups("*")
        if err != nil {
                return nil, nil, nil, err
        }
        for _, g := range googleGroups {
                gGroupDetailCache[g.Email] = g
        }

	// Fetch Users
        log.Debug("get users from google, based on UserMatch,  regardless of group membership")
        googleUsers, err = s.google.GetUsers(queryUsers)
        if err != nil {
                return nil, nil, nil, err
        }

        log.Debug("process users from google, filtering as required")
	for _, u := range googleUsers {
		log.WithField("email", u).Debug("processing userMatch")

                // Remove any users that should be ignored
		if s.ignoreUser(u.PrimaryEmail) {
                	log.WithField("id", u.PrimaryEmail).Debug("ignoring user")
			continue
		}
                _, ok := gUniqUsers[u.PrimaryEmail]
                if !ok {
                	log.WithField("id", u.PrimaryEmail).Debug("adding user")
                	gUniqUsers[u.PrimaryEmail] = u
                }

        }

	log.Debug("get groups from google")
        gGroups, err := s.google.GetGroups(queryGroups)
        if err != nil {
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

		log := log.WithFields(log.Fields{"group": g.Name})

		if s.ignoreGroup(g.Email) {
			log.Debug("ignoring group")
			continue
		}

		log.Debug("get group members from google")
		membersUsers := s.getGoogleUsersInGroup(g, gUserDetailCache, gGroupDetailCache)

		// If we've not seen the user email address before add it to the list of unique users
		// also, we need to deduplicate the list of members.
		gUniqMembers := make(map[string]*admin.User)
                for _, m := range membersUsers {
			_, ok := gUniqUsers[m.PrimaryEmail]
			if !ok {
				gUniqUsers[m.PrimaryEmail] = gUserDetailCache[m.PrimaryEmail]
			}

			_, ok = gUniqMembers[m.PrimaryEmail]
                        if !ok {
                                gUniqMembers[m.PrimaryEmail] = gUserDetailCache[m.PrimaryEmail]
                        }
		}

	        gMembers := make([]*admin.User, 0)
	        for _, member := range gUniqMembers {
                        gMembers = append(gMembers, member)
                }
		gGroupsUsers[g.Name] = gMembers
	}

	for _, user := range gUniqUsers {
		gUsers = append(gUsers, user)
	}

	return gGroups, gUsers, gGroupsUsers, nil
}

// getGroupOperations returns the groups of AWS that must be added, deleted and are equals
func getGroupOperations(awsGroups []*aws.Group, googleGroups []*admin.Group) (add []*aws.Group, delete []*aws.Group, equals []*aws.Group) {

 	log.Debug("getGroupOperations()")
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
		 	log.WithField("gGroup", gGroup).Debug("equals")
			equals = append(equals, awsMap[gGroup.Name])
		} else {
		 	log.WithField("gGroup", gGroup).Debug("add")
			add = append(add, aws.NewGroup(gGroup.Name))
		}
	}

	// Google Groups founds and not in aws
	for _, awsGroup := range awsGroups {
		if _, found := googleMap[awsGroup.DisplayName]; !found {
		 	log.WithField("awsGroup", awsGroup).Debug("delete")
			delete = append(delete, aws.NewGroup(awsGroup.DisplayName))
		}
	}

	return add, delete, equals
}

// getUserOperations returns the users of AWS that must be added, deleted, updated and are equals
func getUserOperations(awsUsers []*aws.User, googleUsers []*admin.User) (add []*aws.User, delete []*aws.User, update []*aws.User, equals []*aws.User) {

	log.Debug("getUserOperations()")
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
				log.WithField("gUser", gUser).Debug("update")
				log.WithField("awsUser", awsUser).Debug("update")
				update = append(update, aws.NewUser(gUser.Name.GivenName, gUser.Name.FamilyName, gUser.PrimaryEmail, !gUser.Suspended))

			} else {
			        log.WithField("awsUser", awsUser).Debug("equals")
				equals = append(equals, awsUser)
			}
		} else {
		        log.WithField("gUser", gUser).Debug("add")
			add = append(add, aws.NewUser(gUser.Name.GivenName, gUser.Name.FamilyName, gUser.PrimaryEmail, !gUser.Suspended))
		}
	}

	// Google Users founds and not in aws
	for _, awsUser := range awsUsers {
		if _, found := googleMap[awsUser.Username]; !found {
			log.WithField("awsUser", awsUser).Debug("delete")
			delete = append(delete, aws.NewUser(awsUser.Name.GivenName, awsUser.Name.FamilyName, awsUser.Username, awsUser.Active))
		}
	}

	return add, delete, update, equals
}

// groupUsersOperations returns the groups and its users of AWS that must be delete from these groups and what are equals
func getGroupUsersOperations(gGroupsUsers map[string][]*admin.User, awsGroupsUsers map[string][]*aws.User) (delete map[string][]*aws.User, equals map[string][]*aws.User) {

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

	googleClient, err := google.NewClient(ctx, cfg.GoogleAdmin, creds)
	if err != nil {
	        log.WithField("error", err).Warn("Problem establising a connection to Google directory")
		return err
	}

	awsScimClient, err := aws.NewClient(
		httpClient,
		&aws.Config{
			Endpoint: cfg.SCIMEndpoint,
			Token:    cfg.SCIMAccessToken,
		})
	if err != nil {
	        log.WithField("error", err).Warn("Problem establising a SCIM connection to AWS IAM Identity Center")
		return err
	}

	// Initialize AWS session
	sess, err := aws_sdk_sess.NewSession(&aws_sdk.Config{
		// AWS Region to send requests to, provided by config
		Region: &cfg.Region,
	})

	if err != nil {
	        log.WithField("error", err).Warn("Problem establising a session for Identity Store")
		return err
	}

	// Initialize AWS Identity Store Public API Client with session
	identityStoreClient := identitystore.New(sess)

	response, err := identityStoreClient.ListGroups(
                &identitystore.ListGroupsInput{IdentityStoreId: &cfg.IdentityStoreID})

	if err != nil {
	        log.WithField("error", err).Warn("Problem performing test query against Identity Store")
		return err
	} else {
	        log.WithField("Groups", response).Info("Test call for groups successful")
                
        }

	// Initialize sync client with
	// 1. SCIM API client
	// 2. Google Directory API client
	// 3. Identity Store Public API client
	c := New(cfg, awsScimClient, googleClient, identityStoreClient)

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
	for _, g := range s.cfg.IncludeGroups {
		if g == name {
			return true
		}
	}

	return false
}

var awsGroups []*aws.Group

func (s *syncGSuite) GetGroups() ([]*aws.Group, error) {
	awsGroups = make([]*aws.Group, 0)

	err := s.identityStoreClient.ListGroupsPages(
		&identitystore.ListGroupsInput{IdentityStoreId: &s.cfg.IdentityStoreID},
		ListGroupsPagesCallbackFn,
	)

	if err != nil {
		return nil, err
	}

	return awsGroups, nil
}

func ListGroupsPagesCallbackFn(page *identitystore.ListGroupsOutput, lastPage bool) bool {
	// Loop through each Group returned
	for _, group := range page.Groups {
		// Convert to native Group object
		awsGroups = append(awsGroups, &aws.Group{
			ID:          *group.GroupId,
			Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
			DisplayName: *group.DisplayName,
			Members:     []string{},
		})
	}

	return !lastPage
}

var awsUsers []*aws.User

func (s *syncGSuite) GetUsers() ([]*aws.User, error) {
	awsUsers = make([]*aws.User, 0)

	err := s.identityStoreClient.ListUsersPages(
		&identitystore.ListUsersInput{IdentityStoreId: &s.cfg.IdentityStoreID},
		ListUsersPagesCallbackFn,
	)

	if err != nil {
		return nil, err
	}

	return awsUsers, nil
}

func ListUsersPagesCallbackFn(page *identitystore.ListUsersOutput, lastPage bool) bool {
	// Loop through each User in ListUsersOutput and convert to native User object
	for _, user := range page.Users {
		awsUsers = append(awsUsers, ConvertSdkUserObjToNative(user))
	}
	return !lastPage
}

func ConvertSdkUserObjToNative(user *identitystore.User) *aws.User {
	// Convert emails into native Email object
	userEmails := make([]aws.UserEmail, 0)

	for _, email := range user.Emails {
		if email.Value == nil || email.Type == nil || email.Primary == nil {
              		// This must be a user created by AWS Control Tower
                        // Need feature development to make how these users are treated
			// configurable.
			continue
		}
		userEmails = append(userEmails, aws.UserEmail{
			Value:   *email.Value,
			Type:    *email.Type,
			Primary: *email.Primary,
		})
	}

	// Convert addresses into native Address object
	userAddresses := make([]aws.UserAddress, 0)

	for _, address := range user.Addresses {
		userAddresses = append(userAddresses, aws.UserAddress{
			Type: *address.Type,
		})
	}

	return &aws.User{
		ID:       *user.UserId,
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
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

func CreateUserIDtoUserObjMap(awsUsers []*aws.User) map[string]*aws.User {
	awsUsersMap := make(map[string]*aws.User)

	for _, awsUser := range awsUsers {
		awsUsersMap[awsUser.ID] = awsUser
	}

	return awsUsersMap
}

var ListGroupMembershipPagesCallbackFn func(page *identitystore.ListGroupMembershipsOutput, lastPage bool) bool

func (s *syncGSuite) GetGroupMembershipsLists(awsGroups []*aws.Group, awsUsersMap map[string]*aws.User) (map[string][]*aws.User, error) {
	awsGroupsUsers := make(map[string][]*aws.User)
	curGroup := &aws.Group{}

	ListGroupMembershipPagesCallbackFn = func(page *identitystore.ListGroupMembershipsOutput, lastPage bool) bool {
		for _, member := range page.GroupMemberships { // For every member in the group
			userId := member.MemberId.UserId
			user := awsUsersMap[*userId]

			// Append new user onto existing list of users
			awsGroupsUsers[curGroup.DisplayName] = append(awsGroupsUsers[curGroup.DisplayName], user)
		}

		return !lastPage
	}

	// For every group, get the members and assign in awsGroupsUsers map
	for _, group := range awsGroups {
		curGroup = group
		awsGroupsUsers[curGroup.DisplayName] = make([]*aws.User, 0)

		// Get User ID of every member in group
		err := s.identityStoreClient.ListGroupMembershipsPages(
			&identitystore.ListGroupMembershipsInput{
				IdentityStoreId: &s.cfg.IdentityStoreID,
				GroupId:         &group.ID,
			}, ListGroupMembershipPagesCallbackFn)

		if err != nil {
			return nil, err
		}
	}

	return awsGroupsUsers, nil
}

func (s *syncGSuite) IsUserInGroup(user *aws.User, group *aws.Group) (*bool, error) {
	isUserInGroupOutput, err := s.identityStoreClient.IsMemberInGroups(
		&identitystore.IsMemberInGroupsInput{
			IdentityStoreId: &s.cfg.IdentityStoreID,
			GroupIds:        []*string{&group.ID},
			MemberId:        &identitystore.MemberId{UserId: &user.ID},
		},
	)

	if err != nil {
		return nil, err
	}

	isUserInGroup := isUserInGroupOutput.Results[0].MembershipExists

	return isUserInGroup, nil
}

func (s *syncGSuite) RemoveUserFromGroup(userId *string, groupId *string) error {
	memberIdOutput, err := s.identityStoreClient.GetGroupMembershipId(
		&identitystore.GetGroupMembershipIdInput{
			IdentityStoreId: &s.cfg.IdentityStoreID,
			GroupId:         groupId,
			MemberId:        &identitystore.MemberId{UserId: userId},
		},
	)

	if err != nil {
		return err
	}

	memberId := memberIdOutput.MembershipId

	_, err = s.identityStoreClient.DeleteGroupMembership(
		&identitystore.DeleteGroupMembershipInput{
			IdentityStoreId: &s.cfg.IdentityStoreID,
			MembershipId:    memberId,
		},
	)

	if err != nil {
		return err
	}

	return nil
}

func (s *syncGSuite) getGoogleUsersInGroup(group *admin.Group, userCache map[string]*admin.User, groupCache map[string]*admin.Group) []*admin.User {
	log.WithField("Email:", group.Email).Debug("getGoogleGroupMembers()")

	 // retrieve the members of the group
	groupMembers, err := s.google.GetGroupMembers(group)
	if err != nil {
		return nil
	}
        membersUsers := make([]*admin.User, 0)

	// process the members of the group
        for _, m := range groupMembers {
        	log.WithField("email", m.Email).Debug("processing member")
                // Ignore Owners aren't relevant in Identity Store
		// so are treated as group members.
                if m.Role == "OWNER" {
                	log.WithField("id", m.Email).Debug("owner role")
                }

                // Ignore any external members, since they don't have users
                // that can be synced
                if m.Type == "USER" && m.Status != "ACTIVE" {
                        log.WithField("id", m.Email).Warn("ignoring external user")
                        continue
                }

                // handle nested groups, by adding their membership to the end
                // of googleMembers
                if m.Type == "GROUP" {
		    	log.WithField("Email:", m.Email).Debug("calling getGoogleGroupMembers() for nested group")
			_, found := groupCache[m.Email]
			if found {
                        	membersUsers = append (membersUsers, s.getGoogleUsersInGroup(groupCache[m.Email], userCache, groupCache)...)
			} else {
                        	log.WithField("id", m.Email).Warn("missing nested group")
			}
                        continue
                }
                // Remove any users that should be ignored
                if s.ignoreUser(m.Email) {
                        log.WithField("id", m.Email).Debug("ignoring user")
                        continue
                }

                // Find the group member in the cache of UserDetails
                _, found := userCache[m.Email]
                if found {
                        membersUsers = append(membersUsers, userCache[m.Email])
                } else {
                        log.WithField("id", m.Email).Warn("missing user")
                        continue
                }
        }

        return membersUsers
}
