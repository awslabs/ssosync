package identitystore

import (
	"context"

	"ssosync/internal/interfaces"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
)

// CreateGroup creates a group in the identity store
func CreateGroup(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, displayName *string) (*identitystore.CreateGroupOutput, error) {
	return client.CreateGroup(ctx, &identitystore.CreateGroupInput{
		IdentityStoreId: identityStoreID,
		DisplayName:     displayName,
	})
}

// CreateGroupMembership creates a group membership
func CreateGroupMembership(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupID *string, userID *string) (*identitystore.CreateGroupMembershipOutput, error) {
	return client.CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
		IdentityStoreId: identityStoreID,
		GroupId:         groupID,
		MemberId: &types.MemberIdMemberUserId{
			Value: *userID,
		},
	})
}

// DeleteGroup deletes a group from the identity store
func DeleteGroup(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupID *string) (*identitystore.DeleteGroupOutput, error) {
	return client.DeleteGroup(ctx, &identitystore.DeleteGroupInput{
		IdentityStoreId: identityStoreID,
		GroupId:         groupID,
	})
}

// DeleteUser deletes a user from the identity store
func DeleteUser(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, userID *string) (*identitystore.DeleteUserOutput, error) {
	return client.DeleteUser(ctx, &identitystore.DeleteUserInput{
		IdentityStoreId: identityStoreID,
		UserId:          userID,
	})
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
		return nil, err
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
			return nil, err
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
			return nil, err
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
			return nil, err
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
			return nil, err
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
	return client.GetGroupMembershipId(ctx, &identitystore.GetGroupMembershipIdInput{
		IdentityStoreId: identityStoreID,
		GroupId:         groupID,
		MemberId: &types.MemberIdMemberUserId{
			Value: *userID,
		},
	})
}

// DeleteGroupMembership deletes a group membership
func DeleteGroupMembership(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, membershipID *string) (*identitystore.DeleteGroupMembershipOutput, error) {
	return client.DeleteGroupMembership(ctx, &identitystore.DeleteGroupMembershipInput{
		IdentityStoreId: identityStoreID,
		MembershipId:    membershipID,
	})
}
