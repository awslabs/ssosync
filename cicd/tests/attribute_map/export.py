#!/usr/bin/env python3
"""
Export users and groups from an AWS IAM Identity Center Identity Store to JSON.

CI/CD variant of scripts/export.py for the attribute-map integration tests.
It runs inside AWS CodeBuild and uses the CodeBuild service role's credentials
(the default boto3 credential chain), so there is no --profile option.

Usage:
    python3 export.py \
        --output-dir ./snapshot \
        [--identity-store-id d-1234567890] \
        [--region us-east-1] \
        [--all-fields]

The Identity Store ID is resolved in this order:
    1. --identity-store-id
    2. the IDENTITY_STORE_ID environment variable
    3. auto-discovery via sso-admin ListInstances (requires sso:ListInstances)

The region defaults to AWS_REGION / AWS_DEFAULT_REGION, which CodeBuild sets.

By default, exports only the fields required by import.py.
Use --all-fields to export the full API response.
"""

import argparse
import json
import os
import sys

import boto3
from botocore.exceptions import ClientError


# ---------------------------------------------------------------------------
# Session / Identity Store resolution
# ---------------------------------------------------------------------------

def resolve_region(region: str = None) -> str:
    """Return the explicit region, else the region CodeBuild provides."""
    return region or os.environ.get("AWS_REGION") or os.environ.get("AWS_DEFAULT_REGION")


def resolve_identity_store_id(session, identity_store_id: str = None) -> str:
    """Return the Identity Store ID from the argument, the environment, or ListInstances."""
    if identity_store_id:
        return identity_store_id
    env_id = os.environ.get("IDENTITY_STORE_ID")
    if env_id:
        return env_id
    instances = session.client("sso-admin").list_instances().get("Instances", [])
    if len(instances) != 1:
        print(f"ERROR: expected exactly one IAM Identity Center instance, found {len(instances)}. "
              "Pass --identity-store-id or set IDENTITY_STORE_ID.")
        sys.exit(1)
    return instances[0]["IdentityStoreId"]


# ---------------------------------------------------------------------------
# Users
# ---------------------------------------------------------------------------

def list_all_users(client, identity_store_id: str) -> list:
    """List all users in the Identity Store."""
    users = []
    paginator = client.get_paginator("list_users")
    for page in paginator.paginate(IdentityStoreId=identity_store_id):
        users.extend(page.get("Users", []))
    return users


def format_user_minimal(user: dict) -> dict:
    """Extract only the fields needed by import.py."""
    emails = []
    for email in user.get("Emails", []):
        emails.append({
            "Value": email.get("Value", ""),
            "Type": email.get("Type", "work"),
            "Primary": email.get("Primary", False),
        })

    name = user.get("Name", {})
    external_ids = []
    for eid in user.get("ExternalIds", []):
        external_ids.append({
            "Issuer": eid.get("Issuer", ""),
            "Id": eid.get("Id", ""),
        })

    addresses = []
    for address in user.get("Addresses", []):
        addresses.append({
            "StreetAddress": address.get("StreetAddress", ""),
            "Locality": address.get("Locality", ""),
            "Region": address.get("Region", ""),
            "PostalCode": address.get("PostalCode", ""),
            "Country": address.get("Country", ""),
            "Formatted": address.get("Formatted", ""),
            "Type": address.get("Type", ""),
            "Primary": address.get("Primary", False),
        })

    return {
        "UserName": user.get("UserName", ""),
        "DisplayName": user.get("DisplayName", ""),
        "GivenName": name.get("GivenName", ""),
        "FamilyName": name.get("FamilyName", ""),
        "Active": user.get("Active", True),
        "ExternalIds": external_ids,
        "Emails": emails,
        "Addresses": addresses,
    }


# ---------------------------------------------------------------------------
# Groups
# ---------------------------------------------------------------------------

def list_all_groups(client, identity_store_id: str) -> list:
    """List all groups in the Identity Store."""
    groups = []
    paginator = client.get_paginator("list_groups")
    for page in paginator.paginate(IdentityStoreId=identity_store_id):
        groups.extend(page.get("Groups", []))
    return groups


def list_group_memberships(client, identity_store_id: str, group_id: str) -> list:
    """List all memberships for a group."""
    memberships = []
    paginator = client.get_paginator("list_group_memberships")
    for page in paginator.paginate(IdentityStoreId=identity_store_id, GroupId=group_id):
        memberships.extend(page.get("GroupMemberships", []))
    return memberships


def resolve_user_id_to_username(users: list) -> dict:
    """Build a mapping of UserId -> UserName from the users list."""
    return {user["UserId"]: user.get("UserName", "") for user in users}


def format_group_minimal(group: dict, member_usernames: list) -> dict:
    """Extract only the fields needed by import.py.

    ExternalIds are included for inspection (e.g. to confirm ssosync adopted a
    group). import.py ignores them: imported groups are always local, and
    ssosync's first sync pass sets their ExternalIds.
    """
    external_ids = [
        {"Issuer": eid.get("Issuer", ""), "Id": eid.get("Id", "")}
        for eid in group.get("ExternalIds", [])
    ]
    return {
        "DisplayName": group.get("DisplayName", ""),
        "Description": group.get("Description", ""),
        "ExternalIds": external_ids,
        "Members": member_usernames,
    }


def format_group_full(group: dict, member_usernames: list) -> dict:
    """Return all group fields plus resolved member usernames."""
    result = dict(group)
    result["Members"] = member_usernames
    return result


# ---------------------------------------------------------------------------
# Export
# ---------------------------------------------------------------------------

def export(identity_store_id: str, region: str, output_dir: str, all_fields: bool):
    session = boto3.Session(region_name=resolve_region(region))
    identity_store_id = resolve_identity_store_id(session, identity_store_id)
    client = session.client("identitystore")

    print(f"Identity Store ID: {identity_store_id}")
    print(f"Region:            {session.region_name or '(default)'}")
    print()

    try:
        # --- Export users ---
        print("Fetching users...")
        raw_users = list_all_users(client, identity_store_id)
        print(f"  Found {len(raw_users)} user(s).")

        user_id_map = resolve_user_id_to_username(raw_users)

        if all_fields:
            users_output = raw_users
        else:
            users_output = [format_user_minimal(u) for u in raw_users]

        # --- Export groups ---
        print("Fetching groups...")
        raw_groups = list_all_groups(client, identity_store_id)
        print(f"  Found {len(raw_groups)} group(s).")

        groups_output = []
        for group in raw_groups:
            group_id = group["GroupId"]
            display_name = group.get("DisplayName", group_id)
            print(f"  Resolving members for group: {display_name}")

            memberships = list_group_memberships(client, identity_store_id, group_id)
            member_usernames = []
            for membership in memberships:
                member_id = membership.get("MemberId", {}).get("UserId", "")
                username = user_id_map.get(member_id, member_id)
                member_usernames.append(username)

            if all_fields:
                groups_output.append(format_group_full(group, member_usernames))
            else:
                groups_output.append(format_group_minimal(group, member_usernames))
    except ClientError as e:
        # In CI a failed export must fail the build rather than write a partial snapshot.
        print(f"ERROR: export failed: {e}")
        sys.exit(1)

    # --- Write output ---
    os.makedirs(output_dir, exist_ok=True)

    users_path = os.path.join(output_dir, "users.json")
    groups_path = os.path.join(output_dir, "groups.json")

    with open(users_path, "w") as f:
        json.dump(users_output, f, indent=4, default=str)
    print(f"\nWrote {len(users_output)} user(s) to {users_path}")

    with open(groups_path, "w") as f:
        json.dump(groups_output, f, indent=4, default=str)
    print(f"Wrote {len(groups_output)} group(s) to {groups_path}")


def main():
    parser = argparse.ArgumentParser(
        description="Export users and groups from an AWS Identity Store to JSON "
                    "(CodeBuild variant: uses the build's service role credentials)."
    )
    parser.add_argument(
        "--identity-store-id", default=None,
        help="The Identity Store ID (e.g., d-1234567890). Defaults to IDENTITY_STORE_ID, "
             "then auto-discovery via sso-admin ListInstances."
    )
    parser.add_argument(
        "--region", default=None,
        help="AWS region (defaults to AWS_REGION / AWS_DEFAULT_REGION)"
    )
    parser.add_argument(
        "--output-dir", required=True,
        help="Directory to write users.json and groups.json"
    )
    parser.add_argument(
        "--all-fields", action="store_true",
        help="Export all fields from the API response (default: only fields needed by import.py)"
    )

    args = parser.parse_args()

    print(f"Output directory:  {args.output_dir}")
    print(f"All fields:        {args.all_fields}")

    export(
        identity_store_id=args.identity_store_id,
        region=args.region,
        output_dir=args.output_dir,
        all_fields=args.all_fields,
    )


if __name__ == "__main__":
    main()
