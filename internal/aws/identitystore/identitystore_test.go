package identitystore

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/awslabs/ssosync/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestCreateGroup(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	displayName := "Test Group"

	expectedOutput := &identitystore.CreateGroupOutput{
		GroupId:         aws.String("group-123"),
		IdentityStoreId: aws.String(identityStoreID),
	}

	mockClient.EXPECT().CreateGroup(ctx, &identitystore.CreateGroupInput{
		IdentityStoreId: &identityStoreID,
		DisplayName:     &displayName,
	}).Return(expectedOutput, nil)

	result, err := CreateGroup(ctx, mockClient, &identityStoreID, &displayName)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestCreateGroupMembership(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"
	userID := "user-456"

	expectedOutput := &identitystore.CreateGroupMembershipOutput{
		MembershipId:    aws.String("membership-789"),
		IdentityStoreId: aws.String(identityStoreID),
	}

	mockClient.EXPECT().CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
		MemberId: &types.MemberIdMemberUserId{
			Value: userID,
		},
	}).Return(expectedOutput, nil)

	result, err := CreateGroupMembership(ctx, mockClient, &identityStoreID, &groupID, &userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestDeleteGroup(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"

	expectedOutput := &identitystore.DeleteGroupOutput{}

	mockClient.EXPECT().DeleteGroup(ctx, &identitystore.DeleteGroupInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
	}).Return(expectedOutput, nil)

	result, err := DeleteGroup(ctx, mockClient, &identityStoreID, &groupID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestDeleteUser(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	userID := "user-456"

	expectedOutput := &identitystore.DeleteUserOutput{}

	mockClient.EXPECT().DeleteUser(ctx, &identitystore.DeleteUserInput{
		IdentityStoreId: &identityStoreID,
		UserId:          &userID,
	}).Return(expectedOutput, nil)

	result, err := DeleteUser(ctx, mockClient, &identityStoreID, &userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestIsMemberInGroups_True(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupIDs := []string{"group-123", "group-456"}
	userID := "user-789"

	expectedOutput := &identitystore.IsMemberInGroupsOutput{
		Results: []types.GroupMembershipExistenceResult{
			{
				GroupId:          aws.String("group-123"),
				MembershipExists: true,
			},
			{
				GroupId:          aws.String("group-456"),
				MembershipExists: false,
			},
		},
	}

	mockClient.EXPECT().IsMemberInGroups(ctx, &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: &identityStoreID,
		MemberId: &types.MemberIdMemberUserId{
			Value: userID,
		},
		GroupIds: groupIDs,
	}).Return(expectedOutput, nil)

	result, err := IsMemberInGroups(ctx, mockClient, &identityStoreID, groupIDs, &userID)

	assert.NoError(t, err)
	assert.True(t, *result)
}

func TestIsMemberInGroups_False(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupIDs := []string{"group-123", "group-456"}
	userID := "user-789"

	expectedOutput := &identitystore.IsMemberInGroupsOutput{
		Results: []types.GroupMembershipExistenceResult{
			{
				GroupId:          aws.String("group-123"),
				MembershipExists: false,
			},
			{
				GroupId:          aws.String("group-456"),
				MembershipExists: false,
			},
		},
	}

	mockClient.EXPECT().IsMemberInGroups(ctx, &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: &identityStoreID,
		MemberId: &types.MemberIdMemberUserId{
			Value: userID,
		},
		GroupIds: groupIDs,
	}).Return(expectedOutput, nil)

	result, err := IsMemberInGroups(ctx, mockClient, &identityStoreID, groupIDs, &userID)

	assert.NoError(t, err)
	assert.False(t, *result)
}

// Paginator tests are commented out due to interface compatibility issues
// between the generated mocks and the actual AWS SDK paginator types.
// These would be better tested with integration tests or by using the actual
// AWS SDK paginators in a test environment.

func TestIsMemberInGroups_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupIDs := []string{"group-123"}
	userID := "user-789"

	mockClient.EXPECT().IsMemberInGroups(ctx, &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: &identityStoreID,
		MemberId: &types.MemberIdMemberUserId{
			Value: userID,
		},
		GroupIds: groupIDs,
	}).Return(nil, assert.AnError)

	result, err := IsMemberInGroups(ctx, mockClient, &identityStoreID, groupIDs, &userID)

	assert.Error(t, err)
	assert.Nil(t, result)
}
