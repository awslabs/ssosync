# Product Overview

SSO Sync is a CLI tool and AWS Lambda function that synchronizes Google Workspace (formerly G Suite) users and groups to AWS IAM Identity Center (formerly AWS SSO). 

## Purpose
- Enables automatic provisioning of Google Workspace users and groups into AWS SSO
- Provides uni-directional sync from Google Workspace to AWS
- Supports both CLI execution and serverless Lambda deployment
- Handles user lifecycle management (create, update, delete, suspend)

## Key Features
- Two sync methods: `groups` (default) and `users_groups` 
- Flexible filtering with Google API query parameters
- Support for ignoring specific users/groups
- Dry-run capability for testing
- Cross-account deployment patterns
- Integration with AWS CodePipeline
- Comprehensive logging and error handling

## Deployment Options
- Local CLI execution
- AWS Lambda via Serverless Application Repository
- AWS SAM deployment
- Multiple CloudFormation deployment patterns (app+secrets, app-only, cross-account)

## Target Users
Organizations using Google Workspace as their identity provider who want to centrally manage AWS access through IAM Identity Center.