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
	"strconv"
	"strings"
	"testing"

	"ssosync/internal/interfaces"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

// validUser returns a mapped, already-normalized user that mirrors the
// default-config steady state (names + primary work email + work address +
// active) and must produce zero findings (design acceptance criterion 5).
func validUser() *interfaces.User {
	return &interfaces.User{
		Username:    "bjensen@example.com",
		DisplayName: "Barbara Jensen",
		Active:      true,
		Name:        interfaces.UserName{GivenName: "Barbara", FamilyName: "Jensen"},
		Emails: []interfaces.UserEmail{
			{Value: "bjensen@example.com", Type: "work", Primary: true},
		},
		Addresses: []interfaces.UserAddress{
			{Type: "work"},
		},
	}
}

// validGroup returns a mapped, already-normalized group that must produce zero
// findings.
func validGroup() *interfaces.Group {
	return &interfaces.Group{
		DisplayName: "Engineering",
		Members:     []string{},
	}
}

// findingFor returns the first finding with the given rule+field, or false.
func findingFor(findings []finding, rule, field string) (finding, bool) {
	for _, f := range findings {
		if f.rule == rule && f.field == field {
			return f, true
		}
	}
	return finding{}, false
}

func TestValidateUser_Valid(t *testing.T) {
	assert.Empty(t, validateUser(validUser()))
}

func TestValidateGroup_Valid(t *testing.T) {
	assert.Empty(t, validateGroup(validGroup()))
}

func TestValidateUser_Nil(t *testing.T) {
	assert.Nil(t, validateUser(nil))
}

func TestValidateGroup_Nil(t *testing.T) {
	assert.Nil(t, validateGroup(nil))
}

func TestValidateUser_RequiredFields(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(u *interfaces.User)
		field string
	}{
		{"userName", func(u *interfaces.User) { u.Username = "" }, "userName"},
		{"userName whitespace", func(u *interfaces.User) { u.Username = "   " }, "userName"},
		{"givenName", func(u *interfaces.User) { u.Name.GivenName = "" }, "name.givenName"},
		{"familyName", func(u *interfaces.User) { u.Name.FamilyName = "" }, "name.familyName"},
		{"displayName", func(u *interfaces.User) { u.DisplayName = "" }, "displayName"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := validUser()
			tc.mut(u)
			f, ok := findingFor(validateUser(u), "requiredField", tc.field)
			assert.True(t, ok, "expected requiredField finding for %s", tc.field)
			assert.Equal(t, "empty", f.value)
		})
	}
}

func TestValidateGroup_RequiredDisplayName(t *testing.T) {
	g := validGroup()
	g.DisplayName = ""
	f, ok := findingFor(validateGroup(g), "requiredField", "displayName")
	assert.True(t, ok)
	assert.Equal(t, "empty", f.value)
}

func TestValidateUser_MultiValuedSingleOnly(t *testing.T) {
	t.Run("emails", func(t *testing.T) {
		u := validUser()
		u.Emails = []interfaces.UserEmail{
			{Value: "a@example.com", Type: "work", Primary: true},
			{Value: "b@example.com", Type: "home"},
		}
		f, ok := findingFor(validateUser(u), "multiValuedSingleOnly", "emails")
		assert.True(t, ok)
		assert.Equal(t, strconv.Itoa(2), f.value)
	})

	t.Run("addresses", func(t *testing.T) {
		u := validUser()
		u.Addresses = []interfaces.UserAddress{{Type: "work"}, {Type: "home"}}
		f, ok := findingFor(validateUser(u), "multiValuedSingleOnly", "addresses")
		assert.True(t, ok)
		assert.Equal(t, strconv.Itoa(2), f.value)
	})

	t.Run("phoneNumbers", func(t *testing.T) {
		u := validUser()
		u.PhoneNumbers = []interfaces.UserPhoneNumber{{Value: "1"}, {Value: "2"}, {Value: "3"}}
		f, ok := findingFor(validateUser(u), "multiValuedSingleOnly", "phoneNumbers")
		assert.True(t, ok)
		assert.Equal(t, strconv.Itoa(3), f.value)
	})
}

// Roles multi-value is deliberately NOT a rule (design §4a). More than one role
// must yield no multiValuedSingleOnly finding.
func TestValidateUser_RolesNotChecked(t *testing.T) {
	u := validUser()
	u.Roles = []interfaces.UserRole{{Value: "r1"}, {Value: "r2"}}
	_, ok := findingFor(validateUser(u), "multiValuedSingleOnly", "roles")
	assert.False(t, ok, "roles cardinality must not be validated")
}

func TestValidateUser_EmailNotPrimary(t *testing.T) {
	u := validUser()
	u.Emails = []interfaces.UserEmail{{Value: "solo@example.com", Type: "work", Primary: false}}
	f, ok := findingFor(validateUser(u), "emailNotPrimary", "emails")
	assert.True(t, ok)
	assert.Equal(t, "solo@example.com", f.value)
}

func TestValidateUser_UserNameMaxLength(t *testing.T) {
	t.Run("at limit passes", func(t *testing.T) {
		u := validUser()
		u.Username = strings.Repeat("a", userNameMaxLen)
		_, ok := findingFor(validateUser(u), "maxLength", "userName")
		assert.False(t, ok)
	})
	t.Run("over limit fires", func(t *testing.T) {
		u := validUser()
		u.Username = strings.Repeat("a", userNameMaxLen+1)
		f, ok := findingFor(validateUser(u), "maxLength", "userName")
		assert.True(t, ok)
		assert.Equal(t, userNameMaxLen+1, f.length)
		assert.Equal(t, userNameMaxLen, f.limit)
		assert.Equal(t, "", f.value)
	})
}

func TestValidateGroup_DisplayNameMaxLength(t *testing.T) {
	t.Run("at limit passes", func(t *testing.T) {
		g := validGroup()
		g.DisplayName = strings.Repeat("a", groupDisplayNameMaxLen)
		_, ok := findingFor(validateGroup(g), "maxLength", "displayName")
		assert.False(t, ok)
	})
	t.Run("over limit fires", func(t *testing.T) {
		g := validGroup()
		g.DisplayName = strings.Repeat("a", groupDisplayNameMaxLen+1)
		f, ok := findingFor(validateGroup(g), "maxLength", "displayName")
		assert.True(t, ok)
		assert.Equal(t, groupDisplayNameMaxLen+1, f.length)
		assert.Equal(t, groupDisplayNameMaxLen, f.limit)
	})
}

func TestValidateUser_CharSet(t *testing.T) {
	// userName allows no whitespace: any control char (incl. \t \n \r) fires.
	for _, r := range []rune{'\u0007', '\t', '\n', '\r'} {
		u := validUser()
		u.Username = "ab" + string(r) + "cd"
		f, ok := findingFor(validateUser(u), "charSet", "userName")
		assert.True(t, ok, "expected charSet for rune %q", r)
		assert.Equal(t, strconv.QuoteRune(r), f.value)
	}
}

func TestValidateGroup_CharSet(t *testing.T) {
	// group displayName allows tab/newline/CR -> no finding.
	for _, r := range []rune{'\t', '\n', '\r'} {
		g := validGroup()
		g.DisplayName = "Eng" + string(r) + "ineering"
		_, ok := findingFor(validateGroup(g), "charSet", "displayName")
		assert.False(t, ok, "tab/newline/CR must be allowed in group displayName, rune %q", r)
	}
	// another control char fires.
	g := validGroup()
	g.DisplayName = "Eng\u0007ineering"
	f, ok := findingFor(validateGroup(g), "charSet", "displayName")
	assert.True(t, ok)
	assert.Equal(t, strconv.QuoteRune('\u0007'), f.value)
}

// charSet takes precedence over maxLength for userName: an over-length value
// that also contains a control char reports charSet (the else-branch), matching
// the design's single-issue-per-field ordering.
func TestValidateUser_CharSetBeforeLength(t *testing.T) {
	u := validUser()
	u.Username = strings.Repeat("a", userNameMaxLen+5) + "\u0007"
	findings := validateUser(u)
	_, charSet := findingFor(findings, "charSet", "userName")
	_, maxLen := findingFor(findings, "maxLength", "userName")
	assert.True(t, charSet)
	assert.False(t, maxLen)
}

// logFindings must emit one WARN line per finding at WARN level, carrying the
// documented structured fields and the constant message (design criterion 6).
func TestLogFindings_WarnLevelAndFields(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()
	prev := log.GetLevel()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(prev)

	findings := []finding{
		{rule: "requiredField", field: "name.givenName", value: "empty", source: "aws-createuser-example"},
		{rule: "maxLength", field: "userName", source: "identitystore-sdk", length: 142, limit: 128},
	}
	logFindings(findings, "user", "bjensen@example.com")

	entries := hook.AllEntries()
	assert.Len(t, entries, 2)
	for _, e := range entries {
		assert.Equal(t, log.WarnLevel, e.Level)
		assert.Equal(t, validateUserMsg, e.Message)
		assert.Equal(t, "user", e.Data["kind"])
		assert.Equal(t, "bjensen@example.com", e.Data["id"])
		assert.NotEmpty(t, e.Data["rule"])
		assert.NotEmpty(t, e.Data["field"])
		assert.NotEmpty(t, e.Data["source"])
	}

	// requiredField carries value, no len/limit.
	req := entries[0]
	assert.Equal(t, "empty", req.Data["value"])
	_, hasLen := req.Data["len"]
	_, hasLimit := req.Data["limit"]
	assert.False(t, hasLen)
	assert.False(t, hasLimit)

	// maxLength carries len/limit, no value.
	max := entries[1]
	assert.Equal(t, 142, max.Data["len"])
	assert.Equal(t, 128, max.Data["limit"])
	_, hasValue := max.Data["value"]
	assert.False(t, hasValue)
}

func TestLogFindings_GroupMessage(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	logFindings([]finding{{rule: "requiredField", field: "displayName", value: "empty", source: "aws-createuser-example"}}, "group", "Engineering")

	entries := hook.AllEntries()
	assert.Len(t, entries, 1)
	assert.Equal(t, validateGroupMsg, entries[0].Message)
	assert.Equal(t, "group", entries[0].Data["kind"])
}

func TestLogFindings_NoFindingsNoLog(t *testing.T) {
	hook := test.NewGlobal()
	defer hook.Reset()

	logFindings(nil, "user", "bjensen@example.com")
	assert.Empty(t, hook.AllEntries())
}

func TestValidateIDPrecedence(t *testing.T) {
	assert.Equal(t, "bjensen@example.com", userValidateID(&interfaces.User{Username: "bjensen@example.com"}))
	assert.Equal(t, "ext-1", userValidateID(&interfaces.User{ExternalId: interfaces.ExternalId{Id: "ext-1"}}))
	assert.Equal(t, "", userValidateID(nil))

	assert.Equal(t, "Engineering", groupValidateID(&interfaces.Group{DisplayName: "Engineering"}))
	assert.Equal(t, "ext-g", groupValidateID(&interfaces.Group{ExternalId: interfaces.ExternalId{Id: "ext-g"}}))
	assert.Equal(t, "", groupValidateID(nil))
}
