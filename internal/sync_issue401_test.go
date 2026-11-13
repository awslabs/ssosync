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

	"ssosync/internal/config"
	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
)

// issue401Config is the groups sync config used by the #401 tests, with the
// UserFilter cmd/root.go sets when SyncSuspended is false.
func issue401Config() *config.Config {
	return &config.Config{
		IdentityStoreID: "d-1",
		SyncMethod:      "groups",
		UserFilter:      " isSuspended=false isArchived=false",
		AttributeMap: config.AttributeMaps{
			User:  config.DefaultUserAttributeMap,
			Group: config.DefaultGroupAttributeMap,
		},
	}
}

func TestQuoteGoogleQueryValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"plain value", "jane@example.com", `'jane@example.com'`},
		{"apostrophe", "jane.o'brien@example.com", `'jane.o\'brien@example.com'`},
		{"space", "/Sales Team", `'/Sales Team'`},
		{"space and apostrophe", "Valentine's Day", `'Valentine\'s Day'`},
		{"several apostrophes", "a'b'c", `'a\'b\'c'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, quoteGoogleQueryValue(tt.value))
		})
	}
}

// TestSync_Issue401_ApostropheEmailMemberSynced covers awslabs/ssosync#401: a
// group member whose email contains an apostrophe is looked up with a quoted,
// escaped users.list query (the filter is still appended server-side by
// getUsers as subQuery+filter) and is synced like any other member.
func TestSync_Issue401_ApostropheEmailMemberSynced(t *testing.T) {
	cfg := issue401Config()

	googleClient := mocks.NewMockGoogleClient(t)
	group := &admin.Group{Id: "gg1", Name: "Group1", Email: "g1@example.com"}
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{group}, nil)
	googleClient.EXPECT().GetGroupMembers(group).Return([]*admin.Member{
		{Email: "jane.o'brien@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil)

	// Capture the exact query string handed to the Google client.
	var gotQuery, gotFilter string
	googleClient.EXPECT().GetUsers(`email='jane.o\'brien@example.com'`, cfg.UserFilter).
		Run(func(query string, filter string) {
			gotQuery, gotFilter = query, filter
		}).
		Return([]*admin.User{gUser("gid-1", "jane.o'brien@example.com", "Jane", "O'Brien", false)}, nil).Once()

	// Jane already exists in the Identity Store (matched by ExternalId); the
	// group is new, so it is created and Jane is added to it.
	identityStore := mocks.NewMockIdentityStoreAPI(t)
	identityStore.EXPECT().ListGroups(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupsOutput{}, nil)
	identityStore.EXPECT().ListUsers(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{Users: []types.User{
			sdkUser("is-1", "jane.o'brien@example.com", "Jane", "O'Brien", "gid-1"),
		}}, nil)

	awsClient := mocks.NewMockAwsClient(t)
	awsClient.EXPECT().FindUserByEmail("jane.o'brien@example.com").
		Return(&interfaces.User{ID: "is-1", Username: "jane.o'brien@example.com", Active: true}, nil)
	awsClient.EXPECT().CreateGroup(mock.MatchedBy(func(g *interfaces.Group) bool {
		return g.DisplayName == "Group1"
	})).Return(&interfaces.Group{ID: "grp-1", DisplayName: "Group1"}, nil)
	awsClient.EXPECT().AddUserToGroup(
		mock.MatchedBy(func(u *interfaces.User) bool { return u.ID == "is-1" }),
		mock.MatchedBy(func(g *interfaces.Group) bool { return g.ID == "grp-1" }),
	).Return(nil).Once()

	s := &syncGSuite{
		aws:           awsClient,
		google:        googleClient,
		identityStore: identityStore,
		cfg:           cfg,
		users:         make(map[string]*interfaces.User),
	}

	require.NoError(t, s.Sync("group-q", ""))

	assert.Equal(t, `email='jane.o\'brien@example.com'`, gotQuery)
	assert.Equal(t, `email='jane.o\'brien@example.com' isSuspended=false isArchived=false`, gotQuery+gotFilter)
}

// TestSync_Issue401_MemberLookupErrorFailsRunAndLogsContext pins the agreed
// error handling: a failed member lookup still fails the run immediately (the
// next member is never looked up and no AWS call is made), and the error log
// names the member, the group and the exact query with the filter.
func TestSync_Issue401_MemberLookupErrorFailsRunAndLogsContext(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	cfg := issue401Config()
	googleErr := &googleapi.Error{Code: 400, Message: "Invalid Input: query, invalid"}

	googleClient := mocks.NewMockGoogleClient(t)
	group := &admin.Group{Id: "gg1", Name: "Group1", Email: "g1@example.com"}
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{group}, nil)
	googleClient.EXPECT().GetGroupMembers(group).Return([]*admin.Member{
		{Email: "jane.o'brien@example.com", Type: "USER", Status: "ACTIVE"},
		{Email: "bob@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil)
	googleClient.EXPECT().GetUsers(`email='jane.o\'brien@example.com'`, cfg.UserFilter).
		Return(nil, googleErr).Once()
	// No expectation for bob: the strict mock fails the test if he is looked up.
	// No Identity Store or AWS expectations: Sync must abort before reaching them.

	s := &syncGSuite{
		aws:           mocks.NewMockAwsClient(t),
		google:        googleClient,
		identityStore: mocks.NewMockIdentityStoreAPI(t),
		cfg:           cfg,
		users:         make(map[string]*interfaces.User),
	}

	err := s.Sync("group-q", "")
	require.Error(t, err)
	var gerr *googleapi.Error
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, 400, gerr.Code)

	var entry *log.Entry
	for _, e := range hook.AllEntries() {
		if e.Level == log.ErrorLevel && e.Data["func"] == "getGoogleUsersInGroup" {
			entry = e
			break
		}
	}
	require.NotNil(t, entry, "expected an error log entry for the failed member lookup")
	assert.Equal(t, "jane.o'brien@example.com", entry.Data["Member.Email"])
	assert.Equal(t, "gg1", entry.Data["GroupId"])
	assert.Equal(t, "g1@example.com", entry.Data["GroupEmail"])
	assert.Equal(t, `email='jane.o\'brien@example.com' isSuspended=false isArchived=false`, entry.Data["query"])
	assert.Equal(t, googleErr, entry.Data["error"])
}

// TestGetGoogleGroupsAndUsers_Issue401_PrecacheOrgUnitQueriesQuoted checks the
// PrecacheOrgUnits queries: every OU path is single-quoted with ' escaped, and
// each OU gets its own query (#404), so no query starts with a comma (the
// strict mock fails any call not listed here).
func TestGetGoogleGroupsAndUsers_Issue401_PrecacheOrgUnitQueriesQuoted(t *testing.T) {
	cfg := issue401Config()
	cfg.PrecacheOrgUnits = []string{"/", " /O'Brien Team/ ", "/Sales"}

	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetGroups("*").Return(nil, nil)
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/'`, cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/O\'Brien Team'`, cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/Sales'`, cfg.UserFilter).Return(nil, nil).Once()
	googleClient.EXPECT().GetGroups("group-q").Return(nil, nil)

	s := &syncGSuite{
		google: googleClient,
		cfg:    cfg,
		users:  make(map[string]*interfaces.User),
	}

	_, _, _, err := s.getGoogleGroupsAndUsers("group-q", "")
	require.NoError(t, err)
}

// TestOutputCacheMetrics_Issue401_OrgUnitQueryQuoted checks the cache-metrics
// OU query: the OU path is single-quoted with ' escaped.
func TestOutputCacheMetrics_Issue401_OrgUnitQueryQuoted(t *testing.T) {
	cfg := issue401Config()
	cfg.CacheMetrics = true

	googleClient := mocks.NewMockGoogleClient(t)
	googleClient.EXPECT().GetUsers(`OrgUnitPath='/O\'Brien Team'`, cfg.UserFilter).
		Return([]*admin.User{{}, {}}, nil).Once()

	entry := &CacheMetrics{}
	s := &syncGSuite{
		google:       googleClient,
		cfg:          cfg,
		cacheMetrics: map[string]*CacheMetrics{"/O'Brien Team": entry},
	}

	s.outputCacheMetrics()
	assert.Equal(t, 2, entry.TotalUsers)
}
