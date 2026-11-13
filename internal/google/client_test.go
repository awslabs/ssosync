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

package google

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ssosync/internal/mocks"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

func TestNewClient_InvalidCredentials(t *testing.T) {
	ctx := context.Background()
	invalidJSON := []byte("invalid json")

	client, err := NewClient(ctx, "admin@example.com", "my_custmer", Auth{ServiceAccountKey: invalidJSON})
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestGetUsers_EmptyQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetUsers("", "").Return([]*admin.User{}, nil)

	users, err := mockClient.GetUsers("", "")
	assert.NoError(t, err)
	assert.Len(t, users, 0)
}

func TestGetUsers_WildcardQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedUsers := []*admin.User{
		{
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
		},
	}

	mockClient.EXPECT().GetUsers("*", "").Return(expectedUsers, nil)

	users, err := mockClient.GetUsers("*", "")
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "user1@example.com", users[0].PrimaryEmail)
}

func TestGetUsers_SpecificQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedUsers := []*admin.User{
		{
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
		},
	}

	mockClient.EXPECT().GetUsers("name:John", "").Return(expectedUsers, nil)

	users, err := mockClient.GetUsers("name:John", "")
	assert.NoError(t, err)
	assert.Len(t, users, 1)
}

func TestGetUsers_MultipleQueries(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedUsers := []*admin.User{
		{
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
		},
		{
			PrimaryEmail: "user2@example.com",
			Name: &admin.UserName{
				GivenName:  "Jane",
				FamilyName: "Smith",
			},
		},
	}

	mockClient.EXPECT().GetUsers("name:John,name:Jane", "").Return(expectedUsers, nil)

	users, err := mockClient.GetUsers("name:John,name:Jane", "")
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestGetUsers_Error(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetUsers("invalid", "").Return(nil, assert.AnError)

	users, err := mockClient.GetUsers("invalid", "")
	assert.Error(t, err)
	assert.Nil(t, users)
}

func TestGetDeletedUsers(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedUsers := []*admin.User{
		{
			PrimaryEmail: "deleted@example.com",
			Name: &admin.UserName{
				GivenName:  "Deleted",
				FamilyName: "User",
			},
		},
	}

	mockClient.EXPECT().GetDeletedUsers().Return(expectedUsers, nil)

	users, err := mockClient.GetDeletedUsers()
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "deleted@example.com", users[0].PrimaryEmail)
}

func TestGetDeletedUsers_Error(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetDeletedUsers().Return(nil, assert.AnError)

	users, err := mockClient.GetDeletedUsers()
	assert.Error(t, err)
	assert.Nil(t, users)
}

func TestGetGroups_EmptyQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetGroups("").Return([]*admin.Group{}, nil)

	groups, err := mockClient.GetGroups("")
	assert.NoError(t, err)
	assert.Len(t, groups, 0)
}

func TestGetGroups_WildcardQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedGroups := []*admin.Group{
		{
			Id:    "group1",
			Email: "group1@example.com",
			Name:  "Group 1",
		},
	}

	mockClient.EXPECT().GetGroups("*").Return(expectedGroups, nil)

	groups, err := mockClient.GetGroups("*")
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "group1@example.com", groups[0].Email)
}

func TestGetGroups_SpecificQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedGroups := []*admin.Group{
		{
			Id:    "group1",
			Email: "group1@example.com",
			Name:  "Group 1",
		},
	}

	mockClient.EXPECT().GetGroups("name:Group").Return(expectedGroups, nil)

	groups, err := mockClient.GetGroups("name:Group")
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
}

func TestGetGroups_MultipleQueries(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedGroups := []*admin.Group{
		{
			Id:    "group1",
			Email: "group1@example.com",
			Name:  "Group 1",
		},
		{
			Id:    "group2",
			Email: "group2@example.com",
			Name:  "Group 2",
		},
	}

	mockClient.EXPECT().GetGroups("name:Group1,name:Group2").Return(expectedGroups, nil)

	groups, err := mockClient.GetGroups("name:Group1,name:Group2")
	assert.NoError(t, err)
	assert.Len(t, groups, 2)
}

func TestGetGroups_Error(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetGroups("name:NonExistent").Return(nil, assert.AnError)

	groups, err := mockClient.GetGroups("name:NonExistent")
	assert.Error(t, err)
	assert.Nil(t, groups)
}

func TestGetGroupMembers(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	group := &admin.Group{
		Id:    "group1",
		Email: "group1@example.com",
		Name:  "Group 1",
	}

	expectedMembers := []*admin.Member{
		{
			Email: "member1@example.com",
			Type:  "USER",
		},
		{
			Email: "member2@example.com",
			Type:  "USER",
		},
	}

	mockClient.EXPECT().GetGroupMembers(group).Return(expectedMembers, nil)

	members, err := mockClient.GetGroupMembers(group)
	assert.NoError(t, err)
	assert.Len(t, members, 2)
	assert.Equal(t, "member1@example.com", members[0].Email)
	assert.Equal(t, "member2@example.com", members[1].Email)
}

func TestGetGroupMembers_NoMembers(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	group := &admin.Group{
		Id:    "group2",
		Email: "group2@example.com",
		Name:  "Group 2",
	}

	mockClient.EXPECT().GetGroupMembers(group).Return([]*admin.Member{}, nil)

	members, err := mockClient.GetGroupMembers(group)
	assert.NoError(t, err)
	assert.Empty(t, members)
}

func TestGetGroupMembers_Error(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	group := &admin.Group{
		Id:    "group3",
		Email: "group3@example.com",
		Name:  "Group 3",
	}

	mockClient.EXPECT().GetGroupMembers(group).Return(nil, assert.AnError)

	members, err := mockClient.GetGroupMembers(group)
	assert.Error(t, err)
	assert.Nil(t, members)
}

// #404: PrecacheOrgUnits query starts with a comma, likely caching all users

// TestGetUsers_Issue404_SingleOrgUnitQuery covers awslabs/ssosync#404.
// getGoogleGroupsAndUsers used to build the PrecacheOrgUnits query with a
// leading comma, which getUsers split into an empty first element and sent as a
// filter-only (whole directory) Users.list call. It now sends one single-term
// query per OU; this pins that getUsers turns such a query into exactly one
// Users.list call carrying that term plus the filter.
func TestGetUsers_Issue404_SingleOrgUnitQuery(t *testing.T) {
	var (
		mu      sync.Mutex
		queries []string
	)
	// Fake Directory API: record each Users.list query and return one user per
	// call, so the result shows how many calls were made.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/users") {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		queries = append(queries, r.URL.Query().Get("query"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users":[{"primaryEmail":"u@example.com","name":{"givenName":"U","familyName":"Ser"}}]}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	svc, err := admin.NewService(ctx,
		option.WithEndpoint(srv.URL+"/"),
		option.WithHTTPClient(srv.Client()),
		option.WithoutAuthentication())
	require.NoError(t, err)
	c := &client{ctx: ctx, service: svc, customerID: "my_customer"}

	// The exact string sync.go hands to GetUsers for the OU "/Sales Team".
	query := `OrgUnitPath='/Sales Team'`
	filter := " isSuspended=false isArchived=false"

	users, err := c.GetUsers(query, filter)
	require.NoError(t, err)

	assert.Equal(t, []string{query + filter}, queries, "exactly one Users.list call, with no filter-only call")
	assert.Len(t, users, 1)
}

// #414: Empty element in a comma-separated UserMatch/GroupMatch queries the whole directory

// issue414Client returns a client on a fake Directory API that records the
// query of every Users.list and Groups.list call, returning one user or group
// per call.
func issue414Client(t *testing.T) (c *client, userQueries, groupQueries func() []string) {
	var (
		mu            sync.Mutex
		users, groups []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/users"):
			users = append(users, r.URL.Query().Get("query"))
			_, _ = fmt.Fprint(w, `{"users":[{"primaryEmail":"u@example.com","name":{"givenName":"U","familyName":"Ser"}}]}`)
		case strings.HasSuffix(r.URL.Path, "/groups"):
			groups = append(groups, r.URL.Query().Get("query"))
			_, _ = fmt.Fprint(w, `{"groups":[{"email":"g@example.com"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	svc, err := admin.NewService(ctx,
		option.WithEndpoint(srv.URL+"/"),
		option.WithHTTPClient(srv.Client()),
		option.WithoutAuthentication())
	require.NoError(t, err)
	snapshot := func(s *[]string) func() []string {
		return func() []string {
			mu.Lock()
			defer mu.Unlock()
			return append([]string(nil), (*s)...)
		}
	}
	return &client{ctx: ctx, service: svc, customerID: "my_customer"}, snapshot(&users), snapshot(&groups)
}

func issue414Warnings(hook *test.Hook) []*log.Entry {
	var found []*log.Entry
	for _, e := range hook.AllEntries() {
		if e.Level == log.WarnLevel && e.Message == EmptyQueryElementMsg {
			found = append(found, e)
		}
	}
	return found
}

const issue414Filter = " isSuspended=false isArchived=false"

// An empty element (trailing, leading or doubled comma, or blank) is skipped
// with one warning naming the original query: no filter-only Users.list call.
func TestGetUsers_Issue414_EmptyElementsSkipped(t *testing.T) {
	tests := []struct {
		name, query string
		want        []string
	}{
		{"trailing comma", "email:a*,", []string{"email:a*" + issue414Filter}},
		{"leading comma", ",email:a*", []string{"email:a*" + issue414Filter}},
		{"doubled comma", "email:a*,,email:b*", []string{"email:a*" + issue414Filter, "email:b*" + issue414Filter}},
		{"blank element", "email:a*, ,email:b*", []string{"email:a*" + issue414Filter, "email:b*" + issue414Filter}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := test.NewGlobal()
			defer hook.Reset()
			c, userQueries, _ := issue414Client(t)

			users, err := c.GetUsers(tt.query, issue414Filter)
			require.NoError(t, err)
			assert.Equal(t, tt.want, userQueries())
			assert.Len(t, users, len(tt.want))

			warns := issue414Warnings(hook)
			require.Len(t, warns, 1)
			assert.Equal(t, tt.query, warns[0].Data["query"])
		})
	}
}

// issue414AllEmpty are queries made only of empty elements.
var issue414AllEmpty = []string{",", ",,", " ", " , "}

// A query made only of empty elements fails closed: an error naming the
// query, and no Users.list call, instead of the whole directory or nothing.
func TestGetUsers_Issue414_AllEmptyFails(t *testing.T) {
	for _, query := range issue414AllEmpty {
		t.Run(query, func(t *testing.T) {
			c, userQueries, _ := issue414Client(t)

			users, err := c.GetUsers(query, issue414Filter)
			require.ErrorIs(t, err, ErrAllEmptyQuery)
			assert.Contains(t, err.Error(), fmt.Sprintf("%q", query))
			assert.Nil(t, users)
			assert.Empty(t, userQueries(), "no Users.list call")
		})
	}
}

// The same for GroupMatch: an all-empty query is an error, not zero groups
// (which a groups sync would act on by deleting every unmatched AWS group).
func TestGetGroups_Issue414_AllEmptyFails(t *testing.T) {
	for _, query := range issue414AllEmpty {
		t.Run(query, func(t *testing.T) {
			c, _, groupQueries := issue414Client(t)

			groups, err := c.GetGroups(query)
			require.ErrorIs(t, err, ErrAllEmptyQuery)
			assert.Contains(t, err.Error(), fmt.Sprintf("%q", query))
			assert.Nil(t, groups)
			assert.Empty(t, groupQueries(), "no Groups.list call")
		})
	}
}

// "*" (all) and a query with no empty element are sent as before, unwarned;
// "" (none) makes no call.
func TestGetUsers_Issue414_UnchangedQueries(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()
	c, userQueries, _ := issue414Client(t)

	_, err := c.GetUsers("*", issue414Filter)
	require.NoError(t, err)
	_, err = c.GetUsers("email:a*,email:b*", issue414Filter)
	require.NoError(t, err)
	users, err := c.GetUsers("", issue414Filter)
	require.NoError(t, err)
	assert.Empty(t, users)

	assert.Equal(t, []string{issue414Filter, "email:a*" + issue414Filter, "email:b*" + issue414Filter}, userQueries())
	assert.Empty(t, issue414Warnings(hook))
}

// GroupMatch: an empty element is skipped with one warning, and "*" lists all
// groups with no query, unwarned.
func TestGetGroups_Issue414_EmptyElementsSkipped(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()
	c, _, groupQueries := issue414Client(t)

	groups, err := c.GetGroups("email:aws-*,,name:Admin*,")
	require.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, []string{"email:aws-*", "name:Admin*"}, groupQueries())
	warns := issue414Warnings(hook)
	require.Len(t, warns, 1)
	assert.Equal(t, "email:aws-*,,name:Admin*,", warns[0].Data["query"])

	groups, err = c.GetGroups("*")
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, []string{"email:aws-*", "name:Admin*", ""}, groupQueries(), `"*" lists all groups, with no query`)
	assert.Len(t, issue414Warnings(hook), 1, `"*" is not warned about`)
}

// #416: CacheMetrics: TotalUsers lookup for / lists the whole directory

// TestCountUsers_Issue416_CountsIdsAcrossPages covers awslabs/ssosync#416.
// CountUsers sums the users across every page, asking for ids only, 500 per
// page, and sends the query as one term (no comma split, unlike GetUsers).
func TestCountUsers_Issue416_CountsIdsAcrossPages(t *testing.T) {
	type request struct{ query, maxResults, fields, pageToken string }
	var (
		mu       sync.Mutex
		requests []request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/users") {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		mu.Lock()
		requests = append(requests, request{q.Get("query"), q.Get("maxResults"), q.Get("fields"), q.Get("pageToken")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if q.Get("pageToken") == "" {
			_, _ = fmt.Fprint(w, `{"users":[{"id":"1"},{"id":"2"}],"nextPageToken":"p2"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"users":[{"id":"3"}]}`)
	}))
	defer srv.Close()

	ctx := context.Background()
	svc, err := admin.NewService(ctx,
		option.WithEndpoint(srv.URL+"/"),
		option.WithHTTPClient(srv.Client()),
		option.WithoutAuthentication())
	require.NoError(t, err)
	c := &client{ctx: ctx, service: svc, customerID: "my_customer"}

	// An OU with a comma: one term, so one query per page.
	query := `OrgUnitPath='/Sales, EMEA'`
	filter := " isSuspended=false isArchived=false"
	count, err := c.CountUsers(query, filter)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	require.Len(t, requests, 2, "one Users.list call per page")
	assert.Equal(t, "p2", requests[1].pageToken)
	for _, r := range requests {
		assert.Equal(t, query+filter, r.query)
		assert.Equal(t, "500", r.maxResults)
		assert.Contains(t, r.fields, "nextPageToken")
		assert.Contains(t, r.fields, "users(id)")
	}
}

// An empty query counts nothing and makes no call, as getUsers returns nothing.
func TestCountUsers_Issue416_EmptyQuery(t *testing.T) {
	c := &client{ctx: context.Background()}
	count, err := c.CountUsers("", " isSuspended=false")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
