#!/usr/bin/env bash
# Username remap, CLI, live (the issue #362 use case).
# After the baseline is adopted, users are managed and matched by ExternalId, so
# mapping Username from SSOSync-Test.AWSUsername must UPDATE each user in place:
# no creates, deletes or recreates, and group memberships kept. Then converges.
#
# Google test data relied on: every user has SSOSync-Test.AWSUsername set
# (crpates -> "chris.pates.aws").
source "$(dirname "$0")/../lib/common.sh"
setup_cli

reset_and_adopt

LOG="$LOG_DIR/remap.log"
run_ssosync live "$MAPS/remap_username.json" "$LOG"

check assert_log summary "$LOG" --users update=4 create=0 delete=0 recreate=0 updateForced=0
check assert_log count "$LOG" --msg "attribute map: entry resolved" \
  --field "source=customSchemas.SSOSync-Test.AWSUsername" --field "resolved=chris.pates.aws" --min 1

# Users, groups and memberships all settle under the remapped identity.
check assert_converged "$MAPS/remap_username.json" "$LOG_DIR/converge.log"

finish
