#!/usr/bin/env python3
"""
Local unit tests for assert_log.py (no AWS or Google access needed).

Run from anywhere:
  python3 cicd/tests/attribute_map/lib/test_assert_log.py
"""

import json
import os
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "assert_log.py")


def write_log(lines):
    f = tempfile.NamedTemporaryFile("w", suffix=".log", delete=False)
    for line in lines:
        f.write(line if isinstance(line, str) else json.dumps(line))
        f.write("\n")
    f.close()
    return f.name


def run(*args):
    p = subprocess.run([sys.executable, SCRIPT, *args], capture_output=True, text=True)
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
        self.assertEqual(run("summary", p, "--users", "create=2")[0], 0)
        self.assertEqual(run("summary", p, "--users", "create=3")[0], 1)

    def test_summary_at_least(self):
        p = self.log([users(adoptLocal=4)])
        self.assertEqual(run("summary", p, "--users", "adoptLocal>=4")[0], 0)
        self.assertEqual(run("summary", p, "--users", "adoptLocal>=5")[0], 1)

    def test_summary_missing_line_fails(self):
        p = self.log([{"msg": "something else"}])
        rc, out = run("summary", p, "--users", "create=0")
        self.assertEqual(rc, 1)
        self.assertIn("no 'Change Summary: Users' line", out)

    def test_summary_uses_last_line(self):
        p = self.log([users(create=5), users(create=0)])
        self.assertEqual(run("summary", p, "--users", "create=0")[0], 0)

    def test_summary_members_are_summed(self):
        p = self.log([
            {"msg": "Change Summary: Group Members", "add": 1, "remove": 0},
            {"msg": "Change Summary: Group Members", "add": 2, "remove": 1},
        ])
        self.assertEqual(run("summary", p, "--members", "add=3", "remove=1")[0], 0)
        self.assertEqual(run("summary", p, "--members", "add=1")[0], 1)

    # converged
    def test_converged_pass(self):
        p = self.log([users(equal=4), groups(equal=1),
                      {"msg": "Change Summary: Group Members", "add": 0, "remove": 0}])
        self.assertEqual(run("converged", p)[0], 0)

    def test_converged_fails_on_user_change(self):
        p = self.log([users(update=1), groups()])
        self.assertEqual(run("converged", p)[0], 1)

    def test_converged_fails_on_member_change(self):
        p = self.log([users(), groups(),
                      {"msg": "Change Summary: Group Members", "add": 1, "remove": 0}])
        self.assertEqual(run("converged", p)[0], 1)

    def test_converged_fails_on_dry_run_action(self):
        p = self.log([users(), groups(), {"msg": "DRY RUN: Would update user", "user": "a"}])
        rc, out = run("converged", p)
        self.assertEqual(rc, 1)
        self.assertIn("proposed", out)

    # count / absent
    def test_count_min_eq_and_fields(self):
        p = self.log([
            {"msg": "attribute map: entry resolved", "source": "a", "resolved": "x"},
            {"msg": "attribute map: entry resolved", "source": "a", "resolved": "y"},
        ])
        base = ["count", p, "--msg", "entry resolved"]
        self.assertEqual(run(*base, "--eq", "2")[0], 0)
        self.assertEqual(run(*base, "--eq", "1")[0], 1)
        self.assertEqual(run(*base, "--min", "3")[0], 1)
        self.assertEqual(run(*base, "--field", "resolved=x", "--eq", "1")[0], 0)
        self.assertEqual(run(*base, "--field", "resolved=z", "--min", "1")[0], 1)

    def test_count_matches_empty_field_value(self):
        p = self.log([{"msg": "entry resolved", "resolved": "", "reason": "noPrimaryEntry"}])
        self.assertEqual(run("count", p, "--msg", "entry resolved", "--field", "resolved=",
                             "--field", "reason=noPrimaryEntry", "--min", "1")[0], 0)

    def test_count_missing_field_does_not_match_empty(self):
        p = self.log([{"msg": "entry resolved"}])
        self.assertEqual(run("count", p, "--msg", "entry resolved", "--field", "resolved=",
                             "--min", "1")[0], 1)

    def test_count_bool_field(self):
        p = self.log([{"msg": "m", "flag": True}])
        self.assertEqual(run("count", p, "--msg", "m", "--field", "flag=true", "--eq", "1")[0], 0)

    def test_absent(self):
        p = self.log([users()])
        self.assertEqual(run("absent", p, "--msg", "Change Summary")[0], 1)
        self.assertEqual(run("absent", p, "--msg", "nothing like this")[0], 0)

    # text-format and non-JSON lines
    def test_logrus_text_fatal_line(self):
        p = self.log([
            'time="2026-01-01T00:00:00Z" level=fatal msg="cannot load attribute map file: '
            'unsupported target field \\"Name.Givenname\\""',
        ])
        self.assertEqual(run("count", p, "--msg", "cannot load attribute map file", "--min", "1")[0], 0)
        self.assertEqual(run("count", p, "--msg", 'unsupported target field "Name.Givenname"',
                             "--min", "1")[0], 0)

    def test_raw_non_json_line_is_matchable(self):
        p = self.log(["START RequestId: abc Version: $LATEST"])
        self.assertEqual(run("count", p, "--msg", "START RequestId", "--eq", "1")[0], 0)

    def test_blank_lines_ignored(self):
        p = self.log(["", users(create=1), ""])
        self.assertEqual(run("summary", p, "--users", "create=1")[0], 0)

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
        self.assertEqual(run("same", a, b)[0], 0)

    def test_same_detects_difference(self):
        a = self.log([{"msg": "DRY RUN: Would update user", "user": "a"}])
        b = self.log([{"msg": "DRY RUN: Would update user", "user": "c"}])
        rc, out = run("same", a, b)
        self.assertEqual(rc, 1)
        self.assertIn("only in run 1", out)

    def test_same_detects_multiplicity(self):
        line = {"msg": "DRY RUN: Would update user", "user": "a"}
        a = self.log([line])
        b = self.log([line, line])
        rc, out = run("same", a, b)
        self.assertEqual(rc, 1)
        self.assertIn("multiplicity", out)

    def test_same_fails_when_no_comparable_lines(self):
        a = self.log([{"msg": "unrelated"}])
        self.assertEqual(run("same", a, a)[0], 1)

    def test_same_ignores_unrelated_lines(self):
        a = self.log([users(), {"msg": "noise 1"}])
        b = self.log([users(), {"msg": "noise 2"}])
        self.assertEqual(run("same", a, b)[0], 0)


if __name__ == "__main__":
    unittest.main()
