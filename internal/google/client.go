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
	"errors"

	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// Client is the Interface for the Client
type Client interface {
	GetUsers(string) ([]*admin.User, error)
	GetDeletedUsers() ([]*admin.User, error)
	GetGroups(string) ([]*admin.Group, error)
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

	// If we have an empty query, return nothing.
	if query == "" {
		return u, err
	}

	// If we have wildcard then fetch all users
	if query  == "*" {
                err = c.service.Users.List().Customer("my_customer").Pages(c.ctx, func(users *admin.Users) error {
                        u = append(u, users.Users...)
                        return nil
                })
        } else {

	        // The Google api doesn't support multi-part queries, but we do so we need to split into an array of query strings
		queries := strings.Split(query, ",")

		// Then call the api one query at a time, appending to our list
		for _, subQuery := range queries {
			err = c.service.Users.List().Query(subQuery).Customer("my_customer").Pages(c.ctx, func(users *admin.Users) error {
				u = append(u, users.Users...)
				return nil
			})
		}
	}

	// some people prefer to go by a mononym
	// Google directory will accept a 'zero width space' for an empty name but will not accept a 'space'
	// but
	// Identity Store will accept and a 'space' for an empty name but not a 'zero width space'
	// So we need to replace any 'zero width space' strings with a single 'space' to allow comparison and sync
	for _, user := range u {
		user.Name.GivenName = strings.Replace(user.Name.GivenName, string('\u200B'), " ", -1)
        	user.Name.FamilyName = strings.Replace(user.Name.FamilyName, string('\u200B'), " ", -1)
	}

	// Check we've got some users otherwise something is wrong.
        if len(u) == 0 {
                return u, errors.New("google api returned 0 users?")
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

        // If we have an empty query, then we are not looking for groups
        if query  == "" {
                return g, err
        }

        // If we have wildcard then fetch all groups
        if query  == "*" {
		err = c.service.Groups.List().Customer("my_customer").Pages(context.TODO(), func(groups *admin.Groups) error {
                        g = append(g, groups.Groups...)
                        return nil
                })
		return g, err
        } else {

        	// The Google api doesn't support multi-part queries, but we do so we need to split into an array of query strings
        	queries := strings.Split(query, ",")

        	// Then call the api one query at a time, appending to our list
        	for _, subQuery := range queries {
			err = c.service.Groups.List().Customer("my_customer").Query(subQuery).Pages(context.TODO(), func(groups *admin.Groups) error {
				g = append(g, groups.Groups...)
				return nil
			})
		}
	}

	// Check we've got some users otherwise something is wrong.
	if len(g) == 0 {
		return g, errors.New("google api return 0 groups?")
	}
	return g, err
}
