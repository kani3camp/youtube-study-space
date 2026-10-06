#!/usr/bin/env python3

from __future__ import annotations

import os
import copy
import importlib.util
import json
import re
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path

GITHUB_DIR = Path(__file__).parents[1]
WORKFLOW = GITHUB_DIR / "workflows" / "gcp-user-activity-schema-audit.yml"
CALLER = GITHUB_DIR / "workflows" / "ci.yml"
SUMMARY = GITHUB_DIR.parent / "infra" / "gcp" / "scripts" / "schema_audit_summary.py"
HANDOFF_FIXTURES = GITHUB_DIR.parent / "infra" / "gcp" / "tests" / "fixtures" / "environment-secret-handoff"
spec = importlib.util.spec_from_file_location("schema_audit_summary", SUMMARY)
summary_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(summary_module)


def audit_fixture() -> dict:
    return {
        "mode": "read-only-aggregate",
        "target": dict(summary_module.TARGET),
        "audit": {
            "canonical": False,
            "has_taken_at": True,
            "has_legacy_timestamp": True,
            "legacy_non_null": 900001,
            "legacy_only": 400001,
            "both_equal": 300000,
            "both_different": 200000,
        },
    }


class UserActivitySchemaAuditWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.text = WORKFLOW.read_text(encoding="utf-8")
        cls.caller = CALLER.read_text(encoding="utf-8")

    def test_reusable_workflow_is_not_directly_triggerable(self) -> None:
        self.assertIn("workflow_call:", self.text)
        self.assertNotIn("workflow_dispatch:", self.text)
        self.assertNotIn("pull_request_target", self.text)
        self.assertNotIn("secrets: inherit", self.text)

    def test_audit_is_default_off_and_development_only(self) -> None:
        self.assertIn('DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED: "false"', self.text)
        self.assertIn('[[ "${TARGET}" == "dev" ]]', self.text)
        self.assertIn("Production schema audit is not enabled", self.text)
        self.assertIn("pending dedicated IAM and user approval", self.text)

    def test_query_budget_is_explicit_and_bounded(self) -> None:
        self.assertIn('USER_ACTIVITY_SCHEMA_AUDIT_MAX_BYTES_BILLED: "1073741824"', self.text)
        self.assertNotIn("--dry_run", self.text)
        self.assertNotIn("DryRun", self.text)

    def test_trusted_surface_is_fixed_before_auth(self) -> None:
        guard = self.text.split("      - name: Enforce trusted read-only audit surface", 1)[1]
        auth = guard.index("      - name: Configure dedicated GCP audit credential")
        for token in (
            'GITHUB_EVENT_NAME}" == "workflow_dispatch"',
            'GITHUB_REPOSITORY}" == "kani3camp/youtube-study-space"',
            'GITHUB_REPOSITORY_ID}" == "340900071"',
            'GITHUB_REPOSITORY_OWNER_ID}" == "54093651"',
            "refs/heads/feature/gcp-terraform-iac",
        ):
            self.assertLess(guard.index(token), auth)

    def test_dedicated_environment_and_identity_are_separate_from_terraform(self) -> None:
        self.assertIn("name: gcp-dev-user-activity-schema-audit", self.text)
        self.assertIn("GCP_USER_ACTIVITY_SCHEMA_AUDIT_SERVICE_ACCOUNT", self.text)
        self.assertNotIn("GCP_TERRAFORM_SERVICE_ACCOUNT", self.text)
        self.assertIn("GCP_TERRAFORM_WIF_PROVIDER", self.text)
        self.assertNotIn("AWS_TERRAFORM_", self.text)
        self.assertNotIn("terraform init", self.text)
        self.assertNotIn("terraform plan", self.text)
        self.assertNotIn("terraform apply", self.text)

    def test_business_data_output_stays_private(self) -> None:
        self.assertIn(' >"${RUNNER_TEMP}/user-activity-schema-audit.json"'.strip(), self.text)
        self.assertIn("schema_audit_summary.py", self.text)
        self.assertNotIn("upload-artifact", self.text)
        self.assertNotIn('cat "${RUNNER_TEMP}/user-activity-schema-audit.json"', self.text)
        for key in ("legacy_non_null", "legacy_only", "both_equal", "both_different"):
            self.assertIn(f"- {key} is zero:", summary_module.summarize(audit_fixture()))

    def test_cli_scope_is_exact(self) -> None:
        self.assertIn("go run ./cmd/user-activity-schema-audit", self.text)
        self.assertIn("test-youtube-study-space", self.text)
        self.assertIn("asia-southeast2", self.text)
        self.assertEqual(summary_module.TARGET["dataset"], "firestore_export")
        self.assertEqual(summary_module.TARGET["table"], "user-activity-history")

    def step_script(self, name: str) -> str:
        step = self.text.split(f"      - name: {name}\n", 1)[1]
        return textwrap.dedent(step.split("        run: |\n", 1)[1].split("\n      - name:", 1)[0])

    def test_private_identifiers_are_masked_before_auth(self) -> None:
        name = "Mask private identity fragments before authentication"
        self.assertLess(self.text.index(name), self.text.index("uses: google-github-actions/auth@"))
        provider = "projects/123456789/locations/global/workloadIdentityPools/synthetic-pool/providers/synthetic-provider"
        sa = "synthetic-audit@test-youtube-study-space.iam.gserviceaccount.com"
        result = subprocess.run(["bash", "-c", self.step_script(name)], capture_output=True, text=True,
                                env={"PATH": os.environ["PATH"], "PRIVATE_WIF_PROVIDER": provider, "PRIVATE_AUDIT_SA": sa})
        self.assertEqual(result.returncode, 0)
        self.assertEqual(set(result.stdout.splitlines()), {
            f"::add-mask::{provider}", f"::add-mask::{sa}", "::add-mask::synthetic-audit",
            "::add-mask::123456789", "::add-mask::synthetic-pool", "::add-mask::synthetic-provider",
        })
        result = subprocess.run(["bash", "-c", self.step_script(name)], capture_output=True, text=True,
                                env={"PATH": os.environ["PATH"], "PRIVATE_WIF_PROVIDER": provider,
                                     "PRIVATE_AUDIT_SA": "private-invalid-identity"})
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("private-invalid-identity", result.stdout + result.stderr)

    def test_missing_or_invalid_identity_stops_without_echoing_input(self) -> None:
        provider = "projects/123456789/locations/global/workloadIdentityPools/synthetic-pool/providers/synthetic-provider"
        sa = "synthetic-audit@test-youtube-study-space.iam.gserviceaccount.com"
        for a, b in (("", ""), (provider, ""), ("", sa),
                     ("synthetic-invalid-provider", sa), (provider, "synthetic-invalid-sa")):
            with self.subTest(provider_present=bool(a), sa_present=bool(b)):
                result = subprocess.run(
                    ["bash", "-c", self.step_script("Mask private identity fragments before authentication")],
                    capture_output=True, text=True,
                    env={"PATH": os.environ["PATH"], "PRIVATE_WIF_PROVIDER": a, "PRIVATE_AUDIT_SA": b},
                )
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stdout, "::error::Missing or invalid dedicated audit identity configuration.\n")
                self.assertEqual(result.stderr, "")

    def test_failed_audit_never_prints_private_stderr_or_partial_json(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            fake_go = root / "go"
            fake_go.write_text("#!/bin/bash\necho synthetic-private-output\necho synthetic-private-error >&2\nexit 1\n")
            fake_go.chmod(0o700)
            result = subprocess.run(
                ["bash", "-c", self.step_script("Run aggregate-only audit without public data output")],
                capture_output=True, text=True,
                env={"PATH": directory + os.pathsep + os.environ["PATH"], "RUNNER_TEMP": directory,
                     "GOOGLE_APPLICATION_CREDENTIALS": str(root / "synthetic-credential")},
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn("synthetic-private", result.stdout + result.stderr)
            self.assertIn("synthetic-private-error", (root / "user-activity-schema-audit.stderr").read_text())
            self.assertEqual((root / "user-activity-schema-audit.stderr").stat().st_mode & 0o777, 0o600)
            (root / "gha-creds-synthetic.json").touch()
            subprocess.run(["bash", "-c", self.step_script("Cleanup private audit output")], check=True,
                           env={"PATH": os.environ["PATH"], "RUNNER_TEMP": directory, "GITHUB_WORKSPACE": directory})
            self.assertFalse((root / "user-activity-schema-audit.stderr").exists())
            self.assertFalse((root / "user-activity-schema-audit.json").exists())
            self.assertFalse((root / "gha-creds-synthetic.json").exists())

    def test_caller_exposes_only_explicit_manual_toggle(self) -> None:
        self.assertIn("gcp_user_activity_schema_audit:", self.caller)
        self.assertIn("inputs.gcp_user_activity_schema_audit == true", self.caller)
        self.assertIn("uses: ./.github/workflows/gcp-user-activity-schema-audit.yml", self.caller)
        call = self.caller.split("  gcp-user-activity-schema-audit:\n", 1)[1].split("\n  ci-gate:\n", 1)[0]
        expected = {"GCP_TERRAFORM_WIF_PROVIDER", "GCP_USER_ACTIVITY_SCHEMA_AUDIT_SERVICE_ACCOUNT"}
        declared = self.text.split("    secrets:\n", 1)[1].split("\npermissions:", 1)[0]
        self.assertEqual(set(re.findall(r"^      ([A-Z_]+):$", declared, re.MULTILINE)), expected)
        self.assertEqual(declared.count("required: false"), 2)
        self.assertNotIn("required: true", declared)
        passed = re.findall(r"^      ([A-Z_]+): \$\{\{ secrets\.([A-Z_]+) \}\}$", call, re.MULTILINE)
        self.assertEqual(len(passed), 2)
        self.assertEqual(dict(passed), {name: name for name in expected})
        self.assertEqual(set(re.findall(r"secrets\.([A-Z_]+)", self.text)), expected)
        self.assertNotIn("secrets: inherit", call)
        self.assertIn("target: dev", call)

    def run_guard(self, **overrides: str) -> subprocess.CompletedProcess[str]:
        script = (
            self.text.split("      - name: Enforce trusted read-only audit surface", 1)[1]
            .split("        run: |\n", 1)[1]
            .split("\n      - name: Checkout trusted commit", 1)[0]
        )
        env = {
            "PATH": os.environ["PATH"],
            "GITHUB_EVENT_NAME": "workflow_dispatch",
            "GITHUB_REPOSITORY": "kani3camp/youtube-study-space",
            "GITHUB_REPOSITORY_ID": "340900071",
            "GITHUB_REPOSITORY_OWNER_ID": "54093651",
            "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
            "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac",
            "TARGET": "dev",
            "DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED": "false",
        }
        env.update(overrides)
        return subprocess.run(
            ["bash", "-c", textwrap.dedent(script)],
            env=env,
            capture_output=True,
            text=True,
        )

    def test_guard_rejects_until_enabled_and_rejects_production(self) -> None:
        self.assertNotEqual(self.run_guard().returncode, 0)
        self.assertEqual(
            self.run_guard(DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED="true").returncode,
            0,
        )

        self.assertNotEqual(
            self.run_guard(
                DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED="true",
                TARGET="prod",
            ).returncode,
            0,
        )

    def test_enabled_guard_still_rejects_untrusted_callers(self) -> None:
        for key, value in {
            "GITHUB_EVENT_NAME": "pull_request",
            "GITHUB_REPOSITORY": "synthetic/fork",
            "GITHUB_REPOSITORY_ID": "0",
            "GITHUB_REPOSITORY_OWNER_ID": "0",
            "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/dev",
            "GITHUB_REF": "refs/heads/dev",
        }.items():
            with self.subTest(key=key):
                self.assertNotEqual(self.run_guard(DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED="true",
                                                  **{key: value}).returncode, 0)


class EnvironmentSecretHandoffFixtureTest(unittest.TestCase):
    def probe(self, path: Path) -> str:
        script = path.read_text().split("        run: |\n", 1)[1].split("\n  reusable_", 1)[0]
        return textwrap.dedent(script)

    def test_fixture_probes_classify_without_disclosing_inputs(self) -> None:
        fixtures = list(HANDOFF_FIXTURES.glob("*.yml"))
        self.assertEqual(len(fixtures), 3)
        cases = (
            ({}, ("absent", "absent")),
            ({"SYNTHETIC_A": "synthetic-alpha", "SYNTHETIC_B": "synthetic-beta"}, ("expected", "expected")),
            ({"SYNTHETIC_A": "synthetic-alpha", "SYNTHETIC_B": ""}, ("expected", "absent")),
            ({"SYNTHETIC_A": "synthetic-private\n::error::do-not-print", "SYNTHETIC_B": "synthetic-mismatch"},
             ("unexpected", "unexpected")),
        )
        for path in fixtures:
            for values, states in cases:
                with self.subTest(fixture=path.name, states=states), tempfile.TemporaryDirectory() as directory:
                    summary = Path(directory) / "summary"
                    result = subprocess.run(
                        ["bash", "-c", self.probe(path)], capture_output=True, text=True,
                        env={"PATH": os.environ["PATH"], "GITHUB_STEP_SUMMARY": str(summary), **values},
                    )
                    self.assertEqual(result.returncode, 0)
                    self.assertEqual(result.stdout + result.stderr, "")
                    self.assertEqual(summary.read_text(),
                                     f"- SYNTHETIC_A: {states[0]}\n- SYNTHETIC_B: {states[1]}\n")

    def test_fixture_probes_stop_inherited_trace_before_handling_values(self) -> None:
        for path in HANDOFF_FIXTURES.glob("*.yml"):
            with self.subTest(fixture=path.name), tempfile.TemporaryDirectory() as directory:
                result = subprocess.run(
                    ["bash", "-x", "-c", self.probe(path)], capture_output=True, text=True,
                    env={"PATH": os.environ["PATH"], "GITHUB_STEP_SUMMARY": str(Path(directory) / "summary"),
                         "SYNTHETIC_A": "synthetic-sensitive-input"},
                )
                self.assertEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")
                self.assertEqual(result.stderr, "+ set +x\n")

    def test_fixture_is_inactive_and_contains_no_cloud_auth_or_inherit(self) -> None:
        for path in HANDOFF_FIXTURES.glob("*.yml"):
            with self.subTest(fixture=path.name):
                self.assertNotIn(GITHUB_DIR / "workflows", path.parents)
                text = path.read_text()
                self.assertIn("permissions: {}", text)
                self.assertIn("name: synthetic-secret-handoff", text)
                for forbidden in ("id-token:", "google-github-actions", "checkout@", "upload-artifact",
                                  "secrets: inherit", "go run", "curl ", "wget "):
                    self.assertNotIn(forbidden, text)


class SchemaAuditSummaryTest(unittest.TestCase):
    def test_valid_output_contains_decisions_without_counts(self) -> None:
        payload = audit_fixture()
        result = summary_module.summarize(payload)
        for key in summary_module.COUNTS:
            self.assertIn(f"{key} is zero: no", result)
            self.assertNotIn(str(payload["audit"][key]), result)
        self.assertIn("Legacy timestamp present: yes", result)

    def test_canonical_schema_reports_all_zero(self) -> None:
        payload = audit_fixture()
        payload["audit"].update(canonical=True, has_legacy_timestamp=False)
        for key in summary_module.COUNTS:
            payload["audit"][key] = 0
        result = summary_module.summarize(payload)
        self.assertIn("Legacy timestamp present: no", result)
        self.assertEqual(result.count("is zero: yes"), 4)

    def test_invalid_types_and_inconsistent_counts_fail_closed(self) -> None:
        for key, values in {
            "legacy_non_null": [True, -1, "900001", 900001.0, 2**63, 0],
            "canonical": [1, "false", None, True],
            "has_taken_at": [False, "true"],
            "has_legacy_timestamp": [False, "true"],
        }.items():
            for value in values:
                with self.subTest(key=key, value=value):
                    payload = audit_fixture()
                    payload["audit"][key] = value
                    with self.assertRaises(ValueError):
                        summary_module.summarize(payload)

    def test_unknown_or_missing_fields_and_wrong_target_are_rejected(self) -> None:
        for section in (None, "target", "audit"):
            payload = audit_fixture()
            scope = payload if section is None else payload[section]
            scope["synthetic-private-row-sample"] = "synthetic-private-value"
            with self.assertRaises(ValueError):
                summary_module.summarize(payload)
        for key in ("environment", "project_id", "dataset", "table"):
            payload = audit_fixture()
            payload["target"][key] = "synthetic-wrong-target"
            with self.assertRaises(ValueError):
                summary_module.summarize(payload)
        payload = audit_fixture()
        del payload["audit"]["canonical"]
        with self.assertRaises(ValueError):
            summary_module.summarize(payload)

    def test_invalid_private_json_cannot_reach_public_summary_or_errors(self) -> None:
        payload = audit_fixture()
        extra = copy.deepcopy(payload)
        extra["audit"]["raw_row"] = "synthetic-private-value"
        for raw in (json.dumps(extra), '{"synthetic-private-value":', "x" * 65537,
                    json.dumps(payload).replace('"canonical": false', '"canonical": false, "canonical": true')):
            with self.subTest(raw_length=len(raw)), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                private = root / "private.json"
                public = root / "summary.md"
                private.write_text(raw)
                public.write_text("existing summary\n")
                result = subprocess.run(["python3", str(SUMMARY), str(private), str(public)],
                                        capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(public.read_text(), "existing summary\n")
                self.assertNotIn("synthetic-private-value", result.stdout + result.stderr)
                self.assertNotIn(str(private), result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
