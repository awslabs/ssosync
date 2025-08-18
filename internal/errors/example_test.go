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

package errors_test

import (
	"fmt"
	"net/http"

	"github.com/awslabs/ssosync/internal/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
)

// ExampleInterpretSCIMError demonstrates how SCIM API errors are enhanced with user-friendly guidance
func ExampleInterpretSCIMError() {
	// Simulate a 401 Unauthorized error from SCIM API
	originalErr := fmt.Errorf("HTTP 401: Unauthorized")
	enhancedErr := errors.InterpretSCIMError("CreateUser", http.StatusUnauthorized, originalErr)

	fmt.Println("Service:", enhancedErr.Service)
	fmt.Println("Operation:", enhancedErr.Operation)
	fmt.Println("User Message:", enhancedErr.UserMessage)
	fmt.Println("Number of suggestions:", len(enhancedErr.Suggestions))
	fmt.Println("First suggestion:", enhancedErr.Suggestions[0])

	// Output:
	// Service: AWS SSO SCIM
	// Operation: CreateUser
	// User Message: Authentication failed - the SCIM access token is invalid or expired
	// Number of suggestions: 4
	// First suggestion: Check that the SCIM access token is correct
}

// ExampleInterpretGoogleAPIError demonstrates how Google API errors are enhanced with user-friendly guidance
func ExampleInterpretGoogleAPIError() {
	// Simulate a Google API 403 error with domain delegation issue
	originalErr := &googleapi.Error{
		Code:    http.StatusForbidden,
		Message: "domain-wide delegation not enabled",
	}
	enhancedErr := errors.InterpretGoogleAPIError("GetUsers", originalErr)

	fmt.Println("Service:", enhancedErr.Service)
	fmt.Println("Operation:", enhancedErr.Operation)
	fmt.Println("User Message:", enhancedErr.UserMessage)
	fmt.Println("Number of suggestions:", len(enhancedErr.Suggestions))
	fmt.Println("First suggestion:", enhancedErr.Suggestions[0])

	// Output:
	// Service: Google Workspace
	// Operation: GetUsers
	// User Message: Domain-wide delegation not properly configured
	// Number of suggestions: 4
	// First suggestion: Enable domain-wide delegation for the service account
}

// ExampleAPIError_Error demonstrates the error message formatting
func ExampleAPIError_Error() {
	apiErr := &errors.APIError{
		Service:     "AWS SSO SCIM",
		Operation:   "CreateUser",
		StatusCode:  401,
		UserMessage: "Authentication failed - the SCIM access token is invalid or expired",
	}

	fmt.Println(apiErr.Error())

	// Output:
	// AWS SSO SCIM CreateUser failed: Authentication failed - the SCIM access token is invalid or expired
}

// ExampleHandleSCIMError demonstrates the one-call helper function
func ExampleHandleSCIMError() {
	// Simulate a 401 error and handle it in one call
	originalErr := fmt.Errorf("HTTP 401: Unauthorized")

	// This interprets the error AND logs it automatically
	enhancedErr := errors.HandleSCIMError("CreateUser", http.StatusUnauthorized, originalErr)

	fmt.Println("Error type:", fmt.Sprintf("%T", enhancedErr))
	fmt.Println("Service:", enhancedErr.(*errors.APIError).Service)
	fmt.Println("User Message:", enhancedErr.(*errors.APIError).UserMessage)

	// Output:
	// Error type: *errors.APIError
	// Service: AWS SSO SCIM
	// User Message: Authentication failed - the SCIM access token is invalid or expired
}

// ExampleSetLoggingConfig demonstrates how to control error logging
func ExampleSetLoggingConfig() {
	// Disable suggestion logging to reduce log volume
	errors.SetLoggingConfig(&errors.LoggingConfig{
		LogSuggestions: false,
		LogLevel:       log.ErrorLevel,
	})

	// Now errors will be logged but suggestions won't be
	originalErr := fmt.Errorf("HTTP 401: Unauthorized")
	errors.HandleSCIMError("CreateUser", http.StatusUnauthorized, originalErr)

	// Re-enable suggestions
	errors.SetLoggingConfig(&errors.LoggingConfig{
		LogSuggestions: true,
		LogLevel:       log.ErrorLevel,
	})

	fmt.Println("Logging configuration updated")

	// Output:
	// Logging configuration updated
}
