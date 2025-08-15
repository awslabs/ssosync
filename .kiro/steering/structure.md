# Project Structure

## Root Level
- `main.go` - Application entry point
- `go.mod/go.sum` - Go module dependencies
- `Makefile` - Build automation and common tasks
- `template.yaml` - AWS SAM CloudFormation template
- `.goreleaser.yml` - Cross-platform build configuration

## Core Application (`cmd/`)
- `cmd/root.go` - Cobra CLI setup, Lambda handler, and configuration initialization
- Contains version information and command-line flag definitions

## Internal Packages (`internal/`)

### `internal/aws/`
- AWS-specific client implementations
- `client.go` - SCIM API client for AWS SSO
- `client_dry.go` - Dry-run implementation for testing
- `users.go`, `groups.go` - User and group management logic
- `schema.go` - SCIM schema definitions
- `mock/` - Mock implementations for testing

### `internal/config/`
- `config.go` - Configuration structure and defaults
- `secrets.go` - AWS Secrets Manager integration

### `internal/google/`
- `client.go` - Google Workspace Directory API client

### `internal/mocks/`
- Generated mock interfaces for testing

### Root `internal/`
- `sync.go` - Core synchronization logic and orchestration
- `sync_test.go` - Synchronization tests

## CI/CD (`cicd/`)
- `build/` - Build pipeline configurations
- `cloudformation/` - Infrastructure templates
- `deploy_patterns/` - Different deployment configurations
- `release/` - Release automation
- `tests/` - Integration and end-to-end tests

## Conventions

### Package Organization
- `internal/` for private packages not intended for external use
- Service-specific packages (`aws/`, `google/`) for API integrations
- Separate test files alongside implementation files

### Naming Patterns
- Interface types end with `Client` or `API`
- Mock implementations prefixed with `Mock`
- Test files use `_test.go` suffix
- Dry-run implementations use `_dry.go` suffix

### Configuration
- Environment variables use `SSOSYNC_` prefix
- Configuration struct in `internal/config/config.go`
- Secrets handled separately from regular config

### Error Handling
- Custom error types in service packages
- Structured logging with contextual fields
- Graceful degradation for non-critical failures