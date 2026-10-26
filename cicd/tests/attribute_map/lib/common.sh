#!/usr/bin/env bash
# Shared helpers for the attribute-map integration tests.
# Each test's run.sh sources this file:
#   source "$(dirname "$0")/../lib/common.sh"
#
# Layout of the CodePipeline "Tests" artifact (CODEBUILD_SRC_DIR):
#   ssosync                       the binary under test
#   deploy/                       stack.yml + parameter files
#   tests/attribute_map/          these tests (import.py, export.py, baseline/, maps/, lib/, <test>/)
#
# Environment (set by each test's buildspec.yml):
#   SCIMEndpointUrl, SCIMAccessToken, GoogleAdminEmail   CLI tests only (from Secrets Manager)
#   GROUP_MATCH       Google group filter (default "name:AWS*")
#   FUNCTION_NAME     Lambda under test (default "SSOSyncFunction")

set -euo pipefail

AM_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"   # tests/attribute_map
ROOT_DIR="$(cd "$AM_DIR/../.." && pwd)"                       # artifact root
SSOSYNC="$ROOT_DIR/ssosync"
MAPS="$AM_DIR/maps"
GROUP_MATCH="${GROUP_MATCH:-name:AWS*}"
FUNCTION_NAME="${FUNCTION_NAME:-SSOSyncFunction}"
LOG_DIR="${LOG_DIR:-$ROOT_DIR/logs}"
mkdir -p "$LOG_DIR"

assert_log() { python3 "$AM_DIR/lib/assert_log.py" "$@"; }

# check <command...>: run an assertion but keep going if it fails, so a test
# reports every failed assertion. Call finish at the end of run.sh.
FAILED=0
check() { "$@" || FAILED=1; }
finish() {
  if [ "$FAILED" -ne 0 ]; then
    echo "=== TEST FAILED ==="
    exit 1
  fi
  echo "=== TEST PASSED ==="
}

require_env() {
  local v
  for v in "$@"; do
    if [ -z "${!v:-}" ]; then
      echo "ERROR: required environment variable $v is not set"
      exit 1
    fi
  done
}

# Python dependencies for import.py / export.py / lambda_logs.py.
setup_python() {
  pip3 install --quiet boto3
}

# Everything a CLI test needs: python deps, SCIM/Google credentials, the binary.
setup_cli() {
  require_env SCIMEndpointUrl SCIMAccessToken GoogleAdminEmail
  setup_python
  aws secretsmanager get-secret-value --secret-id SSOSyncGoogleCredentials \
    --query SecretString --output text > "$ROOT_DIR/credentials.json"
  "$SSOSYNC" --version
}

# run_ssosync <live|dry> <map-file|""> <log-file>
# Runs the groups sync method over GROUP_MATCH, including suspended users, with
# JSON debug logs captured to <log-file> (and echoed to the build log).
# Returns ssosync's exit code. "" for map-file means the built-in default map.
run_ssosync() {
  local mode=$1 map=$2 log=$3
  local args=(-t "$SCIMAccessToken" -e "$SCIMEndpointUrl" -u "$GoogleAdminEmail"
              -c "$ROOT_DIR/credentials.json" -s groups -g "$GROUP_MATCH"
              --suspended --log-format json --log-level debug)
  if [ -n "$map" ]; then args+=(--attribute-map-file "$map"); fi
  if [ "$mode" = "dry" ]; then args+=(--dry-run); fi
  echo ">>> ssosync mode=$mode map=${map:-built-in default} log=$log"
  local rc=0
  "$SSOSYNC" "${args[@]}" > "$log" 2>&1 || rc=$?
  cat "$log"
  echo "<<< ssosync exit=$rc"
  return $rc
}

# Reset the Identity Store to the baseline. Every user and group is created as
# a local (unmanaged) record with no ExternalId.
reset_baseline() {
  echo ">>> reset Identity Store to baseline"
  python3 "$AM_DIR/import.py" --input-dir "$AM_DIR/baseline"
}

# Reset, then run ssosync live with the default map so it adopts the local
# records and sets their ExternalIds (its own SCIM tenant becomes the Issuer).
# Afterwards every baseline user/group is a managed record matched by ExternalId.
reset_and_adopt() {
  reset_baseline
  run_ssosync live "" "$LOG_DIR/adopt.log"
  assert_log summary "$LOG_DIR/adopt.log" --users create=0 delete=0 recreate=0
}

# assert_converged <map-file|""> <log-file>
# A dry-run after a live sync must propose no user, group or membership change.
# In dry-run mode the Change Summary reports what WOULD change in a live run, so
# convergence means every summary count is zero (and, consistently, there are no
# "DRY RUN: Would ..." actions). Returns non-zero if ANY step fails.
assert_converged() {
  local map=$1 log=$2 rc=0
  run_ssosync dry "$map" "$log" || rc=1
  assert_log converged "$log" || rc=1
  assert_log dryrun "$log" || rc=1
  return $rc
}

# assert_dry_run <log-file>
# The log is from a dry run whose Change Summary matches the actions it says it
# WOULD have taken (one "DRY RUN: Would ..." line per summarised change).
assert_dry_run() { assert_log dryrun "$1"; }

# invoke_lambda <log-file>
# Invokes FUNCTION_NAME synchronously, fails on FunctionError, then collects the
# function's CloudWatch log lines for this invocation into <log-file>.
invoke_lambda() {
  local log=$1
  local start=$(( ($(date +%s) - 5) * 1000 ))
  echo ">>> invoke $FUNCTION_NAME"
  aws lambda invoke --cli-read-timeout 900 --function-name "$FUNCTION_NAME" \
    "$ROOT_DIR/response.json" > "$ROOT_DIR/invoke.json"
  cat "$ROOT_DIR/invoke.json"
  if jq -e 'has("FunctionError")' "$ROOT_DIR/invoke.json" > /dev/null; then
    echo "FAIL: Lambda returned FunctionError"
    cat "$ROOT_DIR/response.json"
    return 1
  fi
  python3 "$AM_DIR/lib/lambda_logs.py" --log-group "/aws/lambda/$FUNCTION_NAME" \
    --start "$start" --out "$log"
  cat "$log"
}
