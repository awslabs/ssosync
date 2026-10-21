// Package constants defines application-wide constants
package constants

import "net/http"

// SCIM Schema URNs
const (
	SCIMSchemaUser  = "urn:ietf:params:scim:schemas:core:2.0:User"
	SCIMSchemaGroup = "urn:ietf:params:scim:schemas:core:2.0:Group"
	// SCIMSchemaEnterpriseUser is the URN of the SCIM Enterprise User
	// extension. It is used as the JSON key under which the optional
	// enterprise-extension attributes are carried on the SCIM wire. The
	// constructors do NOT inject it into a user's schemas array, so the
	// minimal-create wire output is unchanged; it is only relevant when the
	// enterprise extension object is populated.
	SCIMSchemaEnterpriseUser = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
)

// HTTP Status Codes
const (
	StatusConflict = http.StatusConflict // 409
)

// Content Types
const (
	ContentTypeSCIM = "application/scim+json"
)

// Unicode Characters
const (
	ZeroWidthSpace = '\u200B'
)
