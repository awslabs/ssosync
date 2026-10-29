package interfaces

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_Struct(t *testing.T) {
	user := User{
		ID:          "user-123",
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Username:    "test@example.com",
		ExternalId:  ExternalId{Id: "google-123"},
		DisplayName: "Test User",
		Active:      true,
		Emails: []UserEmail{
			{
				Value:   "test@example.com",
				Type:    "work",
				Primary: true,
			},
		},
		Addresses: []UserAddress{
			{
				Type: "work",
			},
		},
		Name: UserName{
			GivenName:  "Test",
			FamilyName: "User",
		},
	}

	assert.Equal(t, "user-123", user.ID)
	assert.Equal(t, "test@example.com", user.Username)
	assert.Equal(t, "google-123", user.ExternalId.Id)
	assert.Equal(t, "Test User", user.DisplayName)
	assert.True(t, user.Active)
	assert.Len(t, user.Emails, 1)
	assert.Equal(t, "test@example.com", user.Emails[0].Value)
	assert.Equal(t, "work", user.Emails[0].Type)
	assert.True(t, user.Emails[0].Primary)
	assert.Len(t, user.Addresses, 1)
	assert.Equal(t, "work", user.Addresses[0].Type)
	assert.Equal(t, "Test", user.Name.GivenName)
	assert.Equal(t, "User", user.Name.FamilyName)
}

func TestGroup_Struct(t *testing.T) {
	group := Group{
		ID:          "group-123",
		ExternalId:  ExternalId{Id: "google-123"},
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		DisplayName: "Test Group",
		Members:     []string{"user-1", "user-2"},
	}

	assert.Equal(t, "group-123", group.ID)
	assert.Equal(t, "google-123", group.ExternalId.Id)
	assert.Equal(t, "Test Group", group.DisplayName)
	assert.Len(t, group.Members, 2)
	assert.Contains(t, group.Members, "user-1")
	assert.Contains(t, group.Members, "user-2")
}

func TestUserEmail_Struct(t *testing.T) {
	email := UserEmail{
		Value:   "test@example.com",
		Type:    "work",
		Primary: true,
	}

	assert.Equal(t, "test@example.com", email.Value)
	assert.Equal(t, "work", email.Type)
	assert.True(t, email.Primary)
}

func TestUserAddress_Struct(t *testing.T) {
	address := UserAddress{
		Type: "work",
	}

	assert.Equal(t, "work", address.Type)
}

func TestUserFilterResults_Struct(t *testing.T) {
	results := UserFilterResults{
		TotalResults: 2,
		Resources: []User{
			{ID: "user-1", Username: "user1@example.com"},
			{ID: "user-2", Username: "user2@example.com"},
		},
	}

	assert.Equal(t, 2, results.TotalResults)
	assert.Len(t, results.Resources, 2)
	assert.Equal(t, "user-1", results.Resources[0].ID)
	assert.Equal(t, "user-2", results.Resources[1].ID)
}

func TestGroupFilterResults_Struct(t *testing.T) {
	results := GroupFilterResults{
		TotalResults: 2,
		Resources: []Group{
			{ID: "group-1", DisplayName: "Group 1"},
			{ID: "group-2", DisplayName: "Group 2"},
		},
	}

	assert.Equal(t, 2, results.TotalResults)
	assert.Len(t, results.Resources, 2)
	assert.Equal(t, "group-1", results.Resources[0].ID)
	assert.Equal(t, "group-2", results.Resources[1].ID)
}

// TestExternalId_MarshalJSON_BareString verifies that ExternalId serializes to
// a bare JSON string (its Id), preserving the SCIM wire format, and that an
// enclosing User/Group emits externalId as a plain string rather than an object.
func TestExternalId_MarshalJSON_BareString(t *testing.T) {
	b, err := json.Marshal(ExternalId{Id: "x", Issuer: "y"})
	assert.NoError(t, err)
	assert.Equal(t, `"x"`, string(b))

	ub, err := json.Marshal(User{ExternalId: ExternalId{Id: "x"}})
	assert.NoError(t, err)
	assert.Contains(t, string(ub), `"externalId":"x"`)
	assert.False(t, strings.Contains(string(ub), `"externalId":{`))

	gb, err := json.Marshal(Group{ExternalId: ExternalId{Id: "x"}})
	assert.NoError(t, err)
	assert.Contains(t, string(gb), `"externalId":"x"`)
	assert.False(t, strings.Contains(string(gb), `"externalId":{`))
}

// TestExternalId_MarshalJSON_EmptyOmitted verifies that an empty external id is
// omitted from the SCIM wire body entirely (matching the previous string
// field's omitempty behavior) rather than being emitted as "externalId":"".
func TestExternalId_MarshalJSON_EmptyOmitted(t *testing.T) {
	ub, err := json.Marshal(User{Username: "test@example.com"})
	assert.NoError(t, err)
	assert.NotContains(t, string(ub), `"externalId"`)

	gb, err := json.Marshal(Group{DisplayName: "Test Group"})
	assert.NoError(t, err)
	assert.NotContains(t, string(gb), `"externalId"`)
}

// TestExternalId_UnmarshalJSON_BareString verifies that a bare JSON string is
// read into the Id field, with Issuer left empty.
func TestExternalId_UnmarshalJSON_BareString(t *testing.T) {
	var e ExternalId
	err := json.Unmarshal([]byte(`"x"`), &e)
	assert.NoError(t, err)
	assert.Equal(t, "x", e.Id)
	assert.Equal(t, "", e.Issuer)
}

// minimalUser builds a User exactly the way aws.NewUser constructs a minimal
// user (name + single primary work email + single work address + active) so the
// test asserts the real minimal-create wire format.
func minimalUser() User {
	return User{
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Username: "a@b.com",
		Name: UserName{
			FamilyName: "B",
			GivenName:  "A",
		},
		DisplayName: "A B",
		Active:      true,
		Emails: []UserEmail{
			{Value: "a@b.com", Type: "work", Primary: true},
		},
		Addresses: []UserAddress{
			{Type: "work"},
		},
	}
}

// minimalGroup builds a Group the way aws.NewGroup constructs a minimal group
// (displayName + empty members slice).
func minimalGroup() Group {
	return Group{
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		DisplayName: "X",
		Members:     []string{},
	}
}

// TestUser_MarshalJSON_MinimalWireUnchanged asserts that a minimally-created
// user serializes byte-for-byte to the pre-change SCIM wire format, proving the
// schema expansion did not alter the minimal-create output.
func TestUser_MarshalJSON_MinimalWireUnchanged(t *testing.T) {
	want := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"a@b.com","name":{"familyName":"B","givenName":"A"},"displayName":"A B","active":true,"emails":[{"value":"a@b.com","type":"work","primary":true}],"addresses":[{"type":"work"}]}`

	b, err := json.Marshal(minimalUser())
	assert.NoError(t, err)
	assert.Equal(t, want, string(b))
}

// TestGroup_MarshalJSON_MinimalWireUnchanged asserts that a minimally-created
// group serializes byte-for-byte to the pre-change SCIM wire format.
func TestGroup_MarshalJSON_MinimalWireUnchanged(t *testing.T) {
	want := `{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"X","members":[]}`

	b, err := json.Marshal(minimalGroup())
	assert.NoError(t, err)
	assert.Equal(t, want, string(b))
}

// TestUser_MarshalJSON_FullyPopulated sets every new attribute and asserts that
// the expanded attributes (including the enterprise extension object keyed by
// its URN) appear on the SCIM wire.
func TestUser_MarshalJSON_FullyPopulated(t *testing.T) {
	u := User{
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Username: "a@b.com",
		Name: UserName{
			FamilyName:      "B",
			GivenName:       "A",
			Formatted:       "A B",
			MiddleName:      "M",
			HonorificPrefix: "Dr",
			HonorificSuffix: "PhD",
		},
		DisplayName: "A B",
		Active:      true,
		Emails: []UserEmail{
			{Value: "a@b.com", Type: "work", Primary: true},
		},
		Addresses: []UserAddress{
			{
				Type:          "work",
				Formatted:     "1 Main St",
				StreetAddress: "1 Main St",
				Locality:      "Town",
				Region:        "Region",
				PostalCode:    "12345",
				Country:       "US",
				Primary:       true,
			},
		},
		NickName:          "Al",
		ProfileURL:        "https://example.com/a",
		Title:             "Engineer",
		UserType:          "Employee",
		PreferredLanguage: "en-US",
		Locale:            "en-US",
		Timezone:          "America/New_York",
		PhoneNumbers: []UserPhoneNumber{
			{Value: "+1-555-0100", Type: "work", Primary: true},
		},
		Roles: []UserRole{
			{Value: "admin", Type: "role", Primary: true},
		},
		Enterprise: &EnterpriseUser{
			EmployeeNumber: "E123",
			CostCenter:     "CC1",
			Organization:   "Org",
			Division:       "Div",
			Department:     "Dept",
			Manager:        &EnterpriseManager{Value: "mgr-id"},
		},
	}

	b, err := json.Marshal(u)
	assert.NoError(t, err)
	s := string(b)

	assert.Contains(t, s, `"formatted":"A B"`)
	assert.Contains(t, s, `"middleName":"M"`)
	assert.Contains(t, s, `"honorificPrefix":"Dr"`)
	assert.Contains(t, s, `"honorificSuffix":"PhD"`)
	assert.Contains(t, s, `"streetAddress":"1 Main St"`)
	assert.Contains(t, s, `"locality":"Town"`)
	assert.Contains(t, s, `"region":"Region"`)
	assert.Contains(t, s, `"postalCode":"12345"`)
	assert.Contains(t, s, `"country":"US"`)
	assert.Contains(t, s, `"nickName":"Al"`)
	assert.Contains(t, s, `"profileUrl":"https://example.com/a"`)
	assert.Contains(t, s, `"title":"Engineer"`)
	assert.Contains(t, s, `"userType":"Employee"`)
	assert.Contains(t, s, `"preferredLanguage":"en-US"`)
	assert.Contains(t, s, `"locale":"en-US"`)
	assert.Contains(t, s, `"timezone":"America/New_York"`)
	assert.Contains(t, s, `"phoneNumbers":[{"value":"+1-555-0100","type":"work","primary":true}]`)
	assert.Contains(t, s, `"roles":[{"value":"admin","type":"role","primary":true}]`)
	assert.Contains(t, s, `"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User":{`)
	assert.Contains(t, s, `"employeeNumber":"E123"`)
	assert.Contains(t, s, `"manager":{"value":"mgr-id"}`)
}

// TestGroup_MarshalJSON_NoExtraAttributes confirms that the Group schema emits
// no additional attributes (for example a description) beyond the AWS SCIM
// supported set, even when the native group is fully built.
func TestGroup_MarshalJSON_NoExtraAttributes(t *testing.T) {
	g := Group{
		ID:          "group-123",
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		DisplayName: "Test Group",
		ExternalId:  ExternalId{Id: "ext-1"},
		Members:     []string{"user-1"},
	}

	b, err := json.Marshal(g)
	assert.NoError(t, err)
	s := string(b)

	assert.NotContains(t, s, `"description"`)
	assert.Contains(t, s, `"displayName":"Test Group"`)
	assert.Contains(t, s, `"externalId":"ext-1"`)
}
