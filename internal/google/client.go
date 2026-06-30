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
	"os"
	"strconv"
	"strings"

	"ssosync/internal/config"
	"ssosync/internal/constants"
	"ssosync/internal/interfaces"

	log "github.com/sirupsen/logrus"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/impersonate"
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
	scopes := []string{
		admin.AdminDirectoryGroupReadonlyScope,
		admin.AdminDirectoryGroupMemberReadonlyScope,
		admin.AdminDirectoryUserReadonlyScope,
	}

	var ts oauth2.TokenSource

	if len(serviceAccountKey) > 0 {
		// Legacy path: a downloaded service-account key (JSON) performs
		// domain-wide delegation directly via the JWT config's Subject.
		config, err := google.JWTConfigFromJSON(serviceAccountKey, scopes...)
		if err != nil {
			return nil, err
		}
		config.Subject = adminEmail
		ts = config.TokenSource(ctx)
	} else {
		// Keyless path: no key file. Use Application Default Credentials
		// (e.g. an AWS Lambda role federated to GCP via Workload Identity
		// Federation) to impersonate the target service account, carrying
		// the Workspace admin in Subject so domain-wide delegation still
		// applies. GOOGLE_IMPERSONATE_SERVICE_ACCOUNT names the service
		// account that holds the DWD grant.
		targetSA := os.Getenv("GOOGLE_IMPERSONATE_SERVICE_ACCOUNT")
		if targetSA == "" {
			return nil, errors.New("no service-account key provided and GOOGLE_IMPERSONATE_SERVICE_ACCOUNT is not set; cannot authenticate to Google")
		}
		var err error
		ts, err = impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: targetSA,
			Scopes:          scopes,
			Subject:         adminEmail,
		})
		if err != nil {
			return nil, err
		}
	}

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

	// The Users.List projection is set to "full" so admin.User.CustomSchemas is
	// populated and custom-schema source paths (customSchemas.<Schema>.<field>)
	// are resolvable. "full" is EXPECTED to be a strict superset of the default
	// "basic" projection — every standard field the mapper reads today (name,
	// primaryEmail, suspended, organizations, and the opt-in multi-valued fields)
	// is returned identically — PENDING the live operator check in design §13;
	// so the projection change alone introduces no churn in standard-field
	// behaviour. The only extra data "full" surfaces is CustomSchemas. If a
	// future large-directory tenant hits a payload/latency problem, the
	// documented escalation is Projection("custom")+CustomFieldMask(<schema
	// names derived from the configured map>). See design §4.
	if err = c.service.Users.List().Customer(c.customerID).Projection("full").ShowDeleted("true").Pages(c.ctx, func(users *admin.Users) error {
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

	// The Users.List projection is set to "full" so admin.User.CustomSchemas is
	// populated and custom-schema source paths (customSchemas.<Schema>.<field>)
	// are resolvable. "full" is EXPECTED to be a strict superset of the default
	// "basic" projection — every standard field the mapper reads today (name,
	// primaryEmail, suspended, organizations, and the opt-in multi-valued fields)
	// is returned identically — PENDING the live operator check in design §13;
	// so the projection change alone introduces no churn in standard-field
	// behaviour. The only extra data "full" surfaces is CustomSchemas. If a
	// future large-directory tenant hits a payload/latency problem, the
	// documented escalation is Projection("custom")+CustomFieldMask(<schema
	// names derived from the configured map>). See design §4.

	// If we have wildcard then fetch all users
	if query == "*" {
		if err = c.service.Users.List().Query(filter).Customer(c.customerID).Projection("full").Pages(c.ctx, func(users *admin.Users) error {
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
			if err = c.service.Users.List().Query(subQuery+filter).Customer(c.customerID).Projection("full").Pages(c.ctx, func(users *admin.Users) error {
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
//   - Multi-valued interface{} fields (organizations, relations, websites,
//     emails, phones, addresses), resolved by a JSON round-trip into the SDK
//     typed struct and a TYPE-QUALIFIED selector (see resolveGoogleUserSource):
//     <field>[primary].<subfield> selects the entry marked primary, and
//     <field>[type=<value>].<subfield> selects the first entry whose Google
//     type == <value> (case-insensitive). In BOTH cases there is NO first-entry
//     fallback: no match -> NOTHING. e.g. organizations[type=work].title,
//     relations[type=manager].value, emails[primary].address.
//   - Custom-schema fields via admin.User.CustomSchemas (populated by the
//     Projection("full") fetch): customSchemas.<SchemaName>.<fieldName> for
//     SINGLE-VALUED custom fields. A missing custom schema or field logs a WARN
//     and resolves to NOTHING (clears the target); it does not fail the sync.
//     A multi-valued custom field is not supported: it logs a WARN and the
//     mapping is ignored (the target resolves to NOTHING).
//
// The resolver returns only scalars (string/bool); all SCIM element shaping for
// the multi-valued targets (Emails, Addresses, PhoneNumbers) is performed here
// in the mapper per design §5.5.
//
// See /Users/crpates/GitHub/ssosync/.agents/tasks/google-admin-schema-findings.md.
func mapFromGoogleUser(u *admin.User, attrMap config.AttributeMap) (interfaces.User, error) {
	user := interfaces.User{
		Schemas:    []string{constants.SCIMSchemaUser},
		ExternalId: interfaces.ExternalId{Id: u.Id},
	}

	// DEBUG-only mapping diagnostics: dump the raw fetched Google user and the
	// attribute map in effect so an operator can compare the actual source data
	// against the map. Guarded so the expensive json.Marshal and the per-entry
	// trace are NOT built when debug is off. PII (names, emails, phones,
	// addresses, custom attributes) is intentionally NOT redacted here: the whole
	// point is to compare the ACTUAL fetched values against the map. Enable debug
	// ONLY for troubleshooting (design §8).
	if log.IsLevelEnabled(log.DebugLevel) {
		fields := log.Fields{"attributeMap": attrMap, "user": u.PrimaryEmail}
		if b, err := json.Marshal(u); err == nil {
			fields["googleUser"] = string(b)
		}
		log.WithFields(fields).Debug("attribute map: mapping Google user")
		debugTraceUserEntries(u, attrMap)
	}

	for targetField, sourceAttr := range attrMap {
		switch targetField {
		case "Name.GivenName":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			user.Name.GivenName = v.(string)
		case "Name.FamilyName":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			user.Name.FamilyName = v.(string)
		case "Username":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			user.Username = v.(string)
		case "Active":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			user.Active = v.(bool)
		case "Title":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			user.Title = v.(string)
		case "ProfileURL":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			user.ProfileURL = v.(string)
		case "Enterprise.Department":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			if s := v.(string); s != "" {
				ensureEnterprise(&user).Department = s
			}
		case "Enterprise.Organization":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			if s := v.(string); s != "" {
				ensureEnterprise(&user).Organization = s
			}
		case "Enterprise.CostCenter":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			if s := v.(string); s != "" {
				ensureEnterprise(&user).CostCenter = s
			}
		case "Enterprise.Division":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			if s := v.(string); s != "" {
				ensureEnterprise(&user).Division = s
			}
		case "Enterprise.EmployeeNumber":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			if s := v.(string); s != "" {
				ensureEnterprise(&user).EmployeeNumber = s
			}
		case "Enterprise.Manager":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			if s := v.(string); s != "" {
				ensureEnterprise(&user).Manager = &interfaces.EnterpriseManager{Value: s}
			}
		case "Emails":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			if s := v.(string); s != "" {
				user.Emails = []interfaces.UserEmail{{
					Value:   s,
					Type:    scimTypeOrWork(googleEntryType(u, sourceAttr)),
					Primary: true,
				}}
			}
		case "Addresses":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
			if s := v.(string); s != "" {
				user.Addresses = []interfaces.UserAddress{{
					Formatted: s,
					Type:      scimTypeOrWork(googleEntryType(u, sourceAttr)),
					Primary:   true,
				}}
			}
		case "PhoneNumbers":
			v, found, err := resolveGoogleUserSource(u, sourceAttr)
			if err != nil {
				return interfaces.User{}, err
			}
			warnCustomSource(u, targetField, sourceAttr, found)
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

// warnCustomSource emits the custom-schema WARN that applies to one resolved
// attribute-map entry, if any: absent schema/field (found=false), or a
// multi-valued custom field (not supported; the mapping is ignored and the
// target resolves to nothing). Non-custom sources never warn here.
func warnCustomSource(u *admin.User, targetField, sourceAttr string, found bool) {
	if !found {
		warnCustomSourceAbsent(u, targetField, sourceAttr)
		return
	}
	if customFieldIsMultiValued(u, sourceAttr) {
		schema, field, _ := parseCustomSchemaPath(stripSelector(sourceAttr))
		log.WithFields(log.Fields{
			"reason": "customFieldMultiValued",
			"schema": schema,
			"field":  field,
			"target": targetField,
			"user":   customWarnUserID(u),
		}).Warn(MultiValuedCustomFieldIgnoredMsg)
	}
}

// MultiValuedCustomFieldIgnoredMsg is the WARN logged when a mapped custom-schema
// field holds multiple values on a user. Exported so the tests can match it.
const MultiValuedCustomFieldIgnoredMsg = "attribute map: custom-schema field is multi-valued; multi-valued custom attributes are not supported, mapping ignored"

// customFieldIsMultiValued reports whether a custom-schema source names a
// field present on the user as a JSON array (a Google multi-valued custom
// field). It never errors; anything it cannot read is reported as false.
func customFieldIsMultiValued(u *admin.User, sourceAttr string) bool {
	schema, field, ok := parseCustomSchemaPath(stripSelector(sourceAttr))
	if !ok {
		return false
	}
	raw, present := u.CustomSchemas[schema]
	if !present {
		return false
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return false
	}
	trimmed := strings.TrimSpace(string(obj[field]))
	return strings.HasPrefix(trimmed, "[")
}

// customWarnUserID identifies a user in custom-schema WARNs: primary email,
// else the Google id.
func customWarnUserID(u *admin.User) string {
	if u.PrimaryEmail != "" {
		return u.PrimaryEmail
	}
	return u.Id
}

// warnCustomSourceAbsent emits the structured WARN for a custom-schema source
// that resolved to found=false (an ABSENT schema or field on this user). It
// re-derives the schema-absent vs field-absent distinction by re-checking
// u.CustomSchemas[schema] presence (schema present => the FIELD was missing,
// else the SCHEMA itself was missing), so the resolver signature stays
// (interface{}, bool, error) with no extra reason plumbing (design §6.1a).
//
// A found=false signal only ever comes from a custom path, so parseCustomSchemaPath
// is expected to succeed here; if it does not (defensive), the WARN is skipped.
// A present-but-empty value is NOT this case (it returns found=true) and does
// not warn (design §6.3). This runs at WARN visibility and carries only
// identifiers (schema/field/target/user) and the reason, never source values,
// so PII values stay confined to the debug trace.
func warnCustomSourceAbsent(u *admin.User, targetField, sourceAttr string) {
	schema, field, ok := parseCustomSchemaPath(sourceAttr)
	if !ok {
		return
	}
	reason := "customFieldAbsent"
	if _, present := u.CustomSchemas[schema]; !present {
		reason = "customSchemaAbsent"
	}
	log.WithFields(log.Fields{
		"reason": reason,
		"schema": schema,
		"field":  field,
		"target": targetField,
		"user":   customWarnUserID(u),
	}).Warn("attribute map: custom-schema source not found on user; field cleared")
}

// debugTraceUserEntries emits one DEBUG line per attribute-map entry with the
// target, source, and either resolved=<value> or resolved="" reason=<why>, where
// <why> is one of noPrimaryEntry, noTypeMatch:<value>, customSchemaAbsent,
// customFieldAbsent, emptySourceValue (design §8.1). It is ONLY called from the
// debug-gated block in mapFromGoogleUser, so the work is never done when debug
// is off. It never affects resolution.
func debugTraceUserEntries(u *admin.User, attrMap config.AttributeMap) {
	for target, source := range attrMap {
		v, found, err := resolveGoogleUserSource(u, source)
		if err != nil {
			log.WithFields(log.Fields{"target": target, "source": source, "error": err.Error()}).
				Debug("attribute map: entry resolution error")
			continue
		}
		s := fmt.Sprintf("%v", v)
		if !found {
			log.WithFields(log.Fields{"target": target, "source": source, "resolved": "", "reason": debugAbsentReason(u, source)}).
				Debug("attribute map: entry resolved")
			continue
		}
		if s == "" {
			log.WithFields(log.Fields{"target": target, "source": source, "resolved": "", "reason": debugEmptyReason(u, source)}).
				Debug("attribute map: entry resolved")
			continue
		}
		log.WithFields(log.Fields{"target": target, "source": source, "resolved": s}).
			Debug("attribute map: entry resolved")
	}
}

// debugAbsentReason distinguishes customSchemaAbsent from customFieldAbsent for
// a custom source that resolved to found=false (debug trace only).
func debugAbsentReason(u *admin.User, source string) string {
	schema, _, ok := parseCustomSchemaPath(stripSelector(source))
	if !ok {
		return "customFieldAbsent"
	}
	if _, present := u.CustomSchemas[schema]; !present {
		return "customSchemaAbsent"
	}
	return "customFieldAbsent"
}

// debugEmptyReason classifies why a found=true source resolved to empty, for the
// debug trace: a primary selector that matched nothing (noPrimaryEntry), a type
// selector that matched nothing (noTypeMatch:<value>), or a present-but-empty
// value (emptySourceValue).
func debugEmptyReason(u *admin.User, source string) string {
	if customFieldIsMultiValued(u, source) {
		return "customFieldMultiValued"
	}
	if _, selector, _, ok := parseMultiValuedPath(source); ok {
		if selector == "primary" {
			return "noPrimaryEntry"
		}
		if v, has := strings.CutPrefix(selector, "type="); has {
			return "noTypeMatch:" + v
		}
	}
	return "emptySourceValue"
}

// stripSelector removes an optional trailing [selector] from a source path so
// the remainder can be parsed as a custom schema.field path.
func stripSelector(source string) string {
	if open := strings.IndexByte(source, '['); open >= 0 {
		return source[:open]
	}
	return source
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

// selectIndex generalizes primaryIndex to the type-qualified selection grammar.
// For selector "primary" it reuses primaryIndex (the first entry whose
// primary(i) is true). For selector "type=<value>" it returns the first index
// whose typeOf(i) == <value> case-insensitively. Both selectors share the
// identical -1 -> NOTHING discipline: no match (empty slice, no primary entry,
// or no type-matching entry) returns -1 and the caller resolves to nothing. This
// preserves commit 7d42885's no-first-fallback rule for the new type selector
// exactly as for primary, so the type selector inherits the same churn-stability
// guarantee (design §3.2).
//
// With multiple entries sharing the same type, "first match" is order-sensitive;
// the design (§9) documents the single-entry-per-type assumption for stable
// selection.
func selectIndex(n int, selector string, primary func(i int) bool, typeOf func(i int) string) int {
	if selector == "primary" {
		return primaryIndex(n, primary)
	}
	if v, ok := strings.CutPrefix(selector, "type="); ok {
		for i := 0; i < n; i++ {
			if strings.EqualFold(typeOf(i), v) {
				return i
			}
		}
	}
	return -1
}

// parseCustomSchemaPath implements the first-dot parse rule for a custom-schema
// source path (design §2.2). A custom source is the literal prefix
// "customSchemas." followed by <SchemaName>.<fieldName> with an OPTIONAL trailing
// [selector]; this helper returns only the schema and field (the caller strips
// any selector first, so sourceAttr here is the selector-free path).
//
//   - A path without the "customSchemas." prefix is NOT a custom path (ok=false);
//     the caller treats it as a standard path.
//   - The remainder after the prefix is split on the FIRST "." (SplitN .. 2):
//     the part before is the schema, everything after is the field. So a FIELD
//     name may contain dots (customSchemas.X.cost.center -> field "cost.center")
//     but a SCHEMA name may NOT (customSchemas.My.Schema.f -> schema "My"). This
//     asymmetry is a deliberate, documented limitation (design §2.3): Google
//     custom schema names are display-name-constrained and do not contain dots.
//   - Malformed (no field segment, empty schema, or empty field) -> ok=false.
//     The authoritative malformed check happens at load time in the validator;
//     this is the resolver-side contract.
func parseCustomSchemaPath(sourceAttr string) (schema, field string, ok bool) {
	remainder, has := strings.CutPrefix(sourceAttr, "customSchemas.")
	if !has {
		return "", "", false
	}
	parts := strings.SplitN(remainder, ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	schema, field = parts[0], parts[1]
	if schema == "" || field == "" {
		return "", "", false
	}
	return schema, field, true
}

// parseMultiValuedPath parses a standard multi-valued source path of the form
// <field>[<selector>].<subfield> (design §3.1). The selector is split on the
// FIRST "]" so a <subfield> is whatever follows "].". selector is returned
// verbatim ("primary" or "type=<value>") for selectIndex. Returns ok=false for
// any path that is not of this shape (the caller then treats it as scalar or
// custom).
//
// Addressing limitation (documented, accepted, design §3.1): because the selector
// ends at the first "]", a type VALUE containing "]" cannot be expressed. Google
// type labels do not contain "]" in practice, so there is no escaping this phase.
func parseMultiValuedPath(sourceAttr string) (field, selector, subfield string, ok bool) {
	open := strings.IndexByte(sourceAttr, '[')
	if open <= 0 {
		return "", "", "", false
	}
	close := strings.IndexByte(sourceAttr[open:], ']')
	if close < 0 {
		return "", "", "", false
	}
	close += open
	field = sourceAttr[:open]
	selector = sourceAttr[open+1 : close]
	rest := sourceAttr[close+1:]
	sub, has := strings.CutPrefix(rest, ".")
	if !has || sub == "" {
		return "", "", "", false
	}
	return field, selector, sub, true
}

// resolveGoogleUserSource reads a single supported Google admin.User source
// attribute path and returns (value, found, err):
//
//   - value is the scalar (string, or bool for the "suspended" inversion).
//   - found is true for every standard path (scalar or multi-valued), including
//     a multi-valued path that selects nothing (it resolved, to empty — NOT an
//     absence). For a custom path, found is false ONLY when the referenced
//     schema or field is ABSENT on the user; a present-but-empty custom value
//     returns found=true (design §6.1a). The mapper emits a WARN only when
//     !found (an absent custom source), not on empty resolution.
//   - err is reserved for internal-invariant breaches (malformed SDK data).
//
// The "suspended" -> Active inversion is applied here so the caller assigns a
// ready-to-use bool. Name sub-fields get the same zero-width-space -> space
// normalization getUsers applies.
//
// Multi-valued interface{} fields (organizations, relations, websites, emails,
// phones, addresses) are read via a JSON round-trip into the SDK typed struct
// (roundTripSlice) followed by the type-qualified selector grammar
// (parseMultiValuedPath + selectIndex): <field>[primary].<subfield> selects the
// entry marked primary, and <field>[type=<value>].<subfield> selects the first
// entry whose Type == <value> (case-insensitive). In BOTH cases there is NO
// first-entry fallback, so a path that selects nothing resolves to the empty
// string with found=true — this keeps run-to-run selection stable under
// clear-to-consistent + whole-set usersEqual, since Google does not guarantee
// multi-valued ordering. Element SHAPING for the Emails/Addresses/PhoneNumbers
// targets is NOT done here; this resolver only extracts the scalar, and the
// mapper builds the SCIM element (design §7.1).
//
// Custom-schema paths (customSchemas.<Schema>.<field> with an optional
// [type=<value>] selector) are resolved from u.CustomSchemas (design §2.4).
//
// An unrecognised path is an internal error (the load-time validator rejects
// unknown sources before they ever reach here).
func resolveGoogleUserSource(u *admin.User, sourceAttr string) (value interface{}, found bool, err error) {
	// Scalar fast-path.
	switch sourceAttr {
	case "name.givenName":
		if u.Name == nil {
			return "", true, nil
		}
		return strings.ReplaceAll(u.Name.GivenName, string('\u200B'), " "), true, nil
	case "name.familyName":
		if u.Name == nil {
			return "", true, nil
		}
		return strings.ReplaceAll(u.Name.FamilyName, string('\u200B'), " "), true, nil
	case "primaryEmail":
		return u.PrimaryEmail, true, nil
	case "suspended":
		// Active is the logical inverse of Google's "suspended" flag.
		return !u.Suspended, true, nil
	}

	// Custom-schema branch (design §2.4).
	if strings.HasPrefix(sourceAttr, "customSchemas.") {
		return resolveCustomSchemaSource(u, sourceAttr)
	}

	// Generic multi-valued branch (design §3.3).
	field, selector, subfield, ok := parseMultiValuedPath(sourceAttr)
	if !ok {
		return nil, false, fmt.Errorf("mapFromGoogleUser: unsupported source attribute %q (map validated at load time, this is an internal error)", sourceAttr)
	}

	switch field {
	case "organizations":
		var orgs []admin.UserOrganization
		if err := roundTripSlice(u.Organizations, &orgs); err != nil {
			return nil, false, err
		}
		i := selectIndex(len(orgs), selector,
			func(i int) bool { return orgs[i].Primary },
			func(i int) string { return orgs[i].Type })
		if i < 0 {
			return "", true, nil
		}
		switch subfield {
		case "title":
			return orgs[i].Title, true, nil
		case "department":
			return orgs[i].Department, true, nil
		case "name":
			return orgs[i].Name, true, nil
		case "costCenter":
			return orgs[i].CostCenter, true, nil
		case "domain":
			return orgs[i].Domain, true, nil
		}
	case "relations":
		var rels []admin.UserRelation
		if err := roundTripSlice(u.Relations, &rels); err != nil {
			return nil, false, err
		}
		i := selectIndex(len(rels), selector,
			func(i int) bool { return false }, // relations carry no primary flag
			func(i int) string { return rels[i].Type })
		if i < 0 {
			return "", true, nil
		}
		if subfield == "value" {
			return rels[i].Value, true, nil
		}
	case "websites":
		var sites []admin.UserWebsite
		if err := roundTripSlice(u.Websites, &sites); err != nil {
			return nil, false, err
		}
		i := selectIndex(len(sites), selector,
			func(i int) bool { return sites[i].Primary },
			func(i int) string { return sites[i].Type })
		if i < 0 {
			return "", true, nil
		}
		if subfield == "value" {
			return sites[i].Value, true, nil
		}
	case "emails":
		var emails []admin.UserEmail
		if err := roundTripSlice(u.Emails, &emails); err != nil {
			return nil, false, err
		}
		i := selectIndex(len(emails), selector,
			func(i int) bool { return emails[i].Primary },
			func(i int) string { return emails[i].Type })
		if i < 0 {
			return "", true, nil
		}
		if subfield == "address" {
			return emails[i].Address, true, nil
		}
	case "phones":
		var phones []admin.UserPhone
		if err := roundTripSlice(u.Phones, &phones); err != nil {
			return nil, false, err
		}
		i := selectIndex(len(phones), selector,
			func(i int) bool { return phones[i].Primary },
			func(i int) string { return phones[i].Type })
		if i < 0 {
			return "", true, nil
		}
		if subfield == "value" {
			return phones[i].Value, true, nil
		}
	case "addresses":
		var addrs []admin.UserAddress
		if err := roundTripSlice(u.Addresses, &addrs); err != nil {
			return nil, false, err
		}
		i := selectIndex(len(addrs), selector,
			func(i int) bool { return addrs[i].Primary },
			func(i int) string { return addrs[i].Type })
		if i < 0 {
			return "", true, nil
		}
		if subfield == "formatted" {
			return addrs[i].Formatted, true, nil
		}
	}

	return nil, false, fmt.Errorf("mapFromGoogleUser: unsupported source attribute %q (map validated at load time, this is an internal error)", sourceAttr)
}

// resolveCustomSchemaSource resolves a customSchemas.<Schema>.<field> source
// path (with an optional trailing [type=<value>] selector) from
// u.CustomSchemas, which the Projection("full") fetch populates (design §2.4).
//
// Contract:
//   - schema absent (u.CustomSchemas[schema] not present) -> ("", false, nil).
//   - field absent within a present schema              -> ("", false, nil).
//   - present scalar (string/number/bool)              -> stringified, found=true.
//   - present null / present-but-empty                 -> ("", true, nil).
//   - array (a multi-valued custom field)              -> ("", true, nil):
//     multi-valued custom attributes are NOT supported; the mapper logs a WARN
//     (warnCustomSource) and the mapping is ignored.
//   - object                                           -> ("", true, nil).
//   - a scalar with a selector                         -> ("", true, nil)
//     (selectors on custom sources are dropped at load time, see
//     config.ParseAttributeMaps, so this is defensive only).
//
// The resolver NEVER errors on a strange custom JSON shape (the custom source
// surface is open/unvalidated); only malformed RawMessage JSON that should never
// come from the SDK is an internal-invariant error. [primary] is rejected for a
// custom source at load time and never reaches here.
func resolveCustomSchemaSource(u *admin.User, sourceAttr string) (interface{}, bool, error) {
	// Separate an optional trailing selector from the schema.field path.
	path := sourceAttr
	selector := ""
	if open := strings.IndexByte(sourceAttr, '['); open >= 0 {
		if close := strings.IndexByte(sourceAttr[open:], ']'); close >= 0 {
			close += open
			selector = sourceAttr[open+1 : close]
			path = sourceAttr[:open]
		}
	}

	schema, field, ok := parseCustomSchemaPath(path)
	if !ok {
		return nil, false, fmt.Errorf("mapFromGoogleUser: malformed custom-schema source %q (map validated at load time, this is an internal error)", sourceAttr)
	}

	raw, present := u.CustomSchemas[schema]
	if !present {
		return "", false, nil // schema absent
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false, fmt.Errorf("resolveGoogleUserSource: unmarshal custom schema %q (malformed SDK data, internal error): %w", schema, err)
	}

	fieldRaw, ok := obj[field]
	if !ok {
		return "", false, nil // field absent within a present schema
	}

	return decodeCustomField(fieldRaw, selector)
}

// decodeCustomField decodes a single custom-schema field RawMessage per the JSON
// token shape (design §2.4 step 4). selector is "" (no selector) or
// "type=<value>".
func decodeCustomField(fieldRaw json.RawMessage, selector string) (interface{}, bool, error) {
	trimmed := strings.TrimSpace(string(fieldRaw))
	if trimmed == "" || trimmed == "null" {
		return "", true, nil
	}

	switch trimmed[0] {
	case '[':
		// Multi-valued custom field: not supported. Resolves to nothing; the
		// mapper logs a WARN (warnCustomSource) and the mapping is ignored.
		return "", true, nil
	case '{':
		// Object with no selector to pick a scalar -> nothing.
		return "", true, nil
	default:
		// Scalar (string/number/bool). A selector cannot apply to a scalar.
		if selector != "" {
			return "", true, nil
		}
		return jsonScalarToString(fieldRaw), true, nil
	}
}

// jsonScalarToString converts a JSON scalar RawMessage (string, number, or bool)
// to its canonical string form. A JSON string yields its unquoted value; a
// number/bool yields its literal text. Any shape that fails to decode yields the
// empty string (the custom source surface is open; this never errors).
func jsonScalarToString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return strconv.FormatBool(b)
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return ""
}

// googleEntryType returns the Google "type" of the entry selected by the SAME
// selector the resolver uses for a multi-valued source path feeding a SCIM
// multi-valued target (Emails, Addresses, PhoneNumbers). It re-runs the same
// round-trip + selection so the emitted SCIM element carries the Google entry's
// own type (design §3.3/§5.5). A malformed field, a non-multi-valued path, or a
// path with no selected entry yields the empty string, which the caller defaults
// to "work" via scimTypeOrWork. Keeping this read separate preserves the
// resolver's scalar-only value contract.
func googleEntryType(u *admin.User, sourceAttr string) string {
	field, selector, _, ok := parseMultiValuedPath(sourceAttr)
	if !ok {
		return ""
	}
	switch field {
	case "emails":
		var emails []admin.UserEmail
		if err := roundTripSlice(u.Emails, &emails); err != nil {
			return ""
		}
		i := selectIndex(len(emails), selector,
			func(i int) bool { return emails[i].Primary },
			func(i int) string { return emails[i].Type })
		if i < 0 {
			return ""
		}
		return emails[i].Type
	case "addresses":
		var addrs []admin.UserAddress
		if err := roundTripSlice(u.Addresses, &addrs); err != nil {
			return ""
		}
		i := selectIndex(len(addrs), selector,
			func(i int) bool { return addrs[i].Primary },
			func(i int) string { return addrs[i].Type })
		if i < 0 {
			return ""
		}
		return addrs[i].Type
	case "phones":
		var phones []admin.UserPhone
		if err := roundTripSlice(u.Phones, &phones); err != nil {
			return ""
		}
		i := selectIndex(len(phones), selector,
			func(i int) bool { return phones[i].Primary },
			func(i int) string { return phones[i].Type })
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

	// DEBUG-only mapping diagnostics, symmetric with mapFromGoogleUser (groups
	// are minimal: DisplayName <- name). Guarded so nothing is built when debug
	// is off (design §8).
	if log.IsLevelEnabled(log.DebugLevel) {
		fields := log.Fields{"attributeMap": attrMap, "group": g.Name}
		if b, err := json.Marshal(g); err == nil {
			fields["googleGroup"] = string(b)
		}
		log.WithFields(fields).Debug("attribute map: mapping Google group")
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
