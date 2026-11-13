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
	"fmt"
	"testing"

	"ssosync/internal/google"
	"ssosync/internal/mocks"

	"github.com/stretchr/testify/require"
)

// An all-blank GroupMatch or UserMatch fails the run before ssosync reads or
// changes anything in AWS (#414): GetGroups/GetUsers return ErrAllEmptyQuery
// (pinned in internal/google), and Sync returns it. Treated as "" it would list
// no groups, and the groups sync would delete every unmatched AWS group. The
// AWS and Identity Store mocks are strict with no expectations, so any call,
// and in particular any delete, fails the test.
func TestSync_Issue414_AllBlankMatchAbortsBeforeAWS(t *testing.T) {
	tests := []struct {
		name, groups, users string
		blankUsers          bool
	}{
		{name: "group match", groups: " , "},
		{name: "user match", groups: "group-q", users: ",", blankUsers: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			googleClient := mocks.NewMockGoogleClient(t)
			s := issue404Sync(googleClient, nil)
			s.aws = mocks.NewMockAwsClient(t)
			s.identityStore = mocks.NewMockIdentityStoreAPI(t)

			if tt.blankUsers {
				googleClient.EXPECT().GetUsers(tt.users, s.cfg.UserFilter).
					Return(nil, fmt.Errorf("users %w: %q", google.ErrAllEmptyQuery, tt.users)).Once()
			} else {
				googleClient.EXPECT().GetGroups(tt.groups).
					Return(nil, fmt.Errorf("groups %w: %q", google.ErrAllEmptyQuery, tt.groups)).Once()
			}

			err := s.Sync(tt.groups, tt.users)
			require.ErrorIs(t, err, google.ErrAllEmptyQuery)
		})
	}
}
