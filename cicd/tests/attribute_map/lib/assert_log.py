#!/usr/bin/env python3
"""
Assertions over ssosync's JSON log output, for the attribute-map integration tests.

ssosync is run with --log-format json, so each log line is a JSON object with a
"msg" key plus structured fields. A logrus text-format line (for example a fatal
error emitted before the JSON formatter is configured) is parsed into the same
shape, with msg unescaped. Any other line (e.g. Lambda runtime lines) is treated
as {"msg": <raw line>} so it can still be matched.

Usage:
  assert_log.py summary   LOG [--users k=v ...] [--groups k=v ...] [--members k=v ...]
      Check fields of the "Change Summary: Users" / "Change Summary: Groups" lines.
      --users/--groups also accept k>=v (at least).
      --members checks the totals summed over every "Change Summary: Group Members" line.

  assert_log.py converged LOG
      The run proposed no change: every user/group action count is 0, no group
      member is added or removed, and there are no "DRY RUN: Would ..." lines.

  assert_log.py count     LOG --msg TEXT [--field k=v ...] (--eq N | --min N)
      Count lines whose msg contains TEXT and whose fields equal every k=v.

  assert_log.py absent    LOG --msg TEXT [--field k=v ...]
      Shorthand for count --eq 0.

  assert_log.py same      LOG1 LOG2
      Determinism: both runs logged the same change summaries, the same
      "DRY RUN: Would ..." actions and the same attribute-map resolutions
      (compared order-independently, ignoring timestamps).

Exit status is 0 on pass and 1 on failure, with the reason printed.
"""

import argparse
import json
import re
import sys

# Action counters logged in "Change Summary: Users" / "Change Summary: Groups".
CHANGE_KEYS = ["create", "update", "updateForced", "delete", "recreate",
               "adoptLocal", "adoptForeign", "retain"]


# logrus text format: key=value or key="quoted value" with \" escapes.
LOGFMT_FIELD = re.compile(r'(\w+)=("(?:[^"\\]|\\.)*"|\S*)')


def parse_text_line(line):
    """Parse a logrus text-format line (e.g. a fatal emitted before the JSON
    formatter is set) into a dict; fall back to {"msg": line}."""
    fields = {}
    for key, value in LOGFMT_FIELD.findall(line):
        if value.startswith('"'):
            try:
                value = json.loads(value)
            except ValueError:
                value = value[1:-1]
        fields[key] = value
    if "msg" in fields:
        return fields
    return {"msg": line}


def load(path):
    records = []
    with open(path, "r", errors="replace") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
                if not isinstance(rec, dict):
                    rec = {"msg": line}
            except ValueError:
                rec = parse_text_line(line)
            records.append(rec)
    return records


def parse_kv(pairs):
    out = {}
    for p in pairs or []:
        if "=" not in p:
            sys.exit(f"bad k=v argument: {p!r}")
        k, v = p.split("=", 1)
        out[k] = v
    return out


def parse_checks(pairs):
    """Parse summary checks: 'k=v' (equal) or 'k>=v' (at least)."""
    out = []
    for p in pairs or []:
        if ">=" in p:
            k, v = p.split(">=", 1)
            out.append((k, ">=", v))
        elif "=" in p:
            k, v = p.split("=", 1)
            out.append((k, "=", v))
        else:
            sys.exit(f"bad check argument: {p!r} (use k=v or k>=v)")
    return out


def as_str(v):
    if isinstance(v, bool):
        return "true" if v else "false"
    return str(v)


def find_one(records, msg):
    hits = [r for r in records if r.get("msg") == msg]
    return hits[-1] if hits else None


failures = []


def fail(message):
    failures.append(message)
    print(f"FAIL: {message}")


def check_fields(label, rec, expected):
    """expected: a dict of k->v (equality) or a list of (k, op, v) tuples."""
    if rec is None:
        fail(f"no '{label}' line found (did the sync run to completion?)")
        return
    checks = [(k, "=", v) for k, v in expected.items()] if isinstance(expected, dict) else expected
    for k, op, v in checks:
        actual = as_str(rec.get(k, "<missing>"))
        if op == ">=":
            try:
                ok = int(actual) >= int(v)
            except ValueError:
                ok = False
        else:
            ok = actual == v
        if not ok:
            fail(f"{label}: {k}={actual}, expected {op}{v}" if op == ">=" else f"{label}: {k}={actual}, expected {v}")
        else:
            print(f"ok:   {label}: {k}={actual}")


def member_totals(records):
    totals = {"add": 0, "remove": 0}
    seen = False
    for r in records:
        if r.get("msg") == "Change Summary: Group Members":
            seen = True
            for k in totals:
                totals[k] += int(r.get(k, 0) or 0)
    return totals, seen


def cmd_summary(args):
    records = load(args.log)
    if args.users:
        check_fields("Change Summary: Users", find_one(records, "Change Summary: Users"), parse_checks(args.users))
    if args.groups:
        check_fields("Change Summary: Groups", find_one(records, "Change Summary: Groups"), parse_checks(args.groups))
    if args.members:
        totals, _ = member_totals(records)
        for k, v in parse_kv(args.members).items():
            actual = str(totals.get(k, "<missing>"))
            if actual != v:
                fail(f"Group Members (total): {k}={actual}, expected {v}")
            else:
                print(f"ok:   Group Members (total): {k}={actual}")


def cmd_converged(args):
    records = load(args.log)
    zero = {k: "0" for k in CHANGE_KEYS}
    check_fields("Change Summary: Users", find_one(records, "Change Summary: Users"), zero)
    check_fields("Change Summary: Groups", find_one(records, "Change Summary: Groups"), zero)
    totals, _ = member_totals(records)
    for k, v in totals.items():
        if v != 0:
            fail(f"Group Members (total): {k}={v}, expected 0")
        else:
            print(f"ok:   Group Members (total): {k}=0")
    would = [r for r in records if str(r.get("msg", "")).startswith("DRY RUN: Would")]
    if would:
        for r in would:
            print(f"      proposed: {json.dumps(r, sort_keys=True)}")
        fail(f"{len(would)} 'DRY RUN: Would ...' action(s) proposed, expected none")
    else:
        print("ok:   no 'DRY RUN: Would ...' actions proposed")


def matches(rec, text, fields):
    if text not in str(rec.get("msg", "")):
        return False
    return all(as_str(rec.get(k, "<missing>")) == v for k, v in fields.items())


def cmd_count(args, eq=None):
    records = load(args.log)
    fields = parse_kv(args.field)
    n = sum(1 for r in records if matches(r, args.msg, fields))
    want_eq = eq if eq is not None else args.eq
    desc = f"msg~{args.msg!r} {fields}" if fields else f"msg~{args.msg!r}"
    if want_eq is not None:
        if n != want_eq:
            fail(f"{desc}: count={n}, expected {want_eq}")
        else:
            print(f"ok:   {desc}: count={n}")
    elif args.min is not None:
        if n < args.min:
            fail(f"{desc}: count={n}, expected at least {args.min}")
        else:
            print(f"ok:   {desc}: count={n} (>= {args.min})")
    else:
        sys.exit("count needs --eq or --min")


def comparable(records):
    """Lines that describe what a run decided, normalised for comparison."""
    out = []
    for r in records:
        msg = str(r.get("msg", ""))
        if (msg.startswith("Change Summary") or msg.startswith("DRY RUN: Would")
                or msg == "attribute map: entry resolved"):
            norm = {k: v for k, v in r.items() if k not in ("time", "level")}
            out.append(json.dumps(norm, sort_keys=True))
    return sorted(out)


def cmd_same(args):
    a, b = comparable(load(args.log1)), comparable(load(args.log2))
    if not a:
        fail(f"{args.log1} contains no comparable lines")
    if a == b:
        print(f"ok:   both runs produced the same {len(a)} decision line(s)")
        return
    only_a = sorted(set(a) - set(b))
    only_b = sorted(set(b) - set(a))
    for line in only_a:
        print(f"      only in run 1: {line}")
    for line in only_b:
        print(f"      only in run 2: {line}")
    if not only_a and not only_b:
        print("      same lines but different multiplicity")
    fail("the two runs made different decisions")


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)

    s = sub.add_parser("summary")
    s.add_argument("log")
    s.add_argument("--users", nargs="*")
    s.add_argument("--groups", nargs="*")
    s.add_argument("--members", nargs="*")

    c = sub.add_parser("converged")
    c.add_argument("log")

    for name in ("count", "absent"):
        n = sub.add_parser(name)
        n.add_argument("log")
        n.add_argument("--msg", required=True)
        n.add_argument("--field", action="append")
        if name == "count":
            g = n.add_mutually_exclusive_group(required=True)
            g.add_argument("--eq", type=int)
            g.add_argument("--min", type=int)

    m = sub.add_parser("same")
    m.add_argument("log1")
    m.add_argument("log2")

    args = p.parse_args()
    if args.cmd == "summary":
        cmd_summary(args)
    elif args.cmd == "converged":
        cmd_converged(args)
    elif args.cmd == "count":
        cmd_count(args)
    elif args.cmd == "absent":
        args.eq, args.min = None, None
        cmd_count(args, eq=0)
    elif args.cmd == "same":
        cmd_same(args)

    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
