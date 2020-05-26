package aws

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewUser(t *testing.T) {
	u := NewUser("Lee", "Packham", "test@email.com")

	assert.Equal(t, u.Name.GivenName, "Lee")
	assert.Equal(t, u.Name.FamilyName, "Packham")
	assert.Equal(t, u.DisplayName, "Lee Packham")
	assert.Len(t, u.Emails, 1)

	assert.Equal(t, u.Emails[0].Value, "test@email.com")
	assert.Equal(t, u.Emails[0].Primary, true)

	assert.Len(t, u.Schemas, 1)
	assert.Equal(t, u.Schemas[0], "urn:ietf:params:scim:schemas:core:2.0:User")
}