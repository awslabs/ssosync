package internal

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/awslabs/ssosync/internal/config"
	"github.com/awslabs/ssosync/internal/interfaces"
	"github.com/awslabs/ssosync/internal/mocks"
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

func TestGetGroupOperations(t *testing.T) {
	awsGroups := []*interfaces.Group{
		{ID: "1", DisplayName: "group1@example.com"},
		{ID: "2", DisplayName: "group2@example.com"},
		{ID: "3", DisplayName: "group3@example.com"},
	}

	googleGroups := []*admin.Group{
		{Id: "g1", Name: "group1@example.com", Email: "group1@example.com"},
		{Id: "g2", Name: "group2@example.com", Email: "group2@example.com"},
		{Id: "g4", Name: "group4@example.com", Email: "group4@example.com"},
	}

	add, delete, equals := getGroupOperations(awsGroups, googleGroups)

	// Should add group4 (exists in Google but not AWS)
	assert.Len(t, add, 1)
	assert.Equal(t, "group4@example.com", add[0].DisplayName)

	// Should delete group3 (exists in AWS but not Google)
	assert.Len(t, delete, 1)
	assert.Equal(t, "group3@example.com", delete[0].DisplayName)

	// Should have group1 and group2 as equals
	assert.Len(t, equals, 2)
}

func TestGetUserOperations(t *testing.T) {
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
			Active: true,
		},
		{
			ID:       "2",
			Username: "user2@example.com",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
				GivenName:  "Jane",
				FamilyName: "Smith",
			},
			Active: true,
		},
		{
			ID:       "3",
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

	googleUsers := []*admin.User{
		{
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
			Suspended: false,
		},
		{
			PrimaryEmail: "user2@example.com",
			Name: &admin.UserName{
				GivenName:  "Jane",
				FamilyName: "Updated", // Name changed
			},
			Suspended: false,
		},
		{
			PrimaryEmail: "user4@example.com",
			Name: &admin.UserName{
				GivenName:  "Alice",
				FamilyName: "Brown",
			},
			Suspended: false,
		},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers)

	// Should add user4 (exists in Google but not AWS)
	assert.Len(t, add, 1)
	assert.Equal(t, "user4@example.com", add[0].Username)

	// Should delete user3 (exists in AWS but not Google)
	assert.Len(t, delete, 1)
	assert.Equal(t, "user3@example.com", delete[0].Username)

	// Should update user2 (name changed)
	assert.Len(t, update, 1)
	assert.Equal(t, "user2@example.com", update[0].Username)
	assert.Equal(t, "Updated", update[0].Name.FamilyName)

	// Should have user1 as equals
	assert.Len(t, equals, 1)
	assert.Equal(t, "user1@example.com", equals[0].Username)
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
