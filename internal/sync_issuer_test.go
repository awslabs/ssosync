package internal

import (
	"encoding/json"
	"testing"

	"ssosync/internal/interfaces"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
)

// Issuer values in the shape IAM Identity Center uses (issue #383): the real
// issuer has the management account and a mixed-case provisioning tenant id.
const (
	realIssuer      = "arn:aws:identitystore::111111111111:identitystore/d-1234567890/provisioningtenant/Dbv12345678-abcd-EF01"
	lowercaseIssuer = "arn:aws:identitystore::111111111111:identitystore/d-1234567890/provisioningtenant/dbv12345678-abcd-ef01"
	delegatedIssuer = "arn:aws:identitystore::222222222222:identitystore/d-1234567890/provisioningtenant/Dbv12345678-abcd-EF01"
)

// captureDebug enables debug logging and captures entries for the test.
func captureDebug(t *testing.T) *test.Hook {
	t.Helper()
	hook := test.NewGlobal()
	prev := log.GetLevel()
	log.SetLevel(log.DebugLevel)
	t.Cleanup(func() {
		log.SetLevel(prev)
		hook.Reset()
	})
	return hook
}

func entriesWithMsg(hook *test.Hook, msg string) []*log.Entry {
	var out []*log.Entry
	for _, e := range hook.AllEntries() {
		if e.Message == msg {
			out = append(out, e)
		}
	}
	return out
}

func issuerUser(issuer, extID string) *interfaces.User {
	return &interfaces.User{
		ID:       "aws-1",
		Username: "jane@example.com",
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{FamilyName: "Doe", GivenName: "Jane"},
		Active:     true,
		ExternalId: interfaces.ExternalId{Id: extID, Issuer: issuer},
	}
}

func issuerGroup(issuer, extID string) *interfaces.Group {
	return &interfaces.Group{ID: "grp-1", DisplayName: "AWSAdmins", ExternalId: interfaces.ExternalId{Id: extID, Issuer: issuer}}
}

func googleJane(id string) *admin.User {
	return &admin.User{Id: id, PrimaryEmail: "jane@example.com", Name: &admin.UserName{GivenName: "Jane", FamilyName: "Doe"}}
}

func googleAdmins(id string) *admin.Group {
	return &admin.Group{Id: id, Name: "AWSAdmins"}
}

// --- The decision depends on the exact issuer (issue #383 regression) ---

func TestGetUserOperation_IssuerExactMatchIsManaged(t *testing.T) {
	action, _ := getUserOperation(issuerUser(realIssuer, "G1"), googleJane("G1"), false, false, realIssuer)
	assert.Equal(t, Equal, action, "a user provisioned by this tenant must be ssosync-managed")
}

// A different account (e.g. the delegated administrator's) is a different
// issuer: foreign.
func TestGetUserOperation_IssuerMismatchIsForeign(t *testing.T) {
	action, _ := getUserOperation(issuerUser(realIssuer, "G1"), googleJane("G1"), false, false, delegatedIssuer)
	assert.Equal(t, AdoptForeign, action)
}

// A difference only in letter case is the same tenant: managed (issuerMatches).
func TestGetUserOperation_IssuerCaseOnlyDifferenceIsManaged(t *testing.T) {
	action, _ := getUserOperation(issuerUser(realIssuer, "G1"), googleJane("G1"), false, false, lowercaseIssuer)
	assert.Equal(t, Equal, action)
}

// The live finding behind the case-insensitive comparison: IAM Identity Center
// stamped records of ONE SCIM tenant with the tenant id either as in the SCIM
// endpoint (capital H) or lowercased; both spellings occurred among its users,
// and its groups were lowercased. Every record must be managed against the one
// expected IssuerArn (built with its original case).
func TestOperations_IssuerCasingVariesWithinTenant_AllManaged(t *testing.T) {
	const (
		expected   = "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/Hw523407fb9-d9b9-4fc6-8568-cc8afd03d4d1"
		original   = expected
		lowercased = "arn:aws:identitystore::920862271044:identitystore/d-9a671d7705/provisioningtenant/hw523407fb9-d9b9-4fc6-8568-cc8afd03d4d1"
	)
	// Two users of the same tenant, one with each spelling.
	bob := &admin.User{Id: "G2", PrimaryEmail: "bob@example.com", Name: &admin.UserName{GivenName: "Bob", FamilyName: "Roe"}}
	bobAWS := &interfaces.User{ID: "aws-2", Username: "bob@example.com", Active: true, ExternalId: interfaces.ExternalId{Id: "G2", Issuer: lowercased}}
	bobAWS.Name.GivenName, bobAWS.Name.FamilyName = "Bob", "Roe"
	_, _, uUpd, uEq := getUserOperations(
		[]*interfaces.User{issuerUser(original, "G1"), bobAWS},
		[]*admin.User{googleJane("G1"), bob}, false, false, expected)
	assert.Empty(t, uUpd, "no user may be re-adopted, whichever spelling it carries")
	assert.Len(t, uEq, 2)

	groupIssuer := lowercased
	_, _, gUpd, gEq := getGroupOperations([]*interfaces.Group{issuerGroup(groupIssuer, "GG1")}, []*admin.Group{googleAdmins("GG1")}, false, false, expected)
	assert.Empty(t, gUpd, "group must not be re-adopted (AdoptForeign) every run")
	assert.Len(t, gEq, 1)
}

func TestIssuerMatches(t *testing.T) {
	assert.True(t, issuerMatches(realIssuer, realIssuer))
	assert.True(t, issuerMatches(lowercaseIssuer, realIssuer), "case-only difference")
	assert.True(t, issuerMatches(realIssuer, lowercaseIssuer), "case-only difference")
	assert.True(t, issuerMatches("anything", ""), "empty expected disables the check")
	assert.False(t, issuerMatches(delegatedIssuer, realIssuer), "different account")
	assert.False(t, issuerMatches("", realIssuer), "no issuer")
	assert.False(t, issuerMatches(realIssuer+"x", realIssuer), "different tenant")
}

func TestGetGroupOperation_IssuerExactMatchIsManaged(t *testing.T) {
	action, _ := getGroupOperation(issuerGroup(realIssuer, "GG1"), googleAdmins("GG1"), false, false, realIssuer)
	assert.Equal(t, Equal, action)
}

func TestGetGroupOperation_IssuerMismatchIsForeign(t *testing.T) {
	action, _ := getGroupOperation(issuerGroup(realIssuer, "GG1"), googleAdmins("GG1"), false, false, delegatedIssuer)
	assert.Equal(t, AdoptForeign, action)
}

func TestGetGroupOperation_IssuerCaseOnlyDifferenceIsManaged(t *testing.T) {
	action, _ := getGroupOperation(issuerGroup(lowercaseIssuer, "GG1"), googleAdmins("GG1"), false, false, realIssuer)
	assert.Equal(t, Equal, action)
}

// With a matching issuer the ExternalId-change path is reachable again:
// Recreate by default, UpdateForced with --force-externalid-update.
func TestOperations_ExternalIdChangeReachableAfterIssuerMatch(t *testing.T) {
	action, _ := getUserOperation(issuerUser(realIssuer, "OLD"), googleJane("NEW"), false, false, realIssuer)
	assert.Equal(t, Recreate, action)
	action, _ = getUserOperation(issuerUser(realIssuer, "OLD"), googleJane("NEW"), true, false, realIssuer)
	assert.Equal(t, UpdateForced, action)

	gAction, _ := getGroupOperation(issuerGroup(realIssuer, "OLD"), googleAdmins("NEW"), false, false, realIssuer)
	assert.Equal(t, Recreate, gAction)
	gAction, _ = getGroupOperation(issuerGroup(realIssuer, "OLD"), googleAdmins("NEW"), true, false, realIssuer)
	assert.Equal(t, UpdateForced, gAction)
}

// --- Debug messages ---

func TestGetUserOperation_LogsIssuerCheck(t *testing.T) {
	tests := []struct {
		name                                                         string
		expected                                                     string
		issuerMatch, exact, caseInsensitive, account, storeAndTenant bool
	}{
		{"exact match", realIssuer, true, true, true, true, true},
		{"case only difference matches", lowercaseIssuer, true, false, true, true, false},
		{"account mismatch", delegatedIssuer, false, false, false, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hook := captureDebug(t)
			getUserOperation(issuerUser(realIssuer, "G1"), googleJane("G1"), false, false, tc.expected)

			lines := entriesWithMsg(hook, IssuerCheckMsg)
			require.Len(t, lines, 1)
			d := lines[0].Data
			assert.Equal(t, log.DebugLevel, lines[0].Level)
			assert.Equal(t, "user", d["kind"])
			assert.Equal(t, "jane@example.com", d["name"])
			assert.Equal(t, "aws-1", d["awsId"])
			assert.Equal(t, "G1", d["externalId"])
			assert.Equal(t, "G1", d["googleId"])
			assert.Equal(t, true, d["externalIdMatch"])
			assert.Equal(t, realIssuer, d["issuer"])
			assert.Equal(t, tc.expected, d["expectedIssuer"])
			assert.Equal(t, tc.issuerMatch, d["issuerMatch"])
			assert.Equal(t, tc.exact, d["issuerExactMatch"])
			assert.Equal(t, tc.caseInsensitive, d["caseInsensitiveMatch"])
			assert.Equal(t, tc.account, d["accountMatch"])
			assert.Equal(t, "111111111111", d["issuerAccount"])
			assert.Equal(t, tc.storeAndTenant, d["storeAndTenantMatch"])
		})
	}
}

func TestGetGroupOperation_LogsIssuerCheck(t *testing.T) {
	hook := captureDebug(t)
	getGroupOperation(issuerGroup(realIssuer, "GG1"), googleAdmins("GG1"), false, false, realIssuer)

	lines := entriesWithMsg(hook, IssuerCheckMsg)
	require.Len(t, lines, 1)
	assert.Equal(t, "group", lines[0].Data["kind"])
	assert.Equal(t, "AWSAdmins", lines[0].Data["name"])
	assert.Equal(t, true, lines[0].Data["issuerMatch"])
}

func TestIssuerCheck_NotLoggedForLocalOrMissingRecords(t *testing.T) {
	hook := captureDebug(t)
	getUserOperation(nil, googleJane("G1"), false, false, realIssuer)                // create
	getUserOperation(issuerUser("", ""), googleJane("G1"), false, false, realIssuer) // local, no ExternalId
	assert.Empty(t, entriesWithMsg(hook, IssuerCheckMsg))
}

func TestIssuerCheck_DisabledWhenNoExpectedIssuer(t *testing.T) {
	hook := captureDebug(t)
	action, _ := getUserOperation(issuerUser(realIssuer, "G1"), googleJane("G1"), false, false, "")
	assert.Equal(t, Equal, action)
	lines := entriesWithMsg(hook, IssuerCheckMsg)
	require.Len(t, lines, 1)
	assert.Equal(t, true, lines[0].Data["issuerCheckDisabled"])
	assert.Equal(t, true, lines[0].Data["issuerMatch"])
}

func TestOperations_LogExpectedIssuer(t *testing.T) {
	hook := captureDebug(t)
	getUserOperations([]*interfaces.User{issuerUser(realIssuer, "G1")}, []*admin.User{googleJane("G1")}, false, false, realIssuer)
	getGroupOperations([]*interfaces.Group{issuerGroup(realIssuer, "GG1")}, []*admin.Group{googleAdmins("GG1")}, false, false, realIssuer)

	lines := entriesWithMsg(hook, ExpectedIssuerMsg)
	require.Len(t, lines, 2)
	kinds := map[interface{}]bool{}
	for _, l := range lines {
		kinds[l.Data["kind"]] = true
		assert.Equal(t, realIssuer, l.Data["expectedIssuer"])
	}
	assert.Equal(t, map[interface{}]bool{"user": true, "group": true}, kinds)
}

func TestConverters_LogEveryExternalIdRead(t *testing.T) {
	hook := captureDebug(t)
	u := sdkUser("aws-1", "jane@example.com", "Jane", "Doe", "")
	u.ExternalIds = []types.ExternalId{
		{Id: aws.String("G1"), Issuer: aws.String(realIssuer)},
		{Id: aws.String("X9"), Issuer: aws.String("other-idp")},
	}
	native := ConvertSdkUserObjToNative(u)
	assert.Equal(t, realIssuer, native.ExternalId.Issuer, "the first ExternalId is still the one used")

	ConvertIdentityStoreGroupToAWSGroup(sdkGroup("grp-1", "AWSAdmins", "GG1"))

	lines := entriesWithMsg(hook, ExternalIdsReadMsg)
	require.Len(t, lines, 2)
	assert.Equal(t, "user", lines[0].Data["kind"])
	assert.Equal(t, 2, lines[0].Data["count"])
	assert.Equal(t, []map[string]string{
		{"id": "G1", "issuer": realIssuer},
		{"id": "X9", "issuer": "other-idp"},
	}, lines[0].Data["externalIds"])
	assert.Equal(t, "group", lines[1].Data["kind"])
	assert.Equal(t, 1, lines[1].Data["count"])
}

func TestSplitIssuerArn(t *testing.T) {
	acct, rest, ok := splitIssuerArn(realIssuer)
	assert.True(t, ok)
	assert.Equal(t, "111111111111", acct)
	assert.Equal(t, "identitystore/d-1234567890/provisioningtenant/Dbv12345678-abcd-EF01", rest)
	_, _, ok = splitIssuerArn("not-an-arn")
	assert.False(t, ok)
}

// --- Records with more than one ExternalId ---

func multiExtUser(entries ...interfaces.ExternalId) *interfaces.User {
	u := issuerUser("", "")
	u.ExternalIds = entries
	u.ExternalId = entries[0] // what getExternalId (first entry) sets
	return u
}

func multiExtGroup(entries ...interfaces.ExternalId) *interfaces.Group {
	g := issuerGroup("", "")
	g.ExternalIds = entries
	g.ExternalId = entries[0]
	return g
}

var (
	otherIdpEntry  = interfaces.ExternalId{Id: "X9", Issuer: "arn:aws:identitystore::111111111111:identitystore/d-1234567890/provisioningtenant/OtherTenant"}
	thisTenantJane = interfaces.ExternalId{Id: "G1", Issuer: realIssuer}
)

func TestSelectExternalId(t *testing.T) {
	tests := []struct {
		name     string
		current  interfaces.ExternalId
		all      []interfaces.ExternalId
		expected string
		want     interfaces.ExternalId
		wantBy   string
	}{
		{"none", interfaces.ExternalId{}, nil, realIssuer, interfaces.ExternalId{}, externalIdSelectedNone},
		{"only (no list, e.g. test fixtures)", thisTenantJane, nil, realIssuer, thisTenantJane, externalIdSelectedOnly},
		{"only", thisTenantJane, []interfaces.ExternalId{thisTenantJane}, realIssuer, thisTenantJane, externalIdSelectedOnly},
		{"this tenant second", otherIdpEntry, []interfaces.ExternalId{otherIdpEntry, thisTenantJane}, realIssuer, thisTenantJane, externalIdSelectedIssuerMatch},
		{"this tenant first", thisTenantJane, []interfaces.ExternalId{thisTenantJane, otherIdpEntry}, realIssuer, thisTenantJane, externalIdSelectedIssuerMatch},
		{"none from this tenant", otherIdpEntry, []interfaces.ExternalId{otherIdpEntry, {Id: "Y", Issuer: "z"}}, realIssuer, otherIdpEntry, externalIdSelectedFirst},
		{"case-only issuer difference is still this tenant", otherIdpEntry, []interfaces.ExternalId{otherIdpEntry, thisTenantJane}, lowercaseIssuer, thisTenantJane, externalIdSelectedIssuerMatch},
		{"issuer check disabled", otherIdpEntry, []interfaces.ExternalId{otherIdpEntry, thisTenantJane}, "", otherIdpEntry, externalIdSelectedFirst},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, by := selectExternalId(tc.current, tc.all, tc.expected)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantBy, by)
		})
	}
}

// The case seen in the field: this tenant's ExternalId is not the first entry.
// Previously the first entry's issuer was compared and the user was foreign.
func TestGetUserOperation_MultipleExternalIds_UsesThisTenantsEntry(t *testing.T) {
	hook := captureDebug(t)
	u := multiExtUser(otherIdpEntry, thisTenantJane)

	action, _ := getUserOperation(u, googleJane("G1"), false, false, realIssuer)
	assert.Equal(t, Equal, action)
	assert.Equal(t, thisTenantJane, u.ExternalId)

	lines := entriesWithMsg(hook, IssuerCheckMsg)
	require.Len(t, lines, 1)
	assert.Equal(t, 2, lines[0].Data["externalIdCount"])
	assert.Equal(t, externalIdSelectedIssuerMatch, lines[0].Data["externalIdSelectedBy"])
	assert.Equal(t, "G1", lines[0].Data["externalId"])
	assert.Equal(t, true, lines[0].Data["issuerMatch"])
}

func TestGetUserOperation_MultipleExternalIds_NoneFromThisTenantIsForeign(t *testing.T) {
	u := multiExtUser(otherIdpEntry, interfaces.ExternalId{Id: "Y", Issuer: "another"})
	action, _ := getUserOperation(u, googleJane("G1"), false, false, realIssuer)
	assert.Equal(t, AdoptForeign, action)
}

func TestGetGroupOperation_MultipleExternalIds_UsesThisTenantsEntry(t *testing.T) {
	g := multiExtGroup(otherIdpEntry, interfaces.ExternalId{Id: "GG1", Issuer: realIssuer})
	action, _ := getGroupOperation(g, googleAdmins("GG1"), false, false, realIssuer)
	assert.Equal(t, Equal, action)
}

// The batch functions index by the selected entry, so a user/group whose
// tenant ExternalId is not first is still found by ExternalId (not only by
// username/display name) and is not reported as an extra AWS record.
func TestOperations_MultipleExternalIds_MatchedByExternalId(t *testing.T) {
	u := multiExtUser(otherIdpEntry, thisTenantJane)
	u.Username = "renamed@example.com" // only the ExternalId can match it
	_, del, upd, eq := getUserOperations([]*interfaces.User{u}, []*admin.User{googleJane("G1")}, false, false, realIssuer)
	assert.Empty(t, del)
	assert.Empty(t, eq)
	require.Len(t, upd, 1, "matched by this tenant's ExternalId and updated (username differs)")
	assert.Equal(t, "aws-1", upd[0].ID)

	g := multiExtGroup(otherIdpEntry, interfaces.ExternalId{Id: "GG1", Issuer: realIssuer})
	g.DisplayName = "Renamed"
	gAdd, gDel, gUpd, _ := getGroupOperations([]*interfaces.Group{g}, []*admin.Group{googleAdmins("GG1")}, false, false, realIssuer)
	assert.Empty(t, gAdd)
	assert.Empty(t, gDel)
	require.Len(t, gUpd, 1)
}

func TestConverters_KeepEveryExternalId(t *testing.T) {
	u := sdkUser("aws-1", "jane@example.com", "Jane", "Doe", "")
	u.ExternalIds = []types.ExternalId{
		{Id: aws.String("X9"), Issuer: aws.String("other")},
		{Id: aws.String("G1"), Issuer: aws.String(realIssuer)},
	}
	native := ConvertSdkUserObjToNative(u)
	assert.Equal(t, []interfaces.ExternalId{{Id: "X9", Issuer: "other"}, {Id: "G1", Issuer: realIssuer}}, native.ExternalIds)
	assert.Equal(t, interfaces.ExternalId{Id: "X9", Issuer: "other"}, native.ExternalId, "first entry until an operation selects")

	g := ConvertIdentityStoreGroupToAWSGroup(sdkGroup("grp-1", "AWSAdmins", "GG1"))
	assert.Len(t, g.ExternalIds, 1)
}

// ExternalIds never reaches the SCIM wire.
func TestExternalIds_NotSerialized(t *testing.T) {
	u := multiExtUser(otherIdpEntry, thisTenantJane)
	b, err := json.Marshal(u)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "OtherTenant")
	assert.NotContains(t, string(b), "externalIds")

	g := multiExtGroup(otherIdpEntry, thisTenantJane)
	b, err = json.Marshal(g)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "externalIds")
}
