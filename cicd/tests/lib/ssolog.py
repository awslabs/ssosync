#!/usr/bin/env python3
"""
Assertions over ssosync's log output, shared by the live test suites:
  tests/user_group_operations/check_log.py  (issue #383; lib.sh `check`)
  tests/attribute_map/lib/assert_log.py     (issue #392; common.sh `assert_log`)
Both are thin front ends to this module and accept every subcommand below.

ssosync is run with --log-format json, so each line is a JSON object with "msg"
plus fields. A logrus text-format line (for example a fatal error emitted
before the JSON formatter is configured) is parsed into the same shape, with
msg unescaped. Any other line (e.g. Lambda runtime lines) is kept as
{"msg": <raw line>} so it can still be matched.

Subcommands (exit 0 = pass, 1 = fail, with the reason printed):

  summary   LOG [--users k=v ...] [--groups k=v ...] [--members k=v ...]
      Check fields of the last "Change Summary: Users" / "Change Summary:
      Groups" lines; --users/--groups also accept k>=v (at least). --members
      checks the totals summed over every "Change Summary: Group Members" line.

  counts    LOG [--users k=v ...] [--groups k=v ...]
      Exact: every user/group action counter not listed must be 0 (the
      "equal" counter is not an action and is not checked).

  converged LOG
      The run proposed no change: every user/group action counter is 0, no
      group member is added or removed, and no "DRY RUN: Would ..." line.

  count     LOG --msg TEXT [--field k=v ...] (--eq N | --min N)
      Count lines whose msg contains TEXT and whose fields equal every k=v.

  absent    LOG --msg TEXT [--field k=v ...]
      Shorthand for count --eq 0.

  field     LOG --msg MSG --key KEY
      Print KEY from the last line whose msg == MSG.

  same      LOG1 LOG2
      Determinism: both runs logged the same change summaries, the same
      "DRY RUN: Would ..." actions and the same attribute-map resolutions
      (compared order-independently, ignoring timestamps).

  dryrun    LOG
      The log is from a --dry-run, and its Change Summary reports exactly the
      changes that WOULD have been made live: every summary count is matched by
      the corresponding "DRY RUN: Would ..." action lines (see DRYRUN_ACTIONS).

  issuer    LOG --expected ARN [--min-users N] [--min-groups N]
      Every "ExternalId issuer expected" line carries ARN, and every
      "ExternalId issuer check" line matched it (issuerMatch=true, ssosync's
      case-insensitive rule; case-only differences are reported).

  pick      LOG --kind user|group
      Print "awsId<TAB>name<TAB>externalId" of a managed record (issuer and
      ExternalId both matched), choosing the first by name for repeatability.

  record    LOG --kind KIND --name NAME [--field k=v ...]
      The issuer-check line for that record has the given field values.

  find      LOG --kind KIND --name NAME
      Print "awsId<TAB>externalId<TAB>externalIdMatch" of the record named NAME
      (case-insensitive) from its issuer-check line.

  truth     LOG --expected ARN --users USERS.json --groups GROUPS.json
      Cross-check against `aws identitystore list-users/list-groups` output:
      every record ssosync matched carries an ExternalId with that Issuer
      (ignoring letter case, as ssosync does) and Id. Records with more than one
      ExternalId are reported, and ssosync must have selected this tenant's
      entry for them.
"""

import argparse
import json
import re
import sys

ISSUER_CHECK = "ExternalId issuer check"
ISSUER_EXPECTED = "ExternalId issuer expected"
# Action counters logged in "Change Summary: Users" / "Change Summary: Groups".
ACTION_KEYS = ["create", "update", "updateForced", "delete", "recreate",
               "adoptLocal", "adoptForeign", "retain"]

# logrus text format: key=value or key="quoted value" with \" escapes.
LOGFMT_FIELD = re.compile(r'(\w+)=("(?:[^"\\]|\\.)*"|\S*)')

failures = []


def fail(message):
    failures.append(message)
    print(f"FAIL: {message}")


def ok(message):
    print(f"ok:   {message}")


# --- loading and parsing ------------------------------------------------------

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


def as_str(v):
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, (dict, list)):
        # JSON, so `field` output can be read by jq (e.g. "Cache Stats").
        return json.dumps(v, sort_keys=True)
    return str(v)


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


def last(records, msg):
    hits = [r for r in records if r.get("msg") == msg]
    return hits[-1] if hits else None


def issuer_lines(records, kind=None):
    return [r for r in records if r.get("msg") == ISSUER_CHECK
            and (kind is None or r.get("kind") == kind)]


def check_fields(label, rec, expected):
    """expected: a dict of k->v (equality) or a list of (k, op, v) tuples."""
    if rec is None:
        fail(f"no '{label}' line (did the sync run to completion?)")
        return
    checks = [(k, "=", v) for k, v in expected.items()] if isinstance(expected, dict) else expected
    for k, op, v in checks:
        actual = as_str(rec.get(k, "<missing>"))
        if op == ">=":
            try:
                passed = int(actual) >= int(v)
            except ValueError:
                passed = False
        else:
            passed = actual == v
        if passed:
            ok(f"{label}: {k}={actual}")
        else:
            fail(f"{label}: {k}={actual}, expected {op}{v}" if op == ">=" else f"{label}: {k}={actual}, expected {v}")


def member_totals(records):
    totals = {"add": 0, "remove": 0}
    for r in records:
        if r.get("msg") == "Change Summary: Group Members":
            for k in totals:
                totals[k] += int(r.get(k, 0) or 0)
    return totals


# --- change summaries ---------------------------------------------------------

def cmd_summary(a):
    records = load(a.log)
    if a.users:
        check_fields("Change Summary: Users", last(records, "Change Summary: Users"), parse_checks(a.users))
    if a.groups:
        check_fields("Change Summary: Groups", last(records, "Change Summary: Groups"), parse_checks(a.groups))
    if a.members:
        totals = member_totals(records)
        for k, v in parse_kv(a.members).items():
            actual = str(totals.get(k, "<missing>"))
            if actual != v:
                fail(f"Group Members (total): {k}={actual}, expected {v}")
            else:
                ok(f"Group Members (total): {k}={actual}")


def exact(pairs):
    want = {k: "0" for k in ACTION_KEYS}
    for k, v in parse_kv(pairs).items():
        if k not in ACTION_KEYS:
            sys.exit(f"unknown action counter {k!r} (valid: {', '.join(ACTION_KEYS)})")
        want[k] = v
    return want


def cmd_counts(a):
    records = load(a.log)
    check_fields("Change Summary: Users", last(records, "Change Summary: Users"), exact(a.users))
    check_fields("Change Summary: Groups", last(records, "Change Summary: Groups"), exact(a.groups))


def cmd_converged(a):
    records = load(a.log)
    zero = {k: "0" for k in ACTION_KEYS}
    check_fields("Change Summary: Users", last(records, "Change Summary: Users"), zero)
    check_fields("Change Summary: Groups", last(records, "Change Summary: Groups"), zero)
    for k, v in member_totals(records).items():
        if v != 0:
            fail(f"Group Members (total): {k}={v}, expected 0")
        else:
            ok(f"Group Members (total): {k}=0")
    would = [r for r in records if str(r.get("msg", "")).startswith("DRY RUN: Would")]
    for r in would:
        print(f"      proposed: {json.dumps(r, sort_keys=True)}")
    if would:
        fail(f"{len(would)} 'DRY RUN: Would ...' action(s) proposed, expected none")
    else:
        ok("no 'DRY RUN: Would ...' actions proposed")


# --- line matching ------------------------------------------------------------

def matches(rec, text, fields):
    if text not in str(rec.get("msg", "")):
        return False
    return all(as_str(rec.get(k, "<missing>")) == v for k, v in fields.items())


def cmd_count(a, eq=None):
    records = load(a.log)
    fields = parse_kv(a.field)
    n = sum(1 for r in records if matches(r, a.msg, fields))
    want_eq = eq if eq is not None else a.eq
    desc = f"msg~{a.msg!r} {fields}" if fields else f"msg~{a.msg!r}"
    if want_eq is not None:
        if n != want_eq:
            fail(f"{desc}: count={n}, expected {want_eq}")
        else:
            ok(f"{desc}: count={n}")
    elif a.min is not None:
        if n < a.min:
            fail(f"{desc}: count={n}, expected at least {a.min}")
        else:
            ok(f"{desc}: count={n} (>= {a.min})")
    else:
        sys.exit("count needs --eq or --min")


def cmd_absent(a):
    a.eq, a.min = None, None
    cmd_count(a, eq=0)


def cmd_field(a):
    rec = last(load(a.log), a.msg)
    if rec is None or a.key not in rec:
        fail(f"no {a.key!r} on a {a.msg!r} line")
        return
    print(as_str(rec[a.key]))


# --- determinism and dry runs (attribute-map suite) ---------------------------

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


def cmd_same(a):
    one, two = comparable(load(a.log1)), comparable(load(a.log2))
    if not one:
        fail(f"{a.log1} contains no comparable lines")
    if one == two:
        ok(f"both runs produced the same {len(one)} decision line(s)")
        return
    only_one = sorted(set(one) - set(two))
    only_two = sorted(set(two) - set(one))
    for line in only_one:
        print(f"      only in run 1: {line}")
    for line in only_two:
        print(f"      only in run 2: {line}")
    if not only_one and not only_two:
        print("      same lines but different multiplicity")
    fail("the two runs made different decisions")


# In dry-run mode the Change Summary is computed exactly as in a live run, and
# Sync then calls the dry client for each change, which logs one line per
# action instead of writing. Each action line is therefore owed by these
# Change Summary counters (internal/sync.go: the update list carries Update,
# UpdateForced, AdoptLocal, AdoptForeign and Retain; Recreate is a delete plus a
# create).
DRYRUN_ACTIONS = {
    "Change Summary: Users": {
        "DRY RUN: Would create user": ["create", "recreate"],
        "DRY RUN: Would update user": ["update", "updateForced", "adoptLocal", "adoptForeign", "retain"],
        "DRY RUN: Would delete user": ["delete", "recreate"],
    },
    "Change Summary: Groups": {
        "DRY RUN: Would create group": ["create", "recreate"],
        "DRY RUN: Would update group": ["update", "updateForced", "adoptLocal", "adoptForeign", "retain"],
        "DRY RUN: Would delete group": ["delete", "recreate"],
    },
}
DRYRUN_MARKER = "This is a DRY RUN - actions will *not* be actually performed"


def cmd_dryrun(a):
    records = load(a.log)
    if not any(r.get("msg") == DRYRUN_MARKER for r in records):
        fail(f"no {DRYRUN_MARKER!r} line: not a dry-run log")

    def actions(msg):
        # Only the SCIM dry client's lines (internal/aws/client_dry.go), which
        # carry a "user" or "group" field. The dry Identity Store wrapper logs
        # look-alike messages keyed by displayName/userName/userId/groupId.
        return sum(1 for r in records
                   if r.get("msg") == msg and ("user" in r or "group" in r))

    for summary_msg, owed in DRYRUN_ACTIONS.items():
        rec = last(records, summary_msg)
        if rec is None:
            fail(f"no '{summary_msg}' line (did the sync run to completion?)")
            continue
        for action_msg, keys in owed.items():
            want = sum(int(rec.get(k, 0) or 0) for k in keys)
            got = actions(action_msg)
            label = f"{summary_msg} ({'+'.join(keys)}={want}) vs {action_msg!r}={got}"
            if got != want:
                fail(label)
            else:
                ok(label)

    # Group members: removals are only made for groups with a member summary,
    # so they match exactly; additions can also go to groups created or updated
    # in this run (which have no member summary), so they are at least the total.
    totals = member_totals(records)
    removed = actions("DRY RUN: Would remove user from group")
    added = actions("DRY RUN: Would add user to group")
    if removed != totals["remove"]:
        fail(f"Group Members remove={totals['remove']} vs 'Would remove user from group'={removed}")
    else:
        ok(f"Group Members remove={totals['remove']} vs 'Would remove user from group'={removed}")
    if added < totals["add"]:
        fail(f"Group Members add={totals['add']} vs 'Would add user to group'={added}, expected at least the summary")
    else:
        ok(f"Group Members add={totals['add']} vs 'Would add user to group'={added} (>=)")


# --- ExternalId issuer (user and group operations suite) ----------------------

def describe(r):
    keys = ("kind", "name", "externalId", "issuer", "expectedIssuer", "issuerExactMatch", "externalIdCount",
            "externalIdSelectedBy", "caseInsensitiveMatch", "accountMatch", "storeAndTenantMatch")
    return ", ".join(f"{k}={as_str(r.get(k))}" for k in keys if k in r)


def cmd_issuer(a):
    records = load(a.log)
    expected_lines = [r for r in records if r.get("msg") == ISSUER_EXPECTED]
    if not expected_lines:
        fail(f"no {ISSUER_EXPECTED!r} line (run with --log-level debug?)")
    for r in expected_lines:
        if r.get("expectedIssuer") != a.expected:
            fail(f"{r.get('kind')} operations expected issuer {r.get('expectedIssuer')!r}, want {a.expected!r}")
    if expected_lines and all(r.get("expectedIssuer") == a.expected for r in expected_lines):
        ok(f"user and group operations expect {a.expected}")

    for kind, minimum in (("user", a.min_users), ("group", a.min_groups)):
        lines = issuer_lines(records, kind)
        if len(lines) < minimum:
            fail(f"{len(lines)} {kind} issuer-check line(s), expected at least {minimum}")
        bad = [r for r in lines if r.get("issuerMatch") is not True]
        for r in bad:
            fail(f"{kind} issuer did not match: {describe(r)}")
        if lines and not bad:
            # issuerMatch is ssosync's case-insensitive rule; report how many
            # matched only ignoring case (IAM Identity Center's casing of the
            # tenant id varies between records of one tenant, issue #383).
            case_only = sum(1 for r in lines if r.get("issuerExactMatch") is False)
            ok(f"all {len(lines)} {kind} issuer-check line(s) matched the expected issuer"
               + (f" ({case_only} only ignoring letter case)" if case_only else " exactly"))


def managed(records, kind):
    lines = [r for r in issuer_lines(records, kind)
             if r.get("issuerMatch") is True and r.get("externalIdMatch") is True]
    return sorted(lines, key=lambda r: str(r.get("name", "")).lower())


def cmd_pick(a):
    lines = managed(load(a.log), a.kind)
    if not lines:
        fail(f"no managed {a.kind} (issuerMatch and externalIdMatch) to pick")
        return
    r = lines[0]
    print(f"{r['awsId']}\t{r['name']}\t{r['externalId']}")


def cmd_record(a):
    want = parse_kv(a.field)
    lines = [r for r in issuer_lines(load(a.log), a.kind) if r.get("name") == a.name]
    if not lines:
        fail(f"no issuer-check line for {a.kind} {a.name!r}")
        return
    r = lines[-1]
    for k, v in want.items():
        actual = as_str(r.get(k, "<missing>"))
        if actual == v:
            ok(f"{a.kind} {a.name}: {k}={actual}")
        else:
            fail(f"{a.kind} {a.name}: {k}={actual}, expected {v}")


def cmd_find(a):
    lines = [r for r in issuer_lines(load(a.log), a.kind)
             if str(r.get("name", "")).lower() == a.name.lower()]
    if not lines:
        fail(f"no issuer-check line for {a.kind} {a.name!r}: is it in scope and provisioned by ssosync?")
        return
    r = lines[-1]
    print(f"{r.get('awsId', '')}\t{r.get('externalId', '')}\t{as_str(r.get('externalIdMatch'))}")


def cmd_truth(a):
    records = load(a.log)
    with open(a.users) as f:
        users = {u["UserId"]: u for u in json.load(f).get("Users", [])}
    with open(a.groups) as f:
        groups = {g["GroupId"]: g for g in json.load(f).get("Groups", [])}

    multi = 0
    case_only = {}
    for kind, store in (("user", users), ("group", groups)):
        lines = issuer_lines(records, kind)
        checked = 0
        for r in lines:
            if r.get("issuerMatch") is not True:
                continue  # reported by the issuer subcommand
            rec = store.get(r.get("awsId"))
            if rec is None:
                fail(f"{kind} {r.get('name')!r} ({r.get('awsId')}) not found in the Identity Store listing")
                continue
            ext = rec.get("ExternalIds") or []
            if len(ext) > 1:
                multi += 1
                print(f"      {kind} {r.get('name')} has {len(ext)} ExternalIds: {json.dumps(ext)}")
                if r.get("externalIdSelectedBy") != "issuerMatch":
                    fail(f"{kind} {r.get('name')}: has {len(ext)} ExternalIds but ssosync selected by "
                         f"{r.get('externalIdSelectedBy')!r}, expected 'issuerMatch'")
            # Same rule as ssosync (issuerMatches): case-insensitive, because
            # IAM Identity Center stamps the tenant id with its original case on
            # some records and lowercased on others, even among users of one
            # tenant (issue #383). The spelling actually stored is reported.
            hit = [e for e in ext if str(e.get("Issuer", "")).lower() == a.expected.lower()
                   and e.get("Id") == r.get("externalId")]
            if hit:
                checked += 1
                if hit[0].get("Issuer") != a.expected:
                    case_only[kind] = case_only.get(kind, 0) + 1
            else:
                fail(f"{kind} {r.get('name')}: no ExternalId with Issuer == expected (ignoring case) and Id == "
                     f"{r.get('externalId')!r} in the Identity Store: {json.dumps(ext)}")
        if checked:
            ok(f"{checked} {kind}(s): the Identity Store Issuer matches the expected IssuerArn "
               f"({checked - case_only.get(kind, 0)} exactly, {case_only.get(kind, 0)} differing only in letter case)")
    if multi:
        ok(f"{multi} record(s) with more than one ExternalId were matched on this tenant's entry")
    else:
        print("      no record with more than one ExternalId in scope (multi-ExternalId selection not exercised live)")


# --- command line -------------------------------------------------------------

def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)
    kinds = ["user", "group"]

    s = sub.add_parser("summary"); s.add_argument("log")
    s.add_argument("--users", nargs="*"); s.add_argument("--groups", nargs="*"); s.add_argument("--members", nargs="*")
    s = sub.add_parser("counts"); s.add_argument("log"); s.add_argument("--users", nargs="*"); s.add_argument("--groups", nargs="*")
    s = sub.add_parser("converged"); s.add_argument("log")
    for name in ("count", "absent"):
        s = sub.add_parser(name); s.add_argument("log")
        s.add_argument("--msg", required=True); s.add_argument("--field", action="append")
        if name == "count":
            g = s.add_mutually_exclusive_group(required=True)
            g.add_argument("--eq", type=int); g.add_argument("--min", type=int)
    s = sub.add_parser("field"); s.add_argument("log"); s.add_argument("--msg", required=True); s.add_argument("--key", required=True)
    s = sub.add_parser("same"); s.add_argument("log1"); s.add_argument("log2")
    s = sub.add_parser("dryrun"); s.add_argument("log")
    s = sub.add_parser("issuer"); s.add_argument("log"); s.add_argument("--expected", required=True)
    s.add_argument("--min-users", type=int, default=1); s.add_argument("--min-groups", type=int, default=1)
    s = sub.add_parser("pick"); s.add_argument("log"); s.add_argument("--kind", choices=kinds, required=True)
    s = sub.add_parser("record"); s.add_argument("log"); s.add_argument("--kind", choices=kinds, required=True)
    s.add_argument("--name", required=True); s.add_argument("--field", action="append")
    s = sub.add_parser("find"); s.add_argument("log"); s.add_argument("--kind", choices=kinds, required=True)
    s.add_argument("--name", required=True)
    s = sub.add_parser("truth"); s.add_argument("log"); s.add_argument("--expected", required=True)
    s.add_argument("--users", required=True); s.add_argument("--groups", required=True)

    a = p.parse_args(argv)
    {"summary": cmd_summary, "counts": cmd_counts, "converged": cmd_converged, "count": cmd_count,
     "absent": cmd_absent, "field": cmd_field, "same": cmd_same, "dryrun": cmd_dryrun,
     "issuer": cmd_issuer, "pick": cmd_pick, "record": cmd_record, "find": cmd_find,
     "truth": cmd_truth}[a.cmd](a)
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
