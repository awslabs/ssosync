#!/usr/bin/env bash
# Username remap, CLI, live (the issue #362 use case).
# After the baseline is adopted, users are managed and matched by ExternalId, so
# mapping Username from SSOSync-Test.AWSUsername must UPDATE each user in place:
# no creates, deletes or recreates, and group memberships kept. Then converges.
#
# Google test data relied on (see google_test_users.json): every user has
# SSOSync-Test.AWSUsername set to an EMAIL-shaped value that differs from the
# primary email, so all four users are renamed:
#   crpates  -> chris.pates.aws@nothing2see.info
#   AceHole  -> arnold.rimmer.aws@nothing2see.info
#   dave2    -> dave.lister.aws@nothing2see.info
#   olaf     -> olaf.petersen.aws@nothing2see.info
source "$(dirname "$0")/../lib/common.sh"
setup_cli

reset_and_adopt

LOG="$LOG_DIR/remap.log"
run_ssosync live "$MAPS/remap_username.json" "$LOG"

check assert_log summary "$LOG" --users update=4 create=0 delete=0 recreate=0 updateForced=0
for username in chris.pates.aws@nothing2see.info arnold.rimmer.aws@nothing2see.info \
                dave.lister.aws@nothing2see.info olaf.petersen.aws@nothing2see.info; do
  check assert_log count "$LOG" --msg "attribute map: entry resolved" \
    --field "source=customSchemas.SSOSync-Test.AWSUsername" --field "resolved=$username" --min 1
done

# Users, groups and memberships all settle under the remapped identity.
check assert_converged "$MAPS/remap_username.json" "$LOG_DIR/converge.log"

finish
