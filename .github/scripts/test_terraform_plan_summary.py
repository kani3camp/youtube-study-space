#!/usr/bin/env python3

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("terraform_plan_summary.py")
SHA = "a" * 40
SECRETS = [
    "TOP_SECRET_DO_NOT_LEAK",
    "hunter2-do-not-log",
    "projects/example/secrets/private/versions/7",
    "private-import-id-12345",
]


def run_sanitizer(plan: dict, policy: str) -> tuple[subprocess.CompletedProcess[str], str, str, dict]:
    with tempfile.TemporaryDirectory() as tmp:
        json_out = Path(tmp) / "summary.json"
        md_out = Path(tmp) / "summary.md"
        proc = subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "--environment",
                "dev",
                "--git-sha",
                SHA,
                "--policy",
                policy,
                "--json-output",
                str(json_out),
                "--markdown-output",
                str(md_out),
            ],
            input=json.dumps(plan),
            text=True,
            capture_output=True,
            check=False,
        )
        safe_json = json_out.read_text(encoding="utf-8") if json_out.exists() else ""
        safe_md = md_out.read_text(encoding="utf-8") if md_out.exists() else ""
        parsed = json.loads(safe_json) if safe_json else {}
        return proc, safe_json, safe_md, parsed


class TerraformPlanSummaryTest(unittest.TestCase):
    def test_import_only_summary_never_emits_values_or_import_id(self) -> None:
        plan = {
            "terraform_version": "1.16.4",
            "resource_changes": [
                {
                    "address": "google_storage_bucket.backup",
                    "mode": "managed",
                    "change": {
                        "actions": ["no-op"],
                        "importing": {"id": SECRETS[3]},
                        "before": {"token": SECRETS[0]},
                        "after": {"password": SECRETS[1]},
                        "before_sensitive": {"token": True},
                        "after_sensitive": {"password": True},
                    },
                },
                {
                    "address": "data.google_secret_manager_secret_version.private",
                    "mode": "data",
                    "change": {
                        "actions": ["read"],
                        "before": None,
                        "after": {"name": SECRETS[2], "secret_data": SECRETS[0]},
                    },
                },
            ],
            "output_changes": {"secret": {"after": SECRETS[0], "after_sensitive": True}},
            "configuration": {"provider_config": {"google": {"expressions": {"token": SECRETS[1]}}}},
        }
        proc, safe_json, safe_md, parsed = run_sanitizer(plan, "import-only")
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertTrue(parsed["policy_passed"])
        self.assertEqual(parsed["counts"]["import"], 1)
        self.assertEqual(parsed["counts"]["read"], 1)
        for secret in SECRETS:
            self.assertNotIn(secret, safe_json)
            self.assertNotIn(secret, safe_md)
            self.assertNotIn(secret, proc.stdout)
            self.assertNotIn(secret, proc.stderr)

    def test_import_only_stops_create_update_delete_replace_and_drift(self) -> None:
        plan = {
            "terraform_version": "1.16.4",
            "resource_changes": [
                {"address": "example.create", "mode": "managed", "change": {"actions": ["create"], "after": {"v": SECRETS[0]}}},
                {"address": "example.update", "mode": "managed", "change": {"actions": ["update"], "after": {"v": SECRETS[1]}}},
                {"address": "example.delete", "mode": "managed", "change": {"actions": ["delete"], "before": {"v": SECRETS[2]}}},
                {"address": "example.replace", "mode": "managed", "change": {"actions": ["delete", "create"], "after": {"v": SECRETS[3]}}},
            ],
            "resource_drift": [
                {"address": "example.drifted", "change": {"actions": ["update"], "before": {"x": SECRETS[0]}, "after": {"x": SECRETS[1]}}}
            ],
        }
        proc, safe_json, safe_md, parsed = run_sanitizer(plan, "import-only")
        self.assertEqual(proc.returncode, 3)
        self.assertFalse(parsed["policy_passed"])
        self.assertEqual(parsed["counts"]["create"], 1)
        self.assertEqual(parsed["counts"]["update"], 1)
        self.assertEqual(parsed["counts"]["delete"], 1)
        self.assertEqual(parsed["counts"]["replace"], 1)
        self.assertEqual(parsed["counts"]["drift"], 1)
        for secret in SECRETS:
            self.assertNotIn(secret, safe_json)
            self.assertNotIn(secret, safe_md)
            self.assertNotIn(secret, proc.stdout)
            self.assertNotIn(secret, proc.stderr)

    def test_history_import_cannot_include_legacy_column_removal(self) -> None:
        canonical = [{"name": "taken_at", "type": "TIMESTAMP"}]
        plan = {
            "terraform_version": "1.16.4",
            "resource_changes": [{
                "address": "module.user_activity_history[0].google_bigquery_table.retained",
                "mode": "managed",
                "change": {
                    "actions": ["update"], "importing": {"id": SECRETS[3]},
                    "before": {"schema": canonical + [{"name": "timestamp", "type": "TIMESTAMP"}]},
                    "after": {"schema": canonical, "synthetic_private_value": SECRETS[0]},
                },
            }],
        }
        proc, safe_json, safe_md, parsed = run_sanitizer(plan, "import-only")
        self.assertEqual(proc.returncode, 3)
        self.assertFalse(parsed["policy_passed"])
        self.assertEqual(parsed["counts"]["import"], 1)
        self.assertEqual(parsed["counts"]["update"], 1)
        self.assertNotIn("schema", parsed["resources"][0])
        for secret in SECRETS:
            self.assertNotIn(secret, safe_json + safe_md + proc.stdout + proc.stderr)

    def test_no_destroy_allows_create_update_but_stops_delete_or_replace(self) -> None:
        allowed = {
            "terraform_version": "1.16.4",
            "resource_changes": [
                {"address": "example.create", "mode": "managed", "change": {"actions": ["create"]}},
                {"address": "example.update", "mode": "managed", "change": {"actions": ["update"]}},
            ],
        }
        proc, _, _, parsed = run_sanitizer(allowed, "no-destroy")
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertTrue(parsed["policy_passed"])

        blocked = {
            "terraform_version": "1.16.4",
            "resource_changes": [
                {"address": "example.replace", "mode": "managed", "change": {"actions": ["create", "delete"]}}
            ],
        }
        proc, _, _, parsed = run_sanitizer(blocked, "no-destroy")
        self.assertEqual(proc.returncode, 3)
        self.assertFalse(parsed["policy_passed"])

    def test_unknown_action_fails_closed(self) -> None:
        plan = {
            "terraform_version": "1.16.4",
            "resource_changes": [
                {"address": "example.future", "mode": "managed", "change": {"actions": ["mystery-action"]}}
            ],
        }
        proc, _, _, parsed = run_sanitizer(plan, "import-only")
        self.assertEqual(proc.returncode, 3)
        self.assertFalse(parsed["policy_passed"])
        self.assertEqual(parsed["counts"]["other"], 1)

    def test_markdown_escapes_untrusted_resource_address(self) -> None:
        plan = {
            "terraform_version": "1.16.4",
            "resource_changes": [
                {"address": "example.x[\"<tag>|value\"]", "mode": "managed", "change": {"actions": ["no-op"]}}
            ],
        }
        proc, _, safe_md, _ = run_sanitizer(plan, "import-only")
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertNotIn("<tag>", safe_md)
        self.assertNotIn("|value", safe_md)
        self.assertIn("&lt;tag&gt;&#124;value", safe_md)


if __name__ == "__main__":
    unittest.main()
