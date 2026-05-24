package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/awslabs/ssosync/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDryIdentityStore(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)

	dryStore := NewDryIdentityStore(mockClient)

	assert.NotNil(t, dryStore)
	_, ok := dryStore.(*DryIdentityStore)
	assert.True(t, ok, "Expected DryIdentityStore type")
}

func TestDryIdentityStore_CreateGroup(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()
	input := &identitystore.CreateGroupInput{
		IdentityStoreId: aws.String("d-123456789"),
		DisplayName:     aws.String("Test Group"),
	}

	// Underlying client must not be called.
	result, err := dryStore.CreateGroup(ctx, input)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, virtualGroupID("Test Group"), *result.GroupId)
	assert.True(t, isVirtualID(*result.GroupId))
	assert.Equal(t, input.IdentityStoreId, result.IdentityStoreId)
}

func TestDryIdentityStore_CreateGroupMembership(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()
	input := &identitystore.CreateGroupMembershipInput{
		IdentityStoreId: aws.String("d-123456789"),
		GroupId:         aws.String("group-123"),
		MemberId:        &types.MemberIdMemberUserId{Value: "user-123"},
	}

	result, err := dryStore.CreateGroupMembership(ctx, input)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, virtualMembershipID("group-123", "user-123"), *result.MembershipId)
	assert.True(t, isVirtualID(*result.MembershipId))
	assert.Equal(t, input.IdentityStoreId, result.IdentityStoreId)
}

func TestDryIdentityStore_DeleteGroup(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()
	input := &identitystore.DeleteGroupInput{
		IdentityStoreId: aws.String("d-123456789"),
		GroupId:         aws.String("group-123"),
	}

	result, err := dryStore.DeleteGroup(ctx, input)

	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDryIdentityStore_DeleteUser(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()
	input := &identitystore.DeleteUserInput{
		IdentityStoreId: aws.String("d-123456789"),
		UserId:          aws.String("user-123"),
	}

	result, err := dryStore.DeleteUser(ctx, input)

	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDryIdentityStore_IsMemberInGroups_VirtualUser_ShortCircuits(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()
	virtualUID := virtualUserID("newuser@example.com")
	realGroupID := "12345678-1234-1234-1234-123456789012"

	input := &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: aws.String("d-123456789"),
		GroupIds:        []string{realGroupID},
		MemberId:        &types.MemberIdMemberUserId{Value: virtualUID},
	}

	// The underlying client must NOT be called.
	result, err := dryStore.IsMemberInGroups(ctx, input)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].MembershipExists, "virtual user must not be a member")
	assert.Equal(t, realGroupID, *result.Results[0].GroupId)
}

func TestDryIdentityStore_IsMemberInGroups_VirtualGroup_ShortCircuits(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()
	virtualGID := virtualGroupID("NewGroup")
	realUID := "12345678-1234-1234-1234-123456789012"

	input := &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: aws.String("d-123456789"),
		GroupIds:        []string{virtualGID},
		MemberId:        &types.MemberIdMemberUserId{Value: realUID},
	}

	result, err := dryStore.IsMemberInGroups(ctx, input)

	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.False(t, result.Results[0].MembershipExists)
}

func TestDryIdentityStore_IsMemberInGroups_RealIDs_PassesThrough(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()
	input := &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: aws.String("d-123456789"),
		GroupIds:        []string{"group-real"},
		MemberId:        &types.MemberIdMemberUserId{Value: "user-real"},
	}
	expected := &identitystore.IsMemberInGroupsOutput{
		Results: []types.GroupMembershipExistenceResult{
			{GroupId: aws.String("group-real"), MembershipExists: true},
		},
	}

	mockClient.EXPECT().IsMemberInGroups(ctx, input).Return(expected, nil).Once()

	result, err := dryStore.IsMemberInGroups(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestDryIdentityStore_GetGroupMembershipId_VirtualMember_ShortCircuits(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()
	virtualUID := virtualUserID("new@example.com")
	groupID := "12345678-1234-1234-1234-123456789012"

	input := &identitystore.GetGroupMembershipIdInput{
		IdentityStoreId: aws.String("d-123456789"),
		GroupId:         aws.String(groupID),
		MemberId:        &types.MemberIdMemberUserId{Value: virtualUID},
	}

	// Underlying client must NOT be called.
	result, err := dryStore.GetGroupMembershipId(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, virtualMembershipID(groupID, virtualUID), *result.MembershipId)
}

func TestDryIdentityStore_ListGroupMemberships_VirtualGroup_ShortCircuits(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()
	input := &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: aws.String("d-123456789"),
		GroupId:         aws.String(virtualGroupID("NewGroup")),
	}

	// Underlying client must NOT be called.
	result, err := dryStore.ListGroupMemberships(ctx, input)

	require.NoError(t, err)
	assert.Empty(t, result.GroupMemberships)
}

func TestDryIdentityStore_PassThroughMethods(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()

	// Test IsMemberInGroups passes through for real IDs
	isMemberInput := &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: aws.String("d-123456789"),
		GroupIds:        []string{"group-123"},
		MemberId:        &types.MemberIdMemberUserId{Value: "user-123"},
	}
	expectedOutput := &identitystore.IsMemberInGroupsOutput{
		Results: []types.GroupMembershipExistenceResult{
			{GroupId: aws.String("group-123"), MembershipExists: true},
		},
	}

	mockClient.EXPECT().IsMemberInGroups(ctx, isMemberInput).Return(expectedOutput, nil).Once()

	result, err := dryStore.IsMemberInGroups(ctx, isMemberInput)
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, result)

	// Test ListGroups passes through to underlying client
	listGroupsInput := &identitystore.ListGroupsInput{
		IdentityStoreId: aws.String("d-123456789"),
	}
	expectedGroupsOutput := &identitystore.ListGroupsOutput{
		Groups: []types.Group{
			{GroupId: aws.String("group-123"), DisplayName: aws.String("Test Group")},
		},
	}

	mockClient.EXPECT().ListGroups(ctx, listGroupsInput).Return(expectedGroupsOutput, nil).Once()

	groupsResult, err := dryStore.ListGroups(ctx, listGroupsInput)
	require.NoError(t, err)
	assert.Equal(t, expectedGroupsOutput, groupsResult)
}
