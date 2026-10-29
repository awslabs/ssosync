#!/usr/bin/env bash
# Pre-SCIM validation warnings, CLI, dry-run only (no changes are made; the
# Change Summary reports the changes a live run WOULD make).
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
# The run still completed, and its Change Summary reports what WOULD change
# live: all four adopted users are matched by ExternalId and every Username
# changes (Rimmer's to the long value, the others to empty), so 4 updates and
# nothing else. Groups are unchanged, and memberships stay matched by
# ExternalId even though every Username changes.
check assert_log summary "$LOG" \
  --users update=4 equal=0 create=0 delete=0 recreate=0 updateForced=0 \
          adoptLocal=0 adoptForeign=0 retain=0 \
  --groups create=0 update=0 delete=0 recreate=0 updateForced=0 \
           adoptLocal=0 adoptForeign=0 retain=0 \
  --members add=0 remove=0
check assert_dry_run "$LOG"

finish
