package interfaces

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
)

// IdentityStoreAPI defines the interface for the AWS Identity Store API methods used in the application
type IdentityStoreAPI interface {
	identitystore.ListGroupsAPIClient
	identitystore.ListUsersAPIClient
	identitystore.ListGroupMembershipsAPIClient

	IsMemberInGroups(ctx context.Context, params *identitystore.IsMemberInGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.IsMemberInGroupsOutput, error)
	GetGroupMembershipId(ctx context.Context, params *identitystore.GetGroupMembershipIdInput, optFns ...func(*identitystore.Options)) (*identitystore.GetGroupMembershipIdOutput, error)
	DeleteGroupMembership(ctx context.Context, params *identitystore.DeleteGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteGroupMembershipOutput, error)
	CreateGroup(ctx context.Context, params *identitystore.CreateGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupOutput, error)
	DeleteGroup(ctx context.Context, params *identitystore.DeleteGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteGroupOutput, error)
	CreateGroupMembership(ctx context.Context, params *identitystore.CreateGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error)
	DeleteUser(ctx context.Context, params *identitystore.DeleteUserInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteUserOutput, error)
	CreateUser(ctx context.Context, params *identitystore.CreateUserInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error)
}

type IdentityStorePaginators interface {
	NewListUsersPaginator(client identitystore.ListUsersAPIClient, params *identitystore.ListUsersInput, optFns ...func(*identitystore.ListUsersPaginatorOptions)) *ListUsersPaginator
	NewListGroupMembershipsPaginator(client identitystore.ListGroupMembershipsAPIClient, params *identitystore.ListGroupMembershipsInput, optFns ...func(*identitystore.ListGroupMembershipsPaginatorOptions)) *ListGroupMembershipsPaginator
	NewListGroupsPaginator(client identitystore.ListGroupsAPIClient, params *identitystore.ListGroupsInput, optFns ...func(*identitystore.ListGroupsPaginatorOptions)) *ListGroupsPaginator
}
type IdentityStorePaginatedAPI interface {
	ListGroupsPager(ctx context.Context, paginator ListGroupsPaginator, lambdaConvert func(types.Group) *Group) ([]*Group, error)
	ListGroupMembershipsPager(ctx context.Context, paginator ListGroupMembershipsPaginator) ([]string, error)
	ListUsersPager(ctx context.Context, paginator ListUsersPaginator, lambdaConvert func(types.User) *User) ([]*User, error)
}

type ListUsersPaginator interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error)
}

type ListGroupsPaginator interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error)
}
type ListGroupMembershipsPaginator interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupMembershipsOutput, error)
}
