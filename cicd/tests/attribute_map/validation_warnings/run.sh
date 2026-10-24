#!/usr/bin/env bash
# Pre-SCIM validation warnings, CLI, dry-run only (no changes are made).
# Maps Username from SSOSync-Test.LongValue: Rimmer's value is over 128
# characters (maxLength), and the other users have no LongValue, so their
# Username resolves empty (requiredField) after a missing-field warning.
# Warnings must not stop the run.
source "$(dirname "$0")/../lib/common.sh"
setup_cli

reset_and_adopt

LOG="$LOG_DIR/validation.log"
run_ssosync dry "$MAPS/long_username.json" "$LOG"

REJECT="attribute map: mapped user may be rejected by the SCIM API"
check assert_log count "$LOG" --msg "$REJECT" --field rule=maxLength --field field=userName --min 1
check assert_log count "$LOG" --msg "$REJECT" --field rule=requiredField --field field=userName --min 3
check assert_log count "$LOG" --msg "attribute map: custom-schema source not found on user; field cleared" \
  --field reason=customFieldAbsent --field field=LongValue --min 3
# The run still completed.
check assert_log count "$LOG" --msg "Change Summary: Users" --min 1

finish
