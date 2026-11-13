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
	"ssosync/internal/mocks"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
)

var issue418Group2 = &admin.Group{Id: "gg2", Name: "Group2", Email: "g2@example.com"}

func issue418Members(emails ...string) []*admin.Member {
	members := make([]*admin.Member, 0, len(emails))
	for _, e := range emails {
		members = append(members, &admin.Member{Email: e, Type: "USER", Status: "ACTIVE"})
	}
	return members
}

func issue418User(email, orgUnitPath string) *admin.User {
	u := gUser("gid-"+email, email, "Given", "Family", false)
	u.OrgUnitPath = orgUnitPath
	return u
}

func TestUserCacheKey_Issue418(t *testing.T) {
	assert.Equal(t, "acehole@example.com", userCacheKey("AceHole@Example.COM"))
}

// A member listed in a different case from the primary email is found in the
// cache: no lookup (the strict mock has no GetUsers), one Hit.
func TestGetGoogleUsersInGroup_Issue418_CaseDifferentMemberHitsCache(t *testing.T) {
	u := issue418User("AceHole@example.com", "/M")
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("acehole@example.com"), nil).Once()
	userCache := map[string]*admin.User{}
	cacheUser(userCache, u)

	members, err := s.getGoogleUsersInGroup(issue404Group, userCache, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.Equal(t, []*admin.User{u}, members)
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Hits: 1}}, s.cacheMetrics)
}

// Members listed by an alias or a non-editable alias, in any case, are found
// in the cache without a lookup.
func TestGetGoogleUsersInGroup_Issue418_AliasMemberHitsCache(t *testing.T) {
	u := issue418User("acehole@example.com", "/M")
	u.Aliases = []string{"rimmer@example.com"}
	u.NonEditableAliases = []string{"acehole@alias.example.com"}
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("Rimmer@example.com"), nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue418Group2).Return(issue418Members("acehole@alias.example.com"), nil).Once()
	userCache := map[string]*admin.User{}
	cacheUser(userCache, u)

	for _, g := range []*admin.Group{issue404Group, issue418Group2} {
		members, err := s.getGoogleUsersInGroup(g, userCache, map[string]*admin.Group{})
		require.NoError(t, err)
		assert.Equal(t, []*admin.User{u}, members, g.Name)
	}
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Hits: 2}}, s.cacheMetrics)
}

// A cached user listed by an address Google does not report as an alias is
// looked up once (one Miss), and the cached entry is kept; the address is then
// keyed, so the next membership is a Hit with no lookup.
func TestGetGoogleUsersInGroup_Issue418_LookupRecordedOnceAsMiss(t *testing.T) {
	a := issue418User("a@example.com", "/A")
	aAgain := issue418User("a@example.com", "/A")
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("old@example.com"), nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue418Group2).Return(issue418Members("old@example.com"), nil).Once()
	googleClient.EXPECT().GetUsers(`email='old@example.com'`, s.cfg.UserFilter).Return([]*admin.User{aAgain}, nil).Once()
	userCache := map[string]*admin.User{}
	cacheUser(userCache, a)

	for _, g := range []*admin.Group{issue404Group, issue418Group2} {
		members, err := s.getGoogleUsersInGroup(g, userCache, map[string]*admin.Group{})
		require.NoError(t, err)
		require.Len(t, members, 1, g.Name)
		assert.Same(t, a, members[0], g.Name)
	}
	assert.Equal(t, map[string]*CacheMetrics{"/A": {Misses: 1, Hits: 1}}, s.cacheMetrics)
}

// A mixed-case member that is not cached is looked up once: exactly one Miss
// and no Hit; its next membership is exactly one Hit.
func TestGetGoogleUsersInGroup_Issue418_OneOpPerMembership(t *testing.T) {
	u := issue418User("acehole@example.com", "/M")
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("AceHole@example.com"), nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue418Group2).Return(issue418Members("ACEHOLE@example.com"), nil).Once()
	googleClient.EXPECT().GetUsers(`email='AceHole@example.com'`, s.cfg.UserFilter).Return([]*admin.User{u}, nil).Once()
	userCache := map[string]*admin.User{}

	members, err := s.getGoogleUsersInGroup(issue404Group, userCache, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.Equal(t, []*admin.User{u}, members)
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Misses: 1}}, s.cacheMetrics)

	members, err = s.getGoogleUsersInGroup(issue418Group2, userCache, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.Equal(t, []*admin.User{u}, members)
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Misses: 1, Hits: 1}}, s.cacheMetrics)
}

// A precached mixed-case primary email is found for a lower-case member: no
// email= lookup, and the user is returned once.
func TestGetGoogleGroupsAndUsers_Issue418_PrecachedMixedCaseNoLookup(t *testing.T) {
	u := issue418User("AceHole@example.com", "/M")
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{issue404Group}, nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return(issue418Members("acehole@example.com"), nil).Once()
	s := issue404Sync(googleClient, []string{"/M"})
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/M'`, s.cfg.UserFilter).Return([]*admin.User{u}, nil).Once()

	_, users, groupsUsers, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Same(t, u, users[0])
	assert.Equal(t, []*admin.User{u}, groupsUsers[issue404Group.Name])
	assert.Equal(t, map[string]*CacheMetrics{"/M": {Precaches: 1, Hits: 1}}, s.cacheMetrics)
}

// A mixed-case --ignore-users entry (lower-cased when the config loads) still
// ignores the member, whatever case the group lists it in, with no lookup.
func TestIgnoreUser_Issue418_MixedCaseEntry(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	s.cfg.IgnoreUsers = config.LowerCaseAll([]string{"AceHole@Example.com"})
	googleClient.EXPECT().GetGroupMembers(issue404Group).
		Return(issue418Members("acehole@example.com", "ACEHOLE@example.com"), nil).Once()

	members, err := s.getGoogleUsersInGroup(issue404Group, map[string]*admin.User{}, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.Empty(t, members)
	assert.Len(t, issue404Entries(hook, log.InfoLevel, "skip: ignore list"), 2)
}
