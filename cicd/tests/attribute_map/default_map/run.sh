#!/usr/bin/env bash
# Built-in default attribute map, CLI.
#  1. Reset the Identity Store to the baseline (local users/groups, no ExternalIds).
#  2. Live sync with the default map: every baseline user is adopted, none is
#     created, deleted or recreated, and organization-derived fields resolve
#     from the primary organization.
#  3. Convergence: a dry-run straight afterwards proposes no change at all.
#
# Google test data relied on (see google_test_users.json and baseline/):
#   crpates  - primary organization, title "Science Officer"
#   AceHole  - primary (school) organization with no title; blank given name
#   olaf     - suspended; no organizations; apostrophe in his primary email
#              and in his OU (#401), which this suite precaches
source "$(dirname "$0")/../lib/common.sh"
setup_cli

# #401: precache an OU path with an apostrophe and report cache metrics, so both
# OrgUnitPath queries (precache and the per-OU usercache count) carry a quote.
SSOSYNC_EXTRA_ARGS=(--precache-ous "/Red Dwarf/Petersen's Locker" --cache-metrics)

reset_baseline

ADOPT="$LOG_DIR/adopt.log"
run_ssosync live "" "$ADOPT"

# The OU queries were sent and none of them failed (these errors don't fail a run).
check assert_log count "$ADOPT" --msg "Precaching users" --min 1
check assert_log absent "$ADOPT" --msg "Precaching failed, caching on the fly"
check assert_log absent "$ADOPT" --msg "Precaching return no users? Switching to caching on the fly"
check assert_log count "$ADOPT" --msg "Caching Performance" --min 1
check assert_log absent "$ADOPT" --msg "Failed retrieving usercache count"
check assert_log absent "$ADOPT" --msg "Fetching group member from Google failed, aborting sync"

check assert_log summary "$ADOPT" \
  --users adoptLocal=4 create=0 delete=0 recreate=0 updateForced=0 \
  --groups create=0 recreate=0

# Debug mapping trace is emitted for every in-scope user.
check assert_log count "$ADOPT" --msg "attribute map: mapping Google user" --min 4

# organizations[primary] selects the primary organization...
check assert_log count "$ADOPT" --msg "attribute map: entry resolved" \
  --field "source=organizations[primary].title" --field "resolved=Science Officer" --min 1
# ...and resolves to nothing for a user with no primary organization (olaf).
check assert_log count "$ADOPT" --msg "attribute map: entry resolved" \
  --field "source=organizations[primary].title" --field "resolved=" \
  --field "reason=noPrimaryEntry" --min 1

# Pre-SCIM validation warns about Rimmer's blank given name, but does not stop the sync.
check assert_log count "$ADOPT" --msg "attribute map: mapped user may be rejected by the SCIM API" \
  --field rule=requiredField --field field=name.givenName --min 1

# Convergence: nothing left to change.
check assert_converged "" "$LOG_DIR/converge.log"

finish
