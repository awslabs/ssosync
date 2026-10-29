// Copyright (c) 2020, Amazon.com, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"testing"

	"ssosync/internal/config"
	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
)

// remappedUsernameConfig maps Username from name.givenName instead of
// primaryEmail, so the mapped identity ("Jane") differs from PrimaryEmail
// ("jane@example.com").
func remappedUsernameConfig() *config.Config {
	userMap := config.AttributeMap{}
	for k, v := range config.DefaultUserAttributeMap {
		userMap[k] = v
	}
	userMap["Username"] = "name.givenName"
	return &config.Config{
		IdentityStoreID: "d-1",
		SyncMethod:      "groups",
		AttributeMap: config.AttributeMaps{
			User:  userMap,
			Group: config.DefaultGroupAttributeMap,
		},
	}
}

// membershipSuite wires a Sync run over one Google group (Group1, gg1) with one
// member (Jane, gid-1, jane@example.com) and the given Identity Store state.
func membershipSuite(t *testing.T, awsGroups []types.Group, awsUsers []types.User, memberships []types.GroupMembership) (*syncGSuite, *mocks.MockAwsClient) {
	t.Helper()
	cfg := remappedUsernameConfig()

	googleClient := mocks.NewMockGoogleClient(t)
	group := &admin.Group{Id: "gg1", Name: "Group1", Email: "g1@example.com"}
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{group}, nil)
	googleClient.EXPECT().GetGroupMembers(mock.Anything).Return([]*admin.Member{
		{Email: "jane@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil)
	googleClient.EXPECT().GetUsers("email=jane@example.com", cfg.UserFilter).
		Return([]*admin.User{gUser("gid-1", "jane@example.com", "Jane", "Doe", false)}, nil)

	identityStore := mocks.NewMockIdentityStoreAPI(t)
	identityStore.EXPECT().ListGroups(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupsOutput{Groups: awsGroups}, nil)
	identityStore.EXPECT().ListUsers(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{Users: awsUsers}, nil)
	if len(awsGroups) > 0 {
		identityStore.EXPECT().ListGroupMemberships(mock.Anything, mock.Anything, mock.Anything).
			Return(&identitystore.ListGroupMembershipsOutput{GroupMemberships: memberships}, nil)
	}

	awsClient := mocks.NewMockAwsClient(t)
	return &syncGSuite{
		aws:           awsClient,
		google:        googleClient,
		identityStore: identityStore,
		cfg:           cfg,
		users:         make(map[string]*interfaces.User),
	}, awsClient
}

// janeAWSUser is Jane as stored in the Identity Store, equal to the mapped
// Google user when userName is "Jane" (sdkUser's DisplayName "Jane Doe" matches
// the DisplayName the mapper derives from the name).
func janeAWSUser(userName string) types.User {
	return sdkUser("is-1", userName, "Jane", "Doe", "gid-1")
}

func memberOf(userID string) types.GroupMembership {
	return types.GroupMembership{MemberId: &types.MemberIdMemberUserId{Value: userID}}
}

func TestSync_RemappedUsername_NewGroupMemberAdd(t *testing.T) {
	s, awsClient := membershipSuite(t, nil, []types.User{janeAWSUser("Jane")}, nil)

	// Active backfill is by the AWS-side Username.
	awsClient.EXPECT().FindUserByEmail("Jane").
		Return(&interfaces.User{ID: "is-1", Username: "Jane", Active: true}, nil)
	awsClient.EXPECT().CreateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.DisplayName == "Group1"
	})).Return(&interfaces.Group{ID: "grp-1", DisplayName: "Group1"}, nil)
	// The member is the pre-fetched Identity Store user (matched by ExternalId),
	// NOT a SCIM lookup by PrimaryEmail (a strict mock fails on that call).
	awsClient.EXPECT().AddUserToGroup(
		mock.MatchedBy(func(u *interfaces.User) bool { return u.ID == "is-1" }),
		mock.MatchedBy(func(g *interfaces.Group) bool { return g.ID == "grp-1" }),
	).Return(nil).Once()

	require.NoError(t, s.Sync("group-q", ""))
}

func TestSync_RemappedUsername_SteadyStateNoMemberChanges(t *testing.T) {
	s, awsClient := membershipSuite(t,
		[]types.Group{sdkGroup("grp-1", "Group1", "gg1")},
		[]types.User{janeAWSUser("Jane")},
		[]types.GroupMembership{memberOf("is-1")})

	awsClient.EXPECT().FindUserByEmail("Jane").
		Return(&interfaces.User{ID: "is-1", Username: "Jane", Active: true}, nil)

	// No CreateUser/UpdateUser/CreateGroup/UpdateGroup/AddUserToGroup/
	// RemoveUserFromGroup expectations: the strict mock fails on any of them.
	require.NoError(t, s.Sync("group-q", ""))
}

func TestSync_RemappedUsername_RenameTransitionNoMemberChurn(t *testing.T) {
	// The AWS user still has the old Username but the same ExternalId and is
	// already a group member: the user is renamed, the membership is untouched.
	s, awsClient := membershipSuite(t,
		[]types.Group{sdkGroup("grp-1", "Group1", "gg1")},
		[]types.User{janeAWSUser("jane@example.com")},
		[]types.GroupMembership{memberOf("is-1")})

	awsClient.EXPECT().FindUserByEmail("jane@example.com").
		Return(&interfaces.User{ID: "is-1", Username: "jane@example.com", Active: true}, nil)
	awsClient.EXPECT().UpdateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.ID == "is-1" && u.Username == "Jane" && u.ExternalId.Id == "gid-1"
	})).Return(&interfaces.User{ID: "is-1", Username: "Jane"}, nil).Once()

	require.NoError(t, s.Sync("group-q", ""))
}

func TestSync_RemappedUsername_RemovesStaleMember(t *testing.T) {
	// A different AWS user is a member of the group but not in Google: removed,
	// while Jane (matched by mapped identity) stays.
	other := sdkUser("is-2", "Other", "Other", "Person", "gid-2")
	s, awsClient := membershipSuite(t,
		[]types.Group{sdkGroup("grp-1", "Group1", "gg1")},
		[]types.User{janeAWSUser("Jane"), other},
		[]types.GroupMembership{memberOf("is-1"), memberOf("is-2")})

	awsClient.EXPECT().FindUserByEmail("Jane").
		Return(&interfaces.User{ID: "is-1", Username: "Jane", Active: true}, nil)
	awsClient.EXPECT().FindUserByEmail("Other").
		Return(&interfaces.User{ID: "is-2", Username: "Other", Active: true}, nil)
	// "Other" is not in Google at all, so the user is deleted (and removed from
	// the group first).
	awsClient.EXPECT().RemoveUserFromGroup(
		mock.MatchedBy(func(u *interfaces.User) bool { return u.ID == "is-2" }),
		mock.MatchedBy(func(g *interfaces.Group) bool { return g.ID == "grp-1" }),
	).Return(nil).Once()
	awsClient.EXPECT().DeleteUser(mock.MatchedBy(func(u *interfaces.User) bool { return u.ID == "is-2" })).
		Return(nil).Once()

	require.NoError(t, s.Sync("group-q", ""))
}

func TestGetGroupUsersOperations_MappedIdentity(t *testing.T) {
	remap := remappedUsernameConfig().AttributeMap.User

	jane := &admin.User{Id: "gid-1", PrimaryEmail: "jane@example.com", Name: &admin.UserName{GivenName: "Jane", FamilyName: "Doe"}}
	bob := &admin.User{Id: "gid-2", PrimaryEmail: "bob@example.com", Name: &admin.UserName{GivenName: "Bob", FamilyName: "Roe"}}

	tests := []struct {
		name       string
		google     []*admin.User
		aws        []*interfaces.User
		wantAdd    []string // AWS IDs
		wantDelete []string
		wantEqual  []string
		lookups    map[string]string // FindUserByEmail username -> AWS ID
	}{
		{
			name:      "ExternalId match with differing Usernames is equal",
			google:    []*admin.User{jane},
			aws:       []*interfaces.User{{ID: "is-1", Username: "somebody-else", ExternalId: interfaces.ExternalId{Id: "gid-1"}}},
			wantEqual: []string{"is-1"},
		},
		{
			name:      "case-only Username difference is equal",
			google:    []*admin.User{jane},
			aws:       []*interfaces.User{{ID: "is-1", Username: "JANE"}},
			wantEqual: []string{"is-1"},
		},
		{
			name:       "empty ExternalIds never match each other",
			google:     []*admin.User{jane},
			aws:        []*interfaces.User{{ID: "is-9", Username: "Different"}},
			wantDelete: []string{"is-9"},
			wantAdd:    []string{"is-new"},
			lookups:    map[string]string{"Jane": "is-new"},
		},
		{
			name:   "different member: added by ExternalId without a SCIM lookup, stale one removed",
			google: []*admin.User{bob},
			aws: []*interfaces.User{
				{ID: "is-1", Username: "Jane", ExternalId: interfaces.ExternalId{Id: "gid-1"}},
			},
			wantDelete: []string{"is-1"},
			wantAdd:    []string{"is-2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			awsClient := mocks.NewMockAwsClient(t)
			for username, id := range tc.lookups {
				awsClient.EXPECT().FindUserByEmail(username).Return(&interfaces.User{ID: id, Username: username}, nil).Once()
			}
			s := &syncGSuite{aws: awsClient, cfg: remappedUsernameConfig()}

			// Pre-fetched AWS users (for resolving adds by ExternalId): the
			// "different member" case has Bob already in Identity Store.
			prefetched := append([]*interfaces.User{}, tc.aws...)
			if tc.name == "different member: added by ExternalId without a SCIM lookup, stale one removed" {
				prefetched = append(prefetched, &interfaces.User{ID: "is-2", Username: "Bob", ExternalId: interfaces.ExternalId{Id: "gid-2"}})
			}
			mapped := mapGoogleUsers(tc.google, remap)
			ids := newMemberIdentities(mapped, remap, prefetched)

			add, del, equal, err := s.getGroupUsersOperations(
				map[string][]*admin.User{"G": tc.google},
				map[string][]*interfaces.User{"G": tc.aws},
				ids)
			require.NoError(t, err)

			idsOf := func(us []*interfaces.User) []string {
				var out []string
				for _, u := range us {
					out = append(out, u.ID)
				}
				return out
			}
			assert.ElementsMatch(t, tc.wantAdd, idsOf(add["G"]), "add")
			assert.ElementsMatch(t, tc.wantDelete, idsOf(del["G"]), "delete")
			assert.ElementsMatch(t, tc.wantEqual, idsOf(equal["G"]), "equal")
		})
	}
}
