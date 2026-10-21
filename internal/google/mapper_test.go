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
		PrimaryEmail: "jdoe@example.com",
		Name: &admin.UserName{
			GivenName:  "John",
			FamilyName: "Doe",
		},
		Suspended: false,
	}

	got := mapFromGoogleUser(u, config.DefaultUserAttributeMap)

	assert.Equal(t, "jdoe@example.com", got.Username)
	assert.Equal(t, "John", got.Name.GivenName)
	assert.Equal(t, "Doe", got.Name.FamilyName)
	// Active is the logical inverse of Suspended.
	assert.True(t, got.Active)
	assert.Equal(t, []string{constants.SCIMSchemaUser}, got.Schemas)
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

	got := mapFromGoogleUser(u, config.DefaultUserAttributeMap)

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

	got := mapFromGoogleUser(u, config.DefaultUserAttributeMap)

	assert.Equal(t, "Cher", got.Name.GivenName)
	assert.Equal(t, " ", got.Name.FamilyName)
}

func TestMapFromGoogleUser_NilNameDoesNotPanic(t *testing.T) {
	u := &admin.User{
		PrimaryEmail: "noname@example.com",
		Name:         nil,
		Suspended:    false,
	}

	got := mapFromGoogleUser(u, config.DefaultUserAttributeMap)

	assert.Equal(t, "noname@example.com", got.Username)
	assert.Equal(t, "", got.Name.GivenName)
	assert.Equal(t, "", got.Name.FamilyName)
	assert.True(t, got.Active)
}

func TestMapFromGoogleGroup_DefaultMap(t *testing.T) {
	g := &admin.Group{
		Id:    "group1",
		Email: "group1@example.com",
		Name:  "Group One",
	}

	got := mapFromGoogleGroup(g, config.DefaultGroupAttributeMap)

	assert.Equal(t, "Group One", got.DisplayName)
	assert.Equal(t, []string{constants.SCIMSchemaGroup}, got.Schemas)
}

func TestMapFromGoogleGroup_ZeroWidthSpaceNormalized(t *testing.T) {
	g := &admin.Group{
		Id:    "group2",
		Email: "group2@example.com",
		Name:  string('\u200B'),
	}

	got := mapFromGoogleGroup(g, config.DefaultGroupAttributeMap)

	assert.Equal(t, " ", got.DisplayName)
}
