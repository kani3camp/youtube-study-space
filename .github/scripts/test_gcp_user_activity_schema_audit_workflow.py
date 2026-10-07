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
ADOPTION = GITHUB_DIR.parent / "infra/gcp/scripts/prepare_user_activity_history_adoption.py"
adoption_spec = importlib.util.spec_from_file_location("user_activity_adoption", ADOPTION)
adoption_module = importlib.util.module_from_spec(adoption_spec)
adoption_spec.loader.exec_module(adoption_module)


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


class UserActivityAdoptionPreparationTest(unittest.TestCase):
    def metadata(self) -> dict:
        return {
            "tableReference": dict(adoption_module.REFERENCE),
            "type": "TABLE", "location": adoption_module.LOCATION,
            "schema": {"fields": json.loads(adoption_module.SCHEMA.read_text())},
        }

    def test_observed_order_is_preserved_without_enabling_adoption(self) -> None:
        metadata = self.metadata()
        metadata["schema"]["fields"].reverse()
        result = adoption_module.prepare(metadata)
        self.assertIs(result["manage_user_activity_history"], False)
        self.assertEqual(result["user_activity_history_field_order"],
                         [field["name"] for field in metadata["schema"]["fields"]])

    def test_empty_legacy_values_do_not_make_existing_column_canonical(self) -> None:
        audit = audit_fixture()
        for key in summary_module.COUNTS:
            audit["audit"][key] = 0
        summary = summary_module.summarize(audit)
        self.assertIn("Legacy timestamp present: yes", summary)
        self.assertEqual(summary.count("is zero: yes"), 4)
        metadata = self.metadata()
        metadata["schema"]["fields"].insert(3, {"name": "timestamp", "type": "TIMESTAMP", "mode": "NULLABLE"})
        with self.assertRaises(ValueError):
            adoption_module.prepare(metadata)

    def test_missing_duplicate_or_unknown_fields_fail_closed(self) -> None:
        for change in (lambda fields: fields.pop(), lambda fields: fields.append(copy.deepcopy(fields[0])),
                       lambda fields: fields[0].update(name="synthetic-unexpected")):
            metadata = self.metadata()
            change(metadata["schema"]["fields"])
            with self.assertRaises(ValueError):
                adoption_module.prepare(metadata)

    def test_nested_modes_types_order_and_annotations_are_not_discarded(self) -> None:
        for change in (
            lambda fields: fields[1].update(type="STRING"),
            lambda fields: fields[0].update(mode="REQUIRED"),
            lambda fields: fields[5]["fields"].reverse(),
            lambda fields: fields[5]["fields"][0].update(mode="REQUIRED"),
            lambda fields: fields[0].update(description="synthetic-private-description"),
            lambda fields: fields[0].update(policyTags={"names": ["synthetic-private-policy"]}),
        ):
            metadata = self.metadata()
            change(metadata["schema"]["fields"])
            with self.assertRaises(ValueError):
                adoption_module.prepare(metadata)

    def test_equivalent_api_aliases_and_nullable_defaults_are_accepted(self) -> None:
        metadata = self.metadata()
        metadata["schema"]["fields"][0].update(type="INT64", description="")
        del metadata["schema"]["fields"][0]["mode"]
        metadata["schema"]["fields"][4]["type"] = "BOOL"
        metadata["schema"]["fields"][5]["type"] = "STRUCT"
        self.assertIs(adoption_module.prepare(metadata)["manage_user_activity_history"], False)

    def test_wrong_scope_and_unmodeled_table_configuration_are_rejected(self) -> None:
        for key, value in ("location", "synthetic-other-region"), ("type", "VIEW"):
            metadata = self.metadata()
            metadata[key] = value
            with self.assertRaises(ValueError):
                adoption_module.prepare(metadata)
        for key in adoption_module.REFERENCE:
            metadata = self.metadata()
            metadata["tableReference"][key] = "synthetic-wrong-target"
            with self.assertRaises(ValueError):
                adoption_module.prepare(metadata)
        for key, value in {
            "clustering": {"fields": ["user_id"]}, "timePartitioning": {"type": "DAY"},
            "expirationTime": "12345", "encryptionConfiguration": {"kmsKeyName": "synthetic-key"},
            "labels": {"synthetic": "value"}, "description": "synthetic-private-description",
            "tableConstraints": {}, "requirePartitionFilter": True,
        }.items():
            metadata = self.metadata()
            metadata[key] = value
            with self.assertRaises(ValueError):
                adoption_module.prepare(metadata)

    def run_cli(self, path: Path, output: Path):
        return subprocess.run(["python3", str(ADOPTION), str(path), str(output)],
                              capture_output=True, text=True)

    def test_cli_writes_private_default_off_input_and_never_overwrites(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, output = root / "private.json", root / "candidate.tfvars.json"
            source.write_text(json.dumps(self.metadata()))
            source.chmod(0o600)
            result = self.run_cli(source, output)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)
            self.assertIs(json.loads(output.read_text())["manage_user_activity_history"], False)
            before = output.read_bytes()
            self.assertNotEqual(self.run_cli(source, output).returncode, 0)
            self.assertEqual(output.read_bytes(), before)

    def test_invalid_or_exposed_private_inputs_never_echo_or_create_candidate(self) -> None:
        for case in ("legacy", "duplicate-json", "malformed", "exposed", "symlink"):
            with self.subTest(case=case), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                source, output = root / "synthetic-private-input.json", root / "candidate.json"
                metadata = self.metadata()
                metadata["id"] = "synthetic-private-value"
                if case == "legacy":
                    metadata["schema"]["fields"].append({"name": "timestamp", "type": "TIMESTAMP"})
                raw = json.dumps(metadata)
                if case == "duplicate-json":
                    raw = raw.replace('"type": "TABLE"', '"type": "TABLE", "type": "TABLE"')
                elif case == "malformed":
                    raw = '{"synthetic-private-value":'
                source.write_text(raw)
                source.chmod(0o600 if case != "exposed" else 0o644)
                if case == "symlink":
                    link = root / "link.json"
                    link.symlink_to(source)
                    source = link
                result = self.run_cli(source, output)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(output.exists())
                self.assertNotIn("synthetic-private-value", result.stdout + result.stderr)
                self.assertNotIn(str(source), result.stdout + result.stderr)


class UserActivityHistoryPlanTest(unittest.TestCase):
    def setUp(self) -> None:
        path = ADOPTION.with_name("validate_user_activity_history_plan.py")
        spec = importlib.util.spec_from_file_location("history_plan", path)
        self.gate = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.gate)

    def fixture(self, importing: bool = True) -> tuple[dict, dict]:
        from test_terraform_export_function_gate import fixture, EMAIL
        self.email = EMAIL
        metadata = UserActivityAdoptionPreparationTest().metadata()
        fields = metadata["schema"]["fields"]
        # A valid alternate live top-level order must be preserved, not sorted.
        fields[0], fields[1] = fields[1], fields[0]
        value = {"id": self.gate.TABLE_ID, "project": "test-youtube-study-space", "dataset_id": "firestore_export",
                 "table_id": "user-activity-history", "location": "asia-southeast2", "deletion_protection": True,
                 "schema": json.dumps(fields), "labels": {}}
        change = {"actions": ["no-op"], "before": value, "after": copy.deepcopy(value), "after_unknown": {}}
        if importing:
            change["importing"] = {"id": self.gate.TABLE_ID}
        plan = fixture(False)
        plan["resource_changes"].append({"address": self.gate.TABLE, "mode": "managed", "type": "google_bigquery_table",
                                         "provider_name": "registry.terraform.io/hashicorp/google", "change": change})
        return plan, metadata

    def review(self, plan: dict, metadata: dict, phase: str = "before") -> None:
        self.gate.validate(plan, metadata=metadata, phase=phase, execution_email=self.email)

    def test_exact_import_and_post_plan_preserve_existing_eleven_and_field_order(self) -> None:
        for phase, importing in (("before", True), ("post", False)):
            plan, metadata = self.fixture(importing)
            self.review(plan, metadata, phase)
        plan, metadata = self.fixture(False)
        with self.assertRaises(ValueError):
            self.review(plan, metadata)
        plan, metadata = self.fixture()
        with self.assertRaises(ValueError):
            self.review(plan, metadata, "post")

    def test_graph_import_identity_and_planning_uncertainty_fail_closed(self) -> None:
        mutations = (
            lambda p: p["resource_changes"].pop(0),
            lambda p: p["resource_changes"].append(copy.deepcopy(p["resource_changes"][-1])),
            lambda p: p["resource_changes"][-1].update(address="google_bigquery_table.unapproved"),
            lambda p: p["resource_changes"][-1].update(provider_name="registry.terraform.io/example/other"),
            lambda p: p["resource_changes"][-1]["change"]["importing"].update(id="synthetic-private-wrong-target"),
            lambda p: p["resource_changes"][0]["change"].update(importing={"id": "synthetic-second-import"}),
            lambda p: p["resource_changes"][-1]["change"].update(after_unknown={"schema": True}),
            lambda p: p.update(resource_drift=[{}]), lambda p: p.update(deferred_changes=[{}]),
            lambda p: p.update(complete=False), lambda p: p.update(errored=True),
            lambda p: p.update(checks=[{"status": "unknown"}]),
            lambda p: p["output_changes"]["project_id"].update(actions=["update"]),
        )
        for mutate in mutations:
            plan, metadata = self.fixture()
            mutate(plan)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                self.review(plan, metadata)

    def test_schema_repair_and_other_resource_changes_cannot_be_hidden_in_import(self) -> None:
        for index in (0, -1):
            for actions in (["update"], ["create"], ["delete"], ["delete", "create"]):
                plan, metadata = self.fixture()
                plan["resource_changes"][index]["change"]["actions"] = actions
                with self.subTest(index=index, actions=actions), self.assertRaises(ValueError):
                    self.review(plan, metadata)
        plan, metadata = self.fixture()
        before = plan["resource_changes"][-1]["change"]["before"]
        fields = json.loads(before["schema"])
        fields.append({"name": "timestamp", "type": "TIMESTAMP"})
        before["schema"] = json.dumps(fields)
        with self.assertRaises(ValueError):
            self.review(plan, metadata)

    def test_separate_action_invocations_are_rejected_before_and_after_import(self) -> None:
        for phase, importing in (("before", True), ("post", False)):
            plan, metadata = self.fixture(importing)
            plan.update(action_invocations=[], deferred_action_invocations=[])
            self.review(plan, metadata, phase)
            for key in ("action_invocations", "deferred_action_invocations"):
                for value in ([{"address": "action.synthetic", "config_values": {"token": "synthetic-private-value"}}],
                              {"unexpected": "synthetic-private-value"}, None, False):
                    plan, metadata = self.fixture(importing)
                    plan[key] = value
                    with self.subTest(phase=phase, key=key, value=value), self.assertRaises(ValueError):
                        self.review(plan, metadata, phase)

    def test_legacy_nested_and_order_differences_are_rejected_even_if_noop(self) -> None:
        for mutate in (
            lambda f: f.append({"name": "timestamp", "type": "TIMESTAMP"}),
            lambda f: f.reverse(),
            lambda f: f[5]["fields"][0].update(mode="REQUIRED"),
            lambda f: f[0].update(description="synthetic-private-description"),
        ):
            plan, metadata = self.fixture()
            for side in ("before", "after"):
                value = plan["resource_changes"][-1]["change"][side]
                fields = json.loads(value["schema"])
                mutate(fields)
                value["schema"] = json.dumps(fields)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                self.review(plan, metadata)
        plan, metadata = self.fixture()
        metadata["schema"]["fields"].append({"name": "timestamp", "type": "TIMESTAMP"})
        with self.assertRaises(ValueError):
            self.review(plan, metadata)

    def test_move_only_state_changes_are_rejected_for_every_resource(self) -> None:
        for phase, importing in (("before", True), ("post", False)):
            for index in range(12):
                plan, metadata = self.fixture(importing)
                plan["resource_changes"][index]["previous_address"] = "module.synthetic-private-value.old"
                with self.subTest(phase=phase, index=index), self.assertRaises(ValueError):
                    self.review(plan, metadata, phase)

    def test_foreign_table_configuration_and_disabled_protection_are_rejected(self) -> None:
        for key, value in {"project": "synthetic-prod", "location": "synthetic-other-region", "deletion_protection": False,
                           "expiration_time": 12345, "table_constraints": [{"primary_key": ["user_id"]}],
                           "time_partitioning": [{"type": "DAY"}], "labels": {"private": "synthetic-private-value"}}.items():
            plan, metadata = self.fixture()
            for side in ("before", "after"):
                plan["resource_changes"][-1]["change"][side][key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                self.review(plan, metadata)

    def test_cli_is_private_offline_and_never_echoes_input_or_path(self) -> None:
        script = ADOPTION.with_name("validate_user_activity_history_plan.py")
        for failure in (None, "metadata-exposed", "plan-symlink", "plan-duplicate", "plan-malformed", "plan-move", "plan-action",
                        "plan-deferred-action", "wrong-target", "no-identity", "bad-argument"):
            plan, metadata = self.fixture()
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / "synthetic-private-path"
                root.mkdir(mode=0o700)
                paths = root / "metadata.json", root / "plan.json"
                for path, payload in zip(paths, (metadata, plan)):
                    path.write_text(json.dumps(payload)); path.chmod(0o600)
                if failure == "metadata-exposed": paths[0].chmod(0o644)
                if failure == "plan-symlink":
                    actual = root / "actual.json"; paths[1].rename(actual); paths[1].symlink_to(actual)
                if failure == "plan-duplicate": paths[1].write_text('{"complete":true,"complete":true}')
                if failure == "plan-malformed": paths[1].write_text('{"synthetic-private-value":')
                if failure == "plan-move":
                    plan["resource_changes"][0]["previous_address"] = "module.synthetic-private-value.old"
                    paths[1].write_text(json.dumps(plan))
                if failure in ("plan-action", "plan-deferred-action"):
                    key = "action_invocations" if failure == "plan-action" else "deferred_action_invocations"
                    plan[key] = [{"address": "action.synthetic", "config_values": {"token": "synthetic-private-value"}}]
                    paths[1].write_text(json.dumps(plan))
                if failure == "wrong-target":
                    metadata["tableReference"]["projectId"] = "synthetic-private-value"
                    paths[0].write_text(json.dumps(metadata))
                args = ["python3", str(script), "--phase", "before", "--metadata", str(paths[0]), "--plan", str(paths[1])]
                if failure == "bad-argument": args.extend(["--unknown", "synthetic-private-value"])
                env = dict(os.environ, TF_VAR_export_function_execution_service_account_email="" if failure == "no-identity" else self.email)
                result = subprocess.run(args, env=env, capture_output=True, text=True)
                self.assertEqual(result.returncode, 0 if failure is None else 3)
                for secret in (self.email, str(root), "synthetic-private-value", self.gate.TABLE_ID):
                    self.assertNotIn(secret, result.stdout + result.stderr)
                if failure is None:
                    self.assertIn("Execution remains unapproved", result.stdout)


if __name__ == "__main__":
    unittest.main()
