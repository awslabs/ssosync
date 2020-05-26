package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGroup(t *testing.T) {
	g := NewGroup("Test Group")

	assert.Len(t, g.Schemas, 1)
	assert.Equal(t, g.Schemas[0], "urn:ietf:params:scim:schemas:core:2.0:Group")
	assert.Equal(t, g.DisplayName, "Test Group")
}
