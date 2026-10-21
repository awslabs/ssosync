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

	// Username now comes from the given name, not the primary email.
	assert.Equal(t, "John", got.Username)
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

func TestMapFromGoogleUser_NonPrimaryOrganizationStillSelected(t *testing.T) {
	// No entry is marked primary -> the first entry is selected (fallback rule).
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

	assert.Equal(t, "First Title", got.Title)
	if assert.NotNil(t, got.Enterprise) {
		assert.Equal(t, "First Dept", got.Enterprise.Department)
		assert.Equal(t, "First Org", got.Enterprise.Organization)
		assert.Equal(t, "CC-1", got.Enterprise.CostCenter)
	}
}

func TestMapFromGoogleUser_PrimaryOrganizationPreferred(t *testing.T) {
	// The entry marked primary wins even when it is not first.
	u := &admin.User{
		Id:           "G4",
		PrimaryEmail: "p@example.com",
		Name:         &admin.UserName{GivenName: "Pri", FamilyName: "Mary"},
		Organizations: []admin.UserOrganization{
			{Title: "Not This", Primary: false},
			{Title: "The Primary", Department: "D", Primary: true},
		},
	}

	got, err := mapFromGoogleUser(u, config.DefaultUserAttributeMap)
	assert.NoError(t, err)

	assert.Equal(t, "The Primary", got.Title)
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
