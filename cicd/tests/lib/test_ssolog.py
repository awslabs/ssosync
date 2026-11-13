#!/usr/bin/env python3
"""
Local unit tests for ssolog.py, through both of its front ends (no AWS or
Google access needed; `make test-cicd` runs them):
  tests/user_group_operations/check_log.py  (CheckLogTest, CountsAndFindTest)
  tests/attribute_map/lib/assert_log.py     (AssertLogTest)
"""

import json
import os
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
CHECK_LOG = os.path.join(HERE, "..", "user_group_operations", "check_log.py")
ASSERT_LOG = os.path.join(HERE, "..", "attribute_map", "lib", "assert_log.py")

ISSUER = "arn:aws:identitystore::111111111111:identitystore/d-1/provisioningtenant/Dbv1-AbC"
LOWER = ISSUER.lower()


def run(*args):
    p = subprocess.run([sys.executable, CHECK_LOG, *args], capture_output=True, text=True)
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

    def test_field_prints_objects_as_json(self):
        # #404: default_map reads "Cache Stats" with jq.
        stats = {"/Red Dwarf/Petersen's Locker": {"Precaches": 2, "Misses": 0}, "/": {"Precaches": 0}}
        p = self.write([{"msg": "Caching Performance", "Cache Stats": stats, "list": [1, "a"]}])
        rc, out = run("field", p, "--msg", "Caching Performance", "--key", "Cache Stats")
        self.assertEqual(rc, 0)
        self.assertEqual(json.loads(out), stats)
        self.assertEqual(json.loads(run("field", p, "--msg", "Caching Performance", "--key", "list")[1]), [1, "a"])

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

    def test_logrus_text_line_is_parsed(self):
        # Shared with assert_log.py: a text-format fatal has the same shape as JSON.
        p = self.write(['time="2026-01-01T00:00:00Z" level=fatal msg="cannot start: \\"x\\"" code=3'])
        rc, out = run("field", p, "--msg", 'cannot start: "x"', "--key", "code")
        self.assertEqual((rc, out.strip()), (0, "3"))

    def test_converged_fails_on_member_change(self):
        p = self.write([summary("Users"), summary("Groups"),
                        {"msg": "Change Summary: Group Members", "add": 1, "remove": 0}])
        self.assertEqual(run("converged", p)[0], 1)

    def test_issuer_pass(self):
        self.assertEqual(run("issuer", self.good_log(), "--expected", ISSUER)[0], 0)

    # The live finding: records of one tenant carry the tenant id in its
    # original case or lowercased (both seen among users; groups lowercased).
    # Both match (ssosync's case-insensitive rule); the case-only difference is
    # reported.
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



# --- attribute-map suite (assert_log.py) ---------------------------------------

def write_log(lines):
    f = tempfile.NamedTemporaryFile("w", suffix=".log", delete=False)
    for line in lines:
        f.write(line if isinstance(line, str) else json.dumps(line))
        f.write("\n")
    f.close()
    return f.name


def run_am(*args):
    p = subprocess.run([sys.executable, ASSERT_LOG, *args], capture_output=True, text=True)
    return p.returncode, p.stdout


def users(**kw):
    base = dict(equal=0, create=0, adoptLocal=0, adoptForeign=0, update=0,
                updateForced=0, delete=0, recreate=0, retain=0)
    base.update(kw)
    return {"msg": "Change Summary: Users", **base}


def groups(**kw):
    rec = users(**kw)
    rec["msg"] = "Change Summary: Groups"
    return rec


class AssertLogTest(unittest.TestCase):
    def setUp(self):
        self._files = []

    def tearDown(self):
        for f in self._files:
            os.unlink(f)

    def log(self, lines):
        p = write_log(lines)
        self._files.append(p)
        return p

    # summary
    def test_summary_equal_pass_and_fail(self):
        p = self.log([users(create=2)])
        self.assertEqual(run_am("summary", p, "--users", "create=2")[0], 0)
        self.assertEqual(run_am("summary", p, "--users", "create=3")[0], 1)

    def test_summary_at_least(self):
        p = self.log([users(adoptLocal=4)])
        self.assertEqual(run_am("summary", p, "--users", "adoptLocal>=4")[0], 0)
        self.assertEqual(run_am("summary", p, "--users", "adoptLocal>=5")[0], 1)

    def test_summary_missing_line_fails(self):
        p = self.log([{"msg": "something else"}])
        rc, out = run_am("summary", p, "--users", "create=0")
        self.assertEqual(rc, 1)
        self.assertIn("no 'Change Summary: Users' line", out)

    def test_summary_uses_last_line(self):
        p = self.log([users(create=5), users(create=0)])
        self.assertEqual(run_am("summary", p, "--users", "create=0")[0], 0)

    def test_summary_members_are_summed(self):
        p = self.log([
            {"msg": "Change Summary: Group Members", "add": 1, "remove": 0},
            {"msg": "Change Summary: Group Members", "add": 2, "remove": 1},
        ])
        self.assertEqual(run_am("summary", p, "--members", "add=3", "remove=1")[0], 0)
        self.assertEqual(run_am("summary", p, "--members", "add=1")[0], 1)

    # converged
    def test_converged_pass(self):
        p = self.log([users(equal=4), groups(equal=1),
                      {"msg": "Change Summary: Group Members", "add": 0, "remove": 0}])
        self.assertEqual(run_am("converged", p)[0], 0)

    def test_converged_fails_on_user_change(self):
        p = self.log([users(update=1), groups()])
        self.assertEqual(run_am("converged", p)[0], 1)

    def test_converged_fails_on_member_change(self):
        p = self.log([users(), groups(),
                      {"msg": "Change Summary: Group Members", "add": 1, "remove": 0}])
        self.assertEqual(run_am("converged", p)[0], 1)

    def test_converged_fails_on_dry_run_action(self):
        p = self.log([users(), groups(), {"msg": "DRY RUN: Would update user", "user": "a"}])
        rc, out = run_am("converged", p)
        self.assertEqual(rc, 1)
        self.assertIn("proposed", out)

    # count / absent
    def test_count_min_eq_and_fields(self):
        p = self.log([
            {"msg": "attribute map: entry resolved", "source": "a", "resolved": "x"},
            {"msg": "attribute map: entry resolved", "source": "a", "resolved": "y"},
        ])
        base = ["count", p, "--msg", "entry resolved"]
        self.assertEqual(run_am(*base, "--eq", "2")[0], 0)
        self.assertEqual(run_am(*base, "--eq", "1")[0], 1)
        self.assertEqual(run_am(*base, "--min", "3")[0], 1)
        self.assertEqual(run_am(*base, "--field", "resolved=x", "--eq", "1")[0], 0)
        self.assertEqual(run_am(*base, "--field", "resolved=z", "--min", "1")[0], 1)

    def test_count_matches_empty_field_value(self):
        p = self.log([{"msg": "entry resolved", "resolved": "", "reason": "noPrimaryEntry"}])
        self.assertEqual(run_am("count", p, "--msg", "entry resolved", "--field", "resolved=",
                             "--field", "reason=noPrimaryEntry", "--min", "1")[0], 0)

    def test_count_missing_field_does_not_match_empty(self):
        p = self.log([{"msg": "entry resolved"}])
        self.assertEqual(run_am("count", p, "--msg", "entry resolved", "--field", "resolved=",
                             "--min", "1")[0], 1)

    def test_count_bool_field(self):
        p = self.log([{"msg": "m", "flag": True}])
        self.assertEqual(run_am("count", p, "--msg", "m", "--field", "flag=true", "--eq", "1")[0], 0)

    def test_absent(self):
        p = self.log([users()])
        self.assertEqual(run_am("absent", p, "--msg", "Change Summary")[0], 1)
        self.assertEqual(run_am("absent", p, "--msg", "nothing like this")[0], 0)

    # text-format and non-JSON lines
    def test_logrus_text_fatal_line(self):
        p = self.log([
            'time="2026-01-01T00:00:00Z" level=fatal msg="cannot load attribute map file: '
            'unsupported target field \\"Name.Givenname\\""',
        ])
        self.assertEqual(run_am("count", p, "--msg", "cannot load attribute map file", "--min", "1")[0], 0)
        self.assertEqual(run_am("count", p, "--msg", 'unsupported target field "Name.Givenname"',
                             "--min", "1")[0], 0)

    def test_raw_non_json_line_is_matchable(self):
        p = self.log(["START RequestId: abc Version: $LATEST"])
        self.assertEqual(run_am("count", p, "--msg", "START RequestId", "--eq", "1")[0], 0)

    def test_blank_lines_ignored(self):
        p = self.log(["", users(create=1), ""])
        self.assertEqual(run_am("summary", p, "--users", "create=1")[0], 0)

    # same
    def test_same_ignores_time_level_and_order(self):
        a = self.log([
            {"msg": "DRY RUN: Would update user", "user": "a", "time": "1", "level": "info"},
            {"msg": "DRY RUN: Would update user", "user": "b", "time": "2", "level": "info"},
        ])
        b = self.log([
            {"msg": "DRY RUN: Would update user", "user": "b", "time": "9", "level": "info"},
            {"msg": "DRY RUN: Would update user", "user": "a", "time": "8", "level": "info"},
        ])
        self.assertEqual(run_am("same", a, b)[0], 0)

    def test_same_detects_difference(self):
        a = self.log([{"msg": "DRY RUN: Would update user", "user": "a"}])
        b = self.log([{"msg": "DRY RUN: Would update user", "user": "c"}])
        rc, out = run_am("same", a, b)
        self.assertEqual(rc, 1)
        self.assertIn("only in run 1", out)

    def test_same_detects_multiplicity(self):
        line = {"msg": "DRY RUN: Would update user", "user": "a"}
        a = self.log([line])
        b = self.log([line, line])
        rc, out = run_am("same", a, b)
        self.assertEqual(rc, 1)
        self.assertIn("multiplicity", out)

    def test_same_fails_when_no_comparable_lines(self):
        a = self.log([{"msg": "unrelated"}])
        self.assertEqual(run_am("same", a, a)[0], 1)

    # dryrun
    MARKER = {"msg": "This is a DRY RUN - actions will *not* be actually performed"}

    def test_dryrun_summary_matches_actions(self):
        p = self.log([
            self.MARKER,
            users(update=2, adoptLocal=1, create=1, recreate=1, delete=1),
            groups(update=1),
            {"msg": "Change Summary: Group Members", "add": 1, "remove": 1},
            *[{"msg": "DRY RUN: Would update user", "user": f"u{i}"} for i in range(3)],
            {"msg": "DRY RUN: Would create user", "user": "c1"},
            {"msg": "DRY RUN: Would create user", "user": "r1"},
            {"msg": "DRY RUN: Would delete user", "user": "d1"},
            {"msg": "DRY RUN: Would delete user", "user": "r1"},
            {"msg": "DRY RUN: Would update group", "group": "g1"},
            {"msg": "DRY RUN: Would add user to group", "user": "a", "group": "g"},
            {"msg": "DRY RUN: Would add user to group", "user": "b", "group": "new"},
            {"msg": "DRY RUN: Would remove user from group", "user": "c", "group": "g"},
        ])
        rc, out = run_am("dryrun", p)
        self.assertEqual(rc, 0, out)

    def test_dryrun_no_changes(self):
        p = self.log([self.MARKER, users(equal=4), groups(equal=2)])
        self.assertEqual(run_am("dryrun", p)[0], 0)

    def test_dryrun_fails_when_actions_missing(self):
        p = self.log([self.MARKER, users(update=2), groups(),
                      {"msg": "DRY RUN: Would update user", "user": "u1"}])
        rc, out = run_am("dryrun", p)
        self.assertEqual(rc, 1)
        self.assertIn("Would update user", out)

    def test_dryrun_fails_on_unreported_action(self):
        p = self.log([self.MARKER, users(), groups(),
                      {"msg": "DRY RUN: Would delete user", "user": "x"}])
        self.assertEqual(run_am("dryrun", p)[0], 1)

    def test_dryrun_member_removals_exact_additions_at_least(self):
        base = [self.MARKER, users(), groups(),
                {"msg": "Change Summary: Group Members", "add": 2, "remove": 1}]
        ok = self.log(base + [
            {"msg": "DRY RUN: Would remove user from group", "user": "a", "group": "g"},
            *[{"msg": "DRY RUN: Would add user to group", "user": f"u{i}", "group": "g"} for i in range(3)],
        ])
        self.assertEqual(run_am("dryrun", ok)[0], 0)
        too_few_adds = self.log(base + [
            {"msg": "DRY RUN: Would remove user from group", "user": "a", "group": "g"},
            {"msg": "DRY RUN: Would add user to group", "user": "u", "group": "g"},
        ])
        self.assertEqual(run_am("dryrun", too_few_adds)[0], 1)
        extra_remove = self.log(base + [
            {"msg": "DRY RUN: Would remove user from group", "user": "a", "group": "g"},
            {"msg": "DRY RUN: Would remove user from group", "user": "b", "group": "g"},
            *[{"msg": "DRY RUN: Would add user to group", "user": f"u{i}", "group": "g"} for i in range(2)],
        ])
        self.assertEqual(run_am("dryrun", extra_remove)[0], 1)

    def test_dryrun_ignores_identity_store_lookalikes(self):
        p = self.log([self.MARKER, users(), groups(),
                      {"msg": "DRY RUN: Would create group", "displayName": "x"},
                      {"msg": "DRY RUN: Would delete user", "userId": "u-1"}])
        self.assertEqual(run_am("dryrun", p)[0], 0)

    def test_dryrun_requires_dry_run_marker(self):
        p = self.log([users(), groups()])
        rc, out = run_am("dryrun", p)
        self.assertEqual(rc, 1)
        self.assertIn("not a dry-run log", out)

    def test_same_ignores_unrelated_lines(self):
        a = self.log([users(), {"msg": "noise 1"}])
        b = self.log([users(), {"msg": "noise 2"}])
        self.assertEqual(run_am("same", a, b)[0], 0)


if __name__ == "__main__":
    unittest.main()
