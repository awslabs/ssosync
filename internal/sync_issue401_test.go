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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
)

// TestSync_Issue401_ApostropheEmailAbortsSync reproduces awslabs/ssosync#401
// against the CURRENT behaviour on master (it passes today, pinning the bug):
//
//  1. getGoogleUsersInGroup builds the users.list query for a member as
//     "email="+m.Email with no quoting or escaping, so an address containing
//     an apostrophe is sent to Google verbatim as email=jane.o'brien@example.com.
//     Google's search syntax requires a quoted value with \' escaping, and the
//     live API rejects the unquoted form with 400 "Invalid Input: query, invalid"
//     (simulated here by the mock).
//  2. That single member lookup failure aborts the whole Sync: the next member
//     (bob) is never looked up and Sync returns the Google error before any AWS
//     call is made.
//
// The user's decision is that a failed member lookup must be logged and
// skipped. When the fix lands, flip this test: expect the quoted/escaped query
// (or users.get), expect bob to be looked up, and expect Sync to continue.
func TestSync_Issue401_ApostropheEmailAbortsSync(t *testing.T) {
	cfg := &config.Config{
		IdentityStoreID: "d-1",
		SyncMethod:      "groups",
		// Same value cmd/root.go sets when SyncSuspended is false.
		UserFilter: " isSuspended=false isArchived=false",
		AttributeMap: config.AttributeMaps{
			User:  config.DefaultUserAttributeMap,
			Group: config.DefaultGroupAttributeMap,
		},
	}

	googleErr := &googleapi.Error{Code: 400, Message: "Invalid Input: query, invalid"}

	googleClient := mocks.NewMockGoogleClient(t)
	group := &admin.Group{Id: "gg1", Name: "Group1", Email: "g1@example.com"}
	googleClient.EXPECT().GetGroups("group-q").Return([]*admin.Group{group}, nil)
	googleClient.EXPECT().GetGroupMembers(group).Return([]*admin.Member{
		{Email: "jane.o'brien@example.com", Type: "USER", Status: "ACTIVE"},
		{Email: "bob@example.com", Type: "USER", Status: "ACTIVE"},
	}, nil)

	// Capture the exact query string handed to the Google client.
	var gotQuery, gotFilter string
	googleClient.EXPECT().GetUsers("email=jane.o'brien@example.com", cfg.UserFilter).
		Run(func(query string, filter string) {
			gotQuery, gotFilter = query, filter
		}).
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

	// (1) The query is unquoted and unescaped, and the filter is appended to it
	// in getUsers as subQuery+filter, giving
	// email=jane.o'brien@example.com isSuspended=false isArchived=false
	assert.Equal(t, "email=jane.o'brien@example.com", gotQuery)
	assert.Equal(t, "email=jane.o'brien@example.com isSuspended=false isArchived=false", gotQuery+gotFilter)

	// (2) The single member failure aborts the whole sync with the Google error.
	require.Error(t, err)
	var gerr *googleapi.Error
	require.True(t, errors.As(err, &gerr))
	assert.Equal(t, 400, gerr.Code)
}
