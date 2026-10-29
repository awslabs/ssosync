#!/usr/bin/env bash
# Opt-in manager mapping, CLI, live.
# Enterprise.Manager <- relations[type=manager].value carries the manager's email
# address. Checks IAM Identity Center accepts it, and that the value read back
# compares equal (convergence).
#
# Google test data relied on: crpates has a manager relation "dave2@nothing2see.info".
source "$(dirname "$0")/../lib/common.sh"
setup_cli

reset_and_adopt

LOG="$LOG_DIR/manager.log"
run_ssosync live "$MAPS/manager.json" "$LOG"

check assert_log summary "$LOG" --users create=0 delete=0 recreate=0 updateForced=0
check assert_log count "$LOG" --msg "attribute map: entry resolved" \
  --field "source=relations[type=manager].value" --field "resolved=dave2@nothing2see.info" --min 1

check assert_converged "$MAPS/manager.json" "$LOG_DIR/converge.log"

finish
