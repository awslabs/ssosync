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
	// ErrUserNotFound
	ErrUserNotFound      = errors.New("user not found")
	// ErrGroupNotFound
	ErrGroupNotFound     = errors.New("group not found")
	// ErrUserNotSpecified
	ErrUserNotSpecified  = errors.New("user not specified")
)

// ErrHTTPNotOK
type ErrHTTPNotOK struct {
	StatusCode int
}

func (e *ErrHTTPNotOK) Error() string {
	return fmt.Sprintf("status of http response was %d", e.StatusCode)
}

// OperationType handle patch operations for add/remove
type OperationType string

const (
	// OperationAdd is the add operation for a patch
	OperationAdd OperationType = "add"

	// OperationRemove is the remove operation for a patch
	OperationRemove = "remove"
)

// Client represents an interface of methods used
// to communicate with AWS SSO
type Client interface {
	CreateUser(*User) (*User, error)
	FindGroupByDisplayName(string) (*Group, error)
	FindUserByEmail(string) (*User, error)
	UpdateUser(*User) (*User, error)
}

type client struct {
	httpClient  HTTPClient
	endpointURL *url.URL
	bearerToken string
}

// NewClient creates a new client to talk with AWS SSO's SCIM endpoint. It
// requires a http.Client{} as well as the URL and bearer token from the
// console. If the URL is not parsable, an error will be thrown.
func NewClient(c HTTPClient, config *Config) (Client, error) {
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
		err = &ErrHTTPNotOK{resp.StatusCode}
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
func (c *client) CreateUser(u *User) (*User, error) {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return nil, err
	}

	if u == nil {
		err = ErrUserNotSpecified
		return nil, err
	}

	startURL.Path = path.Join(startURL.Path, "/Users")
	resp, err := c.sendRequestWithBody(http.MethodPost, startURL.String(), *u)
	if err != nil {
		return nil, err
	}

	var newUser User
	err = json.Unmarshal(resp, &newUser)
	if err != nil {
		return nil, err
	}
	if newUser.ID == "" {
		return c.FindUserByEmail(u.Username)
	}

	return &newUser, nil
}

// UpdateUser will update/replace the user specified
func (c *client) UpdateUser(u *User) (*User, error) {
	startURL, err := url.Parse(c.endpointURL.String())
	if err != nil {
		return nil, err
	}

	if u == nil {
		err = ErrUserNotFound
		return nil, err
	}

	startURL.Path = path.Join(startURL.Path, fmt.Sprintf("/Users/%s", u.ID))
	resp, err := c.sendRequestWithBody(http.MethodPut, startURL.String(), *u)
	if err != nil {
		return nil, err
	}

	var newUser User
	err = json.Unmarshal(resp, &newUser)
	if err != nil {
		return nil, err
	}
	if newUser.ID == "" {
		return c.FindUserByEmail(u.Username)
	}

	return &newUser, nil
}
