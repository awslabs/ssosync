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
	return &User{
		Schemas:  []string{SCIMSchemaCoreUser},
		Username: email,
		Name: UserName{
			FamilyName: lastName,
			GivenName:  firstName,
		},
		DisplayName: strings.Join([]string{firstName, lastName}, " "),
		Active:      active,
		Emails:      []UserEmail{
			{
				Value:   email,
				Type:    "work",
				Primary: true,
			},
		},
	}
}

// UpdateUser updates a user object representing a user with the given
// details.
func UpdateUser(id string, firstName string, lastName string, email string, active bool) *User {
	return &User{
		Schemas:  []string{SCIMSchemaCoreUser},
		ID:       id,
		Username: email,
		Name: UserName{
			FamilyName: lastName,
			GivenName:  firstName,
		},
		DisplayName: strings.Join([]string{firstName, lastName}, " "),
		Active:      active,
		Emails:      []UserEmail{
			{
				Value:   email,
				Type:    "work",
				Primary: true,
			},
		},
	}
}
