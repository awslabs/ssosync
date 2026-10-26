package config_test

import (
	"os"
	"path/filepath"
	"testing"

	. "ssosync/internal/config"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestLoadAttributeMapsFromFile_ValidFull(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "attrmap.json")
	content := `{
		"user": {
			"Name.GivenName": "name.givenName",
			"Username": "primaryEmail"
		},
		"group": {
			"DisplayName": "name"
		}
	}`
	assert.NoError(os.WriteFile(path, []byte(content), 0o600))

	am, err := LoadAttributeMapsFromFile(path)
	assert.NoError(err)
	assert.Equal(AttributeMap{
		"Name.GivenName": "name.givenName",
		"Username":       "primaryEmail",
	}, am.User)
	assert.Equal(AttributeMap{
		"DisplayName": "name",
	}, am.Group)
}

func TestLoadAttributeMapsFromFile_ValidPartial(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "attrmap.json")
	// Only the group sub-object is provided; user must fall back to default.
	content := `{"group": {"DisplayName": "name"}}`
	assert.NoError(os.WriteFile(path, []byte(content), 0o600))

	am, err := LoadAttributeMapsFromFile(path)
	assert.NoError(err)
	assert.Equal(DefaultUserAttributeMap, am.User)
	assert.Equal(AttributeMap{"DisplayName": "name"}, am.Group)
}

func TestLoadAttributeMapsFromFile_EmptyPathFallsBackToDefault(t *testing.T) {
	assert := assert.New(t)

	am, err := LoadAttributeMapsFromFile("")
	assert.NoError(err)
	assert.Equal(DefaultAttributeMap, am)
}

func TestLoadAttributeMapsFromFile_MissingFile(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	_, err := LoadAttributeMapsFromFile(path)
	assert.Error(err)
	assert.Contains(err.Error(), "cannot read attribute map file")
}

func TestLoadAttributeMapsFromString_Empty(t *testing.T) {
	assert := assert.New(t)

	am, err := LoadAttributeMapsFromString("")
	assert.NoError(err)
	assert.Equal(DefaultAttributeMap, am)

	am, err = LoadAttributeMapsFromString("   \n\t")
	assert.NoError(err)
	assert.Equal(DefaultAttributeMap, am)
}

func TestLoadAttributeMapsFromString_ValidJSON(t *testing.T) {
	assert := assert.New(t)

	am, err := LoadAttributeMapsFromString(`{"user":{"Username":"primaryEmail"}}`)
	assert.NoError(err)
	assert.Equal(AttributeMap{"Username": "primaryEmail"}, am.User)
	// group omitted -> default
	assert.Equal(DefaultGroupAttributeMap, am.Group)
}

func TestLoadAttributeMapsFromString_Malformed(t *testing.T) {
	assert := assert.New(t)

	_, err := LoadAttributeMapsFromString(`{not valid json`)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid attribute map JSON")
}

func TestParseAttributeMaps_EmptyValueFailsValidation(t *testing.T) {
	assert := assert.New(t)

	_, err := ParseAttributeMaps([]byte(`{"user":{"Username":""}}`))
	assert.Error(err)
	assert.Contains(err.Error(), "empty source attribute")
	assert.Contains(err.Error(), "user block")
}

func TestParseAttributeMaps_DefaultMapsPassValidation(t *testing.T) {
	assert := assert.New(t)

	am, err := ParseAttributeMaps([]byte(`{}`))
	assert.NoError(err)
	assert.Equal(DefaultUserAttributeMap, am.User)
	assert.Equal(DefaultGroupAttributeMap, am.Group)
}

func TestParseAttributeMaps_UnsupportedTargetField(t *testing.T) {
	assert := assert.New(t)

	// Wrong case on a known field: "Name.Givenname" is not a supported target.
	_, err := ParseAttributeMaps([]byte(`{"user":{"Name.Givenname":"name.givenName"}}`))
	assert.Error(err)
	msg := err.Error()
	assert.Contains(msg, "attribute map (user block)")
	assert.Contains(msg, `unsupported target field "Name.Givenname"`)
	// The valid-set listing names the supported user target fields, sorted.
	assert.Contains(msg, "Active, Addresses, Emails, Enterprise.CostCenter, "+
		"Enterprise.Department, Enterprise.Division, Enterprise.EmployeeNumber, "+
		"Enterprise.Manager, Enterprise.Organization, Name.FamilyName, "+
		"Name.GivenName, PhoneNumbers, ProfileURL, Title, Username")
}

func TestParseAttributeMaps_SafeSetTokensAccepted(t *testing.T) {
	assert := assert.New(t)

	content := `{"user":{
		"Title":"organizations[primary].title",
		"Enterprise.Department":"organizations[primary].department",
		"Enterprise.Organization":"organizations[primary].name",
		"Enterprise.CostCenter":"organizations[primary].costCenter"
	}}`
	am, err := ParseAttributeMaps([]byte(content))
	assert.NoError(err)
	assert.Equal("organizations[primary].title", am.User["Title"])
	assert.Equal("organizations[primary].department", am.User["Enterprise.Department"])
	assert.Equal("organizations[primary].name", am.User["Enterprise.Organization"])
	assert.Equal("organizations[primary].costCenter", am.User["Enterprise.CostCenter"])
}

func TestParseAttributeMaps_OptInTokensAccepted(t *testing.T) {
	assert := assert.New(t)

	content := `{"user":{
		"Enterprise.Manager":"relations[type=manager].value",
		"Enterprise.Division":"organizations[primary].domain",
		"Enterprise.EmployeeNumber":"organizations[primary].costCenter",
		"ProfileURL":"websites[primary].value",
		"Emails":"emails[primary].address",
		"Addresses":"addresses[primary].formatted",
		"PhoneNumbers":"phones[primary].value"
	}}`
	am, err := ParseAttributeMaps([]byte(content))
	assert.NoError(err)
	assert.Equal("relations[type=manager].value", am.User["Enterprise.Manager"])
	assert.Equal("websites[primary].value", am.User["ProfileURL"])
	assert.Equal("emails[primary].address", am.User["Emails"])
	assert.Equal("addresses[primary].formatted", am.User["Addresses"])
	assert.Equal("phones[primary].value", am.User["PhoneNumbers"])
}

func TestParseAttributeMaps_DeferredAddressSubFieldsRejected(t *testing.T) {
	assert := assert.New(t)

	// Only addresses[primary].formatted is registered this phase; the per-sub-field
	// address paths are deliberately NOT supported yet (design §7.3).
	_, err := ParseAttributeMaps([]byte(`{"user":{"Addresses":"addresses[primary].streetAddress"}}`))
	assert.Error(err)
	assert.Contains(err.Error(), `unsupported source attribute "addresses[primary].streetAddress"`)

	_, err = ParseAttributeMaps([]byte(`{"user":{"Addresses":"addresses[primary].locality"}}`))
	assert.Error(err)
	assert.Contains(err.Error(), `unsupported source attribute "addresses[primary].locality"`)

	// The supported formatted path, by contrast, is accepted.
	_, err = ParseAttributeMaps([]byte(`{"user":{"Addresses":"addresses[primary].formatted"}}`))
	assert.NoError(err)
}

func TestParseAttributeMaps_TypeQualifiedSourcesAccepted(t *testing.T) {
	assert := assert.New(t)

	content := `{"user":{
		"Title":"organizations[type=work].title",
		"Enterprise.Department":"organizations[type=work].department",
		"Enterprise.Manager":"relations[type=manager].value",
		"Emails":"emails[type=home].address",
		"PhoneNumbers":"phones[type=mobile].value",
		"Addresses":"addresses[type=work].formatted",
		"ProfileURL":"websites[type=work].value"
	}}`
	am, err := ParseAttributeMaps([]byte(content))
	assert.NoError(err)
	assert.Equal("organizations[type=work].title", am.User["Title"])
	assert.Equal("emails[type=home].address", am.User["Emails"])
}

func TestParseAttributeMaps_CustomSourcesAccepted(t *testing.T) {
	assert := assert.New(t)

	content := `{"user":{
		"Enterprise.CostCenter":"customSchemas.EmploymentData.costCenter",
		"Title":"customSchemas.Single_Sign-On.providerId"
	}}`
	am, err := ParseAttributeMaps([]byte(content))
	assert.NoError(err)
	assert.Equal("customSchemas.EmploymentData.costCenter", am.User["Enterprise.CostCenter"])
	assert.Equal("customSchemas.Single_Sign-On.providerId", am.User["Title"])
}

// A [type=...] selector on a custom source asks for an element of a
// multi-valued custom field, which is not supported: the entry is dropped with
// a WARN and the rest of the map still loads.
func TestParseAttributeMaps_MultiValuedCustomSourceWarnedAndIgnored(t *testing.T) {
	assert := assert.New(t)
	hook := test.NewGlobal()
	defer hook.Reset()

	content := `{"user":{
		"Username":"primaryEmail",
		"Enterprise.CostCenter":"customSchemas.EmploymentData.costCenter",
		"Enterprise.Division":"customSchemas.Contact.ids[type=gitlab]"
	}}`
	am, err := ParseAttributeMaps([]byte(content))
	assert.NoError(err)

	_, kept := am.User["Enterprise.Division"]
	assert.False(kept, "the multi-valued custom mapping must be ignored")
	assert.Equal("customSchemas.EmploymentData.costCenter", am.User["Enterprise.CostCenter"])
	assert.Equal("primaryEmail", am.User["Username"])

	var warned int
	for _, e := range hook.AllEntries() {
		if e.Message == MultiValuedCustomSourceIgnoredMsg {
			warned++
			assert.Equal(log.WarnLevel, e.Level)
			assert.Equal("Enterprise.Division", e.Data["target"])
			assert.Equal("customSchemas.Contact.ids[type=gitlab]", e.Data["source"])
		}
	}
	assert.Equal(1, warned)
}

// Dropping an entry must never modify the shared default map.
func TestParseAttributeMaps_MultiValuedCustomSourceDoesNotMutateDefaults(t *testing.T) {
	assert := assert.New(t)
	before := len(DefaultAttributeMap.User)

	_, err := ParseAttributeMaps([]byte(`{"group":{"DisplayName":"name"}}`))
	assert.NoError(err)
	am, err := ParseAttributeMaps([]byte(`{"user":{"Username":"primaryEmail","Title":"customSchemas.X.y[type=a]"}}`))
	assert.NoError(err)

	assert.Len(am.User, 1)
	assert.Len(DefaultAttributeMap.User, before)
}

// A map with no multi-valued custom source logs no such warning.
func TestParseAttributeMaps_NoMultiValuedCustomSourceNoWarning(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	_, err := ParseAttributeMaps([]byte(`{"user":{"Username":"primaryEmail","Title":"customSchemas.X.y"}}`))
	assert.NoError(t, err)
	for _, e := range hook.AllEntries() {
		assert.NotEqual(t, MultiValuedCustomSourceIgnoredMsg, e.Message)
	}
}

func TestParseAttributeMaps_CustomSourceToUnsupportedTargetRejected(t *testing.T) {
	assert := assert.New(t)

	// Well-formed custom source, but routed to an unsupported target KEY: it
	// must fail on the TARGET (closed target vocabulary, design §5.1).
	_, err := ParseAttributeMaps([]byte(`{"user":{"CustomThing":"customSchemas.X.y"}}`))
	assert.Error(err)
	assert.Contains(err.Error(), `unsupported target field "CustomThing"`)
}

func TestParseAttributeMaps_MalformedCustomSourceRejected(t *testing.T) {
	assert := assert.New(t)

	for _, src := range []string{
		"customSchemas.EmploymentData", // no field segment
		"customSchemas.",               // no remainder
		"customSchemas..field",         // empty schema
		"customSchemas.Schema.",        // empty field
	} {
		_, err := ParseAttributeMaps([]byte(`{"user":{"Title":"` + src + `"}}`))
		assert.Error(err, src)
		assert.Contains(err.Error(), "malformed custom-schema source", src)
	}
}

func TestParseAttributeMaps_PrimarySelectorRejectedForCustomSource(t *testing.T) {
	assert := assert.New(t)

	_, err := ParseAttributeMaps([]byte(`{"user":{"Title":"customSchemas.X.y[primary]"}}`))
	assert.Error(err)
	msg := err.Error()
	assert.Contains(msg, "malformed custom-schema source")
	assert.Contains(msg, "[primary] is not valid for custom sources")
}

func TestParseAttributeMaps_DeferredAddressTypeSubFieldsRejected(t *testing.T) {
	assert := assert.New(t)

	// Even under the type selector, addresses sub-fields other than formatted
	// stay rejected this phase (design §5.2 / §1.3).
	_, err := ParseAttributeMaps([]byte(`{"user":{"Addresses":"addresses[type=work].streetAddress"}}`))
	assert.Error(err)
	assert.Contains(err.Error(), `unsupported source attribute "addresses[type=work].streetAddress"`)
}

func TestParseAttributeMaps_MalformedSelectorRejected(t *testing.T) {
	assert := assert.New(t)

	// Empty type= value.
	_, err := ParseAttributeMaps([]byte(`{"user":{"Title":"organizations[type=].title"}}`))
	assert.Error(err)
	assert.Contains(err.Error(), "malformed selector")

	// Unknown selector keyword.
	_, err = ParseAttributeMaps([]byte(`{"user":{"Title":"organizations[latest].title"}}`))
	assert.Error(err)
	assert.Contains(err.Error(), "malformed selector")
}

func TestParseAttributeMaps_UnknownBasePathRejected(t *testing.T) {
	assert := assert.New(t)

	_, err := ParseAttributeMaps([]byte(`{"user":{"Title":"departments[type=work].name"}}`))
	assert.Error(err)
	assert.Contains(err.Error(), `unsupported source attribute "departments[type=work].name"`)
}

// Atomic-landing guard (design §7.1/§13): the empty document and a group-only
// partial document must BOTH load under the new organizations[type=work].*
// default. This FAILS if the default is flipped without the validator learning
// to accept type= selectors.
func TestParseAttributeMaps_AtomicLandingDefaultLoads(t *testing.T) {
	assert := assert.New(t)

	am, err := ParseAttributeMaps([]byte(`{}`))
	assert.NoError(err)
	assert.Equal("organizations[type=work].title", am.User["Title"])

	am, err = ParseAttributeMaps([]byte(`{"group":{"DisplayName":"name"}}`))
	assert.NoError(err)
	assert.Equal(DefaultUserAttributeMap, am.User)
}

func TestParseAttributeMaps_UnsupportedSourceAttr(t *testing.T) {
	assert := assert.New(t)

	// "email" is not a supported group source attribute (only "name" is).
	_, err := ParseAttributeMaps([]byte(`{"group":{"DisplayName":"email"}}`))
	assert.Error(err)
	msg := err.Error()
	assert.Contains(msg, "attribute map (group block)")
	assert.Contains(msg, `unsupported source attribute "email"`)
	assert.Contains(msg, "valid group source attributes are: name")
}

func TestParseAttributeMaps_EmptyKeyFailsValidation(t *testing.T) {
	assert := assert.New(t)

	_, err := ParseAttributeMaps([]byte(`{"user":{"":"primaryEmail"}}`))
	assert.Error(err)
	msg := err.Error()
	assert.Contains(msg, "attribute map (user block)")
	assert.Contains(msg, "empty target field")
}

func TestParseAttributeMaps_MultipleViolationsAggregated(t *testing.T) {
	assert := assert.New(t)

	// Two distinct problems in the user block: an unsupported target field and
	// an unsupported source attribute. Both must be reported in one error.
	_, err := ParseAttributeMaps([]byte(`{"user":{"Bogus":"name.givenName","Username":"notAnAttr"}}`))
	assert.Error(err)
	msg := err.Error()
	assert.Contains(msg, `unsupported target field "Bogus"`)
	assert.Contains(msg, `unsupported source attribute "notAnAttr"`)
}
