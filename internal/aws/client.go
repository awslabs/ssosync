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

	"ssosync/internal/constants"
	internal_http "ssosync/internal/http"
	"ssosync/internal/interfaces"

	log "github.com/sirupsen/logrus"
)

var (
	// ErrUserNotFound
	ErrUserNotFound = errors.New("user not found")
	// ErrGroupNotFound
	ErrGroupNotFound = errors.New("group not found")
	// ErrUserNotSpecified
	ErrUserNotSpecified = errors.New("user not specified")
	// ErrGroupNotSpecified
	ErrGroupNotSpecified = errors.New("group not specified")
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
	// OperationReplace is the update operation for a patch
	OperationReplace = "replace"
	// MaxMemberChanges is the maximum number of userids that
	// can be provided in a group PATCH request
	MaxMemberChanges = 100
)

// Client represents an interface of methods used
// to communicate with AWS SSO
type Client interface {
	AddUserToGroup(*interfaces.User, *interfaces.Group) error
	CreateGroup(*interfaces.Group) (*interfaces.Group, error)
	UpdateGroup(*interfaces.Group) (*interfaces.Group, error)
	CreateUser(*interfaces.User) (*interfaces.User, error)
	DeleteGroup(*interfaces.Group) error
	DeleteUser(*interfaces.User) error
	FindGroupByDisplayName(string) (*interfaces.Group, error)
	FindUserByEmail(string) (*interfaces.User, error)
	UpdateUser(*interfaces.User) (*interfaces.User, error)
	RemoveUserFromGroup(*interfaces.User, *interfaces.Group) error
	CreateUsers([]*interfaces.User, []*interfaces.User) ([]*interfaces.User, error)
	UpdateUsers([]*interfaces.User, []*interfaces.User) ([]*interfaces.User, error)
	DeleteUsers([]*interfaces.User) error
	CreateGroups([]*interfaces.Group, []*interfaces.Group) ([]*interfaces.Group, error)
	UpdateGroups([]*interfaces.Group, []*interfaces.Group) ([]*interfaces.Group, error)
	DeleteGroups([]*interfaces.Group) error
	AddMembers(map[string][]string, map[string][]string) (map[string][]string, error)
	RemoveMembers(map[string][]string) error
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
		log.Debugf("prepareRequest: Path %s JSON Body %s", path, string(d))
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

	log.Debugf("HTTP Request for POST %s: %v", path, req)

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

	log.Debugf("HTTP Request for PUT %s: %v", path, req)

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

func (c *client) patch(path string, body any) (response []byte, err error) {
	log.Debug("Sending PATCH request to ", path)
	// Validate URL
	req, err := c.prepareRequest(http.MethodPatch, path, body)

	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Debugf("HTTP error for PATCH %s: %v", path, err)
		return
	}

	log.Debugf("PATCH %s returned status: %d", path, resp.StatusCode)
	defer close(resp.Body)

	response, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Debugf("Error reading response body for PATCH %s: %v", path, err)
		return
	}

	// If we get a non-2xx status code, raise that via an error
	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		log.Debugf("Non-2xx status code %d for PATCH %s", resp.StatusCode, path)
		err = &ErrHTTPNotOK{resp.StatusCode}
	}

	return
}

func (c *client) delete(path string) (response []byte, err error) {
	log.Debug("Sending DELETE request to ", path)
	// Validate URL
	req, err := c.prepareRequest(http.MethodDelete, path, nil)

	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Debugf("HTTP error for DELETE %s: %v", path, err)
		return
	}

	log.Debugf("DELETE %s returned status: %d", path, resp.StatusCode)
	defer close(resp.Body)

	response, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Debugf("Error reading response body for DELETE %s: %v", path, err)
		return
	}

	// If we get a non-2xx status code, raise that via an error
	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		log.Debugf("Non-2xx status code %d for DELETE %s", resp.StatusCode, path)
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

func (c *client) groupChangeOperation(op OperationType, users []string, groupId string) error {
	if groupId == "" {
		return ErrGroupNotSpecified
	}

	if users == nil {
		return ErrUserNotSpecified
	}

	var memberList []interfaces.GroupMemberChangeMember
	for index, userId := range users {
		memberList = append(memberList, interfaces.GroupMemberChangeMember{Value: userId})

		if ((index + 1) % MaxMemberChanges) == 0 {
			gc := &interfaces.GroupMemberChange{
				Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
				Operations: []interfaces.GroupMemberChangeOperation{
					{
						Operation: string(op),
						Path:      "members",
						Members:   memberList,
					},
				},
			}
			_, err := c.patch(fmt.Sprintf("/Groups/%s", groupId), gc)
			if err != nil {
				return err
			}
			memberList = nil
		}
	}
	if memberList != nil {
		gc := &interfaces.GroupMemberChange{
			Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
			Operations: []interfaces.GroupMemberChangeOperation{
				{
					Operation: string(op),
					Path:      "members",
					Members:   memberList,
				},
			},
		}
		_, err := c.patch(fmt.Sprintf("/Groups/%s", groupId), gc)
		if err != nil {
			return err
		}
	}

	return nil
}

// AddUserToGroup will add the user specified to the group specified
func (c *client) AddUserToGroup(u *interfaces.User, g *interfaces.Group) error {
	return c.groupChangeOperation(OperationAdd, []string{u.ID}, g.ID)
}

// RemoveUserFromGroup will remove the user specified from the group specified
func (c *client) RemoveUserFromGroup(u *interfaces.User, g *interfaces.Group) error {
	return c.groupChangeOperation(OperationRemove, []string{u.ID}, g.ID)
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

// DeleteUser will remove the current user from the directory
func (c *client) DeleteUser(u *interfaces.User) error {
	if u == nil {
		return ErrUserNotSpecified
	}

	log.Debugf("Deleting user: %s (ID: %s)", u.Username, u.ID)
	_, err := c.delete(fmt.Sprintf("/Users/%s", u.ID))
	if err != nil {
		log.Debugf("Error deleting user %s: %v", u.Username, err)
		return err
	}

	log.Debugf("Successfully deleted user: %s", u.Username)
	return nil
}

// CreateGroup will create a group given
func (c *client) CreateGroup(g *interfaces.Group) (*interfaces.Group, error) {
	if g == nil {
		return nil, ErrGroupNotSpecified
	}

	log.Debugf("Creating group: %s", g.DisplayName)
	resp, err := c.post("/Groups", *g)
	if err != nil {
		log.Debugf("Error creating group %s: %v", g.DisplayName, err)
		return nil, err
	}

	var newGroup interfaces.Group
	err = json.Unmarshal(resp, &newGroup)
	if err != nil {
		log.Debugf("Error unmarshaling create group response for %s: %v", g.DisplayName, err)
		return nil, err
	}
	if newGroup.ID == "" {
		log.Debugf("Group %s created but no ID returned, finding by display name", g.DisplayName)
		return c.FindGroupByDisplayName(g.DisplayName)
	}

	log.Debugf("Successfully created group: %s (ID: %s)", g.DisplayName, newGroup.ID)
	return &newGroup, nil
}

// UpdateUser will update/replace the user specified
func (c *client) UpdateGroup(g *interfaces.Group) (*interfaces.Group, error) {

	if g == nil {
		return nil, ErrGroupNotSpecified
	}

	log.Debugf("Updating group: %s (ID: %s)", g.DisplayName, g.ID)

	gc := &interfaces.GroupChange{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		Operations: []interfaces.GroupChangeOperation{
			{
				Operation: OperationReplace,
				Path:      "displayName",
				Value:     string(g.DisplayName),
			},
			{
				Operation: OperationReplace,
				Path:      "externalId",
				Value:     string(g.ExternalId),
			},
		},
	}

	_, err := c.patch(fmt.Sprintf("/Groups/%s", g.ID), gc)
	if err != nil {
		return nil, err
	}

	log.Debugf("Successfully updated group: %s (ID: %s)", g.DisplayName, g.ID)
	return g, nil
}

// DeleteGroup will delete the group specified
func (c *client) DeleteGroup(g *interfaces.Group) error {
	if g == nil {
		return ErrGroupNotSpecified
	}

	log.Debugf("Deleting group: %s (ID: %s)", g.DisplayName, g.ID)
	_, err := c.delete(fmt.Sprintf("/Groups/%s", g.ID))
	if err != nil {
		log.Debugf("Error deleting group %s: %v", g.DisplayName, err)
		return err
	}

	log.Debugf("Successfully deleted group: %s", g.DisplayName)
	return nil
}

// add Groups in AWS
func (c *client) CreateGroups(awsGroups []*interfaces.Group, addGroups []*interfaces.Group) ([]*interfaces.Group, error) {
	log.Debug("CreateGroups()")
	awsGroupsUpdated := awsGroups

	for _, group := range addGroups {
		log.Debugf("Add group: %s (ID: %s)", group.DisplayName, group.ID)

		updatedGroup, err := c.CreateGroup(group)
		if err != nil {
			log.Error("error creating group")
			return nil, err
		}
		awsGroupsUpdated = append(awsGroupsUpdated, updatedGroup)
	}
	return awsGroupsUpdated, nil
}

// update Groups in AWS
func (c *client) UpdateGroups(awsGroups []*interfaces.Group, updateGroups []*interfaces.Group) ([]*interfaces.Group, error) {
	log.Debug("updateGroups()")
	awsGroupsUpdated := awsGroups

	for _, group := range updateGroups {
		log.Debugf("Update group: %s (ID: %s)", group.DisplayName, group.ID)

		updatedGroup, err := c.UpdateGroup(group)
		if err != nil {
			log.Error("error updating group")
			return nil, err
		}
		awsGroupsUpdated = append(awsGroupsUpdated, updatedGroup)
	}
	return awsGroupsUpdated, nil
}

// remove Groups from AWS via SCIM
func (c *client) DeleteGroups(delGroups []*interfaces.Group) error {
	log.Debug("DeleteGroups()")
	for _, group := range delGroups {
		log.Debugf("Delete group: %s (ID: %s)", group.DisplayName, group.ID)
		err := c.DeleteGroup(group)
		if err != nil {
			log.WithField("group", group).Error("error deleting user")
			return err
		}
	}
	return nil
}

// add Users in AWS
func (c *client) CreateUsers(awsUsers []*interfaces.User, addUsers []*interfaces.User) ([]*interfaces.User, error) {
	log.Debug("CreateUsers()")
	awsUsersUpdated := awsUsers

	for _, user := range addUsers {
		log.Debugf("Create user: %s (ID: %s)", user.DisplayName, user.ID)

		addedUser, err := c.CreateUser(user)
		if err != nil {
			log.WithField("user", user).Error("error creating user")
			return nil, err
		}
		awsUsersUpdated = append(awsUsersUpdated, addedUser)
	}
	return awsUsersUpdated, nil
}

// update Users in AWS
func (c *client) UpdateUsers(awsUsers []*interfaces.User, updateUsers []*interfaces.User) ([]*interfaces.User, error) {
	log.Debug("updateUsers()")
	awsUsersUpdated := awsUsers

	for _, user := range updateUsers {
		log.Debugf("Update user: %s (ID: %s)", user.DisplayName, user.ID)

		updatedUser, err := c.UpdateUser(user)
		if err != nil {
			log.WithField("user", user).Error("error updating user")
			return nil, err
		}
		awsUsersUpdated = append(awsUsersUpdated, updatedUser)
	}
	return awsUsersUpdated, nil
}

// delete Users returns nothing
func (c *client) DeleteUsers(delUsers []*interfaces.User) error {
	log.Debug("DeleteUsers()")
	for _, user := range delUsers {
		log.Debugf("Delete user: %s (ID: %s)", user.DisplayName, user.ID)
		err := c.DeleteUser(user)
		if err != nil {
			log.WithField("user", user).Error("error deleting user")
			return err
		}
	}
	return nil
}

func (c *client) AddMembers(members map[string][]string, addMembers map[string][]string) (map[string][]string, error) {
	log.Debug("AddMembers()")
	updatedMembers := members

	for groupId, memberList := range addMembers {
		log.Debugf("Adding members to group (ID: %s)", groupId)
		err := c.groupChangeOperation(OperationAdd, memberList, groupId)
		if err != nil {
			return nil, err
		}
		updatedMembers[groupId] = addMembers[groupId]
	}

	return updatedMembers, nil
}

func (c *client) RemoveMembers(removeMembers map[string][]string) error {
	log.Debug("RemoveMembers()")

	for groupId, memberList := range removeMembers {
		log.Debugf("Removing members from group (ID: %s)", groupId)
		err := c.groupChangeOperation(OperationRemove, memberList, groupId)
		if err != nil {
			return err
		}
	}

	return nil
}
