# Technology Stack

## Language & Runtime
- **Go 1.17+** - Primary programming language
- **AWS Lambda Runtime**: `provided.al2` on ARM64 architecture
- Cross-platform support (Linux, macOS, Windows)

## Key Dependencies
- **AWS SDK Go v2** - AWS service interactions
- **Google Admin SDK** - Google Workspace API access
- **Cobra** - CLI framework and command handling
- **Viper** - Configuration management
- **Logrus** - Structured logging
- **GoMock** - Testing and mocking

## AWS Services
- **IAM Identity Center** (Identity Store API) - User/group management
- **AWS Lambda** - Serverless execution
- **AWS Secrets Manager** - Credential storage
- **AWS KMS** - Secret encryption
- **CloudWatch Events** - Scheduled execution

## Build System & Tools
- **Make** - Build automation
- **GoReleaser** - Cross-platform binary releases
- **AWS SAM** - Serverless application deployment
- **CloudFormation** - Infrastructure as Code

## Common Commands

### Development
```bash
# Install dependencies
make install

# Build locally
make go-build

# Run tests
make test

# Clean build artifacts
make clean
```

### Lambda Development
```bash
# Build for Lambda
make lambda

# Package for deployment
make package

# Deploy with SAM
sam build
sam deploy --guided
```

### Release
```bash
# Create release binaries
goreleaser build --snapshot --rm-dist

# Validate CloudFormation template
aws cloudformation validate-template --template-body file://template.yaml
```

## Configuration
- Environment variables with `SSOSYNC_` prefix
- AWS credentials via standard AWS credential chain
- Google service account JSON credentials
- Supports both file-based and AWS Secrets Manager configuration