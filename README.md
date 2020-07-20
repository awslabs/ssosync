# SSO Sync

> Helping you populate AWS SSO directly with your Google Apps users

SSO Sync will run on any platform that Go can build for.

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

You have to perform this [tutorial](https://developers.google.com/admin-sdk/directory/v1/guides/delegation) to create a service account that you use to sync your users. Save the JSON file your create during the process and rename it to `credentials.json`. 

> you can also use the `--google-credentials` parameter to explicitly specify the file with the service credentials. Please, keep this file safe, or store it in the AWS Secrets Manager

In the domain-wide delegation for the Admin API, you have to specificy the following scopes for user.

`https://www.googleapis.com/auth/admin.directory.group.readonly,https://www.googleapis.com/auth/admin.directory.group.member.readonly,https://www.googleapis.com/auth/admin.directory.user.readonly`

Back in the Console go to the Dashboard for the API & Services and select "Enable API and Services".
In the Search box type `Admin` and select the `Admin SDK` option. Click the `Enable` button.

### AWS

Go to the AWS Single Sign-On console in the region you have set up AWS SSO and select
Settings. Click `Enable automatic provisioning`.

A pop up will appear with URL and the Access Token. The Access Token will only appear
at this stage. You want to copy both of these as a parameter to the `ssosync` command.

Or you specifc these as environment variables.

```
SSOSYNC_SCIM_ACCESS_TOKEN=<YOUR_TOKEN>
SSOSYNC_SCIM_ENDPOINT=<YOUR_ENDPOINT>
```

## Local Usage

Usage:

The default for ssosync is to run through the sync.

```text
A command line tool to enable you to synchronise your Google
Apps (G-Suite) users to AWS Single Sign-on (AWS SSO)
Complete documentation is available at https://github.com/awslabs/ssosync

Usage:
  ssosync [flags]

Flags:
  -t, --access-token string         SCIM Access Token
  -d, --debug                       Enable verbose / debug logging
  -e, --endpoint string             SCIM Endpoint
  -u, --google-admin string         Google Admin Email
  -c, --google-credentials string   set the path to find credentials for Google (default "credentials.json")
  -h, --help                        help for ssosync
      --log-format string           log format (default "text")
      --log-level string            log level (default "warn")
  -v, --version                     version for ssosync
```

The output of the command when run without 'debug' turned on looks like this:

```
2020-05-26T12:08:14.083+0100	INFO	cmd/root.go:43	Creating the Google and AWS Clients needed
2020-05-26T12:08:14.084+0100	INFO	internal/sync.go:38	Start user sync
2020-05-26T12:08:14.979+0100	INFO	internal/sync.go:73	Clean up AWS Users
2020-05-26T12:08:14.979+0100	INFO	internal/sync.go:89	Start group sync
2020-05-26T12:08:15.578+0100	INFO	internal/sync.go:135	Start group user sync	{"group": "AWS Administrators"}
2020-05-26T12:08:15.703+0100	INFO	internal/sync.go:172	Clean up AWS groups
2020-05-26T12:08:15.703+0100	INFO	internal/sync.go:183	Done sync groups
```

## AWS Lambda Usage

NOTE: Using Lambda may incur costs in your AWS account. Please make sure you have checked
the pricing for AWS Lambda and CloudWatch before continuing.

Running ssosync once means that any changes to your Google directory will not appear in
AWS SSO. To sync. regularly, you can run ssosync via AWS Lambda. 

You will find using the provided CDK deployment scripts the easiest method. Install
the [AWS CDK](https://aws.amazon.com/cdk/) before you start.

## SAM

You can use the AWS Serverless Application Model (SAM) to deploy this to your account.

> Please, install the [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html).

Specify an Amazon S3 Bucket for the upload with `export S3_BUCKET=<YOUR_BUCKET>`.

Execute `make package` in the console. Which will package and upload the function to the bucket. You can then use the `packaged.yaml` to configure and deploy the stack in [AWS CloudFormation Console](https://console.aws.amazon.com/cloudformation).

### Using the right binary for AWS Lambda

You require the AMD64 binary for AWS Lambda. This can be either downloaded from the
Releases page, or built locally. A great way to do this to use
[goreleaser](https://goreleaser.com/) in Snapshot mode which will build the various
system binaries.

Whichever route you take, the CDK stack for deployment requires a folder which only
contains the binary and nothing else. goreleaser will take care of this for you; just
be aware if you are obtaining a binary from any other route.

NOTE: The binaries tagged v0.0.1 on GitHub are not suitable for AWS Lambda usage.

To build with goreleaser you can expect the following kind of output:

```
$ goreleaser build --snapshot

   • building...
   • loading config file       file=.goreleaser.yml
   • running before hooks
      • running go mod download
   • loading environment variables
   • getting and validating git state
      • releasing v0.0.1, commit fcc9977a10ae24a92417b00472267ec9bc40aada
      • pipe skipped              error=disabled during snapshot mode
   • parsing tag
   • setting defaults
      • snapshotting
      • github/gitlab/gitea releases
      • project name
      • building binaries
      • creating source archive
      • archives
      • linux packages
      • snapcraft packages
      • calculating checksums
      • signing artifacts
      • docker images
      • artifactory
      • blobs
      • homebrew tap formula
      • scoop manifests
   • snapshotting
   • checking ./dist
   • writing effective config file
      • writing                   config=dist/config.yaml
   • generating changelog
      • pipe skipped              error=not available for snapshots
   • building binaries
      • building                  binary=/Users/leepac/go/src/github.com/awslabs/ssosync/dist/ssosync_windows_amd64/ssosync.exe
      • building                  binary=/Users/leepac/go/src/github.com/awslabs/ssosync/dist/ssosync_linux_arm64/ssosync
      • building                  binary=/Users/leepac/go/src/github.com/awslabs/ssosync/dist/ssosync_linux_386/ssosync
      • building                  binary=/Users/leepac/go/src/github.com/awslabs/ssosync/dist/ssosync_linux_arm_6/ssosync
      • building                  binary=/Users/leepac/go/src/github.com/awslabs/ssosync/dist/ssosync_linux_amd64/ssosync
      • building                  binary=/Users/leepac/go/src/github.com/awslabs/ssosync/dist/ssosync_darwin_amd64/ssosync
   • build succeeded after 7.31s
```

### Deploying using the AWS CDK

You need to know the locations of the credentials.json, token.json and aws.toml files
that you used for the configuration of ssosync. You also need the binary folder location.

With these files in hand, head into the `deployments/cdk` folder and then run the cdk
deploy command with the AWS_TOML, GOOGLE_CREDENTIALS, GOOGLE_TOKEN and SSOSYNC_PATH
variables set:

NOTE: You might get a warning showing you need to execute `cdk bootstrap` if you have
never used the AWS CDK in the account/region before. You can just run that command
beforehand to solve this.

#### *nix

```
AWS_TOML=../../aws.toml GOOGLE_CREDENTIALS=../../credentials.json GOOGLE_TOKEN=../../token.json SSOSYNC_PATH=../../dist/ssosync_linux_amd64 cdk deploy
```

#### Windows (PowerShell)

```
$env:AWS_TOML = '../../aws.toml'
$env:GOOGLE_CREDENTIALS = '../../credentials.json'
$env:GOOGLE_TOKEN = '../../token.json'
$env:SSOSYNC_PATH = '../../dist/ssosync_linux_amd64'
cdk deploy
```

```
$ AWS_TOML=../../aws.toml GOOGLE_CREDENTIALS=../../credentials.json GOOGLE_TOKEN=../../token.json SSOSYNC_PATH=../../dist/ssosync_linux_amd64 cdk deploy
  This deployment will make potentially sensitive changes according to your current security approval level (--require-approval broadening).
  Please confirm you intend to make the following modifications:
  
  IAM Statement Changes
  ┌───┬──────────────────────────────────┬────────┬──────────────────────────────────┬──────────────────────────────────┬────────────────────────────────────┐
  │   │ Resource                         │ Effect │ Action                           │ Principal                        │ Condition                          │
  ├───┼──────────────────────────────────┼────────┼──────────────────────────────────┼──────────────────────────────────┼────────────────────────────────────┤
  │ + │ ${AwsToml}                       │ Allow  │ secretsmanager:GetSecretValue    │ AWS:${SsoSync/ServiceRole}       │                                    │
  │   │ ${GoogleCred}                    │        │                                  │                                  │                                    │
  │   │ ${GoogleToken}                   │        │                                  │                                  │                                    │
  ├───┼──────────────────────────────────┼────────┼──────────────────────────────────┼──────────────────────────────────┼────────────────────────────────────┤
  │ + │ ${SsoSync.Arn}                   │ Allow  │ lambda:InvokeFunction            │ Service:events.amazonaws.com     │ "ArnLike": {                       │
  │   │                                  │        │                                  │                                  │   "AWS:SourceArn": "${Rule.Arn}"   │
  │   │                                  │        │                                  │                                  │ }                                  │
  ├───┼──────────────────────────────────┼────────┼──────────────────────────────────┼──────────────────────────────────┼────────────────────────────────────┤
  │ + │ ${SsoSync/ServiceRole.Arn}       │ Allow  │ sts:AssumeRole                   │ Service:lambda.amazonaws.com     │                                    │
  └───┴──────────────────────────────────┴────────┴──────────────────────────────────┴──────────────────────────────────┴────────────────────────────────────┘
  IAM Policy Changes
  ┌───┬────────────────────────┬────────────────────────────────────────────────────────────────────────────────┐
  │   │ Resource               │ Managed Policy ARN                                                             │
  ├───┼────────────────────────┼────────────────────────────────────────────────────────────────────────────────┤
  │ + │ ${SsoSync/ServiceRole} │ arn:${AWS::Partition}:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole │
  └───┴────────────────────────┴────────────────────────────────────────────────────────────────────────────────┘
  (NOTE: There may be security-related changes not in this list. See https://github.com/aws/aws-cdk/issues/1299)
  
  Do you wish to deploy these changes (y/n)? y
  SsoSyncStack: deploying...
  [0%] start: Publishing d5e2919f38e8204910b42413d033318f5f422a8489e3bbe706bb21458622971e:current
  [100%] success: Published d5e2919f38e8204910b42413d033318f5f422a8489e3bbe706bb21458622971e:current
  SsoSyncStack: creating CloudFormation changeset...
   0/10 | 13:22:57 | CREATE_IN_PROGRESS   | AWS::SecretsManager::Secret | AwsToml
   0/10 | 13:22:57 | CREATE_IN_PROGRESS   | AWS::CDK::Metadata          | CDKMetadata
   0/10 | 13:22:57 | CREATE_IN_PROGRESS   | AWS::SecretsManager::Secret | GoogleCred
   0/10 | 13:22:57 | CREATE_IN_PROGRESS   | AWS::SecretsManager::Secret | GoogleToken
   0/10 | 13:22:57 | CREATE_IN_PROGRESS   | AWS::IAM::Role              | SsoSync/ServiceRole (SsoSyncServiceRoleE85B4FFE)
   0/10 | 13:22:57 | CREATE_IN_PROGRESS   | AWS::IAM::Role              | SsoSync/ServiceRole (SsoSyncServiceRoleE85B4FFE) Resource creation Initiated
   0/10 | 13:22:59 | CREATE_IN_PROGRESS   | AWS::SecretsManager::Secret | GoogleCred Resource creation Initiated
   0/10 | 13:22:59 | CREATE_IN_PROGRESS   | AWS::SecretsManager::Secret | AwsToml Resource creation Initiated
   1/10 | 13:22:59 | CREATE_COMPLETE      | AWS::SecretsManager::Secret | GoogleCred
   1/10 | 13:22:59 | CREATE_IN_PROGRESS   | AWS::SecretsManager::Secret | GoogleToken Resource creation Initiated
   2/10 | 13:22:59 | CREATE_COMPLETE      | AWS::SecretsManager::Secret | AwsToml
   2/10 | 13:22:59 | CREATE_IN_PROGRESS   | AWS::CDK::Metadata          | CDKMetadata Resource creation Initiated
   3/10 | 13:22:59 | CREATE_COMPLETE      | AWS::SecretsManager::Secret | GoogleToken
   4/10 | 13:22:59 | CREATE_COMPLETE      | AWS::CDK::Metadata          | CDKMetadata
   5/10 | 13:23:11 | CREATE_COMPLETE      | AWS::IAM::Role              | SsoSync/ServiceRole (SsoSyncServiceRoleE85B4FFE)
   5/10 | 13:23:13 | CREATE_IN_PROGRESS   | AWS::IAM::Policy            | SsoSync/ServiceRole/DefaultPolicy (SsoSyncServiceRoleDefaultPolicy1A9D4C1C)
   5/10 | 13:23:14 | CREATE_IN_PROGRESS   | AWS::IAM::Policy            | SsoSync/ServiceRole/DefaultPolicy (SsoSyncServiceRoleDefaultPolicy1A9D4C1C) Resource creation Initiated
   6/10 | 13:23:27 | CREATE_COMPLETE      | AWS::IAM::Policy            | SsoSync/ServiceRole/DefaultPolicy (SsoSyncServiceRoleDefaultPolicy1A9D4C1C)
   6/10 | 13:23:30 | CREATE_IN_PROGRESS   | AWS::Lambda::Function       | SsoSync (SsoSync48C335B6)
   6/10 | 13:23:31 | CREATE_IN_PROGRESS   | AWS::Lambda::Function       | SsoSync (SsoSync48C335B6) Resource creation Initiated
   7/10 | 13:23:31 | CREATE_COMPLETE      | AWS::Lambda::Function       | SsoSync (SsoSync48C335B6)
   7/10 | 13:23:34 | CREATE_IN_PROGRESS   | AWS::Events::Rule           | Rule (Rule4C995B7F)
   7/10 | 13:23:34 | CREATE_IN_PROGRESS   | AWS::Events::Rule           | Rule (Rule4C995B7F) Resource creation Initiated
  7/10 Currently in progress: Rule4C995B7F
   8/10 | 13:24:35 | CREATE_COMPLETE      | AWS::Events::Rule           | Rule (Rule4C995B7F)
   8/10 | 13:24:37 | CREATE_IN_PROGRESS   | AWS::Lambda::Permission     | SsoSync/AllowEventRuleSsoSyncStackRule051D4243 (SsoSyncAllowEventRuleSsoSyncStackRule051D4243FDBD7EFC)
   8/10 | 13:24:38 | CREATE_IN_PROGRESS   | AWS::Lambda::Permission     | SsoSync/AllowEventRuleSsoSyncStackRule051D4243 (SsoSyncAllowEventRuleSsoSyncStackRule051D4243FDBD7EFC) Resource creation Initiated
  
   ✅  SsoSyncStack
  
  Stack ARN:
  arn:aws:cloudformation:us-east-1:xxxx:stack/SsoSyncStack/b2297840-xxxx-xxxx-xxxx-0ea20f614b35
```
