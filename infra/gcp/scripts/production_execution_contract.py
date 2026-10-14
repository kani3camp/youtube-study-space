#!/usr/bin/env python3
"""Offline production plan/state/metadata contract; no cloud or Terraform calls.

Inputs are private snapshots supplied by a future authenticated collector. A
successful offline validation is not authenticated evidence or an apply grant.
"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import re
import stat
import sys
import uuid

from production_backend_packet import PrivateArgumentParser, STATE_KEY, WORKSPACE_PREFIX, private_path, unique_object

TERRAFORM_VERSION = (Path(__file__).resolve().parents[1] / ".terraform-version").read_text().strip()
GOOGLE = "registry.terraform.io/hashicorp/google"
PROJECT = "youtube-study-space"
OUTPUTS = {"environment": "production", "project_id": PROJECT}
MAX_STATE_BYTES = 4 * 1024 * 1024
MAX_BUNDLE_BYTES = 16 * 1024 * 1024
# Resource types already represented in the migration source. Whole IAM policy
# ownership, generated resources and new resource types require another review.
RESOURCE_TYPES = {
    "google_storage_bucket", "google_bigquery_dataset", "google_bigquery_table",
    "google_firestore_backup_schedule", "google_monitoring_notification_channel",
    "google_monitoring_alert_policy", "google_pubsub_topic", "google_cloud_scheduler_job",
    "google_cloudfunctions_function", "google_iam_workload_identity_pool",
    "google_iam_workload_identity_pool_provider", "google_service_account_iam_member",
    "google_project_service",
}
ADDRESS = re.compile(r'(?P<modules>(?:module\.[A-Za-z_][A-Za-z0-9_-]*(?:\[(?:[0-9]+|"(?:[^"\\]|\\.)*")\])?\.)*)(?P<type>google_[a-z0-9_]+)\.(?P<name>[A-Za-z_][A-Za-z0-9_-]*)(?P<index>\[(?:[0-9]+|"(?:[^"\\]|\\.)*")\])?')


def need(condition):
    if not condition:
        raise ValueError("Production private contract rejected")


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)


def no_unknown(value):
    if type(value) is dict:
        return all(no_unknown(item) for item in value.values())
    if type(value) is list:
        return all(no_unknown(item) for item in value)
    return value is False


def contract_shape(contract, git_sha, now):
    need(type(contract) is dict and set(contract) == {
        "schema_version", "git_sha", "issued_utc", "expires_utc", "project_id", "project_number",
        "state_bucket", "state_key", "wave", "adopted", "candidate",
    })
    need(contract["schema_version"] == "production-execution-contract-v1")
    need(type(git_sha) is str and re.fullmatch(r"[0-9a-f]{40}", git_sha) and contract["git_sha"] == git_sha)
    need(contract["project_id"] == PROJECT and type(contract["project_number"]) is str
         and re.fullmatch(r"[0-9]{6,20}", contract["project_number"]))
    need(contract["state_key"] == STATE_KEY and type(contract["state_bucket"]) is str
         and re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", contract["state_bucket"]))
    issued = datetime.strptime(contract["issued_utc"], "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    expires = datetime.strptime(contract["expires_utc"], "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    need(issued <= now < expires and issued < expires <= issued + timedelta(hours=24))
    need(contract["wave"] in {"initialize", "import", "noop"} and type(contract["adopted"]) is list)
    need((contract["wave"] == "import") == (type(contract["candidate"]) is dict))
    need(contract["wave"] == "import" or contract["candidate"] is None)
    need(contract["wave"] != "initialize" or contract["adopted"] == [])
    expected = {}
    for descriptor in contract["adopted"] + ([contract["candidate"]] if contract["candidate"] else []):
        need(type(descriptor) is dict and set(descriptor) == {"address", "type", "schema_version", "import_id", "values"})
        address = descriptor["address"]
        match = ADDRESS.fullmatch(address) if type(address) is str else None
        need(match is not None and match["type"] == descriptor["type"] and descriptor["type"] in RESOURCE_TYPES)
        need(address not in expected and type(descriptor["schema_version"]) is int
             and descriptor["schema_version"] == (4 if descriptor["type"] == "google_storage_bucket" else 0))
        need(type(descriptor["import_id"]) is str and bool(descriptor["import_id"]) and type(descriptor["values"]) is dict)
        values = descriptor["values"]
        need(type(values.get("id")) is str and bool(values["id"]))
        for identifier in (values["id"], descriptor["import_id"].split(" ", 1)[0]):
            if identifier.startswith("projects/"):
                need(any(identifier.startswith(f"projects/{project}/") for project in (PROJECT, contract["project_number"])))
        if descriptor["type"] == "google_service_account_iam_member":
            sa = values.get("service_account_id", "")
            need(type(sa) is str and any(sa.startswith(f"projects/{project}/serviceAccounts/")
                 for project in (PROJECT, contract["project_number"])))
            need(not re.search(r"\s", sa))
            need(values.get("role") == "roles/iam.workloadIdentityUser")
            member, conditions = values.get("member"), values.get("condition")
            need(type(member) is str and bool(member) and type(conditions) is list and len(conditions) <= 1)
            # The pinned importer leaves principalSet casing unchanged. Other
            # IAM principal kinds need their own normalization adapter/review.
            need(member.startswith(f'principalSet://iam.googleapis.com/projects/{contract["project_number"]}/locations/global/workloadIdentityPools/')
                 and not re.search(r"\s", member))
            if conditions:
                condition = conditions[0]
                need(type(condition) is dict and set(condition) == {"title", "description", "expression"}
                     and all(type(value) is str for value in condition.values()) and bool(condition["title"])
                     and not re.search(r"\s", condition["title"]))
            import_id = " ".join([sa, values["role"], member] + ([conditions[0]["title"]] if conditions else []))
            state_id = "/".join([sa, values["role"], member] + (
                [conditions[0][field] for field in ("title", "description", "expression")] if conditions else []))
            need(descriptor["import_id"] == import_id and values["id"] == state_id)
        else:
            need(type(values.get("project")) is str and values["project"] in {PROJECT, contract["project_number"]})
        if descriptor["type"] != "google_service_account_iam_member":
            need(descriptor["import_id"] == values["id"])
        if descriptor["type"] == "google_project_service":
            need(values.get("disable_on_destroy") is False and values.get("disable_dependent_services") is False)
            need(values["id"].startswith(PROJECT + "/") and descriptor["import_id"] == values["id"])
        canonical(values)
        expected[address] = descriptor
    return expected


def listing(value, *, bucket, prefix, expected):
    need(type(value) is dict and {"Name", "Prefix", "MaxKeys", "KeyCount", "IsTruncated"} <= set(value)
         <= {"Name", "Prefix", "MaxKeys", "KeyCount", "IsTruncated", "Contents"})
    contents = value.get("Contents", [])
    need(value["Name"] == bucket and value["Prefix"] == prefix and type(value["MaxKeys"]) is int
         and 1 <= value["MaxKeys"] <= 2 and value["IsTruncated"] is False and type(contents) is list)
    need(type(value["KeyCount"]) is int and value["KeyCount"] == len(contents) == len(expected))
    need(all(type(item) is dict and type(item.get("Key")) is str for item in contents))
    need([item["Key"] for item in contents] == expected)
    return contents


def state_shape(state, expected):
    need(type(state) is dict and {"version", "terraform_version", "serial", "lineage", "outputs", "resources"} <= set(state)
         <= {"version", "terraform_version", "serial", "lineage", "outputs", "resources", "check_results"})
    need(type(state["version"]) is int and state["version"] == 4 and state["terraform_version"] == TERRAFORM_VERSION)
    need(type(state["serial"]) is int and state["serial"] >= 0 and type(state["lineage"]) is str
         and str(uuid.UUID(state["lineage"])) == state["lineage"])
    need(type(state["outputs"]) is dict and set(state["outputs"]) == set(OUTPUTS))
    for name, value in OUTPUTS.items():
        output = state["outputs"][name]
        need(type(output) is dict and {"type", "value"} <= set(output) <= {"type", "value", "sensitive"}
             and output["type"] == "string" and output["value"] == value and output.get("sensitive", False) is False)
    checks = state.get("check_results")
    need(checks is None or (type(checks) is list and all(type(check) is dict and check.get("status") == "pass"
         and (check.get("objects") is None or (type(check["objects"]) is list and all(type(obj) is dict
              and obj.get("status") == "pass" for obj in check["objects"]))) for check in checks)))
    need(type(state["resources"]) is list)
    instances, containers = {}, {}
    for resource in state["resources"]:
        need(type(resource) is dict and {"mode", "type", "name", "provider", "instances"} <= set(resource)
             <= {"module", "mode", "type", "name", "provider", "instances"})
        need(resource["mode"] == "managed" and resource["provider"] == f'provider["{GOOGLE}"]')
        prefix = (resource["module"] + "." if "module" in resource else "") + resource["type"] + "." + resource["name"]
        need(prefix not in containers and type(resource["instances"]) is list and bool(resource["instances"]))
        containers[prefix] = {key: value for key, value in resource.items() if key != "instances"}
        for instance in resource["instances"]:
            need(type(instance) is dict and {"schema_version", "attributes"} <= set(instance) <= {
                "schema_version", "attributes", "sensitive_attributes", "private", "dependencies", "index_key",
                "identity_schema_version", "identity", "create_before_destroy",
            })
            index = instance.get("index_key")
            need("index_key" not in instance or (type(index) is str and bool(index)) or (type(index) is int and index >= 0))
            address = prefix + ("[" + canonical(index) + "]" if "index_key" in instance else "")
            need(address in expected and address not in instances and type(instance["schema_version"]) is int
                 and instance["schema_version"] == expected[address]["schema_version"])
            need(canonical(instance["attributes"]) == canonical(expected[address]["values"]))
            need(instance.get("create_before_destroy", False) is False and type(instance.get("sensitive_attributes", [])) is list)
            need(type(instance.get("private", "")) is str and type(instance.get("dependencies", [])) is list
                 and all(type(dependency) is str for dependency in instance.get("dependencies", [])))
            need(type(instance.get("identity_schema_version", 0)) is int and instance.get("identity_schema_version", 0) >= 0
                 and (instance.get("identity") is None or type(instance["identity"]) is dict))
            instances[address] = instance
    need(set(instances) == set(expected))
    return instances, containers


def snapshot_shape(snapshot, contract, expected, *, absent=False):
    need(type(snapshot) is dict and set(snapshot) == {
        "state_body", "head_before", "get_metadata", "head_after", "state_listing", "lock_listing", "workspace_listing",
    })
    for name, prefix in (("lock_listing", STATE_KEY + ".tflock"), ("workspace_listing", WORKSPACE_PREFIX + "/")):
        listing(snapshot[name], bucket=contract["state_bucket"], prefix=prefix, expected=[])
    listed = listing(snapshot["state_listing"], bucket=contract["state_bucket"], prefix=STATE_KEY,
                     expected=[] if absent else [STATE_KEY])
    if absent:
        need(not expected and all(snapshot[field] is None for field in ("state_body", "head_before", "get_metadata", "head_after")))
        return None, {}, {}
    head, body = snapshot["head_before"], snapshot["state_body"]
    need(type(head) is dict and canonical(head) == canonical(snapshot["head_after"]) and type(body) is str)
    need(type(snapshot["get_metadata"]) is dict and all(snapshot["get_metadata"].get(field) == head.get(field)
         for field in ("VersionId", "ETag", "ContentLength", "ServerSideEncryption")))
    need(type(head.get("ContentLength")) is int and 0 < head["ContentLength"] == len(body.encode()) <= MAX_STATE_BYTES)
    need(type(head.get("VersionId")) is str and head["VersionId"] not in {"", "null"}
         and type(head.get("ETag")) is str and bool(re.fullmatch(r'"[0-9a-f]{32}"', head["ETag"])))
    need(head.get("ServerSideEncryption") == "AES256" and head.get("StorageClass", "STANDARD") == "STANDARD")
    need(not any(head.get(key) for key in ("DeleteMarker", "Restore", "SSECustomerAlgorithm", "ContentEncoding")))
    need(listed[0].get("ETag") == head["ETag"] and type(listed[0].get("Size")) is int and listed[0]["Size"] == head["ContentLength"])
    state = json.loads(body, object_pairs_hook=unique_object, parse_constant=lambda _: need(False))
    instances, containers = state_shape(state, expected)
    return state, instances, containers


def plan_shape(plan, expected, *, imports, initialize=False):
    need(type(plan) is dict and plan.get("format_version") == "1.2" and plan.get("terraform_version") == TERRAFORM_VERSION
         and plan.get("complete") is True and plan.get("errored", False) is False)
    need(set(plan) <= {"format_version", "terraform_version", "planned_values", "resource_changes", "output_changes",
         "prior_state", "configuration", "relevant_attributes", "checks", "timestamp", "applyable", "complete", "errored",
         "variables", "resource_drift", "deferred_changes", "action_invocations", "deferred_action_invocations"})
    for field in ("resource_drift", "deferred_changes", "action_invocations", "deferred_action_invocations"):
        need(type(plan.get(field, [])) is list and plan.get(field, []) == [])
    checks = plan.get("checks", [])
    need(type(checks) is list and all(type(check) is dict and check.get("status") == "pass"
         and type(check.get("instances", [])) is list and all(type(item) is dict and item.get("status") == "pass"
             for item in check.get("instances", [])) for check in checks))
    changes = plan.get("resource_changes", [])
    need(type(changes) is list and len(changes) == len(expected))
    seen = set()
    for resource in changes:
        need(type(resource) is dict and resource.get("address") in expected and resource["address"] not in seen)
        address = resource["address"]
        seen.add(address)
        need(resource.get("mode") == "managed" and resource.get("type") == expected[address]["type"]
             and resource.get("provider_name") == GOOGLE and "previous_address" not in resource and "deposed" not in resource)
        need(set(resource) <= {"address", "module_address", "mode", "type", "name", "index", "provider_name", "change", "action_reason"})
        change = resource.get("change")
        need(type(change) is dict and change.get("actions") == ["no-op"] and no_unknown(change.get("after_unknown", {})))
        need(set(change) <= {"actions", "before", "after", "before_sensitive", "after_sensitive", "after_unknown", "importing", "replace_paths"})
        need(canonical(change.get("before")) == canonical(expected[address]["values"])
             and canonical(change.get("after")) == canonical(expected[address]["values"]))
        need(canonical(change.get("before_sensitive", {})) == canonical(change.get("after_sensitive", {}))
             and not change.get("replace_paths") and "generated_config" not in change)
        need(change.get("importing") == {"id": expected[address]["import_id"]} if address in imports else "importing" not in change)
    need(seen == set(expected))
    outputs = plan.get("output_changes")
    need(type(outputs) is dict and set(outputs) == set(OUTPUTS))
    for name, value in OUTPUTS.items():
        output = outputs[name]
        need(type(output) is dict and output.get("actions") == (["create"] if initialize else ["no-op"])
             and output.get("before") == (None if initialize else value) and output.get("after") == value
             and no_unknown(output.get("after_unknown", False)) and output.get("before_sensitive", False) is False
             and output.get("after_sensitive", False) is False)
    planned = plan.get("planned_values")
    need(type(planned) is dict and set(planned) <= {"outputs", "root_module"} and type(planned.get("outputs")) is dict
         and set(planned["outputs"]) == set(OUTPUTS))
    for name, value in OUTPUTS.items():
        output = planned["outputs"][name]
        need(type(output) is dict and output.get("type") == "string" and output.get("value") == value and output.get("sensitive") is False)
    found = {}
    def walk(module):
        need(type(module) is dict and set(module) <= {"address", "resources", "child_modules"})
        resources, children = module.get("resources", []), module.get("child_modules", [])
        need(type(resources) is list and type(children) is list)
        for resource in resources:
            need(type(resource) is dict and resource.get("address") in expected and resource["address"] not in found)
            found[resource["address"]] = resource
        for child in children:
            walk(child)
    walk(planned.get("root_module"))
    need(set(found) == set(expected))
    for address, resource in found.items():
        need(resource.get("mode") == "managed" and resource.get("type") == expected[address]["type"]
             and resource.get("provider_name") == GOOGLE and type(resource.get("schema_version")) is int
             and resource["schema_version"] == expected[address]["schema_version"]
             and canonical(resource.get("values")) == canonical(expected[address]["values"]))


def validate(bundle, *, git_sha, now=None):
    now = datetime.now(timezone.utc) if now is None else now
    need(type(bundle) is dict and set(bundle) == {"schema_version", "contract", "before", "after", "plan_before", "plan_after"}
         and bundle["schema_version"] == "production-private-validation-bundle-v1")
    contract = bundle["contract"]
    expected = contract_shape(contract, git_sha, now)
    adopted = {item["address"]: item for item in contract["adopted"]}
    candidate = {contract["candidate"]["address"]} if contract["candidate"] else set()
    for phase in ("before", "after"):
        evidence = bundle[phase]
        need(type(evidence) is dict and set(evidence) == {"snapshot", "metadata"} and type(evidence["metadata"]) is dict
             and set(evidence["metadata"]) == set(expected))
        for address, metadata in evidence["metadata"].items():
            need(type(metadata) is dict and set(metadata) == {"project_id", "resource_id", "payload"}
                 and metadata["project_id"] == PROJECT and metadata["resource_id"] == expected[address]["values"]["id"]
                 and type(metadata["payload"]) is dict and bool(metadata["payload"]))
    need(canonical(bundle["before"]["metadata"]) == canonical(bundle["after"]["metadata"]))
    initialize = contract["wave"] == "initialize"
    before, old_instances, old_containers = snapshot_shape(bundle["before"]["snapshot"], contract, adopted, absent=initialize)
    after, new_instances, new_containers = snapshot_shape(bundle["after"]["snapshot"], contract, expected)
    plan_shape(bundle["plan_before"], expected, imports=candidate, initialize=initialize)
    plan_shape(bundle["plan_after"], expected, imports=set())
    if initialize:
        need(not after["resources"])
    elif contract["wave"] == "noop":
        need(canonical(bundle["before"]["snapshot"]) == canonical(bundle["after"]["snapshot"]))
    else:
        need(all(after[field] == before[field] for field in ("lineage", "version", "terraform_version", "outputs"))
             and after["serial"] > before["serial"])
        need(set(old_containers) <= set(new_containers) and all(canonical(new_containers[key]) == canonical(value) for key, value in old_containers.items()))
        need(set(new_instances) - set(old_instances) == candidate and all(canonical(new_instances[address]) == canonical(instance)
             for address, instance in old_instances.items()))
        need(bundle["before"]["snapshot"]["head_before"]["VersionId"] != bundle["after"]["snapshot"]["head_before"]["VersionId"])
        # Until a candidate-specific check-results adapter is reviewed, do not
        # accept any changes to persisted assertions, even additional pass checks.
        need(canonical(before.get("check_results")) == canonical(after.get("check_results")))
    return {"schema_version": "production-offline-validation-v1", "git_sha": git_sha, "wave": contract["wave"],
            "managed_before": len(adopted), "managed_after": len(expected), "imports": len(candidate),
            "resource_change_actions": 0, "live_readiness_attested": False}


def main(argv=None):
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument("--bundle", type=Path, required=True)
        parser.add_argument("--git-sha", required=True)
        args = parser.parse_args(argv)
        private_path(args.bundle)
        descriptor = os.open(args.bundle, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        with os.fdopen(descriptor, "rb") as source:
            metadata = os.fstat(source.fileno())
            need(stat.S_ISREG(metadata.st_mode) and metadata.st_uid == os.getuid() and not metadata.st_mode & 0o077
                 and metadata.st_size <= MAX_BUNDLE_BYTES)
            raw = source.read(MAX_BUNDLE_BYTES + 1)
            need(len(raw) <= MAX_BUNDLE_BYTES)
        bundle = json.loads(raw, object_pairs_hook=unique_object, parse_constant=lambda _: need(False))
        result = validate(bundle, git_sha=args.git_sha)
        print(json.dumps(result, sort_keys=True, separators=(",", ":")))
        return 0
    except Exception:
        print("Production offline validation STOP; private diagnostic suppressed.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
