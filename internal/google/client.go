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

package google

import (
	"context"

	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// Client is the Interface for the Client
type Client interface {
	GetUsers() ([]*admin.User, error)
	GetDeletedUsers() ([]*admin.User, error)
	GetGroups() ([]*admin.Group, error)
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

	config.Subject = adminEmail

	if err != nil {
		return nil, err
	}

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

// GetUsers will get the users from Google's Admin API
func (c *client) GetUsers() ([]*admin.User, error) {
	u := make([]*admin.User, 0)
	err := c.service.Users.List().Customer("my_customer").Pages(c.ctx, func(users *admin.Users) error {
		u = append(u, users.Users...)
		return nil
	})

	return u, err
}

// GetGroups will get the groups from Google's Admin API
func (c *client) GetGroups() ([]*admin.Group, error) {
	g := make([]*admin.Group, 0)
	err := c.service.Groups.List().Customer("my_customer").Pages(context.TODO(), func(groups *admin.Groups) error {
		g = append(g, groups.Groups...)
		return nil
	})

	return g, err
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
