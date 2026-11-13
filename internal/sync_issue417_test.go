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

	"ssosync/internal/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
)

// --user-match fills the cache like --precache-ous (#417): a new user is a
// precache; a user already precached, or listed twice, keeps the cached entry
// and is a hit.
func TestGetGoogleGroupsAndUsers_Issue417_UserMatchIsPrecache(t *testing.T) {
	a, b := issue404Users()
	aCopy := *a
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	s := issue404Sync(googleClient, []string{"/A"})
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers("name:x", s.cfg.UserFilter).Return([]*admin.User{&aCopy, b, b}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("", "name:x")
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)
	for _, u := range users {
		assert.NotSame(t, &aCopy, u, "the precached entry is kept")
	}

	assert.Equal(t, map[string]*CacheMetrics{
		"/A": {Precaches: 1, Hits: 1},
		"/B": {Precaches: 1, Hits: 1},
	}, s.cacheMetrics)
}

// A group member that --user-match already listed is found in the cache (a
// hit, no lookup); a member it did not list is looked up (a miss).
func TestGetGoogleGroupsAndUsers_Issue417_UserMatchMemberIsHit(t *testing.T) {
	a, b := issue404Users()
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue404Sync(googleClient, nil)
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers("name:x", s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{issue404Group}, nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return([]*admin.Member{
		{Email: "a@example.com", Type: "USER", Status: "ACTIVE"},
		{Email: "b@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil).Once()
	// No email='a@example.com' expectation: the strict mock fails on a lookup.
	googleClient.EXPECT().GetUsers(`email='b@example.com'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "name:x")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	assert.Equal(t, map[string]*CacheMetrics{
		"/A": {Precaches: 1, Hits: 1},
		"/B": {Misses: 1},
	}, s.cacheMetrics)
}
