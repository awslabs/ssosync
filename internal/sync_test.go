package internal

import (
	"testing"

	"github.com/awslabs/ssosync/internal/config"
	"github.com/awslabs/ssosync/internal/interfaces"
	"github.com/stretchr/testify/assert"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestIgnoreUserWildcard(t *testing.T) {
	s := &syncGSuite{
		cfg: &config.Config{
			IgnoreUsers: []string{"admin@*", "*@doit.com", "exact-match@example.com"},
		},
	}

	tests := []struct {
		email    string
		expected bool
	}{
		{"admin@example.com", true},
		{"user@doit.com", true},
		{"exact-match@example.com", true},
		{"user@example.com", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, s.ignoreUser(tt.email), tt.email)
	}
}

func TestIgnoreGroupWildcard(t *testing.T) {
	s := &syncGSuite{
		cfg: &config.Config{
			IgnoreGroups: []string{"AWS*", "exact-group"},
		},
	}

	tests := []struct {
		name     string
		expected bool
	}{
		{"AWSAccountFactory", true},
		{"AWSServiceRole", true},
		{"exact-group", true},
		{"OtherGroup", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, s.ignoreGroup(tt.name), tt.name)
	}
}

func TestGetGroupOperationsWithIgnore(t *testing.T) {
	ignoreFn := func(name string) bool {
		return name == "AWSReserved" || name == "ManualGroup"
	}

	awsGroups := []*interfaces.Group{
		{DisplayName: "GroupInBoth"},
		{DisplayName: "AWSReserved"},
		{DisplayName: "ManualGroup"},
		{DisplayName: "DeleteMe"},
	}

	googleGroups := []*admin.Group{
		{Name: "GroupInBoth"},
		{Name: "NewGroup"},
	}

	add, delete, equals := getGroupOperations(awsGroups, googleGroups, ignoreFn)

	assert.Len(t, add, 1)
	assert.Equal(t, "NewGroup", add[0].DisplayName)

	assert.Len(t, delete, 1)
	assert.Equal(t, "DeleteMe", delete[0].DisplayName)

	assert.Len(t, equals, 1)
	assert.Equal(t, "GroupInBoth", equals[0].DisplayName)
}

func TestGetUserOperationsWithIgnore(t *testing.T) {
	ignoreFn := func(name string) bool {
		return name == "ignored@example.com"
	}

	awsUsers := []*interfaces.User{
		{Username: "user@example.com", Active: true, Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{FamilyName: "User", GivenName: "Test"}},
		{Username: "delete-me@example.com"},
		{Username: "ignored@example.com"},
	}

	googleUsers := []*admin.User{
		{PrimaryEmail: "user@example.com", Suspended: false, Name: &admin.UserName{FamilyName: "User", GivenName: "Test"}},
		{PrimaryEmail: "new-user@example.com", Suspended: false, Name: &admin.UserName{FamilyName: "User", GivenName: "New"}},
	}

	add, delete, update, equals := getUserOperations(awsUsers, googleUsers, ignoreFn)

	assert.Len(t, add, 1)
	assert.Equal(t, "new-user@example.com", add[0].Username)

	assert.Len(t, delete, 1)
	assert.Equal(t, "delete-me@example.com", delete[0].Username)

	assert.Len(t, equals, 1)
	assert.Equal(t, "user@example.com", equals[0].Username)

	assert.Len(t, update, 0)
}
