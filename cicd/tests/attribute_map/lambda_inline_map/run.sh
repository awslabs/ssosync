#!/usr/bin/env bash
# Lambda with an inline attribute map in the ATTRIBUTE_MAP environment variable.
# The preceding pipeline action redeploys TestAccountExecution with
# deploy/attributemap.json, which passes maps/rich.json inline as the
# AttributeMap parameter.
#  1. Invoke: the debug trace proves the inline map is the one in effect (a
#     value only reachable through a custom-schema source in rich.json).
#  2. Invoke again: the second run must find nothing to change (convergence).
source "$(dirname "$0")/../lib/common.sh"
setup_python

FIRST="$LOG_DIR/lambda_inline.log"
invoke_lambda "$FIRST"
check assert_log count "$FIRST" --msg "attribute map: entry resolved" \
  --field "source=customSchemas.SSOSync-Test.CostCodes[type=work]" --field "resolved=WORK-100" --min 1
check assert_log summary "$FIRST" --users delete=0 recreate=0 updateForced=0

SECOND="$LOG_DIR/lambda_inline_2.log"
invoke_lambda "$SECOND"
check assert_log converged "$SECOND"

finish
