#!/usr/bin/env python3
"""Unconnected output-only adapter: pure negatives and a native local empty root."""
from __future__ import annotations

from copy import deepcopy
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
import production_initializer as gate

CANARY = "PRIVATE_INITIALIZER_CANARY"
OUTPUTS = {"environment": "production", "project_id": "youtube-study-space"}


def manifest(environment="prod"):
    return {"schema_version": "gcp-migration-manifest-v1", "environment": environment,
            "project_id": gate.contract.ENVIRONMENTS[environment][0], "resources": []}


def plan(*, post=False):
    return {"format_version": "1.2", "terraform_version": gate.contract.TERRAFORM_VERSION,
            "complete": True, "errored": False, "applyable": not post,
            "configuration": {"root_module": {}}, "variables": {"fixture": {"value": CANARY}},
            "resource_changes": [], "resource_drift": [], "checks": [],
            "planned_values": {"root_module": {}, "outputs": {
                key: {"type": "string", "value": value, "sensitive": False} for key, value in OUTPUTS.items()}},
            "output_changes": {key: {"actions": ["no-op"] if post else ["create"],
                "before": value if post else None, "after": value,
                "before_sensitive": False, "after_sensitive": False, "after_unknown": False}
                for key, value in OUTPUTS.items()}}


def state():
    return {"version": 4, "terraform_version": gate.contract.TERRAFORM_VERSION,
            "serial": 1, "lineage": "12345678-1234-1234-1234-123456789abc", "resources": [],
            "outputs": {key: {"type": "string", "value": value} for key, value in OUTPUTS.items()},
            "check_results": None}


class InitializerTest(unittest.TestCase):
    def validate(self, value=None, **kwargs):
        return gate.validate_plan(plan() if value is None else value, manifest=manifest(),
                                  environment="prod", provider_schema={}, **kwargs)

    def rejected(self, edit):
        value = plan()
        edit(value)
        with self.assertRaises((gate.contract.ContractError, ValueError, TypeError, KeyError)):
            self.validate(value)

    def test_selection_requires_exact_positive_production_absence_and_empty_manifest(self):
        self.assertTrue(gate.selected("prod", manifest(), deepcopy(gate.ABSENT)))
        self.assertFalse(gate.selected("dev", manifest("dev"), deepcopy(gate.ABSENT)))
        for snapshot in ({"state": None}, {}, {"head": None, "raw": None, "state": None, "error": "403"},
                         {"head": {}, "raw": None, "state": None},
                         {"head": {}, "raw": "private", "state": state()}):
            with self.subTest(fields=list(snapshot)):
                self.assertFalse(gate.selected("prod", manifest(), snapshot))
        extra = manifest()
        extra["resources"] = [{"address": "google_pubsub_topic.fixture", "type": "google_pubsub_topic",
                               "import_id": "projects/youtube-study-space/topics/fixture"}]
        self.assertFalse(gate.selected("prod", extra, deepcopy(gate.ABSENT)))

    def test_only_two_fixed_output_creates_are_accepted_and_summary_has_no_values(self):
        result = self.validate()
        self.assertTrue(result["production_state_initialization"])
        self.assertEqual((result["managed"], result["imports"], result["mutations"], result["output_creates"]), (0, 0, 0, 2))
        self.assertNotIn(CANARY, json.dumps(result))
        with self.assertRaises(gate.contract.ContractError):
            gate.contract.validate_plan(plan(), manifest=manifest(), environment="prod", provider_schema={})
        with self.assertRaises(gate.contract.ContractError):
            self.validate(state=state())
        with self.assertRaises(gate.contract.ContractError):
            gate.validate_plan(plan(), manifest=manifest("dev"), environment="dev", provider_schema={})

    def test_output_values_actions_masks_and_extra_fields_are_not_generalized(self):
        edits = (
            lambda value: value["output_changes"]["environment"].update(after="development"),
            lambda value: value["output_changes"]["environment"].update(before="production"),
            lambda value: value["output_changes"]["environment"].update(actions=["no-op"]),
            lambda value: value["output_changes"]["environment"].update(after_unknown=True),
            lambda value: value["output_changes"]["environment"].update(after_sensitive=True),
            lambda value: value["output_changes"]["project_id"].update(after="foreign-project"),
            lambda value: value["output_changes"].update(extra=deepcopy(value["output_changes"]["project_id"])),
            lambda value: value["planned_values"]["outputs"]["project_id"].update(type="number"),
            lambda value: value.update(unreviewed_execution={}),
        )
        for number, edit in enumerate(edits):
            with self.subTest(case=number):
                self.rejected(edit)

    def test_resources_imports_drift_unknowns_and_deferred_actions_are_rejected(self):
        for field in ("resource_changes", "resource_drift", "deferred_changes", "action_invocations", "deferred_action_invocations"):
            with self.subTest(field=field):
                self.rejected(lambda value: value.update({field: [{"change": {"actions": ["create"], "importing": {"id": CANARY}}}]}))
        self.rejected(lambda value: value["planned_values"]["root_module"].update(resources=[{"address": CANARY}]))
        self.rejected(lambda value: value.update(checks=[{"status": "unknown"}]))
        self.rejected(lambda value: value.update(complete=False))
        self.rejected(lambda value: value.update(errored=True))

    def test_prior_state_cannot_hide_an_existing_resource(self):
        self.rejected(lambda value: value.update(prior_state={"format_version": "1.0", "values": {
            "outputs": value["planned_values"]["outputs"], "root_module": {"resources": [{"address": CANARY}]}}}))

    def test_initialization_commitment_keeps_a_distinct_operation_and_private_inputs(self):
        payload = gate.plan_commitment_payload(plan(), manifest=manifest(), environment="prod", provider_schema={})
        self.assertEqual(payload["operation"], "production-state-initialize-v1")
        self.assertEqual(payload["initial_output_changes"], plan()["output_changes"])
        self.assertIn(CANARY, json.dumps(payload))  # Private HMAC input, never public output.
        self.assertEqual(payload["validated_plan"]["variables"], plan()["variables"])

    def test_state_delta_requires_new_empty_state_exact_outputs_and_valid_native_shape(self):
        result = gate.validate_state_delta(None, state(), manifest=manifest(), environment="prod", provider_schema={}, plan=plan())
        self.assertEqual((result["managed_after"], result["imports"], result["output_creates"]), (0, 0, 2))
        for edit in (
            lambda value: value.update(serial=True),
            lambda value: value.update(lineage="invalid"),
            lambda value: value["outputs"]["project_id"].update(value="foreign"),
            lambda value: value.update(resources=[{}]),
            lambda value: value.update(check_results=[{"status": "unknown"}]),
            lambda value: value.update(unreviewed_state={}),
        ):
            candidate = state()
            edit(candidate)
            with self.subTest(edit=edit), self.assertRaises((gate.contract.ContractError, ValueError, TypeError, KeyError)):
                gate.validate_state_delta(None, candidate, manifest=manifest(), environment="prod", provider_schema={}, plan=plan())
        with self.assertRaises(gate.contract.ContractError):
            gate.validate_state_delta(state(), state(), manifest=manifest(), environment="prod", provider_schema={}, plan=plan())

    def test_native_zero_resource_omission_is_normalized_without_hiding_any_changes(self):
        value = plan(post=True)
        value.pop("resource_changes")
        normalized = gate.normalize_empty_plan(value, manifest())
        self.assertNotIn("resource_changes", value)
        self.assertEqual(normalized["resource_changes"], [])
        result = gate.contract.validate_plan(normalized, manifest=manifest(), environment="prod", provider_schema={}, state=state(), phase="post")
        self.assertEqual((result["managed"], result["imports"], result["raw_observation_drift"]), (0, 0, 0))


class NativeLocalInitializerTest(unittest.TestCase):
    def test_local_empty_root_initializes_only_outputs_and_finishes_with_normal_noop(self):
        binary = shutil.which("terraform")
        self.assertIsNotNone(binary)
        clean = {"PATH": str(Path(binary).parent) + os.pathsep + os.defpath,
                 "TF_IN_AUTOMATION": "true", "TF_INPUT": "false", "CHECKPOINT_DISABLE": "1"}
        with tempfile.TemporaryDirectory(prefix="production-init-local-contract-") as directory:
            root = Path(directory)
            cli = root / "terraform.rc"
            cli.write_text("disable_checkpoint = true\n")
            clean["TF_CLI_CONFIG_FILE"] = str(cli)
            (root / "main.tf").write_text(
                'terraform { required_version = "= ' + gate.contract.TERRAFORM_VERSION + '" }\n'
                'variable "project_id" { default = "youtube-study-space" }\n'
                'output "environment" { value = "production" }\n'
                'output "project_id" { value = var.project_id }\n')

            def command(*args):
                result = subprocess.run([binary, "-chdir=" + directory, *args],
                    env=clean, stdin=subprocess.DEVNULL, capture_output=True, timeout=60)
                self.assertEqual(result.returncode, 0, "Native local initializer fixture failed; raw output suppressed.")
                return result.stdout

            command("init", "-backend=false", "-input=false")
            command("plan", "-input=false", "-out=before.plan")
            before = json.loads(command("show", "-json", "before.plan"))
            result = gate.validate_plan(before, manifest=manifest(), environment="prod", provider_schema={})
            self.assertEqual(result["output_creates"], 2)
            command("apply", "-input=false", "before.plan")
            after = json.loads((root / "terraform.tfstate").read_bytes())
            gate.validate_state_delta(None, after, manifest=manifest(), environment="prod", provider_schema={}, plan=before)
            command("plan", "-input=false", "-out=post.plan")
            post = gate.normalize_empty_plan(json.loads(command("show", "-json", "post.plan")), manifest())
            summary = gate.contract.validate_plan(post, manifest=manifest(), environment="prod", provider_schema={}, state=after, phase="post")
            self.assertEqual((summary["managed"], summary["imports"], summary["raw_observation_drift"]), (0, 0, 0))
            self.assertFalse(gate.selected("prod", manifest(), {"head": {"VersionId": "fixture"},
                                                              "raw": json.dumps(after), "state": after}))


if __name__ == "__main__":
    unittest.main()
