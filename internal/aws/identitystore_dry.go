package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/identitystore"
	"github.com/aws/aws-sdk-go/service/identitystore/identitystoreiface"
)

type DryIdentityStore struct{}

func NewDryIdentityStore() identitystoreiface.IdentityStoreAPI {
	return &DryIdentityStore{}
}

func (d *DryIdentityStore) CreateGroup(input *identitystore.CreateGroupInput) (*identitystore.CreateGroupOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) CreateGroupMembership(input *identitystore.CreateGroupMembershipInput) (*identitystore.CreateGroupMembershipOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) CreateUser(input *identitystore.CreateUserInput) (*identitystore.CreateUserOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DeleteGroup(input *identitystore.DeleteGroupInput) (*identitystore.DeleteGroupOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DeleteGroupMembership(input *identitystore.DeleteGroupMembershipInput) (*identitystore.DeleteGroupMembershipOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DeleteUser(input *identitystore.DeleteUserInput) (*identitystore.DeleteUserOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DescribeGroup(input *identitystore.DescribeGroupInput) (*identitystore.DescribeGroupOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DescribeGroupMembership(input *identitystore.DescribeGroupMembershipInput) (*identitystore.DescribeGroupMembershipOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DescribeUser(input *identitystore.DescribeUserInput) (*identitystore.DescribeUserOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) GetGroupId(input *identitystore.GetGroupIdInput) (*identitystore.GetGroupIdOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) GetGroupMembershipId(input *identitystore.GetGroupMembershipIdInput) (*identitystore.GetGroupMembershipIdOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) GetUserId(input *identitystore.GetUserIdInput) (*identitystore.GetUserIdOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) IsMemberInGroups(input *identitystore.IsMemberInGroupsInput) (*identitystore.IsMemberInGroupsOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroups(input *identitystore.ListGroupsInput) (*identitystore.ListGroupsOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupMemberships(input *identitystore.ListGroupMembershipsInput) (*identitystore.ListGroupMembershipsOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListUsers(input *identitystore.ListUsersInput) (*identitystore.ListUsersOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) UpdateGroup(input *identitystore.UpdateGroupInput) (*identitystore.UpdateGroupOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) UpdateUser(input *identitystore.UpdateUserInput) (*identitystore.UpdateUserOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupsPages(input *identitystore.ListGroupsInput, fn func(*identitystore.ListGroupsOutput, bool) bool) error {
	return nil
}

func (d *DryIdentityStore) ListGroupMembershipsPages(input *identitystore.ListGroupMembershipsInput, fn func(*identitystore.ListGroupMembershipsOutput, bool) bool) error {
	return nil
}

func (d *DryIdentityStore) ListUsersPages(input *identitystore.ListUsersInput, fn func(*identitystore.ListUsersOutput, bool) bool) error {
	return nil
}

func (d *DryIdentityStore) CreateGroupRequest(input *identitystore.CreateGroupInput) (*request.Request, *identitystore.CreateGroupOutput) {
	return nil, nil
}

func (d *DryIdentityStore) CreateGroupMembershipRequest(input *identitystore.CreateGroupMembershipInput) (*request.Request, *identitystore.CreateGroupMembershipOutput) {
	return nil, nil
}

func (d *DryIdentityStore) CreateUserRequest(input *identitystore.CreateUserInput) (*request.Request, *identitystore.CreateUserOutput) {
	return nil, nil
}

func (d *DryIdentityStore) DeleteGroupRequest(input *identitystore.DeleteGroupInput) (*request.Request, *identitystore.DeleteGroupOutput) {
	return nil, nil
}

func (d *DryIdentityStore) DeleteGroupMembershipRequest(input *identitystore.DeleteGroupMembershipInput) (*request.Request, *identitystore.DeleteGroupMembershipOutput) {
	return nil, nil
}

func (d *DryIdentityStore) DeleteUserRequest(input *identitystore.DeleteUserInput) (*request.Request, *identitystore.DeleteUserOutput) {
	return nil, nil
}

func (d *DryIdentityStore) DescribeGroupRequest(input *identitystore.DescribeGroupInput) (*request.Request, *identitystore.DescribeGroupOutput) {
	return nil, nil
}

func (d *DryIdentityStore) DescribeGroupMembershipRequest(input *identitystore.DescribeGroupMembershipInput) (*request.Request, *identitystore.DescribeGroupMembershipOutput) {
	return nil, nil
}

func (d *DryIdentityStore) DescribeUserRequest(input *identitystore.DescribeUserInput) (*request.Request, *identitystore.DescribeUserOutput) {
	return nil, nil
}

func (d *DryIdentityStore) GetGroupIdRequest(input *identitystore.GetGroupIdInput) (*request.Request, *identitystore.GetGroupIdOutput) {
	return nil, nil
}

func (d *DryIdentityStore) GetGroupMembershipIdRequest(input *identitystore.GetGroupMembershipIdInput) (*request.Request, *identitystore.GetGroupMembershipIdOutput) {
	return nil, nil
}

func (d *DryIdentityStore) GetUserIdRequest(input *identitystore.GetUserIdInput) (*request.Request, *identitystore.GetUserIdOutput) {
	return nil, nil
}

func (d *DryIdentityStore) IsMemberInGroupsRequest(input *identitystore.IsMemberInGroupsInput) (*request.Request, *identitystore.IsMemberInGroupsOutput) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupsRequest(input *identitystore.ListGroupsInput) (*request.Request, *identitystore.ListGroupsOutput) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupMembershipsRequest(input *identitystore.ListGroupMembershipsInput) (*request.Request, *identitystore.ListGroupMembershipsOutput) {
	return nil, nil
}

func (d *DryIdentityStore) ListUsersRequest(input *identitystore.ListUsersInput) (*request.Request, *identitystore.ListUsersOutput) {
	return nil, nil
}

func (d *DryIdentityStore) UpdateGroupRequest(input *identitystore.UpdateGroupInput) (*request.Request, *identitystore.UpdateGroupOutput) {
	return nil, nil
}

func (d *DryIdentityStore) UpdateUserRequest(input *identitystore.UpdateUserInput) (*request.Request, *identitystore.UpdateUserOutput) {
	return nil, nil
}

func (d *DryIdentityStore) CreateGroupWithContext(ctx aws.Context, input *identitystore.CreateGroupInput, opts ...request.Option) (*identitystore.CreateGroupOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) CreateGroupMembershipWithContext(ctx aws.Context, input *identitystore.CreateGroupMembershipInput, opts ...request.Option) (*identitystore.CreateGroupMembershipOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) CreateUserWithContext(ctx aws.Context, input *identitystore.CreateUserInput, opts ...request.Option) (*identitystore.CreateUserOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DeleteGroupWithContext(ctx aws.Context, input *identitystore.DeleteGroupInput, opts ...request.Option) (*identitystore.DeleteGroupOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DeleteGroupMembershipWithContext(ctx aws.Context, input *identitystore.DeleteGroupMembershipInput, opts ...request.Option) (*identitystore.DeleteGroupMembershipOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DeleteUserWithContext(ctx aws.Context, input *identitystore.DeleteUserInput, opts ...request.Option) (*identitystore.DeleteUserOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DescribeGroupWithContext(ctx aws.Context, input *identitystore.DescribeGroupInput, opts ...request.Option) (*identitystore.DescribeGroupOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DescribeGroupMembershipWithContext(ctx aws.Context, input *identitystore.DescribeGroupMembershipInput, opts ...request.Option) (*identitystore.DescribeGroupMembershipOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) DescribeUserWithContext(ctx aws.Context, input *identitystore.DescribeUserInput, opts ...request.Option) (*identitystore.DescribeUserOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) GetGroupIdWithContext(ctx aws.Context, input *identitystore.GetGroupIdInput, opts ...request.Option) (*identitystore.GetGroupIdOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) GetGroupMembershipIdWithContext(ctx aws.Context, input *identitystore.GetGroupMembershipIdInput, opts ...request.Option) (*identitystore.GetGroupMembershipIdOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) GetUserIdWithContext(ctx aws.Context, input *identitystore.GetUserIdInput, opts ...request.Option) (*identitystore.GetUserIdOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) IsMemberInGroupsWithContext(ctx aws.Context, input *identitystore.IsMemberInGroupsInput, opts ...request.Option) (*identitystore.IsMemberInGroupsOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupsWithContext(ctx aws.Context, input *identitystore.ListGroupsInput, opts ...request.Option) (*identitystore.ListGroupsOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupMembershipsWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsInput, opts ...request.Option) (*identitystore.ListGroupMembershipsOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListUsersWithContext(ctx aws.Context, input *identitystore.ListUsersInput, opts ...request.Option) (*identitystore.ListUsersOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) UpdateGroupWithContext(ctx aws.Context, input *identitystore.UpdateGroupInput, opts ...request.Option) (*identitystore.UpdateGroupOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) UpdateUserWithContext(ctx aws.Context, input *identitystore.UpdateUserInput, opts ...request.Option) (*identitystore.UpdateUserOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupMembershipsForMember(input *identitystore.ListGroupMembershipsForMemberInput) (*identitystore.ListGroupMembershipsForMemberOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupMembershipsForMemberRequest(input *identitystore.ListGroupMembershipsForMemberInput) (*request.Request, *identitystore.ListGroupMembershipsForMemberOutput) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupMembershipsForMemberWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsForMemberInput, opts ...request.Option) (*identitystore.ListGroupMembershipsForMemberOutput, error) {
	return nil, nil
}

func (d *DryIdentityStore) ListGroupMembershipsForMemberPages(input *identitystore.ListGroupMembershipsForMemberInput, fn func(*identitystore.ListGroupMembershipsForMemberOutput, bool) bool) error {
	return nil
}

func (d *DryIdentityStore) ListGroupMembershipsForMemberPagesWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsForMemberInput, fn func(*identitystore.ListGroupMembershipsForMemberOutput, bool) bool, opts ...request.Option) error {
	return nil
}

func (d *DryIdentityStore) ListGroupMembershipsPagesWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsInput, fn func(*identitystore.ListGroupMembershipsOutput, bool) bool, opts ...request.Option) error {
	return nil
}

func (d *DryIdentityStore) ListGroupsPagesWithContext(ctx aws.Context, input *identitystore.ListGroupsInput, fn func(*identitystore.ListGroupsOutput, bool) bool, opts ...request.Option) error {
	return nil
}

func (d *DryIdentityStore) ListUsersPagesWithContext(ctx aws.Context, input *identitystore.ListUsersInput, fn func(*identitystore.ListUsersOutput, bool) bool, opts ...request.Option) error {
	return nil
}