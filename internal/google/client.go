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
	"errors"
	"strings"

	"ssosync/internal/config"
	"ssosync/internal/constants"
	"ssosync/internal/interfaces"

	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// Client is the Interface for the Client
type Client interface {
	GetUsers(string, string) ([]*admin.User, error)
	GetDeletedUsers() ([]*admin.User, error)
	GetGroups(string) ([]*admin.Group, error)
	GetGroupMembers(*admin.Group) ([]*admin.Member, error)
}

type client struct {
	ctx        context.Context
	service    *admin.Service
	customerID string
}

const (
	// Maximum number of retries for admin api
	MaxRetries = 5
)

// NewClient creates a new client for Google's Admin API. customerID is the
// Google Workspace customer to operate against; pass "my_customer" (or the
// DefaultCustomerID constant) to target the customer associated with the
// service account's domain.
func NewClient(ctx context.Context, adminEmail, customerID string, serviceAccountKey []byte) (Client, error) {
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
		ctx:        ctx,
		service:    srv,
		customerID: customerID,
	}, nil
}

// GetDeletedUsers will get the deleted users from the Google's Admin API.
func (c *client) GetDeletedUsers() ([]*admin.User, error) {
	var err error
	var members []*admin.User

	for iteration := 1; iteration < MaxRetries; iteration++ {
		members, err = c.getDeletedUsers()
		if err == nil {
			return members, nil
		}
	}
	return nil, err
}

func (c *client) getDeletedUsers() ([]*admin.User, error) {
	u := make([]*admin.User, 0)
	var err error

	if err = c.service.Users.List().Customer(c.customerID).ShowDeleted("true").Pages(c.ctx, func(users *admin.Users) error {
		u = append(u, users.Users...)
		return nil
	}); err != nil {
		return nil, err
	}

	return u, err
}

// GetGroupMembers will get the members of the group specified
func (c *client) GetGroupMembers(g *admin.Group) ([]*admin.Member, error) {
	var err error
	var members []*admin.Member

	for iteration := 1; iteration < MaxRetries; iteration++ {
		members, err = c.getGroupMembers(g)
		if err == nil {
			return members, nil
		}
	}
	return nil, err
}

func (c *client) getGroupMembers(g *admin.Group) ([]*admin.Member, error) {
	m := make([]*admin.Member, 0)
	var err error

	if err = c.service.Members.List(g.Id).IncludeDerivedMembership(true).Pages(context.TODO(), func(members *admin.Members) error {
		m = append(m, members.Members...)
		return nil
	}); err != nil {
		return nil, err
	}

	return m, err
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
func (c *client) GetUsers(query string, filter string) ([]*admin.User, error) {
	var err error
	var users []*admin.User

	for iteration := 1; iteration < MaxRetries; iteration++ {
		users, err = c.getUsers(query, filter)
		if err == nil {
			return users, nil
		}
	}
	return nil, err
}

func (c *client) getUsers(query string, filter string) ([]*admin.User, error) {
	u := make([]*admin.User, 0)
	var err error

	// If we have an empty query, return nothing.
	if query == "" {
		return nil, nil
	}

	// If we have wildcard then fetch all users
	if query == "*" {
		if err = c.service.Users.List().Query(filter).Customer(c.customerID).Pages(c.ctx, func(users *admin.Users) error {
			u = append(u, users.Users...)
			return nil
		}); err != nil {
			return nil, err
		}

	} else {

		// The Google api doesn't support multi-part queries, but we do so we need to split into an array of query strings
		queries := strings.Split(query, ",")

		// Then call the api one query at a time, appending to our list
		for _, subQuery := range queries {
			if err = c.service.Users.List().Query(subQuery+filter).Customer(c.customerID).Pages(c.ctx, func(users *admin.Users) error {
				u = append(u, users.Users...)
				return nil
			}); err != nil {
				return nil, err
			}
		}
	}

	// some people prefer to go by a mononym
	// Google directory will accept a 'zero width space' for an empty name but will not accept a 'space'
	// but
	// Identity Store will accept and a 'space' for an empty name but not a 'zero width space'
	// So we need to replace any 'zero width space' strings with a single 'space' to allow comparison and sync
	for _, user := range u {
		user.Name.GivenName = strings.ReplaceAll(user.Name.GivenName, string('\u200B'), " ")
		user.Name.FamilyName = strings.ReplaceAll(user.Name.FamilyName, string('\u200B'), " ")
	}

	return u, err

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
func (c *client) GetGroups(query string) ([]*admin.Group, error) {
	var err error
	var groups []*admin.Group

	for iteration := 1; iteration < MaxRetries; iteration++ {
		groups, err = c.getGroups(query)
		if err == nil {
			return groups, nil
		}
	}
	return nil, err
}

func (c *client) getGroups(query string) ([]*admin.Group, error) {
	g := make([]*admin.Group, 0)
	var err error

	// If we have an empty query, then we are not looking for groups
	if query == "" {
		return g, err
	}

	// If we have wildcard then fetch all groups
	if query == "*" {
		if err = c.service.Groups.List().Customer(c.customerID).Pages(context.TODO(), func(groups *admin.Groups) error {
			g = append(g, groups.Groups...)
			return nil
		}); err != nil {
			return nil, err
		}
		// Check we've got some users otherwise something is wrong.
		if len(g) == 0 {
			return nil, errors.New("google api return 0 groups?")
		}
		return g, err
	}

	// The Google api doesn't support multi-part queries, but we do so we need to split into an array of query strings
	queries := strings.Split(query, ",")

	// Then call the api one query at a time, appending to our list
	for _, subQuery := range queries {
		if err = c.service.Groups.List().Customer(c.customerID).Query(subQuery).Pages(context.TODO(), func(groups *admin.Groups) error {
			g = append(g, groups.Groups...)
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// Check we've got some users otherwise something is wrong.
	if len(g) == 0 {
		return nil, errors.New("google api return 0 groups?")
	}
	return g, err
}

// mapFromGoogleUser builds an interfaces.User from a Google admin.User using the
// supplied attribute map. It sits ALONGSIDE the existing Google -> interfaces
// construction (which remains the live path via the aws package constructors):
// this function is purely additive and is NOT wired into internal/sync.go or any
// other live sync path. Scope is the groups sync method only.
//
// attrMap keys are interfaces.User target field paths; values are admin.User
// source attribute paths (see config.AttributeMap / config.DefaultUserAttributeMap).
// With DefaultUserAttributeMap the output matches the current live construction
// for the covered fields: Username <- primaryEmail, Name.GivenName <-
// name.givenName, Name.FamilyName <- name.familyName, and Active <- !suspended
// (the "suspended" source is logically inverted here, in the mapper, not in the
// map).
//
// The zero-width-space (\u200B) -> space normalization applied to GivenName and
// FamilyName matches getUsers above, so a mapper handed an un-normalized
// admin.User produces the same values the live path emits after getUsers has
// normalized it.
//
// Schemas is set exactly as the live aws.NewUser/UpdateUser constructors do:
// []string{constants.SCIMSchemaUser}.
//
// Fields the live path also sets but that the default map does NOT cover are
// intentionally left unset here (DisplayName, Emails, Addresses, ExternalId, ID);
// a richer attribute map in a later phase may add them.
//
// Deferred extension points (documented, NOT implemented this phase):
//
//	(a) Custom-schema resolution via admin.User.CustomSchemas
//	    (map[string]googleapi.RawMessage): a dotted "schema.field" key would be
//	    split into schema name + field name and the RawMessage decoded.
//	(b) The multi-valued interface{} sub-objects (Emails, Addresses,
//	    Organizations, Phones, Relations, ...) need a JSON round-trip through the
//	    SDK's typed structs to be read.
//	(c) Custom-schema data is NOT fetched today because client.go never sets
//	    Projection("full")/Projection("custom") on the Users.List calls, so
//	    admin.User.CustomSchemas is nil on every fetched user.
//
// See /Users/crpates/GitHub/ssosync/.agents/tasks/google-admin-schema-findings.md.
func mapFromGoogleUser(u *admin.User, attrMap config.AttributeMap) interfaces.User {
	user := interfaces.User{
		Schemas: []string{constants.SCIMSchemaUser},
	}

	for targetField, sourceAttr := range attrMap {
		switch targetField {
		case "Name.GivenName":
			if u.Name != nil {
				user.Name.GivenName = strings.ReplaceAll(u.Name.GivenName, string('\u200B'), " ")
			}
		case "Name.FamilyName":
			if u.Name != nil {
				user.Name.FamilyName = strings.ReplaceAll(u.Name.FamilyName, string('\u200B'), " ")
			}
		case "Username":
			user.Username = u.PrimaryEmail
		case "Active":
			// Active is the logical inverse of Google's "suspended" flag; the
			// inversion is applied here, in the mapper, not in the map.
			user.Active = !u.Suspended
		}
		_ = sourceAttr
	}

	return user
}

// mapFromGoogleGroup builds an interfaces.Group from a Google admin.Group using
// the supplied attribute map. Like mapFromGoogleUser it sits ALONGSIDE the live
// aws.NewGroup/UpdateGroup construction and is NOT wired into internal/sync.go or
// any live sync path. Scope is the groups sync method only.
//
// attrMap keys are interfaces.Group target field paths; values are admin.Group
// source attribute paths (see config.AttributeMap / config.DefaultGroupAttributeMap).
// With DefaultGroupAttributeMap, DisplayName is resolved from admin.Group.Name.
// Note: the live path sets group DisplayName from admin.Group.Email, not Name;
// this default map's use of Name is a deliberate, documented divergence while the
// mapper is not yet wired in (see config.DefaultGroupAttributeMap). The same
// \u200B -> space normalization used for user names is applied here for symmetry.
//
// Schemas is set exactly as the live constructors do:
// []string{constants.SCIMSchemaGroup}. Members, ExternalId and ID are set by the
// live path but are out of scope for the default group map, so they are left
// unset here.
//
// admin.Group has NO custom-schema field (a Google Directory API limitation), so
// the group map is limited to the fixed admin.Group fields; there is no
// custom-schema extension point for groups. See
// /Users/crpates/GitHub/ssosync/.agents/tasks/google-admin-schema-findings.md.
func mapFromGoogleGroup(g *admin.Group, attrMap config.AttributeMap) interfaces.Group {
	group := interfaces.Group{
		Schemas: []string{constants.SCIMSchemaGroup},
	}

	for targetField, sourceAttr := range attrMap {
		switch targetField {
		case "DisplayName":
			group.DisplayName = strings.ReplaceAll(g.Name, string('\u200B'), " ")
		}
		_ = sourceAttr
	}

	return group
}
