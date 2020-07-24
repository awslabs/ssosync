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

package aws

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	log "github.com/sirupsen/logrus"
)

var (
	ErrUserNotFound  = errors.New("user no found")
	ErrGroupNotFound = errors.New("group not found")
)

// OperationType handle patch operations for add/remove
type OperationType string

const (
	// OperationAdd is the add operation for a patch
	OperationAdd OperationType = "add"

	// OperationRemove is the remove operation for a patch
	OperationRemove = "remove"
)

// IClient represents an interface of methods used
// to communicate with AWS SSO
type Client interface {
	AddUserToGroup(*User, *Group) error
	CreateGroup(*Group) (*Group, error)
	CreateUser(*User) (*User, error)
	DeleteGroup(*Group) error
	DeleteUser(*User) error
	FindGroupByDisplayName(string) (*Group, error)
	FindUserByEmail(string) (*User, error)
	IsUserInGroup(*User, *Group) (bool, error)
	RemoveUserFromGroup(*User, *Group) error
}

type client struct {
	httpClient  HttpClient
	endpointURL *url.URL
	bearerToken string
}

// NewClient creates a new client to talk with AWS SSO's SCIM endpoint. It
// required a http.Client{} as well as the URL and bearer token from the
/// console. If the URL is not parsable, an error will be thrown.
func NewClient(c HttpClient, config *Config) (Client, error) {
	u, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, err
	}
	return &client{
		httpClient:  c,
		endpointURL: u,
		bearerToken: config.Token,
	}, nil
}

// sendRequestWithBody will send the body given to the url/method combination
// with the right Bearer token as well as the correct content type for SCIM.
func (c *client) sendRequestWithBody(method string, url string, body interface{}) (response []byte, err error) {
	// Convert the body to JSON
	d, err := json.Marshal(body)
	if err != nil {
		return
	}

	// Create a request with our body of JSON
	r, err := http.NewRequest(method, url, bytes.NewBuffer(d))
	if err != nil {
		return
	}

	log.WithFields(log.Fields{"url": url, "method": method})

	// Set the content-type and authorization headers
	r.Header.Set("Content-Type", "application/scim+json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))

	// Call the URL
	resp, err := c.httpClient.Do(r)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Read the body back from the response
	response, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// If we get a non-2xx status code, raise that via an error
	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		err = fmt.Errorf("status of http response was %d", resp.StatusCode)
	}

	return
}

func (c *client) sendRequest(method string, url string) (response []byte, err error) {
	r, err := http.NewRequest(method, url, nil)
	if err != nil {
		return
	}

	log.WithFields(log.Fields{"url": url, "method": method})

	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))

	resp, err := c.httpClient.Do(r)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	response, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		err = fmt.Errorf("status of http response was %d", resp.StatusCode)
	}

	return
}

// IsUserInGroup will determine if user (u) is in group (g)
func (c *client) IsUserInGroup(u *User, g *Group) (present bool, err error) {
	if g == nil {
		return false, errors.New("no group specified")
	}

	if u == nil {
		return false, errors.New("no user specified")
	}

	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return
	}

	filter := fmt.Sprintf("id eq \"%s\" and members eq \"%s\"", g.ID, u.ID)

	startURL.Path = path.Join(startURL.Path, "/Groups")
	q := startURL.Query()
	q.Add("filter", filter)

	startURL.RawQuery = q.Encode()
	resp, err := c.sendRequest(http.MethodGet, startURL.String())
	if err != nil {
		return
	}

	var r GroupFilterResults
	err = json.Unmarshal(resp, &r)
	if err != nil {
		return
	}

	present = r.TotalResults > 0

	return
}

func (c *client) groupChangeOperation(op OperationType, u *User, g *Group) error {
	if g == nil {
		return errors.New("no group specified")
	}

	if u == nil {
		return errors.New("no user specified")
	}

	log.WithFields(log.Fields{"operations": op, "user": u.Username, "group": g.DisplayName}).Debug("Group Change")

	gc := &GroupMemberChange{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: []GroupMemberChangeOperation{
			{
				Operation: string(op),
				Path:      "members",
				Members: []GroupMemberChangeMember{
					{Value: u.ID},
				},
			},
		},
	}

	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return err
	}

	startURL.Path = path.Join(startURL.Path, fmt.Sprintf("/Groups/%s", g.ID))
	_, err = c.sendRequestWithBody(http.MethodPatch, startURL.String(), *gc)
	if err != nil {
		return err
	}

	return nil
}

// AddUserToGroup will add the user specified to the group specified
func (c *client) AddUserToGroup(u *User, g *Group) error {
	return c.groupChangeOperation(OperationAdd, u, g)
}

// RemoveUserFromGroup will remove the user specified from the group specified
func (c *client) RemoveUserFromGroup(u *User, g *Group) error {
	return c.groupChangeOperation(OperationRemove, u, g)
}

// FindUserByEmail will find the user by the email address specified
func (c *client) FindUserByEmail(email string) (*User, error) {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return nil, err
	}

	filter := fmt.Sprintf("userName eq \"%s\"", email)

	startURL.Path = path.Join(startURL.Path, "/Users")
	q := startURL.Query()
	q.Add("filter", filter)

	startURL.RawQuery = q.Encode()

	resp, err := c.sendRequest(http.MethodGet, startURL.String())
	if err != nil {
		return nil, err
	}

	var r UserFilterResults
	err = json.Unmarshal(resp, &r)
	if err != nil {
		return nil, err
	}

	if r.TotalResults != 1 {
		return nil, ErrUserNotFound
	}

	return &r.Resources[0], nil
}

// FindGroupByDisplayName will find the group by its displayname.
func (c *client) FindGroupByDisplayName(name string) (*Group, error) {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return nil, err
	}

	filter := fmt.Sprintf("displayName eq \"%s\"", name)

	startURL.Path = path.Join(startURL.Path, "/Groups")
	q := startURL.Query()
	q.Add("filter", filter)

	startURL.RawQuery = q.Encode()

	resp, err := c.sendRequest(http.MethodGet, startURL.String())
	if err != nil {
		return nil, err
	}

	var r GroupFilterResults
	err = json.Unmarshal(resp, &r)
	if err != nil {
		return nil, err
	}

	if r.TotalResults != 1 {
		return nil, ErrGroupNotFound
	}

	return &r.Resources[0], nil
}

// CreateUser will create the user specified
func (c *client) CreateUser(u *User) (user *User, err error) {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return
	}

	if u == nil {
		err = errors.New("no user defined")
		return
	}

	startURL.Path = path.Join(startURL.Path, "/Users")
	resp, err := c.sendRequestWithBody(http.MethodPost, startURL.String(), *u)
	if err != nil {
		return
	}

	var newUser User
	err = json.Unmarshal(resp, &newUser)
	if err != nil {
		return
	}
	if newUser.ID == "" {
		user, err = c.FindUserByEmail(u.Username)
		return
	}

	user = &newUser
	return
}

// DeleteUser will remove the current user from the directory
func (c *client) DeleteUser(u *User) error {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return err
	}

	if u == nil {
		return errors.New("no user specified")
	}

	startURL.Path = path.Join(startURL.Path, fmt.Sprintf("/Users/%s", u.ID))
	_, err = c.sendRequest(http.MethodDelete, startURL.String())
	if err != nil {
		return err
	}

	return nil
}

// CreateGroup will create a group given
func (c *client) CreateGroup(g *Group) (group *Group, err error) {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return
	}

	if g == nil {
		err = errors.New("no group defined")
		return
	}

	startURL.Path = path.Join(startURL.Path, "/Groups")
	resp, err := c.sendRequestWithBody(http.MethodPost, startURL.String(), *g)
	if err != nil {
		return
	}

	var newGroup Group
	err = json.Unmarshal(resp, &newGroup)
	if err != nil {
		return
	}

	group = &newGroup
	return
}

// DeleteGroup will delete the group specified
func (c *client) DeleteGroup(g *Group) error {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return err
	}

	if g == nil {
		return errors.New("no group specified")
	}

	startURL.Path = path.Join(startURL.Path, fmt.Sprintf("/Groups/%s", g.ID))
	_, err = c.sendRequest(http.MethodDelete, startURL.String())
	if err != nil {
		return err
	}

	return nil
}
