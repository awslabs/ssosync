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
	"ssosync/internal/constants"
	"ssosync/internal/interfaces"
)

// buildGroup builds a SCIM-ready *Group from an already-mapped
// *interfaces.Group. The supported SCIM group attribute set is DisplayName
// only (ExternalId and Members are correlation identity / membership). Members
// is normalised to a non-nil empty slice when the input carries none, matching
// the previous constructor behaviour. The ID is handled by the callers
// (NewGroup leaves it empty, UpdateGroup stamps it from the input).
func buildGroup(g *interfaces.Group) *Group {
	members := g.Members
	if members == nil {
		members = []string{} // Initialize as empty slice, not nil
	}

	return &Group{
		Schemas:     []string{constants.SCIMSchemaGroup},
		DisplayName: g.DisplayName,
		ExternalId:  g.ExternalId,
		Members:     members,
	}
}

// NewGroup builds a SCIM-ready create payload from an already-mapped
// *interfaces.Group. The returned object has an empty ID.
func NewGroup(g *interfaces.Group) *Group {
	return buildGroup(g)
}

// UpdateGroup builds a SCIM-ready update payload from an already-mapped
// *interfaces.Group. The AWS record ID is taken from g.ID and is OPTIONAL: an
// empty g.ID yields an object with an empty ID and the constructor still
// functions.
func UpdateGroup(g *interfaces.Group) *Group {
	group := buildGroup(g)
	group.ID = g.ID
	return group
}
