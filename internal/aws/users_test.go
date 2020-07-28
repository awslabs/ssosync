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

	"github.com/stretchr/testify/assert"
)

func TestNewUser(t *testing.T) {
	u := NewUser("Lee", "Packham", "test@email.com", true)
	assert.Equal(t, u.Name.GivenName, "Lee")
	assert.Equal(t, u.Name.FamilyName, "Packham")
	assert.Equal(t, u.DisplayName, "Lee Packham")
	assert.Len(t, u.Emails, 1)

	assert.Equal(t, u.Emails[0].Value, "test@email.com")
	assert.Equal(t, u.Emails[0].Primary, true)

	assert.Equal(t, u.Active, true)

	assert.Len(t, u.Schemas, 1)
	assert.Equal(t, u.Schemas[0], "urn:ietf:params:scim:schemas:core:2.0:User")
}

func TestUpdateUser(t *testing.T) {
	u := UpdateUser("111", "Lee", "Packham", "test@email.com", false)
	assert.Equal(t, u.Name.GivenName, "Lee")
	assert.Equal(t, u.Name.FamilyName, "Packham")
	assert.Equal(t, u.DisplayName, "Lee Packham")
	assert.Len(t, u.Emails, 1)

	assert.Equal(t, u.Emails[0].Value, "test@email.com")
	assert.Equal(t, u.Emails[0].Primary, true)

	assert.Equal(t, u.Active, false)

	assert.Len(t, u.Schemas, 1)
	assert.Equal(t, u.Schemas[0], "urn:ietf:params:scim:schemas:core:2.0:User")
}
