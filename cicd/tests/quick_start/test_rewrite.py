#!/usr/bin/env python3
"""
Local tests for rewrite.py, run against every real quick-start template.

  python3 cicd/tests/quick_start/test_rewrite.py
"""

import glob
import os
import unittest

import yaml

from rewrite import rewrite

HERE = os.path.dirname(os.path.abspath(__file__))
# The repo's quick-start/ by default; run.sh points this at deploy/quick-start
# inside the test artifact.
QUICK_START = os.environ.get("QUICK_START_DIR", os.path.join(HERE, "..", "..", "..", "quick-start"))
ARN = "arn:aws:serverlessrepo:us-east-2:123456789012:applications/SSOSync-Staging"
VERSION = "2.10.0-abc1234"


class CfnLoader(yaml.SafeLoader):
    pass


CfnLoader.add_multi_constructor("!", lambda loader, suffix, node: None)


def templates():
    found = sorted(glob.glob(os.path.join(QUICK_START, "*.yaml")))
    assert found, "no quick-start templates found"
    return found


class RewriteQuickStarts(unittest.TestCase):
    def test_every_template_points_at_the_build_under_test(self):
        for path in templates():
            with self.subTest(template=os.path.basename(path)):
                with open(path) as f:
                    original = f.read()
                result, app_ids = rewrite(original, ARN, VERSION)
                self.assertGreaterEqual(app_ids, 1)
                self.assertNotIn("004480582608", result, "public SAR application still referenced")
                doc = yaml.load(result, Loader=CfnLoader)
                release = doc["Parameters"]["Release"]
                self.assertEqual(VERSION, str(release["Default"]))
                self.assertEqual([VERSION], [str(v) for v in release["AllowedValues"]])
                for name, res in doc["Resources"].items():
                    if res.get("Type") == "AWS::Serverless::Application":
                        self.assertEqual(ARN, res["Properties"]["Location"]["ApplicationId"], name)

    def test_only_the_release_and_application_lines_change(self):
        for path in templates():
            with self.subTest(template=os.path.basename(path)):
                with open(path) as f:
                    original = f.read()
                result, _ = rewrite(original, ARN, VERSION)
                before = [l for l in original.split("\n") if "ApplicationId:" not in l]
                after = [l for l in result.split("\n") if "ApplicationId:" not in l]
                # Drop the Release block (Default + AllowedValues items) from both.
                def strip(lines):
                    out, skip = [], False
                    for l in lines:
                        if "#SemanticVersion" in l:
                            skip = True
                            continue
                        if skip and (l.strip().startswith(("Default:", "AllowedValues:", "- "))):
                            continue
                        skip = False
                        out.append(l)
                    return out
                self.assertEqual(strip(before), strip(after))

    def test_missing_release_block_fails(self):
        with self.assertRaises(ValueError):
            rewrite("Resources:\n  A:\n    Properties:\n      ApplicationId: arn:aws:serverlessrepo:x\n", ARN, VERSION)

    def test_missing_application_fails(self):
        with self.assertRaises(ValueError):
            rewrite("    #SemanticVersion\n    Default: 1\n    AllowedValues:\n      - 1\n", ARN, VERSION)


if __name__ == "__main__":
    unittest.main()
