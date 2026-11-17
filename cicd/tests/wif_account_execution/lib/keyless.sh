#!/usr/bin/env bash
# Keyless (Workload Identity Federation) ssosync CLI runs for the CodeBuild
# tests, sourced by the attribute-map and user/group-operations helpers.
# Kept bash 3.2 compatible.
#
# The CLI's Google provider (<AccountId>-cli) trusts only
# SSOSync_CLIExecutionRole, so ssosync runs in a session of that role: the
# test's own role keeps everything else (Identity Store resets, log reads).
# Assuming it needs sts:AssumeRole on the role and its trust of the test role.
#
#   keyless_setup <artifact-root>   once, before the first run
#   keyless_ssosync <args...>       run ssosync keyless (adds the auth flags)
#
# Environment (from Secrets Manager, set by the buildspec):
#   GoogleCustomerId, GoogleImpersonateServiceAccount
# The credential configuration is written by the staging buildspec as
# tests/wif_account_execution/cli/wif-config-<AccountId>.json.

KEYLESS_ROLE_NAME="SSOSync_CLIExecutionRole"

keyless_setup() {
  local root=$1 v
  for v in GoogleCustomerId GoogleImpersonateServiceAccount; do
    if [ -z "${!v:-}" ]; then echo "ERROR: required environment variable $v is not set"; exit 1; fi
  done
  KEYLESS_SSOSYNC="$root/ssosync"
  KEYLESS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
  KEYLESS_WIF_CONFIG="$root/tests/wif_account_execution/cli/wif-config-${KEYLESS_ACCOUNT_ID}.json"
  if [ ! -f "$KEYLESS_WIF_CONFIG" ]; then
    echo "ERROR: $KEYLESS_WIF_CONFIG not found (written by the staging buildspec)"
    exit 1
  fi
  KEYLESS_ROLE_ARN="arn:aws:iam::${KEYLESS_ACCOUNT_ID}:role/${KEYLESS_ROLE_NAME}"
  echo "keyless: audience $(jq -r .audience "$KEYLESS_WIF_CONFIG"), role $KEYLESS_ROLE_ARN"
}

# keyless_ssosync <args...>: a fresh session per run (role chaining caps a
# session at one hour). Google's AWS credential source reads AWS_ACCESS_KEY_ID /
# AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN, so they are set for ssosync only.
keyless_ssosync() {
  local creds ak sk st
  creds="$(aws sts assume-role --role-arn "$KEYLESS_ROLE_ARN" --role-session-name ssosync-test \
    --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' --output text)" || return 1
  read -r ak sk st <<<"$creds"
  env -u AWS_CONTAINER_CREDENTIALS_RELATIVE_URI -u AWS_CONTAINER_CREDENTIALS_FULL_URI -u AWS_PROFILE \
    AWS_ACCESS_KEY_ID="$ak" AWS_SECRET_ACCESS_KEY="$sk" AWS_SESSION_TOKEN="$st" \
    "$KEYLESS_SSOSYNC" \
      --customer-id "$GoogleCustomerId" \
      --google-impersonate-service-account "$GoogleImpersonateServiceAccount" \
      --google-wif-config "$KEYLESS_WIF_CONFIG" \
      "$@"
}
