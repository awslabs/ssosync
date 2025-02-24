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

const (
	SCIMSchemaCoreUser = "urn:ietf:params:scim:schemas:core:2.0:User"
	SCIMSchemaEnterpriseUser = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
)

// Group represents a Group in AWS SSO
type Group struct {
	ID          string   `json:"id,omitempty"`
	Schemas     []string `json:"schemas"`
	DisplayName string   `json:"displayName"`
	Members     []string `json:"members"`
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

// UserName represents name values of users
type UserName struct {
	Formatted       string `json:"formatted,omitempty"`
	FamilyName      string `json:"familyName"`
	GivenName       string `json:"givenName"`
	MiddleName      string `json:"middleName,omitempty"`
	HonorificPrefix string `json:"honorificPrefix,omitempty"`
	HonorificSuffix string `json:"honorificSuffix,omitempty"`
}

// UserAddress represents address values of users
type UserAddress struct {
	Type          string `json:"type"`
	StreetAddress string `json:"streetAddress,omitempty"`
	Locality      string `json:"locality,omitempty"`
	Region        string `json:"region,omitempty"`
	PostalCode    string `json:"postalCode,omitempty"`
	Country       string `json:"country,omitempty"`
	Formatted     string `json:"formatted,omitempty"`
	Primary       bool   `json:"primary,omitempty"`
}

type UserPhoneNumber struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type ManagerRef struct {
	Value string `json:"value,omitempty"`
	Ref string `json:"$ref,omitempty"`
}

type EnterpriseUser struct {
	EmployeeNumber string `json:"employeeNumber,omitempty"`
    CostCenter string `json:"costCenter,omitempty"`
    Organization string `json:"organization,omitempty"`
    Division string `json:"division,omitempty"`
    Department string `json:"department,omitempty"`
    Manager ManagerRef `json:"manager,omitempty"`
}

// User represents a User in AWS SSO
type User struct {
	ID       string   `json:"id,omitempty"`
	Schemas  []string `json:"schemas"`
	ExternalID        string            `json:"externalId,omitempty"`
	Username string   `json:"userName"`
	Name              UserName          `json:"name"`
	DisplayName string        `json:"displayName"`
	NickName          string            `json:"nickName,omitempty"`
	ProfileUrl        string            `json:"profileUrl,omitempty"`
	Active      bool          `json:"active"`
	Emails      []UserEmail   `json:"emails"`
	Addresses   []UserAddress `json:"addresses"`
	PhoneNumbers      []UserPhoneNumber `json:"phoneNumbers,omitempty"`
	UserType          string            `json:"userType,omitempty"`
	Title             string            `json:"title,omitempty"`
	PreferredLanguage string            `json:"preferredLanguage,omitempty"`
	Locale            string            `json:"locale,omitempty"`
	Timezone          string            `json:"timezone,omitempty"`
	Enterprise *EnterpriseUser `json:"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User,omitempty"`
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
