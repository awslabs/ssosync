#!/usr/bin/env python3
"""
Local consistency tests: the Google test users in google_test_users.json must
satisfy the data rules, and the suites' expectations must match that data.

Run from anywhere:
  python3 cicd/tests/attribute_map/lib/test_google_test_data.py
"""

import json
import os
import re
import shlex
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
AM_DIR = os.path.dirname(HERE)

EMAIL = re.compile(r"^[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$")


def read(*parts):
    with open(os.path.join(AM_DIR, *parts)) as f:
        return f.read()


def users():
    with open(os.path.join(AM_DIR, "google_test_users.json")) as f:
        return json.load(f)["users"]


def schema(user):
    return user.get("customSchemas", {}).get("SSOSync-Test", {})


def in_ou_tree(user_ou, ou):
    """Google's orgUnitPath query matches the OU and every OU under it."""
    return ou == "/" or user_ou == ou or user_ou.startswith(ou + "/")


def staging_precache_ous(stack):
    """The PrecacheOrgUnits of a staging stack, normalised as ssosync does
    (trim; strip a trailing / unless the path is /)."""
    path = os.path.join(AM_DIR, "..", "..", "account_execution", "staging", stack)
    with open(path) as f:
        value = re.search(r"^\s*PrecacheOrgUnits: '((?:[^']|'')*)'", f.read(), re.M).group(1)
    ous = []
    for ou in value.replace("''", "'").split(","):
        ou = ou.strip()
        if ou != "/" and ou.endswith("/"):
            ou = ou[:-1]
        if ou and ou not in ous:
            ous.append(ou)
    return ous


SELECTOR = re.compile(r"^(\w+)\[(primary|type=([^\]]+))\]\.(\w+)$")


def resolve(user, source):
    """What ssosync resolves a standard multi-valued source to for this user:
    [primary] picks the entry marked primary, [type=X] the first entry whose
    type matches case-insensitively; no match is "" (never a fallback)."""
    m = SELECTOR.match(source)
    if not m:
        raise ValueError(f"unsupported source in test: {source}")
    collection, _, type_value, field = m.groups()
    for entry in user.get(collection, []):
        if type_value is None and entry.get("primary") is True:
            return entry.get(field, "")
        if type_value is not None and entry.get("type", "").lower() == type_value.lower():
            return entry.get(field, "")
    return ""


def full_name(user):
    """Google's read-only name.fullName: given and family name joined by a space."""
    return f"{user['name']['givenName']} {user['name']['familyName']}"


def readback_expectations(script, var):
    """{(username, jq-path): value} that a run.sh asserts with assert_user_field
    on the read-back file in $var, from single lines and `for user in` loops."""
    out = {}
    for user, path, value in re.findall(rf'assert_user_field "\${var}" "([^"$]+)" (\.\S+) "([^"]*)"', script):
        out[(user, path)] = value
    for loop, body in re.findall(r"^for user in ([^\n]+); do\n(.*?)^done", script, re.S | re.M):
        for user in shlex.split(loop):
            for path, value in re.findall(rf'assert_user_field "\${var}" "\$user" (\.\S+) "([^"]*)"', body):
                out[(user, path)] = value
    return out


class DataRules(unittest.TestCase):
    def test_aws_username_is_an_email(self):
        for u in users():
            value = schema(u).get("AWSUsername")
            self.assertIsNotNone(value, f"{u['primaryEmail']} has no AWSUsername")
            self.assertRegex(value, EMAIL, f"{u['primaryEmail']}: AWSUsername must be an email")

    def test_aws_username_differs_from_primary_email_and_is_unique(self):
        names = [schema(u)["AWSUsername"].lower() for u in users()]
        self.assertEqual(len(names), len(set(names)), "AWSUsername values must be unique")
        for u in users():
            self.assertNotEqual(schema(u)["AWSUsername"].lower(), u["primaryEmail"].lower(),
                                "AWSUsername must differ so the remap suite updates every user")

    def test_at_most_one_cost_center_and_it_is_a_scalar(self):
        for u in users():
            value = schema(u).get("CostCodes")
            if value is None:
                continue
            self.assertIsInstance(value, str, f"{u['primaryEmail']}: CostCodes is single-valued, not a list")
            self.assertTrue(value.strip())

    def test_no_websites(self):
        for u in users():
            self.assertNotIn("websites", u)
        for name in ("rich_map/run.sh", "lambda_inline_map/run.sh"):
            self.assertNotIn("websites", read(name))
        self.assertNotIn("websites", read("maps", "rich.json"))


class SuiteExpectations(unittest.TestCase):
    def test_username_remap_expects_every_aws_username(self):
        script = read("username_remap", "run.sh")
        expected = {schema(u)["AWSUsername"] for u in users()}
        # The loop in run.sh lists the usernames as bare words.
        listed = set(re.findall(r"[\w.+-]+@[\w.-]+\.\w+", script.split("for username in", 1)[1].split("do", 1)[0]))
        self.assertEqual(expected, listed)

    def test_username_remap_user_count(self):
        self.assertRegex(read("username_remap", "run.sh"), rf"update={len(users())}\b")

    def test_rich_map_cost_centers(self):
        script = read("rich_map", "run.sh")
        source = "customSchemas.SSOSync-Test.CostCodes"
        resolved = set(re.findall(rf'resolved "{re.escape(source)}" "([^"]+)"', script))
        self.assertEqual({schema(u)["CostCodes"] for u in users() if "CostCodes" in schema(u)}, resolved)
        self.assertNotIn("[type=", script.split(source, 1)[1].split('"', 1)[0],
                         "CostCodes is a scalar; no type selector")

    def test_rich_map_absent_cost_center_count(self):
        without = sum(1 for u in users() if "CostCodes" not in schema(u))
        m = re.search(r"field=CostCodes --min (\d+)", read("rich_map", "run.sh"))
        self.assertIsNotNone(m)
        self.assertEqual(without, int(m.group(1)))

    def test_rich_map_every_user_misses_the_absent_custom_fields(self):
        script = read("rich_map", "run.sh")
        for field in ("NoSuchSchema", "DoesNotExist"):
            m = re.search(rf"{field}.*?--min (\d+)", script, re.S)
            self.assertIsNotNone(m, field)
            self.assertEqual(len(users()), int(m.group(1)))

    def test_rich_map_json_uses_scalar_cost_center(self):
        with open(os.path.join(AM_DIR, "maps", "rich.json")) as f:
            rich = json.load(f)["user"]
        self.assertEqual("customSchemas.SSOSync-Test.CostCodes", rich["Enterprise.CostCenter"])

    def test_lambda_inline_expects_a_real_cost_center(self):
        script = read("lambda_inline_map", "run.sh")
        self.assertIn('source=customSchemas.SSOSync-Test.CostCodes"', script)
        value = re.search(r'resolved=([^"\s]+)', script).group(1)
        self.assertIn(value, {schema(u).get("CostCodes") for u in users()})

    def test_manager_optin_expects_the_manager_relation(self):
        script = read("manager_optin", "run.sh")
        managers = {r["value"] for u in users() for r in u.get("relations", []) if r["type"] == "manager"}
        value = re.search(r"resolved=([^\"\s]+)", script).group(1)
        self.assertIn(value, managers)

    def test_multi_valued_custom_field_is_in_the_data(self):
        holders = [u for u in users() if "ProjectCodes" in schema(u)]
        self.assertGreaterEqual(len(holders), 1)
        for u in holders:
            values = schema(u)["ProjectCodes"]
            self.assertIsInstance(values, list, f"{u['primaryEmail']}: ProjectCodes is multi-valued")
            self.assertTrue(values)
            for v in values:
                self.assertTrue(v.get("value"), f"{u['primaryEmail']}: ProjectCodes entry needs a value")

    def test_rich_map_maps_the_multi_valued_custom_field(self):
        with open(os.path.join(AM_DIR, "maps", "rich.json")) as f:
            rich = json.load(f)["user"]
        self.assertIn("customSchemas.SSOSync-Test.ProjectCodes", rich.values())
        for source in rich.values():
            if source.startswith("customSchemas."):
                self.assertNotIn("[", source, "custom sources must not carry a selector")

    def test_rich_map_expects_multi_valued_warning_for_each_holder(self):
        script = read("rich_map", "run.sh")
        holders = {u["primaryEmail"] for u in users() if "ProjectCodes" in schema(u)}
        loop = script.split("for user in", 1)[1].split("; do", 1)[0].split()
        self.assertEqual(holders, set(loop))
        absent = sum(1 for u in users() if "ProjectCodes" not in schema(u))
        m = re.search(r"field=ProjectCodes --min (\d+)", script)
        self.assertEqual(absent, int(m.group(1)))

    def test_rich_map_never_expects_a_multi_valued_value(self):
        script = read("rich_map", "run.sh")
        values = {v["value"] for u in users() for v in schema(u).get("ProjectCodes", [])}
        loop = set(script.split("for value in", 1)[1].split("; do", 1)[0].split())
        self.assertEqual(values, loop, "every multi-valued value must be asserted absent")

    def test_multi_valued_warning_text_matches_the_code(self):
        with open(os.path.join(AM_DIR, "..", "..", "..", "internal", "google", "client.go")) as f:
            code = f.read()
        const = re.search(r'MultiValuedCustomFieldIgnoredMsg = "([^"]+)"', code).group(1)
        self.assertIn(f'MULTI="{const}"', read("rich_map", "run.sh"))

    def test_default_map_org_title(self):
        # default_map: the built-in map's organization title source (the one in
        # config.DefaultUserAttributeMap) resolves to the asserted title for one
        # user and to nothing for another.
        with open(os.path.join(AM_DIR, "..", "..", "..", "internal", "config", "config.go")) as f:
            src = re.search(r'"Title":\s+"([^"]+)"', f.read()).group(1)
        script = read("default_map", "run.sh")
        expected = re.search(rf'"source={re.escape(src)}" --field "resolved=([^"]+)"', script)
        self.assertIsNotNone(expected, f"default_map must assert the default source {src}")
        self.assertIn(expected.group(1), {resolve(u, src) for u in users()})
        self.assertIn("", {resolve(u, src) for u in users()}, f"some user must resolve {src} to nothing")

    def test_default_map_blank_given_name(self):
        self.assertIn("field=name.givenName", read("default_map", "run.sh"))
        self.assertTrue(any(not u["name"]["givenName"].strip() for u in users()),
                        "some user must have a blank given name (requiredField warning)")

    def test_rich_map_standard_selectors(self):
        # Every value rich_map asserts for a standard selector source is what
        # ssosync resolves for some user in the reference.
        script = read("rich_map", "run.sh")
        asserted = re.findall(r'^resolved "([a-z]+\[[^"]+)" "([^"]*)"', script, re.M)
        self.assertGreaterEqual(len(asserted), 6)
        for src, value in asserted:
            self.assertIn(value, {resolve(u, src) for u in users()}, f"{src} -> {value!r}")

    def test_rich_map_primary_differs_from_work(self):
        # [primary] and [type=work] must pick different organizations for some
        # user, or the suite cannot tell the selectors apart.
        self.assertTrue(any(resolve(u, "organizations[primary].name") != resolve(u, "organizations[type=work].name")
                            and resolve(u, "organizations[type=work].name") for u in users()))

    def test_rich_map_no_primary_phone(self):
        self.assertIn("reason=noPrimaryEntry", read("rich_map", "run.sh"))
        self.assertTrue(any(u.get("phones") and not any(p.get("primary") for p in u["phones"]) for u in users()),
                        "some user must have phones but none marked primary")

    def test_reference_is_a_patch_body(self):
        # Only Directory API user fields; no free-text notes.
        allowed = {"primaryEmail", "id", "name", "suspended", "orgUnitPath", "organizations", "phones",
                   "addresses", "relations", "customSchemas"}
        for u in users():
            self.assertLessEqual(set(u), allowed, f"{u['primaryEmail']}: unexpected keys {set(u) - allowed}")
            self.assertEqual({"givenName", "familyName"}, set(u["name"]))
            for o in u.get("organizations", []):
                self.assertIn("type", o, f"{u['primaryEmail']}: organizations must be typed")

    def test_validation_warnings_long_value(self):
        # validation_warnings maps Username from SSOSync-Test.LongValue: exactly
        # the holders exceed the SCIM userName maxLength (128), and every other
        # user has none (missing-field and requiredField warnings).
        with open(os.path.join(AM_DIR, "maps", "long_username.json")) as f:
            self.assertEqual("customSchemas.SSOSync-Test.LongValue", json.load(f)["user"]["Username"])
        holders = [u for u in users() if "LongValue" in schema(u)]
        self.assertGreaterEqual(len(holders), 1)
        for u in holders:
            value = schema(u)["LongValue"]
            self.assertIsInstance(value, str, f"{u['primaryEmail']}: LongValue is single-valued")
            self.assertGreater(len(value), 128, f"{u['primaryEmail']}: LongValue must exceed 128 characters")
        script = read("validation_warnings", "run.sh")
        absent = len(users()) - len(holders)
        self.assertEqual(absent, int(re.search(r"rule=requiredField --field field=userName --min (\d+)", script).group(1)))
        self.assertEqual(absent, int(re.search(r"field=LongValue --min (\d+)", script).group(1)))
        self.assertEqual(len(holders), int(re.search(r"rule=maxLength --field field=userName --min (\d+)", script).group(1)))

    def test_default_map_precaches_an_apostrophe_ou(self):
        # #401: default_map precaches an OU whose path has an apostrophe, and
        # that OU holds a test user whose primary email has one too.
        m = re.search(r"--precache-ous \"([^\"]+)\"", read("default_map", "run.sh"))
        self.assertIsNotNone(m, "default_map must pass --precache-ous")
        ou = m.group(1)
        self.assertIn("'", ou)
        self.assertNotIn(",", ou, "getUsers splits queries on commas")
        holders = [u for u in users() if u.get("orgUnitPath") == ou]
        self.assertEqual(1, len(holders), f"exactly one test user must be in {ou}")
        self.assertIn("'", holders[0]["primaryEmail"])

    def test_every_user_has_an_org_unit(self):
        for u in users():
            self.assertTrue(u.get("orgUnitPath", "").startswith("/"), f"{u['primaryEmail']}: orgUnitPath")

    def test_default_map_missing_ou_holds_no_test_user(self):
        # #404: default_map's second --precache-ous is an OU that does not
        # exist, so the per-OU warning is expected for it; and only the first
        # OU's user may be precached, so no other user is in its tree.
        ous = re.findall(r"--precache-ous \"([^\"]+)\"", read("default_map", "run.sh"))
        self.assertEqual(2, len(ous), "default_map passes two --precache-ous")
        first, missing = ous
        self.assertNotIn(",", missing, "getUsers splits queries on commas")
        self.assertEqual([], [u["primaryEmail"] for u in users() if in_ou_tree(u["orgUnitPath"], missing)])
        self.assertEqual(1, sum(1 for u in users() if in_ou_tree(u["orgUnitPath"], first)),
                         f"only one test user may be in the tree of {first}")
        script = read("default_map", "run.sh")
        self.assertIn(f'PRECACHE_OU="{first}"', script)
        self.assertIn(f'MISSING_OU="{missing}"', script)

    def test_lambda_default_precache_expectations(self):
        # #404: the staging stacks' PrecacheOrgUnits, normalised as ssosync
        # does, are exactly the OUs lambda_default asserts on: the zero-users
        # OU holds no test user, the others each hold at least one.
        lists = {name: staging_precache_ous(name) for name in ("stack.yml", "stack-ng.yml")}
        self.assertEqual(lists["stack.yml"], lists["stack-ng.yml"], "both staging stacks precache the same OUs")
        configured = lists["stack.yml"]
        script = read("lambda_default", "run.sh")
        empty = re.findall(r'"PrecacheOrgUnits: no users found in OrgUnitPath" \\\n\s+--field "orgUnitPath=([^"]+)"', script)
        loop = script.split("for ou in", 1)[1].split("; do", 1)[0]
        with_users = re.findall(r'"([^"]+)"', loop)
        self.assertEqual(["/Catering"], empty)
        self.assertEqual(sorted(configured), sorted(empty + with_users))
        for ou in empty:
            self.assertEqual([], [u["primaryEmail"] for u in users() if in_ou_tree(u["orgUnitPath"], ou)])
        for ou in with_users:
            self.assertTrue(any(in_ou_tree(u["orgUnitPath"], ou) for u in users()), f"no test user in {ou}")

    def test_default_map_sees_every_test_user(self):
        # #413: default_map's exact Cache Stats check expects one key per test
        # user's OU, so every test user must be a member of a group the suites
        # sync (GROUP_MATCH name:AWS*). Members are compared case-insensitively,
        # as the baseline lists some in lower case. The converse holds too: the
        # AWS* groups hold no user outside the fixture, so the jq key set
        # (keys == the fixture's OUs) has no OU the fixture doesn't know. The
        # baseline is the AWS-side seed, so this guards fixture drift, not the
        # live Google membership.
        self.assertIn('GROUP_MATCH="${GROUP_MATCH:-name:AWS*}"', read("lib", "common.sh"))
        self.assertIn('GROUP_MATCH: "name:AWS*"', read("default_map", "buildspec.yml"))
        self.assertIn("google_test_users.json", read("default_map", "run.sh"))
        with open(os.path.join(AM_DIR, "baseline", "groups.json")) as f:
            members = {m.lower() for g in json.load(f) if g["DisplayName"].startswith("AWS") for m in g["Members"]}
        for u in users():
            self.assertIn(u["primaryEmail"].lower(), members, f"{u['primaryEmail']} is in no AWS* group")
        extra = members - {u["primaryEmail"].lower() for u in users()}
        self.assertEqual(set(), extra, "AWS* groups in baseline/groups.json hold users missing from google_test_users.json")

    def test_baseline_matches_google_users(self):
        with open(os.path.join(AM_DIR, "baseline", "users.json")) as f:
            baseline = {b["UserName"].lower(): b for b in json.load(f)}
        self.assertEqual({u["primaryEmail"].lower() for u in users()}, set(baseline))
        for u in users():
            self.assertEqual(u["id"], baseline[u["primaryEmail"].lower()]["ExternalIds"][0]["Id"])

    # #421: Release optional NickName and Name.Formatted mappings (PR #412)

    def test_rich_map_maps_opt_in_names(self):
        with open(os.path.join(AM_DIR, "maps", "rich.json")) as f:
            rich = json.load(f)["user"]
        self.assertEqual("customSchemas.SSOSync-Test.NickName", rich["NickName"])
        self.assertEqual("name.fullName", rich["Name.Formatted"])

    def test_nickname_holders(self):
        # NickName is a single-valued custom field: its holders' values are
        # exactly what rich_map expects resolved, each differs from the
        # holder's given name (else the suite can't tell the targets apart),
        # every other user gets the absent-field warning, and lambda_inline_map
        # expects a holder's value.
        holders = [u for u in users() if "NickName" in schema(u)]
        self.assertGreaterEqual(len(holders), 1)
        for u in holders:
            value = schema(u)["NickName"]
            self.assertIsInstance(value, str, f"{u['primaryEmail']}: NickName is single-valued")
            self.assertTrue(value.strip())
            self.assertNotEqual(value, u["name"]["givenName"], f"{u['primaryEmail']}: NickName must differ from givenName")
        script = read("rich_map", "run.sh")
        source = "customSchemas.SSOSync-Test.NickName"
        resolved = set(re.findall(rf'^resolved "{re.escape(source)}" "([^"]+)"', script, re.M))
        self.assertEqual({schema(u)["NickName"] for u in holders}, resolved)
        m = re.search(r"field=NickName --min (\d+)", script)
        self.assertIsNotNone(m)
        self.assertEqual(len(users()) - len(holders), int(m.group(1)))
        m = re.search(rf'source={re.escape(source)}" --field "resolved=([^"]+)"', read("lambda_inline_map", "run.sh"))
        self.assertIsNotNone(m, "lambda_inline_map must assert a resolved NickName")
        self.assertIn(m.group(1), {schema(u)["NickName"] for u in holders})

    def test_rich_map_formatted_names(self):
        # Name.Formatted <- name.fullName resolves for every user (AceHole's
        # blank given name gives "  Rimmer"); lambda_inline_map expects one.
        expected = {full_name(u) for u in users()}
        self.assertIn("  Rimmer", expected)
        resolved = set(re.findall(r'^resolved "name\.fullName" "([^"]+)"', read("rich_map", "run.sh"), re.M))
        self.assertEqual(expected, resolved)
        m = re.search(r'source=name\.fullName" --field "resolved=([^"]+)"', read("lambda_inline_map", "run.sh"))
        self.assertIsNotNone(m, "lambda_inline_map must assert a resolved name.fullName")
        self.assertIn(m.group(1), expected)

    def test_rich_map_update_counts(self):
        # Every user has a full name, so the rich run (which sets
        # Name.Formatted) and the default-map run (which clears it) each
        # update every user.
        counts = re.findall(r"--users update=(\d+)", read("rich_map", "run.sh"))
        self.assertEqual(2, len(counts))
        for c in counts:
            self.assertEqual(len(users()), int(c))

    def test_rich_map_readback_expectations(self):
        # The read-back after the rich run expects every user's derived full
        # name and the holder's NickName ("" for the others); after the
        # default-map run both are "" for every user.
        script = read("rich_map", "run.sh")
        want = {}
        for u in users():
            want[(u["primaryEmail"], ".Name.Formatted")] = full_name(u)
            want[(u["primaryEmail"], ".NickName")] = schema(u).get("NickName", "")
        self.assertEqual(want, readback_expectations(script, "USERS"))
        cleared = {key: "" for key in want}
        self.assertEqual(cleared, readback_expectations(script, "CLEARED"))


if __name__ == "__main__":
    unittest.main()
