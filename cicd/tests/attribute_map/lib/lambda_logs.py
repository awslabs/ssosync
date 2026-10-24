#!/usr/bin/env python3
"""
Collect the CloudWatch log lines an ssosync Lambda invocation wrote, so the
attribute-map tests can assert on them with assert_log.py.

Polls the function's log group from --start (epoch milliseconds) until a
"Change Summary: Users" line appears (or a fatal line, or the timeout elapses),
then writes every message, one per line, to --out. Uses the CodeBuild service
role's credentials and region.

Usage:
    python3 lambda_logs.py --log-group /aws/lambda/SSOSyncFunction --start 1700000000000 --out lambda.log
"""

import argparse
import sys
import time

import boto3

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


def main():
    p = argparse.ArgumentParser(description="Collect ssosync Lambda log lines from CloudWatch.")
    p.add_argument("--log-group", required=True)
    p.add_argument("--start", type=int, required=True, help="epoch milliseconds")
    p.add_argument("--out", required=True)
    p.add_argument("--timeout", type=int, default=180, help="seconds to wait for the logs")
    args = p.parse_args()

    client = boto3.client("logs")
    deadline = time.time() + args.timeout
    messages = []
    while True:
        try:
            messages = fetch(client, args.log_group, args.start)
        except client.exceptions.ResourceNotFoundException:
            messages = []
        if any(m in line for line in messages for m in DONE_MARKERS):
            break
        if time.time() > deadline:
            print(f"WARNING: no 'Change Summary: Users' line within {args.timeout}s; "
                  f"writing the {len(messages)} line(s) collected")
            break
        time.sleep(10)

    with open(args.out, "w") as f:
        for line in messages:
            f.write(line + "\n")
    print(f"Wrote {len(messages)} log line(s) to {args.out}")


if __name__ == "__main__":
    sys.exit(main())
