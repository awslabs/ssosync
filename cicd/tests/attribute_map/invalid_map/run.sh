#!/usr/bin/env bash
# A malformed attribute map must stop ssosync at startup, before any sync,
# with an error that names every violation. Makes no changes.
source "$(dirname "$0")/../lib/common.sh"
setup_cli

LOG="$LOG_DIR/invalid.log"
rc=0
run_ssosync dry "$MAPS/invalid.json" "$LOG" || rc=$?

if [ "$rc" -eq 0 ]; then
  echo "FAIL: ssosync exited 0 with an invalid attribute map"
  FAILED=1
else
  echo "ok:   ssosync exited $rc"
fi

# The map is loaded before the JSON log formatter is set, so this fatal line is
# text-format; assert_log.py matches it as a raw line.
check assert_log count "$LOG" --msg "cannot load attribute map file" --min 1
check assert_log count "$LOG" --msg 'unsupported target field "Name.Givenname"' --min 1
check assert_log count "$LOG" --msg 'unsupported source attribute "organizations[type=work].nosuchsubfield"' --min 1
check assert_log absent "$LOG" --msg "Change Summary"

finish
