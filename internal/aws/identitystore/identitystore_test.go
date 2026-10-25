package identitystore

import (
	"context"
	"testing"

	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestGetGroupMembershipId(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"
	userID := "user-456"

	expectedOutput := &identitystore.GetGroupMembershipIdOutput{
		MembershipId:    aws.String("membership-789"),
		IdentityStoreId: aws.String(identityStoreID),
	}

	mockClient.EXPECT().GetGroupMembershipId(ctx, &identitystore.GetGroupMembershipIdInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
		MemberId:        &types.MemberIdMemberUserId{Value: userID},
	}).Return(expectedOutput, nil)

	result, err := GetGroupMembershipId(ctx, mockClient, &identityStoreID, &groupID, &userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestGetGroupMembershipId_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"
	userID := "user-456"

	mockClient.EXPECT().GetGroupMembershipId(ctx, &identitystore.GetGroupMembershipIdInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
		MemberId:        &types.MemberIdMemberUserId{Value: userID},
	}).Return(nil, assert.AnError)

	result, err := GetGroupMembershipId(ctx, mockClient, &identityStoreID, &groupID, &userID)

	assert.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, result)
}

func TestDeleteGroupMembership(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	membershipID := "membership-789"

	expectedOutput := &identitystore.DeleteGroupMembershipOutput{}

	mockClient.EXPECT().DeleteGroupMembership(ctx, &identitystore.DeleteGroupMembershipInput{
		IdentityStoreId: &identityStoreID,
		MembershipId:    &membershipID,
	}).Return(expectedOutput, nil)

	result, err := DeleteGroupMembership(ctx, mockClient, &identityStoreID, &membershipID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestDeleteGroupMembership_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	membershipID := "membership-789"

	mockClient.EXPECT().DeleteGroupMembership(ctx, &identitystore.DeleteGroupMembershipInput{
		IdentityStoreId: &identityStoreID,
		MembershipId:    &membershipID,
	}).Return(nil, assert.AnError)

	result, err := DeleteGroupMembership(ctx, mockClient, &identityStoreID, &membershipID)

	assert.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, result)
}

// groupConverter converts SDK groups to interfaces.Group, dropping groups with
// a nil DisplayName so the nil-filter branch is exercised.
func groupConverter(g types.Group) *interfaces.Group {
	if g.DisplayName == nil {
		return nil
	}
	return &interfaces.Group{ID: aws.ToString(g.GroupId), DisplayName: aws.ToString(g.DisplayName)}
}

func TestListGroups_MultiPage(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListGroups(ctx, &identitystore.ListGroupsInput{IdentityStoreId: &identityStoreID}, mock.Anything).
		Return(&identitystore.ListGroupsOutput{
			Groups: []types.Group{
				{GroupId: aws.String("g1"), DisplayName: aws.String("one")},
				{GroupId: aws.String("g2")}, // filtered out
			},
			NextToken: aws.String("page2"),
		}, nil).Once()
	mockClient.EXPECT().ListGroups(ctx, &identitystore.ListGroupsInput{IdentityStoreId: &identityStoreID, NextToken: aws.String("page2")}, mock.Anything).
		Return(&identitystore.ListGroupsOutput{
			Groups: []types.Group{{GroupId: aws.String("g3"), DisplayName: aws.String("three")}},
		}, nil).Once()

	groups, err := ListGroups(ctx, mockClient, &identityStoreID, groupConverter)

	assert.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, "g1", groups[0].ID)
	assert.Equal(t, "g3", groups[1].ID)
}

func TestListGroups_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListGroups(ctx, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	groups, err := ListGroups(ctx, mockClient, &identityStoreID, groupConverter)

	assert.Error(t, err)
	assert.Nil(t, groups)
}

func TestListGroupsPager(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListGroups(ctx, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupsOutput{
			Groups: []types.Group{
				{GroupId: aws.String("g1"), DisplayName: aws.String("one")},
				{GroupId: aws.String("g2")}, // filtered out
			},
		}, nil).Once()

	paginator := identitystore.NewListGroupsPaginator(mockClient, &identitystore.ListGroupsInput{IdentityStoreId: &identityStoreID})
	groups, err := ListGroupsPager(ctx, paginator, groupConverter)

	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "g1", groups[0].ID)
}

func TestListGroupsPager_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListGroups(ctx, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	paginator := identitystore.NewListGroupsPaginator(mockClient, &identitystore.ListGroupsInput{IdentityStoreId: &identityStoreID})
	groups, err := ListGroupsPager(ctx, paginator, groupConverter)

	assert.Error(t, err)
	assert.Nil(t, groups)
}

func TestListUsersPager(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListUsers(ctx, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{
			Users:     []types.User{{UserId: aws.String("u1"), UserName: aws.String("a")}},
			NextToken: aws.String("page2"),
		}, nil).Once()
	mockClient.EXPECT().ListUsers(ctx, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{
			Users: []types.User{{UserId: aws.String("u2")}}, // filtered out
		}, nil).Once()

	converter := func(u types.User) *interfaces.User {
		if u.UserName == nil {
			return nil
		}
		return &interfaces.User{ID: aws.ToString(u.UserId), Username: aws.ToString(u.UserName)}
	}

	paginator := identitystore.NewListUsersPaginator(mockClient, &identitystore.ListUsersInput{IdentityStoreId: &identityStoreID})
	users, err := ListUsersPager(ctx, paginator, converter)

	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "u1", users[0].ID)
}

func TestListUsersPager_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListUsers(ctx, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	paginator := identitystore.NewListUsersPaginator(mockClient, &identitystore.ListUsersInput{IdentityStoreId: &identityStoreID})
	users, err := ListUsersPager(ctx, paginator, func(types.User) *interfaces.User { return nil })

	assert.Error(t, err)
	assert.Nil(t, users)
}

func TestListGroupMembershipsPager(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"

	mockClient.EXPECT().ListGroupMemberships(ctx, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupMembershipsOutput{
			GroupMemberships: []types.GroupMembership{
				{MemberId: &types.MemberIdMemberUserId{Value: "u1"}},
				{MemberId: nil}, // filtered out
			},
		}, nil).Once()

	converter := func(m types.GroupMembership) *interfaces.User {
		member, ok := m.MemberId.(*types.MemberIdMemberUserId)
		if !ok {
			return nil
		}
		return &interfaces.User{ID: member.Value}
	}

	paginator := identitystore.NewListGroupMembershipsPaginator(mockClient, &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
	})
	users, err := ListGroupMembershipsPager(ctx, paginator, converter)

	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "u1", users[0].ID)
}

func TestListGroupMembershipsPager_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"

	mockClient.EXPECT().ListGroupMemberships(ctx, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	paginator := identitystore.NewListGroupMembershipsPaginator(mockClient, &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
	})
	users, err := ListGroupMembershipsPager(ctx, paginator, func(types.GroupMembership) *interfaces.User { return nil })

	assert.Error(t, err)
	assert.Nil(t, users)
}
