#!/usr/bin/env bash
# Lambda with the built-in default attribute map (ATTRIBUTE_MAP unset).
# Runs against the TestAccountExecution stack as deployed by the Deploy stage.
#  1. Reset the Identity Store to the baseline (local records).
#  2. Invoke: the baseline users are adopted (the stack's group match may bring
#     in more users than the baseline, so adoption is checked as a minimum).
#  3. Invoke again: the second run must find nothing to change (convergence).
source "$(dirname "$0")/../lib/common.sh"
setup_python

reset_baseline

FIRST="$LOG_DIR/lambda_default.log"
invoke_lambda "$FIRST"
check assert_log summary "$FIRST" --users "adoptLocal>=4" delete=0 recreate=0 updateForced=0
check assert_log count "$FIRST" --msg "attribute map: mapping Google user" --min 4

SECOND="$LOG_DIR/lambda_default_2.log"
invoke_lambda "$SECOND"
check assert_log converged "$SECOND"

finish
