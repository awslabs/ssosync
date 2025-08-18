package identitystore

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	ssosync_errors "github.com/awslabs/ssosync/internal/errors"
	"github.com/awslabs/ssosync/internal/interfaces"
)

// CreateGroup creates a group in the identity store
func CreateGroup(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, displayName *string) (*identitystore.CreateGroupOutput, error) {
	result, err := client.CreateGroup(ctx, &identitystore.CreateGroupInput{
		IdentityStoreId: identityStoreID,
		DisplayName:     displayName,
	})
	if err != nil {
		return nil, ssosync_errors.HandleIdentityStoreError("CreateGroup", err)
	}
	return result, nil
}

// CreateGroupMembership creates a group membership
func CreateGroupMembership(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupID *string, userID *string) (*identitystore.CreateGroupMembershipOutput, error) {
	result, err := client.CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
		IdentityStoreId: identityStoreID,
		GroupId:         groupID,
		MemberId: &types.MemberIdMemberUserId{
			Value: *userID,
		},
	})
	if err != nil {
		return nil, ssosync_errors.HandleIdentityStoreError("CreateGroupMembership", err)
	}
	return result, nil
}

// DeleteGroup deletes a group from the identity store
func DeleteGroup(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupID *string) (*identitystore.DeleteGroupOutput, error) {
	result, err := client.DeleteGroup(ctx, &identitystore.DeleteGroupInput{
		IdentityStoreId: identityStoreID,
		GroupId:         groupID,
	})
	if err != nil {
		return nil, ssosync_errors.HandleIdentityStoreError("DeleteGroup", err)
	}
	return result, nil
}

// DeleteUser deletes a user from the identity store
func DeleteUser(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, userID *string) (*identitystore.DeleteUserOutput, error) {
	result, err := client.DeleteUser(ctx, &identitystore.DeleteUserInput{
		IdentityStoreId: identityStoreID,
		UserId:          userID,
	})
	if err != nil {
		return nil, ssosync_errors.HandleIdentityStoreError("DeleteUser", err)
	}
	return result, nil
}

// IsMemberInGroups checks if a user is a member of specified groups
func IsMemberInGroups(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupIDs []string, userID *string) (*bool, error) {
	result, err := client.IsMemberInGroups(ctx, &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: identityStoreID,
		MemberId: &types.MemberIdMemberUserId{
			Value: *userID,
		},
		GroupIds: groupIDs,
	})
	if err != nil {
		return nil, ssosync_errors.HandleIdentityStoreError("IsMemberInGroups", err)
	}

	// Check if user is member of any of the groups
	for _, membershipResult := range result.Results {
		if membershipResult.MembershipExists {
			return aws.Bool(true), nil
		}
	}
	return aws.Bool(false), nil
}

// ListGroups lists all groups with pagination support
func ListGroups(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, converter func(types.Group) *interfaces.Group) ([]*interfaces.Group, error) {
	var groups []*interfaces.Group

	paginator := identitystore.NewListGroupsPaginator(client, &identitystore.ListGroupsInput{
		IdentityStoreId: identityStoreID,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, ssosync_errors.HandleIdentityStoreError("ListGroups", err)
		}

		for _, group := range page.Groups {
			if convertedGroup := converter(group); convertedGroup != nil {
				groups = append(groups, convertedGroup)
			}
		}
	}

	return groups, nil
}

// ListGroupsPager is a helper function for paginated group listing
func ListGroupsPager(ctx context.Context, paginator *identitystore.ListGroupsPaginator, converter func(types.Group) *interfaces.Group) ([]*interfaces.Group, error) {
	var groups []*interfaces.Group

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, ssosync_errors.HandleIdentityStoreError("ListGroups", err)
		}

		for _, group := range page.Groups {
			if convertedGroup := converter(group); convertedGroup != nil {
				groups = append(groups, convertedGroup)
			}
		}
	}

	return groups, nil
}

// ListUsersPager is a helper function for paginated user listing
func ListUsersPager(ctx context.Context, paginator *identitystore.ListUsersPaginator, converter func(types.User) *interfaces.User) ([]*interfaces.User, error) {
	var users []*interfaces.User

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, ssosync_errors.HandleIdentityStoreError("ListUsers", err)
		}

		for _, user := range page.Users {
			if convertedUser := converter(user); convertedUser != nil {
				users = append(users, convertedUser)
			}
		}
	}

	return users, nil
}

// ListGroupMembershipsPager is a helper function for paginated group membership listing
func ListGroupMembershipsPager(ctx context.Context, paginator *identitystore.ListGroupMembershipsPaginator, converter func(types.GroupMembership) *interfaces.User) ([]*interfaces.User, error) {
	var users []*interfaces.User

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, ssosync_errors.HandleIdentityStoreError("ListGroupMemberships", err)
		}

		for _, membership := range page.GroupMemberships {
			if convertedUser := converter(membership); convertedUser != nil {
				users = append(users, convertedUser)
			}
		}
	}

	return users, nil
}

// GetGroupMembershipId gets the membership ID for a user in a group
func GetGroupMembershipId(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupID *string, userID *string) (*identitystore.GetGroupMembershipIdOutput, error) {
	result, err := client.GetGroupMembershipId(ctx, &identitystore.GetGroupMembershipIdInput{
		IdentityStoreId: identityStoreID,
		GroupId:         groupID,
		MemberId: &types.MemberIdMemberUserId{
			Value: *userID,
		},
	})
	if err != nil {
		return nil, ssosync_errors.HandleIdentityStoreError("GetGroupMembershipId", err)
	}
	return result, nil
}

// DeleteGroupMembership deletes a group membership
func DeleteGroupMembership(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, membershipID *string) (*identitystore.DeleteGroupMembershipOutput, error) {
	result, err := client.DeleteGroupMembership(ctx, &identitystore.DeleteGroupMembershipInput{
		IdentityStoreId: identityStoreID,
		MembershipId:    membershipID,
	})
	if err != nil {
		return nil, ssosync_errors.HandleIdentityStoreError("DeleteGroupMembership", err)
	}
	return result, nil
}
