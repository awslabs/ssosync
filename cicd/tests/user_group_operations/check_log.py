#!/usr/bin/env python3
"""
Assertions over ssosync JSON debug logs for the user/group operations live test
(issue #383: the expected IssuerArn must match the ExternalId Issuer of every
record ssosync provisioned, including records with more than one ExternalId).

ssosync is run with --log-format json --log-level debug, so each line is a JSON
object with "msg" plus fields. Non-JSON lines are kept as {"msg": <line>}.

Subcommands (exit 0 = pass, 1 = fail, with the reason printed):

  field     LOG --msg MSG --key KEY
      Print KEY from the last line whose msg == MSG.

  summary   LOG [--users k=v ...] [--groups k=v ...]
      Check "Change Summary: Users/Groups" counters.

  converged LOG
      Every user and group action counter is 0 and no "DRY RUN: Would ..." line.

  issuer    LOG --expected ARN [--min-users N] [--min-groups N]
      Every "ExternalId issuer expected" line carries ARN, and every
      "ExternalId issuer check" line matched it (issuerMatch=true, ssosync's
      case-insensitive rule; case-only differences are reported).

  pick      LOG --kind user|group
      Print "awsId<TAB>name<TAB>externalId" of a managed record (issuer and
      ExternalId both matched), choosing the first by name for repeatability.

  record    LOG --kind KIND --name NAME [--field k=v ...]
      The issuer-check line for that record has the given field values.

  counts    LOG [--users k=v ...] [--groups k=v ...]
      Exact: every user/group action counter not listed must be 0 (the
      "equal" counter is not an action and is not checked).

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
import sys

ISSUER_CHECK = "ExternalId issuer check"
ISSUER_EXPECTED = "ExternalId issuer expected"
ACTION_KEYS = ["create", "update", "updateForced", "delete", "recreate",
               "adoptLocal", "adoptForeign", "retain"]

failures = []


def fail(message):
    failures.append(message)
    print(f"FAIL: {message}")


def ok(message):
    print(f"ok:   {message}")


def load(path):
    out = []
    with open(path, errors="replace") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
                if not isinstance(rec, dict):
                    rec = {"msg": line}
            except ValueError:
                rec = {"msg": line}
            out.append(rec)
    return out


def as_str(v):
    if isinstance(v, bool):
        return "true" if v else "false"
    return str(v)


def parse_kv(pairs):
    out = {}
    for p in pairs or []:
        if "=" not in p:
            sys.exit(f"bad k=v argument: {p!r}")
        k, v = p.split("=", 1)
        out[k] = v
    return out


def last(records, msg):
    hits = [r for r in records if r.get("msg") == msg]
    return hits[-1] if hits else None


def issuer_lines(records, kind=None):
    return [r for r in records if r.get("msg") == ISSUER_CHECK
            and (kind is None or r.get("kind") == kind)]


def check_counters(label, rec, expected):
    if rec is None:
        fail(f"no '{label}' line (did the sync complete?)")
        return
    for k, v in expected.items():
        actual = as_str(rec.get(k, "<missing>"))
        if actual == v:
            ok(f"{label}: {k}={actual}")
        else:
            fail(f"{label}: {k}={actual}, expected {v}")


def cmd_field(a):
    rec = last(load(a.log), a.msg)
    if rec is None or a.key not in rec:
        fail(f"no {a.key!r} on a {a.msg!r} line")
        return
    print(as_str(rec[a.key]))


def cmd_summary(a):
    records = load(a.log)
    if a.users:
        check_counters("Change Summary: Users", last(records, "Change Summary: Users"), parse_kv(a.users))
    if a.groups:
        check_counters("Change Summary: Groups", last(records, "Change Summary: Groups"), parse_kv(a.groups))


def exact(pairs):
    want = {k: "0" for k in ACTION_KEYS}
    for k, v in parse_kv(pairs).items():
        if k not in ACTION_KEYS:
            sys.exit(f"unknown action counter {k!r} (valid: {', '.join(ACTION_KEYS)})")
        want[k] = v
    return want


def cmd_counts(a):
    records = load(a.log)
    check_counters("Change Summary: Users", last(records, "Change Summary: Users"), exact(a.users))
    check_counters("Change Summary: Groups", last(records, "Change Summary: Groups"), exact(a.groups))


def cmd_find(a):
    lines = [r for r in issuer_lines(load(a.log), a.kind)
             if str(r.get("name", "")).lower() == a.name.lower()]
    if not lines:
        fail(f"no issuer-check line for {a.kind} {a.name!r}: is it in scope and provisioned by ssosync?")
        return
    r = lines[-1]
    print(f"{r.get('awsId', '')}\t{r.get('externalId', '')}\t{as_str(r.get('externalIdMatch'))}")


def cmd_converged(a):
    records = load(a.log)
    zero = {k: "0" for k in ACTION_KEYS}
    check_counters("Change Summary: Users", last(records, "Change Summary: Users"), zero)
    check_counters("Change Summary: Groups", last(records, "Change Summary: Groups"), zero)
    would = [r for r in records if str(r.get("msg", "")).startswith("DRY RUN: Would")]
    for r in would:
        print(f"      proposed: {json.dumps(r, sort_keys=True)}")
    if would:
        fail(f"{len(would)} 'DRY RUN: Would ...' action(s) proposed, expected none")
    else:
        ok("no 'DRY RUN: Would ...' actions")


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


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)

    s = sub.add_parser("field"); s.add_argument("log"); s.add_argument("--msg", required=True); s.add_argument("--key", required=True)
    s = sub.add_parser("summary"); s.add_argument("log"); s.add_argument("--users", nargs="*"); s.add_argument("--groups", nargs="*")
    s = sub.add_parser("converged"); s.add_argument("log")
    s = sub.add_parser("issuer"); s.add_argument("log"); s.add_argument("--expected", required=True)
    s.add_argument("--min-users", type=int, default=1); s.add_argument("--min-groups", type=int, default=1)
    s = sub.add_parser("pick"); s.add_argument("log"); s.add_argument("--kind", choices=["user", "group"], required=True)
    s = sub.add_parser("record"); s.add_argument("log"); s.add_argument("--kind", choices=["user", "group"], required=True)
    s.add_argument("--name", required=True); s.add_argument("--field", action="append")
    s = sub.add_parser("truth"); s.add_argument("log"); s.add_argument("--expected", required=True)
    s.add_argument("--users", required=True); s.add_argument("--groups", required=True)
    s = sub.add_parser("counts"); s.add_argument("log"); s.add_argument("--users", nargs="*"); s.add_argument("--groups", nargs="*")
    s = sub.add_parser("find"); s.add_argument("log"); s.add_argument("--kind", choices=["user", "group"], required=True)
    s.add_argument("--name", required=True)

    a = p.parse_args()
    {"field": cmd_field, "summary": cmd_summary, "converged": cmd_converged, "issuer": cmd_issuer,
     "pick": cmd_pick, "record": cmd_record, "truth": cmd_truth, "counts": cmd_counts,
     "find": cmd_find}[a.cmd](a)
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
