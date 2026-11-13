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

	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
)

// Log messages of the PrecacheOrgUnits user precache (#404).
const (
	msg404OUQueryFailed = "PrecacheOrgUnits: OrgUnitPath could not be queried and is skipped; it may not exist"
	msg404OUNoUsers     = "PrecacheOrgUnits: no users found in OrgUnitPath; it may not exist or have no users matching the filter"
	msg404AllFailed     = "Precaching failed, caching on the fly"
	msg404NoUsers       = "Precaching return no users? Switching to caching on the fly"
)

// issue404Users are two users in different OUs, members of issue404Group.
func issue404Users() (a, b *admin.User) {
	a = gUser("gid-a", "a@example.com", "Ann", "A", false)
	a.OrgUnitPath = "/A"
	b = gUser("gid-b", "b@example.com", "Bob", "B", false)
	b.OrgUnitPath = "/B"
	return a, b
}

var issue404Group = &admin.Group{Id: "gg1", Name: "Group1", Email: "g1@example.com"}

// issue404Mock returns a strict Google mock that serves the group precache
// and one group (group-q) with members a@ and b@. Each test adds the per-OU
// GetUsers calls, plus any email= lookups of members that were not precached.
func issue404Mock(t *testing.T) *mocks.MockGoogleClient {
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{issue404Group}, nil).Once()
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return([]*admin.Member{
		{Email: "a@example.com", Type: "USER", Status: "ACTIVE"},
		{Email: "b@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil).Once()
	return googleClient
}

func issue404Sync(googleClient *mocks.MockGoogleClient, orgUnits []string) *syncGSuite {
	cfg := issue401Config()
	cfg.PrecacheOrgUnits = orgUnits
	return &syncGSuite{
		google: googleClient,
		cfg:    cfg,
		users:  make(map[string]*interfaces.User),
	}
}

// issue404Entries returns the log entries with the given level and message.
func issue404Entries(hook *test.Hook, level log.Level, msg string) []*log.Entry {
	var found []*log.Entry
	for _, e := range hook.AllEntries() {
		if e.Level == level && e.Message == msg {
			found = append(found, e)
		}
	}
	return found
}

func issue404OUs(entries []*log.Entry) []interface{} {
	ous := make([]interface{}, 0, len(entries))
	for _, e := range entries {
		ous = append(ous, e.Data["orgUnitPath"])
	}
	return ous
}

// Only the listed OUs are queried, one call each, and their users are cached:
// no member is looked up on the fly and nothing is warned.
func TestGetGoogleGroupsAndUsers_Issue404_PrecachesOnlyListedOrgUnits(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()
	// No email= expectations: the strict mock fails the test if a member is
	// looked up on the fly.

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	for _, e := range hook.AllEntries() {
		assert.NotEqual(t, log.WarnLevel, e.Level, "unexpected warning: %s", e.Message)
		assert.NotEqual(t, log.ErrorLevel, e.Level, "unexpected error: %s", e.Message)
	}
}

// An OU that cannot be queried is skipped with a warning naming it; the other
// OU is still precached and the run carries on.
func TestGetGoogleGroupsAndUsers_Issue404_OrgUnitErrorSkippedWithWarning(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleErr := &googleapi.Error{Code: 400, Message: "Invalid Input: INVALID_OU_ID"}
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(nil, googleErr).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()
	// a@ was not precached, so it is looked up on the fly; b@ was.
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	warns := issue404Entries(hook, log.WarnLevel, msg404OUQueryFailed)
	require.Len(t, warns, 1)
	assert.Equal(t, "/A", warns[0].Data["orgUnitPath"])
	assert.Equal(t, googleErr, warns[0].Data["error"])
	assert.Equal(t, "getGoogleGroupsAndUsers", warns[0].Data["func"])
	assert.Empty(t, issue404Entries(hook, log.ErrorLevel, msg404AllFailed))
	assert.Empty(t, issue404Entries(hook, log.WarnLevel, msg404NoUsers))
}

// An OU that returns no users is skipped with a warning naming it; the other
// OU is still precached.
func TestGetGoogleGroupsAndUsers_Issue404_OrgUnitNoUsersSkippedWithWarning(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{}, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	warns := issue404Entries(hook, log.WarnLevel, msg404OUNoUsers)
	require.Len(t, warns, 1)
	assert.Equal(t, "/A", warns[0].Data["orgUnitPath"])
	assert.Equal(t, s.cfg.UserFilter, warns[0].Data["queryFilters"])
	assert.Equal(t, "getGoogleGroupsAndUsers", warns[0].Data["func"])
	assert.Empty(t, issue404Entries(hook, log.WarnLevel, msg404OUQueryFailed))
	assert.Empty(t, issue404Entries(hook, log.WarnLevel, msg404NoUsers))
}

// When every OU returns no users, each is warned about and the existing
// "no users" warning is logged, but not "Precaching failed".
func TestGetGoogleGroupsAndUsers_Issue404_AllOrgUnitsEmpty(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers(`email='b@example.com'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()

	_, _, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)

	assert.ElementsMatch(t, []interface{}{"/A", "/B"}, issue404OUs(issue404Entries(hook, log.WarnLevel, msg404OUNoUsers)))
	assert.Len(t, issue404Entries(hook, log.WarnLevel, msg404NoUsers), 1)
	assert.Empty(t, issue404Entries(hook, log.ErrorLevel, msg404AllFailed))
}

// When every OU fails, each is warned about and "Precaching failed, caching on
// the fly" is logged; members are looked up on the fly and the run carries on.
func TestGetGoogleGroupsAndUsers_Issue404_AllOrgUnitsFail(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	a, b := issue404Users()
	googleErr := &googleapi.Error{Code: 400, Message: "Invalid Input: INVALID_OU_ID"}
	googleClient := issue404Mock(t)
	s := issue404Sync(googleClient, []string{"/A", "/B"})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(nil, googleErr).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/B'`, s.cfg.UserFilter).Return(nil, googleErr).Once()
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers(`email='b@example.com'`, s.cfg.UserFilter).Return([]*admin.User{b}, nil).Once()

	_, users, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, users)

	assert.ElementsMatch(t, []interface{}{"/A", "/B"}, issue404OUs(issue404Entries(hook, log.WarnLevel, msg404OUQueryFailed)))
	assert.Len(t, issue404Entries(hook, log.ErrorLevel, msg404AllFailed), 1)
	assert.Empty(t, issue404Entries(hook, log.WarnLevel, msg404NoUsers))
}

// Overlapping OUs return the same user twice: the first is recorded as a
// PreCache and the repeat as a CacheHit. Counts are only checked for > 0 / == 0
// because recordCacheOp counts every op more than once (#413).
func TestGetGoogleGroupsAndUsers_Issue404_OverlappingOrgUnitsRepeatIsCacheHit(t *testing.T) {
	a, _ := issue404Users()
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	s := issue404Sync(googleClient, []string{"/", "/A"})
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()

	_, _, _, err := s.getGoogleGroupsAndUsers("", "")
	require.NoError(t, err)

	require.Contains(t, s.cacheMetrics, "/A")
	assert.Greater(t, s.cacheMetrics["/A"].Precaches, 0)
	assert.Greater(t, s.cacheMetrics["/A"].Hits, 0)
	assert.Equal(t, 0, s.cacheMetrics["/A"].Misses)
}

// Empty entries are skipped and an OU listed twice (after normalising) is
// queried once.
func TestGetGoogleGroupsAndUsers_Issue404_EmptyAndDuplicateOrgUnitsSkipped(t *testing.T) {
	a, _ := issue404Users()
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	s := issue404Sync(googleClient, []string{"/A", "", "  ", " /A/ "})
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()

	_, _, _, err := s.getGoogleGroupsAndUsers("", "")
	require.NoError(t, err)
}

// Without PrecacheOrgUnits nothing is precached: no GetGroups("*") and no
// GetUsers call.
func TestGetGoogleGroupsAndUsers_Issue404_NilPrecacheOrgUnitsDisabled(t *testing.T) {
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue404Sync(googleClient, nil)

	_, _, _, err := s.getGoogleGroupsAndUsers("", "")
	require.NoError(t, err)
}
