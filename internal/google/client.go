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
	"encoding/json"
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

	// The Users.List projection is intentionally left at its default (basic).
	// The default attribute map only sources standard admin.User fields
	// (name, primaryEmail, suspended, organizations), all returned by the basic
	// projection. Custom-schema attributes (dotted "schema.field" source paths)
	// remain a FUTURE opt-in: enabling them requires promoting this call to
	// Projection("full") (or Projection("custom")+CustomFieldMask) so
	// admin.User.CustomSchemas is populated. See the mapper doc comment and
	// design §6/§7.2.
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

	// The Users.List projection is intentionally left at its default (basic).
	// The default attribute map only sources standard admin.User fields
	// (name, primaryEmail, suspended, organizations), all returned by the basic
	// projection. Custom-schema attributes (dotted "schema.field" source paths)
	// remain a FUTURE opt-in: enabling them requires promoting these calls to
	// Projection("full") (or Projection("custom")+CustomFieldMask) so
	// admin.User.CustomSchemas is populated. See the mapper doc comment and
	// design §6/§7.2.

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
// field.
//
// Resolvable source paths this phase:
//   - Scalar admin.User fields: name.givenName, name.familyName, primaryEmail,
//     suspended (inverted to Active).
//   - Multi-valued interface{} fields, resolved by a JSON round-trip into the
//     SDK typed struct with PRIMARY-entry selection (see resolveGoogleUserSource):
//     the entry marked primary, else NOTHING — there is no first-entry fallback:
//     organizations[primary].{title,department,name,costCenter,domain},
//     relations[type=manager].value, websites[primary].value,
//     emails[primary].address, phones[primary].value,
//     addresses[primary].formatted.
//
// The resolver returns only scalars (string/bool); all SCIM element shaping for
// the multi-valued targets (Emails, Addresses, PhoneNumbers) is performed here
// in the mapper per design §5.5.
//
// Deferred extension point (documented, NOT implemented this phase):
//
//	Custom-schema resolution via admin.User.CustomSchemas
//	(map[string]googleapi.RawMessage): a dotted "schema.field" key would be split
//	into schema name + field name and the RawMessage decoded. It is NOT fetched
//	today because getUsers/getDeletedUsers never set Projection("full")/
//	Projection("custom") on the Users.List calls, so admin.User.CustomSchemas is
//	nil on every fetched user. See the note at those call sites and design §6/§7.2.
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
		case "Title":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			user.Title = v.(string)
		case "ProfileURL":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			user.ProfileURL = v.(string)
		case "Enterprise.Department":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			if s := v.(string); s != "" {
				ensureEnterprise(&user).Department = s
			}
		case "Enterprise.Organization":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			if s := v.(string); s != "" {
				ensureEnterprise(&user).Organization = s
			}
		case "Enterprise.CostCenter":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			if s := v.(string); s != "" {
				ensureEnterprise(&user).CostCenter = s
			}
		case "Enterprise.Division":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			if s := v.(string); s != "" {
				ensureEnterprise(&user).Division = s
			}
		case "Enterprise.EmployeeNumber":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			if s := v.(string); s != "" {
				ensureEnterprise(&user).EmployeeNumber = s
			}
		case "Enterprise.Manager":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			if s := v.(string); s != "" {
				ensureEnterprise(&user).Manager = &interfaces.EnterpriseManager{Value: s}
			}
		case "Emails":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			if s := v.(string); s != "" {
				user.Emails = []interfaces.UserEmail{{
					Value:   s,
					Type:    scimTypeOrWork(googleEntryType(u, sourceAttr)),
					Primary: true,
				}}
			}
		case "Addresses":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			if s := v.(string); s != "" {
				user.Addresses = []interfaces.UserAddress{{
					Formatted: s,
					Type:      scimTypeOrWork(googleEntryType(u, sourceAttr)),
					Primary:   true,
				}}
			}
		case "PhoneNumbers":
			v, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			if s := v.(string); s != "" {
				user.PhoneNumbers = []interfaces.UserPhoneNumber{{
					Value:   s,
					Type:    scimTypeOrWork(googleEntryType(u, sourceAttr)),
					Primary: true,
				}}
			}
		default:
			return interfaces.User{}, fmt.Errorf("mapFromGoogleUser: unsupported target field %q (map validated at load time, this is an internal error)", targetField)
		}
	}

	// DisplayName derivation matches aws.NewUser/UpdateUser exactly and must run
	// after the name fields have been resolved from the map.
	user.DisplayName = strings.Join([]string{user.Name.GivenName, user.Name.FamilyName}, " ")

	return user, nil
}

// ensureEnterprise lazily allocates user.Enterprise and returns it, so an
// Enterprise sub-field is only materialized when a non-empty value is being
// written. A user with no enterprise-sourced data therefore keeps
// user.Enterprise == nil, matching aws.buildUser's u.Enterprise != nil gate and
// keeping usersEqual's reflect.DeepEqual(Enterprise) churn-free (design §7.1 /
// acceptance criterion 5).
func ensureEnterprise(u *interfaces.User) *interfaces.EnterpriseUser {
	if u.Enterprise == nil {
		u.Enterprise = &interfaces.EnterpriseUser{}
	}
	return u.Enterprise
}

// scimTypeOrWork returns the Google entry's own type, defaulting to "work" when
// it is empty. This matches aws.buildUser's synthesized "work" element so a
// tenant that opts into Emails/Addresses/PhoneNumbers with work data stays
// churn-free against a previously-synthesized record (design §5.5).
func scimTypeOrWork(googleType string) string {
	if googleType == "" {
		return "work"
	}
	return googleType
}

// roundTripSlice re-materializes a Google admin.User interface{} multi-valued
// field (which the SDK leaves as interface{} / []interface{} of maps) into the
// SDK typed slice T by marshaling the value to JSON and unmarshaling into *[]T.
// A nil field yields a nil slice and no error. A malformed value (should never
// happen for SDK-sourced data) returns a defensive internal error.
func roundTripSlice[T any](v interface{}, out *[]T) error {
	if v == nil {
		*out = nil
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("resolveGoogleUserSource: marshal multi-valued field (map validated at load time, this is an internal error): %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("resolveGoogleUserSource: unmarshal multi-valued field into %T (map validated at load time, this is an internal error): %w", *out, err)
	}
	return nil
}

// primaryIndex returns the index of the entry to use for a primary-selected
// multi-valued field: the first entry whose primary(i) reports true, else -1
// when no entry is primary (an empty slice OR a non-empty slice with no primary
// entry both yield -1). There is NO positional fallback: a multi-valued field
// with no primary entry resolves to nothing. This keeps run-to-run selection
// stable under clear-to-consistent + whole-set usersEqual, since Google does not
// guarantee multi-valued ordering.
func primaryIndex(n int, primary func(i int) bool) int {
	for i := 0; i < n; i++ {
		if primary(i) {
			return i
		}
	}
	return -1
}

// resolveGoogleUserSource reads a single supported Google admin.User source
// attribute path and returns its scalar value (string, or bool for the
// "suspended" inversion). The "suspended" -> Active inversion is applied here so
// the caller assigns a ready-to-use bool. Name sub-fields get the same
// zero-width-space -> space normalization getUsers applies.
//
// Multi-valued interface{} fields (organizations, relations, websites, emails,
// phones, addresses) are read via a JSON round-trip into the SDK typed struct
// (roundTripSlice) followed by PRIMARY-entry selection (primaryIndex): the entry
// marked primary, else NOTHING (empty string). There is no first-entry fallback,
// so a multi-valued field with no primary entry resolves to the zero value —
// this keeps run-to-run selection stable under clear-to-consistent + whole-set
// usersEqual, since Google does not guarantee multi-valued ordering.
// relations[type=manager] instead selects the first entry whose Type ==
// "manager" (case-insensitive), else the empty string. Element SHAPING for the
// Emails/Addresses/PhoneNumbers targets is NOT done here; this resolver only
// extracts the scalar, and the mapper builds the SCIM element (design §7.1).
//
// An unrecognised path is an internal error (the load-time validator rejects
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

	case "organizations[primary].title",
		"organizations[primary].department",
		"organizations[primary].name",
		"organizations[primary].costCenter",
		"organizations[primary].domain":
		var orgs []admin.UserOrganization
		if err := roundTripSlice(u.Organizations, &orgs); err != nil {
			return nil, err
		}
		i := primaryIndex(len(orgs), func(i int) bool { return orgs[i].Primary })
		if i < 0 {
			return "", nil
		}
		switch sourceAttr {
		case "organizations[primary].title":
			return orgs[i].Title, nil
		case "organizations[primary].department":
			return orgs[i].Department, nil
		case "organizations[primary].name":
			return orgs[i].Name, nil
		case "organizations[primary].costCenter":
			return orgs[i].CostCenter, nil
		default: // organizations[primary].domain
			return orgs[i].Domain, nil
		}

	case "relations[type=manager].value":
		var rels []admin.UserRelation
		if err := roundTripSlice(u.Relations, &rels); err != nil {
			return nil, err
		}
		for _, r := range rels {
			if strings.EqualFold(r.Type, "manager") {
				return r.Value, nil
			}
		}
		return "", nil

	case "websites[primary].value":
		var sites []admin.UserWebsite
		if err := roundTripSlice(u.Websites, &sites); err != nil {
			return nil, err
		}
		i := primaryIndex(len(sites), func(i int) bool { return sites[i].Primary })
		if i < 0 {
			return "", nil
		}
		return sites[i].Value, nil

	case "emails[primary].address":
		var emails []admin.UserEmail
		if err := roundTripSlice(u.Emails, &emails); err != nil {
			return nil, err
		}
		i := primaryIndex(len(emails), func(i int) bool { return emails[i].Primary })
		if i < 0 {
			return "", nil
		}
		return emails[i].Address, nil

	case "phones[primary].value":
		var phones []admin.UserPhone
		if err := roundTripSlice(u.Phones, &phones); err != nil {
			return nil, err
		}
		i := primaryIndex(len(phones), func(i int) bool { return phones[i].Primary })
		if i < 0 {
			return "", nil
		}
		return phones[i].Value, nil

	case "addresses[primary].formatted":
		var addrs []admin.UserAddress
		if err := roundTripSlice(u.Addresses, &addrs); err != nil {
			return nil, err
		}
		i := primaryIndex(len(addrs), func(i int) bool { return addrs[i].Primary })
		if i < 0 {
			return "", nil
		}
		return addrs[i].Formatted, nil

	default:
		return nil, fmt.Errorf("mapFromGoogleUser: unsupported source attribute %q (map validated at load time, this is an internal error)", sourceAttr)
	}
}

// googleEntryType returns the Google "type" of the primary-selected entry for a
// multi-valued source path that feeds a SCIM multi-valued target (Emails,
// Addresses, PhoneNumbers). It re-runs the same round-trip + primary-selection
// the resolver uses so the emitted SCIM element carries the Google entry's own
// type (design §5.5). A malformed field or a path with no type yields the empty
// string, which the caller defaults to "work" via scimTypeOrWork. Keeping this
// read separate preserves the resolver's scalar-only contract.
func googleEntryType(u *admin.User, sourceAttr string) string {
	switch sourceAttr {
	case "emails[primary].address":
		var emails []admin.UserEmail
		if err := roundTripSlice(u.Emails, &emails); err != nil {
			return ""
		}
		i := primaryIndex(len(emails), func(i int) bool { return emails[i].Primary })
		if i < 0 {
			return ""
		}
		return emails[i].Type
	case "addresses[primary].formatted":
		var addrs []admin.UserAddress
		if err := roundTripSlice(u.Addresses, &addrs); err != nil {
			return ""
		}
		i := primaryIndex(len(addrs), func(i int) bool { return addrs[i].Primary })
		if i < 0 {
			return ""
		}
		return addrs[i].Type
	case "phones[primary].value":
		var phones []admin.UserPhone
		if err := roundTripSlice(u.Phones, &phones); err != nil {
			return ""
		}
		i := primaryIndex(len(phones), func(i int) bool { return phones[i].Primary })
		if i < 0 {
			return ""
		}
		return phones[i].Type
	default:
		return ""
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
