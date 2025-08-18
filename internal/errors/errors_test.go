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

package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/smithy-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
)

func TestInterpretSCIMError(t *testing.T) {
	tests := []struct {
		name       string
		operation  string
		statusCode int
		wantMsg    string
	}{
		{
			name:       "401 Unauthorized",
			operation:  "CreateUser",
			statusCode: http.StatusUnauthorized,
			wantMsg:    "Authentication failed - the SCIM access token is invalid or expired",
		},
		{
			name:       "403 Forbidden",
			operation:  "UpdateUser",
			statusCode: http.StatusForbidden,
			wantMsg:    "Access denied - insufficient permissions for SCIM operations",
		},
		{
			name:       "404 Not Found",
			operation:  "FindUser",
			statusCode: http.StatusNotFound,
			wantMsg:    "SCIM endpoint not found or resource doesn't exist",
		},
		{
			name:       "409 Conflict",
			operation:  "CreateGroup",
			statusCode: http.StatusConflict,
			wantMsg:    "Resource conflict - the resource already exists or is in an inconsistent state",
		},
		{
			name:       "429 Too Many Requests",
			operation:  "ListUsers",
			statusCode: http.StatusTooManyRequests,
			wantMsg:    "Rate limit exceeded - too many requests to the SCIM API",
		},
		{
			name:       "500 Internal Server Error",
			operation:  "DeleteUser",
			statusCode: http.StatusInternalServerError,
			wantMsg:    "Internal server error in AWS SSO SCIM service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalErr := errors.New("original error")
			apiErr := InterpretSCIMError(tt.operation, tt.statusCode, originalErr)

			if apiErr.Service != "AWS SSO SCIM" {
				t.Errorf("Expected service 'AWS SSO SCIM', got '%s'", apiErr.Service)
			}

			if apiErr.Operation != tt.operation {
				t.Errorf("Expected operation '%s', got '%s'", tt.operation, apiErr.Operation)
			}

			if apiErr.StatusCode != tt.statusCode {
				t.Errorf("Expected status code %d, got %d", tt.statusCode, apiErr.StatusCode)
			}

			if apiErr.UserMessage != tt.wantMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.wantMsg, apiErr.UserMessage)
			}

			if len(apiErr.Suggestions) == 0 {
				t.Error("Expected suggestions to be provided")
			}

			if apiErr.Unwrap() != originalErr {
				t.Error("Expected Unwrap() to return original error")
			}
		})
	}
}

func TestInterpretGoogleAPIError(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		originalErr error
		wantMsg     string
	}{
		{
			name:      "Google API 401 Error",
			operation: "GetUsers",
			originalErr: &googleapi.Error{
				Code:    http.StatusUnauthorized,
				Message: "Invalid credentials",
			},
			wantMsg: "Authentication failed - Google service account credentials are invalid",
		},
		{
			name:      "Google API 403 Domain Delegation Error",
			operation: "GetGroups",
			originalErr: &googleapi.Error{
				Code:    http.StatusForbidden,
				Message: "domain-wide delegation not enabled",
			},
			wantMsg: "Domain-wide delegation not properly configured",
		},
		{
			name:      "Google API 404 Error",
			operation: "GetGroupMembers",
			originalErr: &googleapi.Error{
				Code:    http.StatusNotFound,
				Message: "Resource not found",
			},
			wantMsg: "Resource not found in Google Workspace",
		},
		{
			name:        "Non-Google API Error",
			operation:   "GetUsers",
			originalErr: errors.New("network error"),
			wantMsg:     "Failed to communicate with Google Workspace API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := InterpretGoogleAPIError(tt.operation, tt.originalErr)

			if apiErr.Service != "Google Workspace" {
				t.Errorf("Expected service 'Google Workspace', got '%s'", apiErr.Service)
			}

			if apiErr.Operation != tt.operation {
				t.Errorf("Expected operation '%s', got '%s'", tt.operation, apiErr.Operation)
			}

			if apiErr.UserMessage != tt.wantMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.wantMsg, apiErr.UserMessage)
			}

			if len(apiErr.Suggestions) == 0 {
				t.Error("Expected suggestions to be provided")
			}

			if apiErr.Unwrap() != tt.originalErr {
				t.Error("Expected Unwrap() to return original error")
			}
		})
	}
}

func TestInterpretIdentityStoreError(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		originalErr error
		wantMsg     string
	}{
		{
			name:      "Access Denied Error",
			operation: "CreateGroup",
			originalErr: &smithy.GenericAPIError{
				Code:    "AccessDenied",
				Message: "Access denied",
			},
			wantMsg: "Access denied - insufficient IAM permissions for Identity Store operations",
		},
		{
			name:      "Resource Not Found Error",
			operation: "DeleteUser",
			originalErr: &smithy.GenericAPIError{
				Code:    "ResourceNotFoundException",
				Message: "Resource not found",
			},
			wantMsg: "Identity Store resource not found",
		},
		{
			name:      "Throttling Error",
			operation: "ListUsers",
			originalErr: &smithy.GenericAPIError{
				Code:    "ThrottlingException",
				Message: "Rate exceeded",
			},
			wantMsg: "Rate limit exceeded for Identity Store API",
		},
		{
			name:        "Non-AWS SDK Error",
			operation:   "CreateUser",
			originalErr: errors.New("network error"),
			wantMsg:     "Failed to communicate with AWS Identity Store API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := InterpretIdentityStoreError(tt.operation, tt.originalErr)

			if apiErr.Service != "AWS Identity Store" {
				t.Errorf("Expected service 'AWS Identity Store', got '%s'", apiErr.Service)
			}

			if apiErr.Operation != tt.operation {
				t.Errorf("Expected operation '%s', got '%s'", tt.operation, apiErr.Operation)
			}

			if apiErr.UserMessage != tt.wantMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.wantMsg, apiErr.UserMessage)
			}

			if len(apiErr.Suggestions) == 0 {
				t.Error("Expected suggestions to be provided")
			}

			if apiErr.Unwrap() != tt.originalErr {
				t.Error("Expected Unwrap() to return original error")
			}
		})
	}
}

func TestAPIErrorError(t *testing.T) {
	originalErr := errors.New("original error")
	apiErr := &APIError{
		Service:     "Test Service",
		Operation:   "TestOperation",
		StatusCode:  500,
		OriginalErr: originalErr,
		UserMessage: "Test user message",
	}

	expected := "Test Service TestOperation failed: Test user message"
	if apiErr.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, apiErr.Error())
	}

	// Test without user message
	apiErr.UserMessage = ""
	expected = "Test Service TestOperation failed with status 500: original error"
	if apiErr.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, apiErr.Error())
	}
}

func TestLoggingConfig(t *testing.T) {
	// Test default config
	defaultConfig := GetLoggingConfig()
	if defaultConfig.LogSuggestions {
		t.Error("Expected default LogSuggestions to be false (cost-conscious default)")
	}

	// Test setting config
	newConfig := &LoggingConfig{
		LogSuggestions: false,
		LogLevel:       log.WarnLevel,
	}
	SetLoggingConfig(newConfig)

	retrievedConfig := GetLoggingConfig()
	if retrievedConfig.LogSuggestions {
		t.Error("Expected LogSuggestions to be false after setting")
	}
	if retrievedConfig.LogLevel != log.WarnLevel {
		t.Errorf("Expected LogLevel to be %v, got %v", log.WarnLevel, retrievedConfig.LogLevel)
	}

	// Reset to default
	SetLoggingConfig(&LoggingConfig{
		LogSuggestions: false,
		LogLevel:       log.ErrorLevel,
	})
}

func TestHandlerFunctions(t *testing.T) {
	// Test HandleSCIMError
	originalErr := errors.New("test error")
	scimErr := HandleSCIMError("TestOperation", http.StatusUnauthorized, originalErr)

	apiErr, ok := scimErr.(*APIError)
	if !ok {
		t.Fatal("Expected HandleSCIMError to return *APIError")
	}
	if apiErr.Service != "AWS SSO SCIM" {
		t.Errorf("Expected service 'AWS SSO SCIM', got '%s'", apiErr.Service)
	}

	// Test HandleGoogleAPIError
	googleErr := &googleapi.Error{
		Code:    http.StatusForbidden,
		Message: "domain-wide delegation not enabled",
	}
	googleAPIErr := HandleGoogleAPIError("TestOperation", googleErr)

	apiErr, ok = googleAPIErr.(*APIError)
	if !ok {
		t.Fatal("Expected HandleGoogleAPIError to return *APIError")
	}
	if apiErr.Service != "Google Workspace" {
		t.Errorf("Expected service 'Google Workspace', got '%s'", apiErr.Service)
	}

	// Test HandleIdentityStoreError
	identityStoreErr := HandleIdentityStoreError("TestOperation", originalErr)

	apiErr, ok = identityStoreErr.(*APIError)
	if !ok {
		t.Fatal("Expected HandleIdentityStoreError to return *APIError")
	}
	if apiErr.Service != "AWS Identity Store" {
		t.Errorf("Expected service 'AWS Identity Store', got '%s'", apiErr.Service)
	}
}
