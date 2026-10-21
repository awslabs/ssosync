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
	"testing"

	"ssosync/internal/interfaces"

	"github.com/stretchr/testify/assert"
)

func TestNewUser(t *testing.T) {
	u := NewUser(&interfaces.User{
		Username:    "test@email.com",
		Name:        interfaces.UserName{GivenName: "Lee", FamilyName: "Packham"},
		DisplayName: "Lee Packham",
		Active:      true,
		ExternalId:  interfaces.ExternalId{Id: "google_id"},
	})
	assert.Equal(t, u.Name.GivenName, "Lee")
	assert.Equal(t, u.Name.FamilyName, "Packham")
	assert.Equal(t, u.DisplayName, "Lee Packham")
	assert.Len(t, u.Emails, 1)

	assert.Equal(t, u.Emails[0].Value, "test@email.com")
	assert.Equal(t, u.Emails[0].Primary, true)
	assert.Equal(t, u.ExternalId.Id, "google_id")

	assert.Equal(t, u.Active, true)

	assert.Len(t, u.Schemas, 1)
	assert.Equal(t, u.Schemas[0], "urn:ietf:params:scim:schemas:core:2.0:User")
	assert.Equal(t, u.ID, "")
}

func TestUpdateUser(t *testing.T) {
	u := UpdateUser(&interfaces.User{
		ID:          "111",
		Username:    "test@email.com",
		Name:        interfaces.UserName{GivenName: "Lee", FamilyName: "Packham"},
		DisplayName: "Lee Packham",
		Active:      false,
		ExternalId:  interfaces.ExternalId{Id: "google_id"},
	})
	assert.Equal(t, u.ID, "111")
	assert.Equal(t, u.Name.GivenName, "Lee")
	assert.Equal(t, u.Name.FamilyName, "Packham")
	assert.Equal(t, u.DisplayName, "Lee Packham")
	assert.Len(t, u.Emails, 1)

	assert.Equal(t, u.Emails[0].Value, "test@email.com")
	assert.Equal(t, u.Emails[0].Primary, true)
	assert.Equal(t, u.ExternalId.Id, "google_id")

	assert.Equal(t, u.Active, false)

	assert.Len(t, u.Schemas, 1)
	assert.Equal(t, u.Schemas[0], "urn:ietf:params:scim:schemas:core:2.0:User")
}

// TestUpdateUser_EmptyID confirms the AWS record ID is optional: UpdateUser
// still produces a valid object (empty ID, synthesized work email) when the
// input carries no ID.
func TestUpdateUser_EmptyID(t *testing.T) {
	u := UpdateUser(&interfaces.User{Username: "x@y.com"})
	assert.NotNil(t, u)
	assert.Equal(t, u.ID, "")
	assert.Len(t, u.Emails, 1)
	assert.Equal(t, u.Emails[0].Value, "x@y.com")
	assert.Equal(t, u.Emails[0].Primary, true)
	assert.Len(t, u.Schemas, 1)
	assert.Equal(t, u.Schemas[0], "urn:ietf:params:scim:schemas:core:2.0:User")
}

// TestNewUser_CarriesExpandedFields confirms the constructor carries across the
// expanded supported attributes and appends the enterprise URN to Schemas only
// when Enterprise is populated.
func TestNewUser_CarriesExpandedFields(t *testing.T) {
	u := NewUser(&interfaces.User{
		Username:   "test@email.com",
		Title:      "Engineer",
		Locale:     "en-US",
		Enterprise: &interfaces.EnterpriseUser{Department: "Eng"},
	})
	assert.Equal(t, u.Title, "Engineer")
	assert.Equal(t, u.Locale, "en-US")
	assert.NotNil(t, u.Enterprise)
	assert.Equal(t, u.Enterprise.Department, "Eng")
	assert.Len(t, u.Schemas, 2)
	assert.Equal(t, u.Schemas[0], "urn:ietf:params:scim:schemas:core:2.0:User")
	assert.Equal(t, u.Schemas[1], "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User")
}
