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

	// Should not call the underlying client
	result, err := dryStore.CreateGroup(ctx, input)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Test Group-virtual", *result.GroupId)
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
	assert.Equal(t, "virtual-membership-id", *result.MembershipId)
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

func TestDryIdentityStore_PassThroughMethods(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	dryStore := NewDryIdentityStore(mockClient)

	ctx := context.Background()

	// Test IsMemberInGroups passes through to underlying client
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
