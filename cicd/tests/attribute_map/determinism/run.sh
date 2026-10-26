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

# In dry-run mode the Change Summary reports what WOULD change live. After the
# default-map adoption every user is managed and matched by ExternalId, so the
# rich map's extra fields can only produce updates: at least crpates (cost
# center, phone, address) and dave2 (primary-org title, cost center, address).
# Groups are unchanged and memberships stay matched by ExternalId.
for run in "$RUN1" "$RUN2"; do
  check assert_log summary "$run" \
    --users "update>=2" create=0 delete=0 recreate=0 updateForced=0 \
            adoptLocal=0 adoptForeign=0 retain=0 \
    --groups create=0 update=0 delete=0 recreate=0 updateForced=0 \
             adoptLocal=0 adoptForeign=0 retain=0 \
    --members add=0 remove=0
  check assert_dry_run "$run"
done
check assert_log same "$RUN1" "$RUN2"

finish
