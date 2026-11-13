package google

import (
	"context"
	"fmt"
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
