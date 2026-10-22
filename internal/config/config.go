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
// Standard Google attributes are supported, including multi-valued fields
// resolved by a type-qualified selector (e.g. "organizations[type=work].title")
// or a primary selector (e.g. "emails[primary].address",
// "relations[type=manager].value"); in both cases there is no first-entry
// fallback, so a selector that matches nothing resolves to nothing.
// Custom-schema attributes are also supported via admin.User.CustomSchemas, as
// "customSchemas.<SchemaName>.<fieldName>" paths (populated by the Users.List
// projection=full fetch). See the mapper functions in internal/google for the
// full source-path grammar.
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

	// DefaultUserAttributeMap is the default Google -> interfaces.User field
	// mapping for the groups sync method. KEY = interfaces.User target field
	// path; VALUE = admin.User source attribute path.
	//
	// It carries the four behaviour-preserving scalars (GivenName, FamilyName,
	// Username, Active) plus the SAFE SET enrichment (Title and the
	// Enterprise.Department/Organization/CostCenter fields) sourced from the
	// user's WORK-typed organization. These safe-set fields are both resolvable
	// from Google and accepted by AWS IAM Identity Center SCIM, so defaulting
	// them enriches synced users; a user with no matching organization keeps them
	// empty (the resolver is empty-safe and Enterprise stays nil), so steady
	// state is churn-free (attrmap-design §5, §8).
	//
	// BEHAVIOUR CHANGE (design §7): the org-sourced entries changed from
	// organizations[primary].* to organizations[type=work].*. The [type=work]
	// selector matches the FIRST organization whose Google type == "work"
	// (case-insensitive) and IGNORES the primary flag; with the no-first-fallback
	// rule, a user whose work/primary org is typed something other than "work"
	// (empty type, "other", a custom label) now resolves to NOTHING for all four
	// fields. Under clear-to-consistent that CLEARS a previously-populated Title
	// and Enterprise.Department/Organization/CostCenter (and Enterprise drops to
	// nil if it holds no other data) on the first run after upgrade.
	//
	// MITIGATION: run with --dry-run after upgrading and inspect the planned
	// Updates BEFORE a live sync. A tenant that wants the old behaviour sets its
	// own attribute map with organizations[primary].* (still fully supported).
	//
	// Note on "Active": it is the logical INVERSE of Google's "suspended" flag
	// (Active == !Suspended). The map stores the raw source path "suspended";
	// the MAPPER applies the inversion, not the map.
	DefaultUserAttributeMap AttributeMap = AttributeMap{
		"Name.GivenName":          "name.givenName",
		"Name.FamilyName":         "name.familyName",
		"Username":                "primaryEmail",
		"Active":                  "suspended",
		"Title":                   "organizations[type=work].title",
		"Enterprise.Department":   "organizations[type=work].department",
		"Enterprise.Organization": "organizations[type=work].name",
		"Enterprise.CostCenter":   "organizations[type=work].costCenter",
		// NOTE: Enterprise.Manager is deliberately NOT defaulted (opt-in only,
		// see attrmap-design §5.7): Google's relation value is a free-form name,
		// not a guaranteed AWS-resolvable manager.value reference. The resolver
		// and validator support "relations[type=manager].value" for tenants that
		// opt in, but it is left out of the default to avoid stamping a
		// possibly-garbage manager reference onto every tenant.
	}

	// DefaultGroupAttributeMap reproduces the Google -> interfaces.Group field
	// mapping for the groups sync method. KEY = interfaces.Group target field
	// path; VALUE = admin.Group source attribute path.
	//
	// Note: DisplayName maps from the group name (DisplayName <- name, i.e.
	// admin.Group.Name), which matches the live groups-sync path
	// (aws.NewGroup/aws.UpdateGroup use the group name). There is no divergence.
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
