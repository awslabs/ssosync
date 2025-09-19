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

// Package google ...
package google

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

const (
	ModuleName = "google"
)

// Client is the Interface for the Client
type Client interface {
	GetUsers() ([]*admin.User, error)
	GetGroups() ([]*admin.Group, error)
	GetGroupMembers() ([]*admin.Member, error)
	GetGroupMembersById(*admin.Group) ([]*admin.Member, error)
}

type client struct {
	ctx              context.Context
	service          *admin.Service
	customerId       string
	userQueries      string
	groupQueries     string
	precacheOrgUnits []string
	ignoreUsers      map[string]string
	ignoreGroups     map[string]string
	includeSuspended bool
	includeArchived  bool
	userCacheId      map[string]*admin.User
	userCacheEmail   map[string]*admin.User
	users            []*admin.User
	usersId          map[string]*admin.User
	groups           []*admin.Group
	groupsId         map[string]*admin.Group
	members          map[string][]*admin.Member
	groupMembers     map[string][]string
	usersInGroup     map[string][]*admin.User
}

// NewClient creates a new client for Google's Admin API
func NewClient(ctx context.Context, adminEmail string, serviceAccountKey []byte, customerId string, queryUsers string, queryGroups string, includeArchived bool, includeSuspended bool, precacheOUs []string, ignoreUsers []string, ignoreGroups []string) (Client, error) {

	config, err := google.JWTConfigFromJSON(serviceAccountKey, admin.AdminDirectoryGroupReadonlyScope,
		admin.AdminDirectoryGroupMemberReadonlyScope,
		admin.AdminDirectoryUserReadonlyScope)

	if err != nil {
		return nil, err
	}

	config.Subject = adminEmail

	ts := config.TokenSource(ctx)

	srv, err := admin.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}

	ignoreUsersMap := make(map[string]string, 0)
	for _, emailAddress := range ignoreUsers {
		ignoreUsersMap[emailAddress] = ""
	}

	ignoreGroupsMap := make(map[string]string, 0)
	for _, emailAddress := range ignoreGroups {
		ignoreGroupsMap[emailAddress] = ""
	}

	return &client{
		ctx:              ctx,
		service:          srv,
		customerId:       "my_customer",
		userQueries:      queryUsers,
		groupQueries:     queryGroups,
		precacheOrgUnits: precacheOUs,
		ignoreUsers:      ignoreUsersMap,
		ignoreGroups:     ignoreGroupsMap,
		includeSuspended: includeSuspended,
		includeArchived:  includeArchived,
		userCacheId:      nil,
		userCacheEmail:   nil,
		users:            nil,
		usersId:          nil,
		groups:           nil,
		groupsId:         nil,
		members:          nil,
		groupMembers:     nil,
		usersInGroup:     nil,
	}, nil
}

//Parameter helper methods
//
//

func (c *client) getUserGlobalFilter() string {
	funcName := "getUserGlobalFilter"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	globalFilter := ""
	if c.includeArchived {
		globalFilter = globalFilter + " isArchived=false"
	}
	if c.includeSuspended {
		globalFilter = globalFilter + " isArchived=false"
	}
	return globalFilter
}

func (c *client) getPrecacheQueries() []string {
	funcName := "getPrecacheQueries"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	// No OrgUnits have been so nothing to return
	if len(c.precacheOrgUnits) == 0 {
		return nil
	}

	// If a wildcard has been passed then return just the
	// global filters
	if c.precacheOrgUnits[0] == "*" {
		return []string{c.getUserGlobalFilter()}
	}

	// Some specific OrgUnits must have been listed
	// so parse them into valid query strings
	precacheQueries := make([]string, 0)
	for _, orgUnitPath := range c.precacheOrgUnits {
		log.WithField("orgUnitPath", orgUnitPath).Debug("format into query string")
		orgUnitPath = strings.TrimSpace(orgUnitPath)
		orgUnitPath = strings.TrimSuffix(orgUnitPath, "/")
		if strings.ContainsRune(orgUnitPath, ' ') {
			precacheQueries = append(precacheQueries, "OrgUnitPath='"+orgUnitPath+"'"+c.getUserGlobalFilter())
		} else {
			precacheQueries = append(precacheQueries, "OrgUnitPath="+orgUnitPath+c.getUserGlobalFilter())
		}
	}
	return precacheQueries
}

func (c *client) getGroupQueries() []string {
	funcName := "getGroupQueries"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	// No group queries have been passed so nothing to return
	if len(c.groupQueries) == 0 {
		return nil
	}

	// If a wildcard has been passed then return an empty string
	if c.groupQueries == "*" {
		return []string{""}
	}

	groupQueries := make([]string, 0)
	for _, query := range strings.Split(c.groupQueries, ",") {
		groupQueries = append(groupQueries, query)
	}
	return groupQueries
}

func (c *client) getUserQueries() []string {
	funcName := "getUserQueries"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	// No group queries have been passed so nothing to return
	if len(c.userQueries) == 0 {
		return nil
	}

	// If a wildcard has been passed then return just the
	// global filters
	if c.userQueries == "*" {
		return []string{c.getUserGlobalFilter()}
	}

	userQueries := make([]string, 0)
	for _, query := range strings.Split(c.userQueries, ",") {
		userQueries = append(userQueries, query+c.getUserGlobalFilter())
	}
	return userQueries
}

// Google Admin API helper methods
//
//

func (c *client) fetchUsers(query string) ([]*admin.User, error) {
	funcName := "fetchUsers"
	log.WithField("module", ModuleName).WithField("func", funcName).WithField("query", query).Debug(funcName + "()")
	var err error

	u := []*admin.User{}
	if len(query) == 0 {
		// Fetching all Groups in the directory
		if err = c.service.Users.List().Customer(c.customerId).Pages(c.ctx, func(users *admin.Users) error {
			u = append(u, users.Users...)
			return nil
		}); err != nil {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("error", err).WithField("query", query).Error("failed")
			return nil, err
		}
	} else {
		if err = c.service.Users.List().Query(query).Customer(c.customerId).Pages(c.ctx, func(users *admin.Users) error {
			u = append(u, users.Users...)
			return nil
		}); err != nil {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("error", err).WithField("query", query).Error("failed")
			return nil, err
		}
	}
	for _, user := range u {
		// TODO : This will move to the attribue mapping module latter
		// some people prefer to go by a mononym
		// Google directory will accept a 'zero width space' for an empty name but will not accept a 'space'
		// but
		// Identity Store will accept and a 'space' for an empty name but not a 'zero width space'
		// So we need to replace any 'zero width space' strings with a single 'space' to allow comparison and sync
		user.Name.GivenName = strings.ReplaceAll(user.Name.GivenName, string('\u200B'), " ")
		user.Name.FamilyName = strings.ReplaceAll(user.Name.FamilyName, string('\u200B'), " ")
	}
	return u, nil
}

func (c *client) fetchUser(uniqueId string) (*admin.User, error) {
	funcName := "fetchUser"
	log.WithField("module", ModuleName).WithField("func", funcName).WithField("uniqueId", uniqueId).Debug(funcName + "()")

	user, err := c.service.Users.Get(uniqueId).Do()
	if err != nil {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("error", err).WithField("uniqueId", uniqueId).Error("failed")
		return nil, err
	}

	// TODO : This will move to the attribue mapping module latter
	// some people prefer to go by a mononym
	// Google directory will accept a 'zero width space' for an empty name but will not accept a 'space'
	// but
	// Identity Store will accept and a 'space' for an empty name but not a 'zero width space'
	// So we need to replace any 'zero width space' strings with a single 'space' to allow comparison and sync
	user.Name.GivenName = strings.ReplaceAll(user.Name.GivenName, string('\u200B'), " ")
	user.Name.FamilyName = strings.ReplaceAll(user.Name.FamilyName, string('\u200B'), " ")

	return user, nil
}

func (c *client) fetchGroups(query string) ([]*admin.Group, error) {
	funcName := "fetchGroupMembers"
	log.WithField("module", ModuleName).WithField("func", funcName).WithField("query", query).Debug(funcName + "()")

	var err error
	var g []*admin.Group

	if err = c.service.Groups.List().Customer(c.customerId).Query(query).Pages(context.TODO(), func(groups *admin.Groups) error {
		g = append(g, groups.Groups...)
		return nil
	}); err != nil {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("error", err).WithField("query", query).Error("failed")
		return nil, err
	}
	return g, nil
}

func (c *client) fetchMembers(groupId string) ([]*admin.Member, error) {
	funcName := "fetchGroupMembers"
	log.WithField("module", ModuleName).WithField("func", funcName).WithField("GroupId", groupId).Debug(funcName + "()")

	var err error
	var m []*admin.Member

	if err = c.service.Members.List(groupId).IncludeDerivedMembership(true).Pages(context.TODO(), func(members *admin.Members) error {
		m = append(m, members.Members...)
		return nil
	}); err != nil {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("error", err).WithField("GroupId", groupId).Error("failed")
		return nil, err
	}

	return m, nil
}

// Local directory cache management helpers
//
//

func (c *client) refreshUserCache() error {
	funcName := "refreshUserCache"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	// Check whether the users have already been populated.
	if c.userCacheId != nil || c.userCacheEmail != nil {
		return nil
	}

	// No precache queries are available initialize an empty cache
	if c.getPrecacheQueries() == nil {
		log.WithField("module", ModuleName).WithField("func", funcName).Info("Precaching disabled, initializing empty caches")
		c.userCacheId = make(map[string]*admin.User, 0)
		c.userCacheEmail = make(map[string]*admin.User, 0)
		return nil
	}

	for _, query := range c.getPrecacheQueries() {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("query", query).Debug("Precaching users")
		users, err := c.fetchUsers(query)
		if err != nil {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("query", query).Error("google.refreshUserCache() failed")
			continue
		}
		for _, user := range users {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("user", user).Debug("processing")
			if _, found := c.ignoreUsers[user.PrimaryEmail]; found {
				log.WithField("module", ModuleName).WithField("func", funcName).WithField("user.Id", user.Id).Info("Ignore user")
				continue
			}
			if err := c.cacheUser(user); err != nil {
				log.WithField("module", ModuleName).WithField("func", funcName).WithField("user.Id", user.Id).WithField("error", err).Error("failed to cache")
				continue
			}
		}
	}
	return nil
}

func (c *client) refreshUsers() error {
	funcName := "refreshUsers"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	// Check whether the users have already been populated.
	if c.users != nil {
		return nil
	}

	// Check dependancies
	if err := c.refreshUserCache(); err != nil {
		return err
	}

	for _, query := range c.getUserQueries() {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("query", query).Debug("fetchUsers")
		users, err := c.fetchUsers(query)
		if err != nil {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("query", query).WithField("error", err).Error("failed")
			return err
		}
		for _, user := range users {
			if err := c.addUser(user); err != nil {
				log.WithField("module", ModuleName).WithField("func", funcName).WithField("user", user).WithField("error", err).Error("failed")
				continue
			}
		}
	}
	return nil
}

func (c *client) refreshGroups() error {
	funcName := "refreshGroups"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	// Check whether the groups have already been populated.
	if c.groups != nil {
		return nil
	}

	// No precache queries are available initialize an empty cache
	if c.getGroupQueries() == nil {
		log.WithField("module", ModuleName).WithField("func", funcName).Debug("No group queries provided, initializing empty")
		c.groups = make([]*admin.Group, 0)
		c.groupsId = make(map[string]*admin.Group, 0)
		return nil
	}

	for _, query := range c.getGroupQueries() {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("query", query).Debug("fetchGroups")
		groups, err := c.fetchGroups(query)
		if err != nil {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("query", query).WithField("error", err).Error("failed")
			return err
		}
		for _, group := range groups {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("group", group).Debug("processing")
			if _, found := c.ignoreGroups[group.Email]; found {
				log.WithField("module", ModuleName).WithField("func", funcName).WithField("group", group).Info("Ignore group")
				continue
			}
			if err := c.addGroup(group); err != nil {
				log.WithField("module", ModuleName).WithField("func", funcName).WithField("groupi.Id", group.Id).WithField("error", err).Error("error adding group")
				continue
			}
		}
	}
	return nil
}

func (c *client) refreshMembers() error {
	funcName := "refreshMembers"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	// Check whether the groupMembers have already been populated.
	if c.groupMembers != nil && c.usersInGroup != nil && c.members != nil {
		return nil
	}

	// Check dependancies have been populated
	if err := c.refreshGroups(); err != nil {
		return err
	}
	if err := c.refreshUserCache(); err != nil {
		return err
	}

	for groupId, _ := range c.groupsId {
		members, err := c.fetchMembers(groupId)

		if err != nil {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("groupId", groupId).Error("google.refreshMembers() failed")
			return err
		}
		memberList := make([]*admin.Member, 0)
		groupMemberList := make([]string, 0)
		userList := make([]*admin.User, 0)

		for _, member := range members {
			user := c.processMember(member)
			if user == nil {
				continue
			}
			memberList = append(memberList, member)
			groupMemberList = append(groupMemberList, user.Id)
			userList = append(userList, user)
		}

		c.members[groupId] = memberList
		c.groupMembers[groupId] = groupMemberList
		c.usersInGroup[groupId] = userList
	}
	return nil
}

// internal helper methods
func (c *client) processMember(member *admin.Member) *admin.User {
	funcName := "processMember"
	log.WithField("module", ModuleName).WithField("func", funcName).WithField("member", member).Debug(funcName + "()")

	if member.Type != "USER" {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("member.Id", member.Id).Info("skipping member: not a USER")
		return nil
	}

	// Ignore any external members, since they don't have users
	// that can be synced
	if member.Status != "ACTIVE" && member.Status != "SUSPENDED" {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("member.Id", member.Id).Info("skipping member: external user")
		return nil
	}

	// Ignore any external members, since they don't have users
	// that can be synced
	if member.Status == "SUSPENDED" && !c.includeSuspended {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("member.Id", member.Id).Info("skipping member: suspended user")
		return nil
	}

	// Remove any users that should be ignored
	if _, found := c.ignoreUsers[member.Email]; found {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("member.Id", member.Id).Info("skipping member: ignore list")
		return nil
	}

	// Find the group member in the cache of UserDetails
	return c.getUser(member.Email)
}

func (c *client) getUser(uniqueId string) *admin.User {
	funcName := "getUser"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	if _, found := c.ignoreUsers[uniqueId]; found {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("email", uniqueId).Info("Ignore user")
		return nil
	}

	// Fetch user from the cache
	if strings.ContainsRune(uniqueId, '@') {
		if user, found := c.userCacheEmail[uniqueId]; found {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("user", user).Debug("from cache")
			return user
		}
	} else {
		if user, found := c.userCacheId[uniqueId]; found {
			log.WithField("module", ModuleName).WithField("func", funcName).WithField("user", user).Debug("from cache")
			return user
		}
	}

	// Fetch the user from the Google Directory
	log.WithField("module", ModuleName).WithField("func", funcName).WithField("user", uniqueId).Debug("not found in cache")
	user, err := c.fetchUser(uniqueId)
	if err != nil {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("error:", err).Error("Fetching user")
		return nil
	}

	c.addUser(user)
	return user
}

func (c *client) addGroup(group *admin.Group) error {
	funcName := "addGroup"
	log.WithField("module", ModuleName).WithField("func", funcName).WithField("group", group).Debug(funcName + "()")

	if err := c.refreshGroups(); err != nil {
		return err
	}

	if _, found := c.ignoreGroups[group.Email]; found {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("group", group).Info("Ignore group")
		return nil
	}

	if _, found := c.groupsId[group.Id]; !found {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("group.Id", group.Id).Debug("adding group")
		c.groupsId[group.Id] = group
		c.groups = append(c.groups, group)
	}
	return nil
}

func (c *client) addUser(user *admin.User) error {
	funcName := "addUser"
	log.WithField("module", ModuleName).WithField("func", funcName).WithField("user", user).Debug(funcName + "()")

	if user == nil {
		log.WithField("module", ModuleName).WithField("func", funcName).Debug("non user supplied")
		return nil
	}

	if err := c.refreshUsers(); err != nil {
		return err
	}

	if _, found := c.ignoreUsers[user.PrimaryEmail]; found {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("user.Id", user.Id).Info("Ignore user")
		return nil
	}

	c.cacheUser(user)

	c.usersId[user.Id] = user
	c.users = append(c.users, user)
	return nil
}

func (c *client) cacheUser(user *admin.User) error {
	funcName := "cacheUser"
	log.WithField("module", ModuleName).WithField("func", funcName).WithField("user", user).Debug(funcName + "()")

	if user == nil {
		log.WithField("module", ModuleName).WithField("func", funcName).Debug("non user supplied")
		return nil
	}

	if err := c.refreshUserCache(); err != nil {
		return err
	}

	if _, found := c.ignoreUsers[user.PrimaryEmail]; found {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("user.Id", user.Id).Info("Ignore user")
		return nil
	}

	if _, found := c.userCacheId[user.Id]; !found {
		log.WithField("module", ModuleName).WithField("func", funcName).WithField("user.Id", user.Id).Debug("caching user")
		c.userCacheId[user.Id] = user
		c.userCacheEmail[user.PrimaryEmail] = user
	}
	return nil
}

// Public methods
//
//

// Returns a map (by group id)
func (c *client) GetGroupsById() (map[string]*admin.Group, error) {
	funcName := "GetGroupsById"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	// Check dependancies have been populated
	if err := c.refreshGroups(); err != nil {
		return nil, err
	}

	return c.groupsId, nil
}

// Returns a map (by user id)
func (c *client) GetUsersById() (map[string]*admin.User, error) {
	funcName := "GetUsersById"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	if err := c.refreshUsers(); err != nil {
		return nil, err
	}

	return c.usersId, nil
}

// Returns a map (by group id)
func (c *client) GetUserById(userId string) *admin.User {
	funcName := "GetUserById"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	user := c.getUser(userId)
	if user == nil {
		c.addUser(user)
	}
	return user
}

// GetGroupMembers will get the members of the group specified
func (c *client) GetGroupMembersBy(g *admin.Group) ([]*admin.Member, error) {
	funcName := "GetGroupMembersBy"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	if err := c.refreshMembers(); err != nil {
		return nil, err
	}

	return c.members[g.Id], nil
}

// GetGroupMembers will get the members for all groups
func (c *client) GetGroupMembers() (map[string][]*admin.Member, error) {
	funcName := "GetGroupMembers"
	log.WithField("module", ModuleName).WithField("func", funcName).Debug(funcName + "()")

	if err := c.refreshMembers(); err != nil {
		return nil, err
	}

	return c.members, nil
}

// GetUsers will get the users from Google's Admin API
// using the Method: users.list with parameter "query"
// References:
// * https://developers.google.com/admin-sdk/directory/reference/rest/v1/users/list
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
func (c *client) GetUsers() ([]*admin.User, error) {
	log.Debug("google.GetUsers()")
	if err := c.refreshUsers(); err != nil {
		return nil, err
	}

	return c.users, nil
}

// GetGroups will get the groups from Google's Admin API
// using the Method: groups.list with parameter "query"
// References:
// * https://developers.google.com/admin-sdk/directory/reference/rest/v1/groups/list
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
func (c *client) GetGroups() ([]*admin.Group, error) {
	log.Debug("google.GetGroups()")
	if err := c.refreshGroups(); err != nil {
		return nil, err
	}

	return c.groups, nil
}
