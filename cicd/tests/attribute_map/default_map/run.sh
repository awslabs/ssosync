#!/usr/bin/env bash
# Built-in default attribute map, CLI.
#  1. Reset the Identity Store to the baseline (local users/groups, no ExternalIds).
#  2. Live sync with the default map: every baseline user is adopted, none is
#     created, deleted or recreated, and organization-derived fields resolve
#     from the work-typed organization.
#  3. Convergence: a dry-run straight afterwards proposes no change at all.
#
# Google test data relied on (see baseline/ and the attribute-map test plan):
#   crpates  - organization typed "work", title "Science Officer"
#   AceHole  - only a school-typed organization; blank given name
#   olaf     - suspended
source "$(dirname "$0")/../lib/common.sh"
setup_cli

reset_baseline

ADOPT="$LOG_DIR/adopt.log"
run_ssosync live "" "$ADOPT"

check assert_log summary "$ADOPT" \
  --users adoptLocal=4 create=0 delete=0 recreate=0 updateForced=0 \
  --groups create=0 recreate=0

# Debug mapping trace is emitted for every in-scope user.
check assert_log count "$ADOPT" --msg "attribute map: mapping Google user" --min 4

# organizations[type=work] selects the work organization...
check assert_log count "$ADOPT" --msg "attribute map: entry resolved" \
  --field "source=organizations[type=work].title" --field "resolved=Science Officer" --min 1
# ...and resolves to nothing for a user who has no work organization (Rimmer).
check assert_log count "$ADOPT" --msg "attribute map: entry resolved" \
  --field "source=organizations[type=work].title" --field "resolved=" \
  --field "reason=noTypeMatch:work" --min 1

# Pre-SCIM validation warns about Rimmer's blank given name, but does not stop the sync.
check assert_log count "$ADOPT" --msg "attribute map: mapped user may be rejected by the SCIM API" \
  --field rule=requiredField --field field=name.givenName --min 1

# Convergence: nothing left to change.
check assert_converged "" "$LOG_DIR/converge.log"

finish
