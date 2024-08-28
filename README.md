# SSO Sync

![Github Action](https://github.com/awslabs/ssosync/workflows/main/badge.svg)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-42%25-brightgreen.svg?longCache=true&style=flat)</a>
[![Go Report Card](https://goreportcard.com/badge/github.com/awslabs/ssosync)](https://goreportcard.com/report/github.com/awslabs/ssosync)
[![License Apache 2](https://img.shields.io/badge/License-Apache2-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Taylor Swift](https://img.shields.io/badge/secured%20by-taylor%20swift-brightgreen.svg)](https://twitter.com/SwiftOnSecurity)

> Helping you populate AWS SSO directly with your Google Apps users

SSO Sync will run on any platform that Go can build for. It is available in the [AWS Serverless Application Repository](https://console.aws.amazon.com/lambda/home#/create/app?applicationId=arn:aws:serverlessrepo:us-east-2:004480582608:applications/SSOSync)

> [!CAUTION]
> When using ssosync with an instance of IAM Identity Center integrated with AWS Control Tower. AWS Control Tower creates a number of groups and users (directly via the Identity Store API), when an external identity provider is configured these users and groups are can not be used to log in. However it is important to remember that because ssosync implemements a uni-directional sync it will make the IAM Identity Store match the subset of your Google Workspaces directory you specify, including removing these groups and users created by AWS Control Tower. There is a PFR [#179 Configurable handling of 'manually created' Users/Groups in IAM Identity Center](https://github.com/awslabs/ssosync/issues/179) to implement an option to ignore these users and groups, hopefully this will be implemented in version 3.x. However, this has a dependancy on PFR [#166 Ensure all groups/user creates in IAM Identity Store are via SCIM api and populate externalId field](https://github.com/awslabs/ssosync/issues/166), to be able to reliably and consistently disinguish between **SCIM Provisioned** users from **Manually Created** users

> [!WARNING]
> There are breaking changes for versions `>= 0.02`

> [!WARNING]
> `>= 1.0.0-rc.5` groups to do not get deleted in AWS SSO when deleted in the Google Directory, and groups are synced by their email address

> [!WARNING]
> `>= 2.0.0` this makes use of the **Identity Store API** which means:
> * if deploying the lambda from the [AWS Serverless Application Repository](https://console.aws.amazon.com/lambda/home#/create/app?applicationId=arn:aws:serverlessrepo:us-east-2:004480582608:applications/SSOSync) then it needs to be deployed into the [IAM Identity Center delegated administration](https://docs.aws.amazon.com/singlesignon/latest/userguide/delegated-admin.html) account. Technically you could deploy in the management account but we would recommend against this.
> * if you are running the project as a cli tool, then the environment will need to be using credentials of a user in the [IAM Identity Center delegated administration](https://docs.aws.amazon.com/singlesignon/latest/userguide/delegated-admin.html) account, with appropriate permissions.

> [!WARNING]
> `>= 2.1.0` make use of named IAM resources, so if deploying via CICD or IaC template will require **CAPABILITY_NAMED_IAM** to be specified.

> [!IMPORTANT]
> `>= 2.1.0` switched to using `provided.al2` powered by ARM64 instances.

> [!IMPORTANT]
> As of `v2.2.0` multiple query patterns are supported for both Group and User matching, simply separate each query with a `,`. For full sync of groups and/or users specify '*' in the relevant match field. 
> User match and group match can now be used in combination with the sync method of groups.
> Nested groups will now be flattened into the top level groups.
> External users are ignored.
> Group owners are treated as regular group members.
> User details are now cached to reduce the number of api calls and improve execution times on large directories.

## Why?

As per the [AWS SSO](https://aws.amazon.com/single-sign-on/) Homepage:

> AWS Single Sign-On (SSO) makes it easy to centrally manage access
> to multiple AWS accounts and business applications and provide users
> with single sign-on access to all their assigned accounts and applications
> from one place.

Key part further down:

> With AWS SSO, you can create and manage user identities in AWS SSO’s
>identity store, or easily connect to your existing identity source including
> Microsoft Active Directory and **Azure Active Directory (Azure AD)**.

AWS SSO can use other Identity Providers as well... such as Google Apps for Domains. Although AWS SSO
supports a subset of the SCIM protocol for populating users, it currently only has support for Azure AD.

This project provides a CLI tool to pull users and groups from Google and push them into AWS SSO.
`ssosync` deals with removing users as well. The heavily commented code provides you with the detail of
what it is going to do.

### References

 * [SCIM Protocol RFC](https://tools.ietf.org/html/rfc7644)
 * [AWS SSO - Connect to Your External Identity Provider](https://docs.aws.amazon.com/singlesignon/latest/userguide/manage-your-identity-source-idp.html)
 * [AWS SSO - Automatic Provisioning](https://docs.aws.amazon.com/singlesignon/latest/userguide/provision-automatically.html)
 * [AWS IAM Identity Center - Identity Store API](https://docs.aws.amazon.com/singlesignon/latest/IdentityStoreAPIReference/welcome.html)

## Installation

The recommended installation is:
* [Setup IAM Identity Center](https://docs.aws.amazon.com/singlesignon/latest/userguide/get-started-enable-identity-center.html), in the management account of your organization
* Created a linked account `Identity` Account from which to manage IAM Identity Center
* [Delegate administration](https://docs.aws.amazon.com/singlesignon/latest/userguide/delegated-admin.html) to the `Identity` account
* Deploy the [SSOSync app](https://console.aws.amazon.com/lambda/home#/create/app?applicationId=arn:aws:serverlessrepo:us-east-2:004480582608:applications/SSOSync) from the AWS Serverless Application Repository


You can also:
You can `go get github.com/awslabs/ssosync` or grab a Release binary from the release page. The binary
can be used from your local computer, or you can deploy to AWS Lambda to run on a CloudWatch Event
for regular synchronization.

## Configuration

You need a few items of configuration. One side from AWS, and the other
from Google Cloud to allow for API access to each. You should have configured
Google as your Identity Provider for AWS SSO already.

You will need the files produced by these steps for AWS Lambda deployment as well
as locally running the ssosync tool.

### Google

First, you have to setup your API. In the project you want to use go to the [Console](https://console.developers.google.com/apis) and select *API & Services* > *Enable APIs and Services*. Search for *Admin SDK* and *Enable* the API.

You have to perform this [tutorial](https://developers.google.com/admin-sdk/directory/v1/guides/delegation) to create a service account that you use to sync your users. Save the `JSON file` you create during the process and rename it to `credentials.json`.

> you can also use the `--google-credentials` parameter to explicitly specify the file with the service credentials. Please, keep this file safe, or store it in the AWS Secrets Manager

In the domain-wide delegation for the Admin API, you have to specify the following scopes for the user.

* https://www.googleapis.com/auth/admin.directory.group.readonly
* https://www.googleapis.com/auth/admin.directory.group.member.readonly
* https://www.googleapis.com/auth/admin.directory.user.readonly

Back in the Console go to the Dashboard for the API & Services and select "Enable API and Services".
In the Search box type `Admin` and select the `Admin SDK` option. Click the `Enable` button.

You will have to specify the email address of an admin via `--google-admin` to assume this users role in the Directory.

### AWS

Go to the AWS Single Sign-On console in the region you have set up AWS SSO and select
Settings. Click `Enable automatic provisioning`.

A pop up will appear with URL and the Access Token. The Access Token will only appear
at this stage. You want to copy both of these as a parameter to the `ssosync` command.

Or you specific these as environment variables.

```bash
SSOSYNC_SCIM_ACCESS_TOKEN=<YOUR_TOKEN>
SSOSYNC_SCIM_ENDPOINT=<YOUR_ENDPOINT>
```

Additionally, authenticate your AWS credentials. Follow this  [section](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#:~:text=Creating%20the%20Credentials%20File) to create a Shared Credentials File in the home directory or export your Credentials with Environment Variables. Ensure that the default credentials are for the AWS account you intended to be synced.

To obtain your `Identity store ID`, go to the AWS Identity Center console and select settings. Under the `Identity Source` section, copy the `Identity store ID`.

## Local Usage

```bash
git clone https://github.com/awslabs/ssosync.git
cd ssosync/
make go-build
```

```bash
./ssosync --help
```

```bash
A command line tool to enable you to synchronise your Google
Apps (Google Workspace) users to AWS Single Sign-on (AWS SSO)
Complete documentation is available at https://github.com/awslabs/ssosync

Usage:
  ssosync [flags]

Flags:
  -t, --access-token string         AWS SSO SCIM API Access Token
  -d, --debug                       enable verbose / debug logging
  -e, --endpoint string             AWS SSO SCIM API Endpoint
  -u, --google-admin string         Google Workspace admin user email
  -c, --google-credentials string   path to Google Workspace credentials file (default "credentials.json")
  -g, --group-match string          Google Workspace Groups filter query parameter, a simple '*' denotes sync all groups (and any users that are members of those groups). example: 'name:Admin*,email:aws-*', 'name=Admins' or '*' see: https://developers.google.com/admin-sdk/directory/v1/guides/search-groups, if left empty no groups will be selected.
  -h, --help                        help for ssosync
      --ignore-groups strings       ignores these Google Workspace groups
      --ignore-users strings        ignores these Google Workspace users
      --include-groups strings      include only these Google Workspace groups, NOTE: only works when --sync-method 'users_groups'
      --log-format string           log format (default "text")
      --log-level string            log level (default "info")
  -s, --sync-method string          Sync method to use (users_groups|groups) (default "groups")
  -m, --user-match string           Google Workspace Users filter query parameter, a simple '*' denotes sync all users in the directory. example: 'name:John*,email:admin*', '*' or name=John Doe,email:admin*' see: https://developers.google.com/admin-sdk/directory/v1/guides/search-users, if left empty no users will be selected but if a pattern has been set for GroupMatch users that are members of the groups it matches will still be selected
  -v, --version                     version for ssosync
  -r, --region                      AWS region where identity store exists
  -i, --identity-store-id           AWS Identity Store ID
```

The function has `two behaviour` and these are controlled by the `--sync-method` flag, this behavior could be

1. `groups`: __(default)__ The sync procedure work base on Groups, gets the Google Workspace groups and their members, then creates in AWS SSO the users (members of the Google Workspace groups), then the groups and at the end assign the users to their respective groups.
2. `users_groups`: __(original behavior, previous versions)__ The sync procedure is simple, gets the Google Workspace users and creates these in AWS SSO Users; then gets Google Workspace groups and creates these in AWS SSO Groups and assigns users to belong to the AWS SSO Groups.

Flags Notes:

* `--include-groups` only works when `--sync-method` is `users_groups`
* `--ignore-users` works for both `--sync-method` values.  Example: `--ignore-users user1@example.com,user2@example.com` or `SSOSYNC_IGNORE_USERS=user1@example.com,user2@example.com`
* `--ignore-groups` works for both `--sync-method` values. Example: --ignore-groups group1@example.com,group1@example.com` or `SSOSYNC_IGNORE_GROUPS=group1@example.com,group1@example.com`
* `--group-match` works for both `--sync-method` values and also in combination with `--ignore-groups` and `--ignore-users`.  This is the filter query passed to the [Google Workspace Directory API when search Groups](https://developers.google.com/admin-sdk/directory/v1/guides/search-groups), if the flag is not used, groups are not filtered.
* `--user-match` works for both `--sync-method` values and also in combination with `--ignore-groups` and `--ignore-users`.  This is the filter query passed to the [Google Workspace Directory API when search Users](https://developers.google.com/admin-sdk/directory/v1/guides/search-users), if the flag is not used, users are not filtered.

> [!NOTE]
> 1. Depending on the number of users and groups you have, maybe you can get `AWS SSO SCIM API rate limits errors`, and more frequently happens if you execute the sync many times in a short time.
> 2. Depending on the number of users and groups you have, `--debug` flag generate too much logs lines in your AWS Lambda function.  So test it in locally with the `--debug` flag enabled and disable it when you use a AWS Lambda function.

## AWS Lambda Usage

> [!TIP]
> Using Lambda may incur costs in your AWS account. Please make sure you have checked
the pricing for AWS Lambda and CloudWatch before continuing.

Additionally, before choosing to deploy with Lambda, please ensure that the [AWS Lambda SLAs](https://aws.amazon.com/lambda/sla/) are sufficient for your use cases.

Running ssosync once means that any changes to your Google directory will not appear in
AWS SSO. To sync regularly, you can run ssosync via AWS Lambda.

> [!WARNING]
> You find it in the [AWS Serverless Application Repository](https://eu-west-1.console.aws.amazon.com/lambda/home#/create/app?applicationId=arn:aws:serverlessrepo:us-east-2:004480582608:applications/SSOSync).

> [!TIP]
> ### v2.1 Changes
> * user and group selection fields in the Cloudformation template can now be left empty where not required and will not be added as environment variables to the Lambda function, this provides consistency with CLI use of ssosync.
> * Stronger validation of parameters in the Cloudformation template, to improve likelhood of success for new users.
> * Now supports multiple deployment patterns, defaults are consistent with previous versions.

**App + secrets** This is the default mode and fully backwards compatible with previous versions

**App only** This mode does not create the secrets but expects you to deployed a separate stack using the **Secrets only** mode within the same account
> [!CAUTION]
> If you want to use your own existing secrets then provide them as a comma separated list in the ##CrossStackConfigI## field in the following order:
> __GoogleCredentials ARN__,__GoogleAdminEmail ARN__,__SCIMEndpoint ARN__,__SCIMAccessToken ARN__,__Region ARN__,__IdentityStoreID ARN__
> 
**App for cross-account** This mode is used where you have deployed the secrets in a separate account, the arns of the KMS key and secrets need to be passed into the __CrossStackConfig__ field, It is easiest to have created the secrets in the other account using the ** Secrest for cross-account** mode, as the output can simply copied and pasted into the above field.

> [!CAUTION]
> If you want to use your own existing secrets then provide them as a comma separated list in the __CrossStackConfig__ field in the following order:
> __GoogleCredentials ARN__,__GoogleAdminEmail ARN__,__SCIMEndpoint ARN__,__SCIMAccessToken ARN__,__Region ARN__,__IdentityStoreID ARN__,__KMS Key ARN__

> [!IMPORTANT]
> Be sure to allow access to the key and secrets in their respective policies to the role __SSOSyncAppRole__ in the app account.

**Secrets only** This mode creates a set of secrets but does not deploy the app itself, it requires the app is deployed in that same account using the **App only** mode. This allows for decoupling of the secrets and the app.

**Secrets for cross-account** This mode creates a set of secrets and KMS key but does not deploy the app itself, this is for use with an app stack, deployed using the **App for cross-account** mode. This allows for a single set of secrets to be shared with multipl app instance for testing, and improve secrets security.

## SAM

You can use the AWS Serverless Application Model (SAM) to deploy this to your account.

> Please, install the [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html) and [GoReleaser](https://goreleaser.com/install/).

Specify an Amazon S3 Bucket for the upload with `export S3_BUCKET=<YOUR_BUCKET>` and an S3 prefix with `export S3_PREFIX=<YOUR_PREFIX>`.

Execute `make package` in the console. Which will package and upload the function to the bucket. You can then use the `packaged.yaml` to configure and deploy the stack in [AWS CloudFormation Console](https://console.aws.amazon.com/cloudformation).

### Example

Build

```bash
aws cloudformation validate-template --template-body  file://template.yaml 1>/dev/null &&
sam validate &&
sam build
```

Deploy

```bash
sam deploy --guided
```

## License

[Apache-2.0](/LICENSE)
