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
	"unicode"
	"unicode/utf8"

	"ssosync/internal/interfaces"

	log "github.com/sirupsen/logrus"
)

// Pre-SCIM attribute validation.
//
// This is a runtime, pre-SCIM predictive check that inspects each mapped
// interfaces.User / interfaces.Group just before it is sent to the AWS IAM
// Identity Center SCIM create/update calls and logs a WARN for every rule AWS
// documents with certainty. It is warn-only: it never hard-fails, never aborts
// the sync, and never silently drops a user/group without a WARN. The object is
// always sent regardless of findings; AWS remains the authoritative acceptance
// decision.
//
// This is a DIFFERENT check from the attribute-map config load-time validator
// (internal/config); it operates on the already-mapped, already-normalized
// wire object the Sync loop passes to the SCIM client.

// Authoritative per-field length limits. Both come from the aws-sdk-go-v2
// service/identitystore types model (types/types.go @ v1.38.0), which describes
// the same underlying store objects the SCIM API writes:
//
//   - User.UserName: "The length limit is 128 characters."
//   - Group.DisplayName: "The length limit is 1,024 characters."
//
// These are the only two authoritative per-field lengths discoverable; no other
// field gets a length check. See design §4c/§A1.
const (
	userNameMaxLen         = 128
	groupDisplayNameMaxLen = 1024
)

// finding is one predicted SCIM reject. Its fields back the structured WARN log
// keys exactly (design §2.1/§3).
type finding struct {
	// rule is the stable machine token for the violated rule:
	// requiredField, emailNotPrimary, multiValuedSingleOnly, charSet, maxLength.
	rule string
	// field is the SCIM target field, e.g. "name.givenName", "userName".
	field string
	// value depends on the rule:
	//   requiredField         -> "empty"
	//   emailNotPrimary       -> the offending email value
	//   multiValuedSingleOnly -> decimal element count (strconv.Itoa)
	//   charSet               -> strconv.QuoteRune of the first offending rune
	//   maxLength             -> empty (len/limit carry the detail)
	value string
	// source is the authoritative citation token: identitystore-sdk,
	// aws-createuser-example, or issue-178.
	source string
	// length is the measured rune length; set only for maxLength findings.
	length int
	// limit is the authoritative limit; set only for maxLength findings.
	limit int
}

// validateMsg is the constant WARN message. It reuses the "attribute map:"
// prefix of the existing mapping diagnostics in google/client.go so the
// validation warnings read as the same diagnostic story. The <user|group>
// placeholder is substituted per object kind.
const (
	validateUserMsg  = "attribute map: mapped user may be rejected by the SCIM API; check the attribute map (run with --dry-run to preview)"
	validateGroupMsg = "attribute map: mapped group may be rejected by the SCIM API; check the attribute map (run with --dry-run to preview)"
)

// validateUser inspects a mapped, already-normalized *interfaces.User and
// returns the predicted SCIM rejects as a deterministic, fixed-order slice of
// findings. It is pure, never errors, and never panics on nil (returns nil).
func validateUser(u *interfaces.User) []finding {
	if u == nil {
		return nil
	}

	var findings []finding

	// requiredField: userName, name.givenName, name.familyName, displayName.
	// Source: aws-createuser-example (example-derived predictive).
	if strings.TrimSpace(u.Username) == "" {
		findings = append(findings, finding{rule: "requiredField", field: "userName", value: "empty", source: "aws-createuser-example"})
	}
	if strings.TrimSpace(u.Name.GivenName) == "" {
		findings = append(findings, finding{rule: "requiredField", field: "name.givenName", value: "empty", source: "aws-createuser-example"})
	}
	if strings.TrimSpace(u.Name.FamilyName) == "" {
		findings = append(findings, finding{rule: "requiredField", field: "name.familyName", value: "empty", source: "aws-createuser-example"})
	}
	if strings.TrimSpace(u.DisplayName) == "" {
		findings = append(findings, finding{rule: "requiredField", field: "displayName", value: "empty", source: "aws-createuser-example"})
	}

	// multiValuedSingleOnly: emails, addresses, phoneNumbers.
	// Source: aws-createuser-example (single-element arrays in the CreateUser JSON).
	if len(u.Emails) > 1 {
		findings = append(findings, finding{rule: "multiValuedSingleOnly", field: "emails", value: strconv.Itoa(len(u.Emails)), source: "aws-createuser-example"})
	}
	if len(u.Addresses) > 1 {
		findings = append(findings, finding{rule: "multiValuedSingleOnly", field: "addresses", value: strconv.Itoa(len(u.Addresses)), source: "aws-createuser-example"})
	}
	if len(u.PhoneNumbers) > 1 {
		findings = append(findings, finding{rule: "multiValuedSingleOnly", field: "phoneNumbers", value: strconv.Itoa(len(u.PhoneNumbers)), source: "aws-createuser-example"})
	}

	// emailNotPrimary: exactly one email present AND it is not primary.
	// Source: aws-createuser-example (email carries "primary": true).
	if len(u.Emails) == 1 && !u.Emails[0].Primary {
		findings = append(findings, finding{rule: "emailNotPrimary", field: "emails", value: u.Emails[0].Value, source: "aws-createuser-example"})
	}

	// charSet + maxLength on userName. Source: identitystore-sdk.
	// userName allows letters, accented chars, symbols, numbers, punctuation —
	// NO whitespace of any kind — so fire on any control character.
	if r, ok := firstOffendingRune(u.Username, func(r rune) bool {
		return unicode.IsControl(r)
	}); ok {
		findings = append(findings, finding{rule: "charSet", field: "userName", value: strconv.QuoteRune(r), source: "identitystore-sdk"})
	} else if n := utf8.RuneCountInString(u.Username); n > userNameMaxLen {
		findings = append(findings, finding{rule: "maxLength", field: "userName", source: "identitystore-sdk", length: n, limit: userNameMaxLen})
	}

	return findings
}

// validateGroup inspects a mapped, already-normalized *interfaces.Group and
// returns the predicted SCIM rejects. It validates only displayName (required +
// charSet + maxLength); members shape is handled by the dedicated add/remove
// path and is not validated here, and group description is not SCIM-settable.
// It is pure, never errors, and never panics on nil (returns nil).
func validateGroup(g *interfaces.Group) []finding {
	if g == nil {
		return nil
	}

	var findings []finding

	// requiredField: displayName.
	// Source: aws-createuser-example (CreateGroup needs a displayName).
	if strings.TrimSpace(g.DisplayName) == "" {
		findings = append(findings, finding{rule: "requiredField", field: "displayName", value: "empty", source: "aws-createuser-example"})
	}

	// charSet + maxLength on displayName. Source: identitystore-sdk.
	// group displayName allows letters, accented chars, symbols, numbers,
	// punctuation, tab, newline, carriage return, space, nonbreaking space —
	// so fire on a control char that is not tab/newline/carriage return.
	if r, ok := firstOffendingRune(g.DisplayName, func(r rune) bool {
		return unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r'
	}); ok {
		findings = append(findings, finding{rule: "charSet", field: "displayName", value: strconv.QuoteRune(r), source: "identitystore-sdk"})
	} else if n := utf8.RuneCountInString(g.DisplayName); n > groupDisplayNameMaxLen {
		findings = append(findings, finding{rule: "maxLength", field: "displayName", source: "identitystore-sdk", length: n, limit: groupDisplayNameMaxLen})
	}

	return findings
}

// userValidateID returns the identifier used in the WARN "id" field for a user:
// Username, falling back to ExternalId.Id when Username is empty. This mirrors
// the PrimaryEmail->Id precedence the Google-side warnCustomSourceAbsent helper
// uses. A nil user yields an empty id.
func userValidateID(u *interfaces.User) string {
	if u == nil {
		return ""
	}
	if u.Username != "" {
		return u.Username
	}
	return u.ExternalId.Id
}

// groupValidateID returns the identifier used in the WARN "id" field for a
// group: DisplayName, falling back to ExternalId.Id when DisplayName is empty.
// A nil group yields an empty id.
func groupValidateID(g *interfaces.Group) string {
	if g == nil {
		return ""
	}
	if g.DisplayName != "" {
		return g.DisplayName
	}
	return g.ExternalId.Id
}

// firstOffendingRune returns the first rune in s for which offends returns true,
// scanning left to right so the result is deterministic.
func firstOffendingRune(s string, offends func(rune) bool) (rune, bool) {
	for _, r := range s {
		if offends(r) {
			return r, true
		}
	}
	return 0, false
}

// logFindings emits one WARN line per finding via logrus structured fields,
// using the constant message for the given kind. The len/limit keys are emitted
// only for maxLength findings; the value key is emitted for every rule except
// maxLength. kind is "user" or "group"; id is the object identifier (design §3).
func logFindings(findings []finding, kind, id string) {
	if len(findings) == 0 {
		return
	}

	msg := validateUserMsg
	if kind == "group" {
		msg = validateGroupMsg
	}

	for _, f := range findings {
		fields := log.Fields{
			"kind":   kind,
			"id":     id,
			"rule":   f.rule,
			"field":  f.field,
			"source": f.source,
		}
		if f.rule == "maxLength" {
			fields["len"] = f.length
			fields["limit"] = f.limit
		} else {
			fields["value"] = f.value
		}
		log.WithFields(fields).Warn(msg)
	}
}
