#!/usr/bin/env bash
# Quick-start template smoke test (release pipeline, delegated admin account).
#
# Deploys one quick-start template against the build under test, invokes the
# function it creates, checks the result like the Lambda smoke test, and
# always deletes the stack again.
#
#  1. Rewrite the template (rewrite.py) so it deploys the staging SAR
#     application and version instead of the public release.
#  2. Read the secret values from the release account's Pipeline* secrets
#     (Arns in deploy/quickstart.json) at runtime; they are written only to a
#     private temporary parameters file, never to the build artifacts or log.
#  3. Create the stack (quick-starts create the SSOSync* secrets, so this runs
#     after the TestAccountExecution stack has been removed) and invoke the
#     function: no FunctionError expected.
#  4. Check ATTRIBUTE_MAP: unset by default, or the SSOSyncAttributeMap secret
#     Arn when an inline map is passed (EXPECT_ATTRIBUTE_MAP=secret).
#
# Environment (set by the CodeBuild project):
#   QUICKSTART_TEMPLATE   file name under deploy/quick-start/, e.g. single-account.yaml
#   STACK_NAME            stack to create (deleted again at the end)
#   CFN_ROLE_ARN          CloudFormation service role for the stack operations
#   GROUP_MATCH           GoogleGroupMatch for the deployed function (default name:AWS*)
#   ATTRIBUTE_MAP_FILE    optional: inline map passed as the AttributeMap parameter
#   EXPECT_ATTRIBUTE_MAP  none (default) | secret
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"   # artifact root
QS_DIR="$ROOT_DIR/tests/quick_start"
DEPLOY="$ROOT_DIR/deploy"
CONFIG="$DEPLOY/quickstart.json"
FUNCTION_NAME="SSOSync"   # fixed by the quick-start templates
GROUP_MATCH="${GROUP_MATCH:-name:AWS*}"
EXPECT_ATTRIBUTE_MAP="${EXPECT_ATTRIBUTE_MAP:-none}"
WORK="$(mktemp -d)"
chmod 700 "$WORK"

for v in QUICKSTART_TEMPLATE STACK_NAME CFN_ROLE_ARN; do
  if [ -z "${!v:-}" ]; then echo "ERROR: $v is not set"; exit 1; fi
done
[ -f "$CONFIG" ] || { echo "ERROR: $CONFIG not found (built by the staging step)"; exit 1; }
[ -f "$DEPLOY/quick-start/$QUICKSTART_TEMPLATE" ] || { echo "ERROR: deploy/quick-start/$QUICKSTART_TEMPLATE not found"; exit 1; }

cfg() { jq -er --arg k "$1" '.[$k] | select(. != null and . != "")' "$CONFIG"; }

stack_exists() {
  aws cloudformation describe-stacks --stack-name "$STACK_NAME" > /dev/null 2>&1
}

show_events() {
  echo ">>> recent stack events ($STACK_NAME)"
  aws cloudformation describe-stack-events --stack-name "$STACK_NAME" \
    --query 'StackEvents[?contains(ResourceStatus, `FAILED`)].[Timestamp,LogicalResourceId,ResourceStatus,ResourceStatusReason]' \
    --output text 2>/dev/null | head -20 || true
}

delete_stack() {
  if stack_exists; then
    echo ">>> delete stack $STACK_NAME"
    aws cloudformation delete-stack --stack-name "$STACK_NAME" --role-arn "$CFN_ROLE_ARN"
    aws cloudformation wait stack-delete-complete --stack-name "$STACK_NAME"
  fi
}

cleanup() {
  local rc=$?
  rm -rf "$WORK"
  if [ $rc -ne 0 ]; then show_events; fi
  delete_stack || { echo "ERROR: could not delete $STACK_NAME"; rc=1; }
  if [ $rc -eq 0 ]; then echo "=== TEST PASSED ==="; else echo "=== TEST FAILED ==="; fi
  exit $rc
}
trap cleanup EXIT

QUICK_START_DIR="$DEPLOY/quick-start" python3 "$QS_DIR/test_rewrite.py"

APP_ARN="$(cfg AppArn)"
APP_VERSION="$(cfg AppVersion)"
echo ">>> $QUICKSTART_TEMPLATE against $APP_ARN version $APP_VERSION"
python3 "$QS_DIR/rewrite.py" --in "$DEPLOY/quick-start/$QUICKSTART_TEMPLATE" \
  --out "$WORK/template.yaml" --app-arn "$APP_ARN" --app-version "$APP_VERSION"

# A stack left behind by an earlier run would hold the SSOSync* secret names.
delete_stack

# The quick-start's Secrets stack creates these names; fail clearly if one
# already exists outside it.
for name in SSOSyncGoogleCustomerId SSOSyncGoogleCredentials SSOSyncGoogleAdminEmail \
            SSOSyncSCIMEndpointUrl SSOSyncSCIMAccessToken SSOSyncAttributeMap; do
  if aws secretsmanager describe-secret --secret-id "$name" > /dev/null 2>&1; then
    echo "ERROR: secret $name already exists in this account; the quick-start creates it"
    exit 1
  fi
done

secret() { aws secretsmanager get-secret-value --secret-id "$1" --query SecretString --output text; }

# Plain assignments, so a failed read stops the test (set -e) instead of
# passing an empty value.
GOOGLE_CUSTOMER_ID="$(secret "$(cfg GoogleCustomerIdArn)")"
GOOGLE_CREDENTIALS="$(secret "$(cfg GoogleCredentialsArn)")"
GOOGLE_ADMIN_EMAIL="$(secret "$(cfg GoogleAdminEmailArn)")"
SCIM_ENDPOINT_URL="$(secret "$(cfg SCIMEndpointUrlArn)")"
SCIM_ACCESS_TOKEN="$(secret "$(cfg SCIMAccessTokenArn)")"

ATTRIBUTE_MAP=""
if [ -n "${ATTRIBUTE_MAP_FILE:-}" ]; then
  ATTRIBUTE_MAP="$(jq -c . "$ROOT_DIR/$ATTRIBUTE_MAP_FILE")"
fi

# Parameters file: secret values only ever live in this private file.
umask 077
jq -n \
  --arg Release "$APP_VERSION" \
  --arg GoogleCustomerId "$GOOGLE_CUSTOMER_ID" \
  --arg GoogleCredentials "$GOOGLE_CREDENTIALS" \
  --arg GoogleAdminEmail "$GOOGLE_ADMIN_EMAIL" \
  --arg SCIMEndpointUrl "$SCIM_ENDPOINT_URL" \
  --arg SCIMEndpointAccessToken "$SCIM_ACCESS_TOKEN" \
  --arg GoogleGroupMatch "$GROUP_MATCH" \
  --arg AttributeMap "$ATTRIBUTE_MAP" \
  --arg HasAttributeMap "$(grep -qE '^  AttributeMap:' "$WORK/template.yaml" && echo yes || echo no)" \
  '[
     {ParameterKey: "Release", ParameterValue: $Release},
     {ParameterKey: "GoogleCustomerId", ParameterValue: $GoogleCustomerId},
     {ParameterKey: "GoogleCredentials", ParameterValue: $GoogleCredentials},
     {ParameterKey: "GoogleAdminEmail", ParameterValue: $GoogleAdminEmail},
     {ParameterKey: "SCIMEndpointUrl", ParameterValue: $SCIMEndpointUrl},
     {ParameterKey: "SCIMEndpointAccessToken", ParameterValue: $SCIMEndpointAccessToken},
     {ParameterKey: "GoogleGroupMatch", ParameterValue: $GoogleGroupMatch},
     {ParameterKey: "ScheduleExpression", ParameterValue: ""},
     {ParameterKey: "LogLevel", ParameterValue: "info"}
   ] + (if $HasAttributeMap == "yes" then [{ParameterKey: "AttributeMap", ParameterValue: $AttributeMap}] else [] end)' \
  > "$WORK/params.json"
umask 022
unset GOOGLE_CUSTOMER_ID GOOGLE_CREDENTIALS GOOGLE_ADMIN_EMAIL SCIM_ENDPOINT_URL SCIM_ACCESS_TOKEN
if [ -n "$ATTRIBUTE_MAP" ] && ! grep -qE '^  AttributeMap:' "$WORK/template.yaml"; then
  echo "ERROR: ATTRIBUTE_MAP_FILE is set but $QUICKSTART_TEMPLATE has no AttributeMap parameter"
  exit 1
fi

echo ">>> create stack $STACK_NAME"
aws cloudformation create-stack --stack-name "$STACK_NAME" \
  --template-body "file://$WORK/template.yaml" \
  --parameters "file://$WORK/params.json" \
  --capabilities CAPABILITY_IAM CAPABILITY_NAMED_IAM CAPABILITY_AUTO_EXPAND \
  --role-arn "$CFN_ROLE_ARN" --query StackId --output text
rm -f "$WORK/params.json"
aws cloudformation wait stack-create-complete --stack-name "$STACK_NAME"
echo "ok:   stack $STACK_NAME created"

# Smoke test, as tests/account_execution/lambda: the function runs without error.
aws lambda invoke --cli-read-timeout 900 --function-name "$FUNCTION_NAME" "$WORK/response.json" > "$WORK/invoke.json"
cat "$WORK/invoke.json"
if jq -e 'has("FunctionError")' "$WORK/invoke.json" > /dev/null; then
  echo "FAIL: $FUNCTION_NAME returned FunctionError"
  cat "$WORK/response.json"
  exit 1
fi
echo "ok:   $FUNCTION_NAME ran without FunctionError"

# ATTRIBUTE_MAP wiring.
ENV_MAP="$(aws lambda get-function-configuration --function-name "$FUNCTION_NAME" \
  --query 'Environment.Variables.ATTRIBUTE_MAP' --output text)"
case "$EXPECT_ATTRIBUTE_MAP" in
  none)
    if [ "$ENV_MAP" != "None" ]; then echo "FAIL: ATTRIBUTE_MAP is set ($ENV_MAP), expected unset"; exit 1; fi
    echo "ok:   ATTRIBUTE_MAP unset (built-in default map)"
    ;;
  secret)
    case "$ENV_MAP" in
      arn:aws:secretsmanager:*:secret:SSOSyncAttributeMap-*) echo "ok:   ATTRIBUTE_MAP is the SSOSyncAttributeMap secret ($ENV_MAP)" ;;
      *) echo "FAIL: ATTRIBUTE_MAP is $ENV_MAP, expected the SSOSyncAttributeMap secret Arn"; exit 1 ;;
    esac
    if [ "$(secret "$ENV_MAP" | jq -c .)" != "$ATTRIBUTE_MAP" ]; then
      echo "FAIL: the SSOSyncAttributeMap secret does not hold the inline map"
      exit 1
    fi
    echo "ok:   SSOSyncAttributeMap holds the inline map"
    ;;
  *) echo "ERROR: EXPECT_ATTRIBUTE_MAP must be none or secret"; exit 1 ;;
esac
