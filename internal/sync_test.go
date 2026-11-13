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

package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	awsinternal "ssosync/internal/aws"
	"ssosync/internal/config"
	"ssosync/internal/google"
	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
)

// opsConfig builds a minimal *config.Config carrying only the fields that
// getUserOperations/getGroupOperations read, so existing operation tests can
// pass the same ForceExternalIdUpdate/RetainUnmatched/IssuerArn values they
// previously passed positionally.
func opsConfig(forceExternalIdUpdate, retainUnmatched bool, issuerArn string) *config.Config {
	return &config.Config{
		ForceExternalIdUpdate: forceExternalIdUpdate,
		RetainUnmatched:       retainUnmatched,
		IssuerArn:             issuerArn,
	}
}

func TestNew(t *testing.T) {
	cfg := &config.Config{}
	awsClient := mocks.NewMockAwsClient(t)
	googleClient := mocks.NewMockGoogleClient(t)
	identityStore := mocks.NewMockIdentityStoreAPI(t)

	sync := New(cfg, awsClient, googleClient, identityStore)
	assert.NotNil(t, sync)
}

func TestIgnoreUser(t *testing.T) {
	cfg := &config.Config{
		IgnoreUsers: []string{"ignore1@example.com", "ignore2@example.com"},
	}

	sync := &syncGSuite{
		cfg: cfg,
	}

	assert.True(t, sync.ignoreUser("ignore1@example.com"))
	assert.True(t, sync.ignoreUser("ignore2@example.com"))
	assert.False(t, sync.ignoreUser("allow@example.com"))
}

func TestIgnoreGroup(t *testing.T) {
	cfg := &config.Config{
		IgnoreGroups: []string{"ignore-group1@example.com", "ignore-group2@example.com"},
	}

	sync := &syncGSuite{
		cfg: cfg,
	}

	assert.True(t, sync.ignoreGroup("ignore-group1@example.com"))
	assert.True(t, sync.ignoreGroup("ignore-group2@example.com"))
	assert.False(t, sync.ignoreGroup("allow-group@example.com"))
}

func TestIncludeGroup(t *testing.T) {
	cfg := &config.Config{
		IncludeGroups: []string{"include-group1@example.com", "include-group2@example.com"},
	}

	sync := &syncGSuite{
		cfg: cfg,
	}

	assert.True(t, sync.includeGroup("include-group1@example.com"))
	assert.True(t, sync.includeGroup("include-group2@example.com"))
	assert.False(t, sync.includeGroup("other-group@example.com"))
}

func TestIncludeGroup_EmptyList(t *testing.T) {
	cfg := &config.Config{
		IncludeGroups: []string{},
	}

	sync := &syncGSuite{
		cfg: cfg,
	}

	// When include list is empty, all groups should be included
	assert.False(t, sync.includeGroup("any-group@example.com"))
}

func TestGetGroupOperations_NoChange(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G1",
			Name:  "Group 1",
			Email: "group1@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, false, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Equal(t, "Group 1", equals[0].DisplayName)
	assert.Equal(t, "G1", equals[0].ExternalId.Id)
}

func TestGetGroupOperations_Add(t *testing.T) {
	awsGroups := []*interfaces.Group{}

	googleGroups := []*admin.Group{
		{
			Id:    "G2",
			Name:  "Group 2",
			Email: "group2@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, false, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 2", add[0].DisplayName)
	assert.Equal(t, "G2", add[0].ExternalId.Id)
}

func TestGetGroupOperations_Delete(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
		},
	}

	googleGroups := []*admin.Group{}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, false, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 1", delete[0].DisplayName)
	assert.Equal(t, "G1", delete[0].ExternalId.Id)
}

func TestGetGroupOperations_SyncedGroupMatched_Delete(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
		},
	}

	googleGroups := []*admin.Group{}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, true, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 1", delete[0].DisplayName)
	assert.Equal(t, "G1", delete[0].ExternalId.Id)
}

func TestGetGroupOperations_RetainUnmatched(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
		},
	}

	googleGroups := []*admin.Group{}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, true, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 1", update[0].DisplayName)
}

func TestGetGroupOperations_LocalGroupMatch(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "3",
			DisplayName: "Group 3",
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G3",
			Name:  "Group 3",
			Email: "group3@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, false, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 3", update[0].DisplayName)
	assert.Equal(t, "G3", update[0].ExternalId.Id)
}

func TestGetGroupOperations_SyncedGroupMatched_Update(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "4",
			ExternalId:  interfaces.ExternalId{Id: "G4"},
			DisplayName: "Group 4",
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G4",
			Name:  "Different Group Name",
			Email: "group4@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, false, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Different Group Name", update[0].DisplayName)
	assert.Equal(t, "G4", update[0].ExternalId.Id)
}

func TestGetGroupOperations_ExternalIdUpdate_Update(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "5",
			ExternalId:  interfaces.ExternalId{Id: "G5"},
			DisplayName: "Group 5",
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G50",
			Name:  "Group 5",
			Email: "group5@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(true, false, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 5", update[0].DisplayName)
	assert.Equal(t, "G50", update[0].ExternalId.Id)
}

func TestGetGroupOperations_ExternalIdUpdate_DeleteCreate(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "5",
			ExternalId:  interfaces.ExternalId{Id: "G5"},
			DisplayName: "Group 5",
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G50",
			Name:  "Group 5",
			Email: "group5@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, false, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 5", delete[0].DisplayName)
	assert.Equal(t, "G5", delete[0].ExternalId.Id)
	assert.Equal(t, "Group 5", add[0].DisplayName)
	assert.Equal(t, "G50", add[0].ExternalId.Id)
}

func TestGetUserOperations_NoChange(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A1",
			Username:   "user1@example.com",
			ExternalId: interfaces.ExternalId{Id: "G1"},
			Name: interfaces.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
			// DisplayName mirrors what the mapper derives for the Google user
			// ("GivenName FamilyName"); a real ssosync-managed AWS user carries
			// it, so the full-attribute comparison still reports Equal.
			DisplayName: "John Doe",
			Active:      true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Equal(t, equals[0].ExternalId.Id, "G1")
	assert.Equal(t, equals[0].Username, "user1@example.com")
	assert.Equal(t, equals[0].Name.GivenName, "John")
	assert.Equal(t, equals[0].Name.FamilyName, "Doe")
}

// TestGetUserOperations_SynthesizedEmailsAddresses_Equal is a regression guard
// for the Emails/Addresses asymmetry: a real ssosync-managed AWS user returned
// by ConvertSdkUserObjToNative carries a synthesized primary work email
// (== Username) and a {Type:"work"} address, while the default-mapped Google
// user carries neither. usersEqual must normalize both operands through the same
// builder so the user compares Equal (no spurious Update every run) rather than
// being re-classified as Update on each sync.
func TestGetUserOperations_SynthesizedEmailsAddresses_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A1",
			Username:   "user1@example.com",
			ExternalId: interfaces.ExternalId{Id: "G1"},
			Name: interfaces.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
			DisplayName: "John Doe",
			Active:      true,
			// As ConvertSdkUserObjToNative produces for an ssosync-created user:
			// the Identity Store returns the synthesized work email and address.
			Emails: []interfaces.UserEmail{{
				Value:   "user1@example.com",
				Type:    "work",
				Primary: true,
			}},
			Addresses: []interfaces.UserAddress{{
				Type: "work",
			}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "steady-state managed user must not be re-classified as Update")
	assert.Len(t, equals, 1)
	assert.Equal(t, equals[0].ExternalId.Id, "G1")
}

// TestGetUserOperations_EnrichmentFromOrganization verifies the Step D default
// map enrichment: a Google user with a primary organization but an AWS record
// that does not yet carry Title/Enterprise is classified as Update, and the
// desired user written to AWS carries the enriched Title and Enterprise.* values.
func TestGetUserOperations_EnrichmentFromOrganization(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			// No Title / Enterprise yet: the record predates the enrichment.
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Organizations: []admin.UserOrganization{
				{
					Title:      "Staff Engineer",
					Department: "Platform",
					Name:       "ExampleCorp",
					CostCenter: "CC-100",
					Type:       "work",
					Primary:    true,
				},
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		assert.Equal(t, "Staff Engineer", update[0].Title)
		if assert.NotNil(t, update[0].Enterprise) {
			assert.Equal(t, "Platform", update[0].Enterprise.Department)
			assert.Equal(t, "ExampleCorp", update[0].Enterprise.Organization)
			assert.Equal(t, "CC-100", update[0].Enterprise.CostCenter)
		}
	}
}

// TestGetUserOperations_EnrichedSteadyState_Equal verifies NFR4 / criterion 6:
// once the AWS record already carries the enriched Title/Enterprise (plus the
// synthesized primary work email/address), a user whose Google organization is
// unchanged compares Equal, so there is no churn.
func TestGetUserOperations_EnrichedSteadyState_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Title:       "Staff Engineer",
			Enterprise: &interfaces.EnterpriseUser{
				Department:   "Platform",
				Organization: "ExampleCorp",
				CostCenter:   "CC-100",
			},
			// As ConvertSdkUserObjToNative produces for a managed user.
			Emails: []interfaces.UserEmail{{
				Value:   "user1@example.com",
				Type:    "work",
				Primary: true,
			}},
			Addresses: []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Organizations: []admin.UserOrganization{
				{
					Title:      "Staff Engineer",
					Department: "Platform",
					Name:       "ExampleCorp",
					CostCenter: "CC-100",
					Type:       "work",
					Primary:    true,
				},
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "enriched steady-state user must not churn")
	assert.Len(t, equals, 1)
	assert.Equal(t, equals[0].ExternalId.Id, "G1")
}

// TestGetUserOperations_EnrichedTitleChanged verifies a changed Google org title
// trips Update and the desired user carries the new title.
func TestGetUserOperations_EnrichedTitleChanged(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Title:       "Engineer",
			Enterprise: &interfaces.EnterpriseUser{
				Department:   "Platform",
				Organization: "ExampleCorp",
				CostCenter:   "CC-100",
			},
			Emails:    []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses: []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Organizations: []admin.UserOrganization{
				{
					Title:      "Senior Engineer",
					Department: "Platform",
					Name:       "ExampleCorp",
					CostCenter: "CC-100",
					Type:       "work",
					Primary:    true,
				},
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		assert.Equal(t, "Senior Engineer", update[0].Title)
	}
}

func TestGetUserOperations_Add(t *testing.T) {
	awsUsers := []*interfaces.User{}

	googleUsers := []*admin.User{
		{
			Id:           "G7",
			PrimaryEmail: "user7@example.com",
			Name: &admin.UserName{
				GivenName:  "Bob",
				FamilyName: "Smith",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, add[0].ExternalId.Id, "G7")
	assert.Equal(t, add[0].Username, "user7@example.com")
	assert.Equal(t, add[0].Name.GivenName, "Bob")
	assert.Equal(t, add[0].Name.FamilyName, "Smith")
}

func TestGetUserOperations_UpdateAttribute(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A2",
			Username:   "user2@example.com",
			ExternalId: interfaces.ExternalId{Id: "G2"},
			Name: interfaces.UserName{
				GivenName:  "Jane",
				FamilyName: "Smith",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G2",
			PrimaryEmail: "user2@example.com",
			Name: &admin.UserName{
				GivenName:  "Jane",
				FamilyName: "Updated", // Name changed
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].ExternalId.Id, "G2")
	assert.Equal(t, update[0].Username, "user2@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Jane")
	assert.Equal(t, update[0].Name.FamilyName, "Updated")
}

func TestGetUserOperations_UpdateMissingExternalId(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:       "A5",
			Username: "user5@example.com",
			Name: interfaces.UserName{
				GivenName:  "Jane",
				FamilyName: "Doe",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G5",
			PrimaryEmail: "user5@example.com",
			Name: &admin.UserName{
				GivenName:  "Jane",
				FamilyName: "Doe",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].ExternalId.Id, "G5")
	assert.Equal(t, update[0].Username, "user5@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Jane")
	assert.Equal(t, update[0].Name.FamilyName, "Doe")
}

func TestGetUserOperations_UpdatePrimaryEmail(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A2",
			Username:   "user2@example.com",
			ExternalId: interfaces.ExternalId{Id: "G2"},
			Name: interfaces.UserName{
				GivenName:  "Jane",
				FamilyName: "Smith",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G2",
			PrimaryEmail: "user20@example.com",
			Name: &admin.UserName{
				GivenName:  "Jane",
				FamilyName: "Smith",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].ExternalId.Id, "G2")
	assert.Equal(t, update[0].Username, "user20@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Jane")
	assert.Equal(t, update[0].Name.FamilyName, "Smith")
}

func TestGetUserOperations_UpdateExternalId_DeleteRecreate(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A6",
			ExternalId: interfaces.ExternalId{Id: "GX"},
			Username:   "user6@example.com",
			Name: interfaces.UserName{
				GivenName:  "Alan",
				FamilyName: "Brown",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G6",
			PrimaryEmail: "user6@example.com",
			Name: &admin.UserName{
				GivenName:  "Alan",
				FamilyName: "Brown",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, delete[0].ExternalId.Id, "GX")
	assert.Equal(t, delete[0].Username, "user6@example.com")
	assert.Equal(t, delete[0].Name.GivenName, "Alan")
	assert.Equal(t, delete[0].Name.FamilyName, "Brown")
	assert.Equal(t, add[0].ExternalId.Id, "G6")
	assert.Equal(t, add[0].Username, "user6@example.com")
	assert.Equal(t, add[0].Name.GivenName, "Alan")
	assert.Equal(t, add[0].Name.FamilyName, "Brown")
}

func TestGetUserOperations_UpdateExternalId_ForceUpdate(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A6",
			ExternalId: interfaces.ExternalId{Id: "GX"},
			Username:   "user6@example.com",
			Name: interfaces.UserName{
				GivenName:  "Alan",
				FamilyName: "Brown",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G6",
			PrimaryEmail: "user6@example.com",
			Name: &admin.UserName{
				GivenName:  "Alan",
				FamilyName: "Brown",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(true, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].ExternalId.Id, "G6")
	assert.Equal(t, update[0].Username, "user6@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Alan")
	assert.Equal(t, update[0].Name.FamilyName, "Brown")
}

func TestGetUserOperations_DeleteNoExternalId(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:       "A3",
			Username: "user3@example.com",
			Name: interfaces.UserName{
				GivenName:  "Bob",
				FamilyName: "Johnson",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, delete[0].Username, "user3@example.com")
	assert.Equal(t, delete[0].Name.GivenName, "Bob")
	assert.Equal(t, delete[0].Name.FamilyName, "Johnson")
}

func TestGetUserOperations_DeleteExternalId(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A4",
			Username:   "user4@example.com",
			ExternalId: interfaces.ExternalId{Id: "G4"},
			Name: interfaces.UserName{
				GivenName:  "Belinda",
				FamilyName: "Johnson",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, delete[0].ExternalId.Id, "G4")
	assert.Equal(t, delete[0].Username, "user4@example.com")
	assert.Equal(t, delete[0].Name.GivenName, "Belinda")
	assert.Equal(t, delete[0].Name.FamilyName, "Johnson")
}

func TestGetUserOperations_RetainUnmatched_LocalUserUnmatched(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:       "A4",
			Username: "user4@example.com",
			Name: interfaces.UserName{
				GivenName:  "Belinda",
				FamilyName: "Johnson",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, true, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].Username, "user4@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Belinda")
	assert.Equal(t, update[0].Name.FamilyName, "Johnson")
	assert.Equal(t, update[0].Active, false)
}

func TestGetUserOperations_RetainUnmatched_LocalUserMatched(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:       "A4",
			Username: "user4@example.com",
			Name: interfaces.UserName{
				GivenName:  "Belinda",
				FamilyName: "Johnson",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G4",
			PrimaryEmail: "user4@example.com",
			Name: &admin.UserName{
				GivenName:  "Belinda",
				FamilyName: "Johnson",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, true, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].ExternalId.Id, "G4")
	assert.Equal(t, update[0].Username, "user4@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Belinda")
	assert.Equal(t, update[0].Name.FamilyName, "Johnson")
}

func TestGetUserOperations_RetainUnmatched_SyncedUserUnmatched(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A4",
			Username:   "user4@example.com",
			ExternalId: interfaces.ExternalId{Id: "G4"},
			Name: interfaces.UserName{
				GivenName:  "Belinda",
				FamilyName: "Johnson",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, true, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, delete[0].ExternalId.Id, "G4")
	assert.Equal(t, delete[0].Username, "user4@example.com")
	assert.Equal(t, delete[0].Name.GivenName, "Belinda")
	assert.Equal(t, delete[0].Name.FamilyName, "Johnson")
}

func TestGetUserOperations_RetainUnmatched_SyncedUserMismatched(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A6",
			ExternalId: interfaces.ExternalId{Id: "GX"},
			Username:   "user6@example.com",
			Name: interfaces.UserName{
				GivenName:  "Alan",
				FamilyName: "Brown",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G6",
			PrimaryEmail: "user6@example.com",
			Name: &admin.UserName{
				GivenName:  "Alan",
				FamilyName: "Brown",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, true, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, delete[0].ExternalId.Id, "GX")
	assert.Equal(t, delete[0].Username, "user6@example.com")
	assert.Equal(t, delete[0].Name.GivenName, "Alan")
	assert.Equal(t, delete[0].Name.FamilyName, "Brown")
	assert.Equal(t, add[0].ExternalId.Id, "G6")
	assert.Equal(t, add[0].Username, "user6@example.com")
	assert.Equal(t, add[0].Name.GivenName, "Alan")
	assert.Equal(t, add[0].Name.FamilyName, "Brown")
}

func TestGetUserOperations_RetainUnmatched_SyncedUserMatched(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A6",
			ExternalId:  interfaces.ExternalId{Id: "G6"},
			Username:    "user6@example.com",
			DisplayName: "Alan Brown",
			Name: interfaces.UserName{
				GivenName:  "Alan",
				FamilyName: "Brown",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G6",
			PrimaryEmail: "user6@example.com",
			Name: &admin.UserName{
				GivenName:  "Alan",
				FamilyName: "Brown",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, true, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Equal(t, equals[0].ExternalId.Id, "G6")
	assert.Equal(t, equals[0].Username, "user6@example.com")
	assert.Equal(t, equals[0].Name.GivenName, "Alan")
	assert.Equal(t, equals[0].Name.FamilyName, "Brown")
}

// The following tests exercise each path through getUserOperation, which
// returns the OperationAction and the desired user object for a single
// aws/google user pairing. The issuerArn argument lets these tests reach the
// "foreign IdP" branches that getUserOperations cannot (it always passes "").

// Path: awsUser == nil (no existing AWS user) -> Create.
func TestGetUserOperation_Create(t *testing.T) {
	// Mapped interface view of the Google user (default map: Username <-
	// primaryEmail, Active <- !suspended, ExternalId.Id <- id), as
	// getUserOperations would produce via the mapper.
	googleUser := &interfaces.User{
		ExternalId: interfaces.ExternalId{Id: "G7"},
		Username:   "user7@example.com",
		Name: interfaces.UserName{
			GivenName:  "Bob",
			FamilyName: "Smith",
		},
		Active: true,
	}

	action, user := getUserOperation(nil, googleUser, false, false, "")

	assert.Equal(t, Create, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.ExternalId.Id, "G7")
	assert.Equal(t, user.Username, "user7@example.com")
	assert.Equal(t, user.Name.GivenName, "Bob")
	assert.Equal(t, user.Name.FamilyName, "Smith")
	assert.Equal(t, user.Active, true)
}

// Path: local user (empty ExternalId.Id), no Google user, retain enabled -> Retain.
func TestGetUserOperation_LocalUnmatched_Retain(t *testing.T) {
	awsUser := &interfaces.User{
		ID:       "A4",
		Username: "user4@example.com",
		Name: interfaces.UserName{
			GivenName:  "Belinda",
			FamilyName: "Johnson",
		},
		Active: true,
	}

	action, user := getUserOperation(awsUser, nil, false, true, "")

	assert.Equal(t, Retain, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.Username, "user4@example.com")
	assert.Equal(t, user.Name.GivenName, "Belinda")
	assert.Equal(t, user.Name.FamilyName, "Johnson")
	assert.Equal(t, user.Active, false)
}

// Path: local user (empty ExternalId.Id), no Google user, retain disabled -> Delete.
func TestGetUserOperation_LocalUnmatched_Delete(t *testing.T) {
	awsUser := &interfaces.User{
		ID:       "A4",
		Username: "user4@example.com",
		Name: interfaces.UserName{
			GivenName:  "Belinda",
			FamilyName: "Johnson",
		},
		Active: true,
	}

	action, user := getUserOperation(awsUser, nil, false, false, "")

	assert.Equal(t, Delete, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.Username, "user4@example.com")
	assert.Equal(t, user.Name.GivenName, "Belinda")
	assert.Equal(t, user.Name.FamilyName, "Johnson")
}

// Path: local user (empty ExternalId.Id) with a matching Google user -> AdoptLocal.
func TestGetUserOperation_LocalMatched_AdoptLocal(t *testing.T) {
	awsUser := &interfaces.User{
		ID:       "A5",
		Username: "user5@example.com",
		Name: interfaces.UserName{
			GivenName:  "Jane",
			FamilyName: "Doe",
		},
		Active: true,
	}

	googleUser := &interfaces.User{
		ExternalId: interfaces.ExternalId{Id: "G5"},
		Username:   "user5@example.com",
		Name: interfaces.UserName{
			GivenName:  "Jane",
			FamilyName: "Doe",
		},
		Active: true,
	}

	action, user := getUserOperation(awsUser, googleUser, false, false, "")

	assert.Equal(t, AdoptLocal, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.ExternalId.Id, "G5")
	assert.Equal(t, user.Username, "user5@example.com")
	assert.Equal(t, user.Name.GivenName, "Jane")
	assert.Equal(t, user.Name.FamilyName, "Doe")
}

// Path: managed user (issuer matches / empty), no Google user -> Delete.
func TestGetUserOperation_ManagedUnmatched_Delete(t *testing.T) {
	awsUser := &interfaces.User{
		ID:         "A1",
		Username:   "user1@example.com",
		ExternalId: interfaces.ExternalId{Id: "G1"},
		Name: interfaces.UserName{
			GivenName:  "John",
			FamilyName: "Doe",
		},
		Active: true,
	}

	action, user := getUserOperation(awsUser, nil, false, false, "")

	assert.Equal(t, Delete, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.ExternalId.Id, "G1")
	assert.Equal(t, user.Username, "user1@example.com")
}

// Path: managed user, ExternalId.Id matches Google, all attributes equal -> Equal.
func TestGetUserOperation_ManagedMatched_Equal(t *testing.T) {
	awsUser := &interfaces.User{
		ID:         "A1",
		Username:   "user1@example.com",
		ExternalId: interfaces.ExternalId{Id: "G1"},
		Name: interfaces.UserName{
			GivenName:  "John",
			FamilyName: "Doe",
		},
		Active: true,
	}

	googleUser := &interfaces.User{
		ExternalId: interfaces.ExternalId{Id: "G1"},
		Username:   "user1@example.com",
		Name: interfaces.UserName{
			GivenName:  "John",
			FamilyName: "Doe",
		},
		Active: true,
	}

	action, user := getUserOperation(awsUser, googleUser, false, false, "")

	assert.Equal(t, Equal, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.ExternalId.Id, "G1")
	assert.Equal(t, user.Username, "user1@example.com")
	assert.Equal(t, user.Name.GivenName, "John")
	assert.Equal(t, user.Name.FamilyName, "Doe")
}

// Path: managed user, ExternalId.Id matches Google, an attribute differs -> Update.
func TestGetUserOperation_ManagedMatched_Update(t *testing.T) {
	awsUser := &interfaces.User{
		ID:         "A2",
		Username:   "user2@example.com",
		ExternalId: interfaces.ExternalId{Id: "G2"},
		Name: interfaces.UserName{
			GivenName:  "Jane",
			FamilyName: "Smith",
		},
		Active: true,
	}

	googleUser := &interfaces.User{
		ExternalId: interfaces.ExternalId{Id: "G2"},
		Username:   "user2@example.com",
		Name: interfaces.UserName{
			GivenName:  "Jane",
			FamilyName: "Updated", // Name changed
		},
		Active: true,
	}

	action, user := getUserOperation(awsUser, googleUser, false, false, "")

	assert.Equal(t, Update, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.ExternalId.Id, "G2")
	assert.Equal(t, user.Username, "user2@example.com")
	assert.Equal(t, user.Name.GivenName, "Jane")
	assert.Equal(t, user.Name.FamilyName, "Updated")
}

// Path: managed user, ExternalId.Id differs from Google, force enabled -> UpdateForced.
func TestGetUserOperation_ManagedMismatched_UpdateForced(t *testing.T) {
	awsUser := &interfaces.User{
		ID:         "A6",
		Username:   "user6@example.com",
		ExternalId: interfaces.ExternalId{Id: "GX"},
		Name: interfaces.UserName{
			GivenName:  "Alan",
			FamilyName: "Brown",
		},
		Active: true,
	}

	googleUser := &interfaces.User{
		ExternalId: interfaces.ExternalId{Id: "G6"},
		Username:   "user6@example.com",
		Name: interfaces.UserName{
			GivenName:  "Alan",
			FamilyName: "Brown",
		},
		Active: true,
	}

	action, user := getUserOperation(awsUser, googleUser, true, false, "")

	assert.Equal(t, UpdateForced, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.ExternalId.Id, "G6")
	assert.Equal(t, user.Username, "user6@example.com")
	assert.Equal(t, user.Name.GivenName, "Alan")
	assert.Equal(t, user.Name.FamilyName, "Brown")
}

// Path: managed user, ExternalId.Id differs from Google, force disabled -> Recreate.
func TestGetUserOperation_ManagedMismatched_Recreate(t *testing.T) {
	awsUser := &interfaces.User{
		ID:         "A6",
		Username:   "user6@example.com",
		ExternalId: interfaces.ExternalId{Id: "GX"},
		Name: interfaces.UserName{
			GivenName:  "Alan",
			FamilyName: "Brown",
		},
		Active: true,
	}

	googleUser := &interfaces.User{
		ExternalId: interfaces.ExternalId{Id: "G6"},
		Username:   "user6@example.com",
		Name: interfaces.UserName{
			GivenName:  "Alan",
			FamilyName: "Brown",
		},
		Active: true,
	}

	action, user := getUserOperation(awsUser, googleUser, false, false, "")

	assert.Equal(t, Recreate, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.ExternalId.Id, "G6")
	assert.Equal(t, user.Username, "user6@example.com")
	assert.Equal(t, user.Name.GivenName, "Alan")
	assert.Equal(t, user.Name.FamilyName, "Brown")
}

// Path: foreign user (ExternalId set, Issuer != issuerArn), no Google user,
// retain enabled -> Retain.
func TestGetUserOperation_ForeignUnmatched_Retain(t *testing.T) {
	awsUser := &interfaces.User{
		ID:       "A8",
		Username: "user8@example.com",
		ExternalId: interfaces.ExternalId{
			Id:     "G8",
			Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
		},
		Name: interfaces.UserName{
			GivenName:  "Carol",
			FamilyName: "White",
		},
		Active: true,
	}

	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	action, user := getUserOperation(awsUser, nil, false, true, issuerArn)

	assert.Equal(t, Retain, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.Username, "user8@example.com")
	assert.Equal(t, user.Name.GivenName, "Carol")
	assert.Equal(t, user.Name.FamilyName, "White")
	assert.Equal(t, user.Active, false)
}

// Path: foreign user (ExternalId set, Issuer != issuerArn), no Google user,
// retain disabled -> Delete.
func TestGetUserOperation_ForeignUnmatched_Delete(t *testing.T) {
	awsUser := &interfaces.User{
		ID:       "A8",
		Username: "user8@example.com",
		ExternalId: interfaces.ExternalId{
			Id:     "G8",
			Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
		},
		Name: interfaces.UserName{
			GivenName:  "Carol",
			FamilyName: "White",
		},
		Active: true,
	}

	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	action, user := getUserOperation(awsUser, nil, false, false, issuerArn)

	assert.Equal(t, Delete, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.ExternalId.Id, "G8")
	assert.Equal(t, user.Username, "user8@example.com")
}

// Path: foreign user (ExternalId set, Issuer != issuerArn) with a matching
// Google user -> AdoptForeign.
func TestGetUserOperation_ForeignMatched_AdoptForeign(t *testing.T) {
	awsUser := &interfaces.User{
		ID:       "A8",
		Username: "user8@example.com",
		ExternalId: interfaces.ExternalId{
			Id:     "G8",
			Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
		},
		Name: interfaces.UserName{
			GivenName:  "Carol",
			FamilyName: "White",
		},
		Active: true,
	}

	googleUser := &interfaces.User{
		ExternalId: interfaces.ExternalId{Id: "G8"},
		Username:   "user8@example.com",
		Name: interfaces.UserName{
			GivenName:  "Carol",
			FamilyName: "White",
		},
		Active: true,
	}

	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	action, user := getUserOperation(awsUser, googleUser, false, false, issuerArn)

	assert.Equal(t, AdoptForeign, action)
	assert.NotNil(t, user)
	assert.Equal(t, user.ExternalId.Id, "G8")
	assert.Equal(t, user.Username, "user8@example.com")
	assert.Equal(t, user.Name.GivenName, "Carol")
	assert.Equal(t, user.Name.FamilyName, "White")
}

// TestUsersEqual_SupportedFieldDiffers confirms usersEqual now compares the full
// canonical supported attribute set: a difference in any supported field (Title,
// PhoneNumbers, Enterprise.*) makes the users unequal, while two users identical
// across the whole supported set compare equal.
func TestUsersEqual_SupportedFieldDiffers(t *testing.T) {
	base := func() *interfaces.User {
		return &interfaces.User{
			Username:    "user@example.com",
			Name:        interfaces.UserName{GivenName: "Jane", FamilyName: "Doe"},
			DisplayName: "Jane Doe",
			Active:      true,
		}
	}

	// Identical across the whole supported set -> equal.
	assert.True(t, usersEqual(base(), base()))

	// Title differs -> not equal.
	withTitle := base()
	withTitle.Title = "Engineer"
	assert.False(t, usersEqual(base(), withTitle))

	// PhoneNumbers differ -> not equal.
	withPhone := base()
	withPhone.PhoneNumbers = []interfaces.UserPhoneNumber{{Value: "+1-555-0100", Type: "work", Primary: true}}
	assert.False(t, usersEqual(base(), withPhone))

	// Enterprise.Department differs -> not equal.
	withEnterprise := base()
	withEnterprise.Enterprise = &interfaces.EnterpriseUser{Department: "Eng"}
	assert.False(t, usersEqual(base(), withEnterprise))

	// Emails/Addresses asymmetry: an AWS-side user carrying the synthesized work
	// email/address compares Equal to a Google-side user carrying neither,
	// because both operands are normalized through the same builder (the builder
	// synthesizes the same work email == Username and work address). Without the
	// normalization this would spuriously report a difference.
	awsSide := base()
	awsSide.Emails = []interfaces.UserEmail{{Value: "user@example.com", Type: "work", Primary: true}}
	awsSide.Addresses = []interfaces.UserAddress{{Type: "work"}}
	assert.True(t, usersEqual(awsSide, base()))
}

// TestGetUserOperation_ClearToConsistent confirms clear-to-consistent semantics:
// an AWS user with a Title that the attribute map does not populate is reported
// as an Update, and the returned desired object has an empty (cleared) Title
// while the mapped Google attributes are preserved.
func TestGetUserOperation_ClearToConsistent(t *testing.T) {
	awsUser := &interfaces.User{
		ID:         "A9",
		Username:   "user9@example.com",
		ExternalId: interfaces.ExternalId{Id: "G9"},
		Name: interfaces.UserName{
			GivenName:  "Sam",
			FamilyName: "Green",
		},
		Active: true,
		Title:  "Engineer", // expanded field the default map does not populate
	}

	// Mapped Google user as the default map produces it: no Title.
	googleUser := &interfaces.User{
		ExternalId: interfaces.ExternalId{Id: "G9"},
		Username:   "user9@example.com",
		Name: interfaces.UserName{
			GivenName:  "Sam",
			FamilyName: "Green",
		},
		Active: true,
	}

	action, user := getUserOperation(awsUser, googleUser, false, false, "")

	assert.Equal(t, Update, action)
	assert.NotNil(t, user)
	// The desired object stamps the AWS record ID so the update targets it.
	assert.Equal(t, "A9", user.ID)
	// Title is cleared to make AWS consistent with Google.
	assert.Equal(t, "", user.Title)
	// Mapped Google attributes are preserved.
	assert.Equal(t, "user9@example.com", user.Username)
	assert.Equal(t, "Sam", user.Name.GivenName)
	assert.Equal(t, "Green", user.Name.FamilyName)
	assert.Equal(t, true, user.Active)
}

func TestGetGroupOperations_RetainUnmatched_LocalGroupMatched(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G1",
			Name:  "Group 1",
			Email: "group1@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, true, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 1", update[0].DisplayName)
	assert.Equal(t, "G1", update[0].ExternalId.Id)
}

func TestGetGroupOperations_RetainUnmatched_LocalGroupUnmatched(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
		},
	}

	googleGroups := []*admin.Group{}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, true, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 1", update[0].DisplayName)
}

func TestGetGroupOperations_RetainUnmatched_SyncedGroupUnMatched(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
		},
	}

	googleGroups := []*admin.Group{}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, true, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 1", delete[0].DisplayName)
	assert.Equal(t, "G1", delete[0].ExternalId.Id)
}

func TestGetGroupOperations_RetainUnmatched_SyncedGroupMismatched(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
			ExternalId:  interfaces.ExternalId{Id: "GX"},
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G1",
			Name:  "Group 1",
			Email: "group1@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, true, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 1", delete[0].DisplayName)
	assert.Equal(t, "GX", delete[0].ExternalId.Id)
	assert.Equal(t, "Group 1", add[0].DisplayName)
	assert.Equal(t, "G1", add[0].ExternalId.Id)
}

func TestGetGroupOperations_RetainUnmatched_SyncedGroupMatched(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G1",
			Name:  "Group 1",
			Email: "group1@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, true, ""), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Equal(t, "Group 1", equals[0].DisplayName)
	assert.Equal(t, "G1", equals[0].ExternalId.Id)
}

// Path: awsGroup == nil (no existing AWS group) -> Create.
func TestGetGroupOperation_Create(t *testing.T) {
	// Mapped interface view of the Google group (default map: DisplayName <- name,
	// ExternalId.Id <- id), as getGroupOperations would produce via the mapper.
	googleGroup := &interfaces.Group{
		DisplayName: "Group 1",
		ExternalId:  interfaces.ExternalId{Id: "G1"},
	}

	action, group := getGroupOperation(nil, googleGroup, false, false, "")

	assert.Equal(t, Create, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 1", group.DisplayName)
	assert.Equal(t, "G1", group.ExternalId.Id)
}

// Path: local group (empty ExternalId.Id), no Google group, retain enabled -> Retain.
func TestGetGroupOperation_LocalUnmatched_Retain(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A1",
		DisplayName: "Group 1",
	}

	action, group := getGroupOperation(awsGroup, nil, false, true, "")

	assert.Equal(t, Retain, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 1", group.DisplayName)
	assert.Equal(t, "", group.ExternalId.Id)
}

// Path: local group (empty ExternalId.Id), no Google group, retain disabled -> Delete.
func TestGetGroupOperation_LocalUnmatched_Delete(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A1",
		DisplayName: "Group 1",
	}

	action, group := getGroupOperation(awsGroup, nil, false, false, "")

	assert.Equal(t, Delete, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 1", group.DisplayName)
	assert.Equal(t, "", group.ExternalId.Id)
}

// Path: local group (empty ExternalId.Id) with a matching Google group -> AdoptLocal.
func TestGetGroupOperation_LocalMatched_AdoptLocal(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A3",
		DisplayName: "Group 3",
	}

	googleGroup := &interfaces.Group{
		DisplayName: "Group 3",
		ExternalId:  interfaces.ExternalId{Id: "G3"},
	}

	action, group := getGroupOperation(awsGroup, googleGroup, false, false, "")

	assert.Equal(t, AdoptLocal, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 3", group.DisplayName)
	assert.Equal(t, "G3", group.ExternalId.Id)
}

// Path: managed group (issuer matches / empty), no Google group -> Delete.
func TestGetGroupOperation_ManagedUnmatched_Delete(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A1",
		DisplayName: "Group 1",
		ExternalId:  interfaces.ExternalId{Id: "G1"},
	}

	action, group := getGroupOperation(awsGroup, nil, false, false, "")

	assert.Equal(t, Delete, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 1", group.DisplayName)
	assert.Equal(t, "G1", group.ExternalId.Id)
}

// Path: managed group, ExternalId.Id matches Google, DisplayName equal -> Equal.
func TestGetGroupOperation_ManagedMatched_Equal(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A1",
		DisplayName: "Group 1",
		ExternalId:  interfaces.ExternalId{Id: "G1"},
	}

	googleGroup := &interfaces.Group{
		DisplayName: "Group 1",
		ExternalId:  interfaces.ExternalId{Id: "G1"},
	}

	action, group := getGroupOperation(awsGroup, googleGroup, false, false, "")

	assert.Equal(t, Equal, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 1", group.DisplayName)
	assert.Equal(t, "G1", group.ExternalId.Id)
}

// Path: managed group, ExternalId.Id matches Google, DisplayName differs -> Update.
func TestGetGroupOperation_ManagedMatched_Update(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A4",
		DisplayName: "Group 4",
		ExternalId:  interfaces.ExternalId{Id: "G4"},
	}

	googleGroup := &interfaces.Group{
		DisplayName: "Different Group Name",
		ExternalId:  interfaces.ExternalId{Id: "G4"},
	}

	action, group := getGroupOperation(awsGroup, googleGroup, false, false, "")

	assert.Equal(t, Update, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Different Group Name", group.DisplayName)
	assert.Equal(t, "G4", group.ExternalId.Id)
}

// Path: managed group, ExternalId.Id differs from Google, force enabled -> UpdateForced.
func TestGetGroupOperation_ManagedMismatched_UpdateForced(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A5",
		DisplayName: "Group 5",
		ExternalId:  interfaces.ExternalId{Id: "GX"},
	}

	googleGroup := &interfaces.Group{
		DisplayName: "Group 5",
		ExternalId:  interfaces.ExternalId{Id: "G50"},
	}

	action, group := getGroupOperation(awsGroup, googleGroup, true, false, "")

	assert.Equal(t, UpdateForced, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 5", group.DisplayName)
	assert.Equal(t, "G50", group.ExternalId.Id)
}

// Path: managed group, ExternalId.Id differs from Google, force disabled -> Recreate.
func TestGetGroupOperation_ManagedMismatched_Recreate(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A5",
		DisplayName: "Group 5",
		ExternalId:  interfaces.ExternalId{Id: "GX"},
	}

	googleGroup := &interfaces.Group{
		DisplayName: "Group 5",
		ExternalId:  interfaces.ExternalId{Id: "G50"},
	}

	action, group := getGroupOperation(awsGroup, googleGroup, false, false, "")

	assert.Equal(t, Recreate, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 5", group.DisplayName)
	assert.Equal(t, "G50", group.ExternalId.Id)
}

// Path: foreign group (ExternalId set, Issuer != issuerArn), no Google group,
// retain enabled -> Retain.
func TestGetGroupOperation_ForeignUnmatched_Retain(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A8",
		DisplayName: "Group 8",
		ExternalId: interfaces.ExternalId{
			Id:     "G8",
			Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
		},
	}

	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	action, group := getGroupOperation(awsGroup, nil, false, true, issuerArn)

	assert.Equal(t, Retain, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 8", group.DisplayName)
	assert.Equal(t, "G8", group.ExternalId.Id)
}

// Path: foreign group (ExternalId set, Issuer != issuerArn), no Google group,
// retain disabled -> Delete.
func TestGetGroupOperation_ForeignUnmatched_Delete(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A8",
		DisplayName: "Group 8",
		ExternalId: interfaces.ExternalId{
			Id:     "G8",
			Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
		},
	}

	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	action, group := getGroupOperation(awsGroup, nil, false, false, issuerArn)

	assert.Equal(t, Delete, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 8", group.DisplayName)
	assert.Equal(t, "G8", group.ExternalId.Id)
}

// Path: foreign group (ExternalId set, Issuer != issuerArn) with a matching
// Google group -> AdoptForeign.
func TestGetGroupOperation_ForeignMatched_AdoptForeign(t *testing.T) {
	awsGroup := &interfaces.Group{
		ID:          "A8",
		DisplayName: "Group 8",
		ExternalId: interfaces.ExternalId{
			Id:     "G8",
			Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
		},
	}

	googleGroup := &interfaces.Group{
		DisplayName: "Group 8",
		ExternalId:  interfaces.ExternalId{Id: "G8"},
	}

	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	action, group := getGroupOperation(awsGroup, googleGroup, false, false, issuerArn)

	assert.Equal(t, AdoptForeign, action)
	assert.NotNil(t, group)
	assert.Equal(t, "Group 8", group.DisplayName)
	assert.Equal(t, "G8", group.ExternalId.Id)
}

func TestGetUserOperations_SuspendedStateChange(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:       "1",
			Username: "user1@example.com",
			Name: interfaces.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
			Active: true, // Currently active in AWS
		},
	}

	googleUsers := []*admin.User{
		{
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
			Suspended: true, // Suspended in Google
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	// Should update user1 (suspended state changed)
	assert.Len(t, update, 1)
	assert.Equal(t, "user1@example.com", update[0].Username)
	assert.False(t, update[0].Active) // Should be inactive now

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
}

func TestGetGroupUsersOperations(t *testing.T) {
	googleGroupsUsers := map[string][]*admin.User{
		"group1@example.com": {
			{PrimaryEmail: "user1@example.com"},
			{PrimaryEmail: "user2@example.com"},
		},
		"group2@example.com": {
			{PrimaryEmail: "user1@example.com"},
		},
	}

	awsGroupsUsers := map[string][]*interfaces.User{
		"group1@example.com": {
			{Username: "user1@example.com"},
			{Username: "user3@example.com"}, // Should be removed
		},
		"group2@example.com": {
			{Username: "user1@example.com"},
			{Username: "user4@example.com"}, // Should be removed
		},
	}

	mockAws := mocks.NewMockAwsClient(t)
	mockAws.On("FindUserByEmail", "user2@example.com").Return(&interfaces.User{Username: "user2@example.com"}, nil)

	s := &syncGSuite{
		aws: mockAws,
		cfg: &config.Config{AttributeMap: config.DefaultAttributeMap},
	}

	ids := newMemberIdentities(nil, config.DefaultAttributeMap.User, nil)
	addUsers, deleteUsers, unchangedUsers, err := s.getGroupUsersOperations(googleGroupsUsers, awsGroupsUsers, ids)
	assert.NoError(t, err)

	// Should Add user2 in group1
	assert.Len(t, addUsers["group1@example.com"], 1)

	// Should remove user3 from group1 and user4 from group2
	assert.Len(t, deleteUsers["group1@example.com"], 1)
	assert.Equal(t, "user3@example.com", deleteUsers["group1@example.com"][0].Username)

	assert.Len(t, deleteUsers["group2@example.com"], 1)
	assert.Equal(t, "user4@example.com", deleteUsers["group2@example.com"][0].Username)

	// Should keep user1 in group1, user1 in group2
	assert.Len(t, unchangedUsers["group1@example.com"], 1)
	assert.Len(t, unchangedUsers["group2@example.com"], 1)
}

func TestCreateUserIDtoUserObjMap(t *testing.T) {
	users := []*interfaces.User{
		{ID: "1", Username: "user1@example.com"},
		{ID: "2", Username: "user2@example.com"},
		{ID: "3", Username: "user3@example.com"},
	}

	userMap := CreateUserIDtoUserObjMap(users)

	assert.Len(t, userMap, 3)
	assert.Equal(t, "user1@example.com", userMap["1"].Username)
	assert.Equal(t, "user2@example.com", userMap["2"].Username)
	assert.Equal(t, "user3@example.com", userMap["3"].Username)
}

func TestConvertIdentityStoreGroupToAWSGroup(t *testing.T) {
	// Test with valid group
	groupId := "group-123"
	displayName := "Test Group"

	identityStoreGroup := types.Group{
		GroupId:     &groupId,
		DisplayName: &displayName,
	}

	awsGroup := ConvertIdentityStoreGroupToAWSGroup(identityStoreGroup)
	assert.NotNil(t, awsGroup)
	assert.Equal(t, "group-123", awsGroup.ID)
	assert.Equal(t, "Test Group", awsGroup.DisplayName)
	assert.Empty(t, awsGroup.Members)

	// Test with nil GroupId
	identityStoreGroupNoId := types.Group{
		DisplayName: &displayName,
	}

	awsGroupNoId := ConvertIdentityStoreGroupToAWSGroup(identityStoreGroupNoId)
	assert.Nil(t, awsGroupNoId)

	// Test with nil DisplayName
	identityStoreGroupNoName := types.Group{
		GroupId: &groupId,
	}

	awsGroupNoName := ConvertIdentityStoreGroupToAWSGroup(identityStoreGroupNoName)
	assert.Nil(t, awsGroupNoName)
}

// TestConvertSdkUserObjToNative_ExternalIdFull verifies the native user built
// from an SDK user carries the full ExternalId structure (both Id and Issuer),
// consuming what the Identity Store SDK exposes.
func TestConvertSdkUserObjToNative_ExternalIdFull(t *testing.T) {
	identityStoreUser := sdkUser("user-123", "user1@example.com", "John", "Doe", "G1")

	native := ConvertSdkUserObjToNative(identityStoreUser)

	assert.NotNil(t, native)
	assert.Equal(t, "G1", native.ExternalId.Id)
	assert.Equal(t, "ssosync", native.ExternalId.Issuer)
}

// TestConvertIdentityStoreGroupToAWSGroup_ExternalIdFull verifies the native
// group built from an SDK group carries the full ExternalId structure.
func TestConvertIdentityStoreGroupToAWSGroup_ExternalIdFull(t *testing.T) {
	identityStoreGroup := types.Group{
		GroupId:     aws.String("group-123"),
		DisplayName: aws.String("Test Group"),
		ExternalIds: []types.ExternalId{
			{Id: aws.String("G1"), Issuer: aws.String("ssosync")},
		},
	}

	native := ConvertIdentityStoreGroupToAWSGroup(identityStoreGroup)

	assert.NotNil(t, native)
	assert.Equal(t, "G1", native.ExternalId.Id)
	assert.Equal(t, "ssosync", native.ExternalId.Issuer)
}

// TestGetExternalId_EmptySlice verifies getExternalId returns the zero-value
// struct when no external IDs are present.
func TestGetExternalId_EmptySlice(t *testing.T) {
	// Test with a nil slice.
	assert.Equal(t, interfaces.ExternalId{}, getExternalId(nil))

	// Test with an empty slice.
	assert.Equal(t, interfaces.ExternalId{}, getExternalId([]types.ExternalId{}))
}

// TestConvertSdkUserObjToNative_AddressEmptyType verifies that a present
// address with a nil or empty Type still yields a UserAddress entry with an
// empty Type rather than being dropped.
func TestConvertSdkUserObjToNative_AddressEmptyType(t *testing.T) {
	// Test with a nil address Type.
	userNilType := sdkUser("user-123", "user1@example.com", "John", "Doe", "G1")
	userNilType.Addresses = []types.Address{
		{Type: nil},
	}

	nativeNilType := ConvertSdkUserObjToNative(userNilType)
	assert.NotNil(t, nativeNilType)
	assert.Len(t, nativeNilType.Addresses, 1)
	assert.Equal(t, "", nativeNilType.Addresses[0].Type)

	// Test with an explicitly empty address Type.
	userEmptyType := sdkUser("user-123", "user1@example.com", "John", "Doe", "G1")
	userEmptyType.Addresses = []types.Address{
		{Type: aws.String("")},
	}

	nativeEmptyType := ConvertSdkUserObjToNative(userEmptyType)
	assert.NotNil(t, nativeEmptyType)
	assert.Len(t, nativeEmptyType.Addresses, 1)
	assert.Equal(t, "", nativeEmptyType.Addresses[0].Type)
}

// TestConvertSdkUserObjToNative_AddressNilSlice verifies that a user with no
// address block yields a non-nil, empty Addresses slice and does not panic.
func TestConvertSdkUserObjToNative_AddressNilSlice(t *testing.T) {
	identityStoreUser := sdkUser("user-123", "user1@example.com", "John", "Doe", "G1")
	identityStoreUser.Addresses = nil

	native := ConvertSdkUserObjToNative(identityStoreUser)

	assert.NotNil(t, native)
	assert.NotNil(t, native.Addresses)
	assert.Len(t, native.Addresses, 0)
}

func TestDoSync_InvalidCredentials(t *testing.T) {
	cfg := &config.Config{
		GoogleCredentials: "invalid-path.json",
		IsLambda:          false,
	}

	err := DoSync(context.Background(), cfg)
	assert.Error(t, err)
}

func TestDoSync_DryRun(t *testing.T) {
	cfg := &config.Config{
		GoogleCredentials: "testdata/credentials.json", // Would need valid test credentials
		DryRun:            true,
		IsLambda:          false,
	}

	// This test would require valid Google credentials and AWS setup
	// In a real test environment, you'd mock the dependencies
	err := DoSync(context.Background(), cfg)
	// We expect this to fail without proper credentials, but we're testing the dry run path
	assert.Error(t, err) // Expected to fail without valid credentials
}

func TestLegacySyncGroups_DryRunUserWithoutID(t *testing.T) {
	cfg := &config.Config{
		SyncMethod:      "users_groups",
		IncludeGroups:   []string{"group@example.com"},
		IdentityStoreID: "d-1234567890",
		DryRun:          true,
	}

	group := &admin.Group{Email: "group@example.com", Id: "gid"}

	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("").Return([]*admin.Group{group}, nil)
	googleClient.EXPECT().GetGroupMembers(group).
		Return([]*admin.Member{{Email: "new@example.com"}}, nil)

	// LegacySyncGroups now sources existing AWS groups from the Identity Store so
	// their ExternalId is populated for matching.
	identityStore := mocks.NewMockIdentityStoreAPI(t)
	identityStore.EXPECT().ListGroups(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupsOutput{
			Groups: []types.Group{
				{GroupId: aws.String("aws-gid"), DisplayName: aws.String("group@example.com")},
			},
		}, nil)
	// No EXPECT for IsMemberInGroups: mockery fails the test on any unexpected
	// call, so this asserts the API is never reached for an ID-less user.

	awsClient := mocks.NewMockAwsClient(t)
	// The AWS group matches by display name but has no ExternalId, so it is
	// updated to backfill the ExternalId (the Google group id).
	awsClient.EXPECT().UpdateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-gid" && g.DisplayName == "group@example.com" && g.ExternalId.Id == "gid"
	})).Return(&interfaces.Group{ID: "aws-gid", DisplayName: "group@example.com", ExternalId: interfaces.ExternalId{Id: "gid"}}, nil)
	awsClient.EXPECT().AddUserToGroup(mock.Anything, mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-gid"
	})).Return(nil)

	s := &syncGSuite{
		aws:           awsClient,
		google:        googleClient,
		identityStore: identityStore,
		cfg:           cfg,
		users: map[string]*interfaces.User{
			// As dryClient.CreateUser leaves it: no ID.
			"new@example.com": {Username: "new@example.com", ID: ""},
		},
	}

	assert.NoError(t, s.LegacySyncGroups(""))
}

// sdkUser builds an Identity Store SDK user with all the pointer fields that
// ConvertSdkUserObjToNative dereferences populated.
func sdkUser(id, userName, givenName, familyName, externalID string) types.User {
	u := types.User{
		UserId:      aws.String(id),
		UserName:    aws.String(userName),
		DisplayName: aws.String(givenName + " " + familyName),
		Name: &types.Name{
			GivenName:  aws.String(givenName),
			FamilyName: aws.String(familyName),
		},
	}
	if externalID != "" {
		u.ExternalIds = []types.ExternalId{{Id: aws.String(externalID), Issuer: aws.String("ssosync")}}
	}
	return u
}

// sdkGroup builds an Identity Store SDK group with an optional external id.
func sdkGroup(id, displayName, externalID string) types.Group {
	g := types.Group{
		GroupId:     aws.String(id),
		DisplayName: aws.String(displayName),
	}
	if externalID != "" {
		g.ExternalIds = []types.ExternalId{{Id: aws.String(externalID), Issuer: aws.String("ssosync")}}
	}
	return g
}

// newLegacySyncUsersSuite wires up a syncGSuite for LegacySyncUsers tests. It stubs the
// google deleted-users lookup (empty) and both GetUsers calls, and the
// Identity Store ListUsers call that s.GetUsers() drives.
func newLegacySyncUsersSuite(t *testing.T, cfg *config.Config, googleUsers []*admin.User, awsSdkUsers []types.User) (*syncGSuite, *mocks.MockAwsClient, *mocks.MockGoogleClient) {
	t.Helper()

	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetDeletedUsers().Return([]*admin.User{}, nil)
	googleClient.EXPECT().GetUsers(mock.Anything, mock.Anything).Return(googleUsers, nil)

	identityStore := mocks.NewMockIdentityStoreAPI(t)
	identityStore.EXPECT().ListUsers(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{Users: awsSdkUsers}, nil)

	awsClient := mocks.NewMockAwsClient(t)

	s := &syncGSuite{
		aws:           awsClient,
		google:        googleClient,
		identityStore: identityStore,
		cfg:           cfg,
		users:         make(map[string]*interfaces.User),
	}
	return s, awsClient, googleClient
}

func gUser(id, email, given, family string, suspended bool) *admin.User {
	return &admin.User{
		Id:           id,
		PrimaryEmail: email,
		Suspended:    suspended,
		Name:         &admin.UserName{GivenName: given, FamilyName: family},
	}
}

func TestLegacySyncUsers_ExternalIdMatch_Update(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// Same ExternalId, but the family name changed in Google -> update.
	google := []*admin.User{gUser("gid-1", "user@example.com", "Jane", "Doe", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Jane", "Smith", "gid-1")}

	s, awsClient, _ := newLegacySyncUsersSuite(t, cfg, google, awsSdk)

	// Active is backfilled via FindUserByEmail for each existing AWS user, and
	// setUser re-fetches the synced user afterwards.
	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)
	awsClient.EXPECT().UpdateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.ExternalId.Id == "gid-1" && u.Name.FamilyName == "Doe"
	})).Return(&interfaces.User{ID: "aws-1", Username: "user@example.com"}, nil)

	assert.NoError(t, s.LegacySyncUsers(""))
}

func TestLegacySyncUsers_EmailMatch_ForceExternalIdUpdate(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups", ForceExternalIdUpdate: true}

	// Same email, different Google id, existing AWS user has an ExternalId.
	google := []*admin.User{gUser("gid-new", "user@example.com", "Jane", "Doe", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Jane", "Doe", "gid-old")}

	s, awsClient, _ := newLegacySyncUsersSuite(t, cfg, google, awsSdk)

	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)
	// Force option -> update in place with the new ExternalId, no delete/create.
	awsClient.EXPECT().UpdateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.ExternalId.Id == "gid-new"
	})).Return(&interfaces.User{ID: "aws-1", Username: "user@example.com"}, nil)

	assert.NoError(t, s.LegacySyncUsers(""))
}

func TestLegacySyncUsers_EmailMatch_DeleteAndRecreate(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups", ForceExternalIdUpdate: false}

	google := []*admin.User{gUser("gid-new", "user@example.com", "Jane", "Doe", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Jane", "Doe", "gid-old")}

	s, awsClient, _ := newLegacySyncUsersSuite(t, cfg, google, awsSdk)

	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)
	// No force -> delete existing, create new to avoid inherited privileges.
	awsClient.EXPECT().DeleteUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.ExternalId.Id == "gid-old"
	})).Return(nil)
	awsClient.EXPECT().CreateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "" && u.ExternalId.Id == "gid-new" && u.Username == "user@example.com"
	})).Return(&interfaces.User{ID: "aws-2", Username: "user@example.com"}, nil)

	assert.NoError(t, s.LegacySyncUsers(""))
}

func TestLegacySyncUsers_EmailMatch_AdoptNoExternalId(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// Existing AWS user matched by email with no ExternalId -> adopt (update).
	google := []*admin.User{gUser("gid-1", "user@example.com", "Jane", "Doe", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Jane", "Doe", "")}

	s, awsClient, _ := newLegacySyncUsersSuite(t, cfg, google, awsSdk)

	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)
	awsClient.EXPECT().UpdateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.ExternalId.Id == "gid-1"
	})).Return(&interfaces.User{ID: "aws-1", Username: "user@example.com"}, nil)

	assert.NoError(t, s.LegacySyncUsers(""))
}

func TestLegacySyncUsers_NoMatch_Create(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// New google user, no existing AWS users.
	google := []*admin.User{gUser("gid-1", "new@example.com", "New", "User", false)}

	s, awsClient, _ := newLegacySyncUsersSuite(t, cfg, google, []types.User{})

	awsClient.EXPECT().CreateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.Username == "new@example.com" && u.ExternalId.Id == "gid-1"
	})).Return(&interfaces.User{ID: "aws-9", Username: "new@example.com"}, nil)
	// setUser re-fetch after create.
	awsClient.EXPECT().FindUserByEmail("new@example.com").
		Return(&interfaces.User{ID: "aws-9", Username: "new@example.com"}, nil)

	assert.NoError(t, s.LegacySyncUsers(""))
}

func TestLegacySyncUsers_Unmatched_Delete(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// No google users; one existing AWS user that is unmatched -> delete.
	awsSdk := []types.User{sdkUser("aws-1", "stale@example.com", "Stale", "User", "gid-stale")}

	s, awsClient, _ := newLegacySyncUsersSuite(t, cfg, []*admin.User{}, awsSdk)

	awsClient.EXPECT().FindUserByEmail("stale@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "stale@example.com", Active: true}, nil)
	awsClient.EXPECT().DeleteUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.Username == "stale@example.com"
	})).Return(nil)

	assert.NoError(t, s.LegacySyncUsers(""))
}

func TestLegacySyncUsers_Unmatched_RetainUnmatched(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups", RetainUnmatched: true}

	awsSdk := []types.User{sdkUser("aws-1", "stale@example.com", "Stale", "User", "gid-stale")}

	s, awsClient, _ := newLegacySyncUsersSuite(t, cfg, []*admin.User{}, awsSdk)

	// RetainUnmatched -> deletion suppressed. Active is still backfilled.
	// No DeleteUser expectation: mockery fails on any unexpected call.
	awsClient.EXPECT().FindUserByEmail("stale@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "stale@example.com", Active: true}, nil)

	assert.NoError(t, s.LegacySyncUsers(""))
}

// newLegacySyncGroupsSuite wires up a syncGSuite for LegacySyncGroups tests. It stubs the
// google groups lookup and the Identity Store ListGroups call that
// s.GetGroups() drives. s.users is left empty so member sync is a no-op.
func newLegacySyncGroupsSuite(t *testing.T, cfg *config.Config, googleGroups []*admin.Group, awsSdkGroups []types.Group) (*syncGSuite, *mocks.MockAwsClient, *mocks.MockGoogleClient) {
	t.Helper()

	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups(mock.Anything).Return(googleGroups, nil)

	identityStore := mocks.NewMockIdentityStoreAPI(t)
	identityStore.EXPECT().ListGroups(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupsOutput{Groups: awsSdkGroups}, nil)

	awsClient := mocks.NewMockAwsClient(t)

	s := &syncGSuite{
		aws:           awsClient,
		google:        googleClient,
		identityStore: identityStore,
		cfg:           cfg,
		users:         make(map[string]*interfaces.User),
	}
	return s, awsClient, googleClient
}

func TestLegacySyncGroups_ExternalIdMatch_UpdateDisplayName(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}
	// Matched by ExternalId but the display name differs -> update.
	awsSdk := []types.Group{sdkGroup("aws-1", "old-name@example.com", "gid-1")}

	s, awsClient, googleClient := newLegacySyncGroupsSuite(t, cfg, google, awsSdk)
	googleClient.EXPECT().GetGroupMembers(google[0]).Return([]*admin.Member{}, nil)

	awsClient.EXPECT().UpdateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-1" && g.DisplayName == "group@example.com" && g.ExternalId.Id == "gid-1"
	})).Return(&interfaces.Group{ID: "aws-1", DisplayName: "group@example.com", ExternalId: interfaces.ExternalId{Id: "gid-1"}}, nil)

	assert.NoError(t, s.LegacySyncGroups(""))
}

func TestLegacySyncGroups_DisplayNameMatch_BackfillExternalId(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}
	// Matched by display name, no ExternalId -> update to backfill it.
	awsSdk := []types.Group{sdkGroup("aws-1", "group@example.com", "")}

	s, awsClient, googleClient := newLegacySyncGroupsSuite(t, cfg, google, awsSdk)
	googleClient.EXPECT().GetGroupMembers(google[0]).Return([]*admin.Member{}, nil)

	awsClient.EXPECT().UpdateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-1" && g.ExternalId.Id == "gid-1"
	})).Return(&interfaces.Group{ID: "aws-1", DisplayName: "group@example.com", ExternalId: interfaces.ExternalId{Id: "gid-1"}}, nil)

	assert.NoError(t, s.LegacySyncGroups(""))
}

func TestLegacySyncGroups_NoMatch_Create(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}

	s, awsClient, googleClient := newLegacySyncGroupsSuite(t, cfg, google, []types.Group{})
	googleClient.EXPECT().GetGroupMembers(google[0]).Return([]*admin.Member{}, nil)

	awsClient.EXPECT().CreateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.DisplayName == "group@example.com" && g.ExternalId.Id == "gid-1"
	})).Return(&interfaces.Group{ID: "aws-new", DisplayName: "group@example.com", ExternalId: interfaces.ExternalId{Id: "gid-1"}}, nil)

	assert.NoError(t, s.LegacySyncGroups(""))
}

func TestLegacySyncGroups_Unmatched_Delete(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1"}

	// No google groups; one existing AWS group -> delete.
	awsSdk := []types.Group{sdkGroup("aws-1", "stale@example.com", "gid-stale")}

	s, awsClient, _ := newLegacySyncGroupsSuite(t, cfg, []*admin.Group{}, awsSdk)

	awsClient.EXPECT().DeleteGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-1" && g.DisplayName == "stale@example.com"
	})).Return(nil)

	assert.NoError(t, s.LegacySyncGroups(""))
}

func TestLegacySyncGroups_Unmatched_RetainUnmatched(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", RetainUnmatched: true}

	awsSdk := []types.Group{sdkGroup("aws-1", "stale@example.com", "gid-stale")}

	// RetainUnmatched -> no DeleteGroup expectation; mockery fails on unexpected calls.
	s, _, _ := newLegacySyncGroupsSuite(t, cfg, []*admin.Group{}, awsSdk)

	assert.NoError(t, s.LegacySyncGroups(""))
}

// --- LegacySyncUsers: deleted google user pruning ---

// newLegacySyncUsersDeleteSuite wires a syncGSuite where GetDeletedUsers returns the
// supplied deleted users and both GetUsers calls return activeGoogleUsers. The
// Identity Store ListUsers (driven by s.GetUsers()) returns awsSdkUsers.
func newLegacySyncUsersDeleteSuite(t *testing.T, cfg *config.Config, deletedUsers, activeGoogleUsers []*admin.User, awsSdkUsers []types.User) (*syncGSuite, *mocks.MockAwsClient) {
	t.Helper()

	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetDeletedUsers().Return(deletedUsers, nil)
	googleClient.EXPECT().GetUsers(mock.Anything, mock.Anything).Return(activeGoogleUsers, nil)

	identityStore := mocks.NewMockIdentityStoreAPI(t)
	identityStore.EXPECT().ListUsers(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{Users: awsSdkUsers}, nil)

	awsClient := mocks.NewMockAwsClient(t)

	s := &syncGSuite{
		aws:           awsClient,
		google:        googleClient,
		identityStore: identityStore,
		cfg:           cfg,
		users:         make(map[string]*interfaces.User),
	}
	return s, awsClient
}

func TestLegacySyncUsers_DeletedUser_Deleted(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	deleted := []*admin.User{gUser("gid-del", "gone@example.com", "Gone", "User", false)}

	// No active google users, and no existing AWS users to correlate.
	s, awsClient := newLegacySyncUsersDeleteSuite(t, cfg, deleted, []*admin.User{}, []types.User{})

	// The deleted google user still exists in AWS -> looked up and deleted.
	deletedAwsUser := &interfaces.User{ID: "aws-del", Username: "gone@example.com"}
	awsClient.EXPECT().FindUserByEmail("gone@example.com").Return(deletedAwsUser, nil)
	awsClient.EXPECT().DeleteUser(deletedAwsUser).Return(nil)

	assert.NoError(t, s.LegacySyncUsers(""))
}

func TestLegacySyncUsers_DeletedUser_ActiveAgain_NotDeleted(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// The user appears in both the deleted list and the active list -> it was
	// re-activated, so it must NOT be deleted.
	active := []*admin.User{gUser("gid-1", "user@example.com", "Active", "Again", false)}
	deleted := []*admin.User{gUser("gid-1", "user@example.com", "Active", "Again", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Active", "Again", "gid-1")}

	s, awsClient := newLegacySyncUsersDeleteSuite(t, cfg, deleted, active, awsSdk)

	// Active status backfill + setUser re-fetch; no DeleteUser for the deletion
	// pruning (mockery fails on any unexpected DeleteUser call).
	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)

	assert.NoError(t, s.LegacySyncUsers(""))
}

func TestLegacySyncUsers_DeletedUser_AlreadyGone(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	deleted := []*admin.User{gUser("gid-del", "gone@example.com", "Gone", "User", false)}

	s, awsClient := newLegacySyncUsersDeleteSuite(t, cfg, deleted, []*admin.User{}, []types.User{})

	// The deleted google user is already absent from AWS -> no DeleteUser call.
	awsClient.EXPECT().FindUserByEmail("gone@example.com").Return(nil, awsinternal.ErrUserNotFound)

	assert.NoError(t, s.LegacySyncUsers(""))
}

// --- LegacySyncGroups: member add/remove ---

func TestLegacySyncGroups_MemberAdded(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}
	awsSdk := []types.Group{sdkGroup("aws-1", "group@example.com", "gid-1")}

	s, awsClient, googleClient := newLegacySyncGroupsSuite(t, cfg, google, awsSdk)

	// A synced user that is a google member but not yet an AWS group member.
	user := &interfaces.User{ID: "user-1", Username: "member@example.com"}
	s.users["member@example.com"] = user

	googleClient.EXPECT().GetGroupMembers(google[0]).
		Return([]*admin.Member{{Email: "member@example.com"}}, nil)

	// Group matched by ExternalId with the same display name -> no UpdateGroup.
	// Not currently a member -> AddUserToGroup.
	s.identityStore.(*mocks.MockIdentityStoreAPI).EXPECT().
		IsMemberInGroups(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.IsMemberInGroupsOutput{
			Results: []types.GroupMembershipExistenceResult{{MembershipExists: false}},
		}, nil)
	awsClient.EXPECT().AddUserToGroup(user, mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-1"
	})).Return(nil)

	assert.NoError(t, s.LegacySyncGroups(""))
}

func TestLegacySyncGroups_MemberRemoved(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}
	awsSdk := []types.Group{sdkGroup("aws-1", "group@example.com", "gid-1")}

	s, awsClient, googleClient := newLegacySyncGroupsSuite(t, cfg, google, awsSdk)

	// A synced user who is an existing AWS group member but no longer a google member.
	user := &interfaces.User{ID: "user-1", Username: "stale@example.com"}
	s.users["stale@example.com"] = user

	// No google members for this group.
	googleClient.EXPECT().GetGroupMembers(google[0]).Return([]*admin.Member{}, nil)

	// Currently a member -> RemoveUserFromGroup.
	s.identityStore.(*mocks.MockIdentityStoreAPI).EXPECT().
		IsMemberInGroups(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.IsMemberInGroupsOutput{
			Results: []types.GroupMembershipExistenceResult{{MembershipExists: true}},
		}, nil)
	awsClient.EXPECT().RemoveUserFromGroup(user, mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-1"
	})).Return(nil)

	assert.NoError(t, s.LegacySyncGroups(""))
}

// Aggregate-level tests exercising the managed-vs-foreign distinction with a
// non-empty configured issuerArn. The configured (ssosync) issuer differs from
// the foreign issuer carried on the AWS objects, so the managed guard is false
// and the objects take the foreign path.

// Foreign unmatched (no Google counterpart) with retainUnmatched=true -> retain.
func TestGetUserOperations_ForeignIssuer_UnmatchedRetain(t *testing.T) {
	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	awsUsers := []*interfaces.User{
		{
			ID:       "A8",
			Username: "user8@example.com",
			ExternalId: interfaces.ExternalId{
				Id:     "G8",
				Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
			},
			Name: interfaces.UserName{
				GivenName:  "Carol",
				FamilyName: "White",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, true, issuerArn), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.False(t, update[0].Active)
}

// Foreign unmatched with retainUnmatched=false -> delete.
func TestGetUserOperations_ForeignIssuer_UnmatchedDelete(t *testing.T) {
	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	awsUsers := []*interfaces.User{
		{
			ID:       "A8",
			Username: "user8@example.com",
			ExternalId: interfaces.ExternalId{
				Id:     "G8",
				Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
			},
			Name: interfaces.UserName{
				GivenName:  "Carol",
				FamilyName: "White",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, issuerArn), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
}

// Foreign matched (Google counterpart exists, ExternalId.Id == Google Id,
// Issuer differs from issuerArn) -> AdoptForeign -> update.
func TestGetUserOperations_ForeignIssuer_MatchedAdoptForeign(t *testing.T) {
	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	awsUsers := []*interfaces.User{
		{
			ID:       "A8",
			Username: "user8@example.com",
			ExternalId: interfaces.ExternalId{
				Id:     "G8",
				Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
			},
			Name: interfaces.UserName{
				GivenName:  "Carol",
				FamilyName: "White",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G8",
			PrimaryEmail: "user8@example.com",
			Name: &admin.UserName{
				GivenName:  "Carol",
				FamilyName: "White",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, issuerArn), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "G8", update[0].ExternalId.Id)
}

// Managed positive control: AWS Issuer == configured issuerArn, fully matching
// the Google user -> Equal.
func TestGetUserOperations_ManagedIssuer_Equals(t *testing.T) {
	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	awsUsers := []*interfaces.User{
		{
			ID:       "A8",
			Username: "user8@example.com",
			ExternalId: interfaces.ExternalId{
				Id:     "G8",
				Issuer: issuerArn,
			},
			Name: interfaces.UserName{
				GivenName:  "Carol",
				FamilyName: "White",
			},
			DisplayName: "Carol White",
			Active:      true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G8",
			PrimaryEmail: "user8@example.com",
			Name: &admin.UserName{
				GivenName:  "Carol",
				FamilyName: "White",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, issuerArn), config.DefaultUserAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Equal(t, "G8", equals[0].ExternalId.Id)
}

// Foreign unmatched group with retainUnmatched=true -> retain.
func TestGetGroupOperations_ForeignIssuer_UnmatchedRetain(t *testing.T) {
	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	awsGroups := []*interfaces.Group{
		{
			ID:          "A8",
			DisplayName: "Group 8",
			ExternalId: interfaces.ExternalId{
				Id:     "G8",
				Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
			},
		},
	}

	googleGroups := []*admin.Group{}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, true, issuerArn), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 8", update[0].DisplayName)
}

// Foreign unmatched group with retainUnmatched=false -> delete.
func TestGetGroupOperations_ForeignIssuer_UnmatchedDelete(t *testing.T) {
	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	awsGroups := []*interfaces.Group{
		{
			ID:          "A8",
			DisplayName: "Group 8",
			ExternalId: interfaces.ExternalId{
				Id:     "G8",
				Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
			},
		},
	}

	googleGroups := []*admin.Group{}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, false, issuerArn), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
}

// Foreign matched group (Google counterpart exists, ExternalId.Id == Google Id,
// Issuer differs from issuerArn) -> AdoptForeign -> update.
func TestGetGroupOperations_ForeignIssuer_MatchedAdoptForeign(t *testing.T) {
	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	awsGroups := []*interfaces.Group{
		{
			ID:          "A8",
			DisplayName: "Group 8",
			ExternalId: interfaces.ExternalId{
				Id:     "G8",
				Issuer: "arn:aws:identitystore::111111111111:identitystore/d-other/provisioningtenant/other",
			},
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G8",
			Name:  "Group 8",
			Email: "group8@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, false, issuerArn), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "G8", update[0].ExternalId.Id)
}

// Managed positive control: AWS group Issuer == configured issuerArn, fully
// matching the Google group -> Equal.
func TestGetGroupOperations_ManagedIssuer_Equals(t *testing.T) {
	issuerArn := "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/ssosync"

	awsGroups := []*interfaces.Group{
		{
			ID:          "A8",
			DisplayName: "Group 8",
			ExternalId: interfaces.ExternalId{
				Id:     "G8",
				Issuer: issuerArn,
			},
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G8",
			Name:  "Group 8",
			Email: "group8@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups, opsConfig(false, false, issuerArn), config.DefaultGroupAttributeMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Equal(t, "G8", equals[0].ExternalId.Id)
}

// TestGetUserOperations_CustomAttrMap_ChangesMatching proves that the discrete
// attrMap parameter is actually used end-to-end: remapping Username away from
// primaryEmail to name.givenName changes which AWS user the Google user matches
// on the Username-fallback path. The AWS user has no ExternalId, so matching can
// only happen via Username.
func TestGetUserOperations_CustomAttrMap_ChangesMatching(t *testing.T) {
	// AWS user whose Username equals the Google user's GivenName (lowercased),
	// but NOT the Google user's primaryEmail.
	awsUsers := []*interfaces.User{
		{
			ID:       "A99",
			Username: "bob",
			Name: interfaces.UserName{
				GivenName:  "Bob",
				FamilyName: "Smith",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G99",
			PrimaryEmail: "bob.smith@example.com",
			Name: &admin.UserName{
				GivenName:  "Bob",
				FamilyName: "Smith",
			},
			Suspended: false,
		},
	}

	// Custom map: Username is sourced from name.givenName instead of primaryEmail.
	customMap := config.AttributeMap{
		"Name.GivenName":  "name.givenName",
		"Name.FamilyName": "name.familyName",
		"Username":        "name.givenName",
		"Active":          "suspended",
	}

	// With the custom map the mapped Username == "Bob", which matches the AWS
	// user "bob" on the Username-fallback path: the local user is adopted.
	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), customMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "G99", update[0].ExternalId.Id)
	assert.Equal(t, "Bob", update[0].Username)

	// Contrast: with the DEFAULT map the mapped Username == primaryEmail
	// ("bob.smith@example.com"), which does NOT match the AWS user "bob", so the
	// Google user is created and the unmatched local AWS user is deleted.
	add, delete, update, equals = getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, "G99", add[0].ExternalId.Id)
	assert.Equal(t, "bob.smith@example.com", add[0].Username)
	assert.Equal(t, "A99", delete[0].ID)

	// Third sub-scenario (item 6a): an AWS user already synced with
	// Username == "Bob" and the Google ExternalId set. Running the same
	// Username <- name.givenName custom map must classify Equal with an empty
	// update: an already-synced user whose Username equals the mapped value does
	// not churn.
	syncedAwsUsers := []*interfaces.User{
		{
			ID:          "A99",
			Username:    "Bob",
			ExternalId:  interfaces.ExternalId{Id: "G99"},
			Name:        interfaces.UserName{GivenName: "Bob", FamilyName: "Smith"},
			DisplayName: "Bob Smith",
			Active:      true,
		},
	}

	add, delete, update, equals = getUserOperations(syncedAwsUsers, googleUsers, opsConfig(false, false, ""), customMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "already-synced user whose Username equals the mapped givenName must not churn")
	assert.Len(t, equals, 1)
	assert.Equal(t, "G99", equals[0].ExternalId.Id)
	assert.Equal(t, "Bob", equals[0].Username)
}

// optInMap builds a custom AttributeMap from the four default scalar entries
// (Name.GivenName, Name.FamilyName, Username, Active) plus the single opt-in
// target<-source pair under test, so matching still works via Username while the
// one opt-in multi-valued/enterprise field is exercised at the getUserOperations
// level.
func optInMap(target, source string) config.AttributeMap {
	m := config.AttributeMap{
		"Name.GivenName":  "name.givenName",
		"Name.FamilyName": "name.familyName",
		"Username":        "primaryEmail",
		"Active":          "suspended",
	}
	m[target] = source
	return m
}

// --- Item 5: getUserOperations-level opt-in coverage (enrichment + steady-state) ---

func TestGetUserOperations_OptInEmails_Enrichment(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			// Only the synthesized work email the Identity Store returns; no
			// opt-in email yet.
			Emails:    []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses: []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Emails: []admin.UserEmail{
				{Address: "john.personal@example.com", Type: "home", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Emails", "emails[primary].address"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		if assert.Len(t, update[0].Emails, 1) {
			assert.Equal(t, "john.personal@example.com", update[0].Emails[0].Value)
			assert.Equal(t, "home", update[0].Emails[0].Type)
			assert.True(t, update[0].Emails[0].Primary)
		}
	}
}

func TestGetUserOperations_OptInEmails_SteadyState_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			// The opt-in email is already present AWS-side, as the mapper would
			// shape it (single element, Primary:true, type passed through).
			Emails:    []interfaces.UserEmail{{Value: "john.personal@example.com", Type: "home", Primary: true}},
			Addresses: []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Emails: []admin.UserEmail{
				{Address: "john.personal@example.com", Type: "home", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Emails", "emails[primary].address"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "steady-state opt-in Emails user must not churn")
	assert.Len(t, equals, 1)
}

func TestGetUserOperations_OptInAddresses_Enrichment(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Addresses: []admin.UserAddress{
				// Empty Google type -> defaults to "work".
				{Formatted: "1 Example Way", Type: "", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Addresses", "addresses[primary].formatted"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		if assert.Len(t, update[0].Addresses, 1) {
			assert.Equal(t, "1 Example Way", update[0].Addresses[0].Formatted)
			assert.Equal(t, "work", update[0].Addresses[0].Type)
			assert.True(t, update[0].Addresses[0].Primary)
		}
	}
}

func TestGetUserOperations_OptInAddresses_SteadyState_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Formatted: "1 Example Way", Type: "work", Primary: true}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Addresses: []admin.UserAddress{
				{Formatted: "1 Example Way", Type: "", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Addresses", "addresses[primary].formatted"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "steady-state opt-in Addresses user must not churn")
	assert.Len(t, equals, 1)
}

func TestGetUserOperations_OptInPhoneNumbers_Enrichment(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Phones: []admin.UserPhone{
				{Value: "+15551234567", Type: "mobile", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("PhoneNumbers", "phones[primary].value"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		if assert.Len(t, update[0].PhoneNumbers, 1) {
			assert.Equal(t, "+15551234567", update[0].PhoneNumbers[0].Value)
			assert.Equal(t, "mobile", update[0].PhoneNumbers[0].Type)
			assert.True(t, update[0].PhoneNumbers[0].Primary)
		}
	}
}

func TestGetUserOperations_OptInPhoneNumbers_SteadyState_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:           "A1",
			Username:     "user1@example.com",
			ExternalId:   interfaces.ExternalId{Id: "G1"},
			Name:         interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName:  "John Doe",
			Active:       true,
			Emails:       []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:    []interfaces.UserAddress{{Type: "work"}},
			PhoneNumbers: []interfaces.UserPhoneNumber{{Value: "+15551234567", Type: "mobile", Primary: true}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Phones: []admin.UserPhone{
				{Value: "+15551234567", Type: "mobile", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("PhoneNumbers", "phones[primary].value"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "steady-state opt-in PhoneNumbers user must not churn")
	assert.Len(t, equals, 1)
}

func TestGetUserOperations_OptInManager_Enrichment(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Relations: []admin.UserRelation{
				{Type: "assistant", Value: "asst@example.com"},
				{Type: "manager", Value: "boss@example.com"},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Enterprise.Manager", "relations[type=manager].value"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		if assert.NotNil(t, update[0].Enterprise) && assert.NotNil(t, update[0].Enterprise.Manager) {
			assert.Equal(t, "boss@example.com", update[0].Enterprise.Manager.Value)
		}
	}
}

func TestGetUserOperations_OptInManager_SteadyState_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
			Enterprise:  &interfaces.EnterpriseUser{Manager: &interfaces.EnterpriseManager{Value: "boss@example.com"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Relations: []admin.UserRelation{
				{Type: "manager", Value: "boss@example.com"},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Enterprise.Manager", "relations[type=manager].value"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "steady-state opt-in Manager user must not churn")
	assert.Len(t, equals, 1)
}

// Absent-manager variant: no type==manager relation -> Enterprise stays nil and
// there is no spurious Update (Equal against an AWS record that also lacks it).
func TestGetUserOperations_OptInManager_AbsentNoChurn(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
			// No Enterprise: the record never had a manager.
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Relations: []admin.UserRelation{
				{Type: "assistant", Value: "asst@example.com"},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Enterprise.Manager", "relations[type=manager].value"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "absent manager must not create a spurious Update")
	if assert.Len(t, equals, 1) {
		assert.Nil(t, equals[0].Enterprise)
	}
}

func TestGetUserOperations_OptInDivision_Enrichment(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Organizations: []admin.UserOrganization{
				{Domain: "sales.example.com", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Enterprise.Division", "organizations[primary].domain"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		if assert.NotNil(t, update[0].Enterprise) {
			assert.Equal(t, "sales.example.com", update[0].Enterprise.Division)
		}
	}
}

func TestGetUserOperations_OptInDivision_SteadyState_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
			Enterprise:  &interfaces.EnterpriseUser{Division: "sales.example.com"},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Organizations: []admin.UserOrganization{
				{Domain: "sales.example.com", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Enterprise.Division", "organizations[primary].domain"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "steady-state opt-in Division user must not churn")
	assert.Len(t, equals, 1)
}

// EmployeeNumber assignment coverage only: Google admin.User has no first-class
// scalar employee-number source (design §5.3), so the source here is ARBITRARY
// (organizations[primary].name) purely to prove the Enterprise.EmployeeNumber
// TARGET case resolves and is churn-free — NOT to assert semantic correctness of
// that mapping.
func TestGetUserOperations_OptInEmployeeNumber_Enrichment(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Organizations: []admin.UserOrganization{
				{Name: "ExampleCorp", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Enterprise.EmployeeNumber", "organizations[primary].name"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		if assert.NotNil(t, update[0].Enterprise) {
			assert.Equal(t, "ExampleCorp", update[0].Enterprise.EmployeeNumber)
		}
	}
}

func TestGetUserOperations_OptInEmployeeNumber_SteadyState_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
			Enterprise:  &interfaces.EnterpriseUser{EmployeeNumber: "ExampleCorp"},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Organizations: []admin.UserOrganization{
				{Name: "ExampleCorp", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Enterprise.EmployeeNumber", "organizations[primary].name"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "steady-state opt-in EmployeeNumber user must not churn")
	assert.Len(t, equals, 1)
}

func TestGetUserOperations_OptInProfileURL_Enrichment(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Websites: []admin.UserWebsite{
				{Value: "https://profile.example.com/john", Type: "profile", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("ProfileURL", "websites[primary].value"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		assert.Equal(t, "https://profile.example.com/john", update[0].ProfileURL)
	}
}

func TestGetUserOperations_OptInProfileURL_SteadyState_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			ProfileURL:  "https://profile.example.com/john",
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Websites: []admin.UserWebsite{
				{Value: "https://profile.example.com/john", Type: "profile", Primary: true},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("ProfileURL", "websites[primary].value"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "steady-state opt-in ProfileURL user must not churn")
	assert.Len(t, equals, 1)
}

// --- Item 6a: Username mapped to a non-email source, at getUserOperations level ---

// TestGetUserOperations_UsernameMappedToGivenName_DesiredPayload: a custom map
// sources Username from name.givenName. The AWS user's Username equals the
// Google GivenName (so matching happens on the Username-fallback path; the AWS
// user has no ExternalId yet), so the local user is adopted (Update), the
// ExternalId is stamped from the Google id, and the desired Username is the
// MAPPED givenName value, NOT primaryEmail.
func TestGetUserOperations_UsernameMappedToGivenName_DesiredPayload(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:       "A1",
			Username: "John",
			Name:     interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			Active:   true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "jdoe@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
		},
	}

	customMap := config.AttributeMap{
		"Name.GivenName":  "name.givenName",
		"Name.FamilyName": "name.familyName",
		"Username":        "name.givenName",
		"Active":          "suspended",
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), customMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		assert.Equal(t, "G1", update[0].ExternalId.Id)
		assert.Equal(t, "John", update[0].Username)
		assert.NotEqual(t, "jdoe@example.com", update[0].Username)
	}
}

func TestGetUserOperations_UsernameMappedToGivenName_SteadyState_Equal(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "John",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "jdoe@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
		},
	}

	customMap := config.AttributeMap{
		"Name.GivenName":  "name.givenName",
		"Name.FamilyName": "name.familyName",
		"Username":        "name.givenName",
		"Active":          "suspended",
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), customMap)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "already-synced user whose Username is sourced from givenName must not churn")
	if assert.Len(t, equals, 1) {
		assert.Equal(t, "John", equals[0].Username)
	}
}

// --- Item 6b: multiple address values at getUserOperations level ---

func TestGetUserOperations_OptInAddresses_MultiplePrimarySelected(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Addresses: []admin.UserAddress{
				{Formatted: "1 First St", Type: "home", Primary: false},
				{Formatted: "2 Primary Ave", Type: "work", Primary: true},
				{Formatted: "3 Third Rd", Type: "other", Primary: false},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Addresses", "addresses[primary].formatted"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		if assert.Len(t, update[0].Addresses, 1) {
			assert.Equal(t, "2 Primary Ave", update[0].Addresses[0].Formatted)
			assert.Equal(t, "work", update[0].Addresses[0].Type)
			assert.True(t, update[0].Addresses[0].Primary)
		}
	}
}

// Multiple addresses, NONE primary -> nothing emitted and Equal against an AWS
// record that also lacks the opt-in address (only the synthesized work address).
// FAILS against pre-fix first-fallback code; PASSES after the behaviour fix.
func TestGetUserOperations_OptInAddresses_MultipleNoPrimaryNoChurn(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	googleUsers := []*admin.User{
		{
			Id:           "G1",
			PrimaryEmail: "user1@example.com",
			Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Addresses: []admin.UserAddress{
				{Formatted: "1 First St", Type: "home", Primary: false},
				{Formatted: "2 Second Ave", Type: "work", Primary: false},
				{Formatted: "3 Third Rd", Type: "other", Primary: false},
			},
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), optInMap("Addresses", "addresses[primary].formatted"))

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0, "no primary address -> nothing selected -> no churn")
	assert.Len(t, equals, 1)
}

// --- Item 6c: randomized-order churn regression at getUserOperations level ---

// reverseAdminAddresses / rotateAdminAddresses give explicit deterministic
// orderings (reversed and a single-position rotation) for the shuffle regression
// tests — NO random seed.
func reverseAdminAddresses(in []admin.UserAddress) []admin.UserAddress {
	out := make([]admin.UserAddress, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}

func rotateAdminAddresses(in []admin.UserAddress) []admin.UserAddress {
	if len(in) < 2 {
		return append([]admin.UserAddress(nil), in...)
	}
	return append(append([]admin.UserAddress(nil), in[1:]...), in[0])
}

// With one primary address, the classification is Equal against a record already
// carrying the primary value, IDENTICALLY across every entry ordering (identity,
// reversed, rotated).
func TestGetUserOperations_ShuffledMultiValuedNoChurn(t *testing.T) {
	addrs := []admin.UserAddress{
		{Formatted: "1 First St", Type: "home", Primary: false},
		{Formatted: "2 Primary Ave", Type: "work", Primary: true},
		{Formatted: "3 Third Rd", Type: "other", Primary: false},
	}

	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Formatted: "2 Primary Ave", Type: "work", Primary: true}},
		},
	}

	build := func(a []admin.UserAddress) []*admin.User {
		return []*admin.User{
			{
				Id:           "G1",
				PrimaryEmail: "user1@example.com",
				Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
				Addresses:    a,
			},
		}
	}

	for _, tc := range []struct {
		name string
		g    []*admin.User
	}{
		{"identity", build(addrs)},
		{"reversed", build(reverseAdminAddresses(addrs))},
		{"rotated", build(rotateAdminAddresses(addrs))},
	} {
		add, delete, update, equals := getUserOperations(awsUsers, tc.g, opsConfig(false, false, ""), optInMap("Addresses", "addresses[primary].formatted"))
		assert.Len(t, add, 0, tc.name)
		assert.Len(t, delete, 0, tc.name)
		assert.Len(t, update, 0, tc.name+": primary-selected value is stable so no churn")
		assert.Len(t, equals, 1, tc.name)
	}
}

// With NO primary address in any ordering, the opt-in field resolves to nothing
// and the user is Equal against a record that also lacks it, IDENTICALLY across
// orderings. FAILS against pre-fix first-fallback code (different orderings
// select different entry [0], producing a spurious Update in some orderings);
// PASSES after the behaviour fix.
func TestGetUserOperations_ShuffledNoPrimaryNoChurn(t *testing.T) {
	addrs := []admin.UserAddress{
		{Formatted: "1 A St", Type: "home", Primary: false},
		{Formatted: "2 B St", Type: "work", Primary: false},
		{Formatted: "3 C St", Type: "other", Primary: false},
	}

	awsUsers := []*interfaces.User{
		{
			ID:          "A1",
			Username:    "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe",
			Active:      true,
			Emails:      []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:   []interfaces.UserAddress{{Type: "work"}},
		},
	}

	build := func(a []admin.UserAddress) []*admin.User {
		return []*admin.User{
			{
				Id:           "G1",
				PrimaryEmail: "user1@example.com",
				Name:         &admin.UserName{GivenName: "John", FamilyName: "Doe"},
				Addresses:    a,
			},
		}
	}

	for _, tc := range []struct {
		name string
		g    []*admin.User
	}{
		{"identity", build(addrs)},
		{"reversed", build(reverseAdminAddresses(addrs))},
		{"rotated", build(rotateAdminAddresses(addrs))},
	} {
		add, delete, update, equals := getUserOperations(awsUsers, tc.g, opsConfig(false, false, ""), optInMap("Addresses", "addresses[primary].formatted"))
		assert.Len(t, add, 0, tc.name)
		assert.Len(t, delete, 0, tc.name)
		assert.Len(t, update, 0, tc.name+": no primary -> nothing selected -> no churn in any ordering")
		assert.Len(t, equals, 1, tc.name)
	}
}

// --- run-178: type-qualified + custom-schema getUserOperations coverage ---

func reverseAdminOrgs(in []admin.UserOrganization) []admin.UserOrganization {
	out := make([]admin.UserOrganization, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}

func rotateAdminOrgs(in []admin.UserOrganization) []admin.UserOrganization {
	if len(in) < 2 {
		return append([]admin.UserOrganization(nil), in...)
	}
	return append(append([]admin.UserOrganization(nil), in[1:]...), in[0])
}

// Default map [primary]: a Google user whose primary org is untyped (as the
// Admin console stores it) enriches an AWS record that does not yet carry
// Title/Enterprise (Update), carrying the primary org's values; a work-typed
// org that is not primary is ignored.
func TestGetUserOperations_DefaultMapPrimary_Enrichment(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID: "A1", Username: "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe", Active: true,
		},
	}
	googleUsers := []*admin.User{
		{
			Id: "G1", PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Organizations: []admin.UserOrganization{
				{Title: "Other", Type: "work", Primary: false},
				{Title: "Staff Engineer", Department: "Platform", Name: "ExampleCorp", CostCenter: "CC-100", Type: "", Primary: true},
			},
		},
	}

	add, del, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)
	assert.Len(t, add, 0)
	assert.Len(t, del, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		assert.Equal(t, "Staff Engineer", update[0].Title)
		if assert.NotNil(t, update[0].Enterprise) {
			assert.Equal(t, "Platform", update[0].Enterprise.Department)
		}
	}
}

// Default map [primary]: a user with no primary org resolves to NOTHING (no
// fallback to the first entry, even a work-typed one); an AWS record that
// carried Title/Enterprise is cleared (Update), and once AWS reflects the
// clear the user is Equal (steady state).
func TestGetUserOperations_DefaultMapPrimary_NoPrimaryOrgClears(t *testing.T) {
	googleUsers := []*admin.User{
		{
			Id: "G1", PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			Organizations: []admin.UserOrganization{
				// Typed work but NOT primary -> under [primary] resolves to nothing.
				{Title: "Staff Engineer", Department: "Platform", Type: "work", Primary: false},
			},
		},
	}

	// AWS record still carries the previously-enriched values.
	awsPopulated := []*interfaces.User{
		{
			ID: "A1", Username: "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe", Active: true,
			Title:      "Staff Engineer",
			Enterprise: &interfaces.EnterpriseUser{Department: "Platform"},
			Emails:     []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:  []interfaces.UserAddress{{Type: "work"}},
		},
	}

	add, del, update, equals := getUserOperations(awsPopulated, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)
	assert.Len(t, add, 0)
	assert.Len(t, del, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		assert.Equal(t, "", update[0].Title, "no primary org clears Title under [primary]")
		assert.Nil(t, update[0].Enterprise, "Enterprise cleared to nil")
	}

	// Steady state: AWS now reflects the cleared record -> Equal, no churn.
	awsCleared := []*interfaces.User{
		{
			ID: "A1", Username: "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe", Active: true,
			Emails:    []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses: []interfaces.UserAddress{{Type: "work"}},
		},
	}
	add, del, update, equals = getUserOperations(awsCleared, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)
	assert.Len(t, add, 0)
	assert.Len(t, del, 0)
	assert.Len(t, update, 0, "cleared steady-state user must not churn")
	assert.Len(t, equals, 1)
}

// Shuffled multi-valued order with exactly one primary org produces no
// spurious Update in any ordering (churn-freedom for the default [primary]
// selector).
func TestGetUserOperations_PrimaryShuffledNoChurn(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID: "A1", Username: "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe", Active: true,
			Title:      "Staff Engineer",
			Enterprise: &interfaces.EnterpriseUser{Department: "Platform", Organization: "ExampleCorp", CostCenter: "CC-100"},
			Emails:     []interfaces.UserEmail{{Value: "user1@example.com", Type: "work", Primary: true}},
			Addresses:  []interfaces.UserAddress{{Type: "work"}},
		},
	}
	orgs := []admin.UserOrganization{
		{Title: "Other", Department: "DO", Name: "OrgO", CostCenter: "CCO", Type: "work", Primary: false},
		{Title: "Staff Engineer", Department: "Platform", Name: "ExampleCorp", CostCenter: "CC-100", Type: "", Primary: true},
		{Title: "Home", Department: "DH", Name: "OrgH", CostCenter: "CCH", Type: "home", Primary: false},
	}
	build := func(o []admin.UserOrganization) []*admin.User {
		return []*admin.User{
			{
				Id: "G1", PrimaryEmail: "user1@example.com",
				Name:          &admin.UserName{GivenName: "John", FamilyName: "Doe"},
				Organizations: o,
			},
		}
	}
	for _, tc := range []struct {
		name string
		g    []*admin.User
	}{
		{"identity", build(orgs)},
		{"reversed", build(reverseAdminOrgs(orgs))},
		{"rotated", build(rotateAdminOrgs(orgs))},
	} {
		add, del, update, equals := getUserOperations(awsUsers, tc.g, opsConfig(false, false, ""), config.DefaultUserAttributeMap)
		assert.Len(t, add, 0, tc.name)
		assert.Len(t, del, 0, tc.name)
		assert.Len(t, update, 0, tc.name+": stable primary selection -> no churn")
		assert.Len(t, equals, 1, tc.name)
	}
}

// Custom-schema source at the getUserOperations level: a user with a custom
// schema field enriches, then is Equal once AWS reflects it.
func TestGetUserOperations_CustomSchemaSource_EnrichThenSteady(t *testing.T) {
	custMap := config.AttributeMap{
		"Name.GivenName":        "name.givenName",
		"Name.FamilyName":       "name.familyName",
		"Username":              "primaryEmail",
		"Active":                "suspended",
		"Enterprise.CostCenter": "customSchemas.EmploymentData.costCenter",
	}
	googleUsers := []*admin.User{
		{
			Id: "G1", PrimaryEmail: "user1@example.com",
			Name:          &admin.UserName{GivenName: "John", FamilyName: "Doe"},
			CustomSchemas: map[string]googleapi.RawMessage{"EmploymentData": googleapi.RawMessage(`{"costCenter":"CC-777"}`)},
		},
	}

	awsUsers := []*interfaces.User{
		{
			ID: "A1", Username: "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe", Active: true,
		},
	}
	add, del, update, equals := getUserOperations(awsUsers, googleUsers, opsConfig(false, false, ""), custMap)
	assert.Len(t, add, 0)
	assert.Len(t, del, 0)
	assert.Len(t, equals, 0)
	if assert.Len(t, update, 1) {
		if assert.NotNil(t, update[0].Enterprise) {
			assert.Equal(t, "CC-777", update[0].Enterprise.CostCenter)
		}
	}

	awsEnriched := []*interfaces.User{
		{
			ID: "A1", Username: "user1@example.com",
			ExternalId:  interfaces.ExternalId{Id: "G1"},
			Name:        interfaces.UserName{GivenName: "John", FamilyName: "Doe"},
			DisplayName: "John Doe", Active: true,
			Enterprise: &interfaces.EnterpriseUser{CostCenter: "CC-777"},
		},
	}
	add, del, update, equals = getUserOperations(awsEnriched, googleUsers, opsConfig(false, false, ""), custMap)
	assert.Len(t, add, 0)
	assert.Len(t, del, 0)
	assert.Len(t, update, 0, "custom-schema steady state must not churn")
	assert.Len(t, equals, 1)
}

// #421: Release optional NickName and Name.Formatted mappings (PR #412)

func TestGetUserOperations_OptInNames(t *testing.T) {
	attrMap, err := config.ParseAttributeMaps([]byte(`{"user":{
		"Username":"primaryEmail", "Active":"suspended",
		"Name.GivenName":"name.givenName", "Name.FamilyName":"name.familyName",
		"NickName":"name.givenName", "Name.Formatted":"name.fullName"
	}}`))
	require.NoError(t, err)
	googleUsers := []*admin.User{{
		Id: "G1", PrimaryEmail: "jane@example.com",
		Name: &admin.UserName{GivenName: "Jane", FamilyName: "Doe", FullName: "Dr. Jane Q. Doe"},
	}}
	current := &interfaces.User{
		ID: "A1", Username: "jane@example.com", Active: true,
		ExternalId: interfaces.ExternalId{Id: "G1"},
		Name:       interfaces.UserName{GivenName: "Jane", FamilyName: "Doe"}, DisplayName: "Jane Doe",
	}
	add, del, update, equal := getUserOperations([]*interfaces.User{current}, googleUsers, opsConfig(false, false, ""), attrMap.User)
	require.Len(t, update, 1)
	assert.Empty(t, add)
	assert.Empty(t, del)
	assert.Empty(t, equal)
	assert.Equal(t, "A1", update[0].ID)
	assert.Equal(t, "Jane", update[0].NickName)
	assert.Equal(t, "Dr. Jane Q. Doe", update[0].Name.Formatted)
	assert.Equal(t, "Jane Doe", update[0].DisplayName)
	payload, err := json.Marshal(update[0])
	require.NoError(t, err)
	var wire map[string]interface{}
	require.NoError(t, json.Unmarshal(payload, &wire))
	assert.Equal(t, "Jane", wire["nickName"])
	assert.Equal(t, "Dr. Jane Q. Doe", wire["name"].(map[string]interface{})["formatted"])

	add, del, changes, equal := getUserOperations(update, googleUsers, opsConfig(false, false, ""), attrMap.User)
	assert.Empty(t, add)
	assert.Empty(t, del)
	assert.Empty(t, changes)
	assert.Len(t, equal, 1)

	for _, field := range []string{"nickname", "formatted name"} {
		t.Run(field+" changed", func(t *testing.T) {
			stale := *update[0]
			if field == "nickname" {
				stale.NickName = "old nickname"
			} else {
				stale.Name.Formatted = "old formatted name"
			}
			_, _, changes, equal := getUserOperations([]*interfaces.User{&stale}, googleUsers, opsConfig(false, false, ""), attrMap.User)
			require.Len(t, changes, 1)
			assert.Empty(t, equal)
			assert.Equal(t, "Jane", changes[0].NickName)
			assert.Equal(t, "Dr. Jane Q. Doe", changes[0].Name.Formatted)
		})
	}

	_, _, changes, _ = getUserOperations(update, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)
	require.Len(t, changes, 1)
	assert.Empty(t, changes[0].NickName)
	assert.Empty(t, changes[0].Name.Formatted)
}

// #401: BUG: Sync fails for all users when a group member's email contains an apostrophe (`googleapi: Error 400: Invalid Input: query, invalid`)

// issue401Config is the groups sync config used by the #401 tests, with the
// UserFilter cmd/root.go sets when SyncSuspended is false.
func issue401Config() *config.Config {
	return &config.Config{
		IdentityStoreID: "d-1",
		SyncMethod:      "groups",
		UserFilter:      " isSuspended=false isArchived=false",
		AttributeMap: config.AttributeMaps{
			User:  config.DefaultUserAttributeMap,
			Group: config.DefaultGroupAttributeMap,
		},
	}
}

func TestQuoteGoogleQueryValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"plain value", "jane@example.com", `'jane@example.com'`},
		{"apostrophe", "jane.o'brien@example.com", `'jane.o\'brien@example.com'`},
		{"space", "/Sales Team", `'/Sales Team'`},
		{"space and apostrophe", "Valentine's Day", `'Valentine\'s Day'`},
		{"several apostrophes", "a'b'c", `'a\'b\'c'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, quoteGoogleQueryValue(tt.value))
		})
	}
}

// TestSync_Issue401_ApostropheEmailMemberSynced covers awslabs/ssosync#401: a
// group member whose email contains an apostrophe is looked up with a quoted,
// escaped users.list query (the filter is still appended server-side by
// getUsers as subQuery+filter) and is synced like any other member.
func TestSync_Issue401_ApostropheEmailMemberSynced(t *testing.T) {
	cfg := issue401Config()

	googleClient := mocks.NewMockGoogleClient(t)
	group := &admin.Group{Id: "gg1", Name: "Group1", Email: "g1@example.com"}
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{group}, nil)
	googleClient.EXPECT().GetGroupMembers(group).Return([]*admin.Member{
		{Email: "jane.o'brien@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil)

	// Capture the exact query string handed to the Google client.
	var gotQuery, gotFilter string
	googleClient.EXPECT().GetUsers(`email='jane.o\'brien@example.com'`, cfg.UserFilter).
		Run(func(query string, filter string) {
			gotQuery, gotFilter = query, filter
		}).
		Return([]*admin.User{gUser("gid-1", "jane.o'brien@example.com", "Jane", "O'Brien", false)}, nil).Once()

	// Jane already exists in the Identity Store (matched by ExternalId); the
	// group is new, so it is created and Jane is added to it.
	identityStore := mocks.NewMockIdentityStoreAPI(t)
	identityStore.EXPECT().ListGroups(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupsOutput{}, nil)
	identityStore.EXPECT().ListUsers(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{Users: []types.User{
			sdkUser("is-1", "jane.o'brien@example.com", "Jane", "O'Brien", "gid-1"),
		}}, nil)

	awsClient := mocks.NewMockAwsClient(t)
	awsClient.EXPECT().FindUserByEmail("jane.o'brien@example.com").
		Return(&interfaces.User{ID: "is-1", Username: "jane.o'brien@example.com", Active: true}, nil)
	awsClient.EXPECT().CreateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.DisplayName == "Group1"
	})).Return(&interfaces.Group{ID: "grp-1", DisplayName: "Group1"}, nil)
	awsClient.EXPECT().AddUserToGroup(
		mock.MatchedBy(func(u *interfaces.User) bool { return u.ID == "is-1" }),
		mock.MatchedBy(func(g *interfaces.Group) bool { return g.ID == "grp-1" }),
	).Return(nil).Once()

	s := &syncGSuite{
		aws:           awsClient,
		google:        googleClient,
		identityStore: identityStore,
		cfg:           cfg,
		users:         make(map[string]*interfaces.User),
	}

	require.NoError(t, s.Sync("group-q", ""))

	assert.Equal(t, `email='jane.o\'brien@example.com'`, gotQuery)
	assert.Equal(t, `email='jane.o\'brien@example.com' isSuspended=false isArchived=false`, gotQuery+gotFilter)
}

// TestSync_Issue401_MemberLookupErrorFailsRunAndLogsContext pins the agreed
// error handling: a failed member lookup still fails the run immediately (the
// next member is never looked up and no AWS call is made), and the error log
// names the member, the group and the exact query with the filter.
func TestSync_Issue401_MemberLookupErrorFailsRunAndLogsContext(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	cfg := issue401Config()
	googleErr := &googleapi.Error{Code: 400, Message: "Invalid Input: query, invalid"}

	googleClient := mocks.NewMockGoogleClient(t)
	group := &admin.Group{Id: "gg1", Name: "Group1", Email: "g1@example.com"}
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{group}, nil)
	googleClient.EXPECT().GetGroupMembers(group).Return([]*admin.Member{
		{Email: "jane.o'brien@example.com", Type: "USER", Status: "ACTIVE"},
		{Email: "bob@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil)
	googleClient.EXPECT().GetUsers(`email='jane.o\'brien@example.com'`, cfg.UserFilter).
		Return(nil, googleErr).Once()
	// No expectation for bob: the strict mock fails the test if he is looked up.
	// No Identity Store or AWS expectations: Sync must abort before reaching them.

	s := &syncGSuite{
		aws:           mocks.NewMockAwsClient(t),
		google:        googleClient,
		identityStore: mocks.NewMockIdentityStoreAPI(t),
		cfg:           cfg,
		users:         make(map[string]*interfaces.User),
	}

	err := s.Sync("group-q", "")
	require.Error(t, err)
	var gerr *googleapi.Error
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, 400, gerr.Code)

	var entry *log.Entry
	for _, e := range hook.AllEntries() {
		if e.Level == log.ErrorLevel && e.Data["func"] == "getGoogleUsersInGroup" {
			entry = e
			break
		}
	}
	require.NotNil(t, entry, "expected an error log entry for the failed member lookup")
	assert.Equal(t, "jane.o'brien@example.com", entry.Data["Member.Email"])
	assert.Equal(t, "gg1", entry.Data["GroupId"])
	assert.Equal(t, "g1@example.com", entry.Data["GroupEmail"])
	assert.Equal(t, `email='jane.o\'brien@example.com' isSuspended=false isArchived=false`, entry.Data["query"])
	assert.Equal(t, googleErr, entry.Data["error"])
}

// TestGetGoogleGroupsAndUsers_Issue401_PrecacheOrgUnitQueriesQuoted checks the
// PrecacheOrgUnits queries: every OU path is single-quoted with ' escaped, and
// each OU gets its own query (#404), so no query starts with a comma (the
// strict mock fails any call not listed here).
func TestGetGoogleGroupsAndUsers_Issue401_PrecacheOrgUnitQueriesQuoted(t *testing.T) {
	cfg := issue401Config()
	cfg.PrecacheOrgUnits = []string{"/", " /O'Brien Team/ ", "/Sales"}

	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil)
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/'`, cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/O\'Brien Team'`, cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/Sales'`, cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetGroups("group-q").Return(nil, nil)

	s := &syncGSuite{
		google: googleClient,
		cfg:    cfg,
		users:  make(map[string]*interfaces.User),
	}

	_, _, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
}

// TestOutputCacheMetrics_Issue401_OrgUnitQueryQuoted checks the cache-metrics
// OU query: the OU path is single-quoted with ' escaped.
func TestOutputCacheMetrics_Issue401_OrgUnitQueryQuoted(t *testing.T) {
	cfg := issue401Config()
	cfg.CacheMetrics = true

	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().CountUsers(`OrgUnitPath='/O\'Brien Team'`, cfg.UserFilter).
		Return(2, nil).Once()

	entry := &CacheMetrics{}
	s := &syncGSuite{
		google:       googleClient,
		cfg:          cfg,
		cacheMetrics: map[string]*CacheMetrics{"/O'Brien Team": entry},
	}

	s.outputCacheMetrics()
	assert.Equal(t, 2, entry.TotalUsers)
}

// #404: PrecacheOrgUnits query starts with a comma, likely caching all users

// Log messages of the PrecacheOrgUnits user precache (#404).
const (
	msg404OUQueryFailed = "PrecacheOrgUnits: OrgUnitPath could not be queried and is skipped; it may not exist"
	msg404OUNoUsers     = "PrecacheOrgUnits: no users found in OrgUnitPath; it may not exist or have no users matching the filter"
	msg404AllFailed     = "Precaching failed, caching on the fly"
	msg404NoUsers       = "Precaching return no users? Switching to caching on the fly"
)

// issue404Users are two users in different OUs, members of issue404Group.
func issue404Users() (a, b *admin.User) {
	a = gUser("gid-a", "a@example.com", "Ann", "A", false)
	a.OrgUnitPath = "/A"
	b = gUser("gid-b", "b@example.com", "Bob", "B", false)
	b.OrgUnitPath = "/B"
	return a, b
}

var issue404Group = &admin.Group{Id: "gg1", Name: "Group1", Email: "g1@example.com"}

// issue404Mock returns a strict Google mock that serves the group precache
// and one group (group-q) with members a@ and b@. Each test adds the per-OU
// GetUsers calls, plus any email= lookups of members that were not precached.
func issue404Mock(t *testing.T) *mocks.MockGoogleClient {
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{issue404Group}, nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return([]*admin.Member{
		{Email: "a@example.com", Type: "USER", Status: "ACTIVE"},
		{Email: "b@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil).Once()
	return googleClient
}

func issue404Sync(googleClient *mocks.MockGoogleClient, orgUnits []string) *syncGSuite {
	cfg := issue401Config()
	cfg.PrecacheOrgUnits = orgUnits
	return &syncGSuite{
		google: googleClient,
		cfg:    cfg,
		users:  make(map[string]*interfaces.User),
	}
}

// issue404Entries returns the log entries with the given level and message.
func issue404Entries(hook *test.Hook, level log.Level, msg string) []*log.Entry {
	var found []*log.Entry
	for _, e := range hook.AllEntries() {
		if e.Level == level && e.Message == msg {
			found = append(found, e)
		}
	}
	return found
}

func issue404OUs(entries []*log.Entry) []interface{} {
	ous := make([]interface{}, 0, len(entries))
	for _, e := range entries {
		ous = append(ous, e.Data["orgUnitPath"])
	}
	return ous
}

// Only the listed OUs are queried, one call each, and their users are cached:
// no member is looked up on the fly and nothing is warned.
func TestGetGoogleGroupsAndUsers_Issue404_PrecachesOnlyListedOrgUnits(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()
	// No email= expectations: the strict mock fails the test if a member is
	// looked up on the fly.

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	for _, e := range hook.AllEntries() {
		assert.NotEqual(t, log.WarnLevel, e.Level, "unexpected warning: %s", e.Message)
		assert.NotEqual(t, log.ErrorLevel, e.Level, "unexpected error: %s", e.Message)
	}
}

// An OU that cannot be queried is skipped with a warning naming it; the other
// OU is still precached and the run carries on.
func TestGetGoogleGroupsAndUsers_Issue404_OrgUnitErrorSkippedWithWarning(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleErr := &googleapi.Error{Code: 400, Message: "Invalid Input: INVALID_OU_ID"}
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(nil, googleErr).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()
	// a@ was not precached, so it is looked up on the fly; b@ was.
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	warns := issue404Entries(hook, log.WarnLevel, msg404OUQueryFailed)
	require.Len(t, warns, 1)
	assert.Equal(t, "/A", warns[0].Data["orgUnitPath"])
	assert.Equal(t, googleErr, warns[0].Data["error"])
	assert.Equal(t, "getGoogleGroupsAndUsers", warns[0].Data["func"])
	assert.Empty(t, issue404Entries(hook, log.ErrorLevel, msg404AllFailed))
	assert.Empty(t, issue404Entries(hook, log.WarnLevel, msg404NoUsers))
}

// An OU that returns no users is skipped with a warning naming it; the other
// OU is still precached.
func TestGetGoogleGroupsAndUsers_Issue404_OrgUnitNoUsersSkippedWithWarning(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{}, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	warns := issue404Entries(hook, log.WarnLevel, msg404OUNoUsers)
	require.Len(t, warns, 1)
	assert.Equal(t, "/A", warns[0].Data["orgUnitPath"])
	assert.Equal(t, s.cfg.UserFilter, warns[0].Data["queryFilters"])
	assert.Equal(t, "getGoogleGroupsAndUsers", warns[0].Data["func"])
	assert.Empty(t, issue404Entries(hook, log.WarnLevel, msg404OUQueryFailed))
	assert.Empty(t, issue404Entries(hook, log.WarnLevel, msg404NoUsers))
}

// When every OU returns no users, each is warned about and the existing
// "no users" warning is logged, but not "Precaching failed".
func TestGetGoogleGroupsAndUsers_Issue404_AllOrgUnitsEmpty(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers(`email='b@example.com'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()

	_, _, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)

	assert.ElementsMatch(t, []interface{}{"/A", "/B"}, issue404OUs(issue404Entries(hook, log.WarnLevel, msg404OUNoUsers)))
	assert.Len(t, issue404Entries(hook, log.WarnLevel, msg404NoUsers), 1)
	assert.Empty(t, issue404Entries(hook, log.ErrorLevel, msg404AllFailed))
}

// When every OU fails, each is warned about and "Precaching failed, caching on
// the fly" is logged; members are looked up on the fly and the run carries on.
func TestGetGoogleGroupsAndUsers_Issue404_AllOrgUnitsFail(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleErr := &googleapi.Error{Code: 400, Message: "Invalid Input: INVALID_OU_ID"}
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(nil, googleErr).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return(nil, googleErr).Once()
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers(`email='b@example.com'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	assert.ElementsMatch(t, []interface{}{"/A", "/B"}, issue404OUs(issue404Entries(hook, log.WarnLevel, msg404OUQueryFailed)))
	assert.Len(t, issue404Entries(hook, log.ErrorLevel, msg404AllFailed), 1)
	assert.Empty(t, issue404Entries(hook, log.WarnLevel, msg404NoUsers))
}

// Overlapping OUs return the same user twice: the first is recorded as a
// PreCache and the repeat as a CacheHit, each counted once (#413).
func TestGetGoogleGroupsAndUsers_Issue404_OverlappingOrgUnitsRepeatIsCacheHit(t *testing.T) {
	a, _ := issue404Users()
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	s := issue404Sync(googleClient, []string{"/", "/A"})
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()

	_, _, _, err := s.getGoogleGroupsAndUsers("", "")
	require.NoError(t, err)

	require.Contains(t, s.cacheMetrics, "/A")
	assert.Equal(t, CacheMetrics{Precaches: 1, Hits: 1}, *s.cacheMetrics["/A"])
}

// Empty entries are skipped and an OU listed twice (after normalising) is
// queried once.
func TestGetGoogleGroupsAndUsers_Issue404_EmptyAndDuplicateOrgUnitsSkipped(t *testing.T) {
	a, _ := issue404Users()
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	s := issue404Sync(googleClient, []string{"/A", "", "  ", " /A/ "})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()

	_, _, _, err := s.getGoogleGroupsAndUsers("", "")
	require.NoError(t, err)
}

// Without PrecacheOrgUnits nothing is precached: no GetGroups("*") and no
// GetUsers call.
func TestGetGoogleGroupsAndUsers_Issue404_NilPrecacheOrgUnitsDisabled(t *testing.T) {
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue404Sync(googleClient, nil)

	_, _, _, err := s.getGoogleGroupsAndUsers("", "")
	require.NoError(t, err)
}

// #413: CacheMetrics: recordCacheOp double-counts every op and matches parent OUs by substring

const msg413CacheStats = "Caching Performance, orgPaths with a high Miss rate compared with the TotalUsers consider precaching"

// issue413Sync is a syncGSuite with --cache-metrics set.
func issue413Sync(googleClient *mocks.MockGoogleClient) *syncGSuite {
	cfg := issue401Config()
	cfg.CacheMetrics = true
	return &syncGSuite{google: googleClient, cfg: cfg}
}

// Each op is counted once on its own OU; without --cache-metrics nothing is kept.
func TestRecordCacheOp_Issue413_CountsOncePerOp(t *testing.T) {
	s := issue413Sync(nil)
	s.recordCacheOp("/Sales", PreCache)
	s.recordCacheOp("/Sales", CacheHit)
	s.recordCacheOp("/Sales", CacheHit)
	s.recordCacheOp("/Sales", CacheMiss)
	require.Contains(t, s.cacheMetrics, "/Sales")
	assert.Equal(t, CacheMetrics{Precaches: 1, Hits: 2, Misses: 1}, *s.cacheMetrics["/Sales"])

	off := &syncGSuite{cfg: issue401Config()}
	off.recordCacheOp("/Sales", CacheHit)
	assert.Nil(t, off.cacheMetrics)
}

// Only the user's own OU is updated: no credit to /Sales from /Sales/EMEA or
// /Sales Team, whatever the order the OUs are first seen in.
func TestRecordCacheOp_Issue413_StoresOwnCounts(t *testing.T) {
	s := issue413Sync(nil)
	s.recordCacheOp("/Sales", CacheHit)
	s.recordCacheOp("/Sales/EMEA", CacheMiss)
	s.recordCacheOp("/Sales Team", CacheMiss)

	assert.Equal(t, map[string]*CacheMetrics{
		"/Sales":      {Hits: 1},
		"/Sales/EMEA": {Misses: 1},
		"/Sales Team": {Misses: 1},
	}, s.cacheMetrics)
}

func TestOrgUnitInTree_Issue413(t *testing.T) {
	tests := []struct {
		ou, orgPath string
		want        bool
	}{
		{"/", "/", true},
		{"/", "/X", true},
		{"/Sales", "/Sales", true},
		{"/Sales", "/Sales/EMEA", true},
		{"/Sales/", "/Sales/EMEA", true},
		{"/Sales", "/Sales Team", false},
		{"/Sales", "/Presales", false},
		{"/Sales", "/B/Sales", false},
		{"/Sales/EMEA", "/Sales", false},
		{"/A", "/AB", false},
	}
	for _, tt := range tests {
		t.Run(tt.ou+" holds "+tt.orgPath, func(t *testing.T) {
			assert.Equal(t, tt.want, orgUnitInTree(tt.ou, tt.orgPath))
		})
	}
}

// Each OU's counts are its own plus its sub-OUs' (by path, not by substring);
// TotalUsers is copied, not summed; the input is left as it was.
func TestRollUpCacheMetrics_Issue413(t *testing.T) {
	own := map[string]*CacheMetrics{
		"/":           {Hits: 1},
		"/Sales":      {Misses: 1, TotalUsers: 7},
		"/Sales/EMEA": {Precaches: 1, Hits: 2},
		"/Sales Team": {Misses: 4},
		"/Presales":   {Hits: 8},
		"/B/Sales":    {Misses: 16},
	}
	ownBefore := map[string]CacheMetrics{}
	for k, v := range own {
		ownBefore[k] = *v
	}

	assert.Equal(t, map[string]*CacheMetrics{
		"/":           {Precaches: 1, Hits: 11, Misses: 21},
		"/Sales":      {Precaches: 1, Hits: 2, Misses: 1, TotalUsers: 7},
		"/Sales/EMEA": {Precaches: 1, Hits: 2},
		"/Sales Team": {Misses: 4},
		"/Presales":   {Hits: 8},
		"/B/Sales":    {Misses: 16},
	}, rollUpCacheMetrics(own))

	for k, v := range own {
		assert.Equal(t, ownBefore[k], *v, k)
	}
}

// The default_map run (see the plan's worked example): Chris in /, Rimmer in
// /Maintenance, Dave in /Maintenance/Vending Machines, Olaf precached in
// /Red Dwarf/Petersen's Locker; /Catering is empty, so never a key, and
// /Red Dwarf has no users of its own, so it is not reported.
func TestRollUpCacheMetrics_Issue413_TestUsers(t *testing.T) {
	own := map[string]*CacheMetrics{
		"/":                             {Hits: 4, Misses: 1},
		"/Maintenance":                  {Hits: 4, Misses: 1},
		"/Maintenance/Vending Machines": {Hits: 2, Misses: 1},
		"/Red Dwarf/Petersen's Locker":  {Precaches: 1, Hits: 1},
	}
	assert.Equal(t, map[string]*CacheMetrics{
		"/":                             {Precaches: 1, Hits: 11, Misses: 3},
		"/Maintenance":                  {Hits: 6, Misses: 2},
		"/Maintenance/Vending Machines": {Hits: 2, Misses: 1},
		"/Red Dwarf/Petersen's Locker":  {Precaches: 1, Hits: 1},
	}, rollUpCacheMetrics(own))
}

// A member that had to be looked up counts as a CacheMiss only; a member found
// in the user cache counts as a CacheHit.
func TestGetGoogleUsersInGroup_Issue413_LookedUpMemberIsMissOnly(t *testing.T) {
	a, b := issue404Users()
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return([]*admin.Member{
		{Email: "a@example.com", Type: "USER", Status: "ACTIVE"},
		{Email: "b@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil).Once()
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	userCache := map[string]*admin.User{b.PrimaryEmail: b}

	members, err := s.getGoogleUsersInGroup(issue404Group, userCache, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, members)

	assert.Equal(t, map[string]*CacheMetrics{
		"/A": {Misses: 1},
		"/B": {Hits: 1},
	}, s.cacheMetrics)
}

// The Cache Stats log field carries the rolled-up counts with each OU's
// TotalUsers; s.cacheMetrics keeps the own counts.
func TestOutputCacheMetrics_Issue413_LogsRolledUpCounts(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	s.cacheMetrics = map[string]*CacheMetrics{"/": {Hits: 1}, "/A": {Misses: 1}}
	googleClient.EXPECT().CountUsers(`OrgUnitPath='/'`, s.cfg.UserFilter).
		Return(3, nil).Once()
	googleClient.EXPECT().CountUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).
		Return(1, nil).Once()

	s.outputCacheMetrics()

	var stats []*log.Entry
	for _, e := range hook.AllEntries() {
		if e.Level == log.InfoLevel && e.Message == msg413CacheStats {
			stats = append(stats, e)
		}
	}
	require.Len(t, stats, 1)
	assert.Equal(t, map[string]*CacheMetrics{
		"/":  {Hits: 1, Misses: 1, TotalUsers: 3},
		"/A": {Misses: 1, TotalUsers: 1},
	}, stats[0].Data["Cache Stats"])
	assert.Equal(t, 1, s.cacheMetrics["/"].Hits)
	assert.Equal(t, 0, s.cacheMetrics["/"].Misses)
}

// #414: Empty element in a comma-separated UserMatch/GroupMatch queries the whole directory

// An all-blank GroupMatch or UserMatch fails the run before ssosync reads or
// changes anything in AWS (#414): GetGroups/GetUsers return ErrAllEmptyQuery
// (pinned in internal/google), and Sync returns it. Treated as "" it would list
// no groups, and the groups sync would delete every unmatched AWS group. The
// AWS and Identity Store mocks are strict with no expectations, so any call,
// and in particular any delete, fails the test.
func TestSync_Issue414_AllBlankMatchAbortsBeforeAWS(t *testing.T) {
	tests := []struct {
		name, groups, users string
		blankUsers          bool
	}{
		{name: "group match", groups: " , "},
		{name: "user match", groups: "group-q", users: ",", blankUsers: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			googleClient := mocks.NewMockGoogleClient(t)
			s := issue404Sync(googleClient, nil)
			s.aws = mocks.NewMockAwsClient(t)
			s.identityStore = mocks.NewMockIdentityStoreAPI(t)

			if tt.blankUsers {
				googleClient.EXPECT().GetUsers(tt.users, s.cfg.UserFilter).
					Return(nil, fmt.Errorf("users %w: %q", google.ErrAllEmptyQuery, tt.users)).Once()
			} else {
				googleClient.EXPECT().GetGroups(tt.groups).
					Return(nil, fmt.Errorf("groups %w: %q", google.ErrAllEmptyQuery, tt.groups)).Once()
			}

			err := s.Sync(tt.groups, tt.users)
			require.ErrorIs(t, err, google.ErrAllEmptyQuery)
		})
	}
}

// #416: CacheMetrics: TotalUsers lookup for / lists the whole directory

const msg416CountFailed = "Failed retrieving usercache count"

// An OU that --precache-ous listed reuses that count as TotalUsers: no
// CountUsers (or second GetUsers) call, which the strict mock would fail.
func TestOutputCacheMetrics_Issue416_ReusesPrecacheCount(t *testing.T) {
	a, b := issue404Users()
	a.OrgUnitPath, b.OrgUnitPath = "/", "/"
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	s := issue404Sync(googleClient, []string{"/"})
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/'`, s.cfg.UserFilter).Return([]*admin.User{a, b}, nil).Once()

	_, _, _, err := s.getGoogleGroupsAndUsers("", "")
	require.NoError(t, err)
	s.outputCacheMetrics()

	require.Contains(t, s.cacheMetrics, "/")
	assert.Equal(t, 2, s.cacheMetrics["/"].TotalUsers)
}

// An OU that was not precached is counted with CountUsers, never GetUsers.
func TestOutputCacheMetrics_Issue416_CountsOtherOUs(t *testing.T) {
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	s.cacheMetrics = map[string]*CacheMetrics{"/A": {Misses: 1}}
	googleClient.EXPECT().CountUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(5, nil).Once()

	s.outputCacheMetrics()

	assert.Equal(t, 5, s.cacheMetrics["/A"].TotalUsers)
}

// A failed count is logged and leaves TotalUsers at 0.
func TestOutputCacheMetrics_Issue416_CountErrorLogged(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	s.cacheMetrics = map[string]*CacheMetrics{"/A": {Misses: 1}}
	googleErr := errors.New("boom")
	googleClient.EXPECT().CountUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(0, googleErr).Once()

	s.outputCacheMetrics()

	errs := issue404Entries(hook, log.ErrorLevel, msg416CountFailed)
	require.Len(t, errs, 1)
	assert.Equal(t, googleErr, errs[0].Data["error"])
	assert.Equal(t, 0, s.cacheMetrics["/A"].TotalUsers)
}

// #417: CacheMetrics: userMatch path records bulk-listed users as Misses and overwrites precached users

// --user-match fills the cache like --precache-ous (#417): a new user is a
// precache; a user already precached, or listed twice, keeps the cached entry
// and is a hit.
func TestGetGoogleGroupsAndUsers_Issue417_UserMatchIsPrecache(t *testing.T) {
	a, b := issue404Users()
	aCopy := *a
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	s := issue404Sync(googleClient, []string{"/A"})
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers("name:x", s.cfg.UserFilter).Return([]*admin.User{&aCopy, b, b}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("", "name:x")
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)
	for _, u := range users {
		assert.NotSame(t, &aCopy, u, "the precached entry is kept")
	}

	assert.Equal(t, map[string]*CacheMetrics{
		"/A": {Precaches: 1, Hits: 1},
		"/B": {Precaches: 1, Hits: 1},
	}, s.cacheMetrics)
}

// A group member that --user-match already listed is found in the cache (a
// hit, no lookup); a member it did not list is looked up (a miss).
func TestGetGoogleGroupsAndUsers_Issue417_UserMatchMemberIsHit(t *testing.T) {
	a, b := issue404Users()
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue404Sync(googleClient, nil)
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers("name:x", s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{issue404Group}, nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return([]*admin.Member{
		{Email: "a@example.com", Type: "USER", Status: "ACTIVE"},
		{Email: "b@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil).Once()
	// No email='a@example.com' expectation: the strict mock fails on a lookup.
	googleClient.EXPECT().GetUsers(`email='b@example.com'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "name:x")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	assert.Equal(t, map[string]*CacheMetrics{
		"/A": {Precaches: 1, Hits: 1},
		"/B": {Misses: 1},
	}, s.cacheMetrics)
}

// #418: Group member cache misses on case-different or alias member emails

var issue418Group2 = &admin.Group{Id: "gg2", Name: "Group2", Email: "g2@example.com"}

func issue418Members(emails ...string) []*admin.Member {
	members := make([]*admin.Member, 0, len(emails))
	for _, e := range emails {
		members = append(members, &admin.Member{Email: e, Type: "USER", Status: "ACTIVE"})
	}
	return members
}

func issue418User(email, orgUnitPath string) *admin.User {
	u := gUser("gid-"+email, email, "Given", "Family", false)
	u.OrgUnitPath = orgUnitPath
	return u
}

func TestUserCacheKey_Issue418(t *testing.T) {
	assert.Equal(t, "acehole@example.com", userCacheKey("AceHole@Example.COM"))
}

// A member listed in a different case from the primary email is found in the
// cache: no lookup (the strict mock has no GetUsers), one Hit.
func TestGetGoogleUsersInGroup_Issue418_CaseDifferentMemberHitsCache(t *testing.T) {
	u := issue418User("AceHole@example.com", "/M")
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("acehole@example.com"), nil).Once()
	userCache := map[string]*admin.User{}
	cacheUser(userCache, u)

	members, err := s.getGoogleUsersInGroup(issue404Group, userCache, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.Equal(t, []*admin.User{u}, members)
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Hits: 1}}, s.cacheMetrics)
}

// Members listed by an alias or a non-editable alias, in any case, are found
// in the cache without a lookup.
func TestGetGoogleUsersInGroup_Issue418_AliasMemberHitsCache(t *testing.T) {
	u := issue418User("acehole@example.com", "/M")
	u.Aliases = []string{"rimmer@example.com"}
	u.NonEditableAliases = []string{"acehole@alias.example.com"}
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("Rimmer@example.com"), nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue418Group2).Return(issue418Members("acehole@alias.example.com"), nil).Once()
	userCache := map[string]*admin.User{}
	cacheUser(userCache, u)

	for _, g := range []*admin.Group{issue404Group, issue418Group2} {
		members, err := s.getGoogleUsersInGroup(g, userCache, map[string]*admin.Group{})
		require.NoError(t, err)
		assert.Equal(t, []*admin.User{u}, members, g.Name)
	}
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Hits: 2}}, s.cacheMetrics)
}

// A cached user listed by an address Google does not report as an alias is
// looked up once (one Miss), and the cached entry is kept; the address is then
// keyed, so the next membership is a Hit with no lookup.
func TestGetGoogleUsersInGroup_Issue418_LookupRecordedOnceAsMiss(t *testing.T) {
	a := issue418User("a@example.com", "/A")
	aAgain := issue418User("a@example.com", "/A")
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("old@example.com"), nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue418Group2).Return(issue418Members("old@example.com"), nil).Once()
	googleClient.EXPECT().GetUsers(`email='old@example.com'`, s.cfg.UserFilter).Return([]*admin.User{aAgain}, nil).Once()
	userCache := map[string]*admin.User{}
	cacheUser(userCache, a)

	for _, g := range []*admin.Group{issue404Group, issue418Group2} {
		members, err := s.getGoogleUsersInGroup(g, userCache, map[string]*admin.Group{})
		require.NoError(t, err)
		require.Len(t, members, 1, g.Name)
		assert.Same(t, a, members[0], g.Name)
	}
	assert.Equal(t, map[string]*CacheMetrics{"/A": {Misses: 1, Hits: 1}}, s.cacheMetrics)
}

// A mixed-case member that is not cached is looked up once: exactly one Miss
// and no Hit; its next membership is exactly one Hit.
func TestGetGoogleUsersInGroup_Issue418_OneOpPerMembership(t *testing.T) {
	u := issue418User("acehole@example.com", "/M")
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("AceHole@example.com"), nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue418Group2).Return(issue418Members("ACEHOLE@example.com"), nil).Once()
	googleClient.EXPECT().GetUsers(`email='AceHole@example.com'`, s.cfg.UserFilter).Return([]*admin.User{u}, nil).Once()
	userCache := map[string]*admin.User{}

	members, err := s.getGoogleUsersInGroup(issue404Group, userCache, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.Equal(t, []*admin.User{u}, members)
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Misses: 1}}, s.cacheMetrics)

	members, err = s.getGoogleUsersInGroup(issue418Group2, userCache, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.Equal(t, []*admin.User{u}, members)
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Misses: 1, Hits: 1}}, s.cacheMetrics)
}

// A precached mixed-case primary email is found for a lower-case member: no
// email= lookup, and the user is returned once.
func TestGetGoogleGroupsAndUsers_Issue418_PrecachedMixedCaseNoLookup(t *testing.T) {
	u := issue418User("AceHole@example.com", "/M")
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{issue404Group}, nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("acehole@example.com"), nil).Once()
	s := issue404Sync(googleClient, []string{"/M"})
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/M'`, s.cfg.UserFilter).Return([]*admin.User{u}, nil).Once()

	_, users, groupsUsers, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Same(t, u, users[0])
	assert.Equal(t, []*admin.User{u}, groupsUsers[issue404Group.Name])
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Precaches: 1, Hits: 1}}, s.cacheMetrics)
}

// A mixed-case --ignore-users entry (lower-cased when the config loads) still
// ignores the member, whatever case the group lists it in, with no lookup.
func TestIgnoreUser_Issue418_MixedCaseEntry(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	s.cfg.IgnoreUsers = config.LowerCaseAll([]string{"AceHole@Example.com"})
	googleClient.EXPECT().GetGroupMembers(issue404Group).
		Return(issue418Members("acehole@example.com", "ACEHOLE@example.com"), nil).Once()

	members, err := s.getGoogleUsersInGroup(issue404Group, map[string]*admin.User{}, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.Empty(t, members)
	assert.Len(t, issue404Entries(hook, log.InfoLevel, "skip: ignore list"), 2)
}
