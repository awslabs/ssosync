package aws

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var awsIDRegex = regexp.MustCompile(`^([0-9a-f]{10}-|)[A-Fa-f0-9]{8}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{4}-[A-Fa-f0-9]{12}$`)

func TestVirtualUserID_MatchesAWSRegex(t *testing.T) {
	id := virtualUserID("alice@example.com")
	assert.Regexp(t, awsIDRegex, id)
}

func TestVirtualGroupID_MatchesAWSRegex(t *testing.T) {
	id := virtualGroupID("Engineering")
	assert.Regexp(t, awsIDRegex, id)
}

func TestVirtualMembershipID_MatchesAWSRegex(t *testing.T) {
	id := virtualMembershipID(virtualGroupID("Eng"), virtualUserID("alice@example.com"))
	assert.Regexp(t, awsIDRegex, id)
}

func TestVirtualUserID_Deterministic(t *testing.T) {
	a := virtualUserID("bob@example.com")
	b := virtualUserID("bob@example.com")
	assert.Equal(t, a, b, "same input must produce the same virtual ID")
}

func TestVirtualUserID_Unique(t *testing.T) {
	a := virtualUserID("alice@example.com")
	b := virtualUserID("bob@example.com")
	assert.NotEqual(t, a, b, "different inputs must produce different virtual IDs")
}

func TestIsVirtualID(t *testing.T) {
	assert.True(t, isVirtualID(virtualUserID("x@y.com")))
	assert.True(t, isVirtualID(virtualGroupID("G")))
	assert.False(t, isVirtualID("12345678-1234-1234-1234-123456789012"))
	assert.False(t, isVirtualID(""))
}
