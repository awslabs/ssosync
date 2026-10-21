# SSO Sync

This AWS Serverless Application populates AWS IAM Identity Center (formerly AWS SSO) directly with your Google Workspace users and groups.

> [!IMPORTANT]
> The recommended way to deploy ssosync is **not** to launch this Serverless Application Repository template directly. Instead use one of the [quick-start](https://github.com/awslabs/ssosync/tree/master/quick-start) templates with the **Sync from Git** option. The quick-start templates wrap this application and make ongoing updates far simpler, letting you track a released version straight from the repository.

> [!NOTE]
> Deploy ssosync into an ordinary linked account. It works through the Identity Store and SCIM APIs using only the IAM permissions granted to its execution role, and does not require IAM Identity Center delegated administration. Deploying into the Organization management account is not recommended.

## Setup

Before you start, [enable AWS IAM Identity Center](https://docs.aws.amazon.com/singlesignon/latest/userguide/step1.html) in AWS Organizations. The next steps configure access to the Google APIs and the AWS IAM Identity Center SCIM endpoint.

### Google

ssosync reads your directory as a **service account** that impersonates a Google Workspace admin via **domain-wide delegation**. These steps produce the two things ssosync needs: the `credentials.json` key (`GoogleCredentials`) and the admin email it impersonates (`GoogleAdminEmail`). The [AWS Control Tower workshop](https://catalog.workshops.aws/control-tower/en-US/authentication-authorization/google-workspace/3-provisioning-scim) provides a fully guided walkthrough.

Steps 1–4 are done in the [Google Cloud Console](https://console.cloud.google.com/); step 5 is done in the [Google Admin console](https://admin.google.com/) and requires a **super admin**.

1. **Create or select a Google Cloud project** to own the service account.
2. **Enable the Admin SDK API** in that project (*APIs & Services* > *Enable APIs and Services* > search for *Admin SDK API* > **Enable**). The service account cannot read the directory until this API is enabled.
3. **Create a service account** (*APIs & Services* > *Credentials* > **Create credentials** > **Service account**). No Google Cloud roles are required. Then, on the service account's **Keys** tab, choose **Add key** > **Create new key** > **JSON**, and rename the downloaded file to `credentials.json`. This is the value for `GoogleCredentials`. Keep it secret.
4. **Copy the service account's Client ID** from its **Advanced settings** / **Details** (the numeric OAuth2 client / unique ID).
5. **Grant domain-wide delegation** in the Admin console: **Security** > **Access and data control** > **API controls** > **Manage Domain Wide Delegation** > **Add new**. Enter the Client ID from step 4 and the following read-only scopes, then click **Authorize**:

   ```
   https://www.googleapis.com/auth/admin.directory.group.readonly,https://www.googleapis.com/auth/admin.directory.group.member.readonly,https://www.googleapis.com/auth/admin.directory.user.readonly
   ```

6. **Choose the admin to impersonate.** Note the email address of an existing Google Workspace admin that can read the directory (a super admin always works). This is the value for `GoogleAdminEmail`.

### AWS

Go to the AWS IAM Identity Center console in the region where you enabled IAM Identity Center and select *Settings*. Click `Enable automatic provisioning`.

A pop up appears with a **SCIM Endpoint URL** and an **Access Token**. The Access Token is only shown at this stage, so copy both values for use in the configuration below.

## Configuration

Configuration is supplied as the template parameters described below. When deployed as a Lambda function these are mapped to the environment variables shown in the right-hand column (the same `SSOSYNC_`-less names ssosync reads at runtime). This document only covers the deployment parameters / environment variables, as the CLI switches are not relevant to a Lambda deployment.

Sensitive values can be provided inline or as the ARN of a secret in [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/); ssosync resolves the secret automatically at runtime.

### Google Workspace Credentials

| Parameter | Environment Variable | Description | Default |
|-----------|---------------------|-------------|---------|
| `GoogleCredentials` | `GOOGLE_CREDENTIALS` | Contents of the `credentials.json` file, or the ARN of a secret containing it. | Required |
| `GoogleAdminEmail` | `GOOGLE_ADMIN` | Email address of a Google Workspace super admin, or the ARN of a secret containing it. | Required |
| `GoogleCustomerId` | `CUSTOMER_ID` | Google Workspace customer ID (from the [account settings profile](https://admin.google.com/ac/accountsettings/profile)), or the ARN of a secret containing it. Must be set to your real customer ID (not `my_customer`) when using `memberKey` in `GoogleGroupMatch`. | `my_customer` |

### AWS IAM Identity Center

| Parameter | Environment Variable | Description | Default |
|-----------|---------------------|-------------|---------|
| `SCIMEndpointUrl` | `SCIM_ENDPOINT` | AWS IAM Identity Center SCIM endpoint URL, or the ARN of a secret containing it. The AWS region is derived from this URL. | Required |
| `SCIMEndpointAccessToken` | `SCIM_ACCESS_TOKEN` | AWS IAM Identity Center SCIM access token, or the ARN of a secret containing it. | Required |

> [!NOTE]
> **Region** and **IdentityStoreID** are no longer configurable. Region is extracted from the SCIM endpoint URL and the Identity Store ID is resolved automatically via the IAM Identity Center `ListInstances` API. The deprecated `Region` and `IdentityStoreID` parameters remain in the template only for backward compatibility and will be removed in v3.x.

### Sync Configuration

| Parameter | Environment Variable | Description | Default |
|-----------|---------------------|-------------|---------|
| `GoogleGroupMatch` | `GROUP_MATCH` | Google Workspace groups filter query. Use `*` to sync all groups (and their member users). See [Group matching](#group-matching) below. | `*` |
| `GoogleUserMatch` | `USER_MATCH` | Google Workspace users filter query. Leave empty to select only users that are members of matched groups; use `*` to sync all users. See [Google Users Search](https://developers.google.com/admin-sdk/directory/v1/guides/search-users). | `""` |
| `IgnoreGroups` | `IGNORE_GROUPS` | Comma-separated list of Google Workspace group emails to ignore. | `""` |
| `IgnoreUsers` | `IGNORE_USERS` | Comma-separated list of Google Workspace user emails to ignore. | `""` |
| `SyncSuspended` | `SYNC_SUSPENDED` | Whether to include suspended users when syncing (`ignore` or `sync`). | `ignore` |
| `PrecacheOrgUnits` | `PRECACHE_ORG_UNITS` | Comma-separated list of Google OrgUnit paths to precache (e.g. `/OU1,/OU2/OU3`). Use `/` for the whole directory, or leave empty to disable precaching. | `/` |
| `ExternalIdChange` | `FORCE_EXTERNAL_ID_UPDATE` | When a Google user is deleted and recreated (same email, new ID), either `update` the existing Identity Store user to preserve assignments, or `delete/recreate` to prevent permission inheritance. | `delete/recreate` |
| `RetainUnmatched` | `RETAIN_UNMATCHED` | Retain 'local' users and groups that exist in the Identity Store but cannot be matched to Google Workspace, instead of deleting them (`remove` or `retain`). | `remove` |

### Lambda Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `FunctionName` | Optional fixed name for the function, useful when triggering ssosync from a pipeline. | `""` |
| `ScheduleExpression` | CloudWatch schedule expression controlling how often ssosync runs. Leave empty to trigger by another method. | `rate(15 minutes)` |
| `DryRun` | `dry-run` simulates the changes without modifying the Identity Store; `live` applies them. | `live` |
| `MemorySize` | RAM allocated to the function (128–10240 MB). | `128` |
| `TimeOut` | Function timeout in seconds (1–900). | `300` |

### Log Configuration

| Parameter | Environment Variable | Description | Default |
|-----------|---------------------|-------------|---------|
| `LogLevel` | `LOG_LEVEL` | Log level (`panic`, `fatal`, `error`, `warn`, `info`, `debug`, `trace`). | `warn` |
| `LogFormat` | `LOG_FORMAT` | Log format (`json` or `text`). | `json` |
| `LogRetention` | — | Number of days to retain CloudWatch logs; `0` retains them indefinitely. | `0` |

### `users_groups` Mode (to be deprecated in v3.x)

| Parameter | Environment Variable | Description | Default |
|-----------|---------------------|-------------|---------|
| `SyncMethod` | `SYNC_METHOD` | Sync method to use (`groups` or `users_groups`). | `groups` |
| `IncludeGroups` | `INCLUDE_GROUPS` | Include only these Google Workspace groups. Only applies when `SyncMethod` is `users_groups`. | `""` |

## Group matching

The `GoogleGroupMatch` parameter is passed through to the Google Admin SDK `groups.list` query. Separate multiple patterns with a `,`; each pattern runs as a separate query and the results are merged.

| Field | Prefix (`:`) | Exact (`=`) | Example |
|-------|:---:|:---:|---------|
| `name` | ✓ | ✓ | `name:AWS*` or `name=Administrators` |
| `email` | ✓ | ✓ | `email:aws-*` or `email=aws-admins@company.com` |
| `memberKey` | — | ✓ | `memberKey=user@company.com` |

- Use `:` for prefix/wildcard matching and `=` for exact matching.
- Use `memberKey=<email>` to match every group that a specific user (or group) is a direct member of. This is an exact match only, wildcards are not supported, and you **must** set `GoogleCustomerId` to your real customer ID (not the `my_customer` placeholder). The template enforces this: deployment fails if `memberKey` is used while `GoogleCustomerId` is left as `my_customer`.
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

Remember that `memberKey` requires `GoogleCustomerId` to be set to your real customer ID (not `my_customer`); the template fails deployment otherwise.

For the complete query syntax see [Google Groups Search](https://developers.google.com/admin-sdk/directory/v1/guides/search-groups).

## Deployment patterns

This application supports several deployment patterns via the `DeployPattern` parameter:

| Pattern | Description |
|---------|-------------|
| **App + secrets** | Default. Creates the application and the secrets from the values you provide. |
| **App only** | Creates the application and uses existing secrets you reference by ARN in this account. |
| **App for cross-account** | Creates the application and uses secrets held in another account, encrypted with a KMS key you provide. |
| **Secrets only** | Creates just the secrets. |
| **Secrets for cross-account** | Creates the secrets, encrypts them with a KMS key, and shares them with the target account. |

For the full set of deployment options and step-by-step guidance, start from the [quick-start](https://github.com/awslabs/ssosync/tree/master/quick-start) templates and the [project README](https://github.com/awslabs/ssosync/blob/master/README.md).
