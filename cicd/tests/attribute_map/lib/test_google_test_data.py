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

    def test_default_map_work_title(self):
        # default_map: the built-in map's organizations[type=work].title resolves
        # to "Science Officer" for one user and to nothing (noTypeMatch) for another.
        script = read("default_map", "run.sh")
        src = "organizations[type=work].title"
        expected = re.search(rf'"source={re.escape(src)}" --field "resolved=([^"]+)"', script).group(1)
        self.assertIn(expected, {resolve(u, src) for u in users()})
        self.assertIn("", {resolve(u, src) for u in users()}, "some user must have no work organization")

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
        allowed = {"primaryEmail", "id", "name", "suspended", "organizations", "phones",
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

    def test_baseline_matches_google_users(self):
        with open(os.path.join(AM_DIR, "baseline", "users.json")) as f:
            baseline = {b["UserName"].lower(): b for b in json.load(f)}
        self.assertEqual({u["primaryEmail"].lower() for u in users()}, set(baseline))
        for u in users():
            self.assertEqual(u["id"], baseline[u["primaryEmail"].lower()]["ExternalIds"][0]["Id"])


if __name__ == "__main__":
    unittest.main()
