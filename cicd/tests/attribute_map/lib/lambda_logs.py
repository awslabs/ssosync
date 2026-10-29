#!/usr/bin/env python3
"""
Collect the CloudWatch log lines an ssosync Lambda invocation wrote, so the
attribute-map tests can assert on them with assert_log.py.

With --request-id (from lambda_invoke.py) only that invocation's lines are
kept: from its "START RequestId: <id>" line to its "REPORT RequestId: <id>"
line. The function has a reserved concurrency of 1, so every line in between
belongs to it. This matters when a test invokes twice in a row: the --start
window also covers the end of the previous invocation, whose Change Summary
would otherwise be read as this one's. Polls until that REPORT line appears.

Without --request-id every line since --start is kept, and polling stops at
the first "Change Summary: Users" (or fatal) line.

Uses the CodeBuild service role's credentials and region.

Usage:
    python3 lambda_logs.py --log-group /aws/lambda/SSOSyncFunction --start 1700000000000 \
        [--request-id <id>] --out lambda.log
"""

import argparse
import sys
import time

DONE_MARKERS = ('"Change Summary: Users"', '"level":"fatal"')


def fetch(client, log_group, start):
    messages = []
    kwargs = {"logGroupName": log_group, "startTime": start}
    while True:
        resp = client.filter_log_events(**kwargs)
        messages.extend(e["message"].rstrip("\n") for e in resp.get("events", []))
        token = resp.get("nextToken")
        if not token:
            return messages
        kwargs["nextToken"] = token


def select_invocation(messages, request_id):
    """Return (lines of the invocation request_id, whether its REPORT line was seen)."""
    start_marker = f"START RequestId: {request_id}"
    report_marker = f"REPORT RequestId: {request_id}"
    out, inside = [], False
    for line in messages:
        if not inside and line.startswith(start_marker):
            inside = True
        if inside:
            out.append(line)
            if line.startswith(report_marker):
                return out, True
    return out, False


def done_without_request_id(messages):
    return any(m in line for line in messages for m in DONE_MARKERS)


def main():
    p = argparse.ArgumentParser(description="Collect ssosync Lambda log lines from CloudWatch.")
    p.add_argument("--log-group", required=True)
    p.add_argument("--start", type=int, required=True, help="epoch milliseconds")
    p.add_argument("--request-id", help="keep only this invocation's lines (from lambda_invoke.py)")
    p.add_argument("--out", required=True)
    p.add_argument("--timeout", type=int, default=180, help="seconds to wait for the logs")
    args = p.parse_args()

    import boto3

    client = boto3.client("logs")
    deadline = time.time() + args.timeout
    messages = []
    while True:
        try:
            messages = fetch(client, args.log_group, args.start)
        except client.exceptions.ResourceNotFoundException:
            messages = []
        if args.request_id:
            selected, complete = select_invocation(messages, args.request_id)
        else:
            selected, complete = messages, done_without_request_id(messages)
        if complete:
            break
        if time.time() > deadline:
            what = f"REPORT RequestId: {args.request_id}" if args.request_id else "'Change Summary: Users'"
            print(f"WARNING: no {what} line within {args.timeout}s; "
                  f"writing the {len(selected)} line(s) collected")
            break
        time.sleep(10)

    with open(args.out, "w") as f:
        for line in selected:
            f.write(line + "\n")
    print(f"Wrote {len(selected)} log line(s) to {args.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
