#!/usr/bin/env python3
"""Credentialless native prod/dev runtime import source contracts, never live admission."""
from __future__ import annotations

import copy
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
GCP = ROOT / "infra/gcp"
sys.dont_write_bytecode = True
sys.path.insert(0, str(ROOT / ".github/scripts"))
from terraform_plan_summary import build_summary
from test_runtime_ownership import inventory, prepare_module

POOL = "module.runtime_wif.google_iam_workload_identity_pool.runtime[0]"
PROVIDER = "module.runtime_wif.google_iam_workload_identity_pool_provider.runtime[0]"
GRANTS = {key: f'module.runtime_wif.google_service_account_iam_member.runtime["{key}"]'
          for key in ("grant-01", "grant-02")}
API = 'module.owned_apis.google_project_service.owned["pubsub.googleapis.com"]'
PROD, DEV = "youtube-study-space", "test-youtube-study-space"


class ProductionRuntimeOwnership(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory(prefix="prod-runtime-source-")
        cls.addClassCleanup(cls.temporary.cleanup)
        cls.base = Path(cls.temporary.name)
        cls.binary = shutil.which("terraform")
        if not cls.binary:
            raise AssertionError("Pinned Terraform is required; this contract must not skip.")
        # No HOME, ADC, AWS/Google profiles, Terraform inputs or CLI flags enter
        # the subprocess. Provider mocks never contact a real service/backend.
        cls.env = {key: os.environ[key] for key in (
            "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "http_proxy", "https_proxy",
            "all_proxy", "no_proxy", "SSL_CERT_FILE", "SSL_CERT_DIR", "REQUESTS_CA_BUNDLE", "CURL_CA_BUNDLE"
        ) if key in os.environ}
        cli = "disable_checkpoint = true\n"
        if mirror := os.environ.get("GCP_TEST_PROVIDER_MIRROR"):
            location = Path(mirror)
            if not location.is_absolute() or location.is_symlink():
                raise AssertionError("Provider mirror must be an absolute public cache directory.")
            cli += "provider_installation {\n filesystem_mirror {\n path = " + json.dumps(mirror) + "\n }\n}\n"
        config = cls.base / "terraform.rc"
        config.write_text(cli)
        cls.env.update(PATH=str(Path(cls.binary).parent) + os.pathsep + os.defpath,
                       TF_CLI_CONFIG_FILE=str(config), TF_IN_AUTOMATION="true", TF_INPUT="false",
                       TF_WORKSPACE="default", CHECKPOINT_DISABLE="1")
        cls.roots, cls.inputs, cls.plans = {}, {}, {}
        for project in (PROD, DEV):
            base = cls.base / project
            target = base / "environments/prod"
            target.mkdir(parents=True)
            shutil.copytree(GCP / "modules", base / "modules", ignore=shutil.ignore_patterns(
                ".terraform", "*.tfstate*", "*.tfplan", "*.tfvars*", "__pycache__"))
            if project == PROD:
                for source in (GCP / "environments/prod").glob("*.tf"):
                    shutil.copyfile(source, target / source.name)
            else:
                # Use the unchanged development runtime definition, isolated
                # from unrelated always-owned retained development resources.
                shutil.copyfile(GCP / "environments/dev/runtime-ownership.tf", target / "runtime-ownership.tf")
                main = (GCP / "environments/prod/main.tf").read_text().replace(PROD, DEV)
                (target / "main.tf").write_text(main)
            shutil.copyfile(GCP / "environments/prod/.terraform.lock.hcl", target / ".terraform.lock.hcl")
            (target / "tests").mkdir()
            raw = json.loads(json.dumps(inventory()).replace("example-study-space-dev", project))
            with patch.object(prepare_module, "PROJECT", project):
                cls.inputs[project] = prepare_module.prepare(raw)
            cls.roots[project] = target
            cls.command(target, "init", "-backend=false", "-input=false", "-lockfile=readonly")
            cls.plans[project] = cls.native(project, cls.wave_runs())

    @classmethod
    def command(cls, target, *args, expected_failure=False):
        result = subprocess.run([cls.binary, "-chdir=" + str(target), *args], env=cls.env,
                                stdin=subprocess.DEVNULL, capture_output=True, timeout=120)
        if not expected_failure and result.returncode:
            raise AssertionError("Credentialless runtime mock contract failed; raw output suppressed.")
        return result

    @classmethod
    def overrides(cls, project):
        data = cls.inputs[project]["runtime_wif_inventory"]
        pool = f"projects/{project}/locations/global/workloadIdentityPools/aws-runtime"
        values = {
            POOL: {"id": pool, "name": "projects/111111111111/locations/global/workloadIdentityPools/aws-runtime",
                   "project": project, "workload_identity_pool_id": "aws-runtime", **data["pool"]},
            PROVIDER: {"id": pool + "/providers/aws-provider", "project": project,
                       "workload_identity_pool_id": "aws-runtime", "workload_identity_pool_provider_id": "aws-provider",
                       **{key: data["provider"][key] for key in (
                           "display_name", "description", "disabled", "attribute_mapping", "attribute_condition")},
                       "aws": [{"account_id": data["provider"]["aws_account_id"]}]},
            API: {"id": project + "/pubsub.googleapis.com", "project": project,
                  "service": "pubsub.googleapis.com", "disable_on_destroy": False, "disable_dependent_services": False},
        }
        for alias, grant in data["grants"].items():
            identity = " ".join(filter(None, [data["service_account_id"], "roles/iam.workloadIdentityUser",
                                            grant["member"], (grant["condition"] or {}).get("title")]))
            values[GRANTS[alias]] = {"id": identity, "service_account_id": data["service_account_id"],
                                    "role": "roles/iam.workloadIdentityUser", "member": grant["member"],
                                    "condition": [] if grant["condition"] is None else [grant["condition"]]}
        return "\n".join("override_resource {\n target = " + address + "\n values = " + json.dumps(value) + "\n}\n"
                         for address, value in values.items())

    @staticmethod
    def wave_runs():
        return [
            ("default", {"runtime_wif_inventory": None, "api_classification": {}}),
            ("private_metadata_only", {}),
            ("pool", {"own_runtime_wif_pool": True}),
            ("provider", {"own_runtime_wif_pool": True, "own_runtime_wif_provider": True}),
            ("one_grant", {"own_runtime_wif_pool": True, "own_runtime_wif_provider": True,
                           "runtime_wif_grant_keys": ["grant-01"]}),
            ("two_grants", {"own_runtime_wif_pool": True, "own_runtime_wif_provider": True,
                            "runtime_wif_grant_keys": ["grant-01", "grant-02"]}),
            ("apis", {"own_runtime_wif_pool": True, "own_runtime_wif_provider": True,
                      "runtime_wif_grant_keys": ["grant-01", "grant-02"], "owned_api_keys": ["pubsub.googleapis.com"]}),
        ]

    @classmethod
    def native(cls, project, runs, *, expected_failure=False):
        inputs = cls.inputs[project]
        text = 'mock_provider "google" {}\n' + cls.overrides(project)
        text += "variables {\n" + "\n".join(key + " = " + json.dumps(inputs[key]) for key in
                                                ("runtime_wif_inventory", "api_classification")) + "\n}\n"
        for name, variables in runs:
            text += 'run "' + name + '" {\n command = plan\n variables {\n'
            text += "\n".join(key + " = " + json.dumps(value) for key, value in variables.items()) + "\n }\n}\n"
        (cls.roots[project] / "tests/runtime.tftest.hcl").write_text(text)
        result = cls.command(cls.roots[project], "test", "-json", "-verbose", expected_failure=expected_failure)
        events = [json.loads(line) for line in result.stdout.splitlines()]
        if expected_failure:
            return result.returncode, [event["diagnostic"] for event in events if event.get("type") == "diagnostic"]
        return {event["@testrun"]: event["test_plan"] for event in events if event.get("type") == "test_plan"}

    def test_defaults_and_private_metadata_alone_own_nothing(self):
        for project in (PROD, DEV):
            for run in ("default", "private_metadata_only"):
                summary = build_summary(self.plans[project][run], environment="prod" if project == PROD else "dev",
                                        git_sha="0" * 40, policy="import-only")
                self.assertTrue(summary["policy_passed"])
                self.assertEqual(summary["resources"], [])
                self.assertTrue(all(value == 0 for value in summary["counts"].values()))

    def test_cumulative_imports_keep_addresses_values_and_sensitivity(self):
        waves = [("pool", {POOL}), ("provider", {POOL, PROVIDER}), ("one_grant", {POOL, PROVIDER, GRANTS["grant-01"]}),
                 ("two_grants", {POOL, PROVIDER, *GRANTS.values()}), ("apis", {POOL, PROVIDER, *GRANTS.values(), API})]
        for project in (PROD, DEV):
            for run, addresses in waves:
                with self.subTest(project=project, wave=run):
                    plan = self.plans[project][run]
                    summary = build_summary(plan, environment="prod" if project == PROD else "dev",
                                            git_sha="0" * 40, policy="import-only")
                    self.assertTrue(summary["policy_passed"])
                    self.assertEqual(summary["counts"], {"import": len(addresses), "no-op": len(addresses),
                        **{key: 0 for key in ("create", "update", "delete", "replace", "read", "other", "drift")}})
                    self.assertEqual({item["address"] for item in summary["resources"]}, addresses)
                    for resource in plan["resource_changes"]:
                        change = resource["change"]
                        self.assertEqual(change["before"], change["after"])
                        self.assertEqual(change["before_sensitive"], change["after_sensitive"])
                        self.assertEqual(change["after_unknown"], {})

    def test_shared_prod_and_development_import_contracts_match(self):
        for run, _ in self.wave_runs()[2:]:
            def projection(project):
                return {item["address"]: item["change"]["importing"]["id"].replace(project, "TARGET")
                        for item in self.plans[project][run]["resource_changes"]}
            self.assertEqual(projection(PROD), projection(DEV))
        for project in (PROD, DEV):
            resources = {item["address"]: item["change"]["after"]
                         for item in self.plans[project]["apis"]["resource_changes"]}
            private = self.inputs[project]["runtime_wif_inventory"]
            self.assertEqual(resources[PROVIDER]["attribute_mapping"], private["provider"]["attribute_mapping"])
            self.assertEqual(resources[PROVIDER]["attribute_condition"], private["provider"]["attribute_condition"])
            self.assertEqual(resources[PROVIDER]["aws"], [{"account_id": private["provider"]["aws_account_id"]}])
            self.assertEqual(resources[POOL]["mode"], private["pool"]["mode"])
            for alias, address in GRANTS.items():
                self.assertEqual(resources[address]["member"], private["grants"][alias]["member"])
                self.assertEqual(resources[address]["role"], "roles/iam.workloadIdentityUser")
                self.assertEqual(resources[address]["condition"], [] if private["grants"][alias]["condition"] is None
                                 else [private["grants"][alias]["condition"]])
            self.assertFalse(resources[API]["disable_on_destroy"])
            self.assertFalse(resources[API]["disable_dependent_services"])

    def assert_rejected(self, variables, message):
        code, diagnostics = self.native(PROD, [("reject", variables)], expected_failure=True)
        self.assertNotEqual(code, 0)
        self.assertTrue(any(item.get("summary") == "Invalid value for variable" and message in item.get("detail", "")
                            for item in diagnostics), "Expected the specific contract rejection; raw diagnostics suppressed.")

    def test_production_rejects_development_project_and_invalid_adoption_order(self):
        self.assert_rejected({"project_id": DEV}, "The production root only manages its existing production project.")
        self.assert_rejected({"own_runtime_wif_provider": True}, "Provider adoption requires persistent pool ownership.")
        self.assert_rejected({"runtime_wif_grant_keys": ["grant-01"]}, "Exact grant aliases require persistent provider ownership.")
        self.assert_rejected({"own_runtime_wif_pool": True, "own_runtime_wif_provider": True,
                              "runtime_wif_grant_keys": ["private-role-must-not-be-an-address"]},
                             "Exact grant aliases require persistent provider ownership.")

    def test_unreviewed_disabled_or_unjustified_apis_are_rejected(self):
        for update in ({"classification": "Platform/External"}, {"classification": "Investigate"},
                       {"state": "DISABLED"}, {"dependency_addresses": []}, {"reason": ""}):
            data = copy.deepcopy(self.inputs[PROD]["api_classification"])
            data["pubsub.googleapis.com"].update(update)
            self.assert_rejected({"owned_api_keys": ["pubsub.googleapis.com"], "api_classification": data},
                "Only existing enabled Own APIs with explicit reviewed resource dependency evidence may be adopted.")

    def test_invalid_runtime_trust_inventory_is_rejected(self):
        data = copy.deepcopy(self.inputs[PROD]["runtime_wif_inventory"])
        data["grants"]["grant-01"]["member"] = "principalSet://iam.googleapis.com/projects/111111111111/locations/global/workloadIdentityPools/aws-runtime/*"
        self.assert_rejected({"own_runtime_wif_pool": True, "own_runtime_wif_provider": True,
                              "runtime_wif_grant_keys": ["grant-01"], "runtime_wif_inventory": data},
                             "Adoption requires the existing active AWS federation and exact role-scoped grants")


if __name__ == "__main__":
    unittest.main()
