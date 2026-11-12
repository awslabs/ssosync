package internal

import (
	"encoding/json"
	"testing"

	"ssosync/internal/config"
	"ssosync/internal/interfaces"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestGetUserOperations_OptInNames(t *testing.T) {
	attrMap, err := config.ParseAttributeMaps([]byte(`{"user":{
		"Username":"primaryEmail", "Active":"suspended",
		"Name.GivenName":"name.givenName", "Name.FamilyName":"name.familyName",
		"NickName":"name.givenName", "Name.Formatted":"name.fullName"
	}}`))
	require.NoError(t, err)
	googleUsers := []*admin.User{{
		Id: "G1", PrimaryEmail: "jane@example.com",
		Name: &admin.UserName{GivenName: "Jane", FamilyName: "Doe", FullName: "Dr. Jane Q. Doe"},
	}}
	current := &interfaces.User{
		ID: "A1", Username: "jane@example.com", Active: true,
		ExternalId: interfaces.ExternalId{Id: "G1"},
		Name:       interfaces.UserName{GivenName: "Jane", FamilyName: "Doe"}, DisplayName: "Jane Doe",
	}
	add, del, update, equal := getUserOperations([]*interfaces.User{current}, googleUsers, opsConfig(false, false, ""), attrMap.User)
	require.Len(t, update, 1)
	assert.Empty(t, add)
	assert.Empty(t, del)
	assert.Empty(t, equal)
	assert.Equal(t, "A1", update[0].ID)
	assert.Equal(t, "Jane", update[0].NickName)
	assert.Equal(t, "Dr. Jane Q. Doe", update[0].Name.Formatted)
	assert.Equal(t, "Jane Doe", update[0].DisplayName)
	payload, err := json.Marshal(update[0])
	require.NoError(t, err)
	var wire map[string]interface{}
	require.NoError(t, json.Unmarshal(payload, &wire))
	assert.Equal(t, "Jane", wire["nickName"])
	assert.Equal(t, "Dr. Jane Q. Doe", wire["name"].(map[string]interface{})["formatted"])

	add, del, changes, equal := getUserOperations(update, googleUsers, opsConfig(false, false, ""), attrMap.User)
	assert.Empty(t, add)
	assert.Empty(t, del)
	assert.Empty(t, changes)
	assert.Len(t, equal, 1)

	for _, field := range []string{"nickname", "formatted name"} {
		t.Run(field+" changed", func(t *testing.T) {
			stale := *update[0]
			if field == "nickname" {
				stale.NickName = "old nickname"
			} else {
				stale.Name.Formatted = "old formatted name"
			}
			_, _, changes, equal := getUserOperations([]*interfaces.User{&stale}, googleUsers, opsConfig(false, false, ""), attrMap.User)
			require.Len(t, changes, 1)
			assert.Empty(t, equal)
			assert.Equal(t, "Jane", changes[0].NickName)
			assert.Equal(t, "Dr. Jane Q. Doe", changes[0].Name.Formatted)
		})
	}

	_, _, changes, _ = getUserOperations(update, googleUsers, opsConfig(false, false, ""), config.DefaultUserAttributeMap)
	require.Len(t, changes, 1)
	assert.Empty(t, changes[0].NickName)
	assert.Empty(t, changes[0].Name.Formatted)
}
