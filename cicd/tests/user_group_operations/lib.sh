#!/usr/bin/env bash
# Helpers for the user/group operations live test (sourced by run.sh).
# Kept bash 3.2 compatible (no associative arrays, mapfile or ${v,,}).

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # tests/user_group_operations
ROOT="$(cd "$HERE/../.." && pwd)"                      # artifact root
SSOSYNC="$ROOT/ssosync"
LOG_DIR="${LOG_DIR:-$ROOT/logs}"
GROUP_MATCH="${GROUP_MATCH:-name:AWS*}"
TS="$(date +%s)"
mkdir -p "$LOG_DIR"

check() { python3 "$HERE/check_log.py" "$@"; }
lower() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }
note() { echo "      $*"; }

# --- ssosync -----------------------------------------------------------------

# run_ssosync <log-file> [extra flags...]; returns ssosync's exit code.
run_ssosync() {
  local log=$1; shift
  echo ">>> ssosync $* (log: $(basename "$log"))"
  local rc=0
  "$SSOSYNC" -t "$SCIMAccessToken" -e "$SCIMEndpointUrl" -u "$GoogleAdminEmail" \
    -c "$ROOT/credentials.json" -s groups -g "$GROUP_MATCH" \
    --log-format json --log-level debug "$@" > "$log" 2>&1 || rc=$?
  cat "$log"
  echo "<<< ssosync exit=$rc"
  return $rc
}

# --- SCIM (this tenant: every ExternalId it writes carries our Issuer) --------

scim() { # scim <method> <path> [body-file]; prints the response body
  local args=(-sS -f -X "$1" -H "Authorization: Bearer $SCIMAccessToken" -H "Content-Type: application/scim+json")
  [ $# -ge 3 ] && args+=(--data-binary "@$3")
  curl "${args[@]}" "${SCIMEndpointUrl%/}$2"
}

scim_create_user() { # <userName> <externalId>; prints the new id
  jq -n --arg u "$1" --arg x "$2" '{schemas:["urn:ietf:params:scim:schemas:core:2.0:User"],
    userName:$u, externalId:$x, name:{givenName:"ugo",familyName:"orphan"}, displayName:"ugo orphan",
    active:true, emails:[{value:$u,type:"work",primary:true}], addresses:[{type:"work"}]}' > "$LOG_DIR/scim_body.json"
  scim POST /Users "$LOG_DIR/scim_body.json" | jq -er .id
}

scim_create_group() { # <displayName> <externalId>; prints the new id
  jq -n --arg n "$1" --arg x "$2" '{schemas:["urn:ietf:params:scim:schemas:core:2.0:Group"],
    displayName:$n, externalId:$x, members:[]}' > "$LOG_DIR/scim_body.json"
  scim POST /Groups "$LOG_DIR/scim_body.json" | jq -er .id
}

group_patch() { # <groupId> <displayName> <externalId> (the operations ssosync's UpdateGroup sends)
  jq -n --arg n "$2" --arg x "$3" '{schemas:["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
    Operations:[{op:"replace",path:"displayName",value:$n},{op:"replace",path:"externalId",value:$x}]}' \
    > "$LOG_DIR/group_patch.json"
  scim PATCH "/Groups/$1" "$LOG_DIR/group_patch.json" > /dev/null
}

user_put() { scim PUT "/Users/$1" "$2" > /dev/null; } # <userId> <doc-file>

# Idempotent deletes for undos: a record the scenario (or ssosync) already
# removed is fine; any other error still fails.
scim_delete_if_exists() { # <Users|Groups> <id>
  if scim GET "/$1/$2" > /dev/null 2>&1; then scim DELETE "/$1/$2" > /dev/null
  else echo "      (SCIM $1/$2 already gone)"; fi
}

# --- Identity Store API (local records, and ExternalIds SCIM cannot write) ----

ids() { aws identitystore "$@" --identity-store-id "$STORE_ID" --region "$REGION" --output json; }

ids_create_user() { # <userName> <given> <family>; prints the new UserId
  ids create-user --user-name "$1" --display-name "$2 $3" \
    --name "GivenName=$2,FamilyName=$3" --emails "Value=$1,Type=work,Primary=true" | jq -er .UserId
}
ids_create_group() { ids create-group --display-name "$1" | jq -er .GroupId; } # <displayName>
ids_delete_user() { ids delete-user --user-id "$1" > /dev/null; }
ids_delete_group() { ids delete-group --group-id "$1" > /dev/null; }

ids_user_exists() { ids list-users | jq -e --arg i "$1" 'any(.Users[]; .UserId == $i)' > /dev/null; }
ids_group_exists() { ids list-groups | jq -e --arg i "$1" 'any(.Groups[]; .GroupId == $i)' > /dev/null; }
ids_delete_user_if_exists() {
  if ids_user_exists "$1"; then ids_delete_user "$1"; else echo "      (user $1 already gone)"; fi
}
ids_delete_group_if_exists() {
  if ids_group_exists "$1"; then ids_delete_group "$1"; else echo "      (group $1 already gone)"; fi
}

# passed_since <S_FAIL snapshot>: true when no assertion failed since then.
passed_since() { [ "$S_FAIL" -eq 0 ] && [ "$1" -eq 0 ]; }

# ids_set_external_ids <user|group> <id> <json array of {Issuer,Id}>
ids_set_external_ids() {
  local ops
  ops=$(jq -cn --argjson v "$3" '[{AttributePath:"externalIds",AttributeValue:$v}]')
  if [ "$1" = user ]; then ids update-user --user-id "$2" --operations "$ops" > /dev/null
  else ids update-group --group-id "$2" --operations "$ops" > /dev/null; fi
}
ext_json() { jq -cn --arg i "$1" --arg d "$2" '[{Issuer:$i,Id:$d}]'; } # <issuer> <id>

# --- Cleanup registry --------------------------------------------------------
# Every change registers the command that undoes it. A scenario's pending undos
# run when it ends (so the next scenario starts clean); all remaining undos run
# on exit. Commands are stored shell-quoted and run with eval.

CLEANUP_DIR="$(mktemp -d)"
CLEANUP_SEQ=0
SCEN="setup"

undo() { # undo <key> <command...>; undo commands must be idempotent
  CLEANUP_SEQ=$((CLEANUP_SEQ + 1))
  printf '%q ' "${@:2}" > "$CLEANUP_DIR/$(printf '%04d' $CLEANUP_SEQ)__${SCEN}__$1"
}
# done_undo <key>: drop an undo only once the change was PROVEN reverted
# (assertions passed); otherwise it runs when the scenario ends.
done_undo() { rm -f "$CLEANUP_DIR"/*__"${SCEN}"__"$1"; }

run_undos() { # run_undos [scenario]; newest first
  local f rc=0
  for f in $(ls -r "$CLEANUP_DIR" 2>/dev/null); do
    if [ -n "${1:-}" ]; then
      case "$f" in *__"$1"__*) ;; *) continue ;; esac
    fi
    echo ">>> undo: $(cat "$CLEANUP_DIR/$f")"
    eval "$(cat "$CLEANUP_DIR/$f")" || { echo "ERROR: undo failed: $f"; rc=1; }
    rm -f "$CLEANUP_DIR/$f"
  done
  return $rc
}

# --- Scenario bookkeeping ----------------------------------------------------

RESULTS="$LOG_DIR/scenario_results.txt"
: > "$RESULTS"
FAILED=0
S_FAIL=0

expect() { "$@" || S_FAIL=1; }

# A scenario function returns non-zero when its setup could not be completed.
# Because it runs in an "||" context, errexit is off inside it: every setup
# command must be followed by "|| return 1" (or wrapped in "expect").
scenario() { # scenario <id> <title> <function> [skip-reason]
  SCEN=$1; S_FAIL=0
  echo
  echo "================ SCENARIO $1: $2"
  if [ -n "${4:-}" ]; then
    echo "$1|SKIP|$2|$4" >> "$RESULTS"; echo "SKIP: $4"
    return 0
  fi
  "$3" || { echo "FAIL: setup of scenario $1 did not complete"; S_FAIL=1; }
  if [ "$S_FAIL" -ne 0 ]; then
    run_undos "$1" || true
    echo ">>> repair: live sync after the failed scenario"
    run_ssosync "$LOG_DIR/${1}_repair.log" > /dev/null || true
    echo "$1|FAIL|$2|" >> "$RESULTS"; FAILED=1
  else
    run_undos "$1" || { echo "$1|FAIL|$2|undo failed" >> "$RESULTS"; FAILED=1; return 0; }
    echo "$1|PASS|$2|" >> "$RESULTS"
  fi
}

# converge <log>: a dry run straight after must propose nothing, and every
# managed record must match the expected issuer exactly.
converge() {
  expect run_ssosync "$1" --dry-run
  expect check converged "$1"
  expect check issuer "$1" --expected "$EXPECTED" --min-users 1 --min-groups 1
}

summary_table() {
  echo
  echo "================ RESULTS (account type: ${TEST_ACCOUNT_TYPE:-not set})"
  local id res title reason
  while IFS='|' read -r id res title reason; do
    printf '  %-4s %-5s %s%s\n' "$id" "$res" "$title" "${reason:+ — $reason}"
  done < "$RESULTS"
}
