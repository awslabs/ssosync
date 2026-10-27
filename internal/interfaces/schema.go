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

// Group represents a Group in AWS SSO
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

// UserAddress represents address values of users
type UserAddress struct {
	Type string `json:"type"`
}

// User represents a User in AWS SSO
type User struct {
	ID         string     `json:"id,omitempty"`
	Schemas    []string   `json:"schemas"`
	ExternalId ExternalId `json:"externalId,omitempty"`
	Username   string     `json:"userName"`
	Name       struct {
		FamilyName string `json:"familyName"`
		GivenName  string `json:"givenName"`
	} `json:"name"`
	DisplayName string        `json:"displayName"`
	Active      bool          `json:"active"`
	Emails      []UserEmail   `json:"emails"`
	Addresses   []UserAddress `json:"addresses"`
	// ExternalIds holds every ExternalId the Identity Store returned for the
	// user (a record can carry more than one). ExternalId is the one used for
	// matching. Never sent on the SCIM wire.
	ExternalIds []ExternalId `json:"-"`
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
