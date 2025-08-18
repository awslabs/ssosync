# Enhanced Error Handling

This package provides enhanced error handling and interpretation for API failures in SSO Sync. Instead of raw HTTP errors, users now receive user-friendly error messages with actionable troubleshooting guidance.

## Features

- **User-friendly error messages**: Raw HTTP status codes are translated into clear, descriptive messages
- **Actionable troubleshooting suggestions**: Each error includes specific steps to resolve the issue
- **Service-specific guidance**: Different error interpretation for SCIM API, Google Workspace API, and AWS Identity Store API
- **Error chain compatibility**: Enhanced errors maintain the original error for debugging purposes

## Supported APIs

### AWS SSO SCIM API
- **401 Unauthorized**: Invalid or expired SCIM access token
- **403 Forbidden**: Insufficient permissions for SCIM operations
- **404 Not Found**: SCIM endpoint not found or resource doesn't exist
- **409 Conflict**: Resource already exists or is in inconsistent state
- **429 Too Many Requests**: Rate limit exceeded
- **500+ Server Errors**: AWS SSO service issues

### Google Workspace API
- **401 Unauthorized**: Invalid service account credentials
- **403 Forbidden**: Domain-wide delegation issues, quota limits, or insufficient permissions
- **404 Not Found**: Resource not found in Google Workspace
- **429 Too Many Requests**: API rate limit exceeded
- **500+ Server Errors**: Google service issues

### AWS Identity Store API
- **AccessDenied**: Insufficient IAM permissions
- **ResourceNotFoundException**: Identity Store resource not found
- **ConflictException**: Resource conflicts
- **ThrottlingException**: API rate limits
- **ValidationException**: Invalid request parameters
- **InternalServerException**: AWS service issues

## Usage

### One-Call Helper Functions (Recommended)

The simplest way to handle errors is using the helper functions that interpret and log in a single call:

```go
import "github.com/awslabs/ssosync/internal/errors"

// SCIM API Errors - one call handles everything
if resp.StatusCode != http.StatusOK {
    return errors.HandleSCIMError("CreateUser", resp.StatusCode, originalErr)
}

// Google API Errors - one call handles everything
if err != nil {
    return errors.HandleGoogleAPIError("GetUsers", err)
}

// Identity Store API Errors - one call handles everything
if err != nil {
    return errors.HandleIdentityStoreError("CreateGroup", err)
}
```

### Manual Control (Advanced)

For more control over logging behavior, you can use the separate functions:

```go
import "github.com/awslabs/ssosync/internal/errors"

// In your SCIM client code
if resp.StatusCode != http.StatusOK {
    enhancedErr := errors.InterpretSCIMError("CreateUser", resp.StatusCode, originalErr)
    errors.LogEnhancedError(enhancedErr)
    return enhancedErr
}

// Or use wrapper functions (no logging)
if resp.StatusCode != http.StatusOK {
    return errors.WrapSCIMError("CreateUser", resp.StatusCode, originalErr)
}
```

### Controlling Log Output

You can control error logging behavior to reduce log volume and costs:

```go
import "github.com/awslabs/ssosync/internal/errors"

// Disable troubleshooting suggestions to reduce log volume
errors.SetLoggingConfig(&errors.LoggingConfig{
    LogSuggestions: false,
    LogLevel:       log.ErrorLevel,
})

// Or configure via command line flag
// --log-error-suggestions=false
```

## Error Structure

Each enhanced error includes:

- **Service**: The API service that generated the error (e.g., "AWS SSO SCIM", "Google Workspace")
- **Operation**: The specific operation that failed (e.g., "CreateUser", "GetGroups")
- **StatusCode**: HTTP status code (for HTTP-based APIs)
- **UserMessage**: Human-readable description of the error
- **Suggestions**: List of actionable troubleshooting steps
- **OriginalErr**: The original error for debugging purposes

## Example Output

Instead of seeing:
```
Error: status of http response was 401
```

Users now see:
```
AWS SSO SCIM CreateUser failed: Authentication failed - the SCIM access token is invalid or expired

Troubleshooting suggestions:
  1. Check that the SCIM access token is correct
  2. Verify the token hasn't expired (tokens expire after a period of inactivity)
  3. Generate a new SCIM access token in the AWS SSO console
  4. Ensure the token has the necessary permissions for SCIM operations
```

## Benefits

1. **Faster troubleshooting**: Users can quickly identify and resolve common issues
2. **Reduced support burden**: Clear guidance reduces the need for support tickets
3. **Better user experience**: Friendly error messages instead of cryptic HTTP codes
4. **Actionable guidance**: Specific steps to resolve each type of error
5. **Maintains debugging capability**: Original errors are preserved for technical analysis
6. **Cost-conscious logging**: Configurable suggestion logging to control log volume and costs
7. **Simple API**: One-call helper functions reduce boilerplate code

## Configuration

### Command Line Options

- `--log-error-suggestions`: Enable/disable logging of troubleshooting suggestions (default: true)

### Environment Variables

- `LOG_ERROR_SUGGESTIONS`: Set to "false" to disable suggestion logging

### Programmatic Configuration

```go
// Disable suggestions to reduce log volume
errors.SetLoggingConfig(&errors.LoggingConfig{
    LogSuggestions: false,
    LogLevel:       log.ErrorLevel,
})

// Get current configuration
config := errors.GetLoggingConfig()
```

## Testing

The package includes comprehensive tests for all error interpretation functions:

```bash
go test ./internal/errors/... -v
```

Example tests demonstrate the enhanced error handling in action and can be run with:

```bash
go test ./internal/errors/... -run Example
```