#!/bin/bash

# AWS_ACCESS_KEY_ID and SECRET_KEY must be set, so aws client can connect

exec /ssosync "--access-token" "$AWS_SSO_SCIM_API_ACCESS_TOKEN" \
    "--endpoint" "$AWS_SSO_SCIM_API_ENDPOINT" \
    "--google-admin" "$GOOGLE_WORKSPACE_ADMIN_USER_EMAIL" \
    "--google-credentials" "$GOOGLE_WORKSPACE_CREDENTIALS_FILE" \
    "--group-match" "$GOOGLE_WORKSPACE_GROUPS" \
    "--log-level" debug \
    "--region" "$AWS_REGION" \
    "--identity-store-id" "$AWS_SSO_SCIM_IDENTITY_STORE_ID"
