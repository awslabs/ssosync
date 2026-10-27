#!/usr/bin/env bash
# Live test of getUserOperations / getGroupOperations (issue #383).
#
# Every outcome of getUserOperation/getGroupOperation is set up for real in the
# IAM Identity Center tenant, then proven with exact dry-run Change Summaries, a
# live run, and convergence (a dry run straight after proposes nothing):
#
#   S1  Baseline / Equal   expected IssuerArn (instance owner account, case kept)
#                          == the real Identity Store Issuer; managed records Equal
#   S2  Update             managed, same ExternalId, attribute changed (SCIM)
#   S3  Recreate /         managed, ExternalId changed (SCIM): Recreate by default,
#       UpdateForced       UpdateForced with --force-externalid-update
#   S4  Delete (managed)   SCIM-created orphan not in Google; never retained, even with -r
#   S5  Delete / Retain    local (Identity Store API) record not in Google; Retain with -r
#   S6  AdoptLocal         dedicated test user/group re-created as local records
#   S7  Create             dedicated test user/group deleted from the Identity Store
#   S8a AdoptForeign       managed record given a foreign-issuer ExternalId
#   S8b Delete / Retain    foreign record not in Google; Retain with -r
#   S8c Multi-ExternalId   record with a foreign AND this tenant's ExternalId is Equal
#
# S8 needs the Identity Store API to write ExternalIds; a probe checks that first
# and the S8 scenarios are SKIPPED (with the API error) if it cannot, unless
# UGO_FOREIGN_REQUIRED=true, which makes a skip fail the test.
#
# S5/S6 (and S8) create LOCAL records, which only the Identity Store API can do.
# Its writes are refused from an account outside the instance owner without
# delegated administration (NonDelegated): there S5/S6 are SKIPPED and proven by
# the Management/Delegated runs; a refusal in any other account FAILS the test.
# Everything else, including S7, changes records through SCIM, which works from
# every account (as ssosync itself does).
#
# Every change is undone (see lib.sh); S6/S7 give the dedicated test records new
# Identity Store ids, so they must be a Google test user/group with no
# permission-set assignments:
#   UGO_TEST_USER   primary email of a Google test user in an in-scope group
#   UGO_TEST_GROUP  name of an in-scope Google test group
# S2/S3/S8 use them too when set, else the first managed user/group by name.
#
# Environment: SCIMEndpointUrl, SCIMAccessToken, GoogleAdminEmail (Secrets
# Manager), GROUP_MATCH (default "name:AWS*"), TEST_ACCOUNT_TYPE (set by the
# release testing stacks: Management | Delegated | NonDelegated).

set -euo pipefail
source "$(dirname "$0")/lib.sh"

for v in SCIMEndpointUrl SCIMAccessToken GoogleAdminEmail; do
  if [ -z "${!v:-}" ]; then echo "ERROR: $v is not set"; exit 1; fi
done
aws secretsmanager get-secret-value --secret-id SSOSyncGoogleCredentials \
  --query SecretString --output text > "$ROOT/credentials.json"
"$SSOSYNC" --version

finish() {
  local rc=$?
  SCEN=""
  run_undos "" || rc=1
  rm -rf "$CLEANUP_DIR"
  summary_table
  if [ "$FAILED" -ne 0 ] || [ $rc -ne 0 ]; then echo "=== TEST FAILED ==="; exit 1; fi
  echo "=== TEST PASSED ==="
}
trap finish EXIT

# =============================================================================
s1_baseline() {
  local live="$LOG_DIR/S1_live.log"
  run_ssosync "$live" || return 1
  EXPECTED=$(check field "$live" --msg "getIssuerArn()" --key IssuerArn) || return 1
  STORE_ID=$(check field "$live" --msg "getIssuerArn()" --key IdentityStoreId) || return 1
  REGION=$(check field "$live" --msg "getRegion()" --key Region) || return 1
  OWNER=$(check field "$live" --msg "getIdentityStoreId()" --key OwnerAccountId) || return 1
  local source issuer_account caller
  source=$(check field "$live" --msg "getIssuerArn()" --key AccountSource) || return 1
  issuer_account=$(check field "$live" --msg "getIssuerArn()" --key AccountId) || return 1
  caller=$(aws sts get-caller-identity --query Account --output text) || return 1
  FOREIGN_ISSUER="arn:aws:identitystore::${OWNER}:identitystore/${STORE_ID}/provisioningtenant/ssosync-ugo-foreign"
  echo "Expected IssuerArn: $EXPECTED"
  echo "Identity Store: $STORE_ID ($REGION); instance owner: $OWNER; ssosync ran in: $caller (type: ${TEST_ACCOUNT_TYPE:-not set})"

  # The IssuerArn account is the instance owner wherever ssosync runs, and each
  # release testing stack exercises the account relationship it names.
  if [ "$source" = instanceOwner ] && [ "$issuer_account" = "$OWNER" ]; then
    echo "ok:   IssuerArn account is the instance owner ($OWNER)"
  else
    echo "FAIL: IssuerArn account=$issuer_account source=$source, expected the instance owner $OWNER"; S_FAIL=1
  fi
  case "${TEST_ACCOUNT_TYPE:-}" in
    Management)
      if [ "$caller" = "$OWNER" ]; then echo "ok:   Management: running in the instance owner account"
      else echo "FAIL: Management: caller $caller is not the instance owner $OWNER"; S_FAIL=1; fi ;;
    Delegated|NonDelegated)
      if [ "$caller" != "$OWNER" ]; then echo "ok:   $TEST_ACCOUNT_TYPE: running outside the instance owner account (non-owner path)"
      else echo "FAIL: $TEST_ACCOUNT_TYPE: caller $caller is the instance owner; the non-owner path is not exercised"; S_FAIL=1; fi ;;
    "") [ "$caller" = "$OWNER" ] && echo "NOTE: running in the instance owner account; the non-owner path is proven by the release Delegated/NonDelegated testing stacks" ;;
    *) echo "FAIL: unknown TEST_ACCOUNT_TYPE '$TEST_ACCOUNT_TYPE'"; S_FAIL=1 ;;
  esac

  local conv="$LOG_DIR/S1_converge.log"
  converge "$conv"
  ids list-users > "$LOG_DIR/users.json" || return 1
  ids list-groups > "$LOG_DIR/groups.json" || return 1
  expect check truth "$conv" --expected "$EXPECTED" --users "$LOG_DIR/users.json" --groups "$LOG_DIR/groups.json"
  BASE_LOG="$conv"
}

# Resolve the user/group the mutating scenarios act on (dedicated if configured).
resolve_test_records() {
  local log=$1 row
  if [ -n "${UGO_TEST_USER:-}" ]; then
    TU_NAME="$UGO_TEST_USER"
    row=$(check find "$log" --kind user --name "$TU_NAME") || return 1
    IFS=$'\t' read -r TU_ID TU_EXT _ <<< "$row"
  else
    row=$(check pick "$log" --kind user) || return 1
    IFS=$'\t' read -r TU_ID TU_NAME TU_EXT <<< "$row"
  fi
  if [ -n "${UGO_TEST_GROUP:-}" ]; then
    TG_NAME="$UGO_TEST_GROUP"
    row=$(check find "$log" --kind group --name "$TG_NAME") || return 1
    IFS=$'\t' read -r TG_ID TG_EXT _ <<< "$row"
  else
    row=$(check pick "$log" --kind group) || return 1
    IFS=$'\t' read -r TG_ID TG_NAME TG_EXT <<< "$row"
  fi
  [ -n "$TU_ID" ] && [ -n "$TU_EXT" ] && [ -n "$TG_ID" ] && [ -n "$TG_EXT" ] || return 1
  echo "Test user: $TU_NAME ($TU_ID, externalId $TU_EXT); test group: $TG_NAME ($TG_ID, externalId $TG_EXT)"
}

# =============================================================================
s2_update() {
  local orig="$LOG_DIR/S2_user_orig.json" changed="$LOG_DIR/S2_user_changed.json"
  scim GET "/Users/$TU_ID" | jq 'del(.meta)' > "$orig" || return 1
  jq --arg g "ugo-update-$TS" '.name.givenName = $g' "$orig" > "$changed" || return 1
  undo user user_put "$TU_ID" "$orig"
  user_put "$TU_ID" "$changed" || return 1
  undo group group_patch "$TG_ID" "$TG_NAME" "$TG_EXT"
  group_patch "$TG_ID" "ugo-update-$TS" "$TG_EXT" || return 1

  local dry="$LOG_DIR/S2_dry.log" live="$LOG_DIR/S2_live.log"
  expect run_ssosync "$dry" --dry-run
  expect check counts "$dry" --users update=1 --groups update=1
  if run_ssosync "$live"; then
    expect check counts "$live" --users update=1 --groups update=1
  else S_FAIL=1; fi
  converge "$LOG_DIR/S2_converge.log"
}

s3_externalid_change() {
  local orig="$LOG_DIR/S3_user_orig.json" changed="$LOG_DIR/S3_user_changed.json" tamper="ssosync-ugo-test-$TS"
  scim GET "/Users/$TU_ID" | jq 'del(.meta)' > "$orig" || return 1
  jq -e --arg x "$TU_EXT" '.externalId == $x' "$orig" > /dev/null || { echo "SCIM user externalId is not $TU_EXT"; return 1; }
  jq --arg x "$tamper" '.externalId = $x' "$orig" > "$changed" || return 1
  undo user user_put "$TU_ID" "$orig"
  user_put "$TU_ID" "$changed" || return 1
  undo group group_patch "$TG_ID" "$TG_NAME" "$TG_EXT"
  group_patch "$TG_ID" "$TG_NAME" "$tamper" || return 1

  local dry="$LOG_DIR/S3_recreate.log" forced="$LOG_DIR/S3_forced.log" live="$LOG_DIR/S3_live_forced.log"
  expect run_ssosync "$dry" --dry-run
  expect check counts "$dry" --users recreate=1 --groups recreate=1
  expect check record "$dry" --kind user --name "$TU_NAME" --field issuerMatch=true --field externalIdMatch=false --field "externalId=$tamper"
  expect check record "$dry" --kind group --name "$TG_NAME" --field issuerMatch=true --field externalIdMatch=false --field "externalId=$tamper"
  expect run_ssosync "$forced" --dry-run --force-externalid-update
  expect check counts "$forced" --users updateForced=1 --groups updateForced=1
  # The live forced run writes the Google ids back.
  if run_ssosync "$live" --force-externalid-update; then
    expect check counts "$live" --users updateForced=1 --groups updateForced=1
  else S_FAIL=1; fi
  local conv="$LOG_DIR/S3_converge.log"
  converge "$conv"
  expect check record "$conv" --kind user --name "$TU_NAME" --field "externalId=$TU_EXT" --field externalIdMatch=true
  expect check record "$conv" --kind group --name "$TG_NAME" --field "externalId=$TG_EXT" --field externalIdMatch=true
}

s4_managed_orphan() {
  local name="ugo-orphan-$TS" uid gid
  uid=$(scim_create_user "$name@example.invalid" "$name") || return 1
  undo user scim_delete_if_exists Users "$uid"
  gid=$(scim_create_group "$name" "$name") || return 1
  undo group scim_delete_if_exists Groups "$gid"

  local dry="$LOG_DIR/S4_dry.log" retain="$LOG_DIR/S4_dry_retain.log" live="$LOG_DIR/S4_live.log"
  expect run_ssosync "$dry" --dry-run
  expect check counts "$dry" --users delete=1 --groups delete=1
  expect check record "$dry" --kind user --name "$name@example.invalid" --field issuerMatch=true
  # Records ssosync manages are never retained, even with --retain-unmatched.
  expect run_ssosync "$retain" --dry-run --retain-unmatched
  expect check counts "$retain" --users delete=1 --groups delete=1
  if run_ssosync "$live"; then
    expect check counts "$live" --users delete=1 --groups delete=1
  else S_FAIL=1; fi
  converge "$LOG_DIR/S4_converge.log"
}

s5_local_unmatched() {
  local name="ugo-local-$TS" uid gid
  uid=$(ids_create_user "$name@example.invalid" ugo local) || return 1
  undo user ids_delete_user_if_exists "$uid"
  gid=$(ids_create_group "$name") || return 1
  undo group ids_delete_group_if_exists "$gid"

  local dry="$LOG_DIR/S5_dry.log" retain="$LOG_DIR/S5_dry_retain.log" live="$LOG_DIR/S5_live.log"
  expect run_ssosync "$dry" --dry-run
  expect check counts "$dry" --users delete=1 --groups delete=1
  expect run_ssosync "$retain" --dry-run --retain-unmatched
  expect check counts "$retain" --users retain=1 --groups retain=1
  if run_ssosync "$live"; then
    expect check counts "$live" --users delete=1 --groups delete=1
  else S_FAIL=1; fi
  converge "$LOG_DIR/S5_converge.log"
}

# S6/S7 delete the dedicated test records; a live sync re-creates/adopts them.
refresh_test_records() { resolve_test_records "$1" > /dev/null; }

s6_adopt_local() {
  undo repair run_ssosync "$LOG_DIR/S6_undo_sync.log"
  ids_delete_user "$TU_ID" || return 1
  ids_delete_group "$TG_ID" || return 1
  ids_create_user "$TU_NAME" ugo adopt > /dev/null || return 1
  ids_create_group "$TG_NAME" > /dev/null || return 1

  local dry="$LOG_DIR/S6_dry.log" live="$LOG_DIR/S6_live.log" conv="$LOG_DIR/S6_converge.log"
  expect run_ssosync "$dry" --dry-run
  expect check counts "$dry" --users adoptLocal=1 --groups adoptLocal=1
  if run_ssosync "$live"; then
    expect check counts "$live" --users adoptLocal=1 --groups adoptLocal=1
    [ "$S_FAIL" -eq 0 ] && done_undo repair
  else S_FAIL=1; fi
  converge "$conv"
  refresh_test_records "$conv" || S_FAIL=1
}

s7_create() {
  undo repair run_ssosync "$LOG_DIR/S7_undo_sync.log"
  # Delete through SCIM (as ssosync itself writes), not the Identity Store API:
  # SCIM works from every account, whereas Identity Store writes are refused in
  # an account outside the instance owner without delegated administration.
  scim DELETE "/Users/$TU_ID" > /dev/null || return 1
  scim DELETE "/Groups/$TG_ID" > /dev/null || return 1

  local dry="$LOG_DIR/S7_dry.log" live="$LOG_DIR/S7_live.log" conv="$LOG_DIR/S7_converge.log"
  expect run_ssosync "$dry" --dry-run
  expect check counts "$dry" --users create=1 --groups create=1
  if run_ssosync "$live"; then
    expect check counts "$live" --users create=1 --groups create=1
    [ "$S_FAIL" -eq 0 ] && done_undo repair
  else S_FAIL=1; fi
  converge "$conv"
  refresh_test_records "$conv" || S_FAIL=1
}

# --- Identity Store API write access -------------------------------------------
# S5, S6 and S8 must create records with the Identity Store API (only it can
# create LOCAL records; SCIM always creates managed ones). Those writes are
# refused when the test runs in an account outside the instance owner without
# delegated administration (no resource-based policy allows them), while
# ssosync itself never needs them (it writes through SCIM). Probe once.
IDS_WRITE_SKIP=""
probe_identity_store_writes() {
  local gid err="$LOG_DIR/ids_write_probe.err"
  if gid=$(ids_create_group "ugo-writeprobe-$TS" 2> "$err"); then
    echo "ok:   the Identity Store API accepts writes from this account"
    ids_delete_group "$gid" || echo "ERROR: could not delete write-probe group $gid"
  else
    IDS_WRITE_SKIP="the Identity Store API refused writes from this account: $(tr '\n' ' ' < "$err")"
    echo "NOTE: $IDS_WRITE_SKIP"
  fi
}

# --- S8: foreign issuers (needs the Identity Store API to write ExternalIds) --
FOREIGN_SKIP=""
probe_foreign() {
  local gid err="$LOG_DIR/S8_probe.err"
  if [ -n "$IDS_WRITE_SKIP" ]; then FOREIGN_SKIP="$IDS_WRITE_SKIP"; return 0; fi
  gid=$(ids_create_group "ugo-probe-$TS") || { FOREIGN_SKIP="could not create the probe group"; return 0; }
  if ids_set_external_ids group "$gid" "$(ext_json "$FOREIGN_ISSUER" "ugo-probe-$TS")" 2> "$err"; then
    echo "ok:   the Identity Store API can write a foreign-issuer ExternalId"
  else
    FOREIGN_SKIP="the Identity Store API rejected writing ExternalIds: $(tr '\n' ' ' < "$err")"
  fi
  ids_delete_group "$gid" || echo "ERROR: could not delete probe group $gid"
}

restore_ext_user() { ids_set_external_ids user "$TU_ID" "$(ext_json "$EXPECTED" "$TU_EXT")"; }
restore_ext_group() { ids_set_external_ids group "$TG_ID" "$(ext_json "$EXPECTED" "$TG_EXT")"; }

s8c_multi_external_id() {
  local two
  undo user restore_ext_user
  two=$(jq -cn --arg f "$FOREIGN_ISSUER" --arg e "$EXPECTED" --arg d "$TU_EXT" '[{Issuer:$f,Id:"ugo-other"},{Issuer:$e,Id:$d}]')
  ids_set_external_ids user "$TU_ID" "$two" || return 1
  undo group restore_ext_group
  two=$(jq -cn --arg f "$FOREIGN_ISSUER" --arg e "$EXPECTED" --arg d "$TG_EXT" '[{Issuer:$f,Id:"ugo-other"},{Issuer:$e,Id:$d}]')
  ids_set_external_ids group "$TG_ID" "$two" || return 1

  # This tenant's entry is the second one: ssosync must select it and see Equal.
  local dry="$LOG_DIR/S8c_dry.log"
  expect run_ssosync "$dry" --dry-run
  expect check converged "$dry"
  expect check record "$dry" --kind user --name "$TU_NAME" --field externalIdCount=2 --field externalIdSelectedBy=issuerMatch --field issuerMatch=true --field externalIdMatch=true
  expect check record "$dry" --kind group --name "$TG_NAME" --field externalIdCount=2 --field externalIdSelectedBy=issuerMatch --field issuerMatch=true --field externalIdMatch=true
  ids list-users > "$LOG_DIR/S8c_users.json" && ids list-groups > "$LOG_DIR/S8c_groups.json" || return 1
  expect check truth "$dry" --expected "$EXPECTED" --users "$LOG_DIR/S8c_users.json" --groups "$LOG_DIR/S8c_groups.json"
}

s8a_adopt_foreign() {
  undo user restore_ext_user
  ids_set_external_ids user "$TU_ID" "$(ext_json "$FOREIGN_ISSUER" "$TU_EXT")" || return 1
  undo group restore_ext_group
  ids_set_external_ids group "$TG_ID" "$(ext_json "$FOREIGN_ISSUER" "$TG_EXT")" || return 1

  local dry="$LOG_DIR/S8a_dry.log" live="$LOG_DIR/S8a_live.log"
  expect run_ssosync "$dry" --dry-run
  expect check counts "$dry" --users adoptForeign=1 --groups adoptForeign=1
  expect check record "$dry" --kind user --name "$TU_NAME" --field issuerMatch=false --field "issuer=$FOREIGN_ISSUER"
  expect run_ssosync "$live"
  expect check counts "$live" --users adoptForeign=1 --groups adoptForeign=1
  # After adopting, ssosync owns the records: nothing further to change.
  converge "$LOG_DIR/S8a_converge.log"
}

s8b_foreign_unmatched() {
  local name="ugo-foreign-$TS" uid gid
  uid=$(ids_create_user "$name@example.invalid" ugo foreign) || return 1
  undo user ids_delete_user_if_exists "$uid"
  ids_set_external_ids user "$uid" "$(ext_json "$FOREIGN_ISSUER" "$name")" || return 1
  gid=$(ids_create_group "$name") || return 1
  undo group ids_delete_group_if_exists "$gid"
  ids_set_external_ids group "$gid" "$(ext_json "$FOREIGN_ISSUER" "$name")" || return 1

  local dry="$LOG_DIR/S8b_dry.log" retain="$LOG_DIR/S8b_dry_retain.log" live="$LOG_DIR/S8b_live.log"
  expect run_ssosync "$dry" --dry-run
  expect check counts "$dry" --users delete=1 --groups delete=1
  expect run_ssosync "$retain" --dry-run --retain-unmatched
  expect check counts "$retain" --users retain=1 --groups retain=1
  if run_ssosync "$live"; then
    expect check counts "$live" --users delete=1 --groups delete=1
  else S_FAIL=1; fi
  converge "$LOG_DIR/S8b_converge.log"
}

# =============================================================================
scenario S1 "Baseline: IssuerArn and Equal" s1_baseline
if grep -q '^S1|FAIL' "$RESULTS"; then
  echo "Baseline failed: the remaining scenarios depend on it and are not run."
  exit 1
fi

if ! resolve_test_records "$BASE_LOG"; then
  echo "FAIL: could not resolve the test user/group (UGO_TEST_USER='${UGO_TEST_USER:-}', UGO_TEST_GROUP='${UGO_TEST_GROUP:-}'); each must be in scope and provisioned by ssosync"
  exit 1
fi

scenario S2 "Update (managed, attribute changed)" s2_update
scenario S3 "Recreate / UpdateForced (managed, ExternalId changed)" s3_externalid_change
scenario S4 "Delete, never retained (managed, not in Google)" s4_managed_orphan

# Local-record scenarios need Identity Store API writes. Refused writes are
# expected only in the NonDelegated account (outside the instance owner, no
# delegated administration); there the scenarios are skipped, and the
# Management and Delegated accounts prove them. Anywhere else a refusal is a
# permissions regression, so the scenarios are skipped AND the test fails.
probe_identity_store_writes
LOCAL_SKIP=""
if [ -n "$IDS_WRITE_SKIP" ]; then
  LOCAL_SKIP="$IDS_WRITE_SKIP"
  if [ "${TEST_ACCOUNT_TYPE:-}" = NonDelegated ]; then
    LOCAL_SKIP="$LOCAL_SKIP (expected in the NonDelegated account; proven in Management/Delegated)"
  else
    echo "FAIL: Identity Store API writes are expected to work in the ${TEST_ACCOUNT_TYPE:-developer} account"
    FAILED=1
  fi
fi
scenario S5 "Delete / Retain (local, not in Google)" s5_local_unmatched "$LOCAL_SKIP"

DEDICATED=""
if [ -z "${UGO_TEST_USER:-}" ] || [ -z "${UGO_TEST_GROUP:-}" ]; then
  DEDICATED="UGO_TEST_USER and UGO_TEST_GROUP must name a dedicated Google test user/group (these scenarios give them new Identity Store ids)"
fi
if [ -n "$DEDICATED" ]; then
  scenario S6 "AdoptLocal (local, in Google)" : "$DEDICATED"; FAILED=1
  scenario S7 "Create (missing, in Google)" : "$DEDICATED"; FAILED=1
else
  scenario S6 "AdoptLocal (local, in Google)" s6_adopt_local "$LOCAL_SKIP"
  scenario S7 "Create (missing, in Google)" s7_create
fi

probe_foreign
if [ -n "$FOREIGN_SKIP" ] && [ "${UGO_FOREIGN_REQUIRED:-false}" = true ]; then FAILED=1; fi
scenario S8c "Equal with multiple ExternalIds (this tenant's entry not first)" s8c_multi_external_id "$FOREIGN_SKIP"
scenario S8a "AdoptForeign (foreign issuer, in Google)" s8a_adopt_foreign "$FOREIGN_SKIP"
scenario S8b "Delete / Retain (foreign issuer, not in Google)" s8b_foreign_unmatched "$FOREIGN_SKIP"
