#!/usr/bin/env python3
"""Migration acceptance is semantic and deterministic; fixtures contain no cloud data."""
from copy import deepcopy
import contextlib
import io
import json
import sys
import unittest

sys.dont_write_bytecode = True
import terraform_migration_contract as gate


def descriptors(environment="dev"):
    project = gate.ENVIRONMENTS[environment][0]
    return [
        {"address": "module.history.google_bigquery_table.retained", "type": "google_bigquery_table",
         "import_id": f"projects/{project}/datasets/fixture/tables/history"},
        {"address": 'module.topics.google_pubsub_topic.retained["topic-a"]', "type": "google_pubsub_topic",
         "import_id": f"projects/{project}/topics/topic-a"},
    ]


def values(environment="dev"):
    table, topic = descriptors(environment)
    project = gate.ENVIRONMENTS[environment][0]
    return [
        {"id": table["import_id"], "project": project, "dataset_id": "fixture", "table_id": "history",
         "schema": '[{"name":"fixture","type":"STRING"}]', "labels": {}, "etag": "fixture-etag-1", "last_modified_time": 1},
        {"id": topic["import_id"], "project": project, "name": "topic-a", "labels": {}},
    ]


def manifest(environment="dev"):
    return {"schema_version": "gcp-migration-manifest-v1", "environment": environment,
            "project_id": gate.ENVIRONMENTS[environment][0], "resources": descriptors(environment)}


def provider_schema():
    blocks = {}
    for descriptor, resource_values in zip(descriptors(), values()):
        attrs = {key: {"type": "string", "optional": True} for key in resource_values}
        if descriptor["type"] == "google_bigquery_table":
            attrs["etag"] = {"type": "string", "computed": True}
            attrs["last_modified_time"] = {"type": "number", "computed": True}
        blocks[descriptor["type"]] = {"version": 0, "block": {"attributes": attrs}}
    return {"format_version": "1.0", "provider_schemas": {gate.GOOGLE: {"resource_schemas": blocks}}}


def outputs(environment="dev"):
    project, label = gate.ENVIRONMENTS[environment]
    return {"environment": {"type": "string", "value": label, "sensitive": False},
            "project_id": {"type": "string", "value": project, "sensitive": False}}


def state(environment="dev", *, count=1, serial=2):
    resources = []
    for descriptor, resource_values in list(zip(descriptors(environment), values(environment)))[:count]:
        match = gate.ADDRESS.fullmatch(descriptor["address"])
        instance = {"schema_version": 0, "attributes": resource_values, "sensitive_attributes": [],
                    "private": "fixture-provider-cache", "dependencies": []}
        if match["index"]:
            instance["index_key"] = json.loads(match["index"][1:-1])
        resources.append({"module": match["modules"].removesuffix("."), "mode": "managed",
                          "type": descriptor["type"], "name": match["name"],
                          "provider": f'provider["{gate.GOOGLE}"]', "instances": [instance]})
    return {"version": 4, "terraform_version": gate.TERRAFORM_VERSION, "serial": serial,
            "lineage": "12345678-1234-1234-1234-123456789abc", "outputs": outputs(environment),
            "resources": resources, "check_results": None}


def plan(environment="dev", *, importing=True):
    changes, children = [], []
    for number, (descriptor, resource_values) in enumerate(zip(descriptors(environment), values(environment))):
        match = gate.ADDRESS.fullmatch(descriptor["address"])
        module = match["modules"].removesuffix(".")
        header = {"address": descriptor["address"], "mode": "managed", "type": descriptor["type"],
                  "name": match["name"], "provider_name": gate.GOOGLE}
        if match["index"]:
            header["index"] = json.loads(match["index"][1:-1])
        change = {"actions": ["no-op"], "before": deepcopy(resource_values), "after": deepcopy(resource_values),
                  "before_sensitive": {}, "after_sensitive": {}, "after_unknown": {}}
        if number == 1 and importing:
            change["importing"] = {"id": descriptor["import_id"]}
        changes.append(dict(header, module_address=module, change=change))
        shown = dict(header, schema_version=0, values=deepcopy(resource_values), sensitive_values={})
        children.append({"address": module, "resources": [shown]})
    return {"format_version": "1.2", "terraform_version": gate.TERRAFORM_VERSION,
            "complete": True, "errored": False, "applyable": importing, "timestamp": "2026-10-16T00:00:00Z",
            "resource_changes": changes, "resource_drift": [], "checks": [],
            "planned_values": {"outputs": outputs(environment), "root_module": {"child_modules": children}},
            "output_changes": {key: {"actions": ["no-op"], "before": value["value"], "after": value["value"],
                                      "after_unknown": False, "before_sensitive": False, "after_sensitive": False}
                               for key, value in outputs(environment).items()},
            "configuration": {"provider_config": {"google": {"full_name": gate.GOOGLE}},
                              "root_module": {"module_calls": {"history": {"source": "../../modules/fixture"}}}},
            "variables": {"project_id": {"value": gate.ENVIRONMENTS[environment][0]},
                          "fixture_private": {"value": {"baseline": "fixed-private-sentinel"}}}}


def observation(value, *, etag="fixture-etag-2", modified=2):
    drift = deepcopy(value["resource_changes"][0])
    drift["change"]["actions"] = ["update"]
    drift["change"]["after"].update(etag=etag, last_modified_time=modified)
    for side in ("before", "after"):
        value["resource_changes"][0]["change"][side].update(etag=etag, last_modified_time=modified)
    value["planned_values"]["root_module"]["child_modules"][0]["resources"][0]["values"].update(etag=etag, last_modified_time=modified)
    value["resource_drift"] = [drift]
    return value


def identity_fixture():
    saved, before, after, schema = plan(), state(), state(count=2, serial=3), provider_schema()
    identity_schemas = {
        "google_bigquery_table": {"version": 1, "attributes": {
            "project": {"type": "string", "optional_for_import": True},
            "dataset_id": {"type": "string", "required_for_import": True},
            "table_id": {"type": "string", "required_for_import": True}}},
        "google_pubsub_topic": {"version": 1, "attributes": {
            "project": {"type": "string", "optional_for_import": True},
            "name": {"type": "string", "required_for_import": True}}},
    }
    schema["provider_schemas"][gate.GOOGLE]["resource_identity_schemas"] = identity_schemas
    for index, change in enumerate(saved["resource_changes"]):
        definition = identity_schemas[change["type"]]
        identity = {key: change["change"]["after"][key] for key in definition["attributes"]}
        change["change"].update(before_identity=deepcopy(identity), after_identity=deepcopy(identity))
        resource = saved["planned_values"]["root_module"]["child_modules"][index]["resources"][0]
        resource.update(identity=deepcopy(identity), identity_schema_version=1)
        after["resources"][index]["instances"][0].update(identity=deepcopy(identity), identity_schema_version=1)
        if index == 0:
            before["resources"][index]["instances"][0].update(identity=deepcopy(identity), identity_schema_version=1)
    return saved, before, after, schema


class MigrationContractTest(unittest.TestCase):
    def validate(self, value=None, *, environment="dev", before=None, phase="before", custom_manifest=None, schema=None):
        return gate.validate_plan(plan(environment) if value is None else value,
            manifest=manifest(environment) if custom_manifest is None else custom_manifest,
            environment=environment, provider_schema=provider_schema() if schema is None else schema,
            state=state(environment) if before is None else before, phase=phase)

    def reject_plan(self, edit, *, original=None, **kwargs):
        candidate = deepcopy(plan() if original is None else original)
        edit(candidate)
        with self.assertRaises(gate.ContractError):
            self.validate(candidate, **kwargs)

    def test_both_environments_accept_one_exact_import_and_full_post_noop(self):
        for environment in gate.ENVIRONMENTS:
            result = self.validate(environment=environment)
            self.assertEqual((result["managed"], result["managed_before"], result["imports"], result["mutations"]), (2, 1, 1, 0))
            self.assertFalse(result["historical_delta_attested"])
            post = self.validate(plan(environment, importing=False), environment=environment,
                                 before=state(environment, count=2), phase="post")
            self.assertEqual((post["imports"], post["no_op"]), (0, 2))

    def test_one_remote_resource_cannot_be_owned_by_distinct_manifest_addresses(self):
        for environment in gate.ENVIRONMENTS:
            candidate = manifest(environment)
            duplicate = deepcopy(candidate["resources"][0])
            duplicate["address"] += "_another_owner"
            candidate["resources"].append(duplicate)
            with self.subTest(environment=environment), self.assertRaises(gate.ContractError) as error:
                gate.validate_manifest(candidate, environment)
            self.assertEqual(error.exception.reason, "MANIFEST")
            self.assertEqual(str(error.exception), "GCP migration contract STOP: MANIFEST")

    def test_distinct_types_or_exact_import_ids_remain_separate_remote_identities(self):
        for environment in gate.ENVIRONMENTS:
            candidate = manifest(environment)
            distinct_id = deepcopy(candidate["resources"][0])
            distinct_id["address"] += "_another_table"
            distinct_id["import_id"] += "_another_table"
            distinct_type = deepcopy(candidate["resources"][0])
            distinct_type.update(address="module.topics.google_pubsub_topic.separate_type",
                                 type="google_pubsub_topic")
            # The manifest identifies remote ownership by (type, exact ID).
            # Provider-specific ID syntax and value binding are checked later.
            candidate["resources"].extend([distinct_id, distinct_type])
            with self.subTest(environment=environment):
                actual = gate.validate_manifest(candidate, environment)
                self.assertEqual(actual, {item["address"]: item for item in candidate["resources"]})

    def test_existing_graph_and_import_ids_must_match_exact_candidate_delta(self):
        self.reject_plan(lambda value: value["resource_changes"][1]["change"].pop("importing"))
        self.reject_plan(lambda value: value["resource_changes"][0]["change"].update(importing={"id": descriptors()[0]["import_id"]}))
        self.reject_plan(lambda value: value["resource_changes"][1]["change"].update(importing={"id": "different-private-id"}))
        self.reject_plan(lambda value: value["resource_changes"].append(deepcopy(value["resource_changes"][0])))
        self.reject_plan(lambda value: value["planned_values"]["root_module"]["child_modules"].pop())
        self.reject_plan(lambda value: value["resource_changes"][1].update(mode="data"))
        self.reject_plan(lambda value: value["resource_changes"][1].update(provider_name="registry.terraform.io/other/google"))
        self.reject_plan(lambda value: None, before=state(count=2))
        self.reject_plan(lambda value: None, phase="post")

    def test_mutations_unknowns_replacements_moves_and_deferred_execution_are_rejected(self):
        for actions in (["create"], ["update"], ["delete"], ["read"], ["forget"], ["delete", "create"]):
            with self.subTest(actions=actions):
                self.reject_plan(lambda value: value["resource_changes"][0]["change"].update(actions=actions))
        for field in ("previous_address", "deposed"):
            self.reject_plan(lambda value: value["resource_changes"][0].update({field: "unapproved"}))
        for field in ("deferred_changes", "action_invocations", "deferred_action_invocations"):
            self.reject_plan(lambda value: value.update({field: [{}]}))
        self.reject_plan(lambda value: value["resource_changes"][0]["change"].update(replace_paths=[["labels"]]))
        self.reject_plan(lambda value: value["resource_changes"][0]["change"].update(after_unknown={"labels": {"nested": [True]}}))
        self.reject_plan(lambda value: value["resource_changes"][0]["change"].update(after_unknown={"labels": None}))
        self.reject_plan(lambda value: value.update(complete=False))
        self.reject_plan(lambda value: value.update(errored=True))
        self.reject_plan(lambda value: value.update(unreviewed_future_execution=[]))

    def test_noop_action_cannot_hide_values_masks_outputs_or_failing_checks(self):
        self.reject_plan(lambda value: value["resource_changes"][0]["change"]["after"].update(labels={"changed": "yes"}))
        self.reject_plan(lambda value: value["resource_changes"][0]["change"].update(after_sensitive={"schema": True}))
        self.reject_plan(lambda value: value["planned_values"]["root_module"]["child_modules"][0]["resources"][0]["values"].update(table_id="wrong"))
        self.reject_plan(lambda value: value["planned_values"]["root_module"]["child_modules"][0]["resources"][0].update(sensitive_values={"schema": True}))
        self.reject_plan(lambda value: value["output_changes"]["environment"].update(actions=["create"]))
        self.reject_plan(lambda value: value["output_changes"]["project_id"].update(after_unknown=True))
        self.reject_plan(lambda value: value["output_changes"].update(extra={"actions": ["no-op"]}))
        self.reject_plan(lambda value: value.update(checks=[{"status": "unknown"}]))
        self.reject_plan(lambda value: value.update(checks=[{"status": "pass", "instances": [{"status": "fail"}]}]))

    def test_only_computed_bq_observation_drift_is_accepted_and_is_reported_raw(self):
        result = self.validate(observation(plan()))
        self.assertEqual((result["raw_observation_drift"], result["observation_leaf_changes"]), (1, 2))
        self.assertEqual(result["mutations"], 0)
        single = observation(plan(), modified=1)
        self.assertEqual(self.validate(single)["observation_leaf_changes"], 1)
        self.reject_plan(lambda value: value["resource_drift"][0]["change"]["after"].update(labels={"not": "observational"}), original=observation(plan()))
        self.reject_plan(lambda value: value["resource_drift"][0]["change"].update(actions=["no-op"]), original=observation(plan()))
        self.reject_plan(lambda value: value["resource_drift"].append(deepcopy(value["resource_drift"][0])), original=observation(plan()))
        self.reject_plan(lambda value: value["resource_drift"][0]["change"]["before"].update(etag="not-state"), original=observation(plan()))
        self.reject_plan(lambda value: value["resource_drift"][0]["change"].update(before_sensitive={"etag": True}, after_sensitive={"etag": True}), original=observation(plan()))
        self.reject_plan(lambda value: value["resource_drift"][0]["change"]["after"].update(last_modified_time=True), original=observation(plan()))
        self.reject_plan(lambda value: value.update(resource_drift=[deepcopy(value["resource_changes"][1])]))
        for name in gate.OBSERVATIONAL:
            for flag, bad in (("computed", False), ("optional", True), ("required", True)):
                schema = provider_schema()
                schema["provider_schemas"][gate.GOOGLE]["resource_schemas"]["google_bigquery_table"]["block"]["attributes"][name][flag] = bad
                with self.subTest(name=name, flag=flag):
                    with self.assertRaises(gate.ContractError):
                        self.validate(observation(plan()), schema=schema)

    def test_wrong_project_resource_type_schema_or_private_baseline_cannot_be_adopted(self):
        candidate = manifest()
        candidate["project_id"] = gate.ENVIRONMENTS["prod"][0]
        with self.assertRaises(gate.ContractError):
            self.validate(custom_manifest=candidate)
        for resource_type in ("google_project_iam_policy", "google_project_iam_binding", "google_compute_instance"):
            candidate = manifest()
            candidate["resources"][0].update(type=resource_type)
            with self.assertRaises(gate.ContractError):
                self.validate(custom_manifest=candidate)
        self.reject_plan(lambda value: value["resource_changes"][0]["change"]["after"].update(project="foreign-project"))
        self.reject_plan(lambda value: value["planned_values"]["root_module"]["child_modules"][0]["resources"][0].update(schema_version=99))
        self.reject_plan(lambda value: value["resource_changes"][0]["change"]["after"].update(unknown_provider_field="unreviewed"))
        candidate = manifest()
        candidate["resources"][0]["values"] = deepcopy(values()[0])
        candidate["resources"][0]["values"]["schema"] = "different-private-baseline"
        with self.assertRaises(gate.ContractError):
            self.validate(custom_manifest=candidate)

    def test_native_before_state_cannot_disappear_or_change_without_reported_drift(self):
        before = state()
        before["resources"][0]["instances"][0]["attributes"]["schema"] = "unexpected-state-value"
        with self.assertRaises(gate.ContractError):
            self.validate(before=before)
        for edit in (
            lambda value: value.update(serial=True),
            lambda value: value.update(lineage="not-a-uuid"),
            lambda value: value["resources"][0].update(provider='provider["other"]'),
            lambda value: value["resources"][0]["instances"].append(deepcopy(value["resources"][0]["instances"][0])),
            lambda value: value["resources"][0].update(instances=[]),
            lambda value: value["resources"][0]["instances"][0].update(deposed="unreviewed"),
        ):
            before = state()
            edit(before)
            with self.assertRaises(gate.ContractError):
                self.validate(before=before)

    def test_private_commitment_retains_all_execution_inputs_but_not_observation_values(self):
        first, second = observation(plan()), observation(plan(), etag="fixture-etag-3", modified=3)
        second["timestamp"] = "2026-10-16T00:05:00Z"
        result = gate.compare_replans(first, second, manifest=manifest(), environment="dev", provider_schema=provider_schema(), state=state())
        self.assertTrue(result["matched"])
        payload = gate.plan_commitment_payload(first, manifest(), "dev", provider_schema(), state=state())
        self.assertIn("configuration", payload)
        self.assertIn("variables", payload)
        self.assertIn("fixed-private-sentinel", gate.canonical(payload))
        self.assertNotIn("timestamp", payload)
        for edit in (
            lambda value: value["variables"]["fixture_private"]["value"].update(baseline="changed-private-input"),
            lambda value: value["configuration"]["root_module"]["module_calls"]["history"].update(source="different-module"),
            lambda value: value.update(checks=[{"address": {"kind": "var", "to_display": "var.fixture"}, "status": "pass", "instances": []}]),
            lambda value: value["resource_changes"][1]["change"].update(before_sensitive={"labels": False}, after_sensitive={"labels": False}),
        ):
            changed = deepcopy(second)
            edit(changed)
            if changed["resource_changes"][1]["change"]["after_sensitive"]:
                changed["planned_values"]["root_module"]["child_modules"][1]["resources"][0]["sensitive_values"] = {"labels": False}
            with self.assertRaises(gate.ContractError):
                gate.compare_replans(first, changed, manifest=manifest(), environment="dev", provider_schema=provider_schema(), state=state())

    def test_post_state_may_add_only_the_exact_import_and_bq_observation_values(self):
        before, after = state(), state(count=2, serial=3)
        after["resources"][0]["instances"][0]["attributes"].update(etag="fixture-etag-2", last_modified_time=2)
        result = gate.validate_state_delta(before, after, manifest=manifest(), environment="dev", provider_schema=provider_schema(), expected_imports={descriptors()[1]["address"]})
        self.assertEqual((result["managed_before"], result["managed_after"], result["imports"]), (1, 2, 1))
        for edit in (
            lambda value: value.update(serial=2),
            lambda value: value.update(lineage="87654321-1234-1234-1234-123456789abc"),
            lambda value: value["resources"][0]["instances"][0]["attributes"].update(labels={"changed": "yes"}),
            lambda value: value["resources"][0]["instances"][0].update(private="unreviewed-cache-change"),
            lambda value: value["resources"][0]["instances"][0].update(identity={"id": "different"}),
            lambda value: value["resources"][0]["instances"][0].update(sensitive_attributes=[[{"type": "get_attr", "value": "schema"}]]),
        ):
            changed = deepcopy(after)
            edit(changed)
            with self.assertRaises(gate.ContractError):
                gate.validate_state_delta(before, changed, manifest=manifest(), environment="dev", provider_schema=provider_schema(), expected_imports={descriptors()[1]["address"]})

    def test_state_check_sensitivity_and_dependency_ordering_does_not_change_semantics(self):
        before, after = state(), state(count=2, serial=3)
        paths = [[{"type": "get_attr", "value": "schema"}], [{"type": "get_attr", "value": "labels"}]]
        for value in (before, after):
            instance = value["resources"][0]["instances"][0]
            instance.update(sensitive_attributes=deepcopy(paths), dependencies=["module.a", "module.b"])
            value["check_results"] = [
                {"object_kind": "var", "config_addr": "var.one", "status": "pass", "objects": [{"object_addr": "var.one", "status": "pass"}]},
                {"object_kind": "var", "config_addr": "var.two", "status": "pass", "objects": [{"object_addr": "var.two", "status": "pass"}]},
            ]
        after["resources"][0]["instances"][0]["sensitive_attributes"].reverse()
        after["resources"][0]["instances"][0]["dependencies"].reverse()
        after["check_results"].reverse()
        gate.validate_state_delta(before, after, manifest=manifest(), environment="dev", provider_schema=provider_schema(), expected_imports={descriptors()[1]["address"]})
        after["check_results"][0]["objects"][0]["status"] = "fail"
        with self.assertRaises(gate.ContractError):
            gate.validate_state_delta(before, after, manifest=manifest(), environment="dev", provider_schema=provider_schema(), expected_imports={descriptors()[1]["address"]})

    def test_new_native_check_cache_must_exactly_match_current_saved_plan_checks(self):
        before, after, saved = state(), state(count=2, serial=3), plan()
        old = {"object_kind": "var", "config_addr": "var.project_id", "status": "pass",
               "objects": [{"object_addr": "var.project_id", "status": "pass"}]}
        new = {"object_kind": "var", "config_addr": "module.export_function.var.environment", "status": "pass",
               "objects": [{"object_addr": "module.export_function[0].var.environment", "status": "pass"}]}
        before["check_results"] = [deepcopy(old)]
        after["check_results"] = [deepcopy(new), deepcopy(old)]
        saved["checks"] = [
            {"address": {"to_display": "var.project_id", "kind": "var", "name": "project_id"}, "status": "pass",
             "instances": [{"address": {"to_display": "var.project_id"}, "status": "pass"}]},
            {"address": {"to_display": "module.export_function.var.environment", "kind": "var", "name": "environment", "module": "module.export_function"},
             "status": "pass", "instances": [{"address": {"to_display": "module.export_function[0].var.environment", "module": "module.export_function[0]"}, "status": "pass"}]},
        ]
        arguments = dict(manifest=manifest(), environment="dev", provider_schema=provider_schema(), expected_imports={descriptors()[1]["address"]}, plan=saved)
        self.assertTrue(gate.validate_state_delta(before, after, **arguments)["saved_plan_checks_match"])
        for edit in (
            lambda value: value["check_results"].pop(),
            lambda value: value["check_results"].append({"object_kind": "var", "config_addr": "var.extra", "status": "pass", "objects": []}),
            lambda value: value["check_results"][0]["objects"][0].update(object_addr="module.export_function[1].var.environment"),
            lambda value: value["check_results"][0].update(objects=False),
        ):
            changed = deepcopy(after)
            edit(changed)
            with self.assertRaises(gate.ContractError):
                gate.validate_state_delta(before, changed, **arguments)
        invalid = deepcopy(saved)
        invalid["checks"][1]["instances"][0]["address"]["module"] = "module.other[0]"
        with self.assertRaises(gate.ContractError):
            gate.validate_state_delta(before, after, **dict(arguments, plan=invalid))

    def test_every_imported_state_value_must_match_saved_plan_even_without_manifest_values(self):
        before, after, saved = state(), state(count=2, serial=3), plan()
        args = dict(manifest=manifest(), environment="dev", provider_schema=provider_schema(),
                    expected_imports={descriptors()[1]["address"]}, plan=saved)
        self.assertTrue(gate.validate_state_delta(before, after, **args)["existing_values_unchanged"])
        after["resources"][1]["instances"][0]["attributes"]["labels"] = {"unplanned": "value"}
        with self.assertRaises(gate.ContractError):
            gate.validate_state_delta(before, after, **args)

    def test_false_or_malformed_check_containers_cannot_be_normalized_to_empty(self):
        for field in ("instances",):
            for bad in (False, 0, "", {}):
                self.reject_plan(lambda value: value.update(checks=[{"address": {"to_display": "var.fixture", "kind": "var", "name": "fixture"}, "status": "pass", field: bad}]))
        self.reject_plan(lambda value: value.update(checks=[{"address": {"to_display": "var.fixture", "kind": "resource", "name": "fixture"}, "status": "pass", "instances": []}]))

    def test_noop_apply_can_leave_state_untouched_but_observation_persistence_increases_serial(self):
        before = state(count=2, serial=5)
        args = dict(manifest=manifest(), environment="dev", provider_schema=provider_schema(), expected_imports=[])
        self.assertEqual(gate.validate_state_delta(before, deepcopy(before), **args)["imports"], 0)
        after = deepcopy(before)
        after["resources"][0]["instances"][0]["attributes"].update(etag="new-observation", last_modified_time=2)
        with self.assertRaises(gate.ContractError):
            gate.validate_state_delta(before, after, **args)
        after["serial"] += 1
        gate.validate_state_delta(before, after, **args)

    def test_imported_iam_member_is_only_exact_project_appspot_aws_runtime_role_scope(self):
        project, number = gate.ENVIRONMENTS["dev"][0], "111111111111"
        account = f"projects/{project}/serviceAccounts/{project}@appspot.gserviceaccount.com"
        member = f"principalSet://iam.googleapis.com/projects/{number}/locations/global/workloadIdentityPools/aws-runtime/attribute.aws_role/fixture-role"
        resource_values = {"id": account + "/roles/iam.workloadIdentityUser/" + member,
                           "service_account_id": account, "role": "roles/iam.workloadIdentityUser", "member": member, "condition": []}
        descriptor = {"address": 'module.runtime_wif.google_service_account_iam_member.runtime["grant-01"]',
                      "type": "google_service_account_iam_member", "import_id": account + " roles/iam.workloadIdentityUser " + member}
        fixed = manifest()
        fixed.update(project_number=number)
        fixed["resources"][1] = descriptor
        schema = provider_schema()
        schema["provider_schemas"][gate.GOOGLE]["resource_schemas"][descriptor["type"]] = {"version": 0, "block": {"attributes": {key: {"computed": True} for key in resource_values}}}
        saved = plan()
        header = {"address": descriptor["address"], "mode": "managed", "type": descriptor["type"], "name": "runtime", "index": "grant-01", "provider_name": gate.GOOGLE}
        change = {"actions": ["no-op"], "before": deepcopy(resource_values), "after": deepcopy(resource_values),
                  "before_sensitive": {}, "after_sensitive": {}, "after_unknown": {}, "importing": {"id": descriptor["import_id"]}}
        saved["resource_changes"][1] = dict(header, module_address="module.runtime_wif", change=change)
        saved["planned_values"]["root_module"]["child_modules"][1] = {"address": "module.runtime_wif", "resources": [dict(header, schema_version=0, values=deepcopy(resource_values), sensitive_values={})]}
        self.validate(saved, custom_manifest=fixed, schema=schema)
        for bad_member in (member + "*", member.replace("aws-runtime", "other-pool"), member.replace(number, "222222222222"), member.replace("attribute.aws_role", "subject")):
            changed = deepcopy(saved)
            for side in ("before", "after"):
                changed["resource_changes"][1]["change"][side]["member"] = bad_member
            changed["planned_values"]["root_module"]["child_modules"][1]["resources"][0]["values"]["member"] = bad_member
            with self.assertRaises(gate.ContractError):
                self.validate(changed, custom_manifest=fixed, schema=schema)
        for edit in (
            lambda item: item.update(role="roles/owner"),
            lambda item: item.update(service_account_id=f"projects/{project}/serviceAccounts/other@{project}.iam.gserviceaccount.com"),
        ):
            changed = deepcopy(saved)
            edit(changed["resource_changes"][1]["change"]["after"])
            with self.assertRaises(gate.ContractError):
                self.validate(changed, custom_manifest=fixed, schema=schema)

    def test_prior_state_is_checked_when_present_and_same_project_is_required(self):
        candidate = plan()
        candidate["prior_state"] = {"format_version": "1.0", "values": deepcopy(candidate["planned_values"])}
        self.validate(candidate)
        self.reject_plan(lambda value: value["prior_state"]["values"]["root_module"]["child_modules"][0]["resources"][0]["values"].update(etag="different-from-current-plan"), original=candidate)
        self.reject_plan(lambda value: value["prior_state"]["values"]["root_module"]["child_modules"].clear(), original=candidate)

    def test_native_identity_schema_binds_existing_and_imported_plan_and_state(self):
        saved, before, after, schema = identity_fixture()
        result = self.validate(saved, before=before, schema=schema)
        self.assertEqual(result["imports"], 1)
        args = dict(manifest=manifest(), environment="dev", provider_schema=schema,
                    expected_imports={descriptors()[1]["address"]}, plan=saved)
        gate.validate_state_delta(before, after, **args)
        self.validate(observation(deepcopy(saved)), before=before, schema=schema)
        candidate = deepcopy(saved)
        candidate["prior_state"] = {"format_version": "1.0", "values": deepcopy(candidate["planned_values"])}
        self.validate(candidate, before=before, schema=schema)
        candidate["prior_state"]["values"]["root_module"]["child_modules"][0]["resources"][0]["identity"] = None
        candidate["prior_state"]["values"]["root_module"]["child_modules"][0]["resources"][0]["identity_schema_version"] = 0
        with self.assertRaises(gate.ContractError):
            self.validate(candidate, before=before, schema=schema)

    def test_changed_unknown_typed_or_deleted_identity_is_rejected(self):
        saved, before, _, schema = identity_fixture()
        edits = [
            lambda value: value["resource_changes"][0]["change"]["after_identity"].update(project=None),
            lambda value: value["resource_changes"][0]["change"]["after_identity"].update(table_id=True),
            lambda value: value["resource_changes"][0]["change"].update(after_identity_unknown={"table_id": True}),
            lambda value: value["resource_changes"][0]["change"]["after_identity"].update(unknown_key="unreviewed"),
            lambda value: value["resource_changes"][0]["change"].pop("after_identity"),
            lambda value: value["resource_changes"][0]["change"].update(before_identity=None, after_identity=None),
            lambda value: value["planned_values"]["root_module"]["child_modules"][0]["resources"][0].update(identity_schema_version=0),
        ]
        for edit in edits:
            candidate = deepcopy(saved)
            edit(candidate)
            with self.assertRaises(gate.ContractError):
                self.validate(candidate, before=before, schema=schema)
        for field in ("before_identity", "after_identity"):
            candidate = deepcopy(saved)
            candidate["resource_changes"][0]["change"][field].pop("dataset_id")
            with self.assertRaises(gate.ContractError):
                self.validate(candidate, before=before, schema=schema)
        for edit in (
            lambda value: value["resources"][0]["instances"][0]["identity"].update(table_id="different-resource"),
            lambda value: value["resources"][0]["instances"][0].update(identity_schema_version=True),
        ):
            changed = deepcopy(before)
            edit(changed)
            with self.assertRaises(gate.ContractError):
                self.validate(saved, before=changed, schema=schema)

    def test_identity_schema_changes_require_review_and_import_state_cannot_drop_identity(self):
        saved, before, after, schema = identity_fixture()
        for edit in (
            lambda value: value["provider_schemas"][gate.GOOGLE].pop("resource_identity_schemas"),
            lambda value: value["provider_schemas"][gate.GOOGLE]["resource_identity_schemas"]["google_bigquery_table"].update(version=2),
            lambda value: value["provider_schemas"][gate.GOOGLE]["resource_identity_schemas"]["google_bigquery_table"]["attributes"]["table_id"].update(type="number"),
            lambda value: value["provider_schemas"][gate.GOOGLE]["resource_identity_schemas"]["google_bigquery_table"]["attributes"]["table_id"].update(optional_for_import=True),
        ):
            changed = deepcopy(schema)
            edit(changed)
            with self.assertRaises(gate.ContractError):
                self.validate(saved, before=before, schema=changed)
        for edit in (
            lambda value: value["resources"][1]["instances"][0].update(identity=None, identity_schema_version=0),
            lambda value: value["resources"][1]["instances"][0]["identity"].update(project=None),
        ):
            changed = deepcopy(after)
            edit(changed)
            with self.assertRaises(gate.ContractError):
                gate.validate_state_delta(before, changed, manifest=manifest(), environment="dev", provider_schema=schema,
                                         expected_imports={descriptors()[1]["address"]}, plan=saved)

    def test_private_commitment_retains_import_identity_even_when_optional_project_is_null(self):
        first, before, _, schema = identity_fixture()
        second = deepcopy(first)
        for field in ("before_identity", "after_identity"):
            second["resource_changes"][1]["change"][field]["project"] = None
        second["planned_values"]["root_module"]["child_modules"][1]["resources"][0]["identity"]["project"] = None
        # Each null is known and optional in this provider identity schema. The
        # private approved projection must nevertheless retain its exact value.
        self.validate(second, before=before, schema=schema)
        with self.assertRaises(gate.ContractError):
            gate.compare_replans(first, second, manifest=manifest(), environment="dev", provider_schema=schema, state=before)

    def test_strict_decoder_and_failures_never_print_or_echo_private_values(self):
        for raw in ('{"private":"sentinel","private":"other"}', '{"private":NaN}', '{"private":Infinity}'):
            with self.assertRaises(gate.ContractError):
                gate.loads(raw)
        output = io.StringIO()
        with contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
            with self.assertRaises(gate.ContractError) as error:
                self.validate({"private": "PRIVATE-SENTINEL"})
        self.assertEqual(output.getvalue(), "")
        self.assertNotIn("PRIVATE-SENTINEL", str(error.exception))


if __name__ == "__main__":
    unittest.main()
