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
