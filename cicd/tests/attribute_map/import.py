#!/usr/bin/env python3
"""
Import (replace) all users and groups in an AWS IAM Identity Center Identity Store
based on JSON input files.

CI/CD variant of scripts/import.py for the attribute-map integration tests. It
resets the target Identity Store to a known baseline before a test runs. It runs
inside AWS CodeBuild and uses the CodeBuild service role's credentials (the
default boto3 credential chain), so there is no --profile option.

Usage:
    python3 import.py \
        --input-dir ./baseline \
        [--identity-store-id d-1234567890] \
        [--region us-east-1] \
        [--dry-run]

The Identity Store ID is resolved in this order:
    1. --identity-store-id
    2. the IDENTITY_STORE_ID environment variable
    3. auto-discovery via sso-admin ListInstances (requires sso:ListInstances)

The region defaults to AWS_REGION / AWS_DEFAULT_REGION, which CodeBuild sets.

Records are created through the Identity Store API, which cannot set
ExternalIds, so every imported user and group is "local" (unmanaged). The tests
then run ssosync once with the default attribute map: it matches the local
records by username / display name, adopts them, and sets their ExternalIds
with its own SCIM tenant as the Issuer. After that first pass they are managed
records matched by ExternalId. ExternalIds in the JSON are informational only.

Any failed API call is counted, and the script exits non-zero at the end if
there were failures, so a bad reset fails the build instead of silently
leaving the Identity Store in an unknown state.

JSON file formats:

users.json:
[
    {
        "UserName": "jdoe@example.com",
        "DisplayName": "John Doe",
        "GivenName": "John",
        "FamilyName": "Doe",
        "Active": true,
        "ExternalIds": [{"Issuer": "example.com", "Id": "jdoe-ext-001"}],
        "Emails": [{"Value": "jdoe@example.com", "Type": "work", "Primary": true}],
        "Addresses": [
            {
                "StreetAddress": "123 Main St",
                "Locality": "Seattle",
                "Region": "WA",
                "PostalCode": "98101",
                "Country": "US",
                "Formatted": "123 Main St, Seattle, WA 98101, US",
                "Type": "work",
                "Primary": true
            }
        ]
    }
]

groups.json:
[
    {
        "DisplayName": "Engineering",
        "Description": "Engineering team",
        "Members": ["jdoe@example.com", "asmith@example.com"]
    }
]
"""

import argparse
import json
import os
import sys

import boto3
from botocore.exceptions import ClientError


# Count of failed API calls; a non-zero value makes the script exit 1.
FAILURES = 0


def record_failure(message: str):
    global FAILURES
    FAILURES += 1
    print(message)


def load_json(path: str) -> list:
    with open(path, "r") as f:
        return json.load(f)


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
    """List all users currently in the Identity Store."""
    users = []
    paginator = client.get_paginator("list_users")
    for page in paginator.paginate(IdentityStoreId=identity_store_id):
        users.extend(page.get("Users", []))
    return users


def delete_user(client, identity_store_id: str, user_id: str):
    client.delete_user(IdentityStoreId=identity_store_id, UserId=user_id)


def create_user(client, identity_store_id: str, user: dict) -> str:
    """Create a user and return the new UserId."""
    # Note: the CreateUser API does NOT accept "Active" or "ExternalIds".
    # Those are read-only/response fields (Active is a SCIM attribute managed
    # by the identity provider, ExternalIds are set via provisioning). They are
    # present in the JSON for round-tripping with export.py but are ignored here.
    params = {
        "IdentityStoreId": identity_store_id,
        "UserName": user["UserName"],
        "DisplayName": user.get("DisplayName", user["UserName"]),
        "Name": {
            "GivenName": user.get("GivenName", ""),
            "FamilyName": user.get("FamilyName", ""),
        },
    }
    if user.get("Emails"):
        params["Emails"] = [
            {
                "Value": e["Value"],
                "Type": e.get("Type", "work"),
                "Primary": e.get("Primary", False),
            }
            for e in user["Emails"]
        ]
    if user.get("Addresses"):
        addresses = []
        for a in user["Addresses"]:
            address = {}
            # All address string fields have a minimum length of 1 in the API,
            # so only include the ones that actually have a value. This lets the
            # JSON carry empty strings (e.g. an unset "Type" or "Country") for
            # round-tripping without the API rejecting them.
            for field in (
                "StreetAddress",
                "Locality",
                "Region",
                "PostalCode",
                "Country",
                "Formatted",
                "Type",
            ):
                value = a.get(field, "")
                if value:
                    address[field] = value
            address["Primary"] = a.get("Primary", False)
            addresses.append(address)
        params["Addresses"] = addresses
    resp = client.create_user(**params)
    return resp["UserId"]


# ---------------------------------------------------------------------------
# Groups
# ---------------------------------------------------------------------------

def list_all_groups(client, identity_store_id: str) -> list:
    """List all groups currently in the Identity Store."""
    groups = []
    paginator = client.get_paginator("list_groups")
    for page in paginator.paginate(IdentityStoreId=identity_store_id):
        groups.extend(page.get("Groups", []))
    return groups


def list_group_memberships(client, identity_store_id: str, group_id: str) -> list:
    memberships = []
    paginator = client.get_paginator("list_group_memberships")
    for page in paginator.paginate(IdentityStoreId=identity_store_id, GroupId=group_id):
        memberships.extend(page.get("GroupMemberships", []))
    return memberships


def delete_group(client, identity_store_id: str, group_id: str):
    client.delete_group(IdentityStoreId=identity_store_id, GroupId=group_id)


def create_group(client, identity_store_id: str, group: dict) -> str:
    """Create a group and return the new GroupId."""
    params = {
        "IdentityStoreId": identity_store_id,
        "DisplayName": group["DisplayName"],
    }
    if group.get("Description"):
        params["Description"] = group["Description"]
    resp = client.create_group(**params)
    return resp["GroupId"]


def add_group_member(client, identity_store_id: str, group_id: str, user_id: str):
    client.create_group_membership(
        IdentityStoreId=identity_store_id,
        GroupId=group_id,
        MemberId={"UserId": user_id},
    )


# ---------------------------------------------------------------------------
# Orchestration
# ---------------------------------------------------------------------------

def import_identitystore(identity_store_id: str, input_dir: str, region: str, dry_run: bool):
    users_file = os.path.join(input_dir, "users.json")
    groups_file = os.path.join(input_dir, "groups.json")

    if not os.path.isfile(users_file):
        print(f"ERROR: {users_file} not found.")
        sys.exit(1)
    if not os.path.isfile(groups_file):
        print(f"ERROR: {groups_file} not found.")
        sys.exit(1)

    desired_users = load_json(users_file)
    desired_groups = load_json(groups_file)

    # In dry-run mode we make no API calls, so avoid creating a client
    # (which would force AWS credential resolution and can fail before
    # we ever print what would happen).
    if dry_run:
        client = None
        print(f"Identity Store ID: {identity_store_id or '(not resolved in dry-run)'}")
    else:
        session = boto3.Session(region_name=resolve_region(region))
        identity_store_id = resolve_identity_store_id(session, identity_store_id)
        client = session.client("identitystore")
        print(f"Identity Store ID: {identity_store_id}")
        print(f"Region:            {session.region_name or '(default)'}")

    # --- Phase 1: Delete existing groups (must remove memberships first) ---
    print("\n=== Phase 1: Removing existing groups ===")
    existing_groups = [] if dry_run else list_all_groups(client, identity_store_id)
    if dry_run:
        print("  [DRY-RUN] Skipping lookup of existing groups (no API calls).")
    print(f"Found {len(existing_groups)} existing group(s) to remove.")

    for group in existing_groups:
        group_id = group["GroupId"]
        display_name = group.get("DisplayName", group_id)
        if dry_run:
            print(f"  [DRY-RUN] Would delete group: {display_name} ({group_id})")
        else:
            print(f"  Deleting group: {display_name} ({group_id})")
            try:
                delete_group(client, identity_store_id, group_id)
            except ClientError as e:
                record_failure(f"    ERROR: Failed to delete group {display_name}: {e}")

    # --- Phase 2: Delete existing users ---
    print("\n=== Phase 2: Removing existing users ===")
    existing_users = [] if dry_run else list_all_users(client, identity_store_id)
    if dry_run:
        print("  [DRY-RUN] Skipping lookup of existing users (no API calls).")
    print(f"Found {len(existing_users)} existing user(s) to remove.")

    for user in existing_users:
        user_id = user["UserId"]
        username = user.get("UserName", user_id)
        if dry_run:
            print(f"  [DRY-RUN] Would delete user: {username} ({user_id})")
        else:
            print(f"  Deleting user: {username} ({user_id})")
            try:
                delete_user(client, identity_store_id, user_id)
            except ClientError as e:
                record_failure(f"    ERROR: Failed to delete user {username}: {e}")

    # --- Phase 3: Create desired users ---
    print("\n=== Phase 3: Creating desired users ===")
    print(f"Creating {len(desired_users)} user(s).")

    # Map lowercased username -> user_id for group membership resolution.
    # Identity Store usernames are case-insensitive, and exports can list a
    # member in a different case from the user record (e.g. AceHole@ vs acehole@).
    username_to_id = {}

    for user in desired_users:
        username = user["UserName"]
        if dry_run:
            print(f"  [DRY-RUN] Would create user: {username}")
            username_to_id[username.lower()] = "dry-run-placeholder"
        else:
            print(f"  Creating user: {username}")
            try:
                user_id = create_user(client, identity_store_id, user)
                username_to_id[username.lower()] = user_id
                print(f"    Created: {user_id}")
            except ClientError as e:
                record_failure(f"    ERROR: Failed to create user {username}: {e}")

    # --- Phase 4: Create desired groups and memberships ---
    print("\n=== Phase 4: Creating desired groups ===")
    print(f"Creating {len(desired_groups)} group(s).")

    for group in desired_groups:
        display_name = group["DisplayName"]
        members = group.get("Members", [])

        if dry_run:
            print(f"  [DRY-RUN] Would create group: {display_name} with {len(members)} member(s)")
        else:
            print(f"  Creating group: {display_name}")
            try:
                group_id = create_group(client, identity_store_id, group)
                print(f"    Created: {group_id}")

                for member_username in members:
                    member_user_id = username_to_id.get(member_username.lower())
                    if not member_user_id:
                        record_failure(f"    ERROR: User '{member_username}' not found, skipping membership.")
                        continue
                    try:
                        add_group_member(client, identity_store_id, group_id, member_user_id)
                        print(f"    Added member: {member_username}")
                    except ClientError as e:
                        record_failure(f"    ERROR: Failed to add {member_username} to {display_name}: {e}")

            except ClientError as e:
                record_failure(f"    ERROR: Failed to create group {display_name}: {e}")

    print("\n=== Import complete ===")
    if dry_run:
        print("(No changes were made — dry-run mode)")

    if FAILURES:
        print(f"\nFAILED: {FAILURES} API call(s) failed; the Identity Store may not match the baseline.")
        sys.exit(1)


def main():
    parser = argparse.ArgumentParser(
        description="Import (replace) all users and groups in an AWS Identity Store from JSON files "
                    "(CodeBuild variant: uses the build's service role credentials)."
    )
    parser.add_argument(
        "--identity-store-id", default=None,
        help="The Identity Store ID (e.g., d-1234567890). Defaults to IDENTITY_STORE_ID, "
             "then auto-discovery via sso-admin ListInstances."
    )
    parser.add_argument(
        "--input-dir", required=True,
        help="Directory containing users.json and groups.json"
    )
    parser.add_argument(
        "--region", default=None,
        help="AWS region (defaults to AWS_REGION / AWS_DEFAULT_REGION)"
    )
    parser.add_argument(
        "--dry-run", action="store_true",
        help="Print what would happen without making changes"
    )

    args = parser.parse_args()

    print(f"Input directory:   {args.input_dir}")
    print(f"Dry run:           {args.dry_run}")

    import_identitystore(
        identity_store_id=args.identity_store_id,
        input_dir=args.input_dir,
        region=args.region,
        dry_run=args.dry_run,
    )


if __name__ == "__main__":
    main()
