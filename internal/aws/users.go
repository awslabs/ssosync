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

// buildUser builds a SCIM-ready *User from an already-mapped *interfaces.User.
//
// It carries across EXACTLY the canonical supported SCIM attribute set — the
// same set compared by usersEqual in internal/sync.go (keep the two in
// lock-step):
//
//	Username, Name (FamilyName, GivenName, Formatted, MiddleName,
//	HonorificPrefix, HonorificSuffix), DisplayName, Active, Emails, Addresses,
//	NickName, ProfileURL, Title, UserType, PreferredLanguage, Locale, Timezone,
//	PhoneNumbers, Roles, Enterprise.
//
// Whatever the mapped input carries is carried across; whatever it does not
// carry is left zero/empty (NOT back-filled from anywhere). ExternalId is also
// carried across as it is correlation identity set by the caller/mapper. The
// ID is NOT part of the supported attribute set and is handled by the callers
// (NewUser leaves it empty, UpdateUser stamps it from the input).
//
// Schemas is rebuilt deterministically: always the core User URN, plus the
// enterprise-extension URN appended only when u.Enterprise is populated, so the
// Schemas slice is stable regardless of what the input carried.
//
// Emails/Addresses derivation: the constructor owns the same identity
// derivation it has always performed. When the mapped input does not carry its
// own Emails it synthesizes a single primary work email equal to the Username;
// when it does not carry its own Addresses it synthesizes a single work
// address. This keeps a default-config user byte-identical on the SCIM wire
// while the mapper does not yet populate these fields. Once the mapper
// populates Emails/Addresses the input values take precedence and no synthesis
// occurs.
func buildUser(u *interfaces.User) *User {
	schemas := []string{constants.SCIMSchemaUser}
	if u.Enterprise != nil {
		schemas = append(schemas, constants.SCIMSchemaEnterpriseUser)
	}

	emails := u.Emails
	if len(emails) == 0 {
		emails = []UserEmail{{
			Value:   u.Username,
			Type:    "work",
			Primary: true,
		}}
	}

	addresses := u.Addresses
	if len(addresses) == 0 {
		addresses = []UserAddress{{
			Type: "work",
		}}
	}

	return &User{
		Schemas:           schemas,
		Username:          u.Username,
		ExternalId:        u.ExternalId,
		Name:              u.Name,
		DisplayName:       u.DisplayName,
		Active:            u.Active,
		Emails:            emails,
		Addresses:         addresses,
		NickName:          u.NickName,
		ProfileURL:        u.ProfileURL,
		Title:             u.Title,
		UserType:          u.UserType,
		PreferredLanguage: u.PreferredLanguage,
		Locale:            u.Locale,
		Timezone:          u.Timezone,
		PhoneNumbers:      u.PhoneNumbers,
		Roles:             u.Roles,
		Enterprise:        u.Enterprise,
	}
}

// NewUser builds a SCIM-ready create payload from an already-mapped
// *interfaces.User. See buildUser for the full set of carried-across fields and
// the Emails/Addresses derivation. The returned object has an empty ID.
func NewUser(u *interfaces.User) *User {
	return buildUser(u)
}

// UpdateUser builds a SCIM-ready update payload from an already-mapped
// *interfaces.User. See buildUser for the full set of carried-across fields and
// the Emails/Addresses derivation.
//
// The AWS record ID is taken from u.ID and is OPTIONAL: an empty u.ID yields an
// object with an empty ID (omitted from the SCIM wire) and the constructor
// still functions.
func UpdateUser(u *interfaces.User) *User {
	user := buildUser(u)
	user.ID = u.ID
	return user
}
