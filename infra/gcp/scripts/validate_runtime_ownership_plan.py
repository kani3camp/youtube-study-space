#!/usr/bin/env python3
"""Fail-closed cumulative history12 + reviewed runtime/API plan contract.

No plan, import, cloud read, approval or state mutation is performed here.
"""
from __future__ import annotations

import re

from prepare_runtime_ownership import PROJECT, ROLE, require
from runtime_ownership_packet import validate_packet
from validate_user_activity_history_plan import validate as validate_history
from terraform_email_adoption_gate import canonical, has_unknown
from terraform_plan_summary import build_summary

GOOGLE = "registry.terraform.io/hashicorp/google"
POOL = "module.runtime_wif.google_iam_workload_identity_pool.runtime[0]"
PROVIDER = "module.runtime_wif.google_iam_workload_identity_pool_provider.runtime[0]"


def identities(candidate):
    inventory = candidate["runtime_wif_inventory"]
    pool_id = f"projects/{PROJECT}/locations/global/workloadIdentityPools/aws-runtime"
    pool_name = pool_id.replace(PROJECT, inventory["project_number"], 1)
    result = {}
    if candidate["own_runtime_wif_pool"]:
        result[POOL] = ("google_iam_workload_identity_pool", pool_id, {
            "id": pool_id, "name": pool_name, "project": PROJECT,
            "workload_identity_pool_id": "aws-runtime", "state": "ACTIVE",
            "display_name": inventory["pool"]["display_name"], "description": inventory["pool"]["description"],
            "disabled": False, "mode": inventory["pool"]["mode"], "deletion_policy": "DELETE",
            "attestation_rules": [], "inline_certificate_issuance_config": [], "inline_trust_config": []})
    if candidate["own_runtime_wif_provider"]:
        result[PROVIDER] = ("google_iam_workload_identity_pool_provider", pool_id + "/providers/aws-provider", {
            "id": pool_id + "/providers/aws-provider", "name": pool_name + "/providers/aws-provider", "project": PROJECT,
            "workload_identity_pool_id": "aws-runtime", "workload_identity_pool_provider_id": "aws-provider",
            "state": "ACTIVE", "display_name": inventory["provider"]["display_name"],
            "description": inventory["provider"]["description"], "disabled": False,
            "attribute_mapping": inventory["provider"]["attribute_mapping"],
            "attribute_condition": inventory["provider"]["attribute_condition"],
            "aws": [{"account_id": inventory["provider"]["aws_account_id"]}],
            "oidc": [], "saml": [], "x509": [], "deletion_policy": "DELETE"})
    for alias in candidate["runtime_wif_grant_keys"]:
        grant = inventory["grants"][alias]
        condition = grant["condition"]
        sa = inventory["service_account_id"]
        import_id = " ".join([sa, ROLE, grant["member"]] + ([condition["title"]] if condition else []))
        # Pinned tpgiamresource importer ID is slash-delimited, including full
        # title/description/expression for conditional grants; never hash trust.
        state_id = sa + "/" + ROLE + "/" + grant["member"]
        if condition:
            state_id += "/" + "/".join(condition[key] for key in ("title", "description", "expression"))
        result[f'module.runtime_wif.google_service_account_iam_member.runtime["{alias}"]'] = (
            "google_service_account_iam_member", import_id, {
                "id": state_id, "service_account_id": sa, "role": ROLE, "member": grant["member"],
                "condition": [condition] if condition else []})
    for service in candidate["owned_api_keys"]:
        resource_id = PROJECT + "/" + service
        result[f'module.owned_apis.google_project_service.owned["{service}"]'] = (
            "google_project_service", resource_id, {"id": resource_id, "project": PROJECT, "service": service,
                "disable_on_destroy": False, "disable_dependent_services": False, "deletion_policy": "DELETE"})
    return result


def validate(plan, *, packet, wave, git_sha, metadata, phase, execution_email, now=None):
    require(phase in {"before", "post"} and type(plan) is dict)
    candidate = validate_packet(packet, wave=wave, git_sha=git_sha, now=now)
    expected = identities(candidate)
    adopted_candidate = dict(candidate, own_runtime_wif_pool=packet["adopted"]["pool"],
                             own_runtime_wif_provider=packet["adopted"]["provider"],
                             runtime_wif_grant_keys=packet["adopted"]["grants"], owned_api_keys=packet["adopted"]["apis"])
    imports = set(expected) - set(identities(adopted_candidate)) if phase == "before" else set()
    require(plan.get("format_version") == "1.2" and plan.get("terraform_version") == "1.16.4")
    require(plan.get("complete") is True and plan.get("errored", False) is False)
    for key in ("resource_drift", "deferred_changes", "action_invocations", "deferred_action_invocations"):
        require(type(plan.get(key, [])) is list and plan.get(key, []) == [])
    checks = plan.get("checks", [])
    require(type(checks) is list and all(type(c) is dict and c.get("status") == "pass" and
            type(c.get("instances", [])) is list and all(type(i) is dict and i.get("status") == "pass"
            for i in c.get("instances", [])) for c in checks))
    changes = plan.get("resource_changes")
    require(type(changes) is list and all(type(r) is dict for r in changes))
    addresses = [r.get("address") for r in changes]
    require(all(type(a) is str for a in addresses) and len(addresses) == len(set(addresses)))
    require(all("previous_address" not in r and "deposed" not in r for r in changes))
    for r in changes:
        change = r.get("change")
        require(type(change) is dict and type(change.get("before")) is dict and type(change.get("after")) is dict)
        require(change.get("actions") == ["no-op"] and not has_unknown(change.get("after_unknown", {})))
        require(canonical(change["before"]) == canonical(change["after"]))
        require(canonical(change.get("before_sensitive", {})) == canonical(change.get("after_sensitive", {})))
        require(not change.get("replace_paths") and "generated_config" not in change)
        address_type = re.search(r"(google_[a-z_]+)\.", r["address"])
        require(address_type is not None and address_type.group(1) == r.get("type"))
        require(change.get("importing") == {"id": expected[r["address"]][1]} if r["address"] in imports else "importing" not in change)
    outputs = plan.get("output_changes", {})
    require(type(outputs) is dict)
    for output in outputs.values():
        require(type(output) is dict and output.get("actions") == ["no-op"])
        require(not has_unknown(output.get("after_unknown", {})))
        require(canonical(output.get("before")) == canonical(output.get("after")))
        require(canonical(output.get("before_sensitive", False)) == canonical(output.get("after_sensitive", False)))
    runtime = [r for r in changes if r["address"] in expected]
    baseline = [r for r in changes if r["address"] not in expected]
    require(len(runtime) == len(expected))
    # This preserves all history schema/order/description and eleven-resource
    # identities/exclusions, requiring history already adopted (import0).
    validate_history(dict(plan, resource_changes=baseline), metadata=metadata,
                     phase="post", execution_email=execution_email)
    require(all(r.get("provider_name") == GOOGLE for r in changes))
    for resource in runtime:
        resource_type, _, values = expected[resource["address"]]
        require(resource.get("mode") == "managed" and resource.get("type") == resource_type)
        actual = resource["change"]["after"]
        require(set(actual) <= set(values) | {"timeouts", "etag"})
        for key, value in values.items():
            # Unspecified remote mode remains provider-computed, but only known
            # federation-only values are accepted; no input value is invented.
            if resource["address"] == POOL and key == "mode" and value in (None, ""):
                require(actual.get(key) in (None, "", "FEDERATION_ONLY"))
            else:
                require(key in actual and type(actual[key]) is type(value) and actual[key] == value)
        require(actual.get("timeouts") in (None, {}))
        if resource_type == "google_service_account_iam_member":
            require(type(actual.get("etag")) is str and bool(actual["etag"]))
        else:
            require("etag" not in actual)
    # Confirm resource schema version and planned values, independently of the
    # change projection. Google 8.5.0 uses schema version 0 for these resources.
    planned = {}
    def walk(module):
        require(type(module) is dict)
        resources, children = module.get("resources", []), module.get("child_modules", [])
        require(type(resources) is list and type(children) is list)
        for r in resources:
            require(type(r) is dict and type(r.get("address")) is str and r["address"] not in planned)
            planned[r["address"]] = r
        for child in children:
            walk(child)
    require(type(plan.get("planned_values")) is dict)
    walk(plan["planned_values"].get("root_module"))
    require(set(planned) == set(addresses))
    for r in changes:
        p = planned[r["address"]]
        require(p.get("mode") == "managed" and p.get("type") == r.get("type") and p.get("provider_name") == GOOGLE)
        require(canonical(p.get("values")) == canonical(r["change"]["after"]))
        require(type(p.get("schema_version")) is int and
                p["schema_version"] == (4 if r.get("type") == "google_storage_bucket" else 0))
    graph = set(addresses)
    for service in candidate["owned_api_keys"]:
        require(set(candidate["api_classification"][service]["dependency_addresses"]) <= graph)
    summary = build_summary(plan, environment="dev", git_sha=git_sha, policy="import-only")
    require(summary["policy_passed"] and summary["counts"] == {
        key: len(imports) if key == "import" else len(addresses) if key == "no-op" else 0
        for key in summary["counts"]})
    return summary
