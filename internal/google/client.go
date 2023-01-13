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

	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// Client is the Interface for the Client
type Client interface {
	GetUsers(string) ([]*admin.User, error)
	GetUser(string) (*admin.User, error)
	GetDeletedUsers() ([]*admin.User, error)
	GetGroups(string) ([]*admin.Group, error)
	GetGroup(string) (*admin.Group, error)
	GetGroupMembers(*admin.Group) ([]*admin.Member, error)
}

type client struct {
	ctx     context.Context
	service *admin.Service
}

// NewClient creates a new client for Google's Admin API
func NewClient(ctx context.Context, adminEmail string, serviceAccountKey []byte) (Client, error) {
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

	return &client{
		ctx:     ctx,
		service: srv,
	}, nil
}

// GetDeletedUsers will get the deleted users from the Google's Admin API.
func (c *client) GetDeletedUsers() ([]*admin.User, error) {
	u := make([]*admin.User, 0)
	err := c.service.Users.List().Customer("my_customer").ShowDeleted("true").Pages(c.ctx, func(users *admin.Users) error {
		u = append(u, users.Users...)
		return nil
	})

	return u, err
}

// GetGroupMembers will get the members of the group specified
func (c *client) GetGroupMembers(g *admin.Group) ([]*admin.Member, error) {
	m := make([]*admin.Member, 0)
	err := c.service.Members.List(g.Id).Pages(context.TODO(), func(members *admin.Members) error {
		m = append(m, members.Members...)
		return nil
	})

	return m, err
}

// GetUsers will get the users from Google's Admin API
// using the Method: users.list with parameter "query"
// References:
// * https://developers.google.com/admin-sdk/directory/reference/rest/v1/users/list
// * https://developers.google.com/admin-sdk/directory/v1/guides/search-users
// query possible values:
// '' --> empty or not defined
//  name:'Jane'
//  email:admin*
//  isAdmin=true
//  manager='janesmith@example.com'
//  orgName=Engineering orgTitle:Manager
//  EmploymentData.projects:'GeneGnomes'
func (c *client) GetUsers(query string) ([]*admin.User, error) {
	u := make([]*admin.User, 0)
	var err error

	if query != "" {
		err = c.service.Users.List().Query(query).Customer("my_customer").Pages(c.ctx, func(users *admin.Users) error {
			u = append(u, users.Users...)
			return nil
		})

	} else {
		err = c.service.Users.List().Customer("my_customer").Pages(c.ctx, func(users *admin.Users) error {
			u = append(u, users.Users...)
			return nil
		})
	}

	return u, err
}

// GetGroups will get the groups from Google's Admin API
// using the Method: groups.list with parameter "query"
// References:
// * https://developers.google.com/admin-sdk/directory/reference/rest/v1/groups/list
// * https://developers.google.com/admin-sdk/directory/v1/guides/search-groups
// query possible values:
// '' --> empty or not defined
//  name='contact'
//  email:admin*
//  memberKey=user@company.com
//  name:contact* email:contact*
//  name:Admin* email:aws-*
//  email:aws-*
func (c *client) GetGroups(query string) ([]*admin.Group, error) {
	g := make([]*admin.Group, 0)
	var err error

	if query != "" {
		err = c.service.Groups.List().Customer("my_customer").Query(query).Pages(context.TODO(), func(groups *admin.Groups) error {
			g = append(g, groups.Groups...)
			return nil
		})
	} else {
		err = c.service.Groups.List().Customer("my_customer").Pages(context.TODO(), func(groups *admin.Groups) error {
			g = append(g, groups.Groups...)
			return nil
		})

	}
	return g, err
}

// GetGroup will get a single group from Google's Admin API
// using the Method: groups.get with parameter "groupKey"
// References:
// * https://developers.google.com/admin-sdk/directory/reference/rest/v1/groups/get
// groupKey possible values:
//  group email address
//  group alias
//  unique group ID
func (c *client) GetGroup(groupKey string) (*admin.Group, error) {
	group, err := c.service.Groups.Get(groupKey).Context(context.TODO()).Do()
	return group, err
}

// GetUser will get a single user from Google's Admin API
// using the Method: users.get with parameter "userKey"
// References:
// * https://developers.google.com/admin-sdk/directory/reference/rest/v1/users/get
// userKey possible values:
//  users primary email address
//  alias email address
//  unique user ID
func (c *client) GetUser(userKey string) (*admin.User, error) {
	user, err := c.service.Users.Get(userKey).Context(context.TODO()).Do()
	return user, err
}
