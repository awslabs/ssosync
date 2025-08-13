package interfaces

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
)

// IdentityStoreAPI interface for AWS Identity Store operations
type IdentityStoreAPI interface {
	CreateGroup(ctx context.Context, params *identitystore.CreateGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupOutput, error)
	CreateGroupMembership(ctx context.Context, params *identitystore.CreateGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error)
	DeleteGroup(ctx context.Context, params *identitystore.DeleteGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteGroupOutput, error)
	DeleteGroupMembership(ctx context.Context, params *identitystore.DeleteGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteGroupMembershipOutput, error)
	DeleteUser(ctx context.Context, params *identitystore.DeleteUserInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteUserOutput, error)
	GetGroupMembershipId(ctx context.Context, params *identitystore.GetGroupMembershipIdInput, optFns ...func(*identitystore.Options)) (*identitystore.GetGroupMembershipIdOutput, error)
	IsMemberInGroups(ctx context.Context, params *identitystore.IsMemberInGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.IsMemberInGroupsOutput, error)
	ListGroupMemberships(ctx context.Context, params *identitystore.ListGroupMembershipsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupMembershipsOutput, error)
	ListGroups(ctx context.Context, params *identitystore.ListGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error)
	ListUsers(ctx context.Context, params *identitystore.ListUsersInput, optFns ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error)
}

// User represents a user in the system
type User struct {
	ID          string        `json:"id"`
	Schemas     []string      `json:"schemas"`
	Username    string        `json:"userName"`
	Name        UserName      `json:"name"`
	DisplayName string        `json:"displayName"`
	Emails      []UserEmail   `json:"emails"`
	Addresses   []UserAddress `json:"addresses"`
	Active      bool          `json:"active"`
}

// UserName represents the name components of a user
type UserName struct {
	FamilyName string `json:"familyName"`
	GivenName  string `json:"givenName"`
}

// UserEmail represents an email address for a user
type UserEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type"`
	Primary bool   `json:"primary"`
}

// UserAddress represents an address for a user
type UserAddress struct {
	Type string `json:"type"`
}

// Group represents a group in the system
type Group struct {
	ID          string   `json:"id"`
	Schemas     []string `json:"schemas"`
	DisplayName string   `json:"displayName"`
	Members     []string `json:"members"`
}