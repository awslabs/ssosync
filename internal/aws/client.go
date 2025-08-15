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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	net_url "net/url"
	"strings"

	"github.com/awslabs/ssosync/internal/constants"

	internal_http "github.com/awslabs/ssosync/internal/http"
	"github.com/awslabs/ssosync/internal/interfaces"
	log "github.com/sirupsen/logrus"
)

var (
	// ErrUserNotFound
	ErrUserNotFound = errors.New("user not found")
	// ErrGroupNotFound
	ErrGroupNotFound = errors.New("group not found")
	// ErrUserNotSpecified
	ErrUserNotSpecified = errors.New("user not specified")
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
	CreateUser(*interfaces.User) (*interfaces.User, error)
	FindGroupByDisplayName(string) (*interfaces.Group, error)
	FindUserByEmail(string) (*interfaces.User, error)
	UpdateUser(*interfaces.User) (*interfaces.User, error)
}

type client struct {
	httpClient  internal_http.Client
	baseURL     string
	bearerToken string
}

type QueryTransformer = func(u *http.Request)

// NewClient creates a new client to talk with AWS SSO's SCIM endpoint. It
// requires a http.Client{} as well as the URL and bearer token from the
// console. If the URL is not parsable, an error will be thrown.
func NewClient(c internal_http.Client, config *Config) (Client, error) {
	u, err := net_url.Parse(config.Endpoint)

	if err != nil || !strings.HasPrefix(u.Scheme, "https") {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}
	return &client{
		httpClient:  c,
		baseURL:     u.String(),
		bearerToken: config.Token,
	}, nil
}

func (c *client) prepareRequest(method string, path string, body any) (req *http.Request, err error) {
	if body == nil {
		req, err = http.NewRequest(method, c.baseURL+path, nil)

		if err != nil {
			return nil, err
		}

	} else {
		d, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequest(method, c.baseURL+path, strings.NewReader(string(d)))
		if err != nil {
			return nil, err
		}
	}

	log.WithFields(log.Fields{"url": c.baseURL, "path": path, "method": method}).Debug("Preparing request")

	// Set the content-type and authorization headers
	req.Header.Set("Content-Type", constants.ContentTypeSCIM)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))
	return req, nil
}

func close(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		log.WithError(err).Warn("Failed to close response body")
	}
}

func (c *client) get(path string, beforeSend QueryTransformer) (response []byte, err error) {
	log.Debug("Sending GET request to ", path)
	// Validate URL
	req, err := c.prepareRequest(http.MethodGet, path, nil)

	if err != nil {
		return nil, err
	}
	if beforeSend != nil {
		beforeSend(req)
		log.WithFields(log.Fields{"query": req.URL.RawQuery}).Debug("Sending GET request to ", path)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Debugf("HTTP error for GET %s: %v", path, err)
		return
	}

	log.Debugf("GET %s returned status: %d", path, resp.StatusCode)
	if resp.Body == nil {
		return nil, &ErrHTTPNotOK{resp.StatusCode}
	}
	defer close(resp.Body)

	response, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Debugf("Error reading response body for GET %s: %v", path, err)
		return
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		log.Debugf("Non-2xx status code %d for GET %s", resp.StatusCode, path)
		err = &ErrHTTPNotOK{resp.StatusCode}
	}

	return
}

func (c *client) post(path string, body any) (response []byte, err error) {
	log.Debug("Sending POST request to ", path)
	// Validate URL
	req, err := c.prepareRequest(http.MethodPost, path, body)

	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Debugf("HTTP error for POST %s: %v", path, err)
		return
	}

	log.Debugf("POST %s returned status: %d", path, resp.StatusCode)
	defer close(resp.Body)

	response, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Debugf("Error reading response body for POST %s: %v", path, err)
		return
	}

	// If we get a non-2xx status code, raise that via an error
	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		log.Debugf("Non-2xx status code %d for POST %s", resp.StatusCode, path)
		err = &ErrHTTPNotOK{resp.StatusCode}
	}

	return

}

func (c *client) put(path string, body any) (response []byte, err error) {
	log.Debug("Sending PUT request to ", path)
	// Validate URL
	req, err := c.prepareRequest(http.MethodPut, path, body)

	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Debugf("HTTP error for PUT %s: %v", path, err)
		return
	}

	log.Debugf("PUT %s returned status: %d", path, resp.StatusCode)
	defer close(resp.Body)

	response, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Debugf("Error reading response body for PUT %s: %v", path, err)
		return
	}

	// If we get a non-2xx status code, raise that via an error
	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		log.Debugf("Non-2xx status code %d for PUT %s", resp.StatusCode, path)
		err = &ErrHTTPNotOK{resp.StatusCode}
	}

	return
}

func beforeSendAddFilter(filter string) QueryTransformer {
	return func(r *http.Request) {
		q := r.URL.Query()
		q.Add("filter", filter)
		r.URL.RawQuery = q.Encode()
	}
}

// FindUserByEmail will find the user by the email address specified
func (c *client) FindUserByEmail(email string) (*interfaces.User, error) {
	log.Debugf("Finding user by email: %s", email)
	filter := fmt.Sprintf("userName eq \"%s\"", email)

	//do a get to /Users and add filter=userName eq "email"
	resp, err := c.get("/Users", beforeSendAddFilter(filter))

	if err != nil {
		log.Debugf("Error finding user %s: %v", email, err)
		return nil, err
	}

	var r interfaces.UserFilterResults
	err = json.Unmarshal(resp, &r)
	if err != nil {
		log.Debugf("Error unmarshaling user response for %s: %v", email, err)
		return nil, err
	}

	log.Debugf("User search for %s returned %d results", email, r.TotalResults)
	if r.TotalResults != 1 {
		return nil, ErrUserNotFound
	}

	log.Debugf("Found user: %s (ID: %s)", email, r.Resources[0].ID)
	return &r.Resources[0], nil
}

func (c *client) FindGroupByDisplayName(name string) (*interfaces.Group, error) {
	log.Debugf("Finding group by display name: %s", name)
	filter := fmt.Sprintf("displayName eq \"%s\"", name)

	//do a get to /Groups and add filter=displayName eq "name"
	resp, err := c.get("/Groups", beforeSendAddFilter(filter))

	if err != nil {
		log.Debugf("Error finding group %s: %v", name, err)
		return nil, err
	}

	var r interfaces.GroupFilterResults
	err = json.Unmarshal(resp, &r)
	if err != nil {
		log.Debugf("Error unmarshaling group response for %s: %v", name, err)
		return nil, err
	}

	log.Debugf("Group search for %s returned %d results", name, r.TotalResults)
	if r.TotalResults != 1 {
		return nil, ErrGroupNotFound
	}

	log.Debugf("Found group: %s (ID: %s)", name, r.Resources[0].ID)
	return &r.Resources[0], nil
}

// CreateUser will create the user specified
func (c *client) CreateUser(u *interfaces.User) (*interfaces.User, error) {
	if u == nil {
		return nil, ErrUserNotSpecified
	}

	log.Debugf("Creating user: %s", u.Username)
	resp, err := c.post("/Users", *u)
	if err != nil {
		log.Debugf("Error creating user %s: %v", u.Username, err)
		return nil, err
	}

	var newUser interfaces.User
	err = json.Unmarshal(resp, &newUser)
	if err != nil {
		log.Debugf("Error unmarshaling create user response for %s: %v", u.Username, err)
		return nil, err
	}
	if newUser.ID == "" {
		log.Debugf("User %s created but no ID returned, finding by email", u.Username)
		return c.FindUserByEmail(u.Username)
	}

	log.Debugf("Successfully created user: %s (ID: %s)", u.Username, newUser.ID)
	return &newUser, nil
}

// UpdateUser will update/replace the user specified
func (c *client) UpdateUser(u *interfaces.User) (*interfaces.User, error) {
	if u == nil {
		return nil, ErrUserNotFound
	}

	log.Debugf("Updating user: %s (ID: %s)", u.Username, u.ID)
	resp, err := c.put(fmt.Sprintf("/Users/%s", u.ID), *u)
	if err != nil {
		log.Debugf("Error updating user %s: %v", u.Username, err)
		return nil, err
	}

	var newUser interfaces.User
	err = json.Unmarshal(resp, &newUser)
	if err != nil {
		log.Debugf("Error unmarshaling update user response for %s: %v", u.Username, err)
		return nil, err
	}
	if newUser.ID == "" {
		log.Debugf("User %s updated but no ID returned, finding by email", u.Username)
		return c.FindUserByEmail(u.Username)
	}

	log.Debugf("Successfully updated user: %s (ID: %s)", u.Username, newUser.ID)
	return &newUser, nil
}
