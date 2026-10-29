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

// --- Type-qualified selector (type=X) coverage ---

func TestMapFromGoogleUser_TypeSelectorMatch(t *testing.T) {
	u := &admin.User{
		Id:           "T1",
		PrimaryEmail: "t1@example.com",
		Name:         &admin.UserName{GivenName: "Type", FamilyName: "Match"},
		Organizations: []admin.UserOrganization{
			{Title: "Other Title", Type: "other", Primary: true},
			{Title: "Work Title", Department: "Eng", Type: "work", Primary: false},
		},
	}

	m := config.AttributeMap{
		"Username":              "primaryEmail",
		"Title":                 "organizations[type=work].title",
		"Enterprise.Department": "organizations[type=work].department",
	}

	got, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)
	assert.Equal(t, "Work Title", got.Title)
	if assert.NotNil(t, got.Enterprise) {
		assert.Equal(t, "Eng", got.Enterprise.Department)
	}
}

func TestMapFromGoogleUser_TypeSelectorNoMatchResolvesToNothing(t *testing.T) {
	u := &admin.User{
		Id:           "T2",
		PrimaryEmail: "t2@example.com",
		Name:         &admin.UserName{GivenName: "No", FamilyName: "Match"},
		Organizations: []admin.UserOrganization{
			{Title: "Other Title", Type: "other", Primary: true},
			{Title: "Home Title", Type: "home", Primary: false},
		},
	}

	m := config.AttributeMap{
		"Username":              "primaryEmail",
		"Title":                 "organizations[type=work].title",
		"Enterprise.Department": "organizations[type=work].department",
	}

	got, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)
	// No work-typed org -> NOTHING, no first-entry fallback.
	assert.Equal(t, "", got.Title)
	assert.Nil(t, got.Enterprise)
}

func TestMapFromGoogleUser_TypeSelectorCaseInsensitive(t *testing.T) {
	// Selector "type=work" matches a stored Type of "Work". The emitted SCIM
	// Type for a multi-valued target carries the Google entry's own (stored)
	// value, so we pin it explicitly.
	u := &admin.User{
		Id:           "T3",
		PrimaryEmail: "t3@example.com",
		Name:         &admin.UserName{GivenName: "Case", FamilyName: "Insens"},
		Emails: []admin.UserEmail{
			{Address: "work@example.com", Type: "Work", Primary: false},
		},
	}

	m := config.AttributeMap{
		"Username": "primaryEmail",
		"Emails":   "emails[type=work].address",
	}

	got, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)
	if assert.Len(t, got.Emails, 1) {
		assert.Equal(t, "work@example.com", got.Emails[0].Value)
		// The emitted SCIM Type is the stored Google value "Work", not the
		// selector's lower-cased "work".
		assert.Equal(t, "Work", got.Emails[0].Type)
		assert.True(t, got.Emails[0].Primary)
	}
}

// --- relations[type=manager] regression (design §13) ---

func TestMapFromGoogleUser_RelationsManagerRegression(t *testing.T) {
	optIn := config.AttributeMap{
		"Username":           "primaryEmail",
		"Enterprise.Manager": "relations[type=manager].value",
	}

	t.Run("single matching relation", func(t *testing.T) {
		u := &admin.User{
			Id: "R1", PrimaryEmail: "r1@example.com",
			Name:      &admin.UserName{GivenName: "R", FamilyName: "One"},
			Relations: []admin.UserRelation{{Type: "manager", Value: "boss@example.com"}},
		}
		got, err := mapFromGoogleUser(u, optIn)
		assert.NoError(t, err)
		if assert.NotNil(t, got.Enterprise) && assert.NotNil(t, got.Enterprise.Manager) {
			assert.Equal(t, "boss@example.com", got.Enterprise.Manager.Value)
		}
	})

	t.Run("cased Manager/MANAGER matches", func(t *testing.T) {
		for _, cased := range []string{"Manager", "MANAGER"} {
			u := &admin.User{
				Id: "R2", PrimaryEmail: "r2@example.com",
				Name:      &admin.UserName{GivenName: "R", FamilyName: "Two"},
				Relations: []admin.UserRelation{{Type: cased, Value: "boss@example.com"}},
			}
			got, err := mapFromGoogleUser(u, optIn)
			assert.NoError(t, err, cased)
			if assert.NotNil(t, got.Enterprise, cased) && assert.NotNil(t, got.Enterprise.Manager, cased) {
				assert.Equal(t, "boss@example.com", got.Enterprise.Manager.Value, cased)
			}
		}
	})

	t.Run("no manager relation -> empty", func(t *testing.T) {
		u := &admin.User{
			Id: "R3", PrimaryEmail: "r3@example.com",
			Name:      &admin.UserName{GivenName: "R", FamilyName: "Three"},
			Relations: []admin.UserRelation{{Type: "assistant", Value: "asst@example.com"}},
		}
		got, err := mapFromGoogleUser(u, optIn)
		assert.NoError(t, err)
		assert.Nil(t, got.Enterprise)
	})

	t.Run("non-manager relation present -> empty", func(t *testing.T) {
		u := &admin.User{
			Id: "R4", PrimaryEmail: "r4@example.com",
			Name: &admin.UserName{GivenName: "R", FamilyName: "Four"},
			Relations: []admin.UserRelation{
				{Type: "assistant", Value: "asst@example.com"},
				{Type: "dotted-line", Value: "dotted@example.com"},
			},
		}
		got, err := mapFromGoogleUser(u, optIn)
		assert.NoError(t, err)
		assert.Nil(t, got.Enterprise)
	})
}

// --- Custom-schema source resolution (design §2.4) ---

func cs(m map[string]string) map[string]googleapi.RawMessage {
	out := make(map[string]googleapi.RawMessage, len(m))
	for k, v := range m {
		out[k] = googleapi.RawMessage(v)
	}
	return out
}

func TestMapFromGoogleUser_CustomScalarPresent(t *testing.T) {
	u := &admin.User{
		Id:           "C1",
		PrimaryEmail: "c1@example.com",
		Name:         &admin.UserName{GivenName: "C", FamilyName: "One"},
		CustomSchemas: cs(map[string]string{
			"EmploymentData": `{"costCenter":"CC-900","level":7,"remote":true}`,
		}),
	}

	m := config.AttributeMap{
		"Username":              "primaryEmail",
		"Enterprise.CostCenter": "customSchemas.EmploymentData.costCenter",
	}

	got, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)
	if assert.NotNil(t, got.Enterprise) {
		assert.Equal(t, "CC-900", got.Enterprise.CostCenter)
	}
}

func TestMapFromGoogleUser_CustomNumberAndBoolStringified(t *testing.T) {
	u := &admin.User{
		Id:           "C1b",
		PrimaryEmail: "c1b@example.com",
		Name:         &admin.UserName{GivenName: "C", FamilyName: "OneB"},
		CustomSchemas: cs(map[string]string{
			"EmploymentData": `{"level":7,"remote":true}`,
		}),
	}

	gotNum, err := mapFromGoogleUser(u, config.AttributeMap{
		"Username": "primaryEmail",
		"Title":    "customSchemas.EmploymentData.level",
	})
	assert.NoError(t, err)
	assert.Equal(t, "7", gotNum.Title)

	gotBool, err := mapFromGoogleUser(u, config.AttributeMap{
		"Username": "primaryEmail",
		"Title":    "customSchemas.EmploymentData.remote",
	})
	assert.NoError(t, err)
	assert.Equal(t, "true", gotBool.Title)
}

func TestMapFromGoogleUser_CustomSchemaAbsentWarnsAndClears(t *testing.T) {
	u := &admin.User{
		Id:           "C2",
		PrimaryEmail: "c2@example.com",
		Name:         &admin.UserName{GivenName: "C", FamilyName: "Two"},
		// No CustomSchemas at all.
	}

	m := config.AttributeMap{
		"Username":              "primaryEmail",
		"Enterprise.CostCenter": "customSchemas.EmploymentData.costCenter",
	}

	got, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)
	// Schema absent -> resolves to nothing, Enterprise stays nil.
	assert.Nil(t, got.Enterprise)
}

func TestMapFromGoogleUser_CustomFieldAbsentWarnsAndClears(t *testing.T) {
	u := &admin.User{
		Id:           "C3",
		PrimaryEmail: "c3@example.com",
		Name:         &admin.UserName{GivenName: "C", FamilyName: "Three"},
		CustomSchemas: cs(map[string]string{
			"EmploymentData": `{"someOtherField":"x"}`,
		}),
	}

	m := config.AttributeMap{
		"Username":              "primaryEmail",
		"Enterprise.CostCenter": "customSchemas.EmploymentData.costCenter",
	}

	got, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)
	// Field absent within present schema -> nothing.
	assert.Nil(t, got.Enterprise)
}

// multiValuedWarnings returns the multi-valued custom-field WARNs logged.
func multiValuedWarnings(hook *test.Hook) []*log.Entry {
	var out []*log.Entry
	for _, e := range hook.AllEntries() {
		if e.Message == MultiValuedCustomFieldIgnoredMsg {
			out = append(out, e)
		}
	}
	return out
}

// Multi-valued custom fields are not supported: whatever shape the values take
// (bare values or {value} objects, as Google returns them), the mapping is
// ignored with a WARN and the target resolves to nothing.
func TestMapFromGoogleUser_CustomMultiValuedWarnedAndIgnored(t *testing.T) {
	for name, field := range map[string]string{
		"value objects": `[{"value":"PRJ-1"},{"value":"PRJ-2"}]`,
		"bare values":   `["PRJ-1","PRJ-2"]`,
		"single entry":  `[{"value":"PRJ-1"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			hook := test.NewGlobal()
			defer hook.Reset()

			u := &admin.User{
				Id:            "C4",
				PrimaryEmail:  "c4@example.com",
				Name:          &admin.UserName{GivenName: "C", FamilyName: "Four"},
				CustomSchemas: cs(map[string]string{"Projects": `{"codes":` + field + `}`}),
			}
			got, err := mapFromGoogleUser(u, config.AttributeMap{
				"Username":              "primaryEmail",
				"Enterprise.CostCenter": "customSchemas.Projects.codes",
			})
			assert.NoError(t, err)
			assert.Nil(t, got.Enterprise, "a multi-valued custom field must not be mapped")

			warns := multiValuedWarnings(hook)
			if assert.Len(t, warns, 1) {
				assert.Equal(t, log.WarnLevel, warns[0].Level)
				assert.Equal(t, "customFieldMultiValued", warns[0].Data["reason"])
				assert.Equal(t, "Projects", warns[0].Data["schema"])
				assert.Equal(t, "codes", warns[0].Data["field"])
				assert.Equal(t, "Enterprise.CostCenter", warns[0].Data["target"])
				assert.Equal(t, "c4@example.com", warns[0].Data["user"])
			}
		})
	}
}

// Single-valued custom fields, absent fields and empty values never log the
// multi-valued warning.
func TestMapFromGoogleUser_CustomSingleValuedNoMultiValuedWarning(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	u := &admin.User{
		Id:            "C5",
		PrimaryEmail:  "c5@example.com",
		Name:          &admin.UserName{GivenName: "C", FamilyName: "Five"},
		CustomSchemas: cs(map[string]string{"S": `{"cc":"CC-1","empty":"","nothing":null}`}),
	}
	got, err := mapFromGoogleUser(u, config.AttributeMap{
		"Username":              "primaryEmail",
		"Enterprise.CostCenter": "customSchemas.S.cc",
		"Title":                 "customSchemas.S.empty",
		"Enterprise.Division":   "customSchemas.S.nothing",
		"ProfileURL":            "customSchemas.S.missing",
	})
	assert.NoError(t, err)
	if assert.NotNil(t, got.Enterprise) {
		assert.Equal(t, "CC-1", got.Enterprise.CostCenter)
	}
	assert.Empty(t, multiValuedWarnings(hook))
}

// The debug trace classifies a multi-valued custom field distinctly.
func TestDebugEmptyReason_CustomMultiValued(t *testing.T) {
	u := &admin.User{CustomSchemas: cs(map[string]string{"P": `{"codes":["a","b"],"one":"x"}`})}
	assert.Equal(t, "customFieldMultiValued", debugEmptyReason(u, "customSchemas.P.codes"))
	assert.Equal(t, "emptySourceValue", debugEmptyReason(u, "customSchemas.P.one"))
}

func TestMapFromGoogleUser_CustomSelectorlessArrayOrObjectResolvesToNothing(t *testing.T) {
	u := &admin.User{
		Id:           "C6",
		PrimaryEmail: "c6@example.com",
		Name:         &admin.UserName{GivenName: "C", FamilyName: "Six"},
		CustomSchemas: cs(map[string]string{
			"Contact": `{"ids":[{"type":"github","value":"octocat"}],"obj":{"a":"b"}}`,
		}),
	}

	// Selector-less array.
	gotArr, err := mapFromGoogleUser(u, config.AttributeMap{
		"Username": "primaryEmail",
		"Title":    "customSchemas.Contact.ids",
	})
	assert.NoError(t, err)
	assert.Equal(t, "", gotArr.Title)

	// Selector-less object.
	gotObj, err := mapFromGoogleUser(u, config.AttributeMap{
		"Username": "primaryEmail",
		"Title":    "customSchemas.Contact.obj",
	})
	assert.NoError(t, err)
	assert.Equal(t, "", gotObj.Title)
}

func TestMapFromGoogleUser_CustomNullAndScalarWithSelectorResolveToEmpty(t *testing.T) {
	u := &admin.User{
		Id:           "C7",
		PrimaryEmail: "c7@example.com",
		Name:         &admin.UserName{GivenName: "C", FamilyName: "Seven"},
		CustomSchemas: cs(map[string]string{
			"S": `{"nothing":null,"scalar":"plain"}`,
		}),
	}

	gotNull, err := mapFromGoogleUser(u, config.AttributeMap{
		"Username": "primaryEmail",
		"Title":    "customSchemas.S.nothing",
	})
	assert.NoError(t, err)
	assert.Equal(t, "", gotNull.Title)

	// A scalar field with a [type=...] selector cannot apply -> empty.
	gotScalarSel, err := mapFromGoogleUser(u, config.AttributeMap{
		"Username": "primaryEmail",
		"Title":    "customSchemas.S.scalar[type=foo]",
	})
	assert.NoError(t, err)
	assert.Equal(t, "", gotScalarSel.Title)
}

// --- Present-but-empty standard and custom values: no warn, resolve empty (§6.3) ---

func TestMapFromGoogleUser_PresentButEmptyValuesResolveEmpty(t *testing.T) {
	// A type=work org exists but its Title is empty -> Title resolves to "" with
	// no absence. A present custom field decoding to "" is likewise empty.
	u := &admin.User{
		Id:           "E1",
		PrimaryEmail: "e1@example.com",
		Name:         &admin.UserName{GivenName: "E", FamilyName: "One"},
		Organizations: []admin.UserOrganization{
			{Title: "", Department: "Eng", Type: "work", Primary: true},
		},
		CustomSchemas: cs(map[string]string{
			"S": `{"empty":""}`,
		}),
	}

	m := config.AttributeMap{
		"Username":              "primaryEmail",
		"Title":                 "organizations[type=work].title",
		"Enterprise.Department": "organizations[type=work].department",
		"Enterprise.CostCenter": "customSchemas.S.empty",
	}

	got, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)
	assert.Equal(t, "", got.Title)
	// Department populated from the same present work org; custom empty clears
	// nothing extra.
	if assert.NotNil(t, got.Enterprise) {
		assert.Equal(t, "Eng", got.Enterprise.Department)
		assert.Equal(t, "", got.Enterprise.CostCenter)
	}
}

// --- Order-independence / churn-freedom for type=X and custom type=X ---

func reverseRawEntries(in []admin.UserOrganization) []admin.UserOrganization {
	return reverseOrgs(in)
}

func TestMapFromGoogleUser_ShuffledOrderTypeSelectorStable(t *testing.T) {
	orgs := []admin.UserOrganization{
		{Title: "Other", Department: "DO", Type: "other", Primary: true},
		{Title: "Work Title", Department: "DW", Name: "WorkCorp", Type: "work", Primary: false},
		{Title: "Home", Department: "DH", Type: "home", Primary: false},
	}

	m := config.AttributeMap{
		"Name.GivenName":          "name.givenName",
		"Name.FamilyName":         "name.familyName",
		"Username":                "primaryEmail",
		"Title":                   "organizations[type=work].title",
		"Enterprise.Department":   "organizations[type=work].department",
		"Enterprise.Organization": "organizations[type=work].name",
	}

	build := func(o []admin.UserOrganization) *admin.User {
		return &admin.User{
			Id:            "S1",
			PrimaryEmail:  "s1@example.com",
			Name:          &admin.UserName{GivenName: "Shuf", FamilyName: "Type"},
			Organizations: o,
		}
	}

	base, err := mapFromGoogleUser(build(orgs), m)
	assert.NoError(t, err)
	reversed, err := mapFromGoogleUser(build(reverseRawEntries(orgs)), m)
	assert.NoError(t, err)
	rotated, err := mapFromGoogleUser(build(rotateOrgs(orgs)), m)
	assert.NoError(t, err)

	assert.Equal(t, base, reversed)
	assert.Equal(t, base, rotated)
	assert.Equal(t, "Work Title", base.Title)
}

func TestMapFromGoogleUser_ShuffledOrderCustomMultiValuedStillIgnored(t *testing.T) {
	// Multi-valued custom arrays in any order are ignored the same way (no
	// order-dependent selection).
	variants := []string{
		`{"ids":[{"value":"octocat"},{"value":"glcat"},{"value":"bbcat"}]}`,
		`{"ids":[{"value":"bbcat"},{"value":"glcat"},{"value":"octocat"}]}`,
	}
	for i, v := range variants {
		u := &admin.User{
			Id:            "S2",
			PrimaryEmail:  "s2@example.com",
			Name:          &admin.UserName{GivenName: "Shuf", FamilyName: "Custom"},
			CustomSchemas: cs(map[string]string{"Contact": v}),
		}
		got, err := mapFromGoogleUser(u, config.AttributeMap{
			"Username": "primaryEmail",
			"Title":    "customSchemas.Contact.ids",
		})
		assert.NoError(t, err, i)
		assert.Equal(t, "", got.Title, i)
	}
}

// --- Newly-opened standard sub-field (enrichment path): organizations[type=work].domain ---

func TestMapFromGoogleUser_OptInDivisionFromWorkDomain(t *testing.T) {
	u := &admin.User{
		Id:           "D1",
		PrimaryEmail: "d1@example.com",
		Name:         &admin.UserName{GivenName: "Div", FamilyName: "User"},
		Organizations: []admin.UserOrganization{
			{Domain: "eng.example.com", Type: "work", Primary: true},
		},
	}

	m := config.AttributeMap{
		"Username":            "primaryEmail",
		"Enterprise.Division": "organizations[type=work].domain",
	}

	got, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)
	if assert.NotNil(t, got.Enterprise) {
		assert.Equal(t, "eng.example.com", got.Enterprise.Division)
	}
}

func TestMapFromGoogleUser_OptInEmailsTypeSelector(t *testing.T) {
	u := &admin.User{
		Id:           "D2",
		PrimaryEmail: "d2@example.com",
		Name:         &admin.UserName{GivenName: "Em", FamilyName: "User"},
		Emails: []admin.UserEmail{
			{Address: "home@example.com", Type: "home", Primary: true},
			{Address: "office@example.com", Type: "work", Primary: false},
		},
	}

	m := config.AttributeMap{
		"Username": "primaryEmail",
		"Emails":   "emails[type=work].address",
	}

	got, err := mapFromGoogleUser(u, m)
	assert.NoError(t, err)
	if assert.Len(t, got.Emails, 1) {
		// type=work selected the non-primary office email (no primary fallback
		// needed because the type matched).
		assert.Equal(t, "office@example.com", got.Emails[0].Value)
		assert.Equal(t, "work", got.Emails[0].Type)
	}
}
