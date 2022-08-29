# SSO Sync

![Github Action](https://github.com/awslabs/ssosync/workflows/main/badge.svg)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-42%25-brightgreen.svg?longCache=true&style=flat)</a>
[![Go Report Card](https://goreportcard.com/badge/github.com/awslabs/ssosync)](https://goreportcard.com/report/github.com/awslabs/ssosync)
[![License Apache 2](https://img.shields.io/badge/License-Apache2-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Taylor Swift](https://img.shields.io/badge/secured%20by-taylor%20swift-brightgreen.svg)](https://twitter.com/SwiftOnSecurity)

> Helping you populate AWS SSO directly with your Google Apps users

SSO Sync will run on any platform that Go can build for. It is available in the [AWS Serverless Application Repository](https://console.aws.amazon.com/lambda/home#/create/app?applicationId=arn:aws:serverlessrepo:us-east-2:004480582608:applications/SSOSync)

> :warning: there are breaking changes for versions `>= 0.02`

> :warning: `>= 1.0.0-rc.5` groups to do not get deleted in AWS SSO when deleted in the Google Directory, and groups are synced by their email address

> 🤔 we hope to support other providers in the future

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

## Installation

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
  -g, --group-match string          Google Workspace Groups filter query parameter, example: 'name:Admin* email:aws-*', see: https://developers.google.com/admin-sdk/directory/v1/guides/search-groups
  -h, --help                        help for ssosync
      --ignore-groups strings       ignores these Google Workspace groups
      --ignore-users strings        ignores these Google Workspace users
      --include-groups strings      include only these Google Workspace groups, NOTE: only works when --sync-method 'users_groups'
      --log-format string           log format (default "text")
      --log-level string            log level (default "info")
  -s, --sync-method string          Sync method to use (users_groups|groups) (default "groups")
  -m, --user-match string           Google Workspace Users filter query parameter, example: 'name:John* email:admin*', see: https://developers.google.com/admin-sdk/directory/v1/guides/search-users
  -v, --version                     version for ssosync
```

The function has `two behaviour` and these are controlled by the `--sync-method` flag, this behavior could be

1. `groups`: __(default)__
   * The sync procedure work base on Groups, gets the Google Workspace groups and their members, then creates in AWS SSO the users (members of the Google Workspace groups), then the groups and at the end assign the users to their respective groups.
   * This method use: `Google Workspace groups name` --> `AWS SSO groups name`
2. `users_groups`: __(original behavior, previous versions)__
   * The sync procedure is simple, gets the Google Workspace users and creates these in AWS SSO Users; then gets Google Workspace groups and creates these in AWS SSO Groups and assigns users to belong to the AWS SSO Groups.
   * This method use: `Google Workspace groups email` --> `AWS SSO groups name`

Flags Notes:

* `--include-groups` only works when `--sync-method` is `users_groups`
* `--ignore-users` works for both `--sync-method` values.  Example: `--ignore-users user1@example.com,user2@example.com` or `SSOSYNC_IGNORE_USERS=user1@example.com,user2@example.com`
* `--ignore-groups` works for both `--sync-method` values. Example: --ignore-groups group1@example.com,group1@example.com` or `SSOSYNC_IGNORE_GROUPS=group1@example.com,group1@example.com`
* `--group-match` works for both `--sync-method` values and also in combination with `--ignore-groups` and `--ignore-users`.  This is the filter query passed to the [Google Workspace Directory API when search Groups](https://developers.google.com/admin-sdk/directory/v1/guides/search-groups), if the flag is not used, groups are not filtered.
* `--user-match` works for both `--sync-method` values and also in combination with `--ignore-groups` and `--ignore-users`.  This is the filter query passed to the [Google Workspace Directory API when search Users](https://developers.google.com/admin-sdk/directory/v1/guides/search-users), if the flag is not used, users are not filtered.

NOTES:

1. Depending on the number of users and groups you have, maybe you can get `AWS SSO SCIM API rate limits errors`, and more frequently happens if you execute the sync many times in a short time.
2. Depending on the number of users and groups you have, `--debug` flag generate too much logs lines in your AWS Lambda function.  So test it in locally with the `--debug` flag enabled and disable it when you use a AWS Lambda function.
3. `--sync-method "Groups"` and `--sync-method "users_groups"` are incompatible, because the first use the Google group name as an AWS group name and the second one use the Google group email, take this into consideration.

## AWS Lambda Usage

NOTE: Using Lambda may incur costs in your AWS account. Please make sure you have checked
the pricing for AWS Lambda and CloudWatch before continuing.

Running ssosync once means that any changes to your Google directory will not appear in
AWS SSO. To sync. regularly, you can run ssosync via AWS Lambda.

:warning: You find it in the [AWS Serverless Application Repository](https://eu-west-1.console.aws.amazon.com/lambda/home#/create/app?applicationId=arn:aws:serverlessrepo:us-east-2:004480582608:applications/SSOSync).

## SAM

You can use the AWS Serverless Application Model (SAM) to deploy this to your account.

> Please, install the [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html) and [GoReleaser](https://goreleaser.com/install/).

Specify an Amazon S3 Bucket for the upload with `export S3_BUCKET=<YOUR_BUCKET>`.

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
