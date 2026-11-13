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
#              and in his OU (#401), which this suite precaches; the other
#              users are outside his OU tree, so they are not precached (#404)
source "$(dirname "$0")/../lib/common.sh"
setup_cli

# #401: precache an OU path with an apostrophe and report cache metrics, so both
# OrgUnitPath queries (precache and the per-OU usercache count) carry a quote.
# #404: a second OU that does not exist must be skipped with a warning, and
# only the first OU's users may be precached. Keep Olaf's OU as the first
# --precache-ous: lib/test_google_test_data.py reads both values.
PRECACHE_OU="/Red Dwarf/Petersen's Locker"
MISSING_OU="/SSOSync 404 No Such OU"
SSOSYNC_EXTRA_ARGS=(--precache-ous "/Red Dwarf/Petersen's Locker" --precache-ous "/SSOSync 404 No Such OU" --cache-metrics)

reset_baseline

ADOPT="$LOG_DIR/adopt.log"
run_ssosync live "" "$ADOPT"

# One query per OU (#404): the missing OU is warned about (as an error or as
# zero users, whichever Google returns for it) and skipped; Olaf's OU is not,
# and precaching as a whole neither failed nor came back empty (none of these
# fail a run).
check assert_log count "$ADOPT" --msg "Precaching users" --min 1
check assert_log count "$ADOPT" --msg "PrecacheOrgUnits: " --field "orgUnitPath=$MISSING_OU" --min 1
check assert_log absent "$ADOPT" --msg "PrecacheOrgUnits: " --field "orgUnitPath=$PRECACHE_OU"
check assert_log absent "$ADOPT" --msg "Precaching failed, caching on the fly"
check assert_log absent "$ADOPT" --msg "Precaching return no users? Switching to caching on the fly"
check assert_log count "$ADOPT" --msg "Caching Performance" --min 1

# Every test user is seen (as a member of an AWS* group, which
# lib/test_google_test_data.py checks against baseline/groups.json), so the
# keys are exactly their OUs. Each OU reports itself plus its sub-OUs (#413):
# Precaches are the users in Olaf's OU tree, Misses the others, from
# google_test_users.json. TotalUsers is exactly the fixture's users in the
# tree (#416: with --suspended the tenant holds just the 4 fixture users):
# Olaf's OU reuses its precache count, and the others are counted with an
# ids-only query, so / 4, /Maintenance 2, Vending Machines 1, Olaf 1. Hits
# depend on Google's membership expansion, so they are only a lower bound:
# each precached user is found in the cache at least once.
STATS="$LOG_DIR/cache_stats.json"
check assert_log field "$ADOPT" --key "Cache Stats" \
  --msg "Caching Performance, orgPaths with a high Miss rate compared with the TotalUsers consider precaching" > "$STATS"
echo "Cache Stats: $(cat "$STATS")"
check jq -e --arg ou "$PRECACHE_OU" --slurpfile f "$AM_DIR/google_test_users.json" '
  def in_tree($t): . == $t or startswith(($t | rtrimstr("/")) + "/");
  [$f[0].users[].orgUnitPath] as $ous
  | (keys == ($ous | unique))
    and .[$ou].Hits >= 1 and .[$ou].TotalUsers == 1
    and all(to_entries[]; .key as $k | .value as $v
      | [$ous[] | select(in_tree($k))] as $tree
      | $v.Precaches == ([$tree[] | select(in_tree($ou))] | length)
        and $v.Misses == ([$tree[] | select(in_tree($ou) | not)] | length)
        and $v.Hits >= $v.Precaches
        and $v.TotalUsers == ($tree | length))' "$STATS"
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
