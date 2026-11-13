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
