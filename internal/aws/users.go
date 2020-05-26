package aws

import (
	"strings"
)

// NewUser creates a user object representing a user with the given
// details.
func NewUser(firstName string, lastName string, email string) *User {
	e := make([]UserEmail, 0)
	e = append(e, UserEmail{
		Value:   email,
		Type:    "work",
		Primary: true,
	})

	a := make([]UserAddress, 0)
	a = append(a, UserAddress{
		Type: "work",
	})

	return &User{
		Schemas:  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		Username: email,
		Name: struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		}{
			FamilyName: lastName,
			GivenName:  firstName,
		},
		DisplayName: strings.Join([]string{firstName, lastName}, " "),
		Active:      true,
		Emails:      e,
		Addresses:   a,
	}
}