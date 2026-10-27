#!/usr/bin/env bash
# Live test of getUserOperations / getGroupOperations (issue #383).
#
# Proves, against the real IAM Identity Center tenant, that:
#  1. the IssuerArn ssosync expects is the Issuer IAM Identity Center stamps on
#     the users and groups it provisioned through this SCIM tenant (exact, case
#     preserved), checked against `aws identitystore list-users/list-groups`;
#  2. those users and groups are classified as ssosync-managed: after a live
#     sync, a dry run proposes nothing (no AdoptForeign every run);
#  3. a record with more than one ExternalId is matched on this tenant's entry;
#  4. the ExternalId-change path is reachable again: with one user's and one
#     group's externalId changed through SCIM, a dry run reports Recreate, a dry
#     run with --force-externalid-update reports UpdateForced, and a live forced
#     run restores them;
#  5. after the restore a dry run converges again.
#
# The changed externalIds are always restored: by the forced live run, or by
# the EXIT trap through SCIM if anything fails first.
#
# Environment (set by buildspec.yml): SCIMEndpointUrl, SCIMAccessToken,
# GoogleAdminEmail (Secrets Manager); GROUP_MATCH (default "name:AWS*").

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # tests/user_group_operations
ROOT="$(cd "$HERE/../.." && pwd)"                      # artifact root
SSOSYNC="$ROOT/ssosync"
LOG_DIR="${LOG_DIR:-$ROOT/logs}"
GROUP_MATCH="${GROUP_MATCH:-name:AWS*}"
mkdir -p "$LOG_DIR"

check() { python3 "$HERE/check_log.py" "$@"; }
FAILED=0
expect() { "$@" || FAILED=1; }

for v in SCIMEndpointUrl SCIMAccessToken GoogleAdminEmail; do
  if [ -z "${!v:-}" ]; then echo "ERROR: $v is not set"; exit 1; fi
done
SCIM="${SCIMEndpointUrl%/}"

aws secretsmanager get-secret-value --secret-id SSOSyncGoogleCredentials \
  --query SecretString --output text > "$ROOT/credentials.json"
"$SSOSYNC" --version

# run_ssosync <log-file> [extra ssosync flags...]; returns ssosync's exit code.
run_ssosync() {
  local log=$1; shift
  echo ">>> ssosync $* (log: $log)"
  local rc=0
  "$SSOSYNC" -t "$SCIMAccessToken" -e "$SCIMEndpointUrl" -u "$GoogleAdminEmail" \
    -c "$ROOT/credentials.json" -s groups -g "$GROUP_MATCH" \
    --log-format json --log-level debug "$@" > "$log" 2>&1 || rc=$?
  cat "$log"
  echo "<<< ssosync exit=$rc"
  return $rc
}

scim() { # scim <method> <path> [body-file]
  local args=(-sS -f -X "$1" -H "Authorization: Bearer $SCIMAccessToken" -H "Content-Type: application/scim+json")
  [ $# -ge 3 ] && args+=(--data-binary "@$3")
  curl "${args[@]}" "$SCIM$2"
}

# --- Restore of the externalIds changed in step 4 (always runs) -------------
USER_ID="" USER_ORIG_DOC="" GROUP_ID="" GROUP_NAME="" GROUP_ORIG_EXT=""
USER_CHANGED=0 GROUP_CHANGED=0 RESTORED=0
restore() {
  local rc=$?
  if [ "$RESTORED" != 1 ]; then
    if [ "$USER_CHANGED" = 1 ]; then
      echo ">>> restoring user $USER_ID externalId through SCIM"
      scim PUT "/Users/$USER_ID" "$USER_ORIG_DOC" > /dev/null || { echo "ERROR: user restore failed"; rc=1; }
    fi
    if [ "$GROUP_CHANGED" = 1 ]; then
      echo ">>> restoring group $GROUP_ID externalId through SCIM"
      group_patch "$GROUP_NAME" "$GROUP_ORIG_EXT" > /dev/null || { echo "ERROR: group restore failed"; rc=1; }
    fi
  fi
  exit $rc
}
trap restore EXIT

group_patch() { # group_patch <displayName> <externalId>  (same operations ssosync's UpdateGroup sends)
  jq -n --arg n "$1" --arg x "$2" '{schemas:["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
    Operations:[{op:"replace",path:"displayName",value:$n},{op:"replace",path:"externalId",value:$x}]}' \
    > "$LOG_DIR/group_patch.json"
  scim PATCH "/Groups/$GROUP_ID" "$LOG_DIR/group_patch.json"
}

# --- 1. Live sync: bring the tenant to the current Google state -------------
LIVE="$LOG_DIR/1_live.log"
run_ssosync "$LIVE"

EXPECTED=$(check field "$LIVE" --msg "getIssuerArn()" --key IssuerArn)
STORE_ID=$(check field "$LIVE" --msg "getIssuerArn()" --key IdentityStoreId)
REGION=$(check field "$LIVE" --msg "getRegion()" --key Region)
echo "Expected IssuerArn: $EXPECTED"
echo "Identity Store: $STORE_ID ($REGION)"

# The IssuerArn account must be the Identity Center instance owner, wherever
# ssosync runs (management, linked or delegated administrator account).
OWNER=$(check field "$LIVE" --msg "getIdentityStoreId()" --key OwnerAccountId)
SOURCE=$(check field "$LIVE" --msg "getIssuerArn()" --key AccountSource)
ISSUER_ACCOUNT=$(check field "$LIVE" --msg "getIssuerArn()" --key AccountId)
CALLER=$(aws sts get-caller-identity --query Account --output text)
echo "Instance owner account: $OWNER; ssosync ran in account: $CALLER (account type: ${TEST_ACCOUNT_TYPE:-not set})"
# TEST_ACCOUNT_TYPE is set by the release pipeline's per-account testing stacks
# (testing.yaml): prove each one exercises the account relationship it names.
case "${TEST_ACCOUNT_TYPE:-}" in
  Management)
    if [ "$CALLER" = "$OWNER" ]; then echo "ok:   Management: running in the instance owner account"
    else echo "FAIL: Management: caller $CALLER is not the instance owner $OWNER"; FAILED=1; fi ;;
  Delegated|NonDelegated)
    if [ "$CALLER" != "$OWNER" ]; then echo "ok:   $TEST_ACCOUNT_TYPE: running outside the instance owner account (non-owner path)"
    else echo "FAIL: $TEST_ACCOUNT_TYPE: caller $CALLER is the instance owner; the non-owner path is not exercised"; FAILED=1; fi ;;
  "")
    [ "$CALLER" = "$OWNER" ] && echo "NOTE: running in the instance owner account; the non-owner path is proven by the release pipeline's Delegated/NonDelegated testing stacks" ;;
  *)
    echo "FAIL: unknown TEST_ACCOUNT_TYPE '$TEST_ACCOUNT_TYPE'"; FAILED=1 ;;
esac
if [ "$SOURCE" = "instanceOwner" ] && [ "$ISSUER_ACCOUNT" = "$OWNER" ]; then
  echo "ok:   IssuerArn account is the instance owner ($OWNER)"
else
  echo "FAIL: IssuerArn account=$ISSUER_ACCOUNT source=$SOURCE, expected the instance owner $OWNER"
  FAILED=1
fi

# --- 2. Converged: everything ssosync manages is Equal ---------------------
CONV1="$LOG_DIR/2_converge.log"
expect run_ssosync "$CONV1" --dry-run
expect check converged "$CONV1"
expect check issuer "$CONV1" --expected "$EXPECTED" --min-users 1 --min-groups 1

# --- 3. Ground truth from the Identity Store (incl. multi-ExternalId) ------
aws identitystore list-users  --identity-store-id "$STORE_ID" --region "$REGION" --output json > "$LOG_DIR/users.json"
aws identitystore list-groups --identity-store-id "$STORE_ID" --region "$REGION" --output json > "$LOG_DIR/groups.json"
expect check truth "$CONV1" --expected "$EXPECTED" --users "$LOG_DIR/users.json" --groups "$LOG_DIR/groups.json"

# --- 4. ExternalId-change path ---------------------------------------------
USER_PICK=$(check pick "$CONV1" --kind user)
GROUP_PICK=$(check pick "$CONV1" --kind group)
IFS=$'\t' read -r USER_ID USER_NAME USER_EXT <<< "$USER_PICK"
IFS=$'\t' read -r GROUP_ID GROUP_NAME GROUP_ORIG_EXT <<< "$GROUP_PICK"
if [ -z "$USER_ID" ] || [ -z "$USER_EXT" ] || [ -z "$GROUP_ID" ] || [ -z "$GROUP_ORIG_EXT" ]; then
  echo "ERROR: could not pick a managed user and group to test the ExternalId-change path"
  exit 1
fi
TAMPER="ssosync-ugo-test-$(date +%s)"
echo "Changing externalId of user '$USER_NAME' ($USER_EXT) and group '$GROUP_NAME' ($GROUP_ORIG_EXT) to $TAMPER"

USER_ORIG_DOC="$LOG_DIR/user_orig.json"
scim GET "/Users/$USER_ID" | jq 'del(.meta)' > "$USER_ORIG_DOC"
jq -e --arg x "$USER_EXT" '.externalId == $x' "$USER_ORIG_DOC" > /dev/null \
  || { echo "ERROR: SCIM user $USER_ID externalId is not $USER_EXT"; exit 1; }
jq --arg x "$TAMPER" '.externalId = $x' "$USER_ORIG_DOC" > "$LOG_DIR/user_tampered.json"
USER_CHANGED=1   # set before the call: a partial failure is still restored
scim PUT "/Users/$USER_ID" "$LOG_DIR/user_tampered.json" > /dev/null
GROUP_CHANGED=1
group_patch "$GROUP_NAME" "$TAMPER" > /dev/null

# 4a. Default: the safe Recreate (issuer still matches, ExternalId differs).
TAMPERED="$LOG_DIR/4a_recreate.log"
expect run_ssosync "$TAMPERED" --dry-run
expect check summary "$TAMPERED" \
  --users recreate=1 updateForced=0 adoptForeign=0 adoptLocal=0 create=0 delete=0 \
  --groups recreate=1 updateForced=0 adoptForeign=0 adoptLocal=0 create=0 delete=0
expect check record "$TAMPERED" --kind user --name "$USER_NAME" \
  --field issuerMatch=true --field externalIdMatch=false --field "externalId=$TAMPER"
expect check record "$TAMPERED" --kind group --name "$GROUP_NAME" \
  --field issuerMatch=true --field externalIdMatch=false --field "externalId=$TAMPER"

# 4b. --force-externalid-update: UpdateForced instead.
FORCED="$LOG_DIR/4b_forced.log"
expect run_ssosync "$FORCED" --dry-run --force-externalid-update
expect check summary "$FORCED" \
  --users updateForced=1 recreate=0 adoptForeign=0 create=0 delete=0 \
  --groups updateForced=1 recreate=0 adoptForeign=0 create=0 delete=0

# 4c. Live forced run: writes the Google ids back (restores the change).
RESTORE="$LOG_DIR/4c_restore.log"
if run_ssosync "$RESTORE" --force-externalid-update; then
  expect check summary "$RESTORE" \
    --users updateForced=1 recreate=0 adoptForeign=0 create=0 delete=0 \
    --groups updateForced=1 recreate=0 adoptForeign=0 create=0 delete=0
  RESTORED=1
else
  FAILED=1
fi

# --- 5. Converged again -----------------------------------------------------
CONV2="$LOG_DIR/5_converge.log"
expect run_ssosync "$CONV2" --dry-run
expect check converged "$CONV2"
expect check issuer "$CONV2" --expected "$EXPECTED" --min-users 1 --min-groups 1
expect check record "$CONV2" --kind user --name "$USER_NAME" --field "externalId=$USER_EXT" --field externalIdMatch=true
expect check record "$CONV2" --kind group --name "$GROUP_NAME" --field "externalId=$GROUP_ORIG_EXT" --field externalIdMatch=true

if [ "$FAILED" -ne 0 ]; then
  echo "=== TEST FAILED ==="
  exit 1
fi
echo "=== TEST PASSED ==="
