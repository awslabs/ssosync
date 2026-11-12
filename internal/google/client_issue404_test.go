package google

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// TestGetUsers_Issue404_PrecacheLeadingCommaSendsEmptySubQuery reproduces
// awslabs/ssosync#404. getGoogleGroupsAndUsers builds the PrecacheOrgUnits query
// with a leading comma (pinned by
// TestGetGoogleGroupsAndUsers_Issue401_PrecacheOrgUnitQueriesQuoted); getUsers
// splits it on "," and sends one Users.list call per element, so the first call
// carries only the UserFilter and would match every user in the directory.
//
// This test asserts TODAY's (buggy) behaviour and passes now; the #404 fix
// flips it to expect exactly one Users.list call per configured OU.
func TestGetUsers_Issue404_PrecacheLeadingCommaSendsEmptySubQuery(t *testing.T) {
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

	// The exact string sync.go hands to GetUsers for
	// PrecacheOrgUnits = ["/Engineering", "/Sales Team"].
	precacheQuery := `,OrgUnitPath='/Engineering',OrgUnitPath='/Sales Team'`
	filter := " isSuspended=false isArchived=false"

	users, err := c.GetUsers(precacheQuery, filter)
	require.NoError(t, err)

	// Today: three Users.list calls for two OUs; the first has an empty OU part,
	// so its query is just the filter (the whole directory, narrowed only by
	// UserFilter).
	assert.Equal(t, []string{
		filter,
		`OrgUnitPath='/Engineering'` + filter,
		`OrgUnitPath='/Sales Team'` + filter,
	}, queries)
	assert.Len(t, users, 3, "the empty sub-query adds an extra page of users")
}
