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


def configured_hints(*, post=False):
    value = plan(post=post)
    config = value["configuration"]
    config["provider_config"] = {"google": {"full_name": gate.contract.GOOGLE}}
    config["root_module"]["module_calls"] = calls = {}
    entries = []
    schemas = {}
    for address, (module, source, kind, name, repeat, output, literals) in gate.contract.INACTIVE_REFERENCES.items():
        local = kind + "." + name
        calls[module] = {"source": source, "module": {
            "resources": [{"address": local, "mode": "managed", "type": kind, "name": name,
                "provider_config_key": "google", "schema_version": 0,
                repeat: {"references": ["var.disabled"]}}],
            "outputs": {output: {"expression": {"references": list(literals)}}}}}
        entries.append({"resource": address, "attribute": ["name"],
                        "extra": {"ordered": [CANARY, True, 1]}})
        schemas[kind] = {"version": 0, "block": {"attributes": {}}}
    value["relevant_attributes"] = entries
    return value, {"format_version": "1.0", "provider_schemas": {gate.contract.GOOGLE: {"resource_schemas": schemas}}}


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


class InitializerConfiguredHintTest(unittest.TestCase):
    def payload(self, value, schema):
        return gate.plan_commitment_payload(value, manifest=manifest(), environment="prod", provider_schema=schema)

    def test_empty_initial_operation_accepts_only_static_catalog_hints_without_fabricating_state(self):
        value, schema = configured_hints()
        original = deepcopy(value)
        result = gate.validate_plan(value, manifest=manifest(), environment="prod", provider_schema=schema)
        self.assertEqual((result["managed"], result["imports"], result["output_creates"]), (0, 0, 2))
        self.assertTrue(value == original)
        normal = deepcopy(value)
        for change in normal["output_changes"].values():
            change["actions"] = ["no-op"]
            change["before"] = change["after"]
        # The ordinary StateNone contract is intentionally still closed.
        with self.assertRaises(gate.contract.ContractError) as blocked:
            gate.contract.validate_plan(normal, manifest=manifest(), environment="prod", provider_schema=schema)
        self.assertEqual(blocked.exception.reason, "SHAPE")
        summary = gate.contract.validate_plan(normal, manifest=manifest(), environment="prod", provider_schema=schema,
                                               state=state(), phase="post")
        self.assertEqual(summary["managed"], 0)

    def test_initial_hint_commitment_sorts_whole_entries_and_retains_all_contents_and_presence(self):
        first, schema = configured_hints()
        first["relevant_attributes"].append(deepcopy(first["relevant_attributes"][0]))
        second = deepcopy(first)
        second["relevant_attributes"].reverse()
        payload = self.payload(first, schema)
        self.assertTrue(gate.contract.canonical(payload) == gate.contract.canonical(self.payload(second, schema)))
        hints = payload["initial_relevant_attributes"]
        self.assertTrue(hints["present"])
        self.assertEqual(len(hints["entries"]), 4)
        self.assertTrue(any(CANARY in str(entry.get("extra")) for entry in hints["entries"]))
        self.assertNotIn("relevant_attributes", payload["validated_plan"])
        for edit in (
            lambda p: p["relevant_attributes"].pop(),
            lambda p: p["relevant_attributes"][0].update(attribute=["labels", 0]),
            lambda p: p["relevant_attributes"][0]["extra"].update(ordered=[CANARY, 1, True]),
        ):
            changed = deepcopy(first)
            edit(changed)
            self.assertTrue(gate.contract.canonical(payload) != gate.contract.canonical(self.payload(changed, schema)))
        absent = deepcopy(first)
        absent.pop("relevant_attributes")
        empty = deepcopy(first)
        empty["relevant_attributes"] = []
        self.assertTrue(self.payload(absent, schema)["initial_relevant_attributes"] == {"present": False, "entries": []})
        self.assertTrue(self.payload(empty, schema)["initial_relevant_attributes"] == {"present": True, "entries": []})
        self.assertTrue(gate.contract.canonical(self.payload(absent, schema)) != gate.contract.canonical(self.payload(empty, schema)))

    def test_hint_path_order_and_unknown_entry_fields_are_never_discarded(self):
        first, schema = configured_hints()
        first["relevant_attributes"][0]["attribute"] = ["labels", "name"]
        payload = self.payload(first, schema)
        changed = deepcopy(first)
        changed["relevant_attributes"][0]["attribute"].reverse()
        self.assertTrue(gate.contract.canonical(payload) != gate.contract.canonical(self.payload(changed, schema)))
        changed = deepcopy(first)
        changed["relevant_attributes"][0]["unknown"] = {"retained": CANARY}
        self.assertTrue(gate.contract.canonical(payload) != gate.contract.canonical(self.payload(changed, schema)))

    def test_unknown_or_malformed_hint_and_static_source_tampering_reject(self):
        edits = (
            lambda p: p.update(relevant_attributes=None),
            lambda p: p.update(relevant_attributes={}),
            lambda p: p["relevant_attributes"][0].update(resource="module.foreign.google_monitoring_alert_policy.quota"),
            lambda p: p["relevant_attributes"][0].update(attribute=[True]),
            lambda p: p["relevant_attributes"][0].update(attribute=[-1]),
            lambda p: p["relevant_attributes"][0].update(attribute=[{}]),
            lambda p: p["configuration"]["provider_config"]["google"].update(full_name="foreign/provider"),
            lambda p: p["configuration"]["root_module"]["module_calls"]["youtube_quota_alerts"].update(source="../../foreign"),
            lambda p: p["configuration"]["root_module"]["module_calls"]["youtube_quota_alerts"].update(count_expression={"constant_value": 1}),
            lambda p: p["configuration"]["root_module"]["module_calls"]["youtube_quota_alerts"]["module"]["outputs"]["policy_names"].update(expression={"references": ["foreign.resource"]}),
            lambda p: p["planned_values"]["root_module"].update(resources=[{}]),
            lambda p: p.update(prior_state={"format_version": "1.0", "values": {"root_module": {"resources": [{}]}}}),
        )
        for number, edit in enumerate(edits):
            value, schema = configured_hints()
            edit(value)
            with self.subTest(case=number), self.assertRaises((gate.contract.ContractError, ValueError, TypeError, KeyError)):
                self.payload(value, schema)


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


class NativeFullProductionInitializerTest(unittest.TestCase):
    def test_real_default_prod_root_has_only_two_outputs_and_bound_inactive_hints(self):
        binary = shutil.which("terraform")
        self.assertIsNotNone(binary)
        repository = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory(prefix="production-init-full-root-") as directory:
            root = Path(directory)
            checkout = root / "checkout"
            names = subprocess.run(["git", "ls-files", "infra/gcp"], cwd=repository,
                capture_output=True, text=True, stdin=subprocess.DEVNULL, timeout=30)
            self.assertEqual(names.returncode, 0)
            for name in names.stdout.splitlines():
                relative = Path(name)
                if relative.suffix not in {".tf", ".tftpl"} and relative.name not in {".terraform.lock.hcl", "schema.json"}:
                    continue
                source = repository / relative
                self.assertFalse(source.is_symlink())
                destination = checkout / relative
                destination.parent.mkdir(parents=True, exist_ok=True)
                destination.write_bytes(source.read_bytes())
            tfroot = checkout / "infra/gcp/environments/prod"
            main = tfroot / "main.tf"
            self.assertEqual(main.read_text().count('backend "s3" {}'), 1)
            main.write_text(main.read_text().replace('backend "s3" {}', ""))
            cli = root / "terraform.rc"
            cli_text = "disable_checkpoint = true\n"
            mirror = os.environ.get("GCP_TEST_PROVIDER_MIRROR")
            if mirror:
                self.assertTrue(Path(mirror).is_absolute() and Path(mirror).is_dir())
                cli_text += 'provider_installation { filesystem_mirror { path = ' + json.dumps(mirror) + ' } }\n'
            cli.write_text(cli_text)
            clean = {"PATH": str(Path(binary).parent) + os.pathsep + os.defpath,
                     "HOME": str(root), "TMPDIR": str(root), "TF_IN_AUTOMATION": "true",
                     "TF_INPUT": "false", "TF_WORKSPACE": "default", "CHECKPOINT_DISABLE": "1",
                     "TF_CLI_CONFIG_FILE": str(cli), "GOOGLE_OAUTH_ACCESS_TOKEN": "unused-native-zero-resource-fixture"}
            def command(*args):
                result = subprocess.run([binary, "-chdir=" + str(tfroot), *args], env=clean,
                    capture_output=True, stdin=subprocess.DEVNULL, timeout=120)
                self.assertEqual(result.returncode, 0, "Full production initializer fixture failed; raw output suppressed.")
                return json.loads(result.stdout) if args[0] in {"show", "providers"} else result.stdout
            # Init may download only the locked public provider. After init every
            # HTTP(S) proxy points to a refused loopback port; cloud egress is denied.
            command("init", "-input=false", "-lockfile=readonly")
            clean.update(HTTP_PROXY="http://127.0.0.1:1", HTTPS_PROXY="http://127.0.0.1:1",
                         ALL_PROXY="http://127.0.0.1:1", http_proxy="http://127.0.0.1:1",
                         https_proxy="http://127.0.0.1:1", all_proxy="http://127.0.0.1:1",
                         NO_PROXY="127.0.0.1,localhost", no_proxy="127.0.0.1,localhost")
            schema = command("providers", "schema", "-json")
            plans = []
            for number in range(2):
                command("plan", "-input=false", "-out=before" + str(number) + ".plan")
                plans.append(command("show", "-json", "before" + str(number) + ".plan"))
            self.assertFalse((tfroot / "terraform.tfstate").exists())
            first = gate.plan_commitment_payload(plans[0], manifest=manifest(), environment="prod", provider_schema=schema)
            second = gate.plan_commitment_payload(plans[1], manifest=manifest(), environment="prod", provider_schema=schema)
            self.assertTrue(gate.contract.canonical(first) == gate.contract.canonical(second))
            self.assertEqual(len(first["initial_relevant_attributes"]["entries"]), 3)
            summary = gate.validate_plan(plans[0], manifest=manifest(), environment="prod", provider_schema=schema)
            self.assertEqual((summary["managed"], summary["imports"], summary["output_creates"]), (0, 0, 2))
            command("apply", "-input=false", "before0.plan")
            after = json.loads((tfroot / "terraform.tfstate").read_bytes())
            gate.validate_state_delta(None, after, manifest=manifest(), environment="prod", provider_schema=schema, plan=plans[0])
            before_post = (tfroot / "terraform.tfstate").read_bytes()
            command("plan", "-input=false", "-out=post.plan")
            post = gate.normalize_empty_plan(command("show", "-json", "post.plan"), manifest())
            result = gate.contract.validate_plan(post, manifest=manifest(), environment="prod", provider_schema=schema, state=after, phase="post")
            self.assertEqual((result["managed"], result["imports"], result["raw_observation_drift"]), (0, 0, 0))
            self.assertTrue((tfroot / "terraform.tfstate").read_bytes() == before_post)


if __name__ == "__main__":
    unittest.main()
