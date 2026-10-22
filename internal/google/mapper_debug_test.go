package google

import (
	"testing"

	"ssosync/internal/config"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
)

// TestMapFromGoogleUser_DebugTrace asserts that at debug level the mapper emits
// the googleUser + attributeMap dump and a per-entry resolution trace with the
// expected reasons; and that at info level NO mapping-trace entries are emitted
// (design §8.3).
func TestMapFromGoogleUser_DebugTrace(t *testing.T) {
	u := &admin.User{
		Id:           "DBG1",
		PrimaryEmail: "dbg@example.com",
		Name:         &admin.UserName{GivenName: "Deb", FamilyName: "Ug"},
		Organizations: []admin.UserOrganization{
			{Title: "Work Title", Type: "work", Primary: true},
			{Title: "Home Title", Type: "home", Primary: false},
		},
		CustomSchemas: map[string]googleapi.RawMessage{
			"S": googleapi.RawMessage(`{"present":"x"}`),
		},
	}

	m := config.AttributeMap{
		// resolves to a value
		"Title": "organizations[type=work].title",
		// type=manager miss -> noTypeMatch:manager
		"Enterprise.Manager": "relations[type=manager].value",
		// custom field absent -> customFieldAbsent
		"Enterprise.CostCenter": "customSchemas.S.missing",
	}

	hook := test.NewGlobal()
	defer hook.Reset()
	prev := log.GetLevel()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(prev)

	_, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)

	var sawDump, sawResolvedValue, sawNoTypeMatch, sawCustomFieldAbsent bool
	for _, e := range hook.AllEntries() {
		if e.Message == "attribute map: mapping Google user" {
			_, hasUser := e.Data["googleUser"]
			_, hasMap := e.Data["attributeMap"]
			if hasUser && hasMap {
				sawDump = true
			}
		}
		if e.Message == "attribute map: entry resolved" {
			switch e.Data["reason"] {
			case "noTypeMatch:manager":
				sawNoTypeMatch = true
			case "customFieldAbsent":
				sawCustomFieldAbsent = true
			case nil:
				if e.Data["resolved"] == "Work Title" {
					sawResolvedValue = true
				}
			}
		}
	}

	assert.True(t, sawDump, "expected googleUser+attributeMap dump at debug")
	assert.True(t, sawResolvedValue, "expected a resolved-value trace line")
	assert.True(t, sawNoTypeMatch, "expected noTypeMatch:manager trace reason")
	assert.True(t, sawCustomFieldAbsent, "expected customFieldAbsent trace reason")
}

func TestMapFromGoogleUser_NoDebugTraceAtInfo(t *testing.T) {
	u := &admin.User{
		Id:           "DBG2",
		PrimaryEmail: "dbg2@example.com",
		Name:         &admin.UserName{GivenName: "No", FamilyName: "Trace"},
		Organizations: []admin.UserOrganization{
			{Title: "Work Title", Type: "work", Primary: true},
		},
	}

	m := config.AttributeMap{"Title": "organizations[type=work].title"}

	hook := test.NewGlobal()
	defer hook.Reset()
	prev := log.GetLevel()
	log.SetLevel(log.InfoLevel)
	defer log.SetLevel(prev)

	_, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)

	for _, e := range hook.AllEntries() {
		assert.NotEqual(t, "attribute map: mapping Google user", e.Message)
		assert.NotEqual(t, "attribute map: entry resolved", e.Message)
	}
}

// TestMapFromGoogleUser_CustomAbsentWarnDriftGuard pins that the resolver's
// found=false classification and the mapper's WARN reason stay in sync for both
// schema-absent and field-absent cases (review NIT b / design §6.1a).
func TestMapFromGoogleUser_CustomAbsentWarnDriftGuard(t *testing.T) {
	cases := []struct {
		name       string
		user       *admin.User
		source     string
		wantReason string
		wantFound  bool
	}{
		{
			name: "schema absent",
			user: &admin.User{Id: "W1", PrimaryEmail: "w1@example.com",
				Name: &admin.UserName{GivenName: "W", FamilyName: "One"}},
			source:     "customSchemas.Missing.field",
			wantReason: "customSchemaAbsent",
			wantFound:  false,
		},
		{
			name: "field absent in present schema",
			user: &admin.User{Id: "W2", PrimaryEmail: "w2@example.com",
				Name: &admin.UserName{GivenName: "W", FamilyName: "Two"},
				CustomSchemas: map[string]googleapi.RawMessage{
					"S": googleapi.RawMessage(`{"other":"x"}`),
				}},
			source:     "customSchemas.S.field",
			wantReason: "customFieldAbsent",
			wantFound:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Resolver classification.
			_, found, err := resolveGoogleUserSource(tc.user, tc.source)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantFound, found)

			// Mapper WARN reason.
			hook := test.NewGlobal()
			defer hook.Reset()
			prev := log.GetLevel()
			log.SetLevel(log.WarnLevel)
			defer log.SetLevel(prev)

			_, err = mapFromGoogleUser(tc.user, config.AttributeMap{
				"Username":              "primaryEmail",
				"Enterprise.CostCenter": tc.source,
			})
			assert.NoError(t, err)

			var reason string
			for _, e := range hook.AllEntries() {
				if e.Message == "attribute map: custom-schema source not found on user; field cleared" {
					if r, ok := e.Data["reason"].(string); ok {
						reason = r
					}
				}
			}
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}
