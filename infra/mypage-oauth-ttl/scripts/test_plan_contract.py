"""Observable scope, destructive-change and private-output regression contracts."""

import copy
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

import plan_contract as contract


def fixture(environment="development"):
    """Synthetic values in Terraform 1.16.4 / Google 8.5.0 saved-plan shape."""
    target = {"schemaVersion": 1, "environment": environment,
              "projectID": f"demo-mypage-ttl-{environment}", "databaseID": "(default)"}
    scope = contract.target_scope(target)
    after = {"project": scope["projectID"], "database": scope["databaseID"],
             "collection": "oauth-transactions", "field": "expiresAt", "skip_wait": False,
             "deletion_policy": "PREVENT", "index_config": [], "ttl_config": [{}], "timeouts": None}
    resource = {"address": contract.ADDRESS, "module_address": "module.oauth_transaction_ttl",
                "mode": "managed", "type": "google_firestore_field", "name": "oauth_transaction_ttl",
                "provider_name": contract.PROVIDER,
                "change": {"actions": ["create"], "before": None, "after": after,
                           "after_unknown": {"id": True, "name": True,
                               "ttl_config": [{"state": True, "expiration_offset": True}]},
                           "before_sensitive": False, "after_sensitive": {"ttl_config": [{}]}}}
    plan = {
        "format_version": "1.2", "terraform_version": "1.16.4", "complete": True,
        "errored": False, "applyable": True,
        "variables": {"project_id": {"value": scope["projectID"]},
                      "database_id": {"value": scope["databaseID"]}},
        "resource_changes": [resource],
        "output_changes": {"scope": {"after": scope, "after_unknown": False, "after_sensitive": False}},
        "planned_values": {"outputs": {"scope": {"value": scope, "sensitive": False}},
            "root_module": {"child_modules": [{"resources": [{
                "address": contract.ADDRESS, "mode": "managed", "provider_name": contract.PROVIDER,
                "values": copy.deepcopy(after), "sensitive_values": {"ttl_config": [{}]}}]}]}},
        "configuration": {"root_module": {"module_calls": {"oauth_transaction_ttl": {"module": {
            "resources": [{"mode": "managed", "type": "google_firestore_field", "expressions": {
                "project": {"references": ["var.project_id"]},
                "database": {"references": ["var.database_id"]},
                "collection": {"constant_value": "oauth-transactions"},
                "field": {"constant_value": "expiresAt"},
                "deletion_policy": {"constant_value": "PREVENT"},
                "skip_wait": {"constant_value": False}, "ttl_config": [{}],
            }}]}}}}},
    }
    return plan, target


class PlanContractTests(unittest.TestCase):
    def setUp(self):
        self.plan, self.target = fixture()
        self.change = self.plan["resource_changes"][0]["change"]

    def rejected(self, mutate):
        plan = copy.deepcopy(self.plan)
        mutate(plan)
        with self.assertRaises(contract.ContractError):
            contract.validate_plan(plan, self.target)

    def sync_values(self):
        self.plan["planned_values"]["root_module"]["child_modules"][0]["resources"][0]["values"] = copy.deepcopy(self.change["after"])

    def test_accepts_bound_create_for_each_environment(self):
        for environment in ("development", "production"):
            plan, target = fixture(environment)
            self.assertEqual(contract.validate_plan(plan, target), "create")

    def test_accepts_ttl_enable_with_existing_identity_and_index_unchanged(self):
        name = "projects/demo-mypage-ttl-development/databases/(default)/collectionGroups/oauth-transactions/fields/expiresAt"
        self.change["actions"] = ["update"]
        self.change["after"].update(id=name, name=name)
        self.change["before"] = copy.deepcopy(self.change["after"])
        self.change["before"].update(ttl_config=[], deletion_policy="DELETE")
        self.change["after_unknown"].pop("id")
        self.change["after_unknown"].pop("name")
        self.sync_values()
        self.assertEqual(contract.validate_plan(self.plan, self.target), "update")

    def test_accepts_noop_without_claiming_live_active(self):
        self.change["actions"] = ["no-op"]
        self.change["after"]["ttl_config"] = [{"state": "CREATING", "expiration_offset": ""}]
        self.change["before"] = copy.deepcopy(self.change["after"])
        self.change["after_unknown"] = {}
        self.sync_values()
        for applyable in (True, False):
            self.plan["applyable"] = applyable
            self.assertEqual(contract.validate_plan(self.plan, self.target), "no-op")

    def test_rejects_wrong_environment_project_database_collection_field(self):
        self.rejected(lambda p: p["output_changes"]["scope"]["after"].update(environment="production"))
        for key, value in (("project", "demo-other-project"), ("database", "other-db"),
                           ("collection", "support-challenges"), ("field", "createdAt")):
            self.rejected(lambda p: p["resource_changes"][0]["change"]["after"].update({key: value}))

    def test_rejects_extra_resources_including_support_and_iam(self):
        for resource_type in ("google_project_iam_member", "google_firestore_field"):
            self.rejected(lambda p: p["resource_changes"].append({"type": resource_type}))
        self.rejected(lambda p: p["planned_values"]["root_module"].update(resources=[{"mode": "data"}]))
        self.rejected(lambda p: p["configuration"]["root_module"].update(resources=[{"mode": "data"}]))

    def test_rejects_destructive_actions_import_move_and_replace(self):
        for action in (["delete"], ["delete", "create"], ["create", "delete"], ["read"], []):
            self.rejected(lambda p: p["resource_changes"][0]["change"].update(actions=action))
        self.rejected(lambda p: p["resource_changes"][0]["change"].update(importing={"id": "synthetic"}))
        self.rejected(lambda p: p["resource_changes"][0].update(previous_address="old.address"))
        self.rejected(lambda p: p["resource_changes"][0].update(deposed="synthetic"))
        self.rejected(lambda p: p["resource_changes"][0]["change"].update(replace_paths=[["field"]]))

    def test_rejects_wrong_provider_address_and_data_mode(self):
        for key, value in (("provider_name", "registry.terraform.io/hashicorp/google-beta"),
                           ("address", "other.address"), ("mode", "data")):
            self.rejected(lambda p: p["resource_changes"][0].update({key: value}))

    def test_rejects_incomplete_errored_drift_deferred_or_failed_checks(self):
        for key, value in (("complete", False), ("errored", True), ("resource_drift", [{}]),
                           ("deferred_changes", [{}]), ("checks", [{"status": "unknown"}]),
                           ("applyable", False), ("format_version", "2.0"),
                           ("terraform_version", "1.16.3")):
            self.rejected(lambda p: p.update({key: value}))

    def test_rejects_ttl_disable_offset_wait_skip_and_protection_loss(self):
        for key, value in (("ttl_config", []), ("ttl_config", [{"expiration_offset": "600s"}]),
                           ("ttl_config", [{"state": "DELETING"}]), ("skip_wait", True),
                           ("deletion_policy", "ABANDON"), ("timeouts", {"create": "1m"})):
            self.rejected(lambda p: p["resource_changes"][0]["change"]["after"].update({key: value}))

    def test_rejects_index_overrides_and_arbitrary_field_keys(self):
        for config in ([{}], [{"indexes": []}], [{"indexes": [{"order": "ASCENDING"}]}]):
            self.rejected(lambda p: p["resource_changes"][0]["change"]["after"].update(index_config=config))
        self.rejected(lambda p: p["resource_changes"][0]["change"]["after"].update(unexpected="synthetic"))

    def test_rejects_existing_index_removal_or_existing_ttl_mutation(self):
        self.change["actions"] = ["update"]
        self.change["before"] = copy.deepcopy(self.change["after"])
        self.change["before"]["index_config"] = [{"indexes": []}]
        with self.assertRaises(contract.ContractError):
            contract.validate_plan(self.plan, self.target)
        self.change["before"]["index_config"] = []
        with self.assertRaises(contract.ContractError):
            contract.validate_plan(self.plan, self.target)

    def test_rejects_unknown_and_sensitive_scope(self):
        for key in ("project", "database", "collection", "field", "deletion_policy", "index_config"):
            self.rejected(lambda p: p["resource_changes"][0]["change"]["after_unknown"].update({key: True}))
            self.rejected(lambda p: p["resource_changes"][0]["change"]["after_sensitive"].update({key: True}))
        self.rejected(lambda p: p["output_changes"]["scope"].update(after_unknown={"environment": True}))

    def test_rejects_input_planned_value_or_configuration_divergence(self):
        self.rejected(lambda p: p["variables"]["project_id"].update(value="demo-other-project"))
        self.rejected(lambda p: p["planned_values"]["root_module"]["child_modules"][0]["resources"][0]["values"].update(field="createdAt"))
        expressions = self.plan["configuration"]["root_module"]["module_calls"]["oauth_transaction_ttl"]["module"]["resources"][0]["expressions"]
        for key, value in (("index_config", [{}]), ("ttl_config", [{"expiration_offset": {"references": ["var.offset"]}}])):
            saved = copy.deepcopy(expressions)
            expressions[key] = value
            with self.assertRaises(contract.ContractError):
                contract.validate_plan(self.plan, self.target)
            expressions.clear()
            expressions.update(saved)

    def test_rejects_malformed_target_and_unknown_fields(self):
        for key, value in (("schemaVersion", True), ("environment", "staging"),
                           ("projectID", ""), ("databaseID", "../wrong"), ("unexpected", "synthetic")):
            with self.assertRaises(contract.ContractError):
                contract.validate_plan(self.plan, self.target | {key: value})

    def cli(self, plan_bytes, target_bytes=None, extra=()):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            plan_path = root / "PRIVATE_PATH_REGRESSION_MARKER.plan.json"
            target_path = root / "target.json"
            plan_path.write_bytes(plan_bytes)
            target_path.write_bytes(target_bytes or json.dumps(self.target).encode())
            result = subprocess.run([sys.executable, str(pathlib.Path(contract.__file__)),
                "--plan", str(plan_path), "--target", str(target_path), *extra],
                capture_output=True, text=True, check=False)
            self.assertEqual(result.stderr, "")
            self.assertNotIn("PRIVATE", result.stdout)
            return result, json.loads(result.stdout)

    def test_cli_success_is_hash_bound_and_never_authorizes_execution(self):
        result, report = self.cli(json.dumps(self.plan).encode())
        self.assertEqual(result.returncode, 0)
        self.assertEqual(report["status"], "accepted")
        self.assertEqual(len(report["planSHA256"]), 64)
        self.assertIs(report["executionAuthorized"], False)
        self.assertIs(report["releaseReady"], False)

    def test_cli_rejects_private_malformed_json_duplicate_nonfinite_and_arguments(self):
        for payload in (b'{"PRIVATE_RAW_REGRESSION_MARKER":', b'{"x":1,"x":2}',
                        b'{"x":NaN}', b'\xff', b'[]'):
            result, report = self.cli(payload)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(report["status"], "rejected")
        result, report = self.cli(json.dumps(self.plan).encode(), extra=("--PRIVATE_ARGUMENT_MARKER",))
        self.assertEqual(result.returncode, 1)
        self.assertEqual(report["code"], "arguments_invalid")

    def test_cli_rejects_oversized_target_without_echo(self):
        result, report = self.cli(json.dumps(self.plan).encode(), b' ' * (64 * 1024 + 1))
        self.assertEqual(result.returncode, 1)
        self.assertEqual(report["code"], "input_too_large")


if __name__ == "__main__":
    unittest.main()
