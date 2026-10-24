#!/usr/bin/env bash
# Rich attribute map (maps/rich.json), CLI, live.
# Exercises [primary] vs [type=work] selection, no-primary resolving to nothing,
# custom-schema scalar and typed sources (case-insensitive type match), opt-in
# multi-valued targets, and warn-and-clear for missing custom schemas/fields.
# Then checks the result converges.
#
# Google test data relied on:
#   crpates - work org "Jupiter Mining Corporation" / "Science Officer" (primary);
#             phones mobile, then work "+44 20 7946 0001" (primary);
#             work address "Deck 16, Red Dwarf"; work website
#             "https://reddwarf.example/crpates";
#             SSOSync-Test.CostCodes: home "HOME-1", Work "WORK-100" (capital W)
#   dave2   - orgs: school "Red Dwarf Academy" / "Student" (primary), then
#             work "Jupiter Mining Corporation" / "Technician, Third Class" /
#             dept "Maintenance"; two phones, neither primary;
#             work address "Cargo Bay, Red Dwarf"; SSOSync-Test.CostCodes: work "WORK-300"
#   AceHole, olaf - SSOSync-Test schema present, no CostCodes
source "$(dirname "$0")/../lib/common.sh"
setup_cli

reset_and_adopt

LOG="$LOG_DIR/rich.log"
run_ssosync live "$MAPS/rich.json" "$LOG"

check assert_log summary "$LOG" --users create=0 delete=0 recreate=0 updateForced=0

resolved() { check assert_log count "$LOG" --msg "attribute map: entry resolved" \
  --field "source=$1" --field "resolved=$2" --min 1; }

# [primary] picks the primary organization even when it is not the work one (dave2)...
resolved "organizations[primary].title" "Science Officer"
resolved "organizations[primary].title" "Student"
# ...while [type=work] picks the work organization.
resolved "organizations[type=work].name" "Jupiter Mining Corporation"
resolved "organizations[type=work].department" "Maintenance"
# Custom-schema typed source; crpates's entry is typed "Work" (case-insensitive match).
resolved "customSchemas.SSOSync-Test.CostCodes[type=work]" "WORK-100"
resolved "customSchemas.SSOSync-Test.CostCodes[type=work]" "WORK-300"
# Opt-in multi-valued targets, each emitted as a single selected element.
resolved "phones[primary].value" "+44 20 7946 0001"
resolved "addresses[type=work].formatted" "Deck 16, Red Dwarf"
resolved "addresses[type=work].formatted" "Cargo Bay, Red Dwarf"
resolved "websites[type=work].value" "https://reddwarf.example/crpates"

# No primary phone (dave2): resolves to nothing, never falls back to the first entry.
check assert_log count "$LOG" --msg "attribute map: entry resolved" \
  --field "source=phones[primary].value" --field "resolved=" --field "reason=noPrimaryEntry" --min 1

# Warn-and-clear for missing custom sources, for every user.
MISSING="attribute map: custom-schema source not found on user; field cleared"
check assert_log count "$LOG" --msg "$MISSING" \
  --field reason=customSchemaAbsent --field schema=NoSuchSchema --min 4
check assert_log count "$LOG" --msg "$MISSING" \
  --field reason=customFieldAbsent --field schema=SSOSync-Test --field field=DoesNotExist --min 4
check assert_log count "$LOG" --msg "$MISSING" \
  --field reason=customFieldAbsent --field field=CostCodes --min 2

check assert_converged "$MAPS/rich.json" "$LOG_DIR/converge.log"

finish
