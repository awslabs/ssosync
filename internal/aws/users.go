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
	"strings"
)

// NewUser creates a user object representing a user with the given
// details.
func NewUser(firstName string, lastName string, email string, active bool) *User {
	e := make([]UserEmail, 0)
	e = append(e, UserEmail{
		Value:   email,
		Type:    "work",
		Primary: true,
	})

	a := make([]UserAddress, 0)
	a = append(a, UserAddress{
		Type: "work",
	})

	return &User{
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Username: email,
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{
			FamilyName: lastName,
			GivenName:  firstName,
		},
		DisplayName: strings.Join([]string{firstName, lastName}, " "),
		Active:      active,
		Emails:      e,
		Addresses:   a,
	}
}

// UpdateUser updates a user object representing a user with the given
// details.
func UpdateUser(id string, firstName string, lastName string, email string, active bool) *User {
	e := make([]UserEmail, 0)
	e = append(e, UserEmail{
		Value:   email,
		Type:    "work",
		Primary: true,
	})

	a := make([]UserAddress, 0)
	a = append(a, UserAddress{
		Type: "work",
	})

	return &User{
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		ID:       id,
		Username: email,
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{
			FamilyName: lastName,
			GivenName:  firstName,
		},
		DisplayName: strings.Join([]string{firstName, lastName}, " "),
		Active:      active,
		Emails:      e,
		Addresses:   a,
	}
}
