## v2.4.0 Latest
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
535-18d6 Update template.yaml

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
