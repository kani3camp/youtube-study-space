#!/usr/bin/env python3
"""Prepare private, default-off ownership inputs from reviewed REST inventory.

No cloud clients, credentials, IAM writes, API changes or Terraform execution.
The caller must obtain fresh complete metadata and independently review scope.
"""
from __future__ import annotations

import json
import os
import re
import stat
import sys
from pathlib import Path

PROJECT = "test-youtube-study-space"
ROLE = "roles/iam.workloadIdentityUser"
CLASSES = {"Own", "Platform/External", "Investigate", "Do not own"}


def require(value: object) -> None:
    if not value:
        raise ValueError("inventory contract rejected")


def text(value: object, *, empty: bool = False) -> str:
    require(type(value) is str and (empty or bool(value.strip())))
    return value


def require_empty_attestation_response(response: object) -> None:
    # IAM v1 ListAttestationRulesResponse has only these two optional fields.
    # A successful {} response may omit empty repeated/default fields; an error
    # envelope, unknown field or malformed value must never mean "no rules".
    require(type(response) is dict and set(response) <= {"attestationRules", "nextPageToken"})
    rules = response.get("attestationRules", [])
    token = response.get("nextPageToken", "")
    require(type(rules) is list and len(rules) == 0)
    require(type(token) is str and token == "")


def prepare(data: object) -> dict:
    require(type(data) is dict)
    project = data["project"]
    require(project["projectId"] == PROJECT)
    number = text(project["projectNumber"])
    require(re.fullmatch(r"[0-9]+", number))
    pool_name = f"projects/{number}/locations/global/workloadIdentityPools/aws-runtime"
    provider_name = f"{pool_name}/providers/aws-provider"
    pool, provider = data["pool"], data["provider"]
    require(pool["name"] == pool_name and provider["name"] == provider_name)
    for item in (pool, provider):
        require(item["state"] == "ACTIVE" and item.get("disabled", False) is False)
        require("expireTime" not in item)
    mode = pool.get("mode")
    require(mode in (None, "", "FEDERATION_ONLY"))
    require_empty_attestation_response(data["pool_attestation_rules"])
    require(not any(key in pool for key in ("inlineCertificateIssuanceConfig", "inlineTrustConfig")))
    require(type(provider["aws"]) is dict and set(provider["aws"]) == {"accountId"})
    account = text(provider["aws"]["accountId"])
    require(re.fullmatch(r"[0-9]{12}", account))
    require(not any(key in provider for key in ("oidc", "saml", "x509")))
    mapping = provider["attributeMapping"]
    require(type(mapping) is dict and all(type(k) is str and type(v) is str for k, v in mapping.items()))
    for key in ("google.subject", "attribute.aws_role", "attribute.account"):
        text(mapping[key])
    # Preserve the live CEL verbatim; this tool never derives/rewrites trust.
    condition = text(provider["attributeCondition"])
    email = f"{PROJECT}@appspot.gserviceaccount.com"
    sa_name = f"projects/{PROJECT}/serviceAccounts/{email}"
    require(data["iam_policy_resource"] == sa_name)
    require(data["runtime_config"] == {"audience": f"//iam.googleapis.com/{provider_name}", "service_account_email": email})
    policy = data["iam_policy"]
    require(type(policy.get("version", 1)) is int and policy.get("version", 1) in (1, 3) and type(policy["bindings"]) is list)
    selected = data["runtime_grants"]
    require(type(selected) is dict)
    grants, identities = {}, set()
    prefix = f"principalSet://iam.googleapis.com/{pool_name}/attribute.aws_role/"
    for alias, selection in selected.items():
        require(re.fullmatch(r"grant-[0-9]{2}", alias))
        member = text(selection["member"])
        require(member.startswith(prefix) and re.fullmatch(r"[A-Za-z0-9_+=,.@-]+", member[len(prefix):]))
        title = selection.get("condition_title", "")
        text(title, empty=True)
        require(title == " ".join(title.split()))
        identity = (member, title)
        require(identity not in identities)
        identities.add(identity)
        # Matches the pinned IAM importer: role/member/condition title must be
        # unique. Same-title different-expression bindings cannot be imported.
        matches = [b for b in policy["bindings"] if b.get("role") == ROLE and member in b.get("members", []) and b.get("condition", {}).get("title", "") == title]
        require(len(matches) == 1)
        binding = matches[0]
        actual_condition = binding.get("condition")
        if actual_condition is not None:
            require(policy["version"] == 3 and set(actual_condition) <= {"title", "description", "expression"})
            actual_condition = {
                "title": text(actual_condition["title"]),
                "description": text(actual_condition.get("description", ""), empty=True),
                "expression": text(actual_condition["expression"]),
            }
        grants[alias] = {"member": member, "condition": actual_condition}
    enabled = data["enabled_services"]
    require(not enabled.get("nextPageToken") and type(enabled["services"]) is list)
    service_states = {}
    for service in enabled["services"]:
        name = text(service["config"]["name"])
        require(service["name"] == f"projects/{number}/services/{name}")
        require(name not in service_states and service["state"] == "ENABLED")
        service_states[name] = service["state"]
    classification = {}
    for name, item in data["api_classification"].items():
        require(re.fullmatch(r"[a-z][a-z0-9-]*\.googleapis\.com", name))
        require(item["classification"] in CLASSES)
        dependencies = item["dependency_addresses"]
        require(type(dependencies) is list and all(type(x) is str and bool(x.strip()) for x in dependencies))
        require(len(dependencies) == len(set(dependencies)))
        reason = text(item["reason"])
        if item["classification"] == "Own":
            require(name in service_states and bool(dependencies))
        classification[name] = {**item, "state": service_states.get(name, "NOT_ENABLED"), "reason": reason}
    require(set(service_states) <= set(classification))
    return {
        "own_runtime_wif_pool": False,
        "own_runtime_wif_provider": False,
        "runtime_wif_grant_keys": [],
        "runtime_wif_inventory": {
            "project_number": number,
            "pool": {"display_name": text(pool.get("displayName", ""), empty=True), "description": text(pool.get("description", ""), empty=True), "disabled": False, "mode": mode},
            "provider": {"display_name": text(provider.get("displayName", ""), empty=True), "description": text(provider.get("description", ""), empty=True), "disabled": False, "aws_account_id": account, "attribute_mapping": mapping.copy(), "attribute_condition": condition},
            "service_account_id": sa_name,
            "grants": grants,
        },
        "owned_api_keys": [],
        "api_classification": classification,
    }


def unique_object(pairs: list[tuple[str, object]]) -> dict:
    result = {}
    for key, value in pairs:
        require(key not in result)
        result[key] = value
    return result


def main() -> int:
    try:
        require(len(sys.argv) == 3)
        repository = Path(__file__).resolve().parents[3]
        require(not Path(sys.argv[1]).resolve().is_relative_to(repository))
        require(not Path(sys.argv[2]).resolve().is_relative_to(repository))
        fd = os.open(sys.argv[1], os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(fd, "rb") as handle:
            info = os.fstat(handle.fileno())
            require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid() and info.st_mode & 0o077 == 0)
            raw = handle.read(4 * 1024 * 1024 + 1)
        require(len(raw) <= 4 * 1024 * 1024)
        candidate = prepare(json.loads(raw.decode(), object_pairs_hook=unique_object))
        # An auto-loaded var file inside a checkout risks publishing private IAM
        # metadata. Require private staging outside this repository.
        fd = os.open(sys.argv[2], os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(candidate, handle, indent=2)
            handle.write("\n")
    except (ValueError, OSError, TypeError, KeyError, AttributeError, RecursionError):
        print("STOP: private ownership preparation rejected; no cloud operation performed.", file=sys.stderr)
        return 1
    print("Offline preparation passed. All ownership gates remain disabled; live review and approval are pending.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
