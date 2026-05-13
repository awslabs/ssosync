package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/awslabs/ssosync/internal/interfaces"
	log "github.com/sirupsen/logrus"
)

type DryIdentityStore struct {
	client interfaces.IdentityStoreAPI
}

func NewDryIdentityStore(client interfaces.IdentityStoreAPI) interfaces.IdentityStoreAPI {
	return &DryIdentityStore{
		client: client,
	}
}

// **********************
// Dry-run implementations - log what would happen but don't execute
// **********************

func (d *DryIdentityStore) CreateGroup(ctx context.Context, params *identitystore.CreateGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupOutput, error) {
	log.WithField("displayName", *params.DisplayName).Info("DRY RUN: Would create group")
	return &identitystore.CreateGroupOutput{
		GroupId:         aws.String(virtualGroupID(*params.DisplayName)),
		IdentityStoreId: params.IdentityStoreId,
	}, nil
}

func (d *DryIdentityStore) CreateGroupMembership(ctx context.Context, params *identitystore.CreateGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error) {
	memberValue := memberIDValue(params.MemberId)
	log.WithFields(log.Fields{
		"groupId": *params.GroupId,
		"userId":  memberValue,
	}).Info("DRY RUN: Would create group membership")
	return &identitystore.CreateGroupMembershipOutput{
		MembershipId:    aws.String(virtualMembershipID(*params.GroupId, memberValue)),
		IdentityStoreId: params.IdentityStoreId,
	}, nil
}

func (d *DryIdentityStore) DeleteGroup(ctx context.Context, params *identitystore.DeleteGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteGroupOutput, error) {
	log.WithField("groupId", *params.GroupId).Info("DRY RUN: Would delete group")
	return &identitystore.DeleteGroupOutput{}, nil
}

func (d *DryIdentityStore) DeleteGroupMembership(ctx context.Context, params *identitystore.DeleteGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteGroupMembershipOutput, error) {
	log.WithField("membershipId", *params.MembershipId).Info("DRY RUN: Would delete group membership")
	return &identitystore.DeleteGroupMembershipOutput{}, nil
}

func (d *DryIdentityStore) DeleteUser(ctx context.Context, params *identitystore.DeleteUserInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteUserOutput, error) {
	log.WithField("userId", *params.UserId).Info("DRY RUN: Would delete user")
	return &identitystore.DeleteUserOutput{}, nil
}

// GetGroupMembershipId short-circuits when either the group or member is virtual
// so the real client is never called with a synthetic ID.
func (d *DryIdentityStore) GetGroupMembershipId(ctx context.Context, params *identitystore.GetGroupMembershipIdInput, optFns ...func(*identitystore.Options)) (*identitystore.GetGroupMembershipIdOutput, error) {
	memberValue := memberIDValue(params.MemberId)
	if isVirtualID(*params.GroupId) || isVirtualID(memberValue) {
		return &identitystore.GetGroupMembershipIdOutput{
			MembershipId:    aws.String(virtualMembershipID(*params.GroupId, memberValue)),
			IdentityStoreId: params.IdentityStoreId,
		}, nil
	}
	return d.client.GetGroupMembershipId(ctx, params, optFns...)
}

// IsMemberInGroups short-circuits when the member or any group is virtual —
// a virtual user was never actually added, so membership is always false.
func (d *DryIdentityStore) IsMemberInGroups(ctx context.Context, params *identitystore.IsMemberInGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.IsMemberInGroupsOutput, error) {
	memberValue := memberIDValue(params.MemberId)
	if !isVirtualID(memberValue) && !slices.ContainsFunc(params.GroupIds, isVirtualID) {
		return d.client.IsMemberInGroups(ctx, params, optFns...)
	}
	results := make([]types.GroupMembershipExistenceResult, len(params.GroupIds))
	for i, gid := range params.GroupIds {
		results[i] = types.GroupMembershipExistenceResult{
			GroupId:          aws.String(gid),
			MembershipExists: false,
		}
	}
	return &identitystore.IsMemberInGroupsOutput{Results: results}, nil
}

// ListGroupMemberships short-circuits when the group is virtual — it has no
// real memberships to enumerate.
func (d *DryIdentityStore) ListGroupMemberships(ctx context.Context, params *identitystore.ListGroupMembershipsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupMembershipsOutput, error) {
	if isVirtualID(*params.GroupId) {
		return &identitystore.ListGroupMembershipsOutput{}, nil
	}
	return d.client.ListGroupMemberships(ctx, params, optFns...)
}

func (d *DryIdentityStore) ListGroups(ctx context.Context, params *identitystore.ListGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
	return d.client.ListGroups(ctx, params, optFns...)
}

func (d *DryIdentityStore) ListUsers(ctx context.Context, params *identitystore.ListUsersInput, optFns ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error) {
	return d.client.ListUsers(ctx, params, optFns...)
}

func (d *DryIdentityStore) CreateUser(ctx context.Context, params *identitystore.CreateUserInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error) {
	log.WithField("userName", *params.UserName).Info("DRY RUN: Would create user")
	return &identitystore.CreateUserOutput{
		UserId:          aws.String(virtualUserID(*params.UserName)),
		IdentityStoreId: params.IdentityStoreId,
	}, nil
}

func memberIDValue(mid types.MemberId) string {
	if mid == nil {
		return ""
	}
	if m, ok := mid.(*types.MemberIdMemberUserId); ok {
		return m.Value
	}
	return ""
}
