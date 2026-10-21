package internal

import (
	"context"
	"testing"

	awsinternal "ssosync/internal/aws"
	"ssosync/internal/config"
	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	admin "google.golang.org/api/admin/directory/v1"
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
	}

	addUsers, deleteUsers, unchangedUsers, err := s.getGroupUsersOperations(googleGroupsUsers, awsGroupsUsers)
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
}
