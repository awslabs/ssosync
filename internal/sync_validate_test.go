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
	"strings"
	"testing"

	"ssosync/internal/config"
	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	admin "google.golang.org/api/admin/directory/v1"
)

// TestSync_WarnDoesNotAbortCreate drives the real Sync create path with a mapped
// user whose userName (default-mapped from PrimaryEmail) is over the authoritative
// 128-char limit, so validateUser emits a maxLength WARN. It asserts the hard
// constraint: the predicted reject does NOT abort the sync and does NOT skip the
// send — CreateUser is still invoked exactly once and Sync returns no error — and
// a WARN line is emitted (design acceptance criteria 2 and 6).
func TestSync_WarnDoesNotAbortCreate(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	// An over-128-char primary email -> over-length userName after mapping.
	longLocal := strings.Repeat("a", 130)
	longEmail := longLocal + "@example.com"

	cfg := &config.Config{
		IdentityStoreID: "d-1",
		SyncMethod:      "groups",
		AttributeMap:    config.DefaultAttributeMap,
	}

	googleClient := mocks.NewMockGoogleClient(t)
	// queryUsers="users" fetches exactly this one new user; queryGroups=""
	// skips group fetch, so no membership path runs.
	googleClient.EXPECT().GetUsers("users", cfg.UserFilter).
		Return([]*admin.User{gUser("gid-1", longEmail, "Jane", "Doe", false)}, nil)

	identityStore := mocks.NewMockIdentityStoreAPI(t)
	// No existing AWS groups or users -> the google user becomes a create.
	identityStore.EXPECT().ListGroups(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupsOutput{}, nil)
	identityStore.EXPECT().ListUsers(mock.Anything, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{}, nil)

	awsClient := mocks.NewMockAwsClient(t)
	// The send is made despite the predicted reject (no skip, no abort). mockery
	// fails the test if this is not called exactly as expected.
	awsClient.EXPECT().CreateUser(mock.MatchedBy(func(u *interfaces.User) bool {
		return u.Username == longEmail && u.ExternalId.Id == "gid-1"
	})).Return(&interfaces.User{ID: "aws-1", Username: longEmail}, nil)

	s := &syncGSuite{
		aws:           awsClient,
		google:        googleClient,
		identityStore: identityStore,
		cfg:           cfg,
		users:         make(map[string]*interfaces.User),
	}

	// Sync must NOT return an error solely because of the predicted-reject finding.
	assert.NoError(t, s.Sync("", "users"))

	// A WARN line must have been emitted for the over-length userName.
	var sawWarn bool
	for _, e := range hook.AllEntries() {
		if e.Level == log.WarnLevel && e.Data["rule"] == "maxLength" && e.Data["field"] == "userName" {
			sawWarn = true
			assert.Equal(t, "user", e.Data["kind"])
			assert.Equal(t, longEmail, e.Data["id"])
		}
	}
	assert.True(t, sawWarn, "expected a maxLength WARN for the over-length userName")
}
