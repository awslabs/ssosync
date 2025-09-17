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
	"os"
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
	googleUsers, err := s.google.GetUsers()
	if err != nil {
		return err
	}

	for _, u := range googleUsers {
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
	googleGroups, err := s.google.GetGroups()
	if err != nil {
		return err
	}

	correlatedGroups := make(map[string]*interfaces.Group)

	for _, g := range googleGroups {
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
//  1. Groups:
//     a. Retrieve Groups from AWS & Google
//     b. Determine Changes (if any) required to be made in AWS
//     c. Execute the required group changes: add, update, remove
//  2. Users:
//     a. Retrieve Users from AWS & Google (explicit and implicit via group membership)
//     b. Determine Changes (if any) required to be made in AWS
//     c. Execute the required user changes: add, update, remove
//  3. Group Memberships:
//     a. Retrieve Members of Groups from AWS & Google
//     b. Determine Changes (if any) required to be made in AWS
//     c. Execute the required user changes: add, remove
func (s *syncGSuite) SyncGroupsUsers(queryGroups string, queryUsers string) error {
	log.WithField("queryGroups", queryGroups).Debug("SyncGroupsUsers()")
	log.WithField("queryUsers", queryUsers).Debug("SyncGroupsUsers()")

	// Process Groups
	log.Debug("Groups: Retrieve information.")
	googleGroups, err := s.google.GetGroups()
	if err != nil {
		return err
	}
	awsGroups, err := s.GetGroups()
	if err != nil {
		return err
	}

	log.Debug("Groups: Determine required operations.")
	addGroups, delGroups, updateGroups, equalGroups := getGroupOperations(awsGroups, googleGroups)

	log.Debug("Groups: Execute changes.")
	groups, err := s.aws.CreateGroups(equalGroups, addGroups)
	if err != nil {
		return err
	}
	groups, err = s.aws.UpdateGroups(groups, updateGroups)
	if err != nil {
		return err
	}
	err = s.aws.DeleteGroups(delGroups)
	if err != nil {
		return err
	}

	// Process Users
	log.Debug("Users: Retrieve information.")
	googleUsers, err := s.google.GetUsers()
	if err != nil {
		return err
	}
	awsUsers, err := s.GetUsers()
	if err != nil {
		return err
	}

	log.Debug("Users: Determine required operations.")
	addUsers, delUsers, updateUsers, equalUsers := getUserOperations(awsUsers, googleUsers)

	log.Debug("Users: Execute changes.")
	users, err := s.aws.CreateUsers(equalUsers, addUsers)
	if err != nil {
		return err
	}
	users, err = s.aws.UpdateUsers(users, updateUsers)
	if err != nil {
		return err
	}
	err = s.aws.DeleteUsers(delUsers)
	if err != nil {
		return err
	}

	// Process Group Members
	log.Debug("Group Members: Retrieve information.")
	googleMembers, err := s.google.GetMembers()
	if err != nil {
		return err
	}
	awsMembers, err := s.getGroupMembers(awsGroups)
	if err != nil {
		return err
	}

	log.Debug("Group Members: Determine required operations.")
	addMembers, delMembers, equalMembers := getMemberOperations(awsUsers, awsGroups, awsMembers, googleMembers)

	log.Debug("Group Members: Execute changes.")
	members, err := s.aws.AddMembers(equalMembers, addMembers)
	if err != nil {
		return err
	}
	err = s.aws.RemoveMembers(delMembers)
	if err != nil {
		return err
	}

	log.Info("sync completed")

	return nil
}

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

// groupMemberOperations returns the groups and its users of AWS that must be delete from these groups and what are equals
func getMemberOperations(awsUsers []*interfaces.User, awsGroups []*interfaces.Group, awsMembers map[string][]*interfaces.User, googleMembers map[string][]*admin.User) (add map[string][]string, delete map[string][]string, equals map[string][]string) {

	log.Debug("getMemberOperations()")

	lookupGoogleToAws = make(map[string]string)
	for _, user := range awsUsers {
		lookupGoogleToAws[user.ExternalId] = user.Id
	}

	add = make(map[string][]string)
	delete = make(map[string][]string)
	equals = make(map[string][]string)

	for _, aGroup := range awsGroups {
		// Tests whether this is an empty AWS Group and not listed in the awsMembers
		if _, found := awsMembers[aGroup.Id]; !found {
			for _, gMember := range googleGroups[aGroup.ExternalId] {
				add[aGroup.Id] = append(add[aGroup.Id], lookupGoogleToAws[gMember.Id])
			}
			continue
		}
		// Iterate through Google Members
		for _, gMember := range googleMembers[aGroup.ExternalId] {
			if _, found := awsMembers[lookupGoogleToAws(gMember.Id)]; !found {
				// Member missing from AWS Group
				add[aGroup.Id] = append(add[aGroup.Id], lookupGoogleToAws[gMember.Id])
			} else {
				// Member already in the AWS Group
				equals[aGroup.Id] = append(equals[aGroup.Id], aMember.Id)
			}
		}
		// Iterate through AWS Members
		for _, aMember := range awsMembers[aGroup.Id] {
			if _, found := googleGroups[aGroup.ExternalId]; !found {
				// Member exists in AWS but not in Google
				delete[aGroup.Id] = append(delete[aGroup.Id], aMember.Id)
			}
		}
	}

	return add, delete, equals
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

	if cfg.SyncMethod != config.DefaultSyncMethod && len(cfg.includeGroups) > 0 {
		for _, groupEmail := range cfg.includeGroups {
			if len(cfg.GroupMatch) == 0 {
				cfg.GroupMatch = cfg.GroupMatch + "email=" + groupEmail
			} else {
				cfg.GroupMatch = cfg.GroupMatch + ", email=" + groupEmail
			}
		}
	}

	googleClient, err := google.NewClient(ctx, cfg.GoogleAdmin, creds, "my_customer", cfg.UserMatch, cfg.GroupMatch, false, cfg.SyncSuspended, cfg.PrecacheOrgUnits, cfg.IgnoreUsers, cfg.IgnoreGroups)
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
