#!/usr/bin/env python3
"""
Invoke an ssosync Lambda function synchronously and record the invocation's
request id, so its CloudWatch log lines can be told apart from those of the
previous invocation (lambda_logs.py --request-id).

Writes the function's response to --response and the invocation result to
--out as JSON: {"StatusCode": ..., "RequestId": ..., "FunctionError": ...}
(FunctionError only when the function failed, like `aws lambda invoke`).

Retries are disabled: a retried invoke would run a second sync.

Usage:
    python3 lambda_invoke.py --function-name SSOSyncFunction --response response.json --out invoke.json
"""

import argparse
import json
import sys


def main():
    p = argparse.ArgumentParser(description="Invoke an ssosync Lambda function and record its request id.")
    p.add_argument("--function-name", required=True)
    p.add_argument("--response", required=True)
    p.add_argument("--out", required=True)
    args = p.parse_args()

    import boto3
    from botocore.config import Config

    client = boto3.client("lambda", config=Config(read_timeout=900, connect_timeout=30,
                                                  retries={"max_attempts": 1, "mode": "standard"}))
    resp = client.invoke(FunctionName=args.function_name, InvocationType="RequestResponse")
    with open(args.response, "wb") as f:
        f.write(resp["Payload"].read())
    result = {"StatusCode": resp["StatusCode"], "RequestId": resp["ResponseMetadata"]["RequestId"]}
    if resp.get("FunctionError"):
        result["FunctionError"] = resp["FunctionError"]
    with open(args.out, "w") as f:
        json.dump(result, f)
    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    sys.exit(main())
