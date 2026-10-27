#!/usr/bin/env python3
"""
Local unit tests for check_log.py (no AWS or Google access needed):
  python3 cicd/tests/user_group_operations/test_check_log.py
"""

import json
import os
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "check_log.py")

ISSUER = "arn:aws:identitystore::111111111111:identitystore/d-1/provisioningtenant/Dbv1-AbC"
LOWER = ISSUER.lower()


def run(*args):
    p = subprocess.run([sys.executable, SCRIPT, *args], capture_output=True, text=True)
    return p.returncode, p.stdout


def summary(kind, **kw):
    rec = {k: 0 for k in ["equal", "create", "update", "updateForced", "delete", "recreate",
                          "adoptLocal", "adoptForeign", "retain"]}
    rec.update(kw)
    rec["msg"] = f"Change Summary: {kind}"
    return rec


def check(kind, name, aws_id, ext_id, issuer=ISSUER, expected=ISSUER, ext_match=True, count=1, by="only"):
    return {"msg": "ExternalId issuer check", "kind": kind, "name": name, "awsId": aws_id,
            "externalId": ext_id, "issuer": issuer, "expectedIssuer": expected,
            # Mirrors ssosync's issuerMatches: case-insensitive (issue #383).
            "issuerMatch": issuer.lower() == expected.lower(), "issuerExactMatch": issuer == expected,
            "externalIdMatch": ext_match,
            "caseInsensitiveMatch": issuer.lower() == expected.lower(),
            "externalIdCount": count, "externalIdSelectedBy": by}


def expected_line(kind, issuer=ISSUER):
    return {"msg": "ExternalId issuer expected", "kind": kind, "expectedIssuer": issuer}


class _LogFiles(unittest.TestCase):
    """Temp-file helpers shared by the test classes (no tests of its own)."""

    def setUp(self):
        self.files = []

    def tearDown(self):
        for f in self.files:
            os.unlink(f)

    def write(self, data, suffix=".log"):
        f = tempfile.NamedTemporaryFile("w", suffix=suffix, delete=False)
        if isinstance(data, list):
            for line in data:
                f.write((line if isinstance(line, str) else json.dumps(line)) + "\n")
        else:
            json.dump(data, f)
        f.close()
        self.files.append(f.name)
        return f.name

    def good_log(self):
        return self.write([
            {"msg": "getIssuerArn()", "IssuerArn": ISSUER, "level": "info"},
            expected_line("user"), check("user", "bob@example.com", "u-2", "G2"),
            check("user", "alice@example.com", "u-1", "G1"), summary("Users", equal=2),
            expected_line("group"), check("group", "AWSAdmins", "g-1", "GG1"), summary("Groups", equal=1),
            "not json at all",
        ])


class CheckLogTest(_LogFiles):
    def test_field(self):
        rc, out = run("field", self.good_log(), "--msg", "getIssuerArn()", "--key", "IssuerArn")
        self.assertEqual(rc, 0)
        self.assertEqual(out.strip(), ISSUER)
        self.assertEqual(run("field", self.good_log(), "--msg", "getIssuerArn()", "--key", "Nope")[0], 1)

    def test_summary_and_converged(self):
        p = self.good_log()
        self.assertEqual(run("summary", p, "--users", "equal=2", "adoptForeign=0")[0], 0)
        self.assertEqual(run("summary", p, "--users", "equal=3")[0], 1)
        self.assertEqual(run("converged", p)[0], 0)

    def test_converged_fails_on_adopt_foreign_and_would_lines(self):
        p = self.write([summary("Users", adoptForeign=4), summary("Groups")])
        self.assertEqual(run("converged", p)[0], 1)
        p = self.write([summary("Users"), summary("Groups"), {"msg": "DRY RUN: Would update user", "user": "x"}])
        self.assertEqual(run("converged", p)[0], 1)

    def test_issuer_pass(self):
        self.assertEqual(run("issuer", self.good_log(), "--expected", ISSUER)[0], 0)

    # The live finding: users carry the tenant id in its original case, groups
    # lowercased. Both match (ssosync's case-insensitive rule); the case-only
    # difference is reported.
    def test_issuer_case_only_difference_passes_and_is_reported(self):
        p = self.write([expected_line("user"), check("user", "a", "u-1", "G1"),
                        expected_line("group"), check("group", "g", "g-1", "GG1", issuer=LOWER)])
        rc, out = run("issuer", p, "--expected", ISSUER)
        self.assertEqual(rc, 0, out)
        self.assertIn("1 only ignoring letter case", out)

    def test_issuer_fails_on_account_mismatch_with_diagnostics(self):
        other = ISSUER.replace("111111111111", "222222222222")
        p = self.write([expected_line("user"), check("user", "a", "u-1", "G1", issuer=other),
                        expected_line("group"), check("group", "g", "g-1", "GG1")])
        rc, out = run("issuer", p, "--expected", ISSUER)
        self.assertEqual(rc, 1)
        self.assertIn("caseInsensitiveMatch=false", out)

    def test_issuer_fails_when_expected_differs_or_missing(self):
        self.assertEqual(run("issuer", self.good_log(), "--expected", LOWER)[0], 1)
        p = self.write([check("user", "a", "u-1", "G1"), check("group", "g", "g-1", "GG1")])
        self.assertEqual(run("issuer", p, "--expected", ISSUER)[0], 1)

    def test_issuer_minimums(self):
        p = self.write([expected_line("user"), expected_line("group"), check("user", "a", "u-1", "G1")])
        rc, out = run("issuer", p, "--expected", ISSUER)
        self.assertEqual(rc, 1)
        self.assertIn("0 group issuer-check", out)

    def test_pick_is_first_managed_by_name(self):
        rc, out = run("pick", self.good_log(), "--kind", "user")
        self.assertEqual(rc, 0)
        self.assertEqual(out.strip(), "u-1\talice@example.com\tG1")
        p = self.write([check("user", "a", "u-1", "G1", ext_match=False)])
        self.assertEqual(run("pick", p, "--kind", "user")[0], 1)

    def test_record(self):
        p = self.write([check("user", "alice@example.com", "u-1", "TAMPER", ext_match=False)])
        self.assertEqual(run("record", p, "--kind", "user", "--name", "alice@example.com",
                             "--field", "issuerMatch=true", "--field", "externalIdMatch=false")[0], 0)
        self.assertEqual(run("record", p, "--kind", "user", "--name", "alice@example.com",
                             "--field", "externalIdMatch=true")[0], 1)
        self.assertEqual(run("record", p, "--kind", "user", "--name", "nobody")[0], 1)

    def test_truth_pass_and_multi_external_ids(self):
        log = self.write([check("user", "alice", "u-1", "G1", count=2, by="issuerMatch"),
                          check("group", "AWSAdmins", "g-1", "GG1")])
        users = self.write({"Users": [{"UserId": "u-1", "ExternalIds": [
            {"Issuer": "other-idp", "Id": "X"}, {"Issuer": ISSUER, "Id": "G1"}]}]}, ".json")
        groups = self.write({"Groups": [{"GroupId": "g-1", "ExternalIds": [{"Issuer": ISSUER, "Id": "GG1"}]}]}, ".json")
        rc, out = run("truth", log, "--expected", ISSUER, "--users", users, "--groups", groups)
        self.assertEqual(rc, 0, out)
        self.assertIn("more than one ExternalId were matched", out)

    def test_truth_fails_when_multi_not_selected_by_issuer(self):
        log = self.write([check("user", "alice", "u-1", "G1", count=2, by="first")])
        users = self.write({"Users": [{"UserId": "u-1", "ExternalIds": [
            {"Issuer": ISSUER, "Id": "G1"}, {"Issuer": "other", "Id": "X"}]}]}, ".json")
        groups = self.write({"Groups": []}, ".json")
        self.assertEqual(run("truth", log, "--expected", ISSUER, "--users", users, "--groups", groups)[0], 1)

    def test_truth_case_only_difference_passes_and_is_reported(self):
        log = self.write([check("group", "AWSAdmins", "g-1", "GG1", issuer=LOWER)])
        users = self.write({"Users": []}, ".json")
        groups = self.write({"Groups": [{"GroupId": "g-1", "ExternalIds": [{"Issuer": LOWER, "Id": "GG1"}]}]}, ".json")
        rc, out = run("truth", log, "--expected", ISSUER, "--users", users, "--groups", groups)
        self.assertEqual(rc, 0, out)
        self.assertIn("0 exactly, 1 differing only in letter case", out)

    def test_truth_fails_on_other_account_or_missing_record(self):
        log = self.write([check("user", "alice", "u-1", "G1")])
        other = ISSUER.replace("111111111111", "222222222222")
        users = self.write({"Users": [{"UserId": "u-1", "ExternalIds": [{"Issuer": other, "Id": "G1"}]}]}, ".json")
        groups = self.write({"Groups": []}, ".json")
        self.assertEqual(run("truth", log, "--expected", ISSUER, "--users", users, "--groups", groups)[0], 1)
        empty = self.write({"Users": []}, ".json")
        self.assertEqual(run("truth", log, "--expected", ISSUER, "--users", empty, "--groups", groups)[0], 1)


class CountsAndFindTest(_LogFiles):
    def test_counts_exact(self):
        p = self.write([summary("Users", update=1, equal=3), summary("Groups", recreate=1)])
        self.assertEqual(run("counts", p, "--users", "update=1", "--groups", "recreate=1")[0], 0)
        # an unlisted non-zero action fails
        self.assertEqual(run("counts", p, "--users", "update=1")[0], 1)
        # "equal" is not an action counter, so asking for it is a usage error
        self.assertNotEqual(run("counts", p, "--users", "update=1", "equal=3", "--groups", "recreate=1")[0], 0)

    def test_counts_rejects_unknown_counter(self):
        p = self.write([summary("Users"), summary("Groups")])
        rc, _ = run("counts", p, "--users", "updated=1")
        self.assertNotEqual(rc, 0)

    def test_find_case_insensitive(self):
        p = self.write([check("user", "Test.User@Example.com", "u-9", "G9")])
        rc, out = run("find", p, "--kind", "user", "--name", "test.user@example.com")
        self.assertEqual(rc, 0)
        self.assertEqual(out.strip(), "u-9\tG9\ttrue")
        self.assertEqual(run("find", p, "--kind", "group", "--name", "x")[0], 1)


if __name__ == "__main__":
    unittest.main()
