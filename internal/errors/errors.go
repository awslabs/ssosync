// Copyright (c) 2020, Amazon.com, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package errors provides enhanced error handling and interpretation for API failures
package errors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/aws/smithy-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
)

// LoggingConfig controls error logging behavior
type LoggingConfig struct {
	// LogSuggestions controls whether troubleshooting suggestions are logged
	LogSuggestions bool
	// LogLevel controls the minimum log level for enhanced errors
	LogLevel log.Level
}

var (
	// Global logging configuration
	loggingConfig = &LoggingConfig{
		LogSuggestions: false,
		LogLevel:       log.ErrorLevel,
	}
	configMutex sync.RWMutex
)

// APIError represents an enhanced API error with user-friendly guidance
type APIError struct {
	Service     string
	Operation   string
	StatusCode  int
	OriginalErr error
	UserMessage string
	Suggestions []string
}

func (e *APIError) Error() string {
	if e.UserMessage != "" {
		return fmt.Sprintf("%s %s failed: %s", e.Service, e.Operation, e.UserMessage)
	}
	return fmt.Sprintf("%s %s failed with status %d: %v", e.Service, e.Operation, e.StatusCode, e.OriginalErr)
}

// Unwrap returns the original error for error chain compatibility
func (e *APIError) Unwrap() error {
	return e.OriginalErr
}

// InterpretSCIMError interprets SCIM API errors and provides user-friendly guidance
func InterpretSCIMError(operation string, statusCode int, originalErr error) *APIError {
	apiErr := &APIError{
		Service:     "AWS SSO SCIM",
		Operation:   operation,
		StatusCode:  statusCode,
		OriginalErr: originalErr,
	}

	switch statusCode {
	case http.StatusUnauthorized: // 401
		apiErr.UserMessage = "Authentication failed - the SCIM access token is invalid or expired"
		apiErr.Suggestions = []string{
			"Check that the SCIM access token is correct",
			"Verify the token hasn't expired (tokens expire after a period of inactivity)",
			"Generate a new SCIM access token in the AWS SSO console",
			"Ensure the token has the necessary permissions for SCIM operations",
		}

	case http.StatusForbidden: // 403
		apiErr.UserMessage = "Access denied - insufficient permissions for SCIM operations"
		apiErr.Suggestions = []string{
			"Verify the SCIM access token has the required permissions",
			"Check that SCIM provisioning is enabled in AWS SSO",
			"Ensure the identity source is configured for external identity provider",
			"Confirm the AWS SSO instance is properly configured",
		}

	case http.StatusNotFound: // 404
		apiErr.UserMessage = "SCIM endpoint not found or resource doesn't exist"
		apiErr.Suggestions = []string{
			"Verify the SCIM endpoint URL is correct",
			"Check that the AWS SSO instance exists and is active",
			"Ensure the identity store ID is valid",
			"Confirm the resource (user/group) exists before attempting operations",
		}

	case http.StatusConflict: // 409
		apiErr.UserMessage = "Resource conflict - the resource already exists or is in an inconsistent state"
		apiErr.Suggestions = []string{
			"Check if the user or group already exists",
			"Verify there are no duplicate email addresses or display names",
			"Try updating the existing resource instead of creating a new one",
			"Wait a moment and retry the operation",
		}

	case http.StatusTooManyRequests: // 429
		apiErr.UserMessage = "Rate limit exceeded - too many requests to the SCIM API"
		apiErr.Suggestions = []string{
			"Reduce the frequency of API calls",
			"Implement exponential backoff retry logic",
			"Consider batching operations if supported",
			"Wait before retrying the operation",
		}

	case http.StatusInternalServerError: // 500
		apiErr.UserMessage = "Internal server error in AWS SSO SCIM service"
		apiErr.Suggestions = []string{
			"This is likely a temporary issue with AWS SSO",
			"Wait a few minutes and retry the operation",
			"Check AWS Service Health Dashboard for any ongoing issues",
			"Contact AWS Support if the issue persists",
		}

	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout: // 502, 503, 504
		apiErr.UserMessage = "AWS SSO SCIM service is temporarily unavailable"
		apiErr.Suggestions = []string{
			"This is likely a temporary service issue",
			"Wait a few minutes and retry the operation",
			"Check AWS Service Health Dashboard for any ongoing issues",
			"Implement retry logic with exponential backoff",
		}

	default:
		if statusCode >= 400 && statusCode < 500 {
			apiErr.UserMessage = "Client error - check your request parameters and authentication"
			apiErr.Suggestions = []string{
				"Verify all required parameters are provided",
				"Check the request format and data types",
				"Ensure authentication credentials are valid",
				"Review the SCIM API documentation for correct usage",
			}
		} else if statusCode >= 500 {
			apiErr.UserMessage = "Server error - AWS SSO SCIM service is experiencing issues"
			apiErr.Suggestions = []string{
				"This is likely a temporary service issue",
				"Wait and retry the operation",
				"Check AWS Service Health Dashboard",
				"Contact AWS Support if the issue persists",
			}
		} else {
			apiErr.UserMessage = fmt.Sprintf("Unexpected HTTP status code: %d", statusCode)
			apiErr.Suggestions = []string{
				"Check the AWS SSO SCIM API documentation",
				"Verify your request is properly formatted",
				"Contact AWS Support for assistance",
			}
		}
	}

	return apiErr
}

// InterpretGoogleAPIError interprets Google API errors and provides user-friendly guidance
func InterpretGoogleAPIError(operation string, originalErr error) *APIError {
	apiErr := &APIError{
		Service:     "Google Workspace",
		Operation:   operation,
		OriginalErr: originalErr,
	}

	// Check if it's a Google API error
	var googleErr *googleapi.Error
	if errors.As(originalErr, &googleErr) {
		apiErr.StatusCode = googleErr.Code

		switch googleErr.Code {
		case http.StatusUnauthorized: // 401
			apiErr.UserMessage = "Authentication failed - Google service account credentials are invalid"
			apiErr.Suggestions = []string{
				"Verify the Google service account JSON credentials are correct",
				"Check that the service account has domain-wide delegation enabled",
				"Ensure the admin email address has the necessary permissions",
				"Confirm the service account key hasn't been revoked or expired",
			}

		case http.StatusForbidden: // 403
			if strings.Contains(googleErr.Message, "domain-wide delegation") {
				apiErr.UserMessage = "Domain-wide delegation not properly configured"
				apiErr.Suggestions = []string{
					"Enable domain-wide delegation for the service account",
					"Add the required OAuth scopes in Google Admin Console",
					"Verify the admin email has super admin privileges",
					"Check that the service account is authorized for the domain",
				}
			} else if strings.Contains(googleErr.Message, "quota") || strings.Contains(googleErr.Message, "rate") {
				apiErr.UserMessage = "API quota or rate limit exceeded"
				apiErr.Suggestions = []string{
					"Reduce the frequency of API calls",
					"Implement exponential backoff retry logic",
					"Check your Google Workspace API quotas",
					"Consider requesting quota increases if needed",
				}
			} else {
				apiErr.UserMessage = "Access denied - insufficient permissions for Google Workspace operations"
				apiErr.Suggestions = []string{
					"Verify the admin email has the required permissions",
					"Check that the service account has the necessary OAuth scopes",
					"Ensure domain-wide delegation is properly configured",
					"Confirm the Google Workspace domain settings allow API access",
				}
			}

		case http.StatusNotFound: // 404
			apiErr.UserMessage = "Resource not found in Google Workspace"
			apiErr.Suggestions = []string{
				"Verify the user or group exists in Google Workspace",
				"Check that the domain name is correct",
				"Ensure the resource hasn't been deleted",
				"Confirm you're querying the correct organizational unit",
			}

		case http.StatusTooManyRequests: // 429
			apiErr.UserMessage = "Rate limit exceeded for Google Workspace API"
			apiErr.Suggestions = []string{
				"Implement exponential backoff retry logic",
				"Reduce the frequency of API calls",
				"Check your API usage against Google's quotas",
				"Consider spreading requests over a longer time period",
			}

		case http.StatusInternalServerError: // 500
			apiErr.UserMessage = "Internal server error in Google Workspace API"
			apiErr.Suggestions = []string{
				"This is likely a temporary issue with Google's services",
				"Wait a few minutes and retry the operation",
				"Check Google Workspace Status page for any ongoing issues",
				"Contact Google Support if the issue persists",
			}

		default:
			apiErr.UserMessage = fmt.Sprintf("Google API error: %s", googleErr.Message)
			apiErr.Suggestions = []string{
				"Check the Google Workspace Admin SDK documentation",
				"Verify your API usage and permissions",
				"Review the error details for specific guidance",
			}
		}
	} else {
		// Handle non-Google API errors
		apiErr.UserMessage = "Failed to communicate with Google Workspace API"
		apiErr.Suggestions = []string{
			"Check your internet connectivity",
			"Verify the Google service account credentials",
			"Ensure the Google Workspace domain is accessible",
			"Review the error details for more information",
		}
	}

	return apiErr
}

// InterpretIdentityStoreError interprets AWS Identity Store API errors and provides user-friendly guidance
func InterpretIdentityStoreError(operation string, originalErr error) *APIError {
	apiErr := &APIError{
		Service:     "AWS Identity Store",
		Operation:   operation,
		OriginalErr: originalErr,
	}

	// Check if it's an AWS SDK error
	var awsErr smithy.APIError
	if errors.As(originalErr, &awsErr) {
		// AWS SDK errors don't have HTTP status codes, so we'll use 0
		apiErr.StatusCode = 0

		switch awsErr.ErrorCode() {
		case "UnauthorizedOperation", "AccessDenied":
			apiErr.UserMessage = "Access denied - insufficient IAM permissions for Identity Store operations"
			apiErr.Suggestions = []string{
				"Verify the IAM role/user has the required Identity Store permissions",
				"Check that the following permissions are granted: identitystore:*",
				"Ensure the AWS credentials are valid and not expired",
				"Confirm the Identity Store ID is correct and accessible",
			}

		case "ResourceNotFound", "ResourceNotFoundException":
			apiErr.UserMessage = "Identity Store resource not found"
			apiErr.Suggestions = []string{
				"Verify the Identity Store ID is correct",
				"Check that the user or group exists in the Identity Store",
				"Ensure you're using the correct AWS region",
				"Confirm the AWS SSO instance is properly configured",
			}

		case "ConflictException":
			apiErr.UserMessage = "Resource conflict - the resource already exists or is in an inconsistent state"
			apiErr.Suggestions = []string{
				"Check if the user or group already exists",
				"Verify there are no duplicate identifiers",
				"Try updating the existing resource instead of creating a new one",
				"Wait a moment and retry the operation",
			}

		case "ThrottlingException":
			apiErr.UserMessage = "Rate limit exceeded for Identity Store API"
			apiErr.Suggestions = []string{
				"Implement exponential backoff retry logic",
				"Reduce the frequency of API calls",
				"Wait before retrying the operation",
				"Consider batching operations if possible",
			}

		case "ValidationException":
			apiErr.UserMessage = "Invalid request parameters for Identity Store operation"
			apiErr.Suggestions = []string{
				"Check that all required parameters are provided",
				"Verify parameter formats and data types",
				"Ensure string lengths are within allowed limits",
				"Review the Identity Store API documentation",
			}

		case "InternalServerException":
			apiErr.UserMessage = "Internal server error in AWS Identity Store service"
			apiErr.Suggestions = []string{
				"This is likely a temporary issue with AWS Identity Store",
				"Wait a few minutes and retry the operation",
				"Check AWS Service Health Dashboard for any ongoing issues",
				"Contact AWS Support if the issue persists",
			}

		case "ServiceUnavailableException":
			apiErr.UserMessage = "AWS Identity Store service is temporarily unavailable"
			apiErr.Suggestions = []string{
				"This is likely a temporary service issue",
				"Wait a few minutes and retry the operation",
				"Check AWS Service Health Dashboard for any ongoing issues",
				"Implement retry logic with exponential backoff",
			}

		default:
			apiErr.UserMessage = fmt.Sprintf("AWS Identity Store error: %s", awsErr.ErrorMessage())
			apiErr.Suggestions = []string{
				"Check the AWS Identity Store API documentation",
				"Verify your IAM permissions and AWS credentials",
				"Review the error details for specific guidance",
			}
		}
	} else {
		// Handle non-AWS SDK errors
		apiErr.UserMessage = "Failed to communicate with AWS Identity Store API"
		apiErr.Suggestions = []string{
			"Check your internet connectivity and AWS region",
			"Verify AWS credentials are properly configured",
			"Ensure the Identity Store service is available in your region",
			"Review the error details for more information",
		}
	}

	return apiErr
}

// SetLoggingConfig updates the global logging configuration
func SetLoggingConfig(config *LoggingConfig) {
	configMutex.Lock()
	defer configMutex.Unlock()
	loggingConfig = config
}

// GetLoggingConfig returns the current logging configuration
func GetLoggingConfig() *LoggingConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return &LoggingConfig{
		LogSuggestions: loggingConfig.LogSuggestions,
		LogLevel:       loggingConfig.LogLevel,
	}
}

// LogEnhancedError logs an enhanced API error with suggestions based on configuration
func LogEnhancedError(err *APIError) {
	configMutex.RLock()
	config := loggingConfig
	configMutex.RUnlock()

	// Check if we should log at this level
	if log.GetLevel() < config.LogLevel {
		return
	}

	logEntry := log.WithFields(log.Fields{
		"service":     err.Service,
		"operation":   err.Operation,
		"status_code": err.StatusCode,
		"error":       err.OriginalErr,
	})

	// Log the main error message
	logEntry.Error(err.UserMessage)

	// Only log suggestions if enabled and we have suggestions
	if config.LogSuggestions && len(err.Suggestions) > 0 {
		log.Info("Troubleshooting suggestions:")
		for i, suggestion := range err.Suggestions {
			log.Infof("  %d. %s", i+1, suggestion)
		}
	}
}

// HandleSCIMError interprets and logs a SCIM API error in a single call
func HandleSCIMError(operation string, statusCode int, originalErr error) error {
	enhancedErr := InterpretSCIMError(operation, statusCode, originalErr)
	LogEnhancedError(enhancedErr)
	return enhancedErr
}

// HandleGoogleAPIError interprets and logs a Google API error in a single call
func HandleGoogleAPIError(operation string, originalErr error) error {
	enhancedErr := InterpretGoogleAPIError(operation, originalErr)
	LogEnhancedError(enhancedErr)
	return enhancedErr
}

// HandleIdentityStoreError interprets and logs an Identity Store API error in a single call
func HandleIdentityStoreError(operation string, originalErr error) error {
	enhancedErr := InterpretIdentityStoreError(operation, originalErr)
	LogEnhancedError(enhancedErr)
	return enhancedErr
}

// WrapSCIMError wraps a SCIM API error with enhanced error information (no logging)
func WrapSCIMError(operation string, statusCode int, originalErr error) error {
	return InterpretSCIMError(operation, statusCode, originalErr)
}

// WrapGoogleAPIError wraps a Google API error with enhanced error information (no logging)
func WrapGoogleAPIError(operation string, originalErr error) error {
	return InterpretGoogleAPIError(operation, originalErr)
}

// WrapIdentityStoreError wraps an Identity Store API error with enhanced error information (no logging)
func WrapIdentityStoreError(operation string, originalErr error) error {
	return InterpretIdentityStoreError(operation, originalErr)
}
