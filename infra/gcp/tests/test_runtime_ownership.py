#!/usr/bin/env python3
"""Dummy-only offline inventory, private-file and Terraform ownership contracts."""
from __future__ import annotations

import copy
import importlib.util
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "infra/gcp/scripts/prepare_runtime_ownership.py"
sys.path.insert(0, str(ROOT / ".github/scripts"))
from terraform_plan_summary import build_summary

spec = importlib.util.spec_from_file_location("prepare_runtime_ownership", SCRIPT)
prepare_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare_module)
# Exercise the fixed development contract with an invented project in tests.
prepare_module.PROJECT = "example-study-space-dev"


def cli_command(source: Path, output: Path) -> list[str]:
    code = (f"import sys; sys.path.insert(0, {str(SCRIPT.parent)!r}); "
            "import prepare_runtime_ownership as p; p.PROJECT = 'example-study-space-dev'; "
            "sys.exit(p.main())")
    return [sys.executable, "-c", code, str(source), str(output)]


def inventory() -> dict:
    # These are deliberately invented identities, never live inventory.
    number, account = "111111111111", "222222222222"
    pool = f"projects/{number}/locations/global/workloadIdentityPools/aws-runtime"
    email = "example-study-space-dev@appspot.gserviceaccount.com"
    prefix = f"principalSet://iam.googleapis.com/{pool}/attribute.aws_role/"
    condition = {"title": "dummy-condition", "description": "dummy private sentinel", "expression": "request.time < timestamp('2030-01-01T00:00:00Z')"}
    return {
        "project": {"projectId": "example-study-space-dev", "projectNumber": number},
        "pool": {"name": pool, "state": "ACTIVE", "displayName": "dummy pool", "description": "dummy private sentinel", "mode": "FEDERATION_ONLY"},
        "pool_attestation_rules": {},
        "provider": {"name": f"{pool}/providers/aws-provider", "state": "ACTIVE", "aws": {"accountId": account}, "attributeMapping": {"google.subject": "assertion.account + ':' + assertion.userid.split(':')[0]", "attribute.account": "assertion.account", "attribute.aws_role": "assertion.arn.extract('assumed-role/{role_name}/')"}, "attributeCondition": "attribute.account == '222222222222' && attribute.aws_role in ['DummyLambda', 'DummyBatch']"},
        "iam_policy_resource": f"projects/example-study-space-dev/serviceAccounts/{email}",
        "runtime_config": {"audience": f"//iam.googleapis.com/{pool}/providers/aws-provider", "service_account_email": email},
        "iam_policy": {"version": 3, "bindings": [{"role": "roles/iam.workloadIdentityUser", "members": [prefix + "DummyLambda", prefix + "UnselectedDummyRole"]}, {"role": "roles/iam.workloadIdentityUser", "members": [prefix + "DummyBatch"], "condition": condition}, {"role": "roles/editor", "members": ["user:dummy@example.invalid"]}]},
        "runtime_grants": {"grant-01": {"member": prefix + "DummyLambda"}, "grant-02": {"member": prefix + "DummyBatch", "condition_title": "dummy-condition"}},
        "enabled_services": {"services": [{"name": f"projects/{number}/services/pubsub.googleapis.com", "config": {"name": "pubsub.googleapis.com"}, "state": "ENABLED"}, {"name": f"projects/{number}/services/firebase.googleapis.com", "config": {"name": "firebase.googleapis.com"}, "state": "ENABLED"}]},
        "api_classification": {"pubsub.googleapis.com": {"classification": "Own", "dependency_addresses": ["module.export_topic[0].google_pubsub_topic.export"], "reason": "Dummy explicit topic dependency evidence"}, "firebase.googleapis.com": {"classification": "Platform/External", "dependency_addresses": [], "reason": "Dummy external platform ownership"}},
    }


class InventoryContracts(unittest.TestCase):
    def test_preserves_trust_and_only_selected_grants_without_enabling(self):
        source = inventory()
        before = copy.deepcopy(source)
        result = prepare_module.prepare(source)
        self.assertEqual(source, before)
        private = result["runtime_wif_inventory"]
        self.assertEqual(private["provider"]["attribute_mapping"], source["provider"]["attributeMapping"])
        self.assertEqual(private["provider"]["attribute_condition"], source["provider"]["attributeCondition"])
        self.assertEqual(private["grants"]["grant-02"]["condition"], source["iam_policy"]["bindings"][1]["condition"])
        self.assertEqual(set(private["grants"]), {"grant-01", "grant-02"})
        self.assertFalse(result["own_runtime_wif_pool"] or result["own_runtime_wif_provider"])
        self.assertEqual(result["runtime_wif_grant_keys"], [])
        self.assertEqual(result["owned_api_keys"], [])
        self.assertNotIn("UnselectedDummyRole", json.dumps(result))
        self.assertNotIn("dummy@example.invalid", json.dumps(result))

    def test_unset_pool_mode_is_preserved_without_inventing_a_value(self):
        data = inventory()
        data["pool"].pop("mode")
        self.assertIsNone(prepare_module.prepare(data)["runtime_wif_inventory"]["pool"]["mode"])
        data["pool"]["mode"] = ""
        self.assertEqual(prepare_module.prepare(data)["runtime_wif_inventory"]["pool"]["mode"], "")

    def test_rejects_incomplete_wrong_or_broad_inventory(self):
        mutations = [
            lambda d: d["project"].update(projectId="youtube-study-space"),
            lambda d: d["pool"].update(name="projects/999/locations/global/workloadIdentityPools/aws-runtime"),
            lambda d: d["pool"].update(state="DELETED"),
            lambda d: d["pool"].update(mode="TRUST_DOMAIN"),
            lambda d: d["pool"].update(inlineTrustConfig={}),
            lambda d: d["pool_attestation_rules"].update(attestationRules=[{"dummy": "unsupported"}]),
            lambda d: d["provider"].update(disabled=True),
            lambda d: d["provider"].update(oidc={}),
            lambda d: d["provider"].update(attributeCondition=""),
            lambda d: d["provider"]["aws"].update(accountId="bad"),
            lambda d: d["runtime_config"].update(audience="wrong"),
            lambda d: d.update(iam_policy_resource="projects/example-study-space-dev/serviceAccounts/other@example.invalid"),
            lambda d: d["runtime_grants"]["grant-01"].update(member=d["runtime_grants"]["grant-01"]["member"].rsplit("/", 1)[0] + "/*"),
            lambda d: d["runtime_grants"]["grant-01"].update(member="allUsers"),
            lambda d: d["runtime_grants"]["grant-01"].update(member=d["runtime_grants"]["grant-01"]["member"].replace("111111111111", "999999999999")),
            lambda d: d["iam_policy"]["bindings"][0].update(role="roles/editor"),
            lambda d: d["iam_policy"].update(version=1),
            lambda d: d["runtime_grants"].update({"grant-03": d["runtime_grants"]["grant-01"]}),
            lambda d: d["iam_policy"]["bindings"].append(copy.deepcopy(d["iam_policy"]["bindings"][1])),
            lambda d: d["enabled_services"].update(nextPageToken="remaining"),
            lambda d: d["enabled_services"]["services"][0].update(state="DISABLED"),
            lambda d: d["enabled_services"]["services"][0].update(name="projects/999/services/pubsub.googleapis.com"),
            lambda d: d["api_classification"]["pubsub.googleapis.com"].update(dependency_addresses=[]),
            lambda d: d["api_classification"]["pubsub.googleapis.com"].update(classification="Unknown"),
            lambda d: d["enabled_services"].update(services=[]),
            lambda d: d["api_classification"].pop("firebase.googleapis.com"),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutations.index(mutation)):
                data = inventory()
                mutation(data)
                with self.assertRaises((ValueError, KeyError)):
                    prepare_module.prepare(data)

    def test_private_cli_never_prints_values_or_overwrites(self):
        with tempfile.TemporaryDirectory() as directory:
            source, output = Path(directory) / "input.json", Path(directory) / "candidate.json"
            source.write_text(json.dumps(inventory()))
            source.chmod(0o600)
            result = subprocess.run(cli_command(source, output), capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)
            self.assertNotIn("dummy private sentinel", result.stdout + result.stderr)
            original = output.read_bytes()
            result = subprocess.run(cli_command(source, output), capture_output=True, text=True)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(output.read_bytes(), original)
            source.chmod(0o644)
            result = subprocess.run(cli_command(source, Path(directory) / "new.json"), capture_output=True, text=True)
            self.assertEqual(result.returncode, 1)
            self.assertFalse((Path(directory) / "new.json").exists())

    def test_duplicate_json_and_symlinks_rejected(self):
        with self.assertRaises(ValueError):
            json.loads('{"a":1,"a":2}', object_pairs_hook=prepare_module.unique_object)
        with tempfile.TemporaryDirectory() as directory:
            real, link = Path(directory) / "real.json", Path(directory) / "link.json"
            real.write_text(json.dumps(inventory()))
            real.chmod(0o600)
            link.symlink_to(real)
            result = subprocess.run(cli_command(link, Path(directory) / "out.json"), capture_output=True)
            self.assertEqual(result.returncode, 1)


def terraform_tests(target: Path, tests: str) -> list[dict]:
    shutil.copyfile(ROOT / "infra/gcp/environments/dev/.terraform.lock.hcl", target / ".terraform.lock.hcl")
    (target / "tests").mkdir()
    (target / "tests/contracts.tftest.hcl").write_text(tests)
    for args in [("init", "-backend=false", "-input=false", "-lockfile=readonly"), ("test", "-json", "-verbose")]:
        result = subprocess.run(["terraform", f"-chdir={target}", *args], capture_output=True, text=True)
        if result.returncode:
            # Dummy-only diagnostics are useful; omit raw verbose plans.
            diagnostics = []
            for line in result.stdout.splitlines():
                try:
                    event = json.loads(line)
                    if event.get("type") == "diagnostic":
                        diagnostic = event["diagnostic"]
                        diagnostics.append(diagnostic.get("summary", "") + ": " + diagnostic.get("detail", ""))
                except ValueError:
                    pass
            raise AssertionError("Credentialless Terraform contracts failed: " + "\n".join(diagnostics) + result.stderr)
    return [event["test_plan"] for line in result.stdout.splitlines() if (event := json.loads(line)).get("type") == "test_plan"]


class TerraformContracts(unittest.TestCase):
    def test_wif_stages_exact_values_and_global_create_rejection(self):
        candidate = prepare_module.prepare(inventory())["runtime_wif_inventory"]
        invalid = copy.deepcopy(candidate)
        invalid["grants"]["grant-01"]["member"] = "allUsers"
        tests = 'mock_provider "google" {}\nvariables {\nproject_id = "example-study-space-dev"\ninventory = ' + json.dumps(candidate) + '\n}\n' + '''
run "default_off" { command = plan }
run "pool_only" {
  command = plan
  variables { own_pool = true }
  assert {
    condition = google_iam_workload_identity_pool.runtime[0].description == var.inventory.pool.description && google_iam_workload_identity_pool.runtime[0].mode == "FEDERATION_ONLY" && !google_iam_workload_identity_pool.runtime[0].disabled
    error_message = "Preserve active pool metadata."
  }
}
run "provider" {
  command = plan
  variables {
    own_pool = true
    own_provider = true
  }
  assert {
    condition = google_iam_workload_identity_pool_provider.runtime[0].attribute_mapping == var.inventory.provider.attribute_mapping && google_iam_workload_identity_pool_provider.runtime[0].attribute_condition == var.inventory.provider.attribute_condition && google_iam_workload_identity_pool_provider.runtime[0].aws[0].account_id == var.inventory.provider.aws_account_id
    error_message = "Preserve existing trust expressions and account verbatim."
  }
}
run "exact_grants" {
  command = plan
  variables {
    own_pool = true
    own_provider = true
    grant_keys = ["grant-01", "grant-02"]
  }
  assert {
    condition = google_service_account_iam_member.runtime["grant-01"].member == var.inventory.grants["grant-01"].member && google_service_account_iam_member.runtime["grant-02"].condition[0].expression == var.inventory.grants["grant-02"].condition.expression && google_service_account_iam_member.runtime["grant-02"].role == "roles/iam.workloadIdentityUser"
    error_message = "Adopt only exact individual grants with unchanged conditions."
  }
}
run "reject_provider_without_pool" {
  command = plan
  variables { own_provider = true }
  expect_failures = [var.own_provider]
}
run "reject_grants_without_provider" {
  command = plan
  variables { grant_keys = ["grant-01"] }
  expect_failures = [var.grant_keys]
}
''' + 'run "reject_broad_member" {\ncommand = plan\nvariables {\nown_pool = true\nown_provider = true\ngrant_keys = ["grant-01"]\ninventory = ' + json.dumps(invalid) + '\n}\nexpect_failures = [var.inventory]\n}\n'
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "module"
            shutil.copytree(ROOT / "infra/gcp/modules/runtime-aws-wif", target)
            plans = terraform_tests(target, tests)
            self.assertFalse(plans[0].get("resource_changes"))
            for plan, count in zip(plans[1:4], [1, 2, 4]):
                summary = build_summary(plan, environment="dev", git_sha="0" * 40, policy="import-only")
                self.assertEqual(summary["counts"]["create"], count)
                self.assertFalse(summary["policy_passed"])
                self.assertNotIn("dummy private sentinel", json.dumps(summary))
                self.assertNotIn("111111111111", json.dumps(summary))
                self.assertNotIn("DummyLambda", json.dumps(summary))

    def test_api_selection_and_destruction_flags(self):
        classification = prepare_module.prepare(inventory())["api_classification"]
        tests = 'mock_provider "google" {}\nvariables {\nproject_id = "example-study-space-dev"\nclassification = ' + json.dumps(classification) + '\n}\n' + '''
run "default_off" { command = plan }
run "one_explicit_api" {
  command = plan
  variables { service_keys = ["pubsub.googleapis.com"] }
  assert {
    condition = length(google_project_service.owned) == 1 && !google_project_service.owned["pubsub.googleapis.com"].disable_on_destroy && !google_project_service.owned["pubsub.googleapis.com"].disable_dependent_services
    error_message = "Never own all enabled APIs or disable dependencies on removal."
  }
}
run "reject_platform_api" {
  command = plan
  variables { service_keys = ["firebase.googleapis.com"] }
  expect_failures = [var.classification]
}
run "reject_uninventoried_api" {
  command = plan
  variables { service_keys = ["storage.googleapis.com"] }
  expect_failures = [var.classification]
}
'''
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "module"
            shutil.copytree(ROOT / "infra/gcp/modules/owned-project-apis", target)
            plans = terraform_tests(target, tests)
            self.assertFalse(plans[0].get("resource_changes"))
            summary = build_summary(plans[1], environment="dev", git_sha="0" * 40, policy="import-only")
            self.assertEqual(summary["counts"]["create"], 1)
            self.assertFalse(summary["policy_passed"])

    def test_real_development_root_defaults_are_empty(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            target = base / "environments/dev"
            target.mkdir(parents=True)
            for name in ("runtime-aws-wif", "owned-project-apis"):
                shutil.copytree(ROOT / "infra/gcp/modules" / name, base / "modules" / name)
            (target / "runtime-ownership.tf").write_text((ROOT / "infra/gcp/environments/dev/runtime-ownership.tf").read_text().replace("test-youtube-study-space", "example-study-space-dev"))
            (target / "main.tf").write_text('terraform {\nrequired_providers {\ngoogle = { source = "hashicorp/google", version = "= 8.5.0" }\n}\n}\nprovider "google" {}\nvariable "project_id" {\ntype = string\ndefault = "example-study-space-dev"\n}\n')
            candidate = prepare_module.prepare(inventory())
            tests = 'mock_provider "google" {}\nvariables {\nruntime_wif_inventory = ' + json.dumps(candidate["runtime_wif_inventory"]) + '\napi_classification = ' + json.dumps(candidate["api_classification"]) + '\n}\n' + '''
run "default_off" { command = plan }
run "prepared_import_addresses" {
  command = plan
  variables {
    own_runtime_wif_pool = true
    own_runtime_wif_provider = true
    runtime_wif_grant_keys = ["grant-01", "grant-02"]
    owned_api_keys = ["pubsub.googleapis.com"]
  }
}
run "reject_production_wif" {
  command = plan
  variables {
    project_id = "youtube-study-space"
    own_runtime_wif_pool = true
  }
  expect_failures = [var.own_runtime_wif_pool]
}
run "reject_production_apis" {
  command = plan
  variables {
    project_id = "youtube-study-space"
    owned_api_keys = ["pubsub.googleapis.com"]
  }
  expect_failures = [var.owned_api_keys]
}
'''
            private = candidate["runtime_wif_inventory"]
            project = "example-study-space-dev"
            pool_id = f"projects/{project}/locations/global/workloadIdentityPools/aws-runtime"
            overrides = {
                "module.runtime_wif.google_iam_workload_identity_pool.runtime[0]": {"id": pool_id, "project": project, "workload_identity_pool_id": "aws-runtime", **private["pool"]},
                "module.runtime_wif.google_iam_workload_identity_pool_provider.runtime[0]": {"id": pool_id + "/providers/aws-provider", "project": project, "workload_identity_pool_id": "aws-runtime", "workload_identity_pool_provider_id": "aws-provider", "display_name": private["provider"]["display_name"], "description": private["provider"]["description"], "disabled": False, "attribute_mapping": private["provider"]["attribute_mapping"], "attribute_condition": private["provider"]["attribute_condition"], "aws": [{"account_id": private["provider"]["aws_account_id"]}]},
                'module.owned_apis.google_project_service.owned["pubsub.googleapis.com"]': {"id": project + "/pubsub.googleapis.com", "project": project, "service": "pubsub.googleapis.com", "disable_on_destroy": False, "disable_dependent_services": False},
            }
            for alias, grant in private["grants"].items():
                overrides[f'module.runtime_wif.google_service_account_iam_member.runtime["{alias}"]'] = {
                    "id": private["service_account_id"] + " roles/iam.workloadIdentityUser " + grant["member"],
                    "service_account_id": private["service_account_id"], "role": "roles/iam.workloadIdentityUser", "member": grant["member"],
                    "condition": [] if grant["condition"] is None else [grant["condition"]],
                }
            blocks = "\n".join('override_resource {\ntarget = ' + address + '\nvalues = ' + json.dumps(values) + '\n}\n' for address, values in overrides.items())
            tests = tests.replace('run "prepared_import_addresses" {\n', 'run "prepared_import_addresses" {\n' + blocks)
            plans = terraform_tests(target, tests)
            self.assertFalse(plans[0].get("resource_changes"))
            imports = {resource["address"]: resource["change"].get("importing", {}).get("id") for resource in plans[1]["resource_changes"]}
            self.assertEqual(set(imports), {
                "module.runtime_wif.google_iam_workload_identity_pool.runtime[0]",
                "module.runtime_wif.google_iam_workload_identity_pool_provider.runtime[0]",
                'module.runtime_wif.google_service_account_iam_member.runtime["grant-01"]',
                'module.runtime_wif.google_service_account_iam_member.runtime["grant-02"]',
                'module.owned_apis.google_project_service.owned["pubsub.googleapis.com"]',
            })
            self.assertEqual(imports['module.owned_apis.google_project_service.owned["pubsub.googleapis.com"]'], "example-study-space-dev/pubsub.googleapis.com")
            self.assertEqual(imports['module.runtime_wif.google_service_account_iam_member.runtime["grant-02"]'], candidate["runtime_wif_inventory"]["service_account_id"] + " roles/iam.workloadIdentityUser " + candidate["runtime_wif_inventory"]["grants"]["grant-02"]["member"] + " dummy-condition")
        for module in ("runtime-aws-wif", "owned-project-apis"):
            code = (ROOT / "infra/gcp/modules" / module / "main.tf").read_text()
            self.assertNotIn('resource "google_project_iam', code)
            self.assertNotIn('resource "google_service_account_iam_binding', code)
            self.assertNotIn('resource "google_service_account_iam_policy', code)
            self.assertEqual(code.count("prevent_destroy = true"), code.count('resource "'))


if __name__ == "__main__":
    unittest.main()
