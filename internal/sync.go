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
	SyncGroupsUsers() error
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

	log.Debug("SyncUsers()")
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

        log.Debug("SyncGroups()")

	log.WithField("query", query).Debug("get google groups")
	googleGroups, err := s.google.GetGroups(query)
	if err != nil {
		return err
	}

	correlatedGroups := make(map[string]*aws.Group)

	for _, g := range googleGroups {
		if s.ignoreGroup(g.Email) {
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
func (s *syncGSuite) SyncGroupsUsers() error {

        log.Debug("SyncGroupsUsers()")

	log.Info("Retrieve Google Directory")
	googleGroups, googleUsers, googleGroupMembership := s.getGoogleDirectory()

	log.Info("Retrieve content of AWS Identity Store")
	awsGroups, awsUsers, awsGroupMembership := s.getIdentityStore()


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
		_, err = s.aws.UpdateUser(aws.UpdateUser(
			awsUserFull.ID,
			awsUser.Name.GivenName,
			awsUser.Name.FamilyName,
			awsUser.Username,
			awsUser.Active))
		if err != nil {
			log.Error("error updating user")
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
			log.Error("error creating user")
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
		for _, googleUser := range googleGroupMembership[awsGroup.DisplayName] {

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
	deleteUsersFromGroup, _ := getGroupUsersOperations(googleGroupMembership, awsGroupMembership)

	// validate groups members are equal in aws and google
	log.Debug("validating groups members, equals in aws and google")
	for _, awsGroup := range equalAWSGroups {

		// add members of the new group
		log := log.WithFields(log.Fields{"group": awsGroup.DisplayName})

		for _, googleUser := range googleGroupMembership[awsGroup.DisplayName] {

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

// getGroups return a list of google groups
// turns IncludeGroups into addition all queries appended to 
// GroupMatch, unless GroupMatch is "*", which matches all groups
// The results are then filtered using IgnoreGroups if they 
// have been specified.
func (s *syncGSuite) getGoogleGroups( ) []*admin.Group {
        log.Debug("getGoogleGroups()")
	if len(s.cfg.GroupMatch) > 0 {
		log.WithField("groupMatch", s.cfg.GroupMatch).Info("get google groups (by query)")
	}
        
        groupQueries := s.cfg.GroupMatch

       	// if GroupMatch is wildcard or no includeGroups have been provided we can skip this
	if len(s.cfg.IncludeGroups) > 0 {
                log.WithField("includeGroups", s.cfg.IncludeGroups).Info("individual google groups (by email address)")
        	if groupQueries != "*" {
                	// Add a query string
                	for _, emailAddress := range s.cfg.IncludeGroups {
		        	if len(groupQueries) > 0 {
					groupQueries += ", email=" + emailAddress
				} else {
					groupQueries += "email=" + emailAddress
				}
			}
                }
        }
        log.WithField("groupQueries", groupQueries).Debug("Group Queries")

	// Retreive the groups based on the above set of queries
        googleGroups, err := s.google.GetGroups(groupQueries)
	if err != nil {
		log.WithField("groupQueries", groupQueries).Error("Failed query")
                return nil
        }

	// Remove any groups that should be ignored
	if len(s.cfg.IgnoreGroups) > 0 {
                log.WithField("ignoreGroups", s.cfg.IgnoreGroups).Info("google groups to ignore")
        }
	filteredGroups := make([]*admin.Group, 0)
	for _, g := range googleGroups {
	        if s.ignoreGroup(g.Email) {
			log.WithField("email", g.Email).Debug("ignoring group")
                        continue
                }
		log.WithField("email", g.Email).Debug("appending group")
		filteredGroups = append(filteredGroups, g)
	}

	return filteredGroups
}


// UserMatch, unless UserMatch is "*", which matches all users
// The results are then filtered using IgnoreUsers if they 
// have been specified.
func (s *syncGSuite) getGoogleUsers( ) []*admin.User {
        log.Debug("getGoogleUsers()")
        log.WithField("userMatch", s.cfg.UserMatch).Info("get google users (by query)")
        log.WithField("includeUsers", s.cfg.IncludeUsers).Info("individual google users (by email address)")
        log.WithField("ignoreUsers", s.cfg.IgnoreUsers).Info("google users to ignore")

        userQueries := s.cfg.UserMatch

        // if UserMatch is wildcard or no includeUsers have been provided we can skip this
        if userQueries != "*" {
                // Add a query string
                for _, emailAddress := range s.cfg.IncludeUsers {
                        if len(userQueries) > 0 {
                                userQueries += ", email=" + emailAddress
                        } else {
                                userQueries += "email=" + emailAddress
                        }
                }
        }

        // Retreive the users based on the above set of queries
        googleUsers, err := s.google.GetUsers(userQueries)
        if err != nil {
	 	log.WithField("userQueries", userQueries).Error("Failed")
	 	log.WithField("error", err).Error("Failed")
                return nil
        }

        // Remove any users that should be ignored
        filteredUsers := make([]*admin.User, 0)
        for _, u := range googleUsers {
	        if s.ignoreUser(u.PrimaryEmail) {
			log.WithField("id", u.PrimaryEmail).Debug("ignoring user")
                	continue
                }
                filteredUsers = append(filteredUsers, u)
        }

        return filteredUsers
}

// getGoogleDirectory()
// Retrieves all the groups, users and group memberships from the Google GSuite directory
// depuplicates each type
// applies matchs, includes and ignores 
// and flattens any nested groups into the parent group
// returning these as maps, keyed on the email address.
func (s *syncGSuite) getGoogleDirectory() ([]*admin.Group, []*admin.User, map[string][]*admin.User) {
        log.Debug("getGoogleDirectory()")

        log.Debug("Retrieve content of Google Directory")
        googleGroups := s.getGoogleGroups()

        log.Debug("Retrieve list of google users")
        googleUsers := s.getGoogleUsers()

	uniqueGroups := make(map[string]*admin.Group)
	uniqueUsers := make(map[string]*admin.User)
	uniqueGroupMembership := make(map[string][]*admin.User)

	// Populate uniqueUsers from googleUsers
	for _, u := range googleUsers {
		uniqueUsers[u.PrimaryEmail] = u
	}

	log.Debug("Retrieve list of google users, as members of groups")
	// Work through the groups supplied to determine their memberships
	for _, g := range googleGroups {

		groupMembers := s.getGoogleGroupMembers(g)

		// Create object to store the groups membership
		// index based on email address we don't end up with duplicate membership
		// entries due to nested groups
		uniqueMembers := make(map[string]*admin.User)

        	for _, m:= range groupMembers {
		        log.WithField("email", m.Email).Debug("processing member")
			// Ignore Owners they aren't relevant in Identity Store
			if m.Role == "OWNER" {
			        log.WithField("id", m.Email).Debug("ignoring owner roles")
				continue
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
				groupMembers = append (groupMembers, s.getGoogleSubGroupMembers(m)...)
                                continue
                        }
        		// Remove any users that should be ignored
                	if s.ignoreUser(m.Email) {
				log.WithField("id", m.Email).Debug("ignoring user")
                        	continue
                	}

			// Retrieve the user and add to list of users to sync
                	uniqueUsers[m.Email] = s.getGoogleUser(m.Email)
			uniqueMembers[m.Email] = uniqueUsers[m.Email]

        	}
		// add the membership of the group
		membership := make([]*admin.User, 0)
		for _, member := range uniqueMembers {
			membership = append(membership, member)
		}
		uniqueGroups[g.Email] = g
		uniqueGroupMembership[g.Name] = membership
	}

        filteredUsers := make([]*admin.User, 0)
	for _, u := range uniqueUsers {
                filteredUsers = append(filteredUsers, u)
        }
	filteredGroups := make([]*admin.Group, 0)
	for _, g := range uniqueGroups {
                filteredGroups = append(filteredGroups, g)
        }
	return filteredGroups, filteredUsers, uniqueGroupMembership
}


// get the users, groups, and group memberships from the AWS IAM IdentityStore
func (s *syncGSuite) getIdentityStore() ([]*aws.Group, []*aws.User, map[string][]*aws.User) {
	log.Debug("getIdenityStore()")

        log.Debug("Retrieve content of AWS IAM Identity Store")
        awsGroups := s.getAwsGroups()

        log.Debug("Retrieve list of AWS users")
        awsUsers := s.getAwsUsers()


	log.Info("get active status for aws users")
        for _, awsUser := range awsUsers {
                scimUser, err := s.aws.FindUserByEmail(awsUser.Username)

                if err != nil {
                        log.Error("error getting active status for user " + awsUser.ID)
                        continue
                }

                awsUser.Active = scimUser.Active
        }

        log.Info("preparing map of user id's to user")
        awsUsersMap := CreateUserIDtoUserObjMap(awsUsers)

        log.Debug("preparing list of aws groups and their members")
        awsGroupMemberships := make(map[string][]*aws.User)
        curGroup := &aws.Group{}
                
        ListGroupMembershipPagesCallbackFn = func(page *identitystore.ListGroupMembershipsOutput, lastPage bool) bool {
                for _, member := range page.GroupMemberships { // For every member in the group
                        userId := member.MemberId.UserId
                        user := awsUsersMap[*userId]

                        // Append new user onto existing list of users
                        awsGroupMemberships[curGroup.DisplayName] = append(awsGroupMemberships[curGroup.DisplayName], user)
                } 
                
                return !lastPage
        }
        
        // For every group, get the members and assign in awsGroupsUsers map
        for _, group := range awsGroups {
                curGroup = group
                awsGroupMemberships[curGroup.DisplayName] = make([]*aws.User, 0)

                // Get User ID of every member in group
                err := s.identityStoreClient.ListGroupMembershipsPages(
                        &identitystore.ListGroupMembershipsInput{
                                IdentityStoreId: &s.cfg.IdentityStoreID,
                                GroupId:         &group.ID,
                        }, ListGroupMembershipPagesCallbackFn)
                
                if err != nil {
		 	log.Error("error getting group memberships for " + curGroup.DisplayName)
			continue
                }
        }

	return awsGroups, awsUsers, awsGroupMemberships
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

        log.Debug("DoSync()")

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

	awsScimClient, err := aws.NewClient(
		httpClient,
		&aws.Config{
			Endpoint: cfg.SCIMEndpoint,
			Token:    cfg.SCIMAccessToken,
		})
	if err != nil {
		return err
	}

	// Initialize AWS session
	sess, err := aws_sdk_sess.NewSession(&aws_sdk.Config{
		// AWS Region to send requests to, provided by config
		Region: &cfg.Region,
	})

	if err != nil {
		return err
	}

	// Initialize AWS Identity Store Public API Client with session
	identityStoreClient := identitystore.New(sess)

	// Initialize sync client with
	// 1. SCIM API client
	// 2. Google Directory API client
	// 3. Identity Store Public API client
	c := New(cfg, awsScimClient, googleClient, identityStoreClient)

	log.WithField("sync_method", cfg.SyncMethod).Info("syncing")
	if cfg.SyncMethod == config.DefaultSyncMethod {
		err = c.SyncGroupsUsers()
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


// if the list of groups to ignore is not empty
// check whether the supplied email address is
// listed
func (s *syncGSuite) ignoreUser(email string) bool {
        if len(s.cfg.IgnoreUsers) > 0 {
                // iterate through the list 
                for _, u := range s.cfg.IgnoreUsers {
                        if u == email {
                                return true
                        }
                }
        }
        return false
}

// if the list of groups to ignore is not empty
// check whether the supplied email address is
// listed
func (s *syncGSuite) ignoreGroup(email string) bool {
        if len(s.cfg.IgnoreGroups) > 0 {
		// iterate through the list 
		for _, g := range s.cfg.IgnoreGroups {
			if g == email {
				return true
			}
		}
	}
	return false
}

func (s *syncGSuite) getGoogleUser(email string) *admin.User {
	// retrieve a single user
        log.WithField("email", email).Debug("getGoogleUser()")
        u, err := s.google.GetUsers("email=" + email)

        if err != nil {
		log.WithField("error:", err).Error("get user failed")
        	return nil
        }
	if len(u) == 1 {
		return u[0]
	} else {
		log.Error("No User found")
	}
	return nil
}

func (s *syncGSuite) getGoogleGroupMembers(g *admin.Group) []*admin.Member {
	// retrieve the members of a group
	log.WithField("Email", g.Email).Debug("getGoogleGroupMembers()")
        groupMembers, err := s.google.GetGroupMembers(g)
        if err != nil {
		log.WithField("error:", err).Error("get group Members failed")
        	return nil
        }
	return groupMembers
}


func (s *syncGSuite) getGoogleSubGroupMembers(m *admin.Member) []*admin.Member {
	log.WithField("Email", m.Email).Debug("getGoogleSubGroupMembers()")
        // retrieve the members of a group
	g, err := s.google.GetGroups("email="+ m.Email)
        if err != nil {
                log.WithField("error:", err).Error("failed to retrieve group")
                return nil
        }

	if len(g) == 1 {
        	log.WithField("Id", g).Debug("fetch members")

        	groupMembers, err := s.google.GetGroupMembers(g[0])
        	if err != nil {
                	log.WithField("error:", err).Error("get group Members failed")
                	return nil
       		}
        	return groupMembers
	} else {
		log.Error("No group found")
	}
        return nil
}

var awsGroups []*aws.Group

func (s *syncGSuite) getAwsGroups() []*aws.Group {

        log.Debug("GetAwsGroups()")

	awsGroups = make([]*aws.Group, 0)

	err := s.identityStoreClient.ListGroupsPages(
		&identitystore.ListGroupsInput{IdentityStoreId: &s.cfg.IdentityStoreID},
		ListGroupsPagesCallbackFn,
	)

        if err != nil {
                log.Error("error getting aws groups")
                return nil
        }
	return awsGroups
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

func (s *syncGSuite) getAwsUsers() []*aws.User {

        log.Debug("GetAwsUsers()")

	awsUsers = make([]*aws.User, 0)

	err := s.identityStoreClient.ListUsersPages(
		&identitystore.ListUsersInput{IdentityStoreId: &s.cfg.IdentityStoreID},
		ListUsersPagesCallbackFn,
	)

	if err != nil {
		log.Error("error getting aws users")
		return nil
	}

	return awsUsers
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

