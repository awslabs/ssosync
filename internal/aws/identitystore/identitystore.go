package identitystore

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/awslabs/ssosync/internal/interfaces"
)

func GetMemberIdMemberUserId(memberId types.MemberId) (*string, error) {
	switch v := memberId.(type) {
	case *types.MemberIdMemberUserId:
		return &v.Value, nil

	case *types.UnknownUnionMember:
		return nil, errors.New("expected a user id, got unknown type id")

	default:
		return nil, errors.New("expected a user id, got unknown type id")
	}
}

func IsMemberInGroups(ctx context.Context, api interfaces.IdentityStoreAPI,
	identityStoreId *string,
	groupIds []string,
	memberId *string,
) (*bool, error) {
	object, err := api.IsMemberInGroups(ctx, &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: identityStoreId,
		GroupIds:        groupIds,
		MemberId:        &types.MemberIdMemberUserId{Value: *memberId},
	})

	if err != nil {
		return nil, err
	}
	isUserInGroup := object.Results[0].MembershipExists
	return &isUserInGroup, nil
}
func GetGroupMembershipId(ctx context.Context, api interfaces.IdentityStoreAPI,
	identityStoreId *string,
	groupId *string,
	memberId *string,
) (*identitystore.GetGroupMembershipIdOutput, error) {
	object, err := api.GetGroupMembershipId(ctx, &identitystore.GetGroupMembershipIdInput{
		IdentityStoreId: identityStoreId,
		GroupId:         groupId,
		MemberId:        &types.MemberIdMemberUserId{Value: *memberId},
	})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func DeleteGroupMembership(ctx context.Context, api interfaces.IdentityStoreAPI,
	identityStoreId *string,
	membershipId *string,
) (*identitystore.DeleteGroupMembershipOutput, error) {
	object, err := api.DeleteGroupMembership(ctx, &identitystore.DeleteGroupMembershipInput{
		IdentityStoreId: identityStoreId,
		MembershipId:    membershipId,
	})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func CreateGroup(ctx context.Context, api interfaces.IdentityStoreAPI,
	identityStoreId *string,
	displayName *string,
) (*identitystore.CreateGroupOutput, error) {
	object, err := api.CreateGroup(ctx, &identitystore.CreateGroupInput{
		IdentityStoreId: identityStoreId,
		DisplayName:     displayName,
	})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func DeleteGroup(ctx context.Context, api interfaces.IdentityStoreAPI, identityStoreId *string, groupId *string) (*identitystore.DeleteGroupOutput, error) {
	object, err := api.DeleteGroup(ctx, &identitystore.DeleteGroupInput{
		IdentityStoreId: identityStoreId,
		GroupId:         groupId,
	})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func CreateGroupMembership(ctx context.Context, api interfaces.IdentityStoreAPI, identityStoreId *string, groupId *string, userId *string) (*identitystore.CreateGroupMembershipOutput, error) {
	object, err := api.CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
		IdentityStoreId: identityStoreId,
		GroupId:         groupId,
		MemberId:        &types.MemberIdMemberUserId{Value: *userId},
	})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func DeleteUser(ctx context.Context, api interfaces.IdentityStoreAPI, identityStoreId *string, userId *string) (*identitystore.DeleteUserOutput, error) {
	object, err := api.DeleteUser(ctx, &identitystore.DeleteUserInput{
		IdentityStoreId: identityStoreId,
		UserId:          userId,
	})
	if err != nil {
		return nil, err
	}
	return object, nil
}

func ListGroups(ctx context.Context, api identitystore.ListGroupsAPIClient, identityStoreId *string, lambdaConvert func(types.Group) *interfaces.Group) ([]*interfaces.Group, error) {

	object, err := api.ListGroups(ctx, &identitystore.ListGroupsInput{
		IdentityStoreId: identityStoreId,
	})
	if err != nil {
		return nil, err
	}
	awsGroups := make([]*interfaces.Group, 0)
	for _, group := range object.Groups {
		awsGroups = append(awsGroups, lambdaConvert(group))
	}
	return awsGroups, nil
}

func ListGroupsPager(ctx context.Context, paginator interfaces.ListGroupsPaginator, lambdaConvert func(types.Group) *interfaces.Group) ([]*interfaces.Group, error) {

	awsGroups := make([]*interfaces.Group, 0)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, group := range page.Groups {
			awsGroups = append(awsGroups, lambdaConvert(group))
		}
	}

	return awsGroups, nil
}

func ListGroupMembershipsPager(ctx context.Context, paginator interfaces.ListGroupMembershipsPaginator) ([]string, error) {
	awsGroupsUsers := make([]string, 0)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, member := range page.GroupMemberships { // For every member in the group
			userID, err := GetMemberIdMemberUserId(member.MemberId)
			if err != nil {
				return nil, err
			}
			awsGroupsUsers = append(awsGroupsUsers, *userID)
		}
	}

	return awsGroupsUsers, nil
}

func ListUsersPager(ctx context.Context, paginator interfaces.ListUsersPaginator, lambdaConvert func(types.User) *interfaces.User) ([]*interfaces.User, error) {
	awsUsers := make([]*interfaces.User, 0)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, user := range page.Users {
			awsUsers = append(awsUsers, lambdaConvert(user))
		}
	}

	return awsUsers, nil
}
