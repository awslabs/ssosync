package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/identitystore"
	"github.com/aws/aws-sdk-go/service/identitystore/identitystoreiface"
)

type DryIdentityStore struct{
    *NullIdentityStore
    client identitystoreiface.IdentityStoreAPI
}

func NewDryIdentityStore(sess *session.Session) identitystoreiface.IdentityStoreAPI {
	return &DryIdentityStore{
		NullIdentityStore: &NullIdentityStore{},
		client: identitystore.New(sess),
	}
}


// **********************
// Noop-success
// **********************

// TODO: return output in terms of input, don't do any real work, don't do things, don't call functions
CreateGroup
CreateGroupMembership
DeleteGroup
DeleteGroupMembership
DeleteUser


// **********************
// Passthrough methods
// **********************

func (d *DryIdentityStore) GetGroupMembershipId(input *identitystore.GetGroupMembershipIdInput) (*identitystore.GetGroupMembershipIdOutput, error) {
	return d.client.GetGroupMembershipId(input)
}

func (d *DryIdentityStore) IsMemberInGroups(input *identitystore.IsMemberInGroupsInput) (*identitystore.IsMemberInGroupsOutput, error) {
	return d.client.IsMemberInGroups(input)
}

func (d *DryIdentityStore) ListGroupMembershipsPages(input *identitystore.ListGroupMembershipsInput, fn func(*identitystore.ListGroupMembershipsOutput, bool) bool) error {
	return d.client.ListGroupMembershipsPages(input, fn)
}

func (d *DryIdentityStore) ListGroupsPages(input *identitystore.ListGroupsInput, fn func(*identitystore.ListGroupsOutput, bool) bool) error {
	return d.client.ListGroupsPages(input, fn)
}

func (d *DryIdentityStore) ListUsersPages(input *identitystore.ListUsersInput, fn func(*identitystore.ListUsersOutput, bool) bool) error {
	return d.client.ListUsersPages(input, fn)
}

// ********************
// LLM generated
// func ... { return nil, nil }
// DO NOT ADD CODE BELOW THIS LINE
// ********************

type NullIdentityStore struct{}

func (_ *NullIdentityStore) CreateGroup(input *identitystore.CreateGroupInput) (*identitystore.CreateGroupOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) CreateGroupMembership(input *identitystore.CreateGroupMembershipInput) (*identitystore.CreateGroupMembershipOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) CreateUser(input *identitystore.CreateUserInput) (*identitystore.CreateUserOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DeleteGroup(input *identitystore.DeleteGroupInput) (*identitystore.DeleteGroupOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DeleteGroupMembership(input *identitystore.DeleteGroupMembershipInput) (*identitystore.DeleteGroupMembershipOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DeleteUser(input *identitystore.DeleteUserInput) (*identitystore.DeleteUserOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DescribeGroup(input *identitystore.DescribeGroupInput) (*identitystore.DescribeGroupOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DescribeGroupMembership(input *identitystore.DescribeGroupMembershipInput) (*identitystore.DescribeGroupMembershipOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DescribeUser(input *identitystore.DescribeUserInput) (*identitystore.DescribeUserOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) GetGroupId(input *identitystore.GetGroupIdInput) (*identitystore.GetGroupIdOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) GetGroupMembershipId(input *identitystore.GetGroupMembershipIdInput) (*identitystore.GetGroupMembershipIdOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) GetUserId(input *identitystore.GetUserIdInput) (*identitystore.GetUserIdOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) IsMemberInGroups(input *identitystore.IsMemberInGroupsInput) (*identitystore.IsMemberInGroupsOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroups(input *identitystore.ListGroupsInput) (*identitystore.ListGroupsOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupMemberships(input *identitystore.ListGroupMembershipsInput) (*identitystore.ListGroupMembershipsOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListUsers(input *identitystore.ListUsersInput) (*identitystore.ListUsersOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) UpdateGroup(input *identitystore.UpdateGroupInput) (*identitystore.UpdateGroupOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) UpdateUser(input *identitystore.UpdateUserInput) (*identitystore.UpdateUserOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupsPages(input *identitystore.ListGroupsInput, fn func(*identitystore.ListGroupsOutput, bool) bool) error {
	return nil
}

func (_ *NullIdentityStore) ListGroupMembershipsPages(input *identitystore.ListGroupMembershipsInput, fn func(*identitystore.ListGroupMembershipsOutput, bool) bool) error {
	return nil
}

func (_ *NullIdentityStore) ListUsersPages(input *identitystore.ListUsersInput, fn func(*identitystore.ListUsersOutput, bool) bool) error {
	return nil
}

func (_ *NullIdentityStore) CreateGroupRequest(input *identitystore.CreateGroupInput) (*request.Request, *identitystore.CreateGroupOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) CreateGroupMembershipRequest(input *identitystore.CreateGroupMembershipInput) (*request.Request, *identitystore.CreateGroupMembershipOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) CreateUserRequest(input *identitystore.CreateUserInput) (*request.Request, *identitystore.CreateUserOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) DeleteGroupRequest(input *identitystore.DeleteGroupInput) (*request.Request, *identitystore.DeleteGroupOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) DeleteGroupMembershipRequest(input *identitystore.DeleteGroupMembershipInput) (*request.Request, *identitystore.DeleteGroupMembershipOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) DeleteUserRequest(input *identitystore.DeleteUserInput) (*request.Request, *identitystore.DeleteUserOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) DescribeGroupRequest(input *identitystore.DescribeGroupInput) (*request.Request, *identitystore.DescribeGroupOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) DescribeGroupMembershipRequest(input *identitystore.DescribeGroupMembershipInput) (*request.Request, *identitystore.DescribeGroupMembershipOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) DescribeUserRequest(input *identitystore.DescribeUserInput) (*request.Request, *identitystore.DescribeUserOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) GetGroupIdRequest(input *identitystore.GetGroupIdInput) (*request.Request, *identitystore.GetGroupIdOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) GetGroupMembershipIdRequest(input *identitystore.GetGroupMembershipIdInput) (*request.Request, *identitystore.GetGroupMembershipIdOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) GetUserIdRequest(input *identitystore.GetUserIdInput) (*request.Request, *identitystore.GetUserIdOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) IsMemberInGroupsRequest(input *identitystore.IsMemberInGroupsInput) (*request.Request, *identitystore.IsMemberInGroupsOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupsRequest(input *identitystore.ListGroupsInput) (*request.Request, *identitystore.ListGroupsOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupMembershipsRequest(input *identitystore.ListGroupMembershipsInput) (*request.Request, *identitystore.ListGroupMembershipsOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) ListUsersRequest(input *identitystore.ListUsersInput) (*request.Request, *identitystore.ListUsersOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) UpdateGroupRequest(input *identitystore.UpdateGroupInput) (*request.Request, *identitystore.UpdateGroupOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) UpdateUserRequest(input *identitystore.UpdateUserInput) (*request.Request, *identitystore.UpdateUserOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) CreateGroupWithContext(ctx aws.Context, input *identitystore.CreateGroupInput, opts ...request.Option) (*identitystore.CreateGroupOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) CreateGroupMembershipWithContext(ctx aws.Context, input *identitystore.CreateGroupMembershipInput, opts ...request.Option) (*identitystore.CreateGroupMembershipOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) CreateUserWithContext(ctx aws.Context, input *identitystore.CreateUserInput, opts ...request.Option) (*identitystore.CreateUserOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DeleteGroupWithContext(ctx aws.Context, input *identitystore.DeleteGroupInput, opts ...request.Option) (*identitystore.DeleteGroupOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DeleteGroupMembershipWithContext(ctx aws.Context, input *identitystore.DeleteGroupMembershipInput, opts ...request.Option) (*identitystore.DeleteGroupMembershipOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DeleteUserWithContext(ctx aws.Context, input *identitystore.DeleteUserInput, opts ...request.Option) (*identitystore.DeleteUserOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DescribeGroupWithContext(ctx aws.Context, input *identitystore.DescribeGroupInput, opts ...request.Option) (*identitystore.DescribeGroupOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DescribeGroupMembershipWithContext(ctx aws.Context, input *identitystore.DescribeGroupMembershipInput, opts ...request.Option) (*identitystore.DescribeGroupMembershipOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) DescribeUserWithContext(ctx aws.Context, input *identitystore.DescribeUserInput, opts ...request.Option) (*identitystore.DescribeUserOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) GetGroupIdWithContext(ctx aws.Context, input *identitystore.GetGroupIdInput, opts ...request.Option) (*identitystore.GetGroupIdOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) GetGroupMembershipIdWithContext(ctx aws.Context, input *identitystore.GetGroupMembershipIdInput, opts ...request.Option) (*identitystore.GetGroupMembershipIdOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) GetUserIdWithContext(ctx aws.Context, input *identitystore.GetUserIdInput, opts ...request.Option) (*identitystore.GetUserIdOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) IsMemberInGroupsWithContext(ctx aws.Context, input *identitystore.IsMemberInGroupsInput, opts ...request.Option) (*identitystore.IsMemberInGroupsOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupsWithContext(ctx aws.Context, input *identitystore.ListGroupsInput, opts ...request.Option) (*identitystore.ListGroupsOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupMembershipsWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsInput, opts ...request.Option) (*identitystore.ListGroupMembershipsOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListUsersWithContext(ctx aws.Context, input *identitystore.ListUsersInput, opts ...request.Option) (*identitystore.ListUsersOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) UpdateGroupWithContext(ctx aws.Context, input *identitystore.UpdateGroupInput, opts ...request.Option) (*identitystore.UpdateGroupOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) UpdateUserWithContext(ctx aws.Context, input *identitystore.UpdateUserInput, opts ...request.Option) (*identitystore.UpdateUserOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupMembershipsForMember(input *identitystore.ListGroupMembershipsForMemberInput) (*identitystore.ListGroupMembershipsForMemberOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupMembershipsForMemberRequest(input *identitystore.ListGroupMembershipsForMemberInput) (*request.Request, *identitystore.ListGroupMembershipsForMemberOutput) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupMembershipsForMemberWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsForMemberInput, opts ...request.Option) (*identitystore.ListGroupMembershipsForMemberOutput, error) {
	return nil, nil
}

func (_ *NullIdentityStore) ListGroupMembershipsForMemberPages(input *identitystore.ListGroupMembershipsForMemberInput, fn func(*identitystore.ListGroupMembershipsForMemberOutput, bool) bool) error {
	return nil
}

func (_ *NullIdentityStore) ListGroupMembershipsForMemberPagesWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsForMemberInput, fn func(*identitystore.ListGroupMembershipsForMemberOutput, bool) bool, opts ...request.Option) error {
	return nil
}

func (_ *NullIdentityStore) ListGroupMembershipsPagesWithContext(ctx aws.Context, input *identitystore.ListGroupMembershipsInput, fn func(*identitystore.ListGroupMembershipsOutput, bool) bool, opts ...request.Option) error {
	return nil
}

func (_ *NullIdentityStore) ListGroupsPagesWithContext(ctx aws.Context, input *identitystore.ListGroupsInput, fn func(*identitystore.ListGroupsOutput, bool) bool, opts ...request.Option) error {
	return nil
}

func (_ *NullIdentityStore) ListUsersPagesWithContext(ctx aws.Context, input *identitystore.ListUsersInput, fn func(*identitystore.ListUsersOutput, bool) bool, opts ...request.Option) error {
	return nil
}
