#!/usr/bin/env python3
"""Manage DEV Terraform branch authorization through GitHub Environments only.

Per-branch authorization lives in the two GitHub DEV Terraform Environments.
AWS/GCP trust are long-lived, branch-name-agnostic control planes and are never
mutated by this script. Optional AWS/GCP flags are read-only audits.
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
from urllib.parse import quote, unquote

import bootstrap_normal_ops as config

REPO = "kani3camp/youtube-study-space"
PROJECT = "test-youtube-study-space"
DEFAULT_BRANCH = "integration/gcp-terraform-normal-ops"
ISSUER = "token.actions.githubusercontent.com"
STATE_REGION = "ap-northeast-1"
LEGACY_AWS_REF = "refs/heads/*"
CLAIM_KEYS = ("repository_id", "repository_owner_id", "environment", "ref",
              "workflow_ref", "job_workflow_ref", "event_name")
BRANCH = re.compile(r"[A-Za-z0-9][A-Za-z0-9._/-]*")
PHASE = "start"


class Stop(ValueError):
    pass


def need(value: bool) -> None:
    if not value:
        raise Stop("branch authorization precondition not met")


def branch_name(value: str) -> str:
    need(type(value) is str and BRANCH.fullmatch(value) is not None
         and value != "main" and not value.startswith("-"))
    return value


def custom_subject(role: str, ref: str) -> str:
    claims = {
        "repository_id": "340900071",
        "repository_owner_id": "54093651",
        "environment": "terraform-dev-" + role,
        "ref": ref,
        "workflow_ref": REPO + "/.github/workflows/ci.yml@" + ref,
        "job_workflow_ref":
            REPO + "/.github/workflows/gcp-terraform-authenticated.yml@" + ref,
        "event_name": "workflow_dispatch",
    }
    return ":".join(f"{key}:{claims[key].replace(':', '%3A')}"
                    for key in CLAIM_KEYS)


def cmd(*args: str, stdin: str | None = None) -> dict:
    result = subprocess.run(args, input=stdin, capture_output=True, text=True,
                            timeout=90, check=False)
    need(result.returncode == 0)
    if not result.stdout.strip():
        return {}
    try:
        return json.loads(result.stdout)
    except ValueError:
        raise Stop("invalid operator API response") from None


def aws(*args: str) -> dict:
    return cmd("aws", *args, "--region", STATE_REGION, "--output", "json")


def gcloud(*args: str) -> dict:
    return cmd("gcloud", *args, "--project=" + PROJECT, "--format=json", "--quiet")


def gh(*args: str, data: dict | None = None) -> dict:
    return cmd("gh", "api", *args,
               stdin=json.dumps(data) if data is not None else None)


def _statements(document):
    values = document.get("Statement", [])
    if isinstance(values, dict):
        values = [values]
    need(isinstance(values, list) and all(isinstance(x, dict) for x in values))
    return values


def _actions(value):
    if isinstance(value, str):
        return [value]
    need(isinstance(value, list) and all(isinstance(x, str) for x in value))
    return value


def audit_aws_role(document: dict, role: str) -> None:
    """Require the existing branch-agnostic seven-claim subject; never mutate it."""
    candidates = []
    for statement in _statements(document):
        principal = statement.get("Principal")
        federated = principal.get("Federated") if isinstance(principal, dict) else None
        if (statement.get("Effect") == "Allow"
                and "sts:AssumeRoleWithWebIdentity" in _actions(statement.get("Action", []))
                and isinstance(federated, str)
                and federated.endswith(":oidc-provider/" + ISSUER)):
            candidates.append(statement)
    need(len(candidates) == 1)
    condition = candidates[0].get("Condition")
    need(isinstance(condition, dict))
    equals = condition.get("StringEquals", {})
    likes = condition.get("StringLike", {})
    need(isinstance(equals, dict) and isinstance(likes, dict)
         and equals.get(ISSUER + ":aud") == "sts.amazonaws.com")
    # This wildcard is not the authorization surface. GitHub Environment is.
    # Every other claim remains fixed inside the custom subject.
    need(likes.get(ISSUER + ":sub") == custom_subject(role, LEGACY_AWS_REF))


def verify_aws() -> None:
    global PHASE
    PHASE = "aws_identity"
    identity = aws("sts", "get-caller-identity")
    need(isinstance(identity.get("Account"), str))
    PHASE = "aws_roles"
    for role in ("plan", "apply"):
        name = "youtube-study-space-terraform-dev-" + role
        value = aws("iam", "get-role", "--role-name", name)["Role"]
        need(value.get("Arn", "").split(":")[4] == identity["Account"])
        document = value.get("AssumeRolePolicyDocument")
        if isinstance(document, str):
            document = json.loads(unquote(document))
        audit_aws_role(document, role)
    print("NORMAL_BRANCH_ACCESS: AWS_READBACK_PASS; mutation=0")


def provider_details() -> dict:
    return gcloud("iam", "workload-identity-pools", "providers", "describe",
                  "github-actions", "--workload-identity-pool=terraform-normal",
                  "--location=global")


def check_provider(value: dict) -> None:
    need(value.get("state") == "ACTIVE")
    need(value.get("oidc", {}).get("issuerUri")
         == "https://token.actions.githubusercontent.com")
    need(value.get("attributeMapping") == config.MAPPING)
    # No branch names are allowed in the durable WIF condition. It verifies
    # branch-shaped refs and same-ref workflow paths; Environment selects names.
    need(value.get("attributeCondition") == config.CONDITION)
    condition = value["attributeCondition"]
    need("refs/heads/dev" not in condition
         and "integration/gcp-terraform-normal-ops" not in condition)


def verify_gcp() -> None:
    global PHASE
    PHASE = "gcp_wif_readback"
    check_provider(provider_details())
    print("NORMAL_BRANCH_ACCESS: GCP_WIF_READBACK_PASS; mutation=0")


def verify_oidc_template() -> None:
    settings = gh(f"repos/{REPO}/actions/oidc/customization/sub")
    need(settings.get("use_default") is False
         and settings.get("include_claim_keys") == list(CLAIM_KEYS))


def branch_policies(role: str) -> tuple[str, dict]:
    endpoint = f"repos/{REPO}/environments/terraform-dev-{role}"
    environment = gh(endpoint)
    policy = environment.get("deployment_branch_policy")
    need(isinstance(policy, dict)
         and policy.get("custom_branch_policies") is True
         and policy.get("protected_branches") is False)
    values = gh(endpoint + "/deployment-branch-policies?per_page=100")
    need(isinstance(values.get("branch_policies"), list)
         and type(values.get("total_count")) is int
         and values["total_count"] <= 100)
    return endpoint, values


def exact_policy_names(values: dict) -> list[str]:
    policies = values["branch_policies"]
    need(all(isinstance(item, dict) and isinstance(item.get("name"), str)
             and item.get("type") in ("branch", None) for item in policies))
    names = [item["name"] for item in policies]
    # Keep this operational lane exact. Broad patterns bypass the explicit
    # per-branch workflow the user chose for DEV.
    need(not any("*" in name or "?" in name for name in names))
    return names


def branch_present(values: dict, target: str) -> bool:
    names = exact_policy_names(values)
    need("dev" in names)
    return target in names


def add_branch(endpoint: str, target: str) -> None:
    gh("-X", "POST", endpoint + "/deployment-branch-policies",
       "-H", "Accept: application/vnd.github+json",
       "--input", "-", data={"name": target})


def dispatch_e2e(target: str) -> None:
    global PHASE
    PHASE = "github_dispatch_preflight"
    operator = gh("user")
    need(operator.get("login") == "kani3camp" and operator.get("id") == 54093651)
    remote = gh(f"repos/{REPO}/branches/{quote(target, safe='')}")
    sha = remote.get("commit", {}).get("sha")
    need(isinstance(sha, str) and re.fullmatch(r"[0-9a-f]{40}", sha) is not None)
    status = gh(f"repos/{REPO}/commits/{sha}/check-runs?per_page=100")
    checks = status.get("check_runs", [])
    need(isinstance(checks, list))
    for required in ("CI Gate", "GCP Terraform Validate"):
        selected = [item for item in checks if item.get("name") == required]
        need(len(selected) == 1 and selected[0].get("conclusion") == "success")
    PHASE = "github_dispatch"
    result = subprocess.run([
        "gh", "workflow", "run", "ci.yml", "--repo", REPO, "--ref", target,
        "-f", "terraform_authenticated=true",
        "-f", "terraform_target=dev",
        "-f", "terraform_mode=normal-e2e",
        "-f", "terraform_expected_sha=" + sha,
    ], capture_output=True, text=True, timeout=60, check=False)
    need(result.returncode == 0)
    print("NORMAL_BRANCH_ACCESS: PASS; normal-e2e dispatched; sha=" + sha)


def main() -> None:
    global PHASE
    PHASE = "arguments"
    parser = argparse.ArgumentParser()
    parser.add_argument("--branch", default=DEFAULT_BRANCH,
                        help="Exact DEV branch to authorize in GitHub Environments")
    parser.add_argument("--apply", action="store_true",
                        help="Add exact branch to DEV plan/apply Environments")
    parser.add_argument("--dispatch-e2e", action="store_true",
                        help="After verified --apply, dispatch one normal-e2e")
    parser.add_argument("--verify-aws", action="store_true",
                        help="Optional read-only AWS OIDC trust audit")
    parser.add_argument("--verify-gcp", action="store_true",
                        help="Optional read-only GCP WIF audit")
    flags = parser.parse_args()
    target = branch_name(flags.branch)
    need(not flags.dispatch_e2e or flags.apply)

    if flags.verify_aws:
        verify_aws()
    if flags.verify_gcp:
        verify_gcp()

    PHASE = "github_oidc_template"
    verify_oidc_template()
    PHASE = "github_environments"
    current = {role: branch_policies(role) for role in ("plan", "apply")}
    for _, values in current.values():
        exact_policy_names(values)

    print("NORMAL_BRANCH_ACCESS: PASS; phase=preflight; branch=" + target
          + "; cloud_mutation=0")
    if flags.apply:
        PHASE = "github_environment_update"
        for role in ("plan", "apply"):
            endpoint, values = current[role]
            if not branch_present(values, target):
                add_branch(endpoint, target)
            need(branch_present(branch_policies(role)[1], target))
        print("NORMAL_BRANCH_ACCESS: PASS; GitHub Environment branch enabled;"
              " AWS/GCP mutation=0")
    if flags.dispatch_e2e:
        dispatch_e2e(target)


if __name__ == "__main__":
    try:
        main()
    except (Stop, OSError, ValueError, KeyError, TypeError,
            subprocess.TimeoutExpired):
        print("NORMAL_BRANCH_ACCESS: STOP; phase=" + PHASE
              + "; raw provider output suppressed")
        raise SystemExit(1)
