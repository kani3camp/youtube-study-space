#!/usr/bin/env python3
"""Fail-closed contract for one MyPage OAuth TTL field; never authorize apply."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import stat
import sys

ADDRESS = "module.oauth_transaction_ttl.google_firestore_field.oauth_transaction_ttl"
PROVIDER = "registry.terraform.io/hashicorp/google"
TERRAFORM_VERSION = "1.16.4"
VALUE_KEYS = {
    "project", "database", "collection", "field", "deletion_policy", "skip_wait",
    "index_config", "ttl_config", "id", "name", "timeouts",
}


class ContractError(Exception):
    """Only fixed error codes may cross the CLI output boundary."""


def require(condition, code):
    if not condition:
        raise ContractError(code)


def target_scope(target):
    require(isinstance(target, dict) and set(target) == {
        "schemaVersion", "environment", "projectID", "databaseID",
    }, "target_invalid")
    require(type(target["schemaVersion"]) is int and target["schemaVersion"] == 1,
            "target_invalid")
    require(target["environment"] in ("development", "production"), "target_invalid")
    require(isinstance(target["projectID"], str) and
            re.fullmatch(r"[a-z][a-z0-9-]{4,28}[a-z0-9]", target["projectID"]),
            "target_invalid")
    database = target["databaseID"]
    require(isinstance(database, str) and (database == "(default)" or
            re.fullmatch(r"[a-z][a-z0-9-]{2,61}[a-z0-9]", database)), "target_invalid")
    return {key: target[key] for key in ("environment", "projectID", "databaseID")} | {
        "collection": "oauth-transactions", "field": "expiresAt",
    }


def true_paths(value, path=()):
    if value is True:
        yield path
    elif value is False:
        return
    elif isinstance(value, dict):
        for key, item in value.items():
            yield from true_paths(item, path + (key,))
    elif isinstance(value, list):
        for index, item in enumerate(value):
            yield from true_paths(item, path + (index,))
    else:
        raise ContractError("marker_invalid")


def check_values(value, scope, *, desired):
    require(isinstance(value, dict) and set(value) <= VALUE_KEYS, "field_shape_invalid")
    expected = {"project": scope["projectID"], "database": scope["databaseID"],
                "collection": scope["collection"], "field": scope["field"]}
    require(all(value.get(key) == item for key, item in expected.items()), "target_mismatch")
    require(value.get("skip_wait") is False and value.get("timeouts") is None,
            "field_controls_invalid")
    require(value.get("index_config") == [], "index_override_forbidden")
    require(value.get("deletion_policy") in (("PREVENT",) if desired else ("PREVENT", "DELETE")),
            "deletion_protection_invalid")
    field_name = (f"projects/{scope['projectID']}/databases/{scope['databaseID']}"
                  "/collectionGroups/oauth-transactions/fields/expiresAt")
    require(all(value.get(key) in (None, field_name) for key in ("id", "name")),
            "field_identity_invalid")
    ttl = value.get("ttl_config")
    require(isinstance(ttl, list) and len(ttl) in ((1,) if desired else (0, 1)),
            "ttl_disabled")
    if ttl:
        require(isinstance(ttl[0], dict) and set(ttl[0]) <= {"expiration_offset", "state"},
                "ttl_shape_invalid")
        require(ttl[0].get("expiration_offset") in (None, ""), "ttl_offset_forbidden")
        require(ttl[0].get("state") in (None, "ACTIVE", "CREATING"), "ttl_state_invalid")


def check_change(resource, scope):
    """Shared by full saved-plan validation and real Terraform mock-plan tests."""
    require(isinstance(resource, dict), "resource_shape_invalid")
    expected = {"address": ADDRESS, "module_address": "module.oauth_transaction_ttl",
                "mode": "managed", "type": "google_firestore_field",
                "name": "oauth_transaction_ttl", "provider_name": PROVIDER}
    require(all(resource.get(key) == item for key, item in expected.items()), "resource_scope_invalid")
    require(not any(key in resource for key in ("previous_address", "deposed", "index")),
            "resource_movement_forbidden")
    change = resource.get("change")
    require(isinstance(change, dict), "change_invalid")
    actions = change.get("actions")
    require(actions in (["create"], ["update"], ["no-op"]), "action_forbidden")
    require(not change.get("importing") and not change.get("replace_paths") and
            not resource.get("generated_config"), "import_or_replace_forbidden")
    require(not list(true_paths(change.get("before_sensitive", False))) and
            not list(true_paths(change.get("after_sensitive", False))), "sensitive_field_forbidden")
    before, after = change.get("before"), change.get("after")
    check_values(after, scope, desired=True)
    if actions == ["create"]:
        require(before is None, "create_before_invalid")
    else:
        check_values(before, scope, desired=False)
        require(before.get("index_config") == after.get("index_config"), "index_change_forbidden")
        for key in VALUE_KEYS - {"ttl_config", "deletion_policy"}:
            require(before.get(key) == after.get(key), "non_ttl_update_forbidden")
        if actions == ["no-op"]:
            require(before == after, "noop_changed")
        else:
            require(before.get("ttl_config") == [], "existing_ttl_update_forbidden")
    allowed_unknown = set()
    if actions == ["create"]:
        allowed_unknown |= {("id",), ("name",)}
    if actions != ["no-op"]:
        allowed_unknown.add(("ttl_config", 0, "state"))
        # Optional+Computed in Google 8.5.0; configuration must omit the offset.
        allowed_unknown.add(("ttl_config", 0, "expiration_offset"))
    require(set(true_paths(change.get("after_unknown", {}))) <= allowed_unknown,
            "unknown_field_forbidden")
    return actions[0]


def value_resources(module):
    require(isinstance(module, dict), "planned_values_invalid")
    resources = module.get("resources", [])
    require(isinstance(resources, list), "planned_values_invalid")
    result = list(resources)
    children = module.get("child_modules", [])
    require(isinstance(children, list), "planned_values_invalid")
    for child in children:
        result.extend(value_resources(child))
    return result


def config_resources(module):
    require(isinstance(module, dict), "configuration_invalid")
    resources = module.get("resources", [])
    require(isinstance(resources, list), "configuration_invalid")
    result = list(resources)
    calls = module.get("module_calls", {})
    require(isinstance(calls, dict), "configuration_invalid")
    for call in calls.values():
        require(isinstance(call, dict), "configuration_invalid")
        result.extend(config_resources(call.get("module")))
    return result


def validate_plan(plan, target):
    scope = target_scope(target)
    require(isinstance(plan, dict), "plan_invalid")
    require(isinstance(plan.get("format_version"), str) and
            re.fullmatch(r"1\.\d+", plan["format_version"]), "format_unsupported")
    require(plan.get("terraform_version") == TERRAFORM_VERSION, "terraform_version_mismatch")
    require(plan.get("complete") is True and plan.get("errored") is False, "plan_incomplete")
    require(not plan.get("resource_drift") and not plan.get("deferred_changes"), "drift_or_deferred")
    checks = plan.get("checks", [])
    require(isinstance(checks, list) and all(isinstance(c, dict) and c.get("status") == "pass"
            for c in checks), "plan_check_failed")
    require(plan.get("variables") == {
        "project_id": {"value": scope["projectID"]},
        "database_id": {"value": scope["databaseID"]},
    }, "input_binding_mismatch")
    changes = plan.get("resource_changes")
    require(isinstance(changes, list) and len(changes) == 1, "resource_count_invalid")
    action = check_change(changes[0], scope)
    require(type(plan.get("applyable")) is bool and
            (action == "no-op" or plan["applyable"]), "plan_action_mismatch")
    outputs = plan.get("output_changes")
    require(isinstance(outputs, dict) and set(outputs) == {"scope"}, "output_scope_invalid")
    output = outputs["scope"]
    require(isinstance(output, dict) and output.get("after") == scope and
            not list(true_paths(output.get("after_unknown", False))) and
            not list(true_paths(output.get("after_sensitive", False))), "environment_binding_mismatch")
    planned = plan.get("planned_values")
    require(isinstance(planned, dict), "planned_values_invalid")
    resources = value_resources(planned.get("root_module"))
    require(len(resources) == 1 and resources[0].get("address") == ADDRESS and
            resources[0].get("mode") == "managed" and resources[0].get("provider_name") == PROVIDER and
            resources[0].get("values") == changes[0]["change"]["after"] and
            not list(true_paths(resources[0].get("sensitive_values", False))), "planned_values_mismatch")
    require(set(planned.get("outputs", {})) == {"scope"} and
            planned["outputs"]["scope"].get("value") == scope and
            planned["outputs"]["scope"].get("sensitive") is False, "planned_output_mismatch")
    configuration = plan.get("configuration", {})
    require(isinstance(configuration, dict), "configuration_invalid")
    configured = config_resources(configuration.get("root_module"))
    require(len(configured) == 1 and configured[0].get("type") == "google_firestore_field" and
            configured[0].get("mode") == "managed", "configuration_scope_invalid")
    expressions = configured[0].get("expressions", {})
    require(isinstance(expressions, dict) and set(expressions) == {
        "project", "database", "collection", "field", "deletion_policy", "skip_wait", "ttl_config",
    } and expressions["ttl_config"] == [{}], "ttl_configuration_invalid")
    return action


def duplicate_safe(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "json_duplicate_key")
        result[key] = value
    return result


def read_json(path, limit):
    try:
        descriptor = os.open(path, os.O_RDONLY | os.O_NONBLOCK)
        with os.fdopen(descriptor, "rb") as source:
            require(stat.S_ISREG(os.fstat(source.fileno()).st_mode), "input_not_regular")
            data = source.read(limit + 1)
        require(len(data) <= limit, "input_too_large")
        def reject_constant(_value):
            raise ContractError("json_invalid")
        value = json.loads(data.decode("utf-8"), object_pairs_hook=duplicate_safe,
                           parse_constant=reject_constant)
        return value, hashlib.sha256(data).hexdigest()
    except ContractError:
        raise
    except (OSError, ValueError, UnicodeError, RecursionError):
        raise ContractError("input_invalid") from None


class SafeParser(argparse.ArgumentParser):
    def error(self, _message):
        raise ContractError("arguments_invalid")


def main(argv=None):
    result = {"schemaVersion": 1, "status": "rejected", "executionAuthorized": False,
              "releaseReady": False}
    try:
        parser = SafeParser(description=__doc__)
        parser.add_argument("--plan", required=True)
        parser.add_argument("--target", required=True)
        args = parser.parse_args(argv)
        plan, plan_hash = read_json(args.plan, 16 * 1024 * 1024)
        target, target_hash = read_json(args.target, 64 * 1024)
        action = validate_plan(plan, target)
        result.update(status="accepted", code="oauth_ttl_scope_valid", action=action,
                      planSHA256=plan_hash, targetSHA256=target_hash)
    except ContractError as error:
        result["code"] = str(error)
    except (KeyError, TypeError, ValueError, AttributeError, RecursionError):
        result["code"] = "plan_shape_invalid"
    print(json.dumps(result, sort_keys=True))
    return 0 if result["status"] == "accepted" else 1


if __name__ == "__main__":
    sys.exit(main())
