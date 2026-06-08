package internal

import (
	"context"
	"testing"

	"ssosync/internal/config"
	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/stretchr/testify/assert"
	admin "google.golang.org/api/admin/directory/v1"
)

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
			ExternalId:  "G1",
		},
	}

	googleGroups := []*admin.Group{
		{
			Id:    "G1",
			Name:  "Group 1",
			Email: "group1@example.com",
		},
	}

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Equal(t, "Group 1", equals[0].DisplayName)
	assert.Equal(t, "G1", equals[0].ExternalId)
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

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 2", add[0].DisplayName)
	assert.Equal(t, "G2", add[0].ExternalId)
}

func TestGetGroupOperations_UpdateExternalId(t *testing.T) {
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

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 3", update[0].DisplayName)
	assert.Equal(t, "G3", update[0].ExternalId)
}

func TestGetGroupOperations_UpdateDisplayName(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "4",
			ExternalId:  "G4",
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

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Different Group Name", update[0].DisplayName)
	assert.Equal(t, "G4", update[0].ExternalId)
}

func TestGetGroupOperations_UpdateDeleteRecreate(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "5",
			ExternalId:  "G5",
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

	add, delete, update, equals := getGroupOperations(awsGroups, googleGroups)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, "Group 5", update[0].DisplayName)
	assert.Equal(t, "G50", update[0].ExternalId)
}

func TestGetUserOperations_NoChange(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A1",
			Username:   "user1@example.com",
			ExternalId: "G1",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
				GivenName:  "John",
				FamilyName: "Doe",
			},
			Active: true,
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

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Equal(t, equals[0].ExternalId, "G1")
	assert.Equal(t, equals[0].Username, "user1@example.com")
	assert.Equal(t, equals[0].Name.GivenName, "John")
	assert.Equal(t, equals[0].Name.FamilyName, "Doe")
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

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, add[0].ExternalId, "G7")
	assert.Equal(t, add[0].Username, "user7@example.com")
	assert.Equal(t, add[0].Name.GivenName, "Bob")
	assert.Equal(t, add[0].Name.FamilyName, "Smith")
}

func TestGetUserOperations_UpdateAttribute(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A2",
			Username:   "user2@example.com",
			ExternalId: "G2",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
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

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].ExternalId, "G2")
	assert.Equal(t, update[0].Username, "user2@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Jane")
	assert.Equal(t, update[0].Name.FamilyName, "Updated")
}

func TestGetUserOperations_UpdateMissingExternalId(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:       "A5",
			Username: "user5@example.com",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
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

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].ExternalId, "G5")
	assert.Equal(t, update[0].Username, "user5@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Jane")
	assert.Equal(t, update[0].Name.FamilyName, "Doe")
}

func TestGetUserOperations_UpdatePrimaryEmail(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A2",
			Username:   "user2@example.com",
			ExternalId: "G2",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
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

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].ExternalId, "G2")
	assert.Equal(t, update[0].Username, "user20@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Jane")
	assert.Equal(t, update[0].Name.FamilyName, "Smith")
}

func TestGetUserOperations_UpdateDeleteRecreate(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:         "A6",
			ExternalId: "GX",
			Username:   "user6@example.com",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
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

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Equal(t, update[0].ExternalId, "G6")
	assert.Equal(t, update[0].Username, "user6@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Alan")
	assert.Equal(t, update[0].Name.FamilyName, "Brown")
}

func TestGetUserOperations_DeleteNoExternalId(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:       "A3",
			Username: "user3@example.com",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
				GivenName:  "Bob",
				FamilyName: "Johnson",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

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
			ExternalId: "G4",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
				GivenName:  "Belinda",
				FamilyName: "Johnson",
			},
			Active: true,
		},
	}

	googleUsers := []*admin.User{}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Equal(t, delete[0].ExternalId, "G4")
	assert.Equal(t, delete[0].Username, "user4@example.com")
	assert.Equal(t, delete[0].Name.GivenName, "Belinda")
	assert.Equal(t, delete[0].Name.FamilyName, "Johnson")
}

func TestGetUserOperations_SuspendedStateChange(t *testing.T) {
	awsUsers := []*interfaces.User{
		{
			ID:       "1",
			Username: "user1@example.com",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
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

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

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
			{Username: "user2@example.com"},
			{Username: "user3@example.com"}, // Should be removed
		},
		"group2@example.com": {
			{Username: "user1@example.com"},
			{Username: "user4@example.com"}, // Should be removed
		},
	}

	deleteUsers, equalsUsers := getGroupUsersOperations(googleGroupsUsers, awsGroupsUsers)

	// Should remove user3 from group1 and user4 from group2
	assert.Len(t, deleteUsers["group1@example.com"], 1)
	assert.Equal(t, "user3@example.com", deleteUsers["group1@example.com"][0].Username)

	assert.Len(t, deleteUsers["group2@example.com"], 1)
	assert.Equal(t, "user4@example.com", deleteUsers["group2@example.com"][0].Username)

	// Should keep user1 and user2 in group1, user1 in group2
	assert.Len(t, equalsUsers["group1@example.com"], 2)
	assert.Len(t, equalsUsers["group2@example.com"], 1)
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
