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

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
)

const msg413CacheStats = "Caching Performance, orgPaths with a high Miss rate compared with the TotalUsers consider precaching"

// issue413Sync is a syncGSuite with --cache-metrics set.
func issue413Sync(googleClient *mocks.MockGoogleClient) *syncGSuite {
	cfg := issue401Config()
	cfg.CacheMetrics = true
	return &syncGSuite{google: googleClient, cfg: cfg}
}

// Each op is counted once on its own OU; without --cache-metrics nothing is kept.
func TestRecordCacheOp_Issue413_CountsOncePerOp(t *testing.T) {
	s := issue413Sync(nil)
	s.recordCacheOp("/Sales", PreCache)
	s.recordCacheOp("/Sales", CacheHit)
	s.recordCacheOp("/Sales", CacheHit)
	s.recordCacheOp("/Sales", CacheMiss)
	require.Contains(t, s.cacheMetrics, "/Sales")
	assert.Equal(t, CacheMetrics{Precaches: 1, Hits: 2, Misses: 1}, *s.cacheMetrics["/Sales"])

	off := &syncGSuite{cfg: issue401Config()}
	off.recordCacheOp("/Sales", CacheHit)
	assert.Nil(t, off.cacheMetrics)
}

// Only the user's own OU is updated: no credit to /Sales from /Sales/EMEA or
// /Sales Team, whatever the order the OUs are first seen in.
func TestRecordCacheOp_Issue413_StoresOwnCounts(t *testing.T) {
	s := issue413Sync(nil)
	s.recordCacheOp("/Sales", CacheHit)
	s.recordCacheOp("/Sales/EMEA", CacheMiss)
	s.recordCacheOp("/Sales Team", CacheMiss)

	assert.Equal(t, map[string]*CacheMetrics{
		"/Sales":      {Hits: 1},
		"/Sales/EMEA": {Misses: 1},
		"/Sales Team": {Misses: 1},
	}, s.cacheMetrics)
}

func TestOrgUnitInTree_Issue413(t *testing.T) {
	tests := []struct {
		ou, orgPath string
		want        bool
	}{
		{"/", "/", true},
		{"/", "/X", true},
		{"/Sales", "/Sales", true},
		{"/Sales", "/Sales/EMEA", true},
		{"/Sales/", "/Sales/EMEA", true},
		{"/Sales", "/Sales Team", false},
		{"/Sales", "/Presales", false},
		{"/Sales", "/B/Sales", false},
		{"/Sales/EMEA", "/Sales", false},
		{"/A", "/AB", false},
	}
	for _, tt := range tests {
		t.Run(tt.ou+" holds "+tt.orgPath, func(t *testing.T) {
			assert.Equal(t, tt.want, orgUnitInTree(tt.ou, tt.orgPath))
		})
	}
}

// Each OU's counts are its own plus its sub-OUs' (by path, not by substring);
// TotalUsers is copied, not summed; the input is left as it was.
func TestRollUpCacheMetrics_Issue413(t *testing.T) {
	own := map[string]*CacheMetrics{
		"/":           {Hits: 1},
		"/Sales":      {Misses: 1, TotalUsers: 7},
		"/Sales/EMEA": {Precaches: 1, Hits: 2},
		"/Sales Team": {Misses: 4},
		"/Presales":   {Hits: 8},
		"/B/Sales":    {Misses: 16},
	}
	ownBefore := map[string]CacheMetrics{}
	for k, v := range own {
		ownBefore[k] = *v
	}

	assert.Equal(t, map[string]*CacheMetrics{
		"/":           {Precaches: 1, Hits: 11, Misses: 21},
		"/Sales":      {Precaches: 1, Hits: 2, Misses: 1, TotalUsers: 7},
		"/Sales/EMEA": {Precaches: 1, Hits: 2},
		"/Sales Team": {Misses: 4},
		"/Presales":   {Hits: 8},
		"/B/Sales":    {Misses: 16},
	}, rollUpCacheMetrics(own))

	for k, v := range own {
		assert.Equal(t, ownBefore[k], *v, k)
	}
}

// The default_map run (see the plan's worked example): Chris in /, Rimmer in
// /Maintenance, Dave in /Maintenance/Vending Machines, Olaf precached in
// /Red Dwarf/Petersen's Locker; /Catering is empty, so never a key, and
// /Red Dwarf has no users of its own, so it is not reported.
func TestRollUpCacheMetrics_Issue413_TestUsers(t *testing.T) {
	own := map[string]*CacheMetrics{
		"/":                             {Hits: 4, Misses: 1},
		"/Maintenance":                  {Hits: 4, Misses: 1},
		"/Maintenance/Vending Machines": {Hits: 2, Misses: 1},
		"/Red Dwarf/Petersen's Locker":  {Precaches: 1, Hits: 1},
	}
	assert.Equal(t, map[string]*CacheMetrics{
		"/":                             {Precaches: 1, Hits: 11, Misses: 3},
		"/Maintenance":                  {Hits: 6, Misses: 2},
		"/Maintenance/Vending Machines": {Hits: 2, Misses: 1},
		"/Red Dwarf/Petersen's Locker":  {Precaches: 1, Hits: 1},
	}, rollUpCacheMetrics(own))
}

// A member that had to be looked up counts as a CacheMiss only; a member found
// in the user cache counts as a CacheHit.
func TestGetGoogleUsersInGroup_Issue413_LookedUpMemberIsMissOnly(t *testing.T) {
	a, b := issue404Users()
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	googleClient.EXPECT().GetGroupMembers(issue404Group).Return([]*admin.Member{
		{Email: "a@example.com", Type: "USER", Status: "ACTIVE"},
		{Email: "b@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil).Once()
	googleClient.EXPECT().GetUsers(`email='a@example.com'`, s.cfg.UserFilter).Return([]*admin.User{a}, nil).Once()
	userCache := map[string]*admin.User{b.PrimaryEmail: b}

	members, err := s.getGoogleUsersInGroup(issue404Group, userCache, map[string]*admin.Group{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []*admin.User{a, b}, members)

	assert.Equal(t, map[string]*CacheMetrics{
		"/A": {Misses: 1},
		"/B": {Hits: 1},
	}, s.cacheMetrics)
}

// The Cache Stats log field carries the rolled-up counts with each OU's
// TotalUsers; s.cacheMetrics keeps the own counts.
func TestOutputCacheMetrics_Issue413_LogsRolledUpCounts(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	s.cacheMetrics = map[string]*CacheMetrics{"/": {Hits: 1}, "/A": {Misses: 1}}
	googleClient.EXPECT().CountUsers(`OrgUnitPath='/'`, s.cfg.UserFilter).
		Return(3, nil).Once()
	googleClient.EXPECT().CountUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).
		Return(1, nil).Once()

	s.outputCacheMetrics()

	var stats []*log.Entry
	for _, e := range hook.AllEntries() {
		if e.Level == log.InfoLevel && e.Message == msg413CacheStats {
			stats = append(stats, e)
		}
	}
	require.Len(t, stats, 1)
	assert.Equal(t, map[string]*CacheMetrics{
		"/":  {Hits: 1, Misses: 1, TotalUsers: 3},
		"/A": {Misses: 1, TotalUsers: 1},
	}, stats[0].Data["Cache Stats"])
	assert.Equal(t, 1, s.cacheMetrics["/"].Hits)
	assert.Equal(t, 0, s.cacheMetrics["/"].Misses)
}
