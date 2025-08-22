package google

import (
	"context"
	"testing"

	"github.com/awslabs/ssosync/internal/mocks"
	"github.com/stretchr/testify/assert"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestNewClient_InvalidCredentials(t *testing.T) {
	ctx := context.Background()
	invalidJSON := []byte("invalid json")

	client, err := NewClient(ctx, "admin@example.com", invalidJSON)
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestGetUsers_EmptyQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetUsers("", "").Return([]*admin.User{}, nil)

	users, err := mockClient.GetUsers("", "")
	assert.NoError(t, err)
	assert.Len(t, users, 0)
}

func TestGetUsers_WildcardQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedUsers := []*admin.User{
		{
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
		},
	}

	mockClient.EXPECT().GetUsers("*", "").Return(expectedUsers, nil)

	users, err := mockClient.GetUsers("*", "")
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "user1@example.com", users[0].PrimaryEmail)
}

func TestGetUsers_SpecificQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedUsers := []*admin.User{
		{
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
		},
	}

	mockClient.EXPECT().GetUsers("name:John", "").Return(expectedUsers, nil)

	users, err := mockClient.GetUsers("name:John", "")
	assert.NoError(t, err)
	assert.Len(t, users, 1)
}

func TestGetUsers_MultipleQueries(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedUsers := []*admin.User{
		{
			PrimaryEmail: "user1@example.com",
			Name: &admin.UserName{
				GivenName:  "John",
				FamilyName: "Doe",
			},
		},
		{
			PrimaryEmail: "user2@example.com",
			Name: &admin.UserName{
				GivenName:  "Jane",
				FamilyName: "Smith",
			},
		},
	}

	mockClient.EXPECT().GetUsers("name:John,name:Jane", "").Return(expectedUsers, nil)

	users, err := mockClient.GetUsers("name:John,name:Jane", "")
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestGetUsers_Error(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetUsers("invalid", "").Return(nil, assert.AnError)

	users, err := mockClient.GetUsers("invalid", "")
	assert.Error(t, err)
	assert.Nil(t, users)
}

func TestGetDeletedUsers(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedUsers := []*admin.User{
		{
			PrimaryEmail: "deleted@example.com",
			Name: &admin.UserName{
				GivenName:  "Deleted",
				FamilyName: "User",
			},
		},
	}

	mockClient.EXPECT().GetDeletedUsers().Return(expectedUsers, nil)

	users, err := mockClient.GetDeletedUsers()
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "deleted@example.com", users[0].PrimaryEmail)
}

func TestGetDeletedUsers_Error(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetDeletedUsers().Return(nil, assert.AnError)

	users, err := mockClient.GetDeletedUsers()
	assert.Error(t, err)
	assert.Nil(t, users)
}

func TestGetGroups_EmptyQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetGroups("").Return([]*admin.Group{}, nil)

	groups, err := mockClient.GetGroups("")
	assert.NoError(t, err)
	assert.Len(t, groups, 0)
}

func TestGetGroups_WildcardQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedGroups := []*admin.Group{
		{
			Id:    "group1",
			Email: "group1@example.com",
			Name:  "Group 1",
		},
	}

	mockClient.EXPECT().GetGroups("*").Return(expectedGroups, nil)

	groups, err := mockClient.GetGroups("*")
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "group1@example.com", groups[0].Email)
}

func TestGetGroups_SpecificQuery(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedGroups := []*admin.Group{
		{
			Id:    "group1",
			Email: "group1@example.com",
			Name:  "Group 1",
		},
	}

	mockClient.EXPECT().GetGroups("name:Group").Return(expectedGroups, nil)

	groups, err := mockClient.GetGroups("name:Group")
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
}

func TestGetGroups_MultipleQueries(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	expectedGroups := []*admin.Group{
		{
			Id:    "group1",
			Email: "group1@example.com",
			Name:  "Group 1",
		},
		{
			Id:    "group2",
			Email: "group2@example.com",
			Name:  "Group 2",
		},
	}

	mockClient.EXPECT().GetGroups("name:Group1,name:Group2").Return(expectedGroups, nil)

	groups, err := mockClient.GetGroups("name:Group1,name:Group2")
	assert.NoError(t, err)
	assert.Len(t, groups, 2)
}

func TestGetGroups_Error(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	mockClient.EXPECT().GetGroups("name:NonExistent").Return(nil, assert.AnError)

	groups, err := mockClient.GetGroups("name:NonExistent")
	assert.Error(t, err)
	assert.Nil(t, groups)
}

func TestGetGroupMembers(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	group := &admin.Group{
		Id:    "group1",
		Email: "group1@example.com",
		Name:  "Group 1",
	}

	expectedMembers := []*admin.Member{
		{
			Email: "member1@example.com",
			Type:  "USER",
		},
		{
			Email: "member2@example.com",
			Type:  "USER",
		},
	}

	mockClient.EXPECT().GetGroupMembers(group).Return(expectedMembers, nil)

	members, err := mockClient.GetGroupMembers(group)
	assert.NoError(t, err)
	assert.Len(t, members, 2)
	assert.Equal(t, "member1@example.com", members[0].Email)
	assert.Equal(t, "member2@example.com", members[1].Email)
}

func TestGetGroupMembers_NoMembers(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	group := &admin.Group{
		Id:    "group2",
		Email: "group2@example.com",
		Name:  "Group 2",
	}

	mockClient.EXPECT().GetGroupMembers(group).Return([]*admin.Member{}, nil)

	members, err := mockClient.GetGroupMembers(group)
	assert.NoError(t, err)
	assert.Empty(t, members)
}

func TestGetGroupMembers_Error(t *testing.T) {
	mockClient := mocks.NewMockGoogleClient(t)

	group := &admin.Group{
		Id:    "group3",
		Email: "group3@example.com",
		Name:  "Group 3",
	}

	mockClient.EXPECT().GetGroupMembers(group).Return(nil, assert.AnError)

	members, err := mockClient.GetGroupMembers(group)
	assert.Error(t, err)
	assert.Nil(t, members)
}
