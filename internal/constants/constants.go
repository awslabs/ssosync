// Package constants defines application-wide constants
package constants

import "net/http"

// SCIM Schema URNs
const (
	SCIMSchemaUser  = "urn:ietf:params:scim:schemas:core:2.0:User"
	SCIMSchemaGroup = "urn:ietf:params:scim:schemas:core:2.0:Group"
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
