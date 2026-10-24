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

package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"ssosync/internal/config"
	"ssosync/internal/constants"
	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// stubHTTPClient returns a fixed 200 response with the given JSON body.
type stubHTTPClient struct{ body string }

func (c stubHTTPClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Body:       io.NopCloser(strings.NewReader(c.body)),
	}, nil
}

// sdkUsersFromListUsersJSON decodes a ListUsers response body through the real
// SDK deserializer, so Extensions hold genuinely-decoded documents (a
// document.NewLazyDocument value cannot be unmarshalled and is not usable).
func sdkUsersFromListUsersJSON(t *testing.T, body string) []types.User {
	t.Helper()
	client := identitystore.New(identitystore.Options{
		Region:      "us-east-1",
		Credentials: awssdk.AnonymousCredentials{},
		HTTPClient:  stubHTTPClient{body: body},
	})
	out, err := client.ListUsers(context.Background(), &identitystore.ListUsersInput{
		IdentityStoreId: awssdk.String("d-1"),
	})
	require.NoError(t, err)
	return out.Users
}

// fullGoogleSideUser is a mapped Google user with EVERY field non-zero. The
// drift guard below fails if a new interfaces.User field is left zero here.
func fullGoogleSideUser() *interfaces.User {
	return &interfaces.User{
		ID:         "is-1",
		Schemas:    []string{constants.SCIMSchemaUser},
		ExternalId: interfaces.ExternalId{Id: "gid-1", Issuer: "issuer-1"},
		Username:   "jane@example.com",
		Name: interfaces.UserName{
			FamilyName:      "Doe",
			GivenName:       "Jane",
			Formatted:       "Dr Jane Q Doe PhD",
			MiddleName:      "Q",
			HonorificPrefix: "Dr",
			HonorificSuffix: "PhD",
		},
		DisplayName: "Jane Doe",
		Active:      true,
		Emails:      []interfaces.UserEmail{{Value: "alt@example.com", Type: "home", Primary: true}},
		Addresses: []interfaces.UserAddress{{
			Type:          "work",
			Formatted:     "1 Main St, Springfield, IL 62701, US",
			StreetAddress: "1 Main St",
			Locality:      "Springfield",
			Region:        "IL",
			PostalCode:    "62701",
			Country:       "US",
			Primary:       true,
		}},
		NickName:          "JJ",
		ProfileURL:        "https://example.com/jane",
		Title:             "Engineer",
		UserType:          "Employee",
		PreferredLanguage: "en-GB",
		Locale:            "en_GB",
		Timezone:          "Europe/London",
		PhoneNumbers:      []interfaces.UserPhoneNumber{{Value: "+44 20 7946 0000", Type: "work", Primary: true}},
		Roles:             []interfaces.UserRole{{Value: "admin", Type: "work", Primary: true}},
		Enterprise: &interfaces.EnterpriseUser{
			EmployeeNumber: "E1",
			CostCenter:     "CC1",
			Organization:   "Org",
			Division:       "Div",
			Department:     "Dept",
			Manager:        &interfaces.EnterpriseManager{Value: "mgr-1"},
		},
	}
}

// fullListUsersJSON is the Identity Store ListUsers response for the same user
// as fullGoogleSideUser, with the enterprise extension in its documented shape.
const fullListUsersJSON = `{"Users":[{
 "IdentityStoreId":"d-1","UserId":"is-1","UserName":"jane@example.com",
 "ExternalIds":[{"Issuer":"issuer-1","Id":"gid-1"}],
 "Name":{"FamilyName":"Doe","GivenName":"Jane","Formatted":"Dr Jane Q Doe PhD",
  "MiddleName":"Q","HonorificPrefix":"Dr","HonorificSuffix":"PhD"},
 "DisplayName":"Jane Doe","NickName":"JJ","ProfileUrl":"https://example.com/jane",
 "Emails":[{"Value":"alt@example.com","Type":"home","Primary":true}],
 "Addresses":[{"Type":"work","Formatted":"1 Main St, Springfield, IL 62701, US",
  "StreetAddress":"1 Main St","Locality":"Springfield","Region":"IL",
  "PostalCode":"62701","Country":"US","Primary":true}],
 "PhoneNumbers":[{"Value":"+44 20 7946 0000","Type":"work","Primary":true}],
 "Roles":[{"Value":"admin","Type":"work","Primary":true}],
 "UserType":"Employee","Title":"Engineer","PreferredLanguage":"en-GB",
 "Locale":"en_GB","Timezone":"Europe/London",
 "Extensions":{"aws:identitystore:enterprise":{"employeeNumber":"E1","costCenter":"CC1",
  "organization":"Org","division":"Div","department":"Dept","manager":{"value":"mgr-1"}}}
}]}`

// convertedFullUser converts fullListUsersJSON and sets Active as Sync does
// (Active is read from SCIM, not the Identity Store).
func convertedFullUser(t *testing.T) *interfaces.User {
	t.Helper()
	users := sdkUsersFromListUsersJSON(t, fullListUsersJSON)
	require.Len(t, users, 1)
	u := ConvertSdkUserObjToNative(users[0])
	u.Active = true
	return u
}

func TestConvertSdkUserObjToNative_FullReadBack_SteadyStateEqual(t *testing.T) {
	converted := convertedFullUser(t)
	assert.True(t, usersEqual(converted, fullGoogleSideUser()),
		"a steady-state user read back from the Identity Store must compare Equal")
	assert.Equal(t, &interfaces.EnterpriseUser{
		EmployeeNumber: "E1", CostCenter: "CC1", Organization: "Org", Division: "Div",
		Department: "Dept", Manager: &interfaces.EnterpriseManager{Value: "mgr-1"},
	}, converted.Enterprise)
}

func TestConvertSdkUserObjToNative_FullReadBack_GenuineDifference(t *testing.T) {
	tests := map[string]func(u *interfaces.User){
		"Title":                    func(u *interfaces.User) { u.Title = "Manager" },
		"Locale":                   func(u *interfaces.User) { u.Locale = "fr_FR" },
		"Address.Locality":         func(u *interfaces.User) { u.Addresses[0].Locality = "Shelbyville" },
		"PhoneNumbers[0].Value":    func(u *interfaces.User) { u.PhoneNumbers[0].Value = "+1 555 0100" },
		"Roles[0].Value":           func(u *interfaces.User) { u.Roles[0].Value = "reader" },
		"Enterprise.Department":    func(u *interfaces.User) { u.Enterprise.Department = "Sales" },
		"Enterprise.Manager.Value": func(u *interfaces.User) { u.Enterprise.Manager.Value = "mgr-2" },
		"Name.MiddleName":          func(u *interfaces.User) { u.Name.MiddleName = "R" },
	}
	converted := convertedFullUser(t)
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			g := fullGoogleSideUser()
			mutate(g)
			assert.False(t, usersEqual(converted, g), "a genuine %s difference must trip Update", name)
		})
	}
}

// pathStep navigates one level of an interfaces.User value.
type pathStep struct {
	kind  byte // 'f' struct field, 'e' slice element 0, 'p' pointer deref
	field int
}

type leaf struct {
	path []pathStep
	name string
}

// collectLeaves lists every scalar leaf reachable from v; nil pointers and
// empty slices are recorded as (zero) leaves themselves.
func collectLeaves(v reflect.Value, path []pathStep, name string, out *[]leaf) {
	next := func(s pathStep) []pathStep { return append(slices.Clone(path), s) }
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			collectLeaves(v.Field(i), next(pathStep{'f', i}), name+"."+v.Type().Field(i).Name, out)
		}
	case reflect.Ptr:
		if v.IsNil() {
			*out = append(*out, leaf{path, name})
			return
		}
		collectLeaves(v.Elem(), next(pathStep{kind: 'p'}), name, out)
	case reflect.Slice:
		if v.Len() == 0 {
			*out = append(*out, leaf{path, name})
			return
		}
		collectLeaves(v.Index(0), next(pathStep{kind: 'e'}), name+"[0]", out)
	default:
		*out = append(*out, leaf{path, name})
	}
}

func navigate(v reflect.Value, path []pathStep) (reflect.Value, bool) {
	for _, s := range path {
		switch s.kind {
		case 'f':
			v = v.Field(s.field)
		case 'e':
			if v.Len() == 0 {
				return reflect.Value{}, false
			}
			v = v.Index(0)
		case 'p':
			if v.IsNil() {
				return reflect.Value{}, false
			}
			v = v.Elem()
		}
	}
	return v, true
}

// TestUsersEqual_ReadBackDriftGuard fails if a field usersEqual compares is not
// read back by ConvertSdkUserObjToNative (the cause of the read-back churn), and
// forces fullGoogleSideUser to grow with interfaces.User.
func TestUsersEqual_ReadBackDriftGuard(t *testing.T) {
	// Fields sourced elsewhere than the Identity Store read-back.
	allow := map[string]string{
		".Active": "read from SCIM (FindUserByEmail) by Sync, not the Identity Store",
	}

	var leaves []leaf
	collectLeaves(reflect.ValueOf(fullGoogleSideUser()).Elem(), nil, "", &leaves)

	// (1) every leaf of the fixture is non-zero.
	for _, l := range leaves {
		v, _ := navigate(reflect.ValueOf(fullGoogleSideUser()).Elem(), l.path)
		assert.False(t, v.IsZero(), "fullGoogleSideUser leaves %s zero; set it so the drift guard covers it", l.name)
	}

	converted := convertedFullUser(t)
	convertedRoot := reflect.ValueOf(converted).Elem()

	// (2) every compared leaf is read back.
	compared := 0
	for _, l := range leaves {
		fixture := fullGoogleSideUser()
		mutated := fullGoogleSideUser()
		mv, _ := navigate(reflect.ValueOf(mutated).Elem(), l.path)
		mv.Set(reflect.Zero(mv.Type()))
		if usersEqual(fixture, mutated) {
			continue // not compared by usersEqual (ID, Schemas, ExternalId)
		}
		compared++
		if _, ok := allow[l.name]; ok {
			continue
		}
		want, _ := navigate(reflect.ValueOf(fixture).Elem(), l.path)
		got, ok := navigate(convertedRoot, l.path)
		if !assert.True(t, ok, "%s is compared by usersEqual but absent after read-back", l.name) {
			continue
		}
		assert.Equal(t, want.Interface(), got.Interface(),
			"%s is compared by usersEqual but not read back by ConvertSdkUserObjToNative", l.name)
	}
	assert.Greater(t, compared, 30, "expected the drift guard to exercise most fields")
}

func TestUsersEqual_OmitEmptyNormalization(t *testing.T) {
	base := func() *interfaces.User {
		return &interfaces.User{Username: "jane@example.com", Active: true}
	}
	pairs := map[string]func(a, b *interfaces.User){
		"nil vs empty PhoneNumbers": func(a, b *interfaces.User) { b.PhoneNumbers = []interfaces.UserPhoneNumber{} },
		"nil vs empty Roles":        func(a, b *interfaces.User) { b.Roles = []interfaces.UserRole{} },
		"nil vs empty Enterprise":   func(a, b *interfaces.User) { b.Enterprise = &interfaces.EnterpriseUser{} },
		"empty Manager vs nil": func(a, b *interfaces.User) {
			a.Enterprise = &interfaces.EnterpriseUser{Department: "D"}
			b.Enterprise = &interfaces.EnterpriseUser{Department: "D", Manager: &interfaces.EnterpriseManager{}}
		},
		"Enterprise with only empty Manager vs nil": func(a, b *interfaces.User) {
			b.Enterprise = &interfaces.EnterpriseUser{Manager: &interfaces.EnterpriseManager{}}
		},
	}
	for name, setup := range pairs {
		t.Run(name, func(t *testing.T) {
			a, b := base(), base()
			setup(a, b)
			assert.True(t, usersEqual(a, b))
			assert.True(t, usersEqual(b, a))
		})
	}

	t.Run("Manager value differs", func(t *testing.T) {
		a, b := base(), base()
		a.Enterprise = &interfaces.EnterpriseUser{Manager: &interfaces.EnterpriseManager{Value: "m1"}}
		b.Enterprise = &interfaces.EnterpriseUser{Manager: &interfaces.EnterpriseManager{Value: "m2"}}
		assert.False(t, usersEqual(a, b))
	})

	t.Run("normalization does not mutate the callers' users", func(t *testing.T) {
		a, b := base(), base()
		b.Enterprise = &interfaces.EnterpriseUser{Manager: &interfaces.EnterpriseManager{}}
		b.PhoneNumbers = []interfaces.UserPhoneNumber{}
		usersEqual(a, b)
		require.NotNil(t, b.Enterprise)
		assert.NotNil(t, b.Enterprise.Manager)
		assert.NotNil(t, b.PhoneNumbers)
	})
}

func TestConvertSdkUserObjToNative_NoExtension(t *testing.T) {
	users := sdkUsersFromListUsersJSON(t, `{"Users":[{"IdentityStoreId":"d-1","UserId":"is-1","UserName":"jane@example.com"}]}`)
	require.Len(t, users, 1)
	native := ConvertSdkUserObjToNative(users[0])
	assert.Nil(t, native.Enterprise)
	assert.Nil(t, native.PhoneNumbers)
	assert.Nil(t, native.Roles)
}

func TestConvertSdkUserObjToNative_EmptyExtension(t *testing.T) {
	body := `{"Users":[{"IdentityStoreId":"d-1","UserId":"is-1","UserName":"j",
	 "Extensions":{"aws:identitystore:enterprise":{"department":"","manager":{"value":""}}}}]}`
	users := sdkUsersFromListUsersJSON(t, body)
	require.Len(t, users, 1)
	assert.Nil(t, ConvertSdkUserObjToNative(users[0]).Enterprise)
}

func TestConvertSdkUserObjToNative_ExtensionWithoutManager(t *testing.T) {
	body := `{"Users":[{"IdentityStoreId":"d-1","UserId":"is-1","UserName":"j",
	 "Extensions":{"aws:identitystore:enterprise":{"department":"Dept"}}}]}`
	users := sdkUsersFromListUsersJSON(t, body)
	require.Len(t, users, 1)
	assert.Equal(t, &interfaces.EnterpriseUser{Department: "Dept"}, ConvertSdkUserObjToNative(users[0]).Enterprise)
}

func TestGetUsers_RequestsEnterpriseExtension(t *testing.T) {
	identityStore := mocks.NewMockIdentityStoreAPI(t)
	identityStore.EXPECT().ListUsers(mock.Anything, mock.MatchedBy(func(in *identitystore.ListUsersInput) bool {
		return slices.Equal(in.Extensions, []string{"aws:identitystore:enterprise"}) &&
			awssdk.ToString(in.IdentityStoreId) == "d-1"
	}), mock.Anything).Return(&identitystore.ListUsersOutput{
		Users: []types.User{sdkUser("is-1", "jane@example.com", "Jane", "Doe", "gid-1")},
	}, nil)

	s := &syncGSuite{identityStore: identityStore, cfg: &config.Config{IdentityStoreID: "d-1"}}
	users, err := s.GetUsers()
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "jane@example.com", users[0].Username)
}

// Guard against the constant drifting from the documented extension name.
func TestIdentityStoreEnterpriseExtensionName(t *testing.T) {
	assert.Equal(t, "aws:identitystore:enterprise", fmt.Sprint(constants.IdentityStoreEnterpriseExtension))
}
