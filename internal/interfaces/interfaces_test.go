package interfaces

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_Struct(t *testing.T) {
	user := User{
		ID:          "user-123",
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Username:    "test@example.com",
		ExternalId:  "google-123",
		DisplayName: "Test User",
		Active:      true,
		Emails: []UserEmail{
			{
				Value:   "test@example.com",
				Type:    "work",
				Primary: true,
			},
		},
		Addresses: []UserAddress{
			{
				Type: "work",
			},
		},
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{
			GivenName:  "Test",
			FamilyName: "User",
		},
	}

	assert.Equal(t, "user-123", user.ID)
	assert.Equal(t, "test@example.com", user.Username)
	assert.Equal(t, "google-123", user.ExternalId)
	assert.Equal(t, "Test User", user.DisplayName)
	assert.True(t, user.Active)
	assert.Len(t, user.Emails, 1)
	assert.Equal(t, "test@example.com", user.Emails[0].Value)
	assert.Equal(t, "work", user.Emails[0].Type)
	assert.True(t, user.Emails[0].Primary)
	assert.Len(t, user.Addresses, 1)
	assert.Equal(t, "work", user.Addresses[0].Type)
	assert.Equal(t, "Test", user.Name.GivenName)
	assert.Equal(t, "User", user.Name.FamilyName)
}

func TestGroup_Struct(t *testing.T) {
	group := Group{
		ID:          "group-123",
		ExternalId:  "google-123",
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		DisplayName: "Test Group",
		Members:     []string{"user-1", "user-2"},
	}

	assert.Equal(t, "group-123", group.ID)
	assert.Equal(t, "google-123", group.ExternalId)
	assert.Equal(t, "Test Group", group.DisplayName)
	assert.Len(t, group.Members, 2)
	assert.Contains(t, group.Members, "user-1")
	assert.Contains(t, group.Members, "user-2")
}

func TestUserEmail_Struct(t *testing.T) {
	email := UserEmail{
		Value:   "test@example.com",
		Type:    "work",
		Primary: true,
	}

	assert.Equal(t, "test@example.com", email.Value)
	assert.Equal(t, "work", email.Type)
	assert.True(t, email.Primary)
}

func TestUserAddress_Struct(t *testing.T) {
	address := UserAddress{
		Type: "work",
	}

	assert.Equal(t, "work", address.Type)
}

func TestUserFilterResults_Struct(t *testing.T) {
	results := UserFilterResults{
		TotalResults: 2,
		Resources: []User{
			{ID: "user-1", Username: "user1@example.com"},
			{ID: "user-2", Username: "user2@example.com"},
		},
	}

	assert.Equal(t, 2, results.TotalResults)
	assert.Len(t, results.Resources, 2)
	assert.Equal(t, "user-1", results.Resources[0].ID)
	assert.Equal(t, "user-2", results.Resources[1].ID)
}

func TestGroupFilterResults_Struct(t *testing.T) {
	results := GroupFilterResults{
		TotalResults: 2,
		Resources: []Group{
			{ID: "group-1", DisplayName: "Group 1"},
			{ID: "group-2", DisplayName: "Group 2"},
		},
	}

	assert.Equal(t, 2, results.TotalResults)
	assert.Len(t, results.Resources, 2)
	assert.Equal(t, "group-1", results.Resources[0].ID)
	assert.Equal(t, "group-2", results.Resources[1].ID)
}
