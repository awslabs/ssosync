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
	"strconv"

	"go.uber.org/zap"
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
type IClient interface {
	GetUsers() (*map[string]User, error)
	GetGroups() (*map[string]Group, error)
	IsUserInGroup(*User, *Group) (bool, error)
	FindUserByEmail(string) (*User, error)
	CreateUser(*User) (*User, error)
	DeleteUser(*User) error
	CreateGroup(*Group) (*Group, error)
	DeleteGroup(*Group) error
	AddUserToGroup(*User, *Group) error
	RemoveUserFromGroup(*User, *Group) error
}

// Client represents an AWS SSO SCIM client
type Client struct {
	logger      *zap.Logger
	httpClient  IHttpClient
	endpointURL *url.URL
	bearerToken string
}

// NewClient creates a new client to talk with AWS SSO's SCIM endpoint. It
// required a http.Client{} as well as the URL and bearer token from the
/// console. If the URL is not parsable, an error will be thrown.
func NewClient(logger *zap.Logger, client IHttpClient, config *Config) (IClient, error) {
	u, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, err
	}
	return &Client{
		logger:      logger,
		httpClient:  client,
		endpointURL: u,
		bearerToken: config.Token,
	}, nil
}

// sendRequestWithBody will send the body given to the url/method combination
// with the right Bearer token as well as the correct content type for SCIM.
func (c *Client) sendRequestWithBody(method string, url string, body interface{}) (response []byte, err error) {
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

	c.logger.Debug("sendRequestWithBody", zap.String("url", url), zap.String("method", method))

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

func (c *Client) sendRequest(method string, url string) (response []byte, err error) {
	r, err := http.NewRequest(method, url, nil)
	if err != nil {
		return
	}

	c.logger.Debug("sendRequest", zap.String("url", url), zap.String("method", method))

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

// getUserPage will retrieve a page of users from AWS SSO using the base URL (sURL)
// and adding the query parameters to it. startIndex declares where to start in
// the pagination.
func (c *Client) getUserPage(sURL *url.URL, startIndex int) (results *UserFilterResults, err error) {
	startURL, err := url.Parse(sURL.String())
	if err != nil {
		return
	}

	// Build query parameters of a count of 10, and the startIndex
	q := startURL.Query()
	q.Add("count", "10")
	q.Add("startIndex", strconv.Itoa(startIndex))
	startURL.RawQuery = q.Encode()

	// Send query to AWS SSO
	resp, err := c.sendRequest(http.MethodGet, startURL.String())
	if err != nil {
		return
	}

	// Convert body back to the filtered results object type
	var r UserFilterResults
	err = json.Unmarshal(resp, &r)
	if err != nil {
		return
	}

	// Return the results
	results = &r
	return
}

// GetUsers will get a map of users, keyed by username, from AWS SSO
func (c *Client) GetUsers() (results *map[string]User, err error) {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return
	}

	startURL.Path = path.Join(startURL.Path, "/Users")

	var resultMap = make(map[string]User)

	si := 1
	for {
		c.logger.Debug("Getting Users Page", zap.Int("startIndex", si))
		r, err := c.getUserPage(startURL, si)
		if err != nil {
			return nil, err
		}

		for _, user := range r.Resources {
			c.logger.Debug("Add user to map", zap.String("username", user.Username))
			resultMap[user.Username] = user
		}

		si = si + 10
		if si > r.TotalResults {
			c.logger.Debug("Last Page obtained", zap.Int("totalResults", r.TotalResults))
			break
		}
	}

	return &resultMap, nil
}

func (c *Client) getGroupPage(sURL *url.URL, startIndex int) (results *GroupFilterResults, err error) {
	startURL, err := url.Parse(sURL.String())
	if err != nil {
		return
	}

	q := startURL.Query()
	q.Add("count", "10")
	q.Add("startIndex", strconv.Itoa(startIndex))
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

	results = &r

	return
}

// GetGroups will retrieve a map of Groups from AWS SSO. The map
// is keyed by the Display Name of the group.
func (c *Client) GetGroups() (results *map[string]Group, err error) {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return
	}

	startURL.Path = path.Join(startURL.Path, "/Groups")

	var resultGroup = make(map[string]Group)

	si := 1
	for {
		c.logger.Debug("Getting Groups Page", zap.Int("startIndex", si))

		r, err := c.getGroupPage(startURL, si)
		if err != nil {
			return nil, err
		}

		for _, group := range r.Resources {
			c.logger.Debug("Add group to map", zap.String("group", group.DisplayName))
			resultGroup[group.DisplayName] = group
		}

		si = si + 10
		if si > r.TotalResults {
			c.logger.Debug("Last Page obtained", zap.Int("totalResults", r.TotalResults))
			break
		}
	}

	return &resultGroup, nil
}

// IsUserInGroup will determine if user (u) is in group (g)
func (c *Client) IsUserInGroup(u *User, g *Group) (present bool, err error) {
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

func (c *Client) groupChangeOperation(op OperationType, u *User, g *Group) error {
	if g == nil {
		return errors.New("no group specified")
	}

	if u == nil {
		return errors.New("no user specified")
	}

	c.logger.Debug("groupChangeOperation",
		zap.String("operation", string(op)),
		zap.String("user", u.Username),
		zap.String("group", g.DisplayName),
	)

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
func (c *Client) AddUserToGroup(u *User, g *Group) error {
	return c.groupChangeOperation(OperationAdd, u, g)
}

// RemoveUserFromGroup will remove the user specified from the group specified
func (c *Client) RemoveUserFromGroup(u *User, g *Group) error {
	return c.groupChangeOperation(OperationRemove, u, g)
}

// FindUserByEmail will find the user by the email address specified
func (c *Client) FindUserByEmail(email string) (user *User, err error) {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return
	}

	filter := fmt.Sprintf("userName eq \"%s\"", email)

	startURL.Path = path.Join(startURL.Path, "/Users")
	q := startURL.Query()
	q.Add("filter", filter)

	startURL.RawQuery = q.Encode()

	resp, err := c.sendRequest(http.MethodGet, startURL.String())
	if err != nil {
		return
	}

	var r UserFilterResults
	err = json.Unmarshal(resp, &r)
	if err != nil {
		return
	}

	if r.TotalResults != 1 {
		err = fmt.Errorf("%s not found in AWS SSO", email)
		return
	}

	user = &r.Resources[0]

	return
}

// CreateUser will create the user specified
func (c *Client) CreateUser(u *User) (user *User, err error) {
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
func (c *Client) DeleteUser(u *User) error {
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
func (c *Client) CreateGroup(g *Group) (group *Group, err error) {
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
func (c *Client) DeleteGroup(g *Group) error {
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
