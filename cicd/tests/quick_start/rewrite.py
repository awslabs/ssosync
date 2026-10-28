#!/usr/bin/env python3
"""
Point a quick-start template at the build under test instead of the public
release.

The quick-start templates deploy SSOSync from the public Serverless Application
Repository application and offer only published releases. For a pipeline test
the template must deploy the staging build instead, so this rewrites:

  * every  ApplicationId: arn:aws:serverlessrepo:...  ->  --app-arn
  * the Release parameter (the block marked #SemanticVersion, as maintained by
    the release workflow): Default and AllowedValues  ->  --app-version only

Everything else is left byte-for-byte unchanged. The script fails if either
rewrite does not happen, so a change to the templates' layout is caught rather
than silently testing the public release.

Usage:
  python3 rewrite.py --in quick-start/single-account.yaml --out template.yaml \
      --app-arn arn:aws:serverlessrepo:<region>:<account>:applications/SSOSync-Staging \
      --app-version 2.10.0-abc1234
"""

import argparse
import re
import sys

APP_ID = re.compile(r"^(\s*ApplicationId:\s*)arn:aws:serverlessrepo:\S+\s*$")
MARKER = re.compile(r"^\s*#\s*Semantic ?Version\s*$")
DEFAULT = re.compile(r"^(\s*Default:\s*).*$")
ALLOWED = re.compile(r"^(\s*)AllowedValues:\s*$")
ITEM = re.compile(r"^(\s*)-\s+\S.*$")


def rewrite(text, app_arn, app_version):
    lines = text.split("\n")
    out = []
    app_ids = releases = 0
    i = 0
    while i < len(lines):
        line = lines[i]
        m = APP_ID.match(line)
        if m:
            out.append(f"{m.group(1)}{app_arn}")
            app_ids += 1
            i += 1
            continue
        if MARKER.match(line):
            out.append(line)
            i += 1
            # Default: <version>
            m = DEFAULT.match(lines[i]) if i < len(lines) else None
            if not m:
                raise ValueError("#SemanticVersion is not followed by a Default line")
            out.append(f"{m.group(1)}{app_version}")
            i += 1
            # AllowedValues: then its "- <version>" items
            m = ALLOWED.match(lines[i]) if i < len(lines) else None
            if not m:
                raise ValueError("Release Default is not followed by AllowedValues")
            out.append(lines[i])
            i += 1
            indent = None
            while i < len(lines):
                item = ITEM.match(lines[i])
                if not item:
                    break
                indent = indent or item.group(1)
                i += 1
            if indent is None:
                raise ValueError("Release AllowedValues has no items")
            out.append(f"{indent}- {app_version}")
            releases += 1
            continue
        out.append(line)
        i += 1
    if app_ids == 0:
        raise ValueError("no 'ApplicationId: arn:aws:serverlessrepo:...' line found")
    if releases != 1:
        raise ValueError(f"expected one #SemanticVersion Release block, found {releases}")
    return "\n".join(out), app_ids


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--in", dest="src", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--app-arn", required=True)
    p.add_argument("--app-version", required=True)
    a = p.parse_args()
    if not a.app_arn.startswith("arn:aws:serverlessrepo:"):
        sys.exit(f"--app-arn is not a Serverless Application Repository Arn: {a.app_arn!r}")
    if not a.app_version.strip():
        sys.exit("--app-version is empty")
    with open(a.src) as f:
        text = f.read()
    try:
        result, app_ids = rewrite(text, a.app_arn, a.app_version)
    except ValueError as e:
        sys.exit(f"{a.src}: {e}")
    with open(a.out, "w") as f:
        f.write(result)
    print(f"{a.src} -> {a.out}: {app_ids} ApplicationId(s) -> {a.app_arn}; Release -> {a.app_version}")


if __name__ == "__main__":
    main()
