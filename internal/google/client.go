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
	"fmt"
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

// mapFromGoogleUser builds an interfaces.User from a Google admin.User, DRIVEN by
// the supplied attribute map. For every entry it resolves the Google SOURCE
// attribute (the map VALUE) from the admin.User and assigns it to the interface
// TARGET field (the map KEY). The map is the single description of which source
// feeds which target; this function only knows how to read each supported source
// path and write each supported target field.
//
// With config.DefaultUserAttributeMap the output matches the live groups-sync
// construction (aws.NewUser) for the covered fields:
//
//	Name.GivenName  <- name.givenName
//	Name.FamilyName <- name.familyName
//	Username        <- primaryEmail
//	Active          <- suspended   (logically inverted: Active == !suspended)
//
// The "suspended" -> Active inversion is applied while RESOLVING that entry
// (Active = !u.Suspended), not stored in the map.
//
// Structural fields that are identity, not configurable attributes, are set
// regardless of the map, exactly as the live path does:
//   - Schemas = []string{constants.SCIMSchemaUser}
//   - ExternalId.Id = u.Id (the Google user id the live getUserOperation feeds
//     into aws.NewUser/UpdateUser as the external id)
//   - DisplayName = "GivenName FamilyName" (the derivation aws.NewUser applies),
//     computed AFTER the map has populated the name fields.
//
// The zero-width-space (\u200B) -> space normalization applied to GivenName and
// FamilyName matches getUsers above, so a mapper handed an un-normalized
// admin.User produces the same values the live path emits after getUsers has
// normalized it.
//
// The attribute map is validated at load time (config.ParseAttributeMaps), so by
// the time it reaches here every entry is a known target<-source pair. A
// genuinely-unexpected unresolved entry is therefore an internal invariant
// breach, reported via the returned error rather than silently dropping the
// field. Standard source paths only this phase.
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
func mapFromGoogleUser(u *admin.User, attrMap config.AttributeMap) (interfaces.User, error) {
	user := interfaces.User{
		Schemas:    []string{constants.SCIMSchemaUser},
		ExternalId: interfaces.ExternalId{Id: u.Id},
	}

	for targetField, sourceAttr := range attrMap {
		switch targetField {
		case "Name.GivenName":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			user.Name.GivenName = v.(string)
		case "Name.FamilyName":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			user.Name.FamilyName = v.(string)
		case "Username":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			user.Username = v.(string)
		case "Active":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			user.Active = v.(bool)
		default:
			return interfaces.User{}, fmt.Errorf("mapFromGoogleUser: unsupported target field %q (map validated at load time, this is an internal error)", targetField)
		}
	}

	// DisplayName derivation matches aws.NewUser/UpdateUser exactly and must run
	// after the name fields have been resolved from the map.
	user.DisplayName = strings.Join([]string{user.Name.GivenName, user.Name.FamilyName}, " ")

	return user, nil
}

// resolveGoogleUserSource reads a single supported Google admin.User source
// attribute path and returns its value. The "suspended" -> Active inversion is
// applied here so the caller assigns a ready-to-use bool. Name sub-fields get
// the same zero-width-space -> space normalization getUsers applies. An
// unrecognised path is an internal error (the load-time validator rejects
// unknown sources before they ever reach here).
func resolveGoogleUserSource(u *admin.User, sourceAttr string) (interface{}, error) {
	switch sourceAttr {
	case "name.givenName":
		if u.Name == nil {
			return "", nil
		}
		return strings.ReplaceAll(u.Name.GivenName, string('\u200B'), " "), nil
	case "name.familyName":
		if u.Name == nil {
			return "", nil
		}
		return strings.ReplaceAll(u.Name.FamilyName, string('\u200B'), " "), nil
	case "primaryEmail":
		return u.PrimaryEmail, nil
	case "suspended":
		// Active is the logical inverse of Google's "suspended" flag.
		return !u.Suspended, nil
	default:
		return nil, fmt.Errorf("mapFromGoogleUser: unsupported source attribute %q (map validated at load time, this is an internal error)", sourceAttr)
	}
}

// mapFromGoogleGroup builds an interfaces.Group from a Google admin.Group, DRIVEN
// by the supplied attribute map, in the same resolve-source-assign-target style
// as mapFromGoogleUser.
//
// With config.DefaultGroupAttributeMap, DisplayName <- name. This is
// behaviour-preserving: the live groups-sync path (getGroupOperation) builds its
// desired group state with aws.NewGroup(googleGroup.Name, ...) / aws.UpdateGroup
// (..., googleGroup.Name, ...), i.e. it uses admin.Group.Name for DisplayName.
// So the default map reproduces the live value exactly; there is no divergence.
//
// Structural identity fields are set regardless of the map, as the live path
// does: Schemas = []string{constants.SCIMSchemaGroup} and ExternalId.Id = g.Id
// (the Google group id the live getGroupOperation feeds into aws.NewGroup/
// UpdateGroup as the external id). The same \u200B -> space normalization used
// for user names is applied to DisplayName for symmetry.
//
// The map is validated at load time, so every entry is a known target<-source
// pair here; an unexpected unresolved entry is an internal invariant breach and
// is returned as an error rather than silently dropped.
//
// admin.Group has NO custom-schema field (a Google Directory API limitation), so
// the group map is limited to the fixed admin.Group fields; there is no
// custom-schema extension point for groups. See
// /Users/crpates/GitHub/ssosync/.agents/tasks/google-admin-schema-findings.md.
func mapFromGoogleGroup(g *admin.Group, attrMap config.AttributeMap) (interfaces.Group, error) {
	group := interfaces.Group{
		Schemas:    []string{constants.SCIMSchemaGroup},
		ExternalId: interfaces.ExternalId{Id: g.Id},
	}

	for targetField, sourceAttr := range attrMap {
		switch targetField {
		case "DisplayName":
			v, err := resolveGoogleGroupSource(g, sourceAttr)
			if err != nil {
				return interfaces.Group{}, err
			}
			group.DisplayName = v
		default:
			return interfaces.Group{}, fmt.Errorf("mapFromGoogleGroup: unsupported target field %q (map validated at load time, this is an internal error)", targetField)
		}
	}

	return group, nil
}

// resolveGoogleGroupSource reads a single supported Google admin.Group source
// attribute path and returns its value, with the same zero-width-space -> space
// normalization applied to user names. An unrecognised path is an internal error
// (the load-time validator rejects unknown sources before they reach here).
func resolveGoogleGroupSource(g *admin.Group, sourceAttr string) (string, error) {
	switch sourceAttr {
	case "name":
		return strings.ReplaceAll(g.Name, string('\u200B'), " "), nil
	default:
		return "", fmt.Errorf("mapFromGoogleGroup: unsupported source attribute %q (map validated at load time, this is an internal error)", sourceAttr)
	}
}

// MapFromGoogleUser is the exported entry point to the attribute-map-driven
// Google admin.User -> interfaces.User mapper, for callers outside this package
// (the groups sync path in internal/sync.go). It delegates to mapFromGoogleUser.
func MapFromGoogleUser(u *admin.User, attrMap config.AttributeMap) (interfaces.User, error) {
	return mapFromGoogleUser(u, attrMap)
}

// MapFromGoogleGroup is the exported entry point to the attribute-map-driven
// Google admin.Group -> interfaces.Group mapper, for callers outside this
// package (the groups sync path in internal/sync.go). It delegates to
// mapFromGoogleGroup.
func MapFromGoogleGroup(g *admin.Group, attrMap config.AttributeMap) (interfaces.Group, error) {
	return mapFromGoogleGroup(g, attrMap)
}
