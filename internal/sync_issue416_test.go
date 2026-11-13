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
	"errors"
	"testing"

	"ssosync/internal/mocks"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
)

const msg416CountFailed = "Failed retrieving usercache count"

// An OU that --precache-ous listed reuses that count as TotalUsers: no
// CountUsers (or second GetUsers) call, which the strict mock would fail.
func TestOutputCacheMetrics_Issue416_ReusesPrecacheCount(t *testing.T) {
	a, b := issue404Users()
	a.OrgUnitPath, b.OrgUnitPath = "/", "/"
	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil).Once()
	s := issue404Sync(googleClient, []string{"/"})
	s.cfg.CacheMetrics = true
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/'`, s.cfg.UserFilter).Return([]*admin.User{a, b}, nil).Once()

	_, _, _, err := s.getGoogleGroupsAndUsers("", "")
	require.NoError(t, err)
	s.outputCacheMetrics()

	require.Contains(t, s.cacheMetrics, "/")
	assert.Equal(t, 2, s.cacheMetrics["/"].TotalUsers)
}

// An OU that was not precached is counted with CountUsers, never GetUsers.
func TestOutputCacheMetrics_Issue416_CountsOtherOUs(t *testing.T) {
	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	s.cacheMetrics = map[string]*CacheMetrics{"/A": {Misses: 1}}
	googleClient.EXPECT().CountUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(5, nil).Once()

	s.outputCacheMetrics()

	assert.Equal(t, 5, s.cacheMetrics["/A"].TotalUsers)
}

// A failed count is logged and leaves TotalUsers at 0.
func TestOutputCacheMetrics_Issue416_CountErrorLogged(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	googleClient := mocks.NewMockGoogleClient(t)
	s := issue413Sync(googleClient)
	s.cacheMetrics = map[string]*CacheMetrics{"/A": {Misses: 1}}
	googleErr := errors.New("boom")
	googleClient.EXPECT().CountUsers(`OrgUnitPath='/A'`, s.cfg.UserFilter).Return(0, googleErr).Once()

	s.outputCacheMetrics()

	errs := issue404Entries(hook, log.ErrorLevel, msg416CountFailed)
	require.Len(t, errs, 1)
	assert.Equal(t, googleErr, errs[0].Data["error"])
	assert.Equal(t, 0, s.cacheMetrics["/A"].TotalUsers)
}
