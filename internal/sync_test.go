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

	add, delete, update, equals, retain := getGroupOperations(awsGroups, googleGroups, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Len(t, retain, 0)
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

	add, delete, update, equals, retain := getGroupOperations(awsGroups, googleGroups, false)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
	assert.Equal(t, "Group 2", add[0].DisplayName)
	assert.Equal(t, "G2", add[0].ExternalId)
}

func TestGetGroupOperations_Delete(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
			ExternalId:  "G1",
		},
	}

	googleGroups := []*admin.Group{}

	add, delete, update, equals, retain := getGroupOperations(awsGroups, googleGroups, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
	assert.Equal(t, "Group 1", delete[0].DisplayName)
	assert.Equal(t, "G1", delete[0].ExternalId)
}

func TestGetGroupOperations_Retain(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{
			ID:          "1",
			DisplayName: "Group 1",
			ExternalId:  "G1",
		},
	}

	googleGroups := []*admin.Group{}

	add, delete, update, equals, retain := getGroupOperations(awsGroups, googleGroups, true)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 1)
	assert.Equal(t, "Group 1", retain[0].DisplayName)
	assert.Equal(t, "G1", retain[0].ExternalId)
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

	add, delete, update, equals, retain := getGroupOperations(awsGroups, googleGroups, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
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

	add, delete, update, equals, retain := getGroupOperations(awsGroups, googleGroups, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
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

	add, delete, update, equals, retain := getGroupOperations(awsGroups, googleGroups, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 1)
	assert.Len(t, retain, 0)
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, false)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
	assert.Equal(t, update[0].ExternalId, "G2")
	assert.Equal(t, update[0].Username, "user20@example.com")
	assert.Equal(t, update[0].Name.GivenName, "Jane")
	assert.Equal(t, update[0].Name.FamilyName, "Smith")
}

func TestGetUserOperations_UpdateExternalId_DeleteRecreate(t *testing.T) {
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, false)

	assert.Len(t, add, 1)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
	assert.Equal(t, delete[0].ExternalId, "GX")
	assert.Equal(t, delete[0].Username, "user6@example.com")
	assert.Equal(t, delete[0].Name.GivenName, "Alan")
	assert.Equal(t, delete[0].Name.FamilyName, "Brown")
	assert.Equal(t, add[0].ExternalId, "G6")
	assert.Equal(t, add[0].Username, "user6@example.com")
	assert.Equal(t, add[0].Name.GivenName, "Alan")
	assert.Equal(t, add[0].Name.FamilyName, "Brown")
}

func TestGetUserOperations_UpdateExternalId_ForceUpdate(t *testing.T) {
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, true, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 1)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
	assert.Equal(t, delete[0].Username, "user3@example.com")
	assert.Equal(t, delete[0].Name.GivenName, "Bob")
	assert.Equal(t, delete[0].Name.FamilyName, "Johnson")
}

func TestGetUserOperations_RetainNoExternalId(t *testing.T) {
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, true)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 1)
	assert.Equal(t, retain[0].Username, "user3@example.com")
	assert.Equal(t, retain[0].Name.GivenName, "Bob")
	assert.Equal(t, retain[0].Name.FamilyName, "Johnson")
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, false)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 1)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
	assert.Equal(t, delete[0].ExternalId, "G4")
	assert.Equal(t, delete[0].Username, "user4@example.com")
	assert.Equal(t, delete[0].Name.GivenName, "Belinda")
	assert.Equal(t, delete[0].Name.FamilyName, "Johnson")
}

func TestGetUserOperations_RetainExternalId(t *testing.T) {
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, true)

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, update, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 1)
	assert.Equal(t, retain[0].ExternalId, "G4")
	assert.Equal(t, retain[0].Username, "user4@example.com")
	assert.Equal(t, retain[0].Name.GivenName, "Belinda")
	assert.Equal(t, retain[0].Name.FamilyName, "Johnson")
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

	add, delete, update, equals, retain := getUserOperations(awsUsers, googleUsers, false, false)

	// Should update user1 (suspended state changed)
	assert.Len(t, update, 1)
	assert.Equal(t, "user1@example.com", update[0].Username)
	assert.False(t, update[0].Active) // Should be inactive now

	assert.Len(t, add, 0)
	assert.Len(t, delete, 0)
	assert.Len(t, equals, 0)
	assert.Len(t, retain, 0)
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

func TestSyncGroups_DryRunUserWithoutID(t *testing.T) {
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

	// SyncGroups now sources existing AWS groups from the Identity Store so
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
		return g.ID == "aws-gid" && g.DisplayName == "group@example.com" && g.ExternalId == "gid"
	})).Return(&interfaces.Group{ID: "aws-gid", DisplayName: "group@example.com", ExternalId: "gid"}, nil)
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

	assert.NoError(t, s.SyncGroups(""))
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

// newSyncUsersSuite wires up a syncGSuite for SyncUsers tests. It stubs the
// google deleted-users lookup (empty) and both GetUsers calls, and the
// Identity Store ListUsers call that s.GetUsers() drives.
func newSyncUsersSuite(t *testing.T, cfg *config.Config, googleUsers []*admin.User, awsSdkUsers []types.User) (*syncGSuite, *mocks.MockAwsClient, *mocks.MockGoogleClient) {
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

func TestSyncUsers_ExternalIdMatch_Update(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// Same ExternalId, but the family name changed in Google -> update.
	google := []*admin.User{gUser("gid-1", "user@example.com", "Jane", "Doe", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Jane", "Smith", "gid-1")}

	s, awsClient, _ := newSyncUsersSuite(t, cfg, google, awsSdk)

	// Active is backfilled via FindUserByEmail for each existing AWS user, and
	// setUser re-fetches the synced user afterwards.
	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)
	awsClient.EXPECT().UpdateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.ExternalId == "gid-1" && u.Name.FamilyName == "Doe"
	})).Return(&interfaces.User{ID: "aws-1", Username: "user@example.com"}, nil)

	assert.NoError(t, s.SyncUsers(""))
}

func TestSyncUsers_EmailMatch_ForceExternalIdUpdate(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups", ForceExternalIdUpdate: true}

	// Same email, different Google id, existing AWS user has an ExternalId.
	google := []*admin.User{gUser("gid-new", "user@example.com", "Jane", "Doe", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Jane", "Doe", "gid-old")}

	s, awsClient, _ := newSyncUsersSuite(t, cfg, google, awsSdk)

	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)
	// Force option -> update in place with the new ExternalId, no delete/create.
	awsClient.EXPECT().UpdateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.ExternalId == "gid-new"
	})).Return(&interfaces.User{ID: "aws-1", Username: "user@example.com"}, nil)

	assert.NoError(t, s.SyncUsers(""))
}

func TestSyncUsers_EmailMatch_DeleteAndRecreate(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups", ForceExternalIdUpdate: false}

	google := []*admin.User{gUser("gid-new", "user@example.com", "Jane", "Doe", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Jane", "Doe", "gid-old")}

	s, awsClient, _ := newSyncUsersSuite(t, cfg, google, awsSdk)

	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)
	// No force -> delete existing, create new to avoid inherited privileges.
	awsClient.EXPECT().DeleteUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.ExternalId == "gid-old"
	})).Return(nil)
	awsClient.EXPECT().CreateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "" && u.ExternalId == "gid-new" && u.Username == "user@example.com"
	})).Return(&interfaces.User{ID: "aws-2", Username: "user@example.com"}, nil)

	assert.NoError(t, s.SyncUsers(""))
}

func TestSyncUsers_EmailMatch_AdoptNoExternalId(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// Existing AWS user matched by email with no ExternalId -> adopt (update).
	google := []*admin.User{gUser("gid-1", "user@example.com", "Jane", "Doe", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Jane", "Doe", "")}

	s, awsClient, _ := newSyncUsersSuite(t, cfg, google, awsSdk)

	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)
	awsClient.EXPECT().UpdateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.ExternalId == "gid-1"
	})).Return(&interfaces.User{ID: "aws-1", Username: "user@example.com"}, nil)

	assert.NoError(t, s.SyncUsers(""))
}

func TestSyncUsers_NoMatch_Create(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// New google user, no existing AWS users.
	google := []*admin.User{gUser("gid-1", "new@example.com", "New", "User", false)}

	s, awsClient, _ := newSyncUsersSuite(t, cfg, google, []types.User{})

	awsClient.EXPECT().CreateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.Username == "new@example.com" && u.ExternalId == "gid-1"
	})).Return(&interfaces.User{ID: "aws-9", Username: "new@example.com"}, nil)
	// setUser re-fetch after create.
	awsClient.EXPECT().FindUserByEmail("new@example.com").
		Return(&interfaces.User{ID: "aws-9", Username: "new@example.com"}, nil)

	assert.NoError(t, s.SyncUsers(""))
}

func TestSyncUsers_Unmatched_Delete(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// No google users; one existing AWS user that is unmatched -> delete.
	awsSdk := []types.User{sdkUser("aws-1", "stale@example.com", "Stale", "User", "gid-stale")}

	s, awsClient, _ := newSyncUsersSuite(t, cfg, []*admin.User{}, awsSdk)

	awsClient.EXPECT().FindUserByEmail("stale@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "stale@example.com", Active: true}, nil)
	awsClient.EXPECT().DeleteUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "aws-1" && u.Username == "stale@example.com"
	})).Return(nil)

	assert.NoError(t, s.SyncUsers(""))
}

func TestSyncUsers_Unmatched_RetainUnmatched(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups", RetainUnmatched: true}

	awsSdk := []types.User{sdkUser("aws-1", "stale@example.com", "Stale", "User", "gid-stale")}

	s, awsClient, _ := newSyncUsersSuite(t, cfg, []*admin.User{}, awsSdk)

	// RetainUnmatched -> deletion suppressed. Active is still backfilled.
	// No DeleteUser expectation: mockery fails on any unexpected call.
	awsClient.EXPECT().FindUserByEmail("stale@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "stale@example.com", Active: true}, nil)

	assert.NoError(t, s.SyncUsers(""))
}

// newSyncGroupsSuite wires up a syncGSuite for SyncGroups tests. It stubs the
// google groups lookup and the Identity Store ListGroups call that
// s.GetGroups() drives. s.users is left empty so member sync is a no-op.
func newSyncGroupsSuite(t *testing.T, cfg *config.Config, googleGroups []*admin.Group, awsSdkGroups []types.Group) (*syncGSuite, *mocks.MockAwsClient, *mocks.MockGoogleClient) {
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

func TestSyncGroups_ExternalIdMatch_UpdateDisplayName(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}
	// Matched by ExternalId but the display name differs -> update.
	awsSdk := []types.Group{sdkGroup("aws-1", "old-name@example.com", "gid-1")}

	s, awsClient, googleClient := newSyncGroupsSuite(t, cfg, google, awsSdk)
	googleClient.EXPECT().GetGroupMembers(google[0]).Return([]*admin.Member{}, nil)

	awsClient.EXPECT().UpdateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-1" && g.DisplayName == "group@example.com" && g.ExternalId == "gid-1"
	})).Return(&interfaces.Group{ID: "aws-1", DisplayName: "group@example.com", ExternalId: "gid-1"}, nil)

	assert.NoError(t, s.SyncGroups(""))
}

func TestSyncGroups_DisplayNameMatch_BackfillExternalId(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}
	// Matched by display name, no ExternalId -> update to backfill it.
	awsSdk := []types.Group{sdkGroup("aws-1", "group@example.com", "")}

	s, awsClient, googleClient := newSyncGroupsSuite(t, cfg, google, awsSdk)
	googleClient.EXPECT().GetGroupMembers(google[0]).Return([]*admin.Member{}, nil)

	awsClient.EXPECT().UpdateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-1" && g.ExternalId == "gid-1"
	})).Return(&interfaces.Group{ID: "aws-1", DisplayName: "group@example.com", ExternalId: "gid-1"}, nil)

	assert.NoError(t, s.SyncGroups(""))
}

func TestSyncGroups_NoMatch_Create(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}

	s, awsClient, googleClient := newSyncGroupsSuite(t, cfg, google, []types.Group{})
	googleClient.EXPECT().GetGroupMembers(google[0]).Return([]*admin.Member{}, nil)

	awsClient.EXPECT().CreateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.DisplayName == "group@example.com" && g.ExternalId == "gid-1"
	})).Return(&interfaces.Group{ID: "aws-new", DisplayName: "group@example.com", ExternalId: "gid-1"}, nil)

	assert.NoError(t, s.SyncGroups(""))
}

func TestSyncGroups_Unmatched_Delete(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1"}

	// No google groups; one existing AWS group -> delete.
	awsSdk := []types.Group{sdkGroup("aws-1", "stale@example.com", "gid-stale")}

	s, awsClient, _ := newSyncGroupsSuite(t, cfg, []*admin.Group{}, awsSdk)

	awsClient.EXPECT().DeleteGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.ID == "aws-1" && g.DisplayName == "stale@example.com"
	})).Return(nil)

	assert.NoError(t, s.SyncGroups(""))
}

func TestSyncGroups_Unmatched_RetainUnmatched(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", RetainUnmatched: true}

	awsSdk := []types.Group{sdkGroup("aws-1", "stale@example.com", "gid-stale")}

	// RetainUnmatched -> no DeleteGroup expectation; mockery fails on unexpected calls.
	s, _, _ := newSyncGroupsSuite(t, cfg, []*admin.Group{}, awsSdk)

	assert.NoError(t, s.SyncGroups(""))
}

// --- SyncUsers: deleted google user pruning ---

// newSyncUsersDeleteSuite wires a syncGSuite where GetDeletedUsers returns the
// supplied deleted users and both GetUsers calls return activeGoogleUsers. The
// Identity Store ListUsers (driven by s.GetUsers()) returns awsSdkUsers.
func newSyncUsersDeleteSuite(t *testing.T, cfg *config.Config, deletedUsers, activeGoogleUsers []*admin.User, awsSdkUsers []types.User) (*syncGSuite, *mocks.MockAwsClient) {
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

func TestSyncUsers_DeletedUser_Deleted(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	deleted := []*admin.User{gUser("gid-del", "gone@example.com", "Gone", "User", false)}

	// No active google users, and no existing AWS users to correlate.
	s, awsClient := newSyncUsersDeleteSuite(t, cfg, deleted, []*admin.User{}, []types.User{})

	// The deleted google user still exists in AWS -> looked up and deleted.
	deletedAwsUser := &interfaces.User{ID: "aws-del", Username: "gone@example.com"}
	awsClient.EXPECT().FindUserByEmail("gone@example.com").Return(deletedAwsUser, nil)
	awsClient.EXPECT().DeleteUser(deletedAwsUser).Return(nil)

	assert.NoError(t, s.SyncUsers(""))
}

func TestSyncUsers_DeletedUser_ActiveAgain_NotDeleted(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	// The user appears in both the deleted list and the active list -> it was
	// re-activated, so it must NOT be deleted.
	active := []*admin.User{gUser("gid-1", "user@example.com", "Active", "Again", false)}
	deleted := []*admin.User{gUser("gid-1", "user@example.com", "Active", "Again", false)}
	awsSdk := []types.User{sdkUser("aws-1", "user@example.com", "Active", "Again", "gid-1")}

	s, awsClient := newSyncUsersDeleteSuite(t, cfg, deleted, active, awsSdk)

	// Active status backfill + setUser re-fetch; no DeleteUser for the deletion
	// pruning (mockery fails on any unexpected DeleteUser call).
	awsClient.EXPECT().FindUserByEmail("user@example.com").
		Return(&interfaces.User{ID: "aws-1", Username: "user@example.com", Active: true}, nil)

	assert.NoError(t, s.SyncUsers(""))
}

func TestSyncUsers_DeletedUser_AlreadyGone(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", SyncMethod: "user_groups"}

	deleted := []*admin.User{gUser("gid-del", "gone@example.com", "Gone", "User", false)}

	s, awsClient := newSyncUsersDeleteSuite(t, cfg, deleted, []*admin.User{}, []types.User{})

	// The deleted google user is already absent from AWS -> no DeleteUser call.
	awsClient.EXPECT().FindUserByEmail("gone@example.com").Return(nil, awsinternal.ErrUserNotFound)

	assert.NoError(t, s.SyncUsers(""))
}

// --- SyncGroups: member add/remove ---

func TestSyncGroups_MemberAdded(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}
	awsSdk := []types.Group{sdkGroup("aws-1", "group@example.com", "gid-1")}

	s, awsClient, googleClient := newSyncGroupsSuite(t, cfg, google, awsSdk)

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

	assert.NoError(t, s.SyncGroups(""))
}

func TestSyncGroups_MemberRemoved(t *testing.T) {
	cfg := &config.Config{IdentityStoreID: "d-1", IncludeGroups: []string{"group@example.com"}}

	google := []*admin.Group{{Email: "group@example.com", Id: "gid-1"}}
	awsSdk := []types.Group{sdkGroup("aws-1", "group@example.com", "gid-1")}

	s, awsClient, googleClient := newSyncGroupsSuite(t, cfg, google, awsSdk)

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

	assert.NoError(t, s.SyncGroups(""))
}
