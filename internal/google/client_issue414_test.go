package google

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

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
