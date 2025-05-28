// Copyright (c) 2020, Amazon.com, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package internal ...
package internal

import (
	"encoding/json"
	"errors"
	"log"
	"reflect"
	"strconv"
	"testing"

	aws_sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/awslabs/ssosync/internal/aws"
	"github.com/awslabs/ssosync/internal/config"
	"github.com/awslabs/ssosync/internal/interfaces"
	mock_interfaces "github.com/awslabs/ssosync/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	admin "google.golang.org/api/admin/directory/v1"
)

// toJSON return a json pretty of the stc
func toJSON(stc interface{}) []byte {
	JSON, err := json.MarshalIndent(stc, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return JSON
}

func Test_getGroupOperations(t *testing.T) {
	type args struct {
		awsGroups    []*interfaces.Group
		googleGroups []*admin.Group
	}
	tests := []struct {
		name       string
		args       args
		wantAdd    []*interfaces.Group
		wantDelete []*interfaces.Group
		wantEquals []*interfaces.Group
	}{
		{
			name: "equal groups google and aws",
			args: args{
				awsGroups: []*interfaces.Group{
					aws.NewGroup("Group-1"),
					aws.NewGroup("Group-2"),
				},
				googleGroups: []*admin.Group{
					{Name: "Group-1"},
					{Name: "Group-2"},
				},
			},
			wantAdd:    nil,
			wantDelete: nil,
			wantEquals: []*interfaces.Group{
				aws.NewGroup("Group-1"),
				aws.NewGroup("Group-2"),
			},
		},
		{
			name: "add two new aws groups",
			args: args{
				awsGroups: nil,
				googleGroups: []*admin.Group{
					{Name: "Group-1"},
					{Name: "Group-2"},
				},
			},
			wantAdd: []*interfaces.Group{
				aws.NewGroup("Group-1"),
				aws.NewGroup("Group-2"),
			},
			wantDelete: nil,
			wantEquals: nil,
		},
		{
			name: "delete two aws groups",
			args: args{
				awsGroups: []*interfaces.Group{
					aws.NewGroup("Group-1"),
					aws.NewGroup("Group-2"),
				}, googleGroups: nil,
			},
			wantAdd: nil,
			wantDelete: []*interfaces.Group{
				aws.NewGroup("Group-1"),
				aws.NewGroup("Group-2"),
			},
			wantEquals: nil,
		},
		{
			name: "add one, delete one and one equal",
			args: args{
				awsGroups: []*interfaces.Group{
					aws.NewGroup("Group-2"),
					aws.NewGroup("Group-3"),
				},
				googleGroups: []*admin.Group{
					{Name: "Group-1"},
					{Name: "Group-2"},
				},
			},
			wantAdd: []*interfaces.Group{
				aws.NewGroup("Group-1"),
			},
			wantDelete: []*interfaces.Group{
				aws.NewGroup("Group-3"),
			},
			wantEquals: []*interfaces.Group{
				aws.NewGroup("Group-2"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAdd, gotDelete, gotEquals := getGroupOperations(tt.args.awsGroups, tt.args.googleGroups)
			if !reflect.DeepEqual(gotAdd, tt.wantAdd) {
				t.Errorf("getGroupOperations() gotAdd = %s, want %s", toJSON(gotAdd), toJSON(tt.wantAdd))
			}
			if !reflect.DeepEqual(gotDelete, tt.wantDelete) {
				t.Errorf("getGroupOperations() gotDelete = %s, want %s", toJSON(gotDelete), toJSON(tt.wantDelete))
			}
			if !reflect.DeepEqual(gotEquals, tt.wantEquals) {
				t.Errorf("getGroupOperations() gotEquals = %s, want %s", toJSON(gotEquals), toJSON(tt.wantEquals))
			}
		})
	}
}

func Test_getUserOperations(t *testing.T) {
	type args struct {
		awsUsers    []*interfaces.User
		googleUsers []*admin.User
	}
	tests := []struct {
		name       string
		args       args
		wantAdd    []*interfaces.User
		wantDelete []*interfaces.User
		wantUpdate []*interfaces.User
		wantEquals []*interfaces.User
	}{
		{
			name: "equal user google and aws",
			args: args{
				awsUsers: []*interfaces.User{
					aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
					aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
				},
				googleUsers: []*admin.User{
					{Name: &admin.UserName{
						GivenName:  "name-1",
						FamilyName: "lastname-1",
					},
						Suspended:    false,
						PrimaryEmail: "user-1@email.com",
					},
					{Name: &admin.UserName{
						GivenName:  "name-2",
						FamilyName: "lastname-2",
					},
						Suspended:    false,
						PrimaryEmail: "user-2@email.com",
					},
				},
			},
			wantAdd:    nil,
			wantDelete: nil,
			wantUpdate: nil,
			wantEquals: []*interfaces.User{
				aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
				aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
			},
		},
		{
			name: "add two new aws users",
			args: args{
				awsUsers: nil,
				googleUsers: []*admin.User{
					{Name: &admin.UserName{
						GivenName:  "name-1",
						FamilyName: "lastname-1",
					},
						Suspended:    false,
						PrimaryEmail: "user-1@email.com",
					},
					{Name: &admin.UserName{
						GivenName:  "name-2",
						FamilyName: "lastname-2",
					},
						Suspended:    false,
						PrimaryEmail: "user-2@email.com",
					},
				},
			},
			wantAdd: []*interfaces.User{
				aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
				aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
			},
			wantDelete: nil,
			wantUpdate: nil,
			wantEquals: nil,
		},
		{
			name: "delete two aws users",
			args: args{
				awsUsers: []*interfaces.User{
					aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
					aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
				},
				googleUsers: nil,
			},
			wantAdd: nil,
			wantDelete: []*interfaces.User{
				aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
				aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
			},
			wantUpdate: nil,
			wantEquals: nil,
		},
		{
			name: "add on, delete one, update one and one equal",
			args: args{
				awsUsers: []*interfaces.User{
					aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
					aws.NewUser("name-3", "lastname-3", "user-3@email.com", true),
					aws.NewUser("name-4", "lastname-4", "user-4@email.com", true),
				},
				googleUsers: []*admin.User{
					{
						Name: &admin.UserName{
							GivenName:  "name-1",
							FamilyName: "lastname-1",
						},
						Suspended:    false,
						PrimaryEmail: "user-1@email.com",
					},
					{
						Name: &admin.UserName{
							GivenName:  "name-2",
							FamilyName: "lastname-2",
						},
						Suspended:    false,
						PrimaryEmail: "user-2@email.com",
					},
					{
						Name: &admin.UserName{
							GivenName:  "name-4",
							FamilyName: "lastname-4",
						},
						Suspended:    true,
						PrimaryEmail: "user-4@email.com",
					},
				},
			},
			wantAdd: []*interfaces.User{
				aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
			},
			wantDelete: []*interfaces.User{
				aws.NewUser("name-3", "lastname-3", "user-3@email.com", true),
			},
			wantUpdate: []*interfaces.User{
				aws.NewUser("name-4", "lastname-4", "user-4@email.com", false),
			},
			wantEquals: []*interfaces.User{
				aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAdd, gotDelete, gotUpdate, gotEquals := getUserOperations(tt.args.awsUsers, tt.args.googleUsers)
			if !reflect.DeepEqual(gotAdd, tt.wantAdd) {
				t.Errorf("getUserOperations() gotAdd = %s, want %s", toJSON(gotAdd), toJSON(tt.wantAdd))
			}
			if !reflect.DeepEqual(gotDelete, tt.wantDelete) {
				t.Errorf("getUserOperations() gotDelete = %s, want %s", toJSON(gotDelete), toJSON(tt.wantDelete))
			}
			if !reflect.DeepEqual(gotUpdate, tt.wantUpdate) {
				t.Errorf("getUserOperations() gotUpdate = %s, want %s", toJSON(gotUpdate), toJSON(tt.wantUpdate))
			}
			if !reflect.DeepEqual(gotEquals, tt.wantEquals) {
				t.Errorf("getUserOperations() gotEquals = %s, want %s", toJSON(gotEquals), toJSON(tt.wantEquals))
			}
		})
	}
}

func Test_getGroupUsersOperations(t *testing.T) {
	type args struct {
		gGroupsUsers   map[string][]*admin.User
		awsGroupsUsers map[string][]*interfaces.User
	}
	tests := []struct {
		name       string
		args       args
		wantDelete map[string][]*interfaces.User
		wantEquals map[string][]*interfaces.User
	}{
		{
			name: "one add, one delete, one equal",
			args: args{
				gGroupsUsers: map[string][]*admin.User{
					"group-1": {
						{
							Name: &admin.UserName{
								GivenName:  "name-1",
								FamilyName: "lastname-1",
							},
							Suspended:    false,
							PrimaryEmail: "user-1@email.com",
						},
					},
				},
				awsGroupsUsers: map[string][]*interfaces.User{
					"group-1": {
						aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
						aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
					},
				},
			},
			wantDelete: map[string][]*interfaces.User{
				"group-1": {
					aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
				},
			},
			wantEquals: map[string][]*interfaces.User{
				"group-1": {
					aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDelete, gotEquals := getGroupUsersOperations(tt.args.gGroupsUsers, tt.args.awsGroupsUsers)
			if !reflect.DeepEqual(gotDelete, tt.wantDelete) {
				t.Errorf("getGroupUsersOperations() gotDelete = %s, want %s", toJSON(gotDelete), toJSON(tt.wantDelete))
			}
			if !reflect.DeepEqual(gotEquals, tt.wantEquals) {
				t.Errorf("getGroupUsersOperations() gotEquals = %s, want %s", toJSON(gotEquals), toJSON(tt.wantEquals))
			}
		})
	}
}

func Test_GetGroupsWithoutPagination(t *testing.T) {

	paginatedMocks := createPaginatedMocks(t)
	// paginator := mock_interfaces.NewMockListGroupsPaginator(t)

	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: paginatedMocks.mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	// >1 group response with no pagination (<100 groups returned)
	sampleResponseNoPagination := &identitystore.ListGroupsOutput{
		Groups: []types.Group{
			{GroupId: aws_sdk.String("group-1-test-id"), DisplayName: aws_sdk.String("group-1-test-displayname")},
			{GroupId: aws_sdk.String("group-2-test-id"), DisplayName: aws_sdk.String("group-2-test-displayname")},
			{GroupId: aws_sdk.String("group-3-test-id"), DisplayName: aws_sdk.String("group-3-test-displayname")},
			{GroupId: aws_sdk.String("group-4-test-id"), DisplayName: aws_sdk.String("group-4-test-displayname")},
		},
	}

	expectedOutput := []*interfaces.Group{
		{ID: "group-1-test-id", Schemas: []string{"urn:ietf:params:scim:schemas:core:2.0:Group"}, DisplayName: "group-1-test-displayname", Members: []string{}},
		{ID: "group-2-test-id", Schemas: []string{"urn:ietf:params:scim:schemas:core:2.0:Group"}, DisplayName: "group-2-test-displayname", Members: []string{}},
		{ID: "group-3-test-id", Schemas: []string{"urn:ietf:params:scim:schemas:core:2.0:Group"}, DisplayName: "group-3-test-displayname", Members: []string{}},
		{ID: "group-4-test-id", Schemas: []string{"urn:ietf:params:scim:schemas:core:2.0:Group"}, DisplayName: "group-4-test-displayname", Members: []string{}},
	}
	// Create a variable of the interface type that the mock implements
	// var paginatorInterface interfaces.ListGroupsPaginator = paginator

	// paginatedMocks.mockIdentityStorePaginators.EXPECT().NewListGroupsPaginator(
	// 	paginatedMocks.mockIdentityStoreClient,
	// 	mock.Anything,
	// ).Return(&paginatorInterface)

	// paginator.EXPECT().NextPage(mock.Anything).Return(sampleResponseNoPagination, nil)
	// Set up ListGroupsPager expectation
	// paginatedMocks.mockIdentityStorePaginatedAPI.EXPECT().ListGroupsPager(
	// 	mock.Anything,
	// 	paginator,
	// 	mock.AnythingOfType("func(types.Group) *interfaces.Group"),
	// )
	paginatedMocks.mockIdentityStoreClient.EXPECT().ListGroups(mock.Anything,
		mock.Anything,
		// {
		// 	IdentityStoreId: aws_sdk.String("test-identity-store-id"),
		// },
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(sampleResponseNoPagination, nil)

	actualOutput, err := mockClient.GetGroups()
	// will call identitystore.ListGroupsPager
	// and iterate with aws_identitystore.NewListGroupsPaginator

	assert.True(t, reflect.DeepEqual(expectedOutput, actualOutput))
	assert.NoError(t, err)
}

func Test_GetGroupsWithPagination(t *testing.T) {

	paginatedMocks := createPaginatedMocks(t)

	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: paginatedMocks.mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	// >1 group response with pagination (150 groups returned)
	sampleGroupsResponseA := make([]types.Group, 0, 100)
	sampleGroupsResponseB := make([]types.Group, 0, 50)
	expectedOutput := make([]*interfaces.Group, 0, 150)

	// Populate responses
	for i := range 150 {
		grp := types.Group{
			GroupId:     aws_sdk.String(strconv.Itoa(i)),
			DisplayName: aws_sdk.String(strconv.Itoa(i)),
		}
		grpNative := ConvertIdentityStoreGroupToAWSGroup(grp)

		expectedOutput = append(expectedOutput, grpNative)

		if i < 100 {
			sampleGroupsResponseA = append(sampleGroupsResponseA, grp)
		} else {
			sampleGroupsResponseB = append(sampleGroupsResponseB, grp)
		}
	}

	sampleResponsePaginationA := &identitystore.ListGroupsOutput{
		Groups:    sampleGroupsResponseA,
		NextToken: aws_sdk.String("next-token"),
	}

	sampleResponsePaginationB := &identitystore.ListGroupsOutput{
		Groups: sampleGroupsResponseB,
	}

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListGroups(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(sampleResponsePaginationA, nil)
	paginatedMocks.mockIdentityStoreClient.EXPECT().ListGroups(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(sampleResponsePaginationB, nil)

	actualOutput, err := mockClient.GetGroups()
	assert.True(t, reflect.DeepEqual(expectedOutput, actualOutput))
	assert.NoError(t, err)
}

func Test_GetGroupsEmptyResponse(t *testing.T) {

	paginatedMocks := createPaginatedMocks(t)

	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: paginatedMocks.mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	expectedOutput := []*interfaces.Group{}

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListGroups(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(&identitystore.ListGroupsOutput{Groups: []types.Group{}}, nil)

	actualOutput, err := mockClient.GetGroups()

	assert.True(t, reflect.DeepEqual(expectedOutput, actualOutput))
	assert.NoError(t, err)
}

func Test_GetGroupsErrorResponse(t *testing.T) {

	paginatedMocks := createPaginatedMocks(t)

	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: paginatedMocks.mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	sampleResponseError := errors.New("Sample error")

	expectedOutput := errors.New("Sample error")

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListGroups(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(nil, sampleResponseError)

	actualOutput, err := mockClient.GetGroups()

	assert.True(t, reflect.DeepEqual(expectedOutput.Error(), err.Error()))
	assert.Nil(t, actualOutput)
}

func Test_GetUsersWithoutPagination(t *testing.T) {

	paginatedMocks := createPaginatedMocks(t)

	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: paginatedMocks.mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	// >1 user response with no pagination (<100 users returned)
	sampleResponseNoPagination := &identitystore.ListUsersOutput{
		Users: []types.User{
			{
				UserId:      aws_sdk.String("user-1-test-id"),
				UserName:    aws_sdk.String("user-1@example.com"),
				Title:       aws_sdk.String("Example title"),
				Name:        &types.Name{FamilyName: aws_sdk.String("1"), GivenName: aws_sdk.String("User")},
				DisplayName: aws_sdk.String("User 1"),
				Addresses:   []types.Address{{Type: aws_sdk.String("Home"), Country: aws_sdk.String("Canada")}},
				Emails:      []types.Email{{Primary: true, Type: aws_sdk.String("work"), Value: aws_sdk.String("user-1@example.com")}},
			},
			{
				UserId:      aws_sdk.String("user-2-test-id"),
				UserName:    aws_sdk.String("user-2@example.com"),
				Name:        &types.Name{FamilyName: aws_sdk.String("2"), GivenName: aws_sdk.String("User")},
				DisplayName: aws_sdk.String("User 2"),
				Addresses:   []types.Address{{Type: aws_sdk.String("Work")}, {Type: aws_sdk.String("Home")}},
				Emails: []types.Email{
					{Primary: true, Type: aws_sdk.String("work"), Value: aws_sdk.String("user-2@example.com")},
					{Primary: false, Type: aws_sdk.String("personal"), Value: aws_sdk.String("user-2-personal@example.com")},
				},
			},
		},
	}

	expectedOutput := []*interfaces.User{
		{
			ID:       "user-1-test-id",
			Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
			Username: "user-1@example.com",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
				FamilyName: "1",
				GivenName:  "User",
			},
			DisplayName: "User 1",
			Emails: []interfaces.UserEmail{
				{Primary: true, Type: "work", Value: "user-1@example.com"},
			},
			Addresses: []interfaces.UserAddress{{Type: "Home"}},
		},
		{
			ID:       "user-2-test-id",
			Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
			Username: "user-2@example.com",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
				FamilyName: "2",
				GivenName:  "User",
			},
			DisplayName: "User 2",
			Emails: []interfaces.UserEmail{
				{Primary: true, Type: "work", Value: "user-2@example.com"},
				{Primary: false, Type: "personal", Value: "user-2-personal@example.com"},
			},
			Addresses: []interfaces.UserAddress{{Type: "Work"}, {Type: "Home"}},
		},
	}

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListUsers(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(sampleResponseNoPagination, nil)

	actualOutput, err := mockClient.GetUsers()

	assert.True(t, reflect.DeepEqual(expectedOutput, actualOutput))
	assert.NoError(t, err)
}

func Test_GetUsersWithPagination(t *testing.T) {

	paginatedMocks := createPaginatedMocks(t)
	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: paginatedMocks.mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	// >1 user response with pagination (150 users returned)
	sampleUsersResponseA := make([]types.User, 0, 100)
	sampleUsersResponseB := make([]types.User, 0, 50)
	expectedOutput := make([]*interfaces.User, 0, 150)

	// Populate responses
	for i := range 150 {
		usr := types.User{
			UserId:      aws_sdk.String(strconv.Itoa(i)),
			UserName:    aws_sdk.String(strconv.Itoa(i)),
			DisplayName: aws_sdk.String("User " + strconv.Itoa(i)),
			Name:        &types.Name{FamilyName: aws_sdk.String(strconv.Itoa(i)), GivenName: aws_sdk.String("User")},
			Emails:      []types.Email{{Primary: true, Type: aws_sdk.String("work"), Value: aws_sdk.String(strconv.Itoa(i) + "@example.com")}},
			Addresses:   []types.Address{{Type: aws_sdk.String("Home"), Primary: true}},
		}
		usrNative := interfaces.User{
			ID:       strconv.Itoa(i),
			Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
			Username: strconv.Itoa(i),
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
				FamilyName: strconv.Itoa(i),
				GivenName:  "User",
			},
			DisplayName: "User " + strconv.Itoa(i),
			Emails:      []interfaces.UserEmail{{Primary: true, Type: "work", Value: strconv.Itoa(i) + "@example.com"}},
			Addresses:   []interfaces.UserAddress{{Type: "Home"}},
		}

		expectedOutput = append(expectedOutput, &usrNative)

		if i < 100 {
			sampleUsersResponseA = append(sampleUsersResponseA, usr)
		} else {
			sampleUsersResponseB = append(sampleUsersResponseB, usr)
		}
	}

	sampleResponsePaginationA := &identitystore.ListUsersOutput{
		Users:     sampleUsersResponseA,
		NextToken: aws_sdk.String("sample NextToken"),
	}

	sampleResponsePaginationB := &identitystore.ListUsersOutput{
		Users: sampleUsersResponseB,
	}

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListUsers(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(sampleResponsePaginationA, nil)

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListUsers(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(sampleResponsePaginationB, nil)
	actualOutput, err := mockClient.GetUsers()

	assert.True(t, reflect.DeepEqual(expectedOutput, actualOutput))
	assert.NoError(t, err)
}

type paginatedMocks struct {
	mockIdentityStoreClient       *mock_interfaces.MockIdentityStoreAPI
	mockIdentityStorePaginators   *mock_interfaces.MockIdentityStorePaginators
	mockIdentityStorePaginatedAPI *mock_interfaces.MockIdentityStorePaginatedAPI
}

func createPaginatedMocks(t *testing.T) paginatedMocks {
	mockIdentityStoreClient := mock_interfaces.NewMockIdentityStoreAPI(t)
	mockIdentityStorePaginators := mock_interfaces.NewMockIdentityStorePaginators(t)
	mockIdentityStorePaginatedAPI := mock_interfaces.NewMockIdentityStorePaginatedAPI(t)

	return paginatedMocks{
		mockIdentityStoreClient:       mockIdentityStoreClient,
		mockIdentityStorePaginators:   mockIdentityStorePaginators,
		mockIdentityStorePaginatedAPI: mockIdentityStorePaginatedAPI,
	}
}

func Test_GetUsersEmptyResponse(t *testing.T) {
	paginatedMocks := createPaginatedMocks(t)

	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: paginatedMocks.mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	expectedOutput := []*interfaces.User{}

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListUsers(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(&identitystore.ListUsersOutput{}, nil)

	actualOutput, err := mockClient.GetUsers()

	assert.True(t, reflect.DeepEqual(expectedOutput, actualOutput))
	assert.NoError(t, err)
}

func Test_GetUsersErrorResponse(t *testing.T) {
	paginatedMocks := createPaginatedMocks(t)

	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: paginatedMocks.mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	sampleResponseError := errors.New("Sample error")

	expectedOutput := errors.New("Sample error")

	//handling returns
	paginatedMocks.mockIdentityStoreClient.EXPECT().ListUsers(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(nil, sampleResponseError)

	actualOutput, err := mockClient.GetUsers()

	assert.True(t, reflect.DeepEqual(expectedOutput.Error(), err.Error()))
	assert.Nil(t, actualOutput)
}

func Test_ConvertSdkUserObjToNative(t *testing.T) {

	// >1 user response with no pagination (<100 users returned)
	sampleInput := types.User{
		UserId:      aws_sdk.String("user-1-test-id"),
		UserName:    aws_sdk.String("user-1@example.com"),
		Title:       aws_sdk.String("Example title"),
		Name:        &types.Name{FamilyName: aws_sdk.String("1"), GivenName: aws_sdk.String("User")},
		DisplayName: aws_sdk.String("User 1"),
		Addresses:   []types.Address{{Type: aws_sdk.String("Home"), Country: aws_sdk.String("Canada")}},
		Emails:      []types.Email{{Primary: true, Type: aws_sdk.String("work"), Value: aws_sdk.String("user-1@example.com")}},
	}

	expectedOutput := &interfaces.User{
		ID:       "user-1-test-id",
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Username: "user-1@example.com",
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{
			FamilyName: "1",
			GivenName:  "User",
		},
		DisplayName: "User 1",
		Emails: []interfaces.UserEmail{
			{Primary: true, Type: "work", Value: "user-1@example.com"},
		},
		Addresses: []interfaces.UserAddress{{Type: "Home"}},
	}

	actualOutput := ConvertSdkUserObjToNative(sampleInput)

	assert.True(t, reflect.DeepEqual(expectedOutput, actualOutput))
}

func Test_CreateUserIDtoUserObjMap(t *testing.T) {

	sampleInput := []*interfaces.User{
		{
			ID:       "user-1-test-id",
			Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
			Username: "user-1@example.com",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
				FamilyName: "1",
				GivenName:  "User",
			},
			DisplayName: "User 1",
			Emails: []interfaces.UserEmail{
				{Primary: true, Type: "work", Value: "user-1@example.com"},
			},
			Addresses: []interfaces.UserAddress{{Type: "Home"}},
		},
		{
			ID:       "user-2-test-id",
			Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
			Username: "user-2@example.com",
			Name: struct {
				FamilyName string `json:"familyName"`
				GivenName  string `json:"givenName"`
			}{
				FamilyName: "2",
				GivenName:  "User",
			},
			DisplayName: "User 2",
			Emails: []interfaces.UserEmail{
				{Primary: true, Type: "work", Value: "user-2@example.com"},
				{Primary: false, Type: "personal", Value: "user-2-personal@example.com"},
			},
			Addresses: []interfaces.UserAddress{{Type: "Work"}, {Type: "Home"}},
		},
	}

	expectedOutput := make(map[string]*interfaces.User)

	expectedOutput["user-1-test-id"] = &interfaces.User{
		ID:       "user-1-test-id",
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Username: "user-1@example.com",
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{
			FamilyName: "1",
			GivenName:  "User",
		},
		DisplayName: "User 1",
		Emails: []interfaces.UserEmail{
			{Primary: true, Type: "work", Value: "user-1@example.com"},
		},
		Addresses: []interfaces.UserAddress{{Type: "Home"}},
	}

	expectedOutput["user-2-test-id"] = &interfaces.User{
		ID:       "user-2-test-id",
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Username: "user-2@example.com",
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{
			FamilyName: "2",
			GivenName:  "User",
		},
		DisplayName: "User 2",
		Emails: []interfaces.UserEmail{
			{Primary: true, Type: "work", Value: "user-2@example.com"},
			{Primary: false, Type: "personal", Value: "user-2-personal@example.com"},
		},
		Addresses: []interfaces.UserAddress{{Type: "Work"}, {Type: "Home"}},
	}

	actualOutput := CreateUserIDtoUserObjMap(sampleInput)

	assert.True(t, reflect.DeepEqual(expectedOutput, actualOutput))
}

func Test_GetGroupMembershipsLists(t *testing.T) {

	paginatedMocks := createPaginatedMocks(t)

	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: paginatedMocks.mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	sampleGroupsInput := []*interfaces.Group{
		{ID: "a", DisplayName: "a"},
		{ID: "b", DisplayName: "b"},
		{ID: "c", DisplayName: "c"},
	}

	sampleUsersMapInput := make(map[string]*interfaces.User)
	sampleUsersMapInput["1"] = &interfaces.User{ID: "1"}
	sampleUsersMapInput["2"] = &interfaces.User{ID: "2"}
	sampleUsersMapInput["3"] = &interfaces.User{ID: "3"}
	sampleUsersMapInput["4"] = &interfaces.User{ID: "4"}

	expectedOutput := make(map[string][]*interfaces.User)
	expectedOutput["a"] = []*interfaces.User{{ID: "1"}, {ID: "2"}}
	expectedOutput["b"] = []*interfaces.User{{ID: "2"}, {ID: "3"}, {ID: "4"}}
	expectedOutput["c"] = []*interfaces.User{}

	sampleResponseGroupA := &identitystore.ListGroupMembershipsOutput{
		GroupMemberships: []types.GroupMembership{
			{GroupId: aws_sdk.String("a"), MemberId: &types.MemberIdMemberUserId{Value: *aws_sdk.String("1")}},
			{GroupId: aws_sdk.String("a"), MemberId: &types.MemberIdMemberUserId{Value: *aws_sdk.String("2")}},
		},
	}

	sampleResponseGroupB := &identitystore.ListGroupMembershipsOutput{
		GroupMemberships: []types.GroupMembership{
			{GroupId: aws_sdk.String("b"), MemberId: &types.MemberIdMemberUserId{Value: *aws_sdk.String("2")}},
			{GroupId: aws_sdk.String("b"), MemberId: &types.MemberIdMemberUserId{Value: *aws_sdk.String("3")}},
			{GroupId: aws_sdk.String("b"), MemberId: &types.MemberIdMemberUserId{Value: *aws_sdk.String("4")}},
		},
	}

	sampleResponseGroupC := &identitystore.ListGroupMembershipsOutput{
		GroupMemberships: []types.GroupMembership{},
	}

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListGroupMemberships(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(sampleResponseGroupA, nil)

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListGroupMemberships(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(sampleResponseGroupB, nil)

	paginatedMocks.mockIdentityStoreClient.EXPECT().ListGroupMemberships(mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]func(*identitystore.Options)"),
	).Once().Return(sampleResponseGroupC, nil)

	actualOutput, err := mockClient.GetGroupMembershipsLists(sampleGroupsInput, sampleUsersMapInput)

	assert.True(t, reflect.DeepEqual(expectedOutput, actualOutput))
	assert.NoError(t, err)
}

func Test_RemoveUserFromGroup(t *testing.T) {
	mockIdentityStoreClient := mock_interfaces.NewMockIdentityStoreAPI(t)

	mockClient := &syncGSuite{
		aws:           nil,
		google:        nil,
		cfg:           &config.Config{IdentityStoreID: "test-identity-store-id"},
		identityStore: mockIdentityStoreClient,
		users:         make(map[string]*interfaces.User),
	}

	sampleUserInput := "test-user-id"
	sampleGroupInput := "test-group-id"

	sampleResponse := &identitystore.GetGroupMembershipIdOutput{
		MembershipId: aws_sdk.String("test-membership-id"),
	}

	mockIdentityStoreClient.EXPECT().GetGroupMembershipId(mock.Anything, mock.Anything).Times(1).Return(sampleResponse, nil)
	mockIdentityStoreClient.EXPECT().DeleteGroupMembership(
		mock.Anything,
		&identitystore.DeleteGroupMembershipInput{
			IdentityStoreId: &mockClient.cfg.IdentityStoreID,
			MembershipId:    sampleResponse.MembershipId,
		},
	).Times(1).Return(&identitystore.DeleteGroupMembershipOutput{}, nil)

	err := mockClient.RemoveUserFromGroup(&sampleUserInput, &sampleGroupInput)

	assert.NoError(t, err)
}
