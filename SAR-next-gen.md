# ssosync (next generation)

This AWS Serverless Application populates AWS IAM Identity Center (formerly AWS SSO) directly with your Google Workspace users and groups. It is the next generation of the [SSOSync](https://serverlessrepo.aws.amazon.com/applications/us-east-2/004480582608/SSOSync) application, built from the same code: Google authentication is keyless only (Workload Identity Federation), with no stored service-account key and no Google admin, and `groups` is the only sync method. New features are added here; SSOSync keeps the 2.10.0 configuration.

> **Important:** the recommended way to deploy ssosync is **not** to launch this Serverless Application Repository template directly. Instead use one of the [quick-start](https://github.com/awslabs/ssosync/tree/master/quick-start) templates with the **Sync from Git** option; start from [`quick-start/next-gen/`](https://github.com/awslabs/ssosync/tree/master/quick-start/next-gen). The quick-start templates wrap this application and make ongoing updates far simpler, letting you track a released version straight from the repository.

> **Note:** deploy ssosync into an ordinary linked account. It works through the Identity Store and SCIM APIs using only the IAM permissions granted to its execution role, and does not require IAM Identity Center delegated administration. Deploying into the Organization management account is not recommended.

## Setup

Before you start, [enable AWS IAM Identity Center](https://docs.aws.amazon.com/singlesignon/latest/userguide/step1.html) in AWS Organizations. The next steps configure access to the Google APIs and the AWS IAM Identity Center SCIM endpoint.

### Google

ssosync reads your directory as a Google **service account**, which acts as itself with a read-only Workspace admin role. It authenticates with **Workload Identity Federation**: Google Cloud trusts the IAM role the function runs as, and every run exchanges that role's credentials for short-lived Google credentials. No service-account key is created or stored, it works where the `iam.disableServiceAccountKeyCreation` organization policy forbids keys, and no Google admin is impersonated.

The [project README](https://github.com/awslabs/ssosync/blob/master/README.md#workload-identity-federation-keyless) has the full, copy-pasteable Cloud Shell steps; in short:

1. **Enable the APIs** (Admin SDK, IAM, Security Token Service, IAM Service Account Credentials) in a Google Cloud project.
2. **Create a service account** without a key.
3. **Assign it a read-only admin role** in the [Google Admin console](https://admin.google.com/) (a **super admin** is needed): a custom role with only *Users* > *Read* and *Groups* > *Read*, assigned to the service account. No domain-wide delegation is needed.
4. **Create a workload identity pool and an AWS provider** for the AWS account, with an attribute condition that accepts only the function's role. Set `RoleName` (for example `SSOSync_ExecutionRole`) so the role has a fixed name to trust; otherwise read the generated name from the stack's `RoleName` output after deploying.
5. **Grant *Workload Identity User*** (`roles/iam.workloadIdentityUser`) on the service account to that role.
6. **Create the credential configuration** (`gcloud iam workload-identity-pools create-cred-config ... --aws`, without `--service-account`). Its contents are the value for `GoogleWIFConfig`; it holds no secret.
7. **Note the customer ID** (Admin console > *Account* > *Account settings* > *Profile*). It is required; `my_customer` is not accepted.

Set `GoogleImpersonateServiceAccount` to the service account email.

### AWS

Go to the AWS IAM Identity Center console in the region where you enabled IAM Identity Center and select *Settings*. Click `Enable automatic provisioning`.

A pop up appears with a **SCIM Endpoint URL** and an **Access Token**. The Access Token is only shown at this stage, so copy both values for use in the configuration below.

## Configuration

Configuration is supplied as the template parameters described below. When deployed as a Lambda function these are mapped to the environment variables shown with each parameter (the same `SSOSYNC_`-less names ssosync reads at runtime). This document only covers the deployment parameters / environment variables, as the CLI switches are not relevant to a Lambda deployment.

Sensitive values can be provided inline or as the ARN of a secret in [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/); ssosync resolves the secret automatically at runtime.

### Google Workspace Credentials

- `GoogleImpersonateServiceAccount` (environment variable `GOOGLE_IMPERSONATE_SERVICE_ACCOUNT`; required): the email of the Google service account to impersonate, or the ARN of a secret containing it.
- `GoogleWIFConfig` (environment variable `GOOGLE_WIF_CONFIG`; required): the Workload Identity Federation credential configuration (the `external_account` JSON from `gcloud iam workload-identity-pools create-cred-config --aws`), or the ARN of a Secrets Manager secret or SSM Parameter Store parameter containing it.
- `GoogleCustomerId` (environment variable `CUSTOMER_ID`; required): Google Workspace customer ID (`Cxxxxxxxx`, from the [account settings profile](https://admin.google.com/ac/accountsettings/profile)), or the ARN of a secret containing it. The `my_customer` alias is not accepted: the service account is not a Workspace user.

### AWS IAM Identity Center

- `SCIMEndpointUrl` (environment variable `SCIM_ENDPOINT`; required): AWS IAM Identity Center SCIM endpoint URL, or the ARN of a secret containing it. The AWS region is derived from this URL.
- `SCIMEndpointAccessToken` (environment variable `SCIM_ACCESS_TOKEN`; required): AWS IAM Identity Center SCIM access token, or the ARN of a secret containing it.

> **Note:** **Region** and **IdentityStoreID** are no longer configurable. Region is extracted from the SCIM endpoint URL and the Identity Store ID is resolved automatically via the IAM Identity Center `ListInstances` API. This application has no `Region` or `IdentityStoreID` parameters.

### Sync Configuration

- `GoogleGroupMatch` (environment variable `GROUP_MATCH`; default `*`): Google Workspace groups filter query. Use `*` to sync all groups (and their member users). Passed to Google verbatim; see [Group matching](#group-matching) and [Quoting values in queries](#quoting-values-in-queries) below.
- `GoogleUserMatch` (environment variable `USER_MATCH`; default `""`): Google Workspace users filter query. Leave empty to select only users that are members of matched groups; use `*` to sync all users. Passed to Google verbatim; see [Quoting values in queries](#quoting-values-in-queries) and [Google Users Search](https://developers.google.com/admin-sdk/directory/v1/guides/search-users).
- `IgnoreGroups` (environment variable `IGNORE_GROUPS`; default `""`): comma-separated list of Google Workspace group emails to ignore.
- `IgnoreUsers` (environment variable `IGNORE_USERS`; default `""`): comma-separated list of Google Workspace user emails to ignore.
- `SyncSuspended` (environment variable `SYNC_SUSPENDED`; default `ignore`): whether to include suspended users when syncing (`ignore` or `sync`).
- `PrecacheOrgUnits` (environment variable `PRECACHE_ORG_UNITS`; default `/`): comma-separated list of Google OrgUnit paths to precache users from (e.g. `/OU1,/OU2/OU3`); each path includes its sub-OUs. Use `/` for the whole directory, or leave empty to disable precaching. A path that cannot be queried (it may not exist), or has no users matching the filter, is skipped with a warning naming it; the other paths are still precached.
- `ExternalIdChange` (environment variable `FORCE_EXTERNAL_ID_UPDATE`; default `delete/recreate`): when a Google user is deleted and recreated (same email, new ID), either `update` the existing Identity Store user to preserve assignments, or `delete/recreate` to prevent permission inheritance.
- `RetainUnmatched` (environment variable `RETAIN_UNMATCHED`; default `remove`): retain 'local' users and groups that exist in the Identity Store but cannot be matched to Google Workspace, instead of deleting them (`remove` or `retain`).
- `AttributeMap` (environment variable `ATTRIBUTE_MAP`; default `""`): attribute map controlling how Google user attributes map onto IAM Identity Center (SCIM) fields. Accepts inline attribute-map JSON, a Secrets Manager ARN, or an SSM Parameter Store ARN. Leave empty to use the built-in default mapping (no secret is created and `ATTRIBUTE_MAP` is not set). With a deployment pattern that creates secrets, inline JSON is stored in the `SSOSyncAttributeMap` secret. See [Attribute mapping](#attribute-mapping) below.

### Lambda Configuration

- `FunctionName` (default `""`): optional fixed name for the function, useful when triggering ssosync from a pipeline.
- `RoleName` (default `""`): optional fixed name for the function's IAM role. Set it so the Google workload identity provider can trust a name that does not change when the stack is redeployed (for example `SSOSync_ExecutionRole`); IAM role names are unique per account. Empty: CloudFormation generates a name, shown in the `RoleName` output.
- `ScheduleExpression` (default `rate(15 minutes)`): CloudWatch schedule expression controlling how often ssosync runs. Leave empty to trigger by another method.
- `DryRun` (default `live`): `dry-run` simulates the changes without modifying the Identity Store; `live` applies them.
- `MemorySize` (default `128`): RAM allocated to the function (128–10240 MB).
- `TimeOut` (default `300`): function timeout in seconds (1–900).

### Log Configuration

- `LogLevel` (environment variable `LOG_LEVEL`; default `warn`): log level (`panic`, `fatal`, `error`, `warn`, `info`, `debug`, `trace`).
- `LogFormat` (environment variable `LOG_FORMAT`; default `json`): log format (`json` or `text`).
- `LogRetention` (no environment variable; default `0`): number of days to retain CloudWatch logs; `0` retains them indefinitely.

## Group matching

The `GoogleGroupMatch` parameter is passed through to the Google Admin SDK `groups.list` query. Separate multiple patterns with a `,`; each pattern runs as a separate query and the results are merged. The fields you can match on are:

- `name`: prefix (`:`) and exact (`=`) matching, for example `name:AWS*` or `name=Administrators`.
- `email`: prefix (`:`) and exact (`=`) matching, for example `email:aws-*` or `email=aws-admins@company.com`.
- `memberKey`: exact (`=`) matching only, for example `memberKey=user@company.com`.

How to use them:

- Use `:` for prefix/wildcard matching and `=` for exact matching.
- Use `memberKey=<email>` to match every group that a specific user (or group) is a direct member of. This is an exact match only, wildcards are not supported,. The template enforces this: deployment fails if `memberKey` is used while `GoogleCustomerId` is left as `my_customer`.
- Use `*` alone to sync all groups (and, with the default `groups` sync method, their member users).

### Syncing groups regardless of naming strategy with `memberKey`

Google's directory query syntax only matches group `name` and `email` with prefix/wildcard patterns, so the common prefix/suffix convention (e.g. `name:aws-*`) requires every group you want synced to follow that naming strategy. If your groups don't share a naming convention, use `memberKey` with a dedicated "marker" group so any group can be synced regardless of its name:

1. Create a single marker group in Google Workspace, for example:
   - **Name**: `Any group with this as a member will be synced to IAM Identity Center`
   - **Description**: `Any group with this as a member will be synced to IAM Identity Center`
   - **Email**: `aws-ssosync@your-domain.com`
2. Add this marker group as a **member** of every group you want to sync.
3. Set `GoogleGroupMatch` to `memberKey=aws-ssosync@your-domain.com`.

ssosync then selects every group that has the marker group as a member, no matter how those groups are named. Add or remove the marker group from a group's membership to change the sync scope later.

### Quoting values in queries

`GoogleGroupMatch` and `GoogleUserMatch` are passed to Google verbatim; ssosync does not quote or escape them. A value containing spaces or an apostrophe must be single-quoted, with the apostrophe escaped as `\'`, for example `name='Valentine\'s Day'`. Commas still separate queries, so a value cannot contain a comma.

For the complete query syntax see [Google Groups Search](https://developers.google.com/admin-sdk/directory/v1/guides/search-groups) and [Google Users Search](https://developers.google.com/admin-sdk/directory/v1/guides/search-users).

## Attribute mapping

The `AttributeMap` parameter (Lambda environment variable `ATTRIBUTE_MAP`) controls how Google Workspace user attributes are mapped onto the AWS IAM Identity Center (SCIM) user fields. Like the credentials parameters, it accepts inline attribute-map JSON, the ARN of a secret in AWS Secrets Manager, or the ARN of an SSM Parameter Store parameter; ssosync resolves ARNs at runtime, and the template grants the function read access to the secret or parameter you give. Leaving it empty uses the built-in default mapping below; no secret is created and `ATTRIBUTE_MAP` is not set.

How the value is deployed depends on the deployment pattern:

- **Empty**
  - `App + secrets`, `Secrets only`, `Secrets for cross-account`: nothing created, `ATTRIBUTE_MAP` not set.
  - `App only`, `App for cross-account`: `ATTRIBUTE_MAP` not set.
- **Inline JSON**
  - `App + secrets`, `Secrets only`, `Secrets for cross-account`: stored in the `SSOSyncAttributeMap` secret (shared with the app account for cross-account, and exported as `AttributeMap`); `ATTRIBUTE_MAP` is the secret ARN.
  - `App only`, `App for cross-account`: `ATTRIBUTE_MAP` is the JSON (the Lambda environment is limited to 4 KB in total, so use a secret or parameter for a large map).
- **Secrets Manager / SSM parameter ARN**
  - `App + secrets`, `Secrets only`, `Secrets for cross-account`: `ATTRIBUTE_MAP` is the ARN.
  - `App only`, `App for cross-account`: `ATTRIBUTE_MAP` is the ARN.

For `App for cross-account`, pass the `AttributeMap` exported by the `Secrets for cross-account` stack.

Groups map `DisplayName` from the Google group name only (AWS SCIM accepts only `externalId`, `displayName` and `members` for groups; a group description is not SCIM-settable).

The following JSON is exactly the built-in default. Copying it verbatim changes nothing, so it is a safe starting point to copy and then customise:

```json
{
  "user": {
    "Name.GivenName": "name.givenName",
    "Name.FamilyName": "name.familyName",
    "Username": "primaryEmail",
    "Active": "suspended",
    "Title": "organizations[primary].title",
    "Enterprise.Department": "organizations[primary].department",
    "Enterprise.Organization": "organizations[primary].name",
    "Enterprise.CostCenter": "organizations[primary].costCenter"
  },
  "group": {
    "DisplayName": "name"
  }
}
```

Each block is optional: an omitted block keeps its default, and a present block replaces that block's mapping. Within a block the key is the SCIM target field and the value is the Google source attribute.

- **Defaulted** user target fields: `Name.GivenName`, `Name.FamilyName`, `Username`, `Active`, `Title`, `Enterprise.Department`, `Enterprise.Organization`, `Enterprise.CostCenter`.
- **Opt-in** user target fields (supported, not in the default): `Enterprise.Manager`, `Enterprise.Division`, `Enterprise.EmployeeNumber`, `ProfileURL`, `Emails`, `Addresses`, `PhoneNumbers`. `Enterprise.Manager` maps from a relation value such as `relations[type=manager].value`, which Google stores as the manager's email address; it works where your deployment's manager relation holds an email the Identity Store can resolve, and is opt-in because it depends on your directory populating that relation.

The organization-derived defaults source from `organizations[primary].*`, the organization marked primary (the Google Admin console marks it primary but does not set its type). A directory that types its organizations can map `organizations[type=work].*` instead. Google user-defined (custom) schema attributes can also be mapped as sources with `customSchemas.<SchemaName>.<fieldName>`.

For the full JSON syntax, the supported source attributes, the custom-schema sources, and the type-qualified selection rule for multi-valued Google fields, see [Attribute mapping in the project README](https://github.com/awslabs/ssosync/blob/master/README.md#attribute-mapping).

> **Caution:** the sync makes the AWS user consistent with the mapped Google user (clear-to-consistent): any supported SCIM field the map does not populate is cleared on the AWS user. Set `DryRun` to `dry-run` and run it **first** to preview exactly what would change before applying a new or changed map — an error in the map (a typo, a `[type=X]` that matches nothing, a missing custom field), or the `[primary]` default itself for users with no organization marked primary, resolves to nothing and clears that attribute.

When a field maps unexpectedly, enable debug logging (it dumps the raw Google source fields and the per-entry resolution) and watch for the pre-SCIM `WARN` lines ssosync emits when a mapped user/group would likely be rejected by the SCIM API — together they help you troubleshoot a custom map.

As with the rest of the deployment, the quick-start templates are the recommended way to set this up.

## Deployment patterns

This application supports several deployment patterns via the `DeployPattern` parameter:

- **App + secrets**: default. Creates the application and the secrets from the values you provide.
- **App only**: creates the application and uses existing secrets you reference by ARN in this account.
- **App for cross-account**: creates the application and uses secrets held in another account, encrypted with a KMS key you provide.
- **Secrets only**: creates just the secrets.
- **Secrets for cross-account**: creates the secrets, encrypts them with a KMS key, and shares them with the target account.

For the full set of deployment options and step-by-step guidance, start from the [quick-start](https://github.com/awslabs/ssosync/tree/master/quick-start) templates and the [project README](https://github.com/awslabs/ssosync/blob/master/README.md).
