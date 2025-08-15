# SSO Sync

![Github Action](https://github.com/awslabs/ssosync/workflows/main/badge.svg)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-61%25-brightgreen.svg?longCache=true&style=flat)</a>
[![Go Report Card](https://goreportcard.com/badge/github.com/awslabs/ssosync)](https://goreportcard.com/report/github.com/awslabs/ssosync)
[![License Apache 2](https://img.shields.io/badge/License-Apache2-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![AWS SDK v2](https://img.shields.io/badge/AWS%20SDK-v2-orange.svg)](https://aws.github.io/aws-sdk-go-v2/)
[![Taylor Swift](https://img.shields.io/badge/secured%20by-taylor%20swift-brightgreen.svg)](https://twitter.com/SwiftOnSecurity)
[![Built with Kiro](https://img.shields.io/badge/built_with_Kiro-9046ff.svg?logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyMCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDIwIDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMy44MDA4MSAxOC41NjYxQzEuMzIzMDYgMjQuMDU3MiA2LjU5OTA0IDI1LjQzNCAxMC40OTA0IDIyLjIyMDVDMTEuNjMzOSAyNS44MjQyIDE1LjkyNiAyMy4xMzYxIDE3LjQ2NTIgMjAuMzQ0NUMyMC44NTc4IDE0LjE5MTUgMTkuNDg3NyA3LjkxNDU5IDE5LjEzNjEgNi42MTk4OEMxNi43MjQ0IC0yLjIwOTcyIDQuNjcwNTUgLTIuMjE4NTIgMi41OTU4MSA2LjY2NDlDMi4xMTEzNiA4LjIxOTQ2IDIuMTAyODQgOS45ODc1MiAxLjgyODQ2IDExLjgyMzNDMS42OTAxMSAxMi43NDkgMS41OTI1OCAxMy4zMzk4IDEuMjM0MzYgMTQuMzEzNUMxLjAyODQxIDE0Ljg3MzMgMC43NDUwNDMgMTUuMzcwNCAwLjI5OTgzMyAxNi4yMDgyQy0wLjM5MTU5NCAxNy41MDk1IC0wLjA5OTg4MDIgMjAuMDIxIDMuNDYzOTcgMTguNzE4NlYxOC43MTk1TDMuODAwODEgMTguNTY2MVoiIGZpbGw9IndoaXRlIj48L3BhdGg+PHBhdGggZD0iTTEwLjk2MTQgMTAuNDQxM0M5Ljk3MjAyIDEwLjQ0MTMgOS44MjQyMiA5LjI1ODkzIDkuODI0MjIgOC41NTQwN0M5LjgyNDIyIDcuOTE3OTEgOS45MzgyNCA3LjQxMjQgMTAuMTU0MiA3LjA5MTk3QzEwLjM0NDEgNi44MTAwMyAxMC42MTU4IDYuNjY2OTkgMTAuOTYxNCA2LjY2Njk5QzExLjMwNzEgNi42NjY5OSAxMS42MDM2IDYuODEyMjggMTEuODEyOCA3LjA5ODkyQzEyLjA1MTEgNy40MjU1NCAxMi4xNzcgNy45Mjg2MSAxMi4xNzcgOC41NTQwN0MxMi4xNzcgOS43MzU5MSAxMS43MjI2IDEwLjQ0MTMgMTAuOTYxNiAxMC40NDEzSDEwLjk2MTRaIiBmaWxsPSJibGFjayI+PC9wYXRoPjxwYXRoIGQ9Ik0xNS4wMzE4IDEwLjQ0MTNDMTQuMDQyMyAxMC40NDEzIDEzLjg5NDUgOS4yNTg5MyAxMy44OTQ1IDguNTU0MDdDMTMuODk0NSA3LjkxNzkxIDE0LjAwODYgNy40MTI0IDE0LjIyNDUgNy4wOTE5N0MxNC40MTQ0IDYuODEwMDMgMTQuNjg2MSA2LjY2Njk5IDE1LjAzMTggNi42NjY5OUMxNS4zNzc0IDYuNjY2OTkgMTUuNjczOSA2LjgxMjI4IDE1Ljg4MzEgNy4wOTg5MkMxNi4xMjE0IDcuNDI1NTQgMTYuMjQ3NCA3LjkyODYxIDE2LjI0NzQgOC41NTQwN0MxNi4yNDc0IDkuNzM1OTEgMTUuNzkzIDEwLjQ0MTMgMTUuMDMxOSAxMC40NDEzSDE1LjAzMThaIiBmaWxsPSJibGFjayI+PC9wYXRoPjwvc3ZnPg==)](https://kiro.dev)

> **Seamlessly synchronize Google Workspace users and groups to AWS IAM Identity Center**

SSO Sync is a powerful CLI tool and AWS Lambda function that enables automatic provisioning of Google Workspace (formerly G Suite) users and groups into AWS IAM Identity Center (formerly AWS SSO). Built with Go and powered by AWS SDK v2, it provides reliable, scalable, and secure identity synchronization.

## ✨ Key Features

- **🔄 Bi-directional Sync**: Supports both `groups` and `users_groups` sync methods
- **🎯 Advanced Filtering**: Flexible user and group filtering with Google API query parameters
- **🛡️ Dry-Run Mode**: Test synchronization without making actual changes
- **⚡ High Performance**: Built with AWS SDK v2 for improved performance and reliability
- **🔧 Multiple Deployment Options**: CLI, AWS Lambda, or AWS SAM deployment
- **📊 Comprehensive Logging**: Structured logging with configurable levels and formats
- **🧪 Extensive Testing**: 61%+ test coverage with comprehensive test suite
- **🔐 Secure**: AWS Secrets Manager integration for credential management
- **📈 Scalable**: Supports large directories with user caching and pagination

## 🚀 Quick Start

Want to dive straight in? Try this [hands-on lab](https://catalog.workshops.aws/control-tower/en-US/authentication-authorization/google-workspace) from the AWS Control Tower Workshop. The lab guides you through the complete setup process for both AWS and Google Workspace using the recommended Lambda deployment from the [AWS Serverless Application Repository](https://console.aws.amazon.com/lambda/home#/create/app?applicationId=arn:aws:serverlessrepo:us-east-2:004480582608:applications/SSOSync).

### Installation Options

| Method | Best For | Setup Time |
|--------|----------|------------|
| **[AWS Serverless App Repository](https://console.aws.amazon.com/lambda/home#/create/app?applicationId=arn:aws:serverlessrepo:us-east-2:004480582608:applications/SSOSync)** | Production use | 5 minutes |
| **CLI Binary** | Local testing, CI/CD | 2 minutes |
| **AWS SAM** | Custom deployments | 10 minutes |

## Why?

As per the [AWS SSO](https://aws.amazon.com/single-sign-on/) Homepage:

> AWS Single Sign-On (SSO) makes it easy to centrally manage access
> to multiple AWS accounts and business applications and provide users
> with single sign-on access to all their assigned accounts and applications
> from one place.

Key part further down:

> With AWS SSO, you can create and manage user identities in AWS SSO's
>identity store, or easily connect to your existing identity source including
> Microsoft Active Directory and **Azure Active Directory (Azure AD)**.

AWS SSO can use other Identity Providers as well... such as Google Apps for Domains. Although AWS SSO
supports a subset of the SCIM protocol for populating users, it currently only has support for Azure AD.

This project provides a CLI tool to pull users and groups from Google and push them into AWS SSO.
`ssosync` deals with removing users as well. The heavily commented code provides you with the detail of
what it is going to do.

## ⚠️ Important Notices

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

## ⚙️ Configuration

SSO Sync requires configuration from both Google Workspace and AWS sides.

### Google Workspace Setup

1. **Enable Admin SDK API**
   - Go to [Google Cloud Console](https://console.developers.google.com/apis)
   - Select *API & Services* > *Enable APIs and Services*
   - Search for *Admin SDK* and *Enable* the API

2. **Create Service Account**
   - Follow this [delegation tutorial](https://developers.google.com/admin-sdk/directory/v1/guides/delegation)
   - Save the JSON credentials file as `credentials.json`
   - Grant domain-wide delegation with these scopes:
     - `https://www.googleapis.com/auth/admin.directory.group.readonly`
     - `https://www.googleapis.com/auth/admin.directory.group.member.readonly`
     - `https://www.googleapis.com/auth/admin.directory.user.readonly`

3. **Specify Admin User**
   - You'll need the email address of a Google Workspace admin user

### AWS Setup

1. **Enable IAM Identity Center**
   - Go to AWS IAM Identity Center console
   - Select Settings → Enable automatic provisioning
   - Copy the **SCIM Endpoint URL** and **Access Token**

2. **Get Identity Store ID**
   - In IAM Identity Center console → Settings
   - Copy the **Identity Store ID** from the Identity Source section

3. **AWS Credentials**
   - Configure AWS credentials using any standard method:
     - AWS credentials file (`~/.aws/credentials`)
     - Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
     - IAM roles (for Lambda deployment)

## 🚀 Usage

### CLI Usage

```bash
./ssosync --help
```

#### Basic Example

```bash
./ssosync \
  --google-admin admin@company.com \
  --google-credentials ./credentials.json \
  --scim-endpoint https://scim.us-east-1.amazonaws.com/... \
  --scim-access-token AQoDYXdzE... \
  --region us-east-1 \
  --identity-store-id d-1234567890 \
  --group-match "name:AWS*"
```

#### Advanced Examples

```bash
# Sync specific groups with dry-run
./ssosync \
  --group-match "name:Engineering*,email:aws-*" \
  --sync-method groups \
  --dry-run \
  --log-level debug

# Sync all users and groups
./ssosync \
  --user-match "*" \
  --group-match "*" \
  --sync-method users_groups

# Ignore specific users/groups
./ssosync \
  --group-match "*" \
  --ignore-users "service@company.com,bot@company.com" \
  --ignore-groups "temp-group@company.com"
```

### Environment Variables

All CLI flags can be set via environment variables with the `SSOSYNC_` prefix:

```bash
export SSOSYNC_GOOGLE_ADMIN="admin@company.com"
export SSOSYNC_GOOGLE_CREDENTIALS="./credentials.json"
export SSOSYNC_SCIM_ENDPOINT="https://scim.us-east-1.amazonaws.com/..."
export SSOSYNC_SCIM_ACCESS_TOKEN="AQoDYXdzE..."
export SSOSYNC_REGION="us-east-1"
export SSOSYNC_IDENTITY_STORE_ID="d-1234567890"
export SSOSYNC_GROUP_MATCH="name:AWS*"
export SSOSYNC_DRY_RUN="true"
```

### Configuration Options

| Flag | Environment Variable | Description | Default |
|------|---------------------|-------------|---------|
| `--google-admin` | `SSOSYNC_GOOGLE_ADMIN` | Google Workspace admin email | Required |
| `--google-credentials` | `SSOSYNC_GOOGLE_CREDENTIALS` | Path to Google credentials JSON | `credentials.json` |
| `--scim-endpoint` | `SSOSYNC_SCIM_ENDPOINT` | AWS SCIM endpoint URL | Required |
| `--scim-access-token` | `SSOSYNC_SCIM_ACCESS_TOKEN` | AWS SCIM access token | Required |
| `--region` | `SSOSYNC_REGION` | AWS region | Required |
| `--identity-store-id` | `SSOSYNC_IDENTITY_STORE_ID` | AWS Identity Store ID | Required |
| `--sync-method` | `SSOSYNC_SYNC_METHOD` | Sync method (`groups` or `users_groups`) | `groups` |
| `--group-match` | `SSOSYNC_GROUP_MATCH` | Google Groups filter query | `*` |
| `--user-match` | `SSOSYNC_USER_MATCH` | Google Users filter query | `""` |
| `--ignore-users` | `SSOSYNC_IGNORE_USERS` | Comma-separated list of users to ignore | `[]` |
| `--ignore-groups` | `SSOSYNC_IGNORE_GROUPS` | Comma-separated list of groups to ignore | `[]` |
| `--include-groups` | `SSOSYNC_INCLUDE_GROUPS` | Include only these groups (users_groups method only) | `[]` |
| `--dry-run` | `SSOSYNC_DRY_RUN` | Enable dry-run mode | `false` |
| `--log-level` | `SSOSYNC_LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `--log-format` | `SSOSYNC_LOG_FORMAT` | Log format (text, json) | `text` |

### Filtering Examples

#### Group Filtering
```bash
# Sync groups starting with "AWS"
--group-match "name:AWS*"

# Sync multiple patterns
--group-match "name:Admin*,email:aws-*"

# Sync specific group
--group-match "name=Administrators"

# Sync all groups
--group-match "*"
```

#### User Filtering
```bash
# Sync users with specific name pattern
--user-match "name:John*"

# Sync users with email pattern
--user-match "email:admin*"

# Sync all users
--user-match "*"

# Complex query
--user-match "name:John*,email:admin*,orgName=Engineering"
```

For complete query syntax, see:
- [Google Groups Search](https://developers.google.com/admin-sdk/directory/v1/guides/search-groups)
- [Google Users Search](https://developers.google.com/admin-sdk/directory/v1/guides/search-users)

## 🔧 Development

### Prerequisites

- Go 1.24+
- Make
- AWS CLI (for deployment)

### Setup Development Environment

```bash
# Clone repository
git clone https://github.com/awslabs/ssosync.git
cd ssosync/

# Install development dependencies
make setup

# Run tests
make test

# Build locally
make go-build

# Run with development configuration
make dev
```

### Available Make Targets

```bash
make help                 # Show all available targets
make setup               # Install all dependencies
make test                # Run tests with coverage
make test-verbose        # Run tests with verbose output
make test-coverage       # Generate HTML coverage report
make lint                # Run linters
make fmt                 # Format code
make go-build           # Build application
make clean              # Clean build artifacts
make ci                 # Run CI pipeline (fmt, vet, test)
```

### Testing

The project includes comprehensive tests with 61%+ coverage:

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Generate coverage report
make test-coverage

# Run integration tests (requires valid credentials)
go test -tags=integration ./internal -v
```

## 🚀 AWS Lambda Deployment

### Serverless Application Repository (Recommended)

Deploy directly from the [AWS Serverless Application Repository](https://console.aws.amazon.com/lambda/home#/create/app?applicationId=arn:aws:serverlessrepo:us-east-2:004480582608:applications/SSOSync).

### Deployment Patterns

| Pattern | Description | Use Case |
|---------|-------------|----------|
| **App + Secrets** | Default mode, creates app and secrets | Single account deployment |
| **App Only** | Creates app, expects existing secrets | Shared secrets across environments |
| **Secrets Only** | Creates secrets without app | Centralized secret management |
| **Cross-Account** | App in one account, secrets in another | Multi-account organizations |

### Manual SAM Deployment

```bash
# Build
sam build

# Deploy with guided setup
sam deploy --guided

# Or deploy with parameters
sam deploy \
  --stack-name ssosync \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides \
    GoogleAdminEmail=admin@company.com \
    ScheduleExpression="rate(15 minutes)"
```

### Lambda Environment Variables

When deployed as Lambda, configuration is managed through environment variables and AWS Secrets Manager:

```bash
# Required secrets (stored in Secrets Manager)
GOOGLE_ADMIN=<secret-arn>
GOOGLE_CREDENTIALS=<secret-arn>
SCIM_ENDPOINT=<secret-arn>
SCIM_ACCESS_TOKEN=<secret-arn>
REGION=<secret-arn>
IDENTITY_STORE_ID=<secret-arn>

# Optional environment variables
LOG_LEVEL=info
LOG_FORMAT=json
SYNC_METHOD=groups
GROUP_MATCH=*
USER_MATCH=
IGNORE_USERS=
IGNORE_GROUPS=
DRY_RUN=false
```

## 📊 Monitoring & Troubleshooting

### CloudWatch Logs

When deployed as Lambda, logs are automatically sent to CloudWatch:

```bash
# View logs
aws logs describe-log-groups --log-group-name-prefix /aws/lambda/ssosync

# Tail logs
aws logs tail /aws/lambda/ssosync-function --follow
```

### Common Issues

#### Rate Limiting
```
Error: AWS SSO SCIM API rate limits errors
```
**Solution**: Reduce sync frequency or implement exponential backoff

#### Authentication Errors
```
Error: cannot read secret: <secret-name>
```
**Solution**: Verify IAM permissions for Secrets Manager access

#### Google API Errors
```
Error: Error Getting Deleted Users
```
**Solution**: Verify Google service account permissions and domain-wide delegation

### Performance Optimization

- **User Caching**: Enable user caching for large directories
- **Filtering**: Use specific group/user filters to reduce API calls
- **Batch Size**: Adjust batch sizes for large sync operations
- **Scheduling**: Set appropriate sync intervals based on directory size

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite: `make ci`
6. Submit a pull request

### Code Standards

- Follow Go best practices
- Maintain test coverage above 60%
- Use structured logging
- Document public APIs
- Follow semantic versioning

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- AWS Labs team for the original implementation
- Google Workspace Directory API team
- AWS IAM Identity Center team
- All contributors and community members

---

**Need help?** Check out our [Issues](https://github.com/awslabs/ssosync/issues) page or start a [Discussion](https://github.com/awslabs/ssosync/discussions).