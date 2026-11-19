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
#                          (dry and live: the retained group and its member are kept, #442)
#   S6  AdoptLocal         dedicated test user/group re-created as local records
#   S7  Create             dedicated test user/group deleted from the Identity Store
#   S9  Pacing (#429)      UGO_RATE_GROUPS Google groups matched by UGO_RATE_GROUP_MATCH
#                          (RateTest_00..RateTest_29; members crpates, acehole and
#                          dave2, never Olaf) are synced, their memberships read at
#                          no more than 18 Identity Store calls a second, never
#                          throttled, then deleted again. Runs last, so the groups
#                          never exist while S6/S7 re-create UGO_TEST_USER.
#
# There is no S8: the foreign-issuer outcomes (AdoptForeign, Delete/Retain of a
# foreign record, multiple ExternalIds) need records whose ExternalIds carry
# another tenant's issuer, which the test cannot set up (#440). Unit tests in
# internal/sync_issuer_test.go and internal/sync_test.go cover them.
#
# S5/S6 create LOCAL records, which only the Identity Store API can do.
# Its writes are refused from an account outside the instance owner without
# delegated administration (NonDelegated): there S5/S6 are SKIPPED and proven by
# the Management/Delegated runs; a refusal in any other account FAILS the test.
# S5 also adds a membership, and runs only in the Delegated account and the
# developer pipeline; Management and NonDelegated skip it (#442, #408).
# Everything else, including S7, changes records through SCIM, which works from
# every account (as ssosync itself does).
#
# Every change is undone (see lib.sh); S6/S7 give the dedicated test records new
# Identity Store ids, so they must be a Google test user/group with no
# permission-set assignments:
#   UGO_TEST_USER   primary email of a Google test user in an in-scope group
#   UGO_TEST_GROUP  name of an in-scope Google test group
# S2/S3 use them too when set, else the first managed user/group by name.
#
# Environment: SCIMEndpointUrl, SCIMAccessToken, GoogleCustomerId,
# GoogleImpersonateServiceAccount (Secrets Manager; ssosync runs keyless, see
# tests/wif_account_execution/lib/keyless.sh), GROUP_MATCH (default "name:AWS*"), TEST_ACCOUNT_TYPE (set by the
# release testing stacks: Management | Delegated | NonDelegated).

set -euo pipefail
source "$(dirname "$0")/lib.sh"

for v in SCIMEndpointUrl SCIMAccessToken; do
  if [ -z "${!v:-}" ]; then echo "ERROR: $v is not set"; exit 1; fi
done
keyless_setup "$ROOT"
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
  # Deleting the group (undo above) also removes this membership.
  ids_add_member "$gid" "$uid" || return 1

  local dry="$LOG_DIR/S5_dry.log" retain="$LOG_DIR/S5_dry_retain.log" live="$LOG_DIR/S5_live.log"
  local live_retain="$LOG_DIR/S5_live_retain.log"
  expect run_ssosync "$dry" --dry-run
  expect check counts "$dry" --users delete=1 --groups delete=1
  expect run_ssosync "$retain" --dry-run --retain-unmatched
  expect check counts "$retain" --users retain=1 --groups retain=1
  # A live retain run leaves the local group and its member as they are; it
  # used to PATCH the group with an empty externalId, and the 400 ended the
  # run (#442). The retained local user is PUT with Active=false.
  if run_ssosync "$live_retain" --retain-unmatched; then
    expect check counts "$live_retain" --users retain=1 --groups retain=1
    expect ids_group_exists "$gid"
    expect ids_is_member "$gid" "$uid"
  else S_FAIL=1; fi
  # The live run without -r then deletes both.
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
# S5 and S6 must create records with the Identity Store API (only it can
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

# S9 (#429): every Identity Store attempt is paced to 18 a second. Each run
# here matches the RateTest groups as well as the default groups; an
# assignment before a function call lasts only for that call.
s9_rate_limit() {
  local match="$GROUP_MATCH,$UGO_RATE_GROUP_MATCH" want="$UGO_RATE_GROUPS"
  # Display names are the group emails, ratetest_NN@..., so match by prefix.
  local prefix="${UGO_RATE_GROUP_MATCH#name:}"; prefix="${prefix%\*}"
  local create="$LOG_DIR/S9_create.log" paced="$LOG_DIR/S9_paced.log" cleanup="$LOG_DIR/S9_cleanup.log"
  # A live run under the default match alone deletes the RateTest groups.
  undo groups run_ssosync "$LOG_DIR/S9_undo.log"

  # A: create the groups and their memberships. A count other than $want means
  # the tenant is not ready, or Google's search index has not caught up.
  GROUP_MATCH="$match" run_ssosync "$create" || return 1
  expect check summary "$create" --groups "create=$want" recreate=0 delete=0 --users create=0 delete=0 recreate=0
  expect check absent "$create" --msg "Identity Store request throttled"

  # B: the measured run. Memberships are read with one ListGroupMemberships
  # call per Identity Store group (each RateTest group is one page).
  GROUP_MATCH="$match" run_ssosync "$paced" --dry-run || return 1
  expect check converged "$paced"
  expect check count "$paced" --msg "Change Summary: Group Members" --prefix "group=$prefix" --eq "$want"
  expect check count "$paced" --msg "Change Summary: Group Members" --prefix "group=$prefix" \
    --field unchanged=3 --field add=0 --field remove=0 --eq "$want"
  expect check absent "$paced" --msg "Identity Store request throttled"
  local groups elapsed bound
  groups=$(check field "$paced" --msg "GetGroupMembershipsLists() Return" --key groups) || return 1
  elapsed=$(check field "$paced" --msg "GetGroupMembershipsLists() Return" --key elapsedMs) || return 1
  # N paced calls take at least (N - 1) / 18 s: 1,611 ms for 30 calls.
  bound=$(( (groups - 1) * 1000 / 18 ))
  note "memberships: $groups calls in ${elapsed} ms (at least ${bound} ms at 18/s);" \
    "$(awk -v g="$groups" -v e="$elapsed" 'BEGIN { printf "%.1f", (g - 1) * 1000 / (e > 0 ? e : 1) }') calls/s"
  if [ "$groups" -lt "$want" ]; then
    echo "FAIL: memberships read for $groups groups, expected at least $want"; S_FAIL=1
  fi
  if [ "$elapsed" -lt "$bound" ]; then
    echo "FAIL: $groups membership calls took ${elapsed} ms, under the ${bound} ms 18/s pacing allows"; S_FAIL=1
  fi

  # C: clean up with a live run under the default match alone.
  if run_ssosync "$cleanup"; then
    expect check summary "$cleanup" --groups "delete=$want" create=0 --users create=0 delete=0
    expect check absent "$cleanup" --msg "Identity Store request throttled"
  else S_FAIL=1; fi
  converge "$LOG_DIR/S9_converge.log"
  passed_since 0 && done_undo groups
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
# S5 needs Identity Store API memberships as well as records (#442), so it runs
# only where tests that manipulate the Identity Store API belong: the Delegated
# release account and the developer pipeline (TEST_ACCOUNT_TYPE not set).
# Management keeps execution tests, NonDelegated the recommended deployment (#408).
S5_SKIP="$LOCAL_SKIP"
case "${TEST_ACCOUNT_TYPE:-}" in
  ""|Delegated) ;;
  *) S5_SKIP="S5 runs only in the Delegated account and the developer pipeline (Identity Store API tests, #408); this is the $TEST_ACCOUNT_TYPE account" ;;
esac
scenario S5 "Delete / Retain (local, not in Google)" s5_local_unmatched "$S5_SKIP"

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

RATE_SKIP=""
if [ -z "${UGO_RATE_GROUP_MATCH:-}" ] || [ -z "${UGO_RATE_GROUPS:-}" ]; then
  RATE_SKIP="UGO_RATE_GROUP_MATCH and UGO_RATE_GROUPS are not set"
fi
scenario S9 "Identity Store pacing (#429)" s9_rate_limit "$RATE_SKIP"
