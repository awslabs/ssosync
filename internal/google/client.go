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
	"net/http"

	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// IClient is the Interface for the Client
type IClient interface {
	GetUsers() ([]*admin.User, error)
	GetGroups() ([]*admin.Group, error)
	GetGroupMembers(*admin.Group) ([]*admin.Member, error)
}

// Client is the Google Apps for Domains Client
type Client struct {
	client  *http.Client
	service *admin.Service
}

// NewClient creates a new client for Google's Admin API
func NewClient(client *AuthClient) (IClient, error) {
	c, err := client.GetClient()
	if err != nil {
		return nil, err
	}

	srv, err := admin.NewService(context.TODO(), option.WithHTTPClient(c))
	if err != nil {
		return nil, err
	}

	return &Client{
		client:  c,
		service: srv,
	}, nil
}

// GetUsers will get the users from Google's Admin API
func (c *Client) GetUsers() (u []*admin.User, err error) {
	u = make([]*admin.User, 0)
	err = c.service.Users.List().Customer("my_customer").Pages(context.TODO(), func(users *admin.Users) error {
		u = append(u, users.Users...)
		return nil
	})

	return
}

// GetGroups will get the groups from Google's Admin API
func (c *Client) GetGroups() (g []*admin.Group, err error) {
	g = make([]*admin.Group, 0)
	err = c.service.Groups.List().Customer("my_customer").Pages(context.TODO(), func(groups *admin.Groups) error {
		g = append(g, groups.Groups...)
		return nil
	})

	return
}

// GetGroupMembers will get the members of the group specified
func (c *Client) GetGroupMembers(g *admin.Group) (m []*admin.Member, err error) {
	m = make([]*admin.Member, 0)
	err = c.service.Members.List(g.Id).Pages(context.TODO(), func(members *admin.Members) error {
		m = append(m, members.Members...)
		return nil
	})

	return
}
