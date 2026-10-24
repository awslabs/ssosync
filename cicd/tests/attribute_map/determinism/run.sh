#!/usr/bin/env bash
# Determinism: two dry-runs of the same map against an unchanged directory must
# propose exactly the same changes and resolve every map entry the same way.
# (Dry-runs write nothing, so the second run sees the same state as the first.)
# Uses the rich map, which exercises [primary], [type=...] and custom-schema
# selection over multi-valued fields whose order Google does not guarantee.
source "$(dirname "$0")/../lib/common.sh"
setup_cli

reset_and_adopt

RUN1="$LOG_DIR/run1.log"
RUN2="$LOG_DIR/run2.log"
run_ssosync dry "$MAPS/rich.json" "$RUN1"
run_ssosync dry "$MAPS/rich.json" "$RUN2"

# The rich map adds fields the default map did not, so there must be work to compare.
check assert_log count "$RUN1" --msg "DRY RUN: Would update user" --min 1
check assert_log same "$RUN1" "$RUN2"

finish
