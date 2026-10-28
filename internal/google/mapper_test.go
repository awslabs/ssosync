package google

import (
	"testing"

	"ssosync/internal/config"
	"ssosync/internal/constants"

	"github.com/stretchr/testify/assert"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestMapFromGoogleUser_DefaultMap(t *testing.T) {
	u := &admin.User{
		Id:           "G1",
		PrimaryEmail: "jdoe@example.com",
		Name: &admin.UserName{
			GivenName:  "John",
			FamilyName: "Doe",
		},
		Suspended: false,
	}

	got, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, "jdoe@example.com", got.Username)
	assert.Equal(t, "John", got.Name.GivenName)
	assert.Equal(t, "Doe", got.Name.FamilyName)
	// Active is the logical inverse of Suspended.
	assert.True(t, got.Active)
	assert.Equal(t, []string{constants.SCIMSchemaUser}, got.Schemas)
	// ExternalId.Id carries the Google user id, matching the live path.
	assert.Equal(t, "G1", got.ExternalId.Id)
	// DisplayName is derived as "GivenName FamilyName", matching aws.NewUser.
	assert.Equal(t, "John Doe", got.DisplayName)
}

func TestMapFromGoogleUser_SuspendedInvertsToInactive(t *testing.T) {
	u := &admin.User{
		PrimaryEmail: "suspended@example.com",
		Name: &admin.UserName{
			GivenName:  "Sus",
			FamilyName: "Pended",
		},
		Suspended: true,
	}

	got, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.NoError(t, err)

	// Suspended: true -> Active: false
	assert.False(t, got.Active)
}

func TestMapFromGoogleUser_ZeroWidthSpaceNormalized(t *testing.T) {
	// Google directory accepts a zero-width space for an empty name; Identity
	// Store expects a plain space. The mapper must normalize it, matching getUsers.
	u := &admin.User{
		PrimaryEmail: "mononym@example.com",
		Name: &admin.UserName{
			GivenName:  "Cher",
			FamilyName: string('\u200B'),
		},
		Suspended: false,
	}

	got, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, "Cher", got.Name.GivenName)
	assert.Equal(t, " ", got.Name.FamilyName)
	// DisplayName uses the normalized name fields.
	assert.Equal(t, "Cher  ", got.DisplayName)
}

func TestMapFromGoogleUser_NilNameDoesNotPanic(t *testing.T) {
	u := &admin.User{
		PrimaryEmail: "noname@example.com",
		Name:         nil,
		Suspended:    false,
	}

	got, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, "noname@example.com", got.Username)
	assert.Equal(t, "", got.Name.GivenName)
	assert.Equal(t, "", got.Name.FamilyName)
	assert.True(t, got.Active)
}

// A non-default custom map must reroute a field: here Username is fed from
// name.givenName instead of primaryEmail, proving the mapper is driven by the
// map rather than hardcoded.
func TestMapFromGoogleUser_CustomMapRerootesField(t *testing.T) {
	u := &admin.User{
		Id:           "G9",
		PrimaryEmail: "jdoe@example.com",
		Name: &admin.UserName{
			GivenName:  "John",
			FamilyName: "Doe",
		},
		Suspended: false,
	}

	customMap := config.AttributeMap{
		"Username":        "name.givenName",
		"Name.FamilyName": "name.familyName",
	}

	got, err := mapFromGoogleUser(u, customMap)
	assert.NoError(t, err)

	// Username now comes from the mapped non-email source (name.givenName), NOT
	// from primaryEmail. The desired payload therefore carries the mapped value
	// ("John") rather than "jdoe@example.com", proving Username is driven by the
	// map even when it is sourced from a field other than the primary email.
	assert.Equal(t, "John", got.Username)
	assert.NotEqual(t, u.PrimaryEmail, got.Username)
	assert.Equal(t, "Doe", got.Name.FamilyName)
	// GivenName was not mapped in this custom map, so it stays empty.
	assert.Equal(t, "", got.Name.GivenName)
}

func TestMapFromGoogleUser_DefaultMap_PrimaryOrganization(t *testing.T) {
	// A user with a primary organization entry: the default map must populate
	// Title and the Enterprise.* safe-set fields from it.
	u := &admin.User{
		Id:           "G2",
		PrimaryEmail: "eng@example.com",
		Name:         &admin.UserName{GivenName: "Eng", FamilyName: "User"},
		Organizations: []admin.UserOrganization{
			{
				Title:      "Staff Engineer",
				Department: "Platform",
				Name:       "ExampleCorp",
				CostCenter: "CC-100",
				Type:       "work",
				Primary:    true,
			},
		},
	}

	got, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, "Staff Engineer", got.Title)
	if assert.NotNil(t, got.Enterprise) {
		assert.Equal(t, "Platform", got.Enterprise.Department)
		assert.Equal(t, "ExampleCorp", got.Enterprise.Organization)
		assert.Equal(t, "CC-100", got.Enterprise.CostCenter)
		// Manager is NOT in the default map, so it stays nil.
		assert.Nil(t, got.Enterprise.Manager)
	}
}

func TestMapFromGoogleUser_NonPrimaryOrganizationResolvesToNothing(t *testing.T) {
	// Under the agreed rule two non-primary orgs resolve to NOTHING: there is no
	// first-entry fallback, so a multi-valued field with no primary entry resolves
	// to the zero value. Title stays empty and Enterprise stays nil (the lazy
	// allocator never runs because every Enterprise.* source resolved to "").
	u := &admin.User{
		Id:           "G3",
		PrimaryEmail: "np@example.com",
		Name:         &admin.UserName{GivenName: "No", FamilyName: "Primary"},
		Organizations: []admin.UserOrganization{
			{
				Title:      "First Title",
				Department: "First Dept",
				Name:       "First Org",
				CostCenter: "CC-1",
				Primary:    false,
			},
			{
				Title:   "Second Title",
				Primary: false,
			},
		},
	}

	got, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, "", got.Title)
	assert.Nil(t, got.Enterprise)
}

func TestMapFromGoogleUser_PrimaryOrganizationPreferred(t *testing.T) {
	// Under the default map (organizations[primary].*), the primary entry wins
	// even when it is not first and is untyped (as the Admin console stores
	// it); a work-typed entry that is not primary is ignored.
	u := &admin.User{
		Id:           "G4",
		PrimaryEmail: "p@example.com",
		Name:         &admin.UserName{GivenName: "Pri", FamilyName: "Mary"},
		Organizations: []admin.UserOrganization{
			{Title: "Not This", Type: "work", Primary: false},
			{Title: "The Primary Org", Department: "D", Type: "", Primary: true},
		},
	}

	got, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, "The Primary Org", got.Title)
	if assert.NotNil(t, got.Enterprise) {
		assert.Equal(t, "D", got.Enterprise.Department)
	}
}

func TestMapFromGoogleUser_NoOrganizationsLeavesEnterpriseNil(t *testing.T) {
	// Acceptance criterion 5: no org data -> Title empty, Enterprise nil, no panic.
	u := &admin.User{
		Id:           "G5",
		PrimaryEmail: "bare@example.com",
		Name:         &admin.UserName{GivenName: "Bare", FamilyName: "User"},
	}

	got, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, "", got.Title)
	assert.Nil(t, got.Enterprise)
}

func TestMapFromGoogleUser_MalformedOrganizationsReturnsError(t *testing.T) {
	// A value that cannot round-trip into []admin.UserOrganization is an internal
	// invariant breach surfaced as an error, not a silent drop.
	u := &admin.User{
		Id:           "G6",
		PrimaryEmail: "bad@example.com",
		Name:         &admin.UserName{GivenName: "Bad", FamilyName: "Data"},
		// A bare string where the SDK expects an array of objects.
		Organizations: "not-an-array",
	}

	_, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal multi-valued field")
}

func TestMapFromGoogleUser_OptInManagerRelationPresent(t *testing.T) {
	u := &admin.User{
		Id:           "G7",
		PrimaryEmail: "report@example.com",
		Name:         &admin.UserName{GivenName: "Report", FamilyName: "Ee"},
		Relations: []admin.UserRelation{
			{Type: "assistant", Value: "asst@example.com"},
			{Type: "Manager", Value: "boss@example.com"},
		},
	}

	optIn := config.AttributeMap{
		"Username":           "primaryEmail",
		"Enterprise.Manager": "relations[type=manager].value",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	if assert.NotNil(t, got.Enterprise) && assert.NotNil(t, got.Enterprise.Manager) {
		assert.Equal(t, "boss@example.com", got.Enterprise.Manager.Value)
	}
}

func TestMapFromGoogleUser_OptInManagerRelationAbsent(t *testing.T) {
	u := &admin.User{
		Id:           "G8",
		PrimaryEmail: "noboss@example.com",
		Name:         &admin.UserName{GivenName: "No", FamilyName: "Boss"},
		Relations: []admin.UserRelation{
			{Type: "assistant", Value: "asst@example.com"},
		},
	}

	optIn := config.AttributeMap{
		"Username":           "primaryEmail",
		"Enterprise.Manager": "relations[type=manager].value",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	// No manager relation -> Enterprise stays nil (empty-safe, no churn).
	assert.Nil(t, got.Enterprise)
}

func TestMapFromGoogleUser_OptInEmailsShaping(t *testing.T) {
	u := &admin.User{
		Id:           "G10",
		PrimaryEmail: "primary@example.com",
		Name:         &admin.UserName{GivenName: "Email", FamilyName: "Shape"},
		Emails: []admin.UserEmail{
			{Address: "home@example.com", Type: "home", Primary: false},
			{Address: "office@example.com", Type: "work", Primary: true},
		},
	}

	optIn := config.AttributeMap{
		"Username": "primaryEmail",
		"Emails":   "emails[primary].address",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	if assert.Len(t, got.Emails, 1) {
		assert.Equal(t, "office@example.com", got.Emails[0].Value)
		assert.Equal(t, "work", got.Emails[0].Type)
		assert.True(t, got.Emails[0].Primary)
	}
}

func TestMapFromGoogleUser_OptInAddressesTypeDefaultsToWork(t *testing.T) {
	// The primary address has an empty Google type -> defaulted to "work".
	u := &admin.User{
		Id:           "G11",
		PrimaryEmail: "addr@example.com",
		Name:         &admin.UserName{GivenName: "Addr", FamilyName: "Shape"},
		Addresses: []admin.UserAddress{
			{Formatted: "1 Example Way", Type: "", Primary: true},
		},
	}

	optIn := config.AttributeMap{
		"Username":  "primaryEmail",
		"Addresses": "addresses[primary].formatted",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	if assert.Len(t, got.Addresses, 1) {
		assert.Equal(t, "1 Example Way", got.Addresses[0].Formatted)
		assert.Equal(t, "work", got.Addresses[0].Type)
		assert.True(t, got.Addresses[0].Primary)
	}
}

func TestMapFromGoogleUser_OptInPhoneNumbersShaping(t *testing.T) {
	u := &admin.User{
		Id:           "G12",
		PrimaryEmail: "phone@example.com",
		Name:         &admin.UserName{GivenName: "Phone", FamilyName: "Shape"},
		Phones: []admin.UserPhone{
			{Value: "+15551234567", Type: "mobile", Primary: true},
		},
	}

	optIn := config.AttributeMap{
		"Username":     "primaryEmail",
		"PhoneNumbers": "phones[primary].value",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	if assert.Len(t, got.PhoneNumbers, 1) {
		assert.Equal(t, "+15551234567", got.PhoneNumbers[0].Value)
		assert.Equal(t, "mobile", got.PhoneNumbers[0].Type)
		assert.True(t, got.PhoneNumbers[0].Primary)
	}
}

func TestMapFromGoogleUser_OptInMultiValuedEmptyLeavesNil(t *testing.T) {
	// No emails/addresses/phones data -> the slices stay nil so buildUser
	// synthesis still applies.
	u := &admin.User{
		Id:           "G13",
		PrimaryEmail: "empty@example.com",
		Name:         &admin.UserName{GivenName: "Empty", FamilyName: "User"},
	}

	optIn := config.AttributeMap{
		"Username":     "primaryEmail",
		"Emails":       "emails[primary].address",
		"Addresses":    "addresses[primary].formatted",
		"PhoneNumbers": "phones[primary].value",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	assert.Nil(t, got.Emails)
	assert.Nil(t, got.Addresses)
	assert.Nil(t, got.PhoneNumbers)
}

func TestMapFromGoogleUser_NonPrimaryEmailsResolvesToNothing(t *testing.T) {
	// Entries present but NONE primary -> resolves to nothing (no first-entry
	// fallback). FAILS against pre-fix code that selected entry [0].
	u := &admin.User{
		Id:           "G14",
		PrimaryEmail: "np-email@example.com",
		Name:         &admin.UserName{GivenName: "No", FamilyName: "Primary"},
		Emails: []admin.UserEmail{
			{Address: "home@example.com", Type: "home", Primary: false},
			{Address: "office@example.com", Type: "work", Primary: false},
		},
	}

	optIn := config.AttributeMap{
		"Username": "primaryEmail",
		"Emails":   "emails[primary].address",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	assert.Nil(t, got.Emails)
}

func TestMapFromGoogleUser_NonPrimaryPhonesResolvesToNothing(t *testing.T) {
	// Entries present but NONE primary -> resolves to nothing (no first-entry
	// fallback). FAILS against pre-fix code that selected entry [0].
	u := &admin.User{
		Id:           "G15",
		PrimaryEmail: "np-phone@example.com",
		Name:         &admin.UserName{GivenName: "No", FamilyName: "Primary"},
		Phones: []admin.UserPhone{
			{Value: "+15550000001", Type: "mobile", Primary: false},
			{Value: "+15550000002", Type: "work", Primary: false},
		},
	}

	optIn := config.AttributeMap{
		"Username":     "primaryEmail",
		"PhoneNumbers": "phones[primary].value",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	assert.Nil(t, got.PhoneNumbers)
}

func TestMapFromGoogleUser_NonPrimaryAddressesResolvesToNothing(t *testing.T) {
	// Entries present but NONE primary -> resolves to nothing (no first-entry
	// fallback). FAILS against pre-fix code that selected entry [0].
	u := &admin.User{
		Id:           "G16",
		PrimaryEmail: "np-addr@example.com",
		Name:         &admin.UserName{GivenName: "No", FamilyName: "Primary"},
		Addresses: []admin.UserAddress{
			{Formatted: "1 First St", Type: "home", Primary: false},
			{Formatted: "2 Second St", Type: "work", Primary: false},
		},
	}

	optIn := config.AttributeMap{
		"Username":  "primaryEmail",
		"Addresses": "addresses[primary].formatted",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	assert.Nil(t, got.Addresses)
}

// Item 6b (mapper level): several addresses, exactly one primary (not first) ->
// only the PRIMARY one is emitted as a single-element SCIM address.
func TestMapFromGoogleUser_MultipleAddressesPrimarySelected(t *testing.T) {
	u := &admin.User{
		Id:           "G17",
		PrimaryEmail: "multi-addr@example.com",
		Name:         &admin.UserName{GivenName: "Multi", FamilyName: "Addr"},
		Addresses: []admin.UserAddress{
			{Formatted: "1 First St", Type: "home", Primary: false},
			{Formatted: "2 Primary Ave", Type: "work", Primary: true},
			{Formatted: "3 Third Rd", Type: "other", Primary: false},
		},
	}

	optIn := config.AttributeMap{
		"Username":  "primaryEmail",
		"Addresses": "addresses[primary].formatted",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	if assert.Len(t, got.Addresses, 1) {
		assert.Equal(t, "2 Primary Ave", got.Addresses[0].Formatted)
		assert.Equal(t, "work", got.Addresses[0].Type)
		assert.True(t, got.Addresses[0].Primary)
	}
}

func TestMapFromGoogleUser_MultipleAddressesNoPrimaryResolvesToNothing(t *testing.T) {
	u := &admin.User{
		Id:           "G18",
		PrimaryEmail: "multi-addr-np@example.com",
		Name:         &admin.UserName{GivenName: "Multi", FamilyName: "Addr"},
		Addresses: []admin.UserAddress{
			{Formatted: "1 First St", Type: "home", Primary: false},
			{Formatted: "2 Second Ave", Type: "work", Primary: false},
			{Formatted: "3 Third Rd", Type: "other", Primary: false},
		},
	}

	optIn := config.AttributeMap{
		"Username":  "primaryEmail",
		"Addresses": "addresses[primary].formatted",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	assert.Nil(t, got.Addresses)
}

func TestMapFromGoogleUser_MultipleEmailsPrimarySelected(t *testing.T) {
	u := &admin.User{
		Id:           "G19",
		PrimaryEmail: "multi-email@example.com",
		Name:         &admin.UserName{GivenName: "Multi", FamilyName: "Email"},
		Emails: []admin.UserEmail{
			{Address: "home@example.com", Type: "home", Primary: false},
			{Address: "office@example.com", Type: "work", Primary: true},
			{Address: "other@example.com", Type: "other", Primary: false},
		},
	}

	optIn := config.AttributeMap{
		"Username": "primaryEmail",
		"Emails":   "emails[primary].address",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	if assert.Len(t, got.Emails, 1) {
		assert.Equal(t, "office@example.com", got.Emails[0].Value)
		assert.Equal(t, "work", got.Emails[0].Type)
		assert.True(t, got.Emails[0].Primary)
	}
}

func TestMapFromGoogleUser_MultipleEmailsNoPrimaryResolvesToNothing(t *testing.T) {
	u := &admin.User{
		Id:           "G20",
		PrimaryEmail: "multi-email-np@example.com",
		Name:         &admin.UserName{GivenName: "Multi", FamilyName: "Email"},
		Emails: []admin.UserEmail{
			{Address: "home@example.com", Type: "home", Primary: false},
			{Address: "office@example.com", Type: "work", Primary: false},
			{Address: "other@example.com", Type: "other", Primary: false},
		},
	}

	optIn := config.AttributeMap{
		"Username": "primaryEmail",
		"Emails":   "emails[primary].address",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	assert.Nil(t, got.Emails)
}

func TestMapFromGoogleUser_MultiplePhonesPrimarySelected(t *testing.T) {
	u := &admin.User{
		Id:           "G21",
		PrimaryEmail: "multi-phone@example.com",
		Name:         &admin.UserName{GivenName: "Multi", FamilyName: "Phone"},
		Phones: []admin.UserPhone{
			{Value: "+15550000001", Type: "home", Primary: false},
			{Value: "+15550000002", Type: "mobile", Primary: true},
			{Value: "+15550000003", Type: "work", Primary: false},
		},
	}

	optIn := config.AttributeMap{
		"Username":     "primaryEmail",
		"PhoneNumbers": "phones[primary].value",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	if assert.Len(t, got.PhoneNumbers, 1) {
		assert.Equal(t, "+15550000002", got.PhoneNumbers[0].Value)
		assert.Equal(t, "mobile", got.PhoneNumbers[0].Type)
		assert.True(t, got.PhoneNumbers[0].Primary)
	}
}

func TestMapFromGoogleUser_MultiplePhonesNoPrimaryResolvesToNothing(t *testing.T) {
	u := &admin.User{
		Id:           "G22",
		PrimaryEmail: "multi-phone-np@example.com",
		Name:         &admin.UserName{GivenName: "Multi", FamilyName: "Phone"},
		Phones: []admin.UserPhone{
			{Value: "+15550000001", Type: "home", Primary: false},
			{Value: "+15550000002", Type: "mobile", Primary: false},
			{Value: "+15550000003", Type: "work", Primary: false},
		},
	}

	optIn := config.AttributeMap{
		"Username":     "primaryEmail",
		"PhoneNumbers": "phones[primary].value",
	}

	got, err := mapFromGoogleUser(u, optIn)
	assert.NoError(t, err)

	assert.Nil(t, got.PhoneNumbers)
}

// shuffledUser rebuilds the same logical user with the multi-valued entries
// reordered by the supplied permutation helper. It is used by the shuffled-order
// churn regression tests (item 6c) with EXPLICIT deterministic permutations (the
// identity order, a reversed copy, and one more fixed rotation) rather than a
// random seed.
func reverseOrgs(in []admin.UserOrganization) []admin.UserOrganization {
	out := make([]admin.UserOrganization, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}

func reverseEmails(in []admin.UserEmail) []admin.UserEmail {
	out := make([]admin.UserEmail, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}

func reverseAddresses(in []admin.UserAddress) []admin.UserAddress {
	out := make([]admin.UserAddress, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}

// rotateOrgs / rotateEmails / rotateAddresses apply a fixed single-position
// left rotation, giving a third deterministic ordering distinct from identity
// and reversed.
func rotateOrgs(in []admin.UserOrganization) []admin.UserOrganization {
	if len(in) < 2 {
		return append([]admin.UserOrganization(nil), in...)
	}
	return append(append([]admin.UserOrganization(nil), in[1:]...), in[0])
}

func rotateEmails(in []admin.UserEmail) []admin.UserEmail {
	if len(in) < 2 {
		return append([]admin.UserEmail(nil), in...)
	}
	return append(append([]admin.UserEmail(nil), in[1:]...), in[0])
}

func rotateAddresses(in []admin.UserAddress) []admin.UserAddress {
	if len(in) < 2 {
		return append([]admin.UserAddress(nil), in...)
	}
	return append(append([]admin.UserAddress(nil), in[1:]...), in[0])
}

// Item 6c (mapper level): with exactly one primary entry in each multi-valued
// field, the mapped result is IDENTICAL regardless of entry ordering. Uses
// explicit deterministic permutations (identity, reversed, rotated), NOT a
// random seed.
func TestMapFromGoogleUser_ShuffledOrderPrimaryStable(t *testing.T) {
	orgs := []admin.UserOrganization{
		{Title: "Not This", Department: "DX", Name: "OrgX", CostCenter: "CCX", Primary: false},
		{Title: "The Primary", Department: "Platform", Name: "ExampleCorp", CostCenter: "CC-100", Primary: true},
		{Title: "Also Not", Department: "DY", Name: "OrgY", CostCenter: "CCY", Primary: false},
	}
	emails := []admin.UserEmail{
		{Address: "home@example.com", Type: "home", Primary: false},
		{Address: "office@example.com", Type: "work", Primary: true},
		{Address: "other@example.com", Type: "other", Primary: false},
	}
	addrs := []admin.UserAddress{
		{Formatted: "1 First St", Type: "home", Primary: false},
		{Formatted: "2 Primary Ave", Type: "work", Primary: true},
		{Formatted: "3 Third Rd", Type: "other", Primary: false},
	}

	optIn := config.AttributeMap{
		"Name.GivenName":          "name.givenName",
		"Name.FamilyName":         "name.familyName",
		"Username":                "primaryEmail",
		"Title":                   "organizations[primary].title",
		"Enterprise.Department":   "organizations[primary].department",
		"Enterprise.Organization": "organizations[primary].name",
		"Emails":                  "emails[primary].address",
		"Addresses":               "addresses[primary].formatted",
	}

	build := func(o []admin.UserOrganization, e []admin.UserEmail, a []admin.UserAddress) *admin.User {
		return &admin.User{
			Id:            "G23",
			PrimaryEmail:  "shuffle@example.com",
			Name:          &admin.UserName{GivenName: "Shuf", FamilyName: "Fle"},
			Organizations: o,
			Emails:        e,
			Addresses:     a,
		}
	}

	base, err := mapFromGoogleUser(build(orgs, emails, addrs), optIn)
	assert.NoError(t, err)

	reversed, err := mapFromGoogleUser(build(reverseOrgs(orgs), reverseEmails(emails), reverseAddresses(addrs)), optIn)
	assert.NoError(t, err)

	rotated, err := mapFromGoogleUser(build(rotateOrgs(orgs), rotateEmails(emails), rotateAddresses(addrs)), optIn)
	assert.NoError(t, err)

	// The whole mapped user is identical across all orderings because the primary
	// entry is selected deterministically regardless of position.
	assert.Equal(t, base, reversed)
	assert.Equal(t, base, rotated)
	assert.Equal(t, "The Primary", base.Title)
	assert.Equal(t, "office@example.com", base.Emails[0].Value)
	assert.Equal(t, "2 Primary Ave", base.Addresses[0].Formatted)
}

// Item 6c (mapper level): with NO primary entry in any field, every selected
// target resolves to NOTHING in EVERY ordering. This FAILS against pre-fix
// first-fallback code (different orderings selected different entry [0]) and
// PASSES after the behaviour fix.
func TestMapFromGoogleUser_ShuffledOrderNoPrimaryResolvesToNothing(t *testing.T) {
	orgs := []admin.UserOrganization{
		{Title: "Title A", Department: "DA", Name: "OrgA", Primary: false},
		{Title: "Title B", Department: "DB", Name: "OrgB", Primary: false},
		{Title: "Title C", Department: "DC", Name: "OrgC", Primary: false},
	}
	emails := []admin.UserEmail{
		{Address: "a@example.com", Type: "home", Primary: false},
		{Address: "b@example.com", Type: "work", Primary: false},
		{Address: "c@example.com", Type: "other", Primary: false},
	}
	addrs := []admin.UserAddress{
		{Formatted: "1 A St", Type: "home", Primary: false},
		{Formatted: "2 B St", Type: "work", Primary: false},
		{Formatted: "3 C St", Type: "other", Primary: false},
	}

	optIn := config.AttributeMap{
		"Name.GivenName":          "name.givenName",
		"Name.FamilyName":         "name.familyName",
		"Username":                "primaryEmail",
		"Title":                   "organizations[primary].title",
		"Enterprise.Department":   "organizations[primary].department",
		"Enterprise.Organization": "organizations[primary].name",
		"Emails":                  "emails[primary].address",
		"Addresses":               "addresses[primary].formatted",
	}

	build := func(o []admin.UserOrganization, e []admin.UserEmail, a []admin.UserAddress) *admin.User {
		return &admin.User{
			Id:            "G24",
			PrimaryEmail:  "shuffle-np@example.com",
			Name:          &admin.UserName{GivenName: "Shuf", FamilyName: "Fle"},
			Organizations: o,
			Emails:        e,
			Addresses:     a,
		}
	}

	for _, tc := range []struct {
		name string
		u    *admin.User
	}{
		{"identity", build(orgs, emails, addrs)},
		{"reversed", build(reverseOrgs(orgs), reverseEmails(emails), reverseAddresses(addrs))},
		{"rotated", build(rotateOrgs(orgs), rotateEmails(emails), rotateAddresses(addrs))},
	} {
		got, err := mapFromGoogleUser(tc.u, optIn)
		assert.NoError(t, err, tc.name)
		assert.Equal(t, "", got.Title, tc.name)
		assert.Nil(t, got.Enterprise, tc.name)
		assert.Nil(t, got.Emails, tc.name)
		assert.Nil(t, got.Addresses, tc.name)
	}
}

func TestMapFromGoogleGroup_DefaultMap(t *testing.T) {
	g := &admin.Group{
		Id:    "group1",
		Email: "group1@example.com",
		Name:  "Group One",
	}

	got, err := mapFromGoogleGroup(g, config.DefaultGroupAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, "Group One", got.DisplayName)
	assert.Equal(t, []string{constants.SCIMSchemaGroup}, got.Schemas)
	// ExternalId.Id carries the Google group id, matching the live path.
	assert.Equal(t, "group1", got.ExternalId.Id)
}

func TestMapFromGoogleGroup_ZeroWidthSpaceNormalized(t *testing.T) {
	g := &admin.Group{
		Id:    "group2",
		Email: "group2@example.com",
		Name:  string('\u200B'),
	}

	got, err := mapFromGoogleGroup(g, config.DefaultGroupAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, " ", got.DisplayName)
}
