## Unreleased
- `PrecacheOrgUnits` (`--precache-ous`) now precaches only the users in the listed OUs and their sub-OUs (#404). The combined query started with a comma, so ssosync also fetched the whole directory on every run, whatever OUs were listed (the default `/` fetched it twice). Each OU is now queried on its own: an OU that cannot be queried, or has no users matching the filter, is skipped with a warning naming it (`PrecacheOrgUnits: OrgUnitPath could not be queried …` / `PrecacheOrgUnits: no users found in OrgUnitPath …`), and the other OUs are still precached; before, one failing OU disabled user precaching entirely. `Precaching failed, caching on the fly` is logged only when every OU fails. Deployments that list OUs to narrow the precache now precache fewer users, and look up members outside those OUs one at a time; with `--cache-metrics` the end-of-run counts cover only OUs of users the run saw.
- With `--cache-metrics`, a user that overlapping `PrecacheOrgUnits` entries (such as `/` and `/Sales`) return more than once is counted once as a precache and then as a cache hit, instead of a precache every time (#404). The duplicate check looked at a map that is still empty during precaching. Only the metrics change; the user was already cached once.
- With `--cache-metrics`, the end-of-run `Cache Stats` count each precache, hit and miss once (#413). Each was counted at least twice, and also credited to any OU whose path merely appeared as text in the user's OU path (`/Sales` for a user in `/Sales Team` or `/Presales`), but only to OUs already seen at that point, so totals depended on order. Each OU now reports itself and its sub-OUs, matched by path, the same tree that `TotalUsers` and `--precache-ous` cover. Sync results are unchanged.
- With `--cache-metrics`, a group member that has to be looked up in Google counts as a miss only, no longer as both a miss and a hit (#413); a hit now always means a lookup the cache avoided.
- With `--cache-metrics`, the end-of-run `TotalUsers` counts no longer download every user in each OU (#416). An OU that `--precache-ous` already listed reuses that count, and any other OU is counted with a query that returns only user ids, 500 per page; for `/` this was a full download of the directory, made twice when `/` was also precached. The counts are unchanged.

- The developer and release test pipelines no longer delete `TestAccountExecution` between the attribute-map and functional tests (`CleanUp-AttributeMap` is gone): `Deploy-Functional` updates the stack in place, back to the default attribute map (#409). Deleting and recreating it reused the role name `SSOSync_ExecutionRole` within seconds, and Lambda could fail to assume the not-yet-propagated role ("The role defined for the function cannot be assumed by Lambda").
- The developer and release test pipelines deploy `TestAccountExecution` and `QuickStartTests` with `REPLACE_ON_FAILURE` instead of `CREATE_UPDATE` (#409). A stack that a failed run left in a failed state (such as `ROLLBACK_COMPLETE`) is now deleted and recreated, so a retry no longer needs it deleted by hand first; a healthy stack is still updated in place, so the in-place updates (attribute-map to functional, classic to next-gen quick-starts) are unchanged.
- The release test pipelines' CodePipeline role has `cloudformation:DescribeStackEvents`, as the developer pipeline's already does, so a failed CloudFormation action shows CloudFormation's reason in CodePipeline instead of "Status reason is not available" (#409).

## v2.11.0

### Highlights

- **Keyless Google authentication with Workload Identity Federation (#345).** ssosync can now authenticate to Google Workspace without a downloaded service-account key: Google Cloud trusts the AWS IAM role ssosync runs as, and every run exchanges that role's credentials for short-lived Google credentials to impersonate the service account. Nothing long-lived is stored, and it works where the `iam.disableServiceAccountKeyCreation` organization policy forbids keys. Select it with `--google-impersonate-service-account` (`GOOGLE_IMPERSONATE_SERVICE_ACCOUNT`, template `GoogleImpersonateServiceAccount`) and pass the Workload Identity Federation credential configuration with `--google-wif-config` (`GOOGLE_WIF_CONFIG`, template `GoogleWIFConfig`; Lambda accepts inline JSON, a Secrets Manager ARN or an SSM Parameter Store ARN). Keyless authentication is explicit and is now the recommended method; see the README section "Workload Identity Federation (keyless)" for step-by-step Cloud Shell instructions. Builds on the work in #347 by Sami Jawhar.
- **No super admin needed (#192).** The Google admin is now optional. Without `--google-admin` (`GOOGLE_ADMIN`, template `GoogleAdminEmail`) the service account calls the Admin SDK as itself, using a read-only Workspace admin role assigned to it, so there is no domain-wide delegation and no admin to act on behalf of. With an admin, domain-wide delegation works as before; this applies to legacy (key) authentication only, as keyless authentication never acts on behalf of an admin and rejects `--google-admin`.
- **Least-privilege Google scopes (#322).** ssosync now requests only `admin.directory.group.readonly` and `admin.directory.user.readonly`. Group memberships (`members.list`) are read with `admin.directory.group.readonly`, so `admin.directory.group.member.readonly` is no longer requested; for legacy (domain-wide delegation) setups it can be removed from the delegation entry, and an entry that still lists it keeps working. **Upgrade ssosync first, then trim the entry:** with domain-wide delegation Google refuses the token if any requested scope is not authorized, so earlier versions (which still request it) fail with `401 unauthorized_client` against a trimmed entry. Keyless authentication has no delegation entry and is unaffected.
- **Two Serverless applications (#399).** 2.11.0 is published as two SAR applications from the same code and with the same version number:
  - **`SSOSync` (classic)** keeps the 2.10.0 parameters (stored key, Google admin, both sync methods), with their patterns widened to accept apostrophes (#401), so existing deployments, and **Sync from Git** deployments of `quick-start/single-account.yaml` and `single-account-attribute-map.yaml`, upgrade to 2.11.0 with no change. It gains the 2.11.0 code (for example the #322 scopes) but not keyless authentication.
  - **`ssosync` (next generation)** is keyless only (`GoogleImpersonateServiceAccount`, `GoogleWIFConfig`) and `groups` only, requires the actual `GoogleCustomerId`, and drops `GoogleCredentials`, `GoogleAdminEmail`, `SyncMethod` and `IncludeGroups`, as well as the deprecated, unused `Region` and `IdentityStoreID` (and their "Deprecated Parameters" group); the classic app and `template.yaml` keep them until v3.x (#406). Its fixed resource names are prefixed `SSOSyncNG` (secrets, the `SSOSyncNGSchedule` rule, `alias/SSOSyncNG`, exports such as `SSOSyncNGFunctionARN`), so a stack can switch from the classic app in place. New features are added here.
  - Both are generated from `template.yaml` by patches in `cicd/build/package/` (`make sar-variants` writes them to `dist/sar/`); `SAR.md` and `SAR-next-gen.md` are their descriptions. The CLI is unchanged.
- **`RoleName` parameter and output (next-gen app).** The function's IAM role can be given a fixed name (for example `SSOSync_ExecutionRole`) so a Google workload identity provider can trust it across redeployments; the `RoleName` output shows the role's name, generated or not.
- **Next-gen quick-start templates.** `quick-start/next-gen/single-account.yaml` and `quick-start/next-gen/single-account-attribute-map.yaml` (release 2.11.0 or later) deploy the `ssosync` app (function `SSOSyncNG`) with keyless authentication and no admin; their `RoleName` defaults to `SSOSync_ExecutionRole`. To move a classic quick-start deployment over, set up Workload Identity Federation, then update the stack with the matching next-gen template. The classic templates in `quick-start/` keep their 2.10.0 parameters, with the same widened patterns.

### Fixes

- **`ssosync --version` reported `dev, commit none, built at unknown by unknown`.** The build's linker flags named the package `github.com/awslabs/ssosync/cmd`, but the module is `ssosync`, so the version, commit and build date were never stamped into the binaries (the Go linker silently ignores an unknown symbol). Binaries now report their release version (snapshot builds: `<next patch>-<commit>`), full commit and build date.
- **Group members with an apostrophe in their email no longer abort the sync (#401).** The `groups` sync method looked up each group member with an unquoted `email=<address>` query, so an address such as `jane.o'brien@example.com` made Google reject the query (`400 Invalid Input: query, invalid`) and the whole run failed. The value is now single-quoted with `'` escaped as `\'`, as Google's search syntax requires. The `PrecacheOrgUnits` and cache-metrics `OrgUnitPath` queries use the same quoting, so OU paths with an apostrophe work too (an OU path containing a comma is still split into separate queries). A failed member or user lookup still fails the run, but the error log now names the member, the group and the exact query (with the filter) along with the Google error; the AWS-side per-user lookups that abort the run also log the user, group and error. `--group-match` / `--user-match` (`GoogleGroupMatch` / `GoogleUserMatch`) are still passed to Google verbatim; the README, SAR.md and SAR-next-gen.md now explain how to quote values in them.
- **Template parameters accept apostrophes (#401).** In both Serverless applications and all four quick-start templates, `PrecacheOrgUnits` accepts an OU path with an apostrophe (for example `/Sales/Petersen's Team`), and `IgnoreUsers`, `IgnoreGroups` and `IncludeGroups` accept an email address with one. `GoogleUserMatch` and `GoogleGroupMatch` accept single-quoted values with `\'` escapes (`name='Valentine\'s Day'`, `email='jane.o\'brien@example.com'`, and for groups `memberKey='...'`), the form Google requires; an unquoted apostrophe is still rejected. `PrecacheOrgUnits` also drops its limits of two OU paths and 50 characters per path: it takes an empty value, `/`, or any number of comma-separated OU paths, up to CloudFormation's 4096-byte parameter limit. Values that were accepted before still are.
- **The version is logged at the start of every run.** ssosync logs `ssosync version` at info level, with the version, commit, build date and builder, before it logs its configuration (CLI and Lambda), so logs attached to bug reports show the build.
- **The Serverless Application Repository pages render properly (#406).** SAR renders an application's README as basic Markdown only, so the tables and GitHub alerts (`> [!NOTE]` and so on) in `SAR.md` and `SAR-next-gen.md` displayed as raw text. The tables are now lists and the alerts are plain blockquotes with a bold label (`> **Note:** ...`); the content is unchanged. `make sar-readmes`, run by `make test` and `make sar-variants`, fails if either file gains a table row or an alert marker again.
- **The CLI smoke test no longer prints the Google service-account key to the build log.** `cicd/tests/account_execution/cli/buildspec.yml` wrote the key to `credentials.json` and then ran `cat credentials.json`, and that value is not masked by CodeBuild. The print is removed, and the two CLI test buildspecs that hold the SCIM token no longer dump their environment (`env`).

### Breaking / behavioural changes

> [!IMPORTANT]
> **A customer ID is required for keyless authentication, and whenever no Google admin is given.** The `my_customer` alias only resolves against the admin being acted on behalf of, so with `--google-impersonate-service-account`, or with `--google-admin` empty, ssosync and the next-gen `ssosync` app now require the actual customer ID (`--customer-id` / `CUSTOMER_ID` / `GoogleCustomerId`, for example `C01abcd23`) and reject `my_customer`. Legacy deployments with an admin, including the classic `SSOSync` app, are unaffected.

> [!NOTE]
> **The `quick-start/wif-auth/` and `quick-start/legacy-auth/` templates and the `AuthMethod` quick-start parameter, which were on `master` during 2.11.0 development but never released, are removed.** Use `quick-start/next-gen/` for keyless authentication and the classic `quick-start/` templates for a stored key.

> [!NOTE]
> Deleting and recreating the Google service account removes both its Workload Identity User grant and its Workspace admin role assignment, even when the email is unchanged; redo both. Admin role assignments can take a while to take effect.

### Pipelines and tests

- The developer and release test pipelines run keyless authentication first ("fail fast"), then legacy authentication, the attribute-map, functional and quick-start tests, all in one sequential `Tests` stage, so overlapping runs no longer interfere. Every test except the legacy-auth block now uses keyless authentication, against the next-gen staging app (`ssosync-staging`, `SSOSyncNG*` secrets); the legacy-auth block deploys the classic one (`SSOSync-Staging`). Packaging builds and publishes both staging apps. The quick-start templates are tested through CloudFormation pipeline actions on one stack, updated from the classic templates to the next-gen ones (the classic to next-gen upgrade); a generated wrapper passes their secrets as Secrets Manager dynamic references, so no secret value is written to the pipeline artifacts. The release staging accounts each use two Google providers, `<AccountId>-lambda` (trusting `SSOSync_ExecutionRole`) and `<AccountId>-cli` (trusting `SSOSync_CLIExecutionRole`).
- `cicd/cloudformation/secrets.yaml` holds the keyless test configuration (`GoogleImpersonateServiceAccount`, `GoogleWIFConfig`) and no longer the unused `IdentityStoreId`.
- The developer and release test pipelines delete a leftover `QuickStartTests` stack (`PreClean-QuickStart`, the first action of the `Tests` stage) before any test deploy. A cancelled run could leave that stack behind, and its classic Secrets nested stack owns the `GoogleCustomerId` export, which made the next run's `Deploy-LegacyAuth` fail. When there is no leftover stack the action succeeds at once.
- The test user Olaf has an apostrophe in his email and OU (#401); the attribute-map CLI suites and the staging Lambdas (`PrecacheOrgUnits`) query them.
- SAR packaging applies every patch in `cicd/build/package/` with no fuzz (`patch -N -F 0`), in both the packaging buildspec and `make sar-variants`, so a patch whose context has drifted fails the build instead of applying near the right place. The classic name patches, which had drifted and only applied with fuzz, are regenerated against the classic feature variant (as `next-gen-staging.patch` is against the next-gen one) and renamed `legacy-staging.patch` and `legacy-release.patch`. How to regenerate each kind of patch is described above `sar-variants` in the Makefile. The published templates are unchanged. Both also check that each staging template differs from its release template only in the SAR `Name` line, and fail otherwise.
- The staging-versus-release check is `diff -I '^    Name:'` on each pair, plus a check that each template has exactly one `    Name:` line, instead of a regex over `diff`'s output. It no longer hides a `diff` error (exit status 2) or leaves `dist/sar/*-names.diff` behind.
- The packaging buildspec builds the four SAR templates with `make sar-variants` instead of its own copy of the patch steps, then `sam package`s them under the same `packaged-*.yaml` names; a patch that does not apply prints its rejects. `make ci`, which GitHub CI runs, now includes `sar-variants`, so a `template.yaml` change that breaks a patch fails there too.
- The Lambda smoke and keyless Lambda tests invoke the function once (`AWS_MAX_ATTEMPTS=1`, `--cli-read-timeout 900`, `--cli-connect-timeout 30`). With the CLI's 60-second read timeout and default retries, a sync slower than a minute was invoked a second time.
- The staging build fails, naming the cause, when the keyless test configuration (the Workload Identity Federation credential configuration and service account in `SecretsConfig`, or the `PipelineGoogleCustomerId` secret) is missing, or when reading it or writing the keyless test files fails (`set -eo pipefail`). It used to warn and carry on, and the failure only showed later as a missing `deploy/wif-<AccountId>.json`. Unset `SecretsConfig` entries (`""`) are normalised in one place, the staging buildspec.
- The staging build rewrites the provider at the end of the keyless credential configuration's `audience` once, with `jq` (`.audience |= sub(...)`), instead of `sed` on the raw JSON in the buildspec and a separate regex in `tests/quick_start/build.py`, which now reads the per-account files the buildspec writes. The values produced are unchanged.
- `make test-cicd` runs every `cicd/tests` Python helper unit test (`python3 -m unittest discover`, one per directory), and `make ci` runs it, so GitHub CI does too. Three of the five never ran anywhere; the other two ran only inside live CodeBuild projects (the user and group operations test and the staging build), which no longer run them. The staging build no longer installs PyYAML, which only that test used, and the attribute-map tests pin `boto3` (`1.44.0`).
- The log assertions of the user and group operations test (`check_log.py`) and the attribute-map tests (`assert_log.py`) share one module, `cicd/tests/lib/ssolog.py`, instead of two copies of the same loader, counters and `converged` check. Both scripts keep their commands and become thin front ends to it, so both suites now parse logrus text-format lines, and the user and group operations `converged` check also requires no group member changes. Their unit tests are merged into `cicd/tests/lib/test_ssolog.py`.
- The build and GoReleaser fetch Go modules through the Go module proxy (`GOPROXY=https://proxy.golang.org,direct`), falling back to direct, so an outage at a module's origin (for example gopkg.in) no longer fails the build. Checksum verification (`GOSUMDB`) is unchanged.

## v2.10.0

### Highlights

- **Configurable attribute mapping (groups sync method).** You can now control how Google Workspace user attributes are mapped onto AWS IAM Identity Center (SCIM) user fields via a JSON attribute map. Supply it with the new `--attribute-map-file` CLI flag or, for Lambda, the `ATTRIBUTE_MAP` environment variable (which accepts a Secrets Manager secret ARN, an SSM Parameter Store ARN, or the inline JSON, mirroring the credentials pattern). When unset, a built-in default mapping is used. See the README "Attribute mapping" section for the syntax and the copy-pasteable default.
- Expanded the internal user/group model to the full set of IAM Identity Center SCIM attributes (full name, nickname, profile URL, title, user type, preferred language, locale, timezone, phone numbers, addresses, and the enterprise extension: organization, department, cost center, division, manager, employee number). Groups remain `displayName`-only, as AWS SCIM accepts only `externalId`/`displayName`/`members` for groups.
- **Source-path grammar.** Attribute-map source values support scalar paths, and multi-valued Google fields (organizations, relations, emails, phone numbers, addresses, websites) support both `[type=<value>]` (case-insensitive type match) and `[primary]` (entry marked primary) selection. In both cases there is **no positional/first-match fallback** — if nothing matches, the field resolves to nothing, so results are stable run-to-run.
- **Google custom (user-defined) schema sources.** A map value may reference a Google custom-schema field with `customSchemas.<SchemaName>.<fieldName>` and route it to a supported core/enterprise SCIM target. Only single-valued custom fields are supported: a multi-valued custom attribute (a custom source with a `[type=<value>]` selector, or a mapped custom field that is multi-valued on a user) logs a warning and that mapping is ignored. Support may be added later. A referenced custom schema/field that is absent logs a warning and resolves to nothing. (SCIM targets stay closed to core + enterprise — AWS does not accept arbitrary custom extension schemas as targets; groups have no custom schemas.) To make custom-schema data available, users are now fetched with Google's `full` projection.
- **Default map** populates `Title`, `Enterprise.Organization`, `Enterprise.Department`, and `Enterprise.CostCenter` from the user's **primary** organization (`organizations[primary]`); the Google Admin console marks an organization primary but does not set its type. Additional attributes (`Enterprise.Manager`, `Enterprise.Division`, `Enterprise.EmployeeNumber`, `ProfileURL`, single-element emails/addresses/phone numbers) are supported but opt-in. `Enterprise.Manager` maps the Google manager relation, which holds the manager's email address; it works where the Identity Store can resolve that email, and is opt-in because it depends on the directory populating the relation.
- **Diagnostics for authoring maps.** Debug-level logging traces, per user, the raw Google source fields, the per-entry resolution, and the produced object — so a mapped value can be compared against the attribute map. A pre-SCIM validation pass logs a **warning** (never fails) when a mapped user/group would likely be rejected by the SCIM API (a required field empty, a multi-valued field with more than one value, a non-primary email, or `userName`/group `displayName` over length or containing disallowed control characters); the object is still sent and AWS remains the authoritative decision.
- Attribute maps are validated at load time; an unsupported target field, unsupported source attribute, or empty key/value causes a hard failure with a detailed, actionable error listing the supported values.

### Fixes

- **Lambda: warm invocations failed when IAM Identity Center is in another region.** The function read its own secrets and SSM parameters (and called CodePipeline) using the Identity Center region taken from the SCIM endpoint. On a cold start that region was not yet known, so the function's region was used and the run succeeded; on every warm invocation the secrets were looked up in the Identity Center region and the run failed with `cannot read secret ... ResourceNotFoundException`. These clients now always use the function's own region.

### Breaking / behavioural changes

> [!IMPORTANT]
> **Clear-to-consistent user sync.** The `groups` sync method now makes the IAM Identity Store user fully consistent with the mapped Google user. Any supported SCIM attribute that the attribute map does **not** populate is **cleared** on the Identity Store user. On the first run after upgrading, user attributes that were previously left untouched (for example Title, phone numbers, addresses, or locale set outside of ssosync) may be cleared unless they are covered by your attribute map. **Run with `--dry-run` first** to review the changes before a live sync.

> [!IMPORTANT]
> **Validate attribute maps with `--dry-run` first.** Because of clear-to-consistent behaviour, an error in a custom attribute map (a typo, a `[type=X]` selector that matches nothing, a missing custom field, or an ignored multi-valued custom field) can **clear** real attributes on a live run. Always test a new or changed attribute map with `--dry-run` — together with debug logging and the pre-SCIM validation warnings — before running it live.

> [!NOTE]
> The default attribute map reads organization-derived fields (Title, Organization, Department, Cost Center) from the user's **primary** organization (`organizations[primary]`). A user with no organization marked primary will have these fields left empty (and therefore cleared). Confirm with a `--dry-run` against your directory that these fields resolve as expected before a live run.

## v2.9.5 - 2026-10-28

### Changelog

* 8dc14316b82b736e05fc2397b6f63bd2235538ac Update CHANGELOG and Quick Start templates (#386)
* 6a883116c24eaed35e446fef3aa2f39d8f65251b build: bump Go to 1.27.3 and update vulnerable dependencies (#387) (#389)

## v2.9.4 - 2026-10-28

### Changelog

* 4d5206c32c3e3d214648218f20c0faf4d07890e1 Create git.md
* c14b2416d8e7bfaa8c804daad18d33646eed69d8 Fix IssuerArn matching from any account and mixed-case tenants (#383) (#384)
* e21d1fc9b090e0172490250fd7a695d2080ea00a Update .gitignore
* 0661d145bc99324bc6704114e283055d20982893 Update CHANGELOG and Quick Start templates (#382)

## v2.9.3 - 2026-10-21

### Changelog

* a65b6fb7f26767dc7d7700bc2886de1d937e123b 358 where a user externalid does not match the user in google workspaces improve the behavior (#377)
* ee8fa280ef03cc51ee428f2c81862b702d72271c Update CHANGELOG and Quick Start templates (#380)
* 74b3d653b6559e44c1a144eee7c51fc55c5df4d9 Update release.yml
* 3088682d37b0f7e662108c16af802fefe6992867 refactor: rename sync entrypoints to clarify modern vs legacy (#381)

## v2.9.2 - 2026-10-19

### Changelog

* f6b4b2b4b1ceb832ba681e9c92b10f88e2dc7464 Update CHANGELOG and Quick Start templates (#376)
* bf9a4801b7881e7d1ea6ef55192bb2aa8f9503e0 username/email address and group DisplayName uses case sensitive comparison (#379)

## v2.9.1 - 2026-10-16

### Changelog

* 7b77c60c4021cd455aea2241e6a595d24f173a00 Address nil pointer handling in ConvertSdkUserObjToNative
* e1d7833e8ffce759855a3e467fce0ea1edbfa1cf Update release.yml
* 001a524276571bba7befae10015e7913b80106e3 Update single-account.yaml
* 48d970e03deea1521d3bdba7de6ac6873a61fe7f Updates to address CVEs (#372)

## v2.7.0 Latest

- Adjust behavior for users that mis-matched ExternalId (#361)
- Update dependancies (#359)
- fix: log group id/displayName values, not pointers, in ConvertIdentityStoreGroupToAWSGroup

## v2.6.3

- 353 changing a users primary email in google workspace permanently aborts every sync (#355)

## v2.6.2

- 341 go toolchain version 1240 results in numerous cves reported by inspector (#342)
- Avoid goreleaser build clashes
- Fix arn handling for none commercial regions (#343)
- update workflows for node24

## v2.6.1

- avoid re-adding (#334)

## v2.6.0

- Add cachestats (#332)
- Rename feature to CacheMetrics

## v2.5.2

- BugFix for #329 v25x updates every user and group on every run externalid is never read back from the identity store (#330)
- BugFix Cross-stack parameter passing in quick-start: single-account.yaml

## v2.5.1

- Correcting error in Outputs Ids

## v2.5.0

- 152 remove unnecessary parameters (#327)
- 324 all createupdatedelete actions via SCIM api (#326)
- Adding CustomerId to template
- Adding integration with SAR pipeline
- CICD updates because should now work in non-delegated
- Fix quickstart workflow job
- Updates to ReadMe and CICD pipelines
- feat: configurable Google Workspace customer ID (#233)
- update CICD templates

## v2.4.0

Changelog

- 309 precache field regex does not all for disabled (#312)
- Configurable LogRetention and QuickStart, plus bugfixes (#316)
- BugFix Release
- Update Pipeline to type: V2
- Update README.md
- Update release.yml
- Update workflows
- fixes

## v2.3.8

- Bump go.opentelemetry.io/otel from 1.39.0 to 1.41.0
- Update template.yaml

## v2.3.7

- 299 scheduleexpression allowedpattern rejects valid singular rate expressions (#303)
- Bump golang.org/x/crypto from 0.40.0 to 0.45.0
- Bump google.golang.org/grpc from 1.74.2 to 1.79.3
- Delete users if inactive only (#304)
- Update runtime to provided.al2023
- fix: Use config.DefaultSyncMethod as the default for cfg.SyncMethod

## v2.3.6

- update regex tennant id has changed length

## v2.3.5

- 278 intermittent 503 from google workspace deletes users and its aliases in aws without retrying (#279)
- ci: scope down GitHub Token permissions (#277)
- fix inaccurate comments (#216)

## v2.3.4

- Add handling for an email alias for a user (#276)

## v2.3.2

- BugFix Release (#270)

## v2.3.2

- BugFix Release (#263)
- Upgrade to AWS SDK v2 for go1.24 (#255)

## v2.3.1

- Lambda Function: Addition of Dry-Run, syncing of Suspended Users and Customer Precaching scope (#254)

## v2.3.0

- Add a --dry-run feature
- Add configuration options: RAM allocation (default to 128MB), Precaching (#249)
- Adding Config option to Lambda
- Bump dependencies to allow nix builds (#246)
- Update README.md typo (#242)
- Update identitystore_dry.go
- Update root.go

## v2.2.9

- 227 problem with googlegroupmatch (#238)
- 234 group sync may silently fail on 503 response from google admin api (#237)
- Update README.md

## v2.2.8

- 224 scimendpointurl with anoother pattern (#225)
- SCIM url regex (#223)

## v2.2.7

- Adding concurrency limits to prevent potential race condition or overlapping execution.
- updates to dependancies for GitHub workflows
- Extend allowed SCIM access token length
- Improve ReadMe readability

## v2.2.6

- 194 sso lambda deletes then recreates users (#203)
- 199 group flattening can lead to conflicts due to non uniqueness (#201)
- 200 name handling (#204)
- Update release.yml

## v2.2.5

- Updating guidance for Match parameters.

## v2.2.4

- Group owners are treated as members (#190)
- Group owners are treated as members (#191)
- Update .gitignore

## v2.2.3

- Group owners are treated as members
- Update .gitignore

## v2.2.2

- Fix nested groups (#188)

## v2.2.1

- Update README.md

## v2.2.0

- Feature multi select (#176)

## v2.1.4

- Adjusting params for AccountExecution Tests.
- Bugfix env vars (#175)
- Update buildspec.yml
- Update testing.yaml

## v2.1.3

- Bugfix ignore regexes (#172)
- Bugfix improve connection test (#174)
- Increase logging level in Account_Execution tests
- Update secrets.yaml

## v2.1.2

- Escape hyphens in user/group character classes
- Update README.md

## v2.1.1

- Bugfix: support for omitting values in template

## 2/0/3

- Fix: A case where the user already exists but it tries to create again by @bkmeneguello in #77
- Add missing environment variable to README by @waigel in #125

## 2.0.2

- Fixes panic on IdentityStore user without primary email (common to users created by AWS Control Tower)
- Fixes panic on Google Directory Group with external users with group be synced

## 2.0.1

- Fixes m,issing IAM permission identityStore:DeleteGroup
- Updates to developer CICD pipeline

## 2.0.0

- Introduced the use of the IdentityStore api to overcome various scaling challenge
- Improvements to CICD to allow for testing in different accounts type within an AWS organization
- Strong recommendation to deploy in IAM Identity Center - delegated administration account

## 1.1.0

- Added Cloudformation deployable CICD pipelines
- To consistently build and test the application

## 1.0.0-rc.10

- #44 fix: ensure old behaviour is supported
- #43 fix: fix ignore-group flag

## 1.0.0-rc.9

- #16 feat: additional include-groups option
- #31 improv: limit IAM policy for lambda to access SecretsManager resources
- #18 improv: do not echo sensitive params
- #6 feat: allow group to match regexp
- #36 feat: major refactor, upgrade to Go 1.16, updated dependencies, added capability to sync only groups and selected members

## 1.0.0-rc.7

- #11 Fixing deleted users not synced

## 1.0.0-rc.5

- #7 Groups are synced by their email address
- #9 Disabled users status is synced

## 1.0.0-rc.1

- #1 Fix: Pagination does not work
- #3 Refactor: New features for Serverless Repo and Google best practices
