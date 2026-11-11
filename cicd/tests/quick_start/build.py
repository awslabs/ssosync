#!/usr/bin/env python3
"""
Build the quick-start pipeline tests (run by the Configurations action).

Every quick-start template takes literal secret values (SCIM token, Google key,
...), which must never be written to the pipeline artifacts. So each test is a
small wrapper template that deploys the quick-start as a nested stack and
passes it {{resolve:secretsmanager:<arn>}} dynamic references in its template
body, the same pattern as cicd/account_execution/staging/stack.yml. The
parameter files hold secret Arns only (plus the non-secret Workload Identity
Federation configuration and attribute map).

Output (--out, e.g. deploy/quick-start-tests):
  quick-start/<path>              the quick-start templates rewritten to the
                                  build under test (rewrite.py)
  <Test>.yml                      the wrapper; its TemplateURL is the local
                                  quick-start/<path>, uploaded by
                                  `aws cloudformation package` (tests/quick_start/
                                  package/buildspec.yml) in the account under test
  <Test>-<AccountId>.json         the CloudFormation action's TemplateConfiguration

All tests deploy the same stack, one after another, so it is updated from one
quick-start to the next; TESTS is in that order. The classic quick-starts deploy
the legacy SSOSync application, quick-start/next-gen/ the next-gen ssosync one
(issue #399): the switch from a classic to a next-gen template is a customer's
upgrade, replacing the nested applications in place.

Environment (set by the staging buildspec): AppArn (the legacy staging
application), NextGenAppArn (the next-gen one), AppVersion,
GoogleCustomerIdArn, SecretGoogleCredentials, SecretGoogleAdminEmail,
SecretWIFServiceAccountEmail, SecretSCIMEndpoint, SecretSCIMAccessToken.
"""

import argparse
import json
import os
import re
import sys

from rewrite import rewrite

HERE = os.path.dirname(os.path.abspath(__file__))
ATTRIBUTE_MAP = os.path.join(HERE, "..", "attribute_map", "maps", "rich.json")
ROLE_NAME = "SSOSync_ExecutionRole"   # trusted by the <AccountId>-lambda provider
GROUP_MATCH = "name:AWS*"

# (test name, quick-start path, auth method), in deployment order. The pipeline
# invokes the function each quick-start names: SSOSync (classic), SSOSyncNG
# (next-gen).
TESTS = [
    ("SingleAccount", "single-account.yaml", "legacy"),
    ("SingleAccountAttributeMap", "single-account-attribute-map.yaml", "legacy"),
    ("NextGenSingleAccount", "next-gen/single-account.yaml", "wif"),   # classic -> next-gen
    ("NextGenSingleAccountAttributeMap", "next-gen/single-account-attribute-map.yaml", "wif"),
]

# Secret parameters: quick-start parameter -> wrapper parameter holding the Arn.
SECRETS = {
    "legacy": {"GoogleCustomerId": "GoogleCustomerIdArn", "GoogleCredentials": "GoogleCredentialsArn",
               "GoogleAdminEmail": "GoogleAdminEmailArn"},
    "wif": {"GoogleCustomerId": "GoogleCustomerIdArn",
            "GoogleImpersonateServiceAccount": "GoogleImpersonateServiceAccountArn"},
}
COMMON_SECRETS = {"SCIMEndpointUrl": "SCIMEndpointUrlArn", "SCIMEndpointAccessToken": "SCIMAccessTokenArn"}
ARN_SOURCES = {   # wrapper parameter -> environment variable
    "GoogleCustomerIdArn": "GoogleCustomerIdArn",
    "GoogleCredentialsArn": "SecretGoogleCredentials",
    "GoogleAdminEmailArn": "SecretGoogleAdminEmail",
    "GoogleImpersonateServiceAccountArn": "SecretWIFServiceAccountEmail",
    "SCIMEndpointUrlArn": "SecretSCIMEndpoint",
    "SCIMAccessTokenArn": "SecretSCIMAccessToken",
}


def parameter_names(template):
    """Top-level parameter names of a quick-start template (stdlib only)."""
    # Up to the next top-level key (column-0 comments such as "# Secrets" stay in).
    m = re.search(r"^Parameters:\n(.*?)(?=^[A-Za-z])", template, re.S | re.M)
    if not m:
        raise ValueError("no Parameters section")
    return set(re.findall(r"^  (\w+):\s*$", m.group(1), re.M))


def per_account_wif_config(config, account):
    """The credential configuration for this account's -lambda provider: only
    the provider at the end of "audience" differs per role and account."""
    cfg = dict(config)
    audience, n = re.subn(r"/providers/[^/]+$", f"/providers/{account}-lambda", cfg["audience"])
    if n != 1:
        raise ValueError(f"unexpected audience {cfg['audience']!r}")
    cfg["audience"] = audience
    return json.dumps(cfg, separators=(",", ":"))


def wrapper(name, path, method, qs_params):
    secrets = {k: v for k, v in {**SECRETS[method], **COMMON_SECRETS}.items()}
    missing = [k for k in secrets if k not in qs_params]
    if missing:
        raise ValueError(f"{path} has no {missing} parameter(s) for {method} authentication")
    wparams = sorted(set(secrets.values()) | {"Release"}
                     | ({"GoogleWIFConfig"} if method == "wif" else set())
                     | ({"AttributeMap"} if "AttributeMap" in qs_params else set()))
    nested = {"Release": "!Ref Release"}
    for qp, wp in secrets.items():
        nested[qp] = "!Sub '{{resolve:secretsmanager:${%s}}}'" % wp
    if method == "wif":
        nested["GoogleWIFConfig"] = "!Ref GoogleWIFConfig"
        if "RoleName" in qs_params:
            nested["RoleName"] = ROLE_NAME
    if "AttributeMap" in qs_params:
        nested["AttributeMap"] = "!Ref AttributeMap"
    nested.update({"GoogleGroupMatch": json.dumps(GROUP_MATCH), "ScheduleExpression": '""',
                   "LogLevel": "info",
                   # No stack-owned log group: one recreated by a late log write
                   # after the stack is deleted would block the next run.
                   "LogRetention": "'0'"})
    unknown = [k for k in nested if k not in qs_params]
    if unknown:
        raise ValueError(f"{path} has no {unknown} parameter(s)")
    lines = [
        "AWSTemplateFormatVersion: '2010-09-09'",
        "Description: >-",
        f"  Pipeline test of quick-start/{path} ({method} authentication), generated by",
        "  tests/quick_start/build.py. The secret parameters are passed as Secrets",
        "  Manager dynamic references, so no secret value is in the artifacts.",
        "Parameters:",
    ]
    for p in wparams:
        lines += [f"  {p}:", "    Type: String"]
        if p in ("GoogleWIFConfig", "AttributeMap"):
            lines += ["    Default: ''"]
    lines += ["Resources:", "  QuickStart:", "    Type: AWS::CloudFormation::Stack", "    Properties:",
              f"      TemplateURL: ./quick-start/{path}", "      Parameters:"]
    lines += [f"        {k}: {v}" for k, v in nested.items()]
    return "\n".join(lines) + "\n", wparams


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--quick-start-dir", required=True, help="the repository's quick-start/")
    ap.add_argument("--out", required=True)
    ap.add_argument("--wif-config", required=True, help="the PipelineGoogleWIFConfig credential configuration")
    ap.add_argument("--accounts", required=True, help="space or comma separated account ids")
    a = ap.parse_args()

    env = {k: os.environ.get(k, "") for k in ["AppArn", "NextGenAppArn", "AppVersion", *set(ARN_SOURCES.values())]}
    # The staging buildspec normalises unset ("") SecretsConfig entries.
    empty = [k for k, v in env.items() if not v]
    if empty:
        sys.exit(f"build.py: not set: {', '.join(sorted(empty))}")
    with open(a.wif_config) as f:
        wif_config = json.load(f)
    with open(ATTRIBUTE_MAP) as f:
        attribute_map = json.dumps(json.load(f), separators=(",", ":"))
    accounts = [x for x in re.split(r"[\s,]+", a.accounts) if x]

    for name, path, method in TESTS:
        with open(os.path.join(a.quick_start_dir, path)) as f:
            text, _ = rewrite(f.read(), env["AppArn"], env["AppVersion"], env["NextGenAppArn"])
        dst = os.path.join(a.out, "quick-start", path)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        with open(dst, "w") as f:
            f.write(text)
        qs_params = parameter_names(text)
        body, wparams = wrapper(name, path, method, qs_params)
        with open(os.path.join(a.out, f"{name}.yml"), "w") as f:
            f.write(body)
        for account in accounts:
            values = {"Release": env["AppVersion"]}
            for wp in wparams:
                if wp in ARN_SOURCES:
                    values[wp] = env[ARN_SOURCES[wp]]
            if "GoogleWIFConfig" in wparams:
                values["GoogleWIFConfig"] = per_account_wif_config(wif_config, account)
            if "AttributeMap" in wparams:
                values["AttributeMap"] = attribute_map
            assert all(v.startswith("arn:") for k, v in values.items() if k.endswith("Arn"))
            with open(os.path.join(a.out, f"{name}-{account}.json"), "w") as f:
                json.dump({"Parameters": values}, f, indent=2)
        print(f"{name}: quick-start/{path} ({method}) -> {name}.yml, {len(accounts)} parameter file(s)")


if __name__ == "__main__":
    main()
