package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/identitystore"
	"github.com/aws/aws-sdk-go/service/identitystore/identitystoreiface"
)

type DryIdentityStore struct{
    *NullIdentityStore
    // TODO: use actual client underneath
}

func NewDryIdentityStore() identitystoreiface.IdentityStoreAPI {
    // TODO: initialize the actual client
	return &DryIdentityStore{}
}

// ********************
// LLM generated
// func ... { return nil, nil }
// DO NOT ADD CODE BELOW THIS LINE
// ********************

type NullIdentityStore struct{}

func (d *NullIdentityStore) CreateGroup(input *identitystore.CreateGroupInput) (*identitystore.CreateGroupOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) CreateGroupMembership(input *identitystore.CreateGroupMembershipInput) (*identitystore.CreateGroupMembershipOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) CreateUser(input *identitystore.CreateUserInput) (*identitystore.CreateUserOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DeleteGroup(input *identitystore.DeleteGroupInput) (*identitystore.DeleteGroupOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DeleteGroupMembership(input *identitystore.DeleteGroupMembershipInput) (*identitystore.DeleteGroupMembershipOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DeleteUser(input *identitystore.DeleteUserInput) (*identitystore.DeleteUserOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DescribeGroup(input *identitystore.DescribeGroupInput) (*identitystore.DescribeGroupOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DescribeGroupMembership(input *identitystore.DescribeGroupMembershipInput) (*identitystore.DescribeGroupMembershipOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DescribeUser(input *identitystore.DescribeUserInput) (*identitystore.DescribeUserOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) GetGroupId(input *identitystore.GetGroupIdInput) (*identitystore.GetGroupIdOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) GetGroupMembershipId(input *identitystore.GetGroupMembershipIdInput) (*identitystore.GetGroupMembershipIdOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) GetUserId(input *identitystore.GetUserIdInput) (*identitystore.GetUserIdOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) IsMemberInGroups(input *identitystore.IsMemberInGroupsInput) (*identitystore.IsMemberInGroupsOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroups(input *identitystore.ListGroupsInput) (*identitystore.ListGroupsOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupMemberships(input *identitystore.ListGroupMembershipsInput) (*identitystore.ListGroupMembershipsOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListUsers(input *identitystore.ListUsersInput) (*identitystore.ListUsersOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) UpdateGroup(input *identitystore.UpdateGroupInput) (*identitystore.UpdateGroupOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) UpdateUser(input *identitystore.UpdateUserInput) (*identitystore.UpdateUserOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupsPages(input *identitystore.ListGroupsInput, fn func(*identitystore.ListGroupsOutput, bool) bool) error {
	return nil
}

func (d *NullIdentityStore) ListGroupMembershipsPages(input *identitystore.ListGroupMembershipsInput, fn func(*identitystore.ListGroupMembershipsOutput, bool) bool) error {
	return nil
}

func (d *NullIdentityStore) ListUsersPages(input *identitystore.ListUsersInput, fn func(*identitystore.ListUsersOutput, bool) bool) error {
	return nil
}

func (d *NullIdentityStore) CreateGroupRequest(input *identitystore.CreateGroupInput) (*request.Request, *identitystore.CreateGroupOutput) {
	return nil, nil
}

func (d *NullIdentityStore) CreateGroupMembershipRequest(input *identitystore.CreateGroupMembershipInput) (*request.Request, *identitystore.CreateGroupMembershipOutput) {
	return nil, nil
}

func (d *NullIdentityStore) CreateUserRequest(input *identitystore.CreateUserInput) (*request.Request, *identitystore.CreateUserOutput) {
	return nil, nil
}

func (d *NullIdentityStore) DeleteGroupRequest(input *identitystore.DeleteGroupInput) (*request.Request, *identitystore.DeleteGroupOutput) {
	return nil, nil
}

func (d *NullIdentityStore) DeleteGroupMembershipRequest(input *identitystore.DeleteGroupMembershipInput) (*request.Request, *identitystore.DeleteGroupMembershipOutput) {
	return nil, nil
}

func (d *NullIdentityStore) DeleteUserRequest(input *identitystore.DeleteUserInput) (*request.Request, *identitystore.DeleteUserOutput) {
	return nil, nil
}

func (d *NullIdentityStore) DescribeGroupRequest(input *identitystore.DescribeGroupInput) (*request.Request, *identitystore.DescribeGroupOutput) {
	return nil, nil
}

func (d *NullIdentityStore) DescribeGroupMembershipRequest(input *identitystore.DescribeGroupMembershipInput) (*request.Request, *identitystore.DescribeGroupMembershipOutput) {
	return nil, nil
}

func (d *NullIdentityStore) DescribeUserRequest(input *identitystore.DescribeUserInput) (*request.Request, *identitystore.DescribeUserOutput) {
	return nil, nil
}

func (d *NullIdentityStore) GetGroupIdRequest(input *identitystore.GetGroupIdInput) (*request.Request, *identitystore.GetGroupIdOutput) {
	return nil, nil
}

func (d *NullIdentityStore) GetGroupMembershipIdRequest(input *identitystore.GetGroupMembershipIdInput) (*request.Request, *identitystore.GetGroupMembershipIdOutput) {
	return nil, nil
}

func (d *NullIdentityStore) GetUserIdRequest(input *identitystore.GetUserIdInput) (*request.Request, *identitystore.GetUserIdOutput) {
	return nil, nil
}

func (d *NullIdentityStore) IsMemberInGroupsRequest(input *identitystore.IsMemberInGroupsInput) (*request.Request, *identitystore.IsMemberInGroupsOutput) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupsRequest(input *identitystore.ListGroupsInput) (*request.Request, *identitystore.ListGroupsOutput) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupMembershipsRequest(input *identitystore.ListGroupMembershipsInput) (*request.Request, *identitystore.ListGroupMembershipsOutput) {
	return nil, nil
}

func (d *NullIdentityStore) ListUsersRequest(input *identitystore.ListUsersInput) (*request.Request, *identitystore.ListUsersOutput) {
	return nil, nil
}

func (d *NullIdentityStore) UpdateGroupRequest(input *identitystore.UpdateGroupInput) (*request.Request, *identitystore.UpdateGroupOutput) {
	return nil, nil
}

func (d *NullIdentityStore) UpdateUserRequest(input *identitystore.UpdateUserInput) (*request.Request, *identitystore.UpdateUserOutput) {
	return nil, nil
}

func (d *NullIdentityStore) CreateGroupWithContext(ctx aws.Context, input *identitystore.CreateGroupInput, opts ...request.Option) (*identitystore.CreateGroupOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) CreateGroupMembershipWithContext(ctx aws.Context, input *identitystore.CreateGroupMembershipInput, opts ...request.Option) (*identitystore.CreateGroupMembershipOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) CreateUserWithContext(ctx aws.Context, input *identitystore.CreateUserInput, opts ...request.Option) (*identitystore.CreateUserOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DeleteGroupWithContext(ctx aws.Context, input *identitystore.DeleteGroupInput, opts ...request.Option) (*identitystore.DeleteGroupOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DeleteGroupMembershipWithContext(ctx aws.Context, input *identitystore.DeleteGroupMembershipInput, opts ...request.Option) (*identitystore.DeleteGroupMembershipOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DeleteUserWithContext(ctx aws.Context, input *identitystore.DeleteUserInput, opts ...request.Option) (*identitystore.DeleteUserOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DescribeGroupWithContext(ctx aws.Context, input *identitystore.DescribeGroupInput, opts ...request.Option) (*identitystore.DescribeGroupOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DescribeGroupMembershipWithContext(ctx aws.Context, input *identitystore.DescribeGroupMembershipInput, opts ...request.Option) (*identitystore.DescribeGroupMembershipOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) DescribeUserWithContext(ctx aws.Context, input *identitystore.DescribeUserInput, opts ...request.Option) (*identitystore.DescribeUserOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) GetGroupIdWithContext(ctx aws.Context, input *identitystore.GetGroupIdInput, opts ...request.Option) (*identitystore.GetGroupIdOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) GetGroupMembershipIdWithContext(ctx aws.Context, input *identitystore.GetGroupMembershipIdInput, opts ...request.Option) (*identitystore.GetGroupMembershipIdOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) GetUserIdWithContext(ctx aws.Context, input *identitystore.GetUserIdInput, opts ...request.Option) (*identitystore.GetUserIdOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) IsMemberInGroupsWithContext(ctx aws.Context, input *identitystore.IsMemberInGroupsInput, opts ...request.Option) (*identitystore.IsMemberInGroupsOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupsWithContext(ctx aws.Context, input *identitystore.ListGroupsInput, opts ...request.Option) (*identitystore.ListGroupsOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupMembershipsWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsInput, opts ...request.Option) (*identitystore.ListGroupMembershipsOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListUsersWithContext(ctx aws.Context, input *identitystore.ListUsersInput, opts ...request.Option) (*identitystore.ListUsersOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) UpdateGroupWithContext(ctx aws.Context, input *identitystore.UpdateGroupInput, opts ...request.Option) (*identitystore.UpdateGroupOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) UpdateUserWithContext(ctx aws.Context, input *identitystore.UpdateUserInput, opts ...request.Option) (*identitystore.UpdateUserOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupMembershipsForMember(input *identitystore.ListGroupMembershipsForMemberInput) (*identitystore.ListGroupMembershipsForMemberOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupMembershipsForMemberRequest(input *identitystore.ListGroupMembershipsForMemberInput) (*request.Request, *identitystore.ListGroupMembershipsForMemberOutput) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupMembershipsForMemberWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsForMemberInput, opts ...request.Option) (*identitystore.ListGroupMembershipsForMemberOutput, error) {
	return nil, nil
}

func (d *NullIdentityStore) ListGroupMembershipsForMemberPages(input *identitystore.ListGroupMembershipsForMemberInput, fn func(*identitystore.ListGroupMembershipsForMemberOutput, bool) bool) error {
	return nil
}

func (d *NullIdentityStore) ListGroupMembershipsForMemberPagesWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsForMemberInput, fn func(*identitystore.ListGroupMembershipsForMemberOutput, bool) bool, opts ...request.Option) error {
	return nil
}

func (d *NullIdentityStore) ListGroupMembershipsPagesWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsInput, fn func(*identitystore.ListGroupMembershipsOutput, bool) bool, opts ...request.Option) error {
	return nil
}

func (d *NullIdentityStore) ListGroupsPagesWithContext(ctx aws.Context, input *identitystore.ListGroupsInput, fn func(*identitystore.ListGroupsOutput, bool) bool, opts ...request.Option) error {
	return nil
}

func (d *NullIdentityStore) ListUsersPagesWithContext(ctx aws.Context, input *identitystore.ListUsersInput, fn func(*identitystore.ListUsersOutput, bool) bool, opts ...request.Option) error {
	return nil
}
