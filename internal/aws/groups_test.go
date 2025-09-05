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

	"github.com/awslabs/ssosync/internal/constants"
	"github.com/stretchr/testify/assert"
)

func TestNewGroup(t *testing.T) {
	displayName := "Test Group"
	group := NewGroup(displayName, "google_id")

	assert.NotNil(t, group)
	assert.Equal(t, displayName, group.DisplayName)
	assert.Contains(t, group.Schemas, constants.SCIMSchemaGroup)
	assert.Empty(t, group.Members)
	assert.Empty(t, group.ID) // ID should be empty for new groups
}

func TestNewGroupWithEmptyName(t *testing.T) {
	group := NewGroup("", "google_id")

	assert.NotNil(t, group)
	assert.Equal(t, "", group.DisplayName)
	assert.Contains(t, group.Schemas, constants.SCIMSchemaGroup)
}

func TestNewGroupSchemas(t *testing.T) {
	group := NewGroup("Test Group", "google_id")

	assert.Len(t, group.Schemas, 1)
	assert.Equal(t, constants.SCIMSchemaGroup, group.Schemas[0])
}

func TestNewGroupMembers(t *testing.T) {
	group := NewGroup("Test Group", "google_id")

	// New groups should have empty members slice
	assert.NotNil(t, group.Members)
	assert.Len(t, group.Members, 0)
}
