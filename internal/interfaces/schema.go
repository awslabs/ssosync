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

package interfaces

import "encoding/json"

// ExternalId represents an external identifier for a user or group. It carries
// both the identifier value and the issuer that assigned it.
//
// On the SCIM wire the external identifier is a bare JSON string, so ExternalId
// marshals to (and unmarshals from) that bare string via its Id field. The
// Issuer field is only populated when consuming the Identity Store SDK, which
// exposes the issuer alongside the id.
type ExternalId struct {
	Id     string `json:"id"`
	Issuer string `json:"issuer"`
}

// MarshalJSON renders ExternalId as a bare JSON string equal to its Id so the
// SCIM wire format (externalId as a plain string) is preserved.
func (e ExternalId) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.Id)
}

// UnmarshalJSON reads a bare JSON string into Id, leaving Issuer empty. This
// matches the SCIM wire format where externalId is a plain string.
func (e *ExternalId) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	e.Id = s
	return nil
}

// Group represents a Group in AWS SSO.
//
// The supported attribute set is taken from the AWS IAM Identity Center SCIM
// Implementation Developer Guide (CreateGroup:
// https://docs.aws.amazon.com/singlesignon/latest/developerguide/creategroup.html).
// AWS IAM Identity Center's SCIM CreateGroup surface accepts only externalId,
// displayName and members. No additional group attributes (for example a group
// description) are accepted on the SCIM wire, even though the Identity Store SDK
// types.Group exposes a Description field. For that reason no new top-level
// fields are added to Group: its schema is already complete for the SCIM surface.
// This omission is intentional and documented, not an oversight.
type Group struct {
	ID          string     `json:"id,omitempty"`
	Schemas     []string   `json:"schemas"`
	DisplayName string     `json:"displayName"`
	ExternalId  ExternalId `json:"externalId,omitempty"`
	Members     []string   `json:"members"`
	// ExternalIds holds every ExternalId the Identity Store returned for the
	// group (a record can carry more than one). ExternalId is the one used for
	// matching. Never sent on the SCIM wire.
	ExternalIds []ExternalId `json:"-"`
}

// MarshalJSON preserves the SCIM wire format for the externalId field: when the
// external id is empty the key is omitted entirely (matching the previous
// string field's omitempty behavior), otherwise it is emitted as a bare JSON
// string via ExternalId.MarshalJSON.
func (g Group) MarshalJSON() ([]byte, error) {
	type alias Group
	if g.ExternalId.Id == "" {
		return json.Marshal(struct {
			alias
			ExternalId *ExternalId `json:"externalId,omitempty"`
		}{alias: alias(g)})
	}
	return json.Marshal(alias(g))
}

// GroupFilterResults represents filtered results when we search for
// groups or List all groups
type GroupFilterResults struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	ItemsPerPage int      `json:"itemsPerPage"`
	StartIndex   int      `json:"startIndex"`
	Resources    []Group  `json:"Resources"`
}

// GroupChangeOperation details the operation to take place on a group
type GroupChangeOperation struct {
	Operation string `json:"op"`
	Path      string `json:"path,omitempty"`
	Value     string `json:"value"`
}

// GroupChange represents a change operation
// for a group
type GroupChange struct {
	Schemas    []string               `json:"schemas"`
	Operations []GroupChangeOperation `json:"Operations"`
}

// GroupMemberChangeMember is a value needed for the ID of the member
// to add/remove
type GroupMemberChangeMember struct {
	Value string `json:"value"`
}

// GroupMemberChangeOperation details the operation to take place on a group
type GroupMemberChangeOperation struct {
	Operation string                    `json:"op"`
	Path      string                    `json:"path"`
	Members   []GroupMemberChangeMember `json:"value"`
}

// GroupMemberChange represents a change operation
// for a group
type GroupMemberChange struct {
	Schemas    []string                     `json:"schemas"`
	Operations []GroupMemberChangeOperation `json:"Operations"`
}

// UserEmail represents a user email address
type UserEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type"`
	Primary bool   `json:"primary"`
}

// UserName maps to the SCIM "name" complex attribute of a user.
//
// FamilyName and GivenName are always emitted (no omitempty) to preserve the
// existing minimal-create wire output. The remaining sub-fields are optional and
// are omitted from the SCIM wire when empty. All fields map to the correspondingly
// named SCIM "name" sub-attributes per the AWS IAM Identity Center SCIM
// Implementation Developer Guide (CreateUser).
type UserName struct {
	// FamilyName maps to SCIM name.familyName. Always emitted.
	FamilyName string `json:"familyName"`
	// GivenName maps to SCIM name.givenName. Always emitted.
	GivenName string `json:"givenName"`
	// Formatted is optional and maps to SCIM name.formatted.
	Formatted string `json:"formatted,omitempty"`
	// MiddleName is optional and maps to SCIM name.middleName.
	MiddleName string `json:"middleName,omitempty"`
	// HonorificPrefix is optional and maps to SCIM name.honorificPrefix.
	HonorificPrefix string `json:"honorificPrefix,omitempty"`
	// HonorificSuffix is optional and maps to SCIM name.honorificSuffix.
	HonorificSuffix string `json:"honorificSuffix,omitempty"`
}

// UserAddress maps to a single value of the SCIM "addresses" multi-valued
// complex attribute of a user.
//
// Type is always emitted (no omitempty) to preserve the existing minimal-create
// wire output, where ssosync emits a single {"type":"work"} address. All other
// fields are optional and omitted from the SCIM wire when empty. Fields map to
// the correspondingly named SCIM "addresses" sub-attributes per the AWS IAM
// Identity Center SCIM Implementation Developer Guide (CreateUser).
type UserAddress struct {
	// Type maps to SCIM addresses.type. Always emitted.
	Type string `json:"type"`
	// Formatted is optional and maps to SCIM addresses.formatted.
	Formatted string `json:"formatted,omitempty"`
	// StreetAddress is optional and maps to SCIM addresses.streetAddress.
	StreetAddress string `json:"streetAddress,omitempty"`
	// Locality is optional and maps to SCIM addresses.locality.
	Locality string `json:"locality,omitempty"`
	// Region is optional and maps to SCIM addresses.region.
	Region string `json:"region,omitempty"`
	// PostalCode is optional and maps to SCIM addresses.postalCode.
	PostalCode string `json:"postalCode,omitempty"`
	// Country is optional and maps to SCIM addresses.country.
	Country string `json:"country,omitempty"`
	// Primary is optional and maps to SCIM addresses.primary.
	Primary bool `json:"primary,omitempty"`
}

// UserPhoneNumber maps to a single value of the SCIM "phoneNumbers"
// multi-valued complex attribute of a user. All fields are optional and omitted
// from the SCIM wire when empty. Per the AWS IAM Identity Center SCIM
// Implementation Developer Guide (CreateUser) only value, type and primary are
// supported.
type UserPhoneNumber struct {
	// Value is optional and maps to SCIM phoneNumbers.value.
	Value string `json:"value,omitempty"`
	// Type is optional and maps to SCIM phoneNumbers.type.
	Type string `json:"type,omitempty"`
	// Primary is optional and maps to SCIM phoneNumbers.primary.
	Primary bool `json:"primary,omitempty"`
}

// UserRole maps to a single value of the SCIM "roles" multi-valued complex
// attribute of a user. All fields are optional and omitted from the SCIM wire
// when empty. Per the AWS IAM Identity Center SCIM Implementation Developer
// Guide (CreateUser) only value, type and primary are supported (the SCIM
// "display" sub-attribute is not accepted).
type UserRole struct {
	// Value is optional and maps to SCIM roles.value.
	Value string `json:"value,omitempty"`
	// Type is optional and maps to SCIM roles.type.
	Type string `json:"type,omitempty"`
	// Primary is optional and maps to SCIM roles.primary.
	Primary bool `json:"primary,omitempty"`
}

// EnterpriseManager maps to the "manager" sub-attribute of the SCIM Enterprise
// User extension. Per the AWS IAM Identity Center SCIM Implementation Developer
// Guide (CreateUser) only manager.value is supported (manager.displayName and
// manager.$ref are not accepted). Value is optional and omitted when empty.
type EnterpriseManager struct {
	// Value is optional and maps to the enterprise extension manager.value.
	Value string `json:"value,omitempty"`
}

// EnterpriseUser maps to the SCIM Enterprise User extension object
// (urn:ietf:params:scim:schemas:extension:enterprise:2.0:User). All fields are
// optional and omitted from the SCIM wire when empty; the whole object is
// omitted when the containing User.Enterprise pointer is nil. Supported fields
// are taken from the AWS IAM Identity Center SCIM Implementation Developer Guide
// (CreateUser).
type EnterpriseUser struct {
	// EmployeeNumber is optional and maps to the enterprise extension employeeNumber.
	EmployeeNumber string `json:"employeeNumber,omitempty"`
	// CostCenter is optional and maps to the enterprise extension costCenter.
	CostCenter string `json:"costCenter,omitempty"`
	// Organization is optional and maps to the enterprise extension organization.
	Organization string `json:"organization,omitempty"`
	// Division is optional and maps to the enterprise extension division.
	Division string `json:"division,omitempty"`
	// Department is optional and maps to the enterprise extension department.
	Department string `json:"department,omitempty"`
	// Manager is optional and maps to the enterprise extension manager object.
	Manager *EnterpriseManager `json:"manager,omitempty"`
}

// User represents a User in AWS SSO.
//
// The supported attribute set is taken from the AWS IAM Identity Center SCIM
// Implementation Developer Guide (CreateUser:
// https://docs.aws.amazon.com/singlesignon/latest/developerguide/createuser.html),
// cross-checked against CreateGroup, the SCIM limitations page and the
// aws-sdk-go-v2 service/identitystore types.
//
// The existing fields (Schemas, ExternalId, Username, Name, DisplayName, Active,
// Emails, Addresses, ID) keep their exact JSON tags so a minimally-created user
// (name + primary work email + work address + active) serializes byte-for-byte
// as it did before the schema was expanded. Every newly added top-level field is
// optional: it uses omitempty (and a pointer/slice type where relevant) so an
// unset value is omitted entirely from the SCIM wire.
type User struct {
	ID          string        `json:"id,omitempty"`
	Schemas     []string      `json:"schemas"`
	ExternalId  ExternalId    `json:"externalId,omitempty"`
	Username    string        `json:"userName"`
	Name        UserName      `json:"name"`
	DisplayName string        `json:"displayName"`
	Active      bool          `json:"active"`
	Emails      []UserEmail   `json:"emails"`
	Addresses   []UserAddress `json:"addresses"`
	// ExternalIds holds every ExternalId the Identity Store returned for the
	// user (a record can carry more than one). ExternalId is the one used for
	// matching. Never sent on the SCIM wire.
	ExternalIds []ExternalId `json:"-"`

	// NickName is optional and maps to the SCIM nickName attribute.
	NickName string `json:"nickName,omitempty"`
	// ProfileURL is optional and maps to the SCIM profileUrl attribute.
	ProfileURL string `json:"profileUrl,omitempty"`
	// Title is optional and maps to the SCIM title attribute.
	Title string `json:"title,omitempty"`
	// UserType is optional and maps to the SCIM userType attribute.
	UserType string `json:"userType,omitempty"`
	// PreferredLanguage is optional and maps to the SCIM preferredLanguage attribute.
	PreferredLanguage string `json:"preferredLanguage,omitempty"`
	// Locale is optional and maps to the SCIM locale attribute.
	Locale string `json:"locale,omitempty"`
	// Timezone is optional and maps to the SCIM timezone attribute.
	Timezone string `json:"timezone,omitempty"`
	// PhoneNumbers is optional and maps to the SCIM phoneNumbers multi-valued attribute.
	PhoneNumbers []UserPhoneNumber `json:"phoneNumbers,omitempty"`
	// Roles is optional and maps to the SCIM roles multi-valued attribute.
	Roles []UserRole `json:"roles,omitempty"`
	// Enterprise is optional and maps to the SCIM Enterprise User extension
	// object. The JSON key is the enterprise extension URN. The whole object is
	// omitted from the SCIM wire when the pointer is nil.
	Enterprise *EnterpriseUser `json:"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User,omitempty"`
}

// MarshalJSON preserves the SCIM wire format for the externalId field: when the
// external id is empty the key is omitted entirely (matching the previous
// string field's omitempty behavior), otherwise it is emitted as a bare JSON
// string via ExternalId.MarshalJSON.
func (u User) MarshalJSON() ([]byte, error) {
	type alias User
	if u.ExternalId.Id == "" {
		return json.Marshal(struct {
			alias
			ExternalId *ExternalId `json:"externalId,omitempty"`
		}{alias: alias(u)})
	}
	return json.Marshal(alias(u))
}

// UserFilterResults represents filtered results when we search for
// users or List all users
type UserFilterResults struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	ItemsPerPage int      `json:"itemsPerPage"`
	StartIndex   int      `json:"startIndex"`
	Resources    []User   `json:"Resources"`
}
