// Package config ...
package config

import "errors"

// Config ...
type Config struct {
	// Verbose toggles the verbosity
	Debug bool
	// LogLevel is the level with with to log for this config
	LogLevel string `mapstructure:"log_level"`
	// LogFormat is the format that is used for logging
	LogFormat string `mapstructure:"log_format"`
	// GoogleCredentials ...
	GoogleCredentials string `mapstructure:"google_credentials"`
	// AttributeMapFile holds the path to a JSON file containing the attribute
	// map (groups sync method only), mirroring how GoogleCredentials holds the
	// credentials file path. Empty means use the built-in DefaultAttributeMap.
	AttributeMapFile string `mapstructure:"attribute_map_file"`
	// AttributeMap holds the loaded user/group attribute maps. It is populated
	// by the attribute-map loader (from AttributeMapFile for the CLI, or the
	// ATTRIBUTE_MAP env var for Lambda), not by viper, so it carries no
	// mapstructure tag.
	AttributeMap AttributeMaps
	// GoogleAdmin ...
	GoogleAdmin string `mapstructure:"google_admin"`
	// CustomerID is the Google Workspace customer ID to operate against.
	// Defaults to "my_customer", which means the customer associated with
	// the service account's domain.
	CustomerID string `mapstructure:"customer_id"`
	// UserMatch ...
	UserMatch string `mapstructure:"user_match"`
	// GroupFilter ...
	GroupMatch string `mapstructure:"group_match"`
	// SCIMEndpoint ....
	SCIMEndpoint string `mapstructure:"scim_endpoint"`
	// SCIMAccessToken ...
	SCIMAccessToken string `mapstructure:"scim_access_token"`
	// IsLambda ...
	IsLambda bool
	// IsLambdaRunningInCodePipeline ...
	IsLambdaRunningInCodePipeline bool
	// Ignore users ...
	IgnoreUsers []string `mapstructure:"ignore_users"`
	// Ignore groups ...
	IgnoreGroups []string `mapstructure:"ignore_groups"`
	// Include groups ...
	IncludeGroups []string `mapstructure:"include_groups"`
	// SyncMethod allow to defined the sync method used to get the user and groups from Google Workspace
	SyncMethod string `mapstructure:"sync_method"`
	// Region is the region that the identity store exists on
	Region string `mapstructure:"region"`
	// IdentityStoreID is the ID of the identity store
	IdentityStoreID string `mapstructure:"identity_store_id"`
	// ExtyernalId->Issuer
	IssuerArn string `mapstructure:"issuer_Arn"`
	// Precaching queries as a comma separated list of query strings
	PrecacheOrgUnits []string
	// Enabled Caching stats to help refine PrecacheOrgUnits configuration
	CacheMetrics bool
	// DryRun flag, when set to true, no change will be made in the Identity Store
	DryRun bool
	// sync suspended user, if true suspended user and their group memberships are sync'd into IAM Identity Center
	SyncSuspended bool
	// Force preservation of Identity Store UserId, when Google Workspace Directory UserId change but email addresses
	// still match. Default behaviour orginial user is deleted and a new user created to ensure excessive privileges
	// are not preserved. This option is required when switching from a different external IdP (Microsoft Entra for example)
	// to Google Workspace Directory (with ssosync) and assignments need to be preserved.
	ForceExternalIdUpdate bool
	// Blocks the deletion of 'local' user and groups that exist in the IAM Identity Store but not in the Google Workspace Directory,
	// a local user/group is defined as one where ExternalId is not set. If an ExternalId is set then, the will be deleted as normal.
	RetainUnmatched bool
	// User filter string
	UserFilter string
}

const (
	// DefaultLogLevel is the default logging level.
	DefaultLogLevel = "info"
	// DefaultLogFormat is the default format of the logger
	DefaultLogFormat = "text"
	// DefaultDebug is the default debug status.
	DefaultDebug = false
	// DefaultGoogleCredentials is the default credentials path
	DefaultGoogleCredentials = "credentials.json"
	// DefaultAttributeMapFile is the default attribute map file path.
	// Empty means no file is provided, so DefaultAttributeMap is used.
	DefaultAttributeMapFile = ""
	// DefaultGoogleAdmin is the default service account email address
	DefaultGoogleAdmin = ""
	// DefaultCustomerID is the default Google Workspace customer ID.
	DefaultCustomerID = "my_customer"
	// DefaultSyncMethod is the default sync method to use.
	DefaultUserMatch       = ""
	DefaultGroupMatch      = "*"
	DefaultSCIMEndpoint    = ""
	DefaultSCIMAccessToken = ""
	DefaultSyncMethod      = "groups"
	DefaultCacheMetrics    = false
	DefaultDryRun          = false
	DefaultSyncSuspended   = false
	// Default behaviour orginial user is deleted and a new user created to ensure excessive privileges
	// are not preserved.
	DefaultForceExternalIdUpdate = false
	DefaultRetainUnmatched       = false
)

// AttributeMap describes how a Google Workspace Directory source attribute is
// mapped onto an interfaces.User / interfaces.Group target field.
//
// Direction: the KEY is the interface TARGET field path (the field mapped TO on
// interfaces.User / interfaces.Group, e.g. "Name.GivenName" or "Username"); the
// VALUE is the Google SOURCE attribute path (the field mapped FROM on
// admin.User / admin.Group, e.g. "name.givenName" or "primaryEmail").
//
// Only standard Google attributes are supported this phase: custom-schema
// attributes (admin.User.CustomSchemas) are not resolved yet. See the deferred
// extension points documented on the mapper functions in internal/google.
type AttributeMap map[string]string

// AttributeMaps pairs the user and group attribute maps that together make up a
// loaded attribute map (groups sync method only).
//
// A single flat map is deliberately NOT used: the user target keys
// (e.g. "Name.GivenName") and group target keys (e.g. "DisplayName") are carried
// in two separate maps today, and the mapper functions (mapFromGoogleUser /
// mapFromGoogleGroup) depend on that user/group distinction. Flattening would
// discard it and be ambiguous to split back out in the loader, so the pair keeps
// the two maps distinct end to end.
type AttributeMaps struct {
	User  AttributeMap `mapstructure:"user" json:"user"`
	Group AttributeMap `mapstructure:"group" json:"group"`
}

// Slice-typed and map-typed defaults cannot be declared as constants in Go, so
// they are defined as package-level variables.
var (
	// DefaultIgnoreUsers is the default set of users to ignore.
	DefaultIgnoreUsers []string = nil
	// DefaultIgnoreGroups is the default set of groups to ignore.
	DefaultIgnoreGroups []string = nil
	// DefaultPrecacheOrgUnits is the default set of org units to precache.
	DefaultPrecacheOrgUnits []string = nil
	// DefaultIncludeGroups is the default set of groups to sync.
	DefaultIncludeGroups []string = nil

	// DefaultUserAttributeMap reproduces the currently implemented
	// Google -> interfaces.User field mappings for the groups sync method.
	// KEY = interfaces.User target field path; VALUE = admin.User source
	// attribute path.
	//
	// Note on "Active": it is the logical INVERSE of Google's "suspended" flag
	// (Active == !Suspended). The map stores the raw source path "suspended";
	// the MAPPER applies the inversion, not the map.
	DefaultUserAttributeMap AttributeMap = AttributeMap{
		"Name.GivenName":  "name.givenName",
		"Name.FamilyName": "name.familyName",
		"Username":        "primaryEmail",
		"Active":          "suspended",
	}

	// DefaultGroupAttributeMap reproduces the Google -> interfaces.Group field
	// mapping for the groups sync method. KEY = interfaces.Group target field
	// path; VALUE = admin.Group source attribute path.
	//
	// Note: the live sync path sets DisplayName from the group's email
	// (admin.Group.Email), whereas this default map resolves DisplayName from
	// admin.Group.Name. This is a deliberate, documented divergence while the
	// mapper sits alongside the live path and is not yet wired in; a future
	// wiring step reconciles it intentionally rather than by accident.
	DefaultGroupAttributeMap AttributeMap = AttributeMap{
		"DisplayName": "name",
	}

	// DefaultAttributeMap is the canonical combined default that the attribute
	// map loader falls back to when no file/env value is provided (groups sync
	// method only). It does NOT redefine any mapping; it only references the two
	// existing maps above, so DefaultUserAttributeMap / DefaultGroupAttributeMap
	// remain the single source of truth for the actual mappings and no existing
	// reference or test is affected.
	DefaultAttributeMap AttributeMaps = AttributeMaps{
		User:  DefaultUserAttributeMap,
		Group: DefaultGroupAttributeMap,
	}
)

// New returns a new Config
func New() *Config {
	return &Config{
		Debug:             DefaultDebug,
		LogLevel:          DefaultLogLevel,
		LogFormat:         DefaultLogFormat,
		SyncMethod:        DefaultSyncMethod,
		GoogleCredentials: DefaultGoogleCredentials,
		AttributeMapFile:  DefaultAttributeMapFile,
		AttributeMap:      DefaultAttributeMap,
		CustomerID:        DefaultCustomerID,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.GoogleAdmin == "" {
		return errors.New("google admin email is required")
	}

	if c.SCIMEndpoint == "" {
		return errors.New("SCIM endpoint is required")
	}

	if c.SCIMAccessToken == "" {
		return errors.New("SCIM access token is required")
	}

	if c.SyncMethod != "groups" && c.SyncMethod != "users_groups" {
		return errors.New("sync method must be either 'groups' or 'users_groups'")
	}

	return nil
}
