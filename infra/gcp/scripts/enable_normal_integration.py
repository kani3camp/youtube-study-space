#!/usr/bin/env python3
"""One-time bounded DEV-only trust update for the normal Terraform integration.

Operator script: requires existing AWS state-account SSO, gcloud DEV login, and
GitHub environment administration. No credentials, state or tokens are stored.
It never touches Production, MyPage, Terraform State or resource permissions.
"""
from __future__ import annotations

import argparse
from copy import deepcopy
import json
import subprocess
from urllib.parse import unquote, quote

import bootstrap_normal_ops as config

REPO = "kani3camp/youtube-study-space"
PROJECT = "test-youtube-study-space"
BRANCH = "integration/gcp-terraform-normal-ops"
GIT_REF = "refs/heads/" + BRANCH
ISSUER = "token.actions.githubusercontent.com"
STATE_REGION = "ap-northeast-1"
OLD_REF = "refs/heads/dev"
OLD_CONDITION = config.CONDITION.replace(
    "assertion.ref in ['refs/heads/dev', 'refs/heads/integration/gcp-terraform-normal-ops']",
    "assertion.ref == 'refs/heads/dev'"
).replace(
    "assertion.workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/ci.yml@' + assertion.ref",
    "assertion.workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/dev'"
).replace(
    "assertion.job_workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@' + assertion.ref",
    "assertion.job_workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/dev'"
)


class Stop(ValueError):
    pass


def need(value: bool) -> None:
    if not value:
        raise Stop("integration trust precondition not met")


def cmd(*args: str, stdin: str | None = None) -> dict:
    outcome = subprocess.run(args, input=stdin, capture_output=True,
                             text=True, timeout=90, check=False)
    need(outcome.returncode == 0)
    if not outcome.stdout.strip():
        return {}
    try:
        return json.loads(outcome.stdout)
    except ValueError:
        raise Stop("invalid operator API response") from None


def aws(*args: str) -> dict:
    return cmd("aws", *args, "--region", STATE_REGION, "--output", "json")


def gcloud(*args: str) -> dict:
    return cmd("gcloud", *args, "--project=" + PROJECT, "--format=json", "--quiet")


def gh(*args: str, data: dict | None = None) -> dict:
    return cmd("gh", "api", *args, stdin=json.dumps(data) if data is not None else None)


def _singular(value):
    if isinstance(value, str):
        return [value]
    need(isinstance(value, list) and all(isinstance(x, str) for x in value))
    return value


def _aws_statements(document: dict, role: str) -> list[dict]:
    need(isinstance(document, dict))
    matches = []
    for statement in _singular_statement(document.get("Statement", [])):
        actions = _singular(statement.get("Action", []))
        principal = statement.get("Principal")
        fed = principal.get("Federated") if isinstance(principal, dict) else None
        if (statement.get("Effect") == "Allow"
                and "sts:AssumeRoleWithWebIdentity" in actions
                and isinstance(fed, str) and fed.endswith(":oidc-provider/" + ISSUER)):
            matches.append(statement)
    need(len(matches) == 1)
    sub = matches[0].get("Condition", {}).get("StringEquals", {}).get(
        ISSUER + ":sub")
    need(sub == "repo:" + REPO + ":environment:terraform-dev-" + role)
    return matches


def _singular_statement(value):
    if isinstance(value, dict):
        return [value]
    need(isinstance(value, list) and all(isinstance(x, dict) for x in value))
    return value


def eligible_roles(roles: list[dict]) -> dict[str, dict]:
    result = {}
    for kind in ("plan", "apply"):
        matches = []
        for role in roles:
            name = role.get("RoleName")
            if not isinstance(name, str) or not name.endswith("-" + kind):
                continue
            document = role.get("AssumeRolePolicyDocument")
            if isinstance(document, str):
                document = json.loads(unquote(document))
            try:
                _aws_statements(document, kind)
            except (Stop, ValueError, AttributeError):
                continue
            matches.append(role)
        need(len(matches) == 1)
        result[kind] = matches[0]
    need(result["plan"]["RoleName"] != result["apply"]["RoleName"])
    return result


def extended_trust(document: dict, role: str) -> dict:
    updated = deepcopy(document)
    for stmt in _aws_statements(updated, role):
        conditions = stmt["Condition"]
        need(isinstance(conditions, dict))
        for operation in ("StringEquals", "StringLike"):
            rule = conditions.get(operation, {})
            need(isinstance(rule, dict))
            for claim, suffix in (
                ("ref", ""), ("workflow_ref", REPO + "/.github/workflows/ci.yml@"),
                ("job_workflow_ref", REPO + "/.github/workflows/gcp-terraform-authenticated.yml@"),
            ):
                key = ISSUER + ":" + claim
                if key not in rule:
                    continue
                one, two = suffix + OLD_REF, suffix + GIT_REF
                actual = rule[key]
                need(actual == one or actual == [one, two])
                rule[key] = [one, two]
    # Preserve every other trust condition (issuer/audience, actor, repo,
    # environment, OIDC principal and action) verbatim.
    return updated


def provider_details() -> dict:
    return gcloud("iam", "workload-identity-pools", "providers", "describe",
                  "github-actions", "--workload-identity-pool=terraform-normal",
                  "--location=global")


def check_provider(value: dict) -> None:
    need(value.get("state") == "ACTIVE")
    need(value.get("oidc", {}).get("issuerUri") == "https://token.actions.githubusercontent.com")
    need(value.get("attributeMapping") == config.MAPPING)
    need(value.get("attributeCondition") in {OLD_CONDITION, config.CONDITION})


def branch_policies(role: str) -> tuple[str, dict]:
    endpoint = f"repos/{REPO}/environments/terraform-dev-{role}"
    env = gh(endpoint)
    rule = env.get("deployment_branch_policy")
    need(isinstance(rule, dict) and rule.get("custom_branch_policies") is True
         and rule.get("protected_branches") is False)
    result = gh(endpoint + "/deployment-branch-policies?per_page=100")
    need(isinstance(result.get("branch_policies"), list) and
         not result.get("total_count", 0) > 100)
    return endpoint, result


def integration_present(policy: dict) -> bool:
    policies = policy["branch_policies"]
    need(all(isinstance(x, dict) and isinstance(x.get("name"), str)
             and x.get("type") in ("branch", None) for x in policies))
    # A wildcard is not a substitute for an exact permitted integration ref.
    need(not any("*" in x["name"] or "?" in x["name"] for x in policies))
    names = [x["name"] for x in policies]
    need("dev" in names)
    return BRANCH in names


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--apply", action="store_true",
                        help="Apply exact DEV integration trust; default is read-only inspection")
    parser.add_argument("--dispatch-e2e", action="store_true",
                        help="After verified --apply, dispatch one DEV normal-e2e from the reviewed integration SHA")
    flags = parser.parse_args()
    need(not flags.dispatch_e2e or flags.apply)
    caller = aws("sts", "get-caller-identity")
    need(isinstance(caller.get("Account"), str))
    aws_roles = aws("iam", "list-roles")
    need(isinstance(aws_roles.get("Roles"), list) and not aws_roles.get("IsTruncated"))
    roles = eligible_roles(aws_roles["Roles"])
    for k, role in roles.items():
        need(caller["Account"] in role["Arn"])
        document = role["AssumeRolePolicyDocument"]
        if isinstance(document, str):
            document = json.loads(unquote(document))
        role["desired_trust"] = extended_trust(document, k)
    provider = provider_details()
    check_provider(provider)
    gh_envs = {kind: branch_policies(kind) for kind in ("plan", "apply")}
    for _, pol in gh_envs.values():
        integration_present(pol)
    print("NORMAL_INTEGRATION: PASS; phase=preflight; target=DEV; roles=2; environments=2")
    if not flags.apply:
        print("NORMAL_INTEGRATION: CHECK_ONLY; rerun with --apply to change exact DEV trust")
        return
    for kind, role in roles.items():
        document = role["desired_trust"]
        previous = role["AssumeRolePolicyDocument"]
        if isinstance(previous, str):
            previous = json.loads(unquote(previous))
        if document != previous:
            aws("iam", "update-assume-role-policy", "--role-name",
                role["RoleName"], "--policy-document", json.dumps(document, separators=(",", ":")))
        current = aws("iam", "get-role", "--role-name", role["RoleName"])["Role"]
        need(current["RoleId"] == role["RoleId"] and
             extended_trust(current["AssumeRolePolicyDocument"], kind) == document)
    if provider["attributeCondition"] != config.CONDITION:
        gcloud("iam", "workload-identity-pools", "providers", "update-oidc", "github-actions",
               "--workload-identity-pool=terraform-normal", "--location=global",
               "--attribute-condition=" + config.CONDITION)
    latest = provider_details()
    check_provider(latest)
    need(latest["attributeCondition"] == config.CONDITION)
    for kind in ("plan", "apply"):
        endpoint, policy = gh_envs[kind]
        if not integration_present(policy):
            gh("-X", "POST", endpoint + "/deployment-branch-policies",
               "-H", "Accept: application/vnd.github+json",
               "--input", "-", data={"name": BRANCH})
        need(integration_present(branch_policies(kind)[1]))
    print("NORMAL_INTEGRATION: PASS; AWS/GCP/GitHub DEV-only trust readback verified")
    print("NORMAL_INTEGRATION: Production, State and role resource grants untouched")
    if flags.dispatch_e2e:
        operator = gh("user")
        need(operator.get("login") == "kani3camp" and operator.get("id") == 54093651)
        branch = gh(f"repos/{REPO}/branches/{quote(BRANCH, safe='')}")
        sha = branch.get("commit", {}).get("sha")
        need(isinstance(sha, str) and len(sha) == 40
             and all(ch in "0123456789abcdef" for ch in sha))
        status = gh(f"repos/{REPO}/commits/{sha}/check-runs?per_page=100")
        checks = status.get("check_runs", [])
        need(isinstance(checks, list) and len(checks) >= 2)
        for required in ("CI Gate", "GCP Terraform Validate"):
            selected = [c for c in checks if c.get("name") == required]
            need(len(selected) == 1 and selected[0].get("conclusion") == "success")
        command = subprocess.run([
            "gh", "workflow", "run", "ci.yml", "--repo", REPO,
            "--ref", BRANCH,
            "-f", "terraform_authenticated=true",
            "-f", "terraform_target=dev",
            "-f", "terraform_mode=normal-e2e",
            "-f", "terraform_expected_sha=" + sha,
        ], capture_output=True, text=True, timeout=60, check=False)
        need(command.returncode == 0)
        print("NORMAL_INTEGRATION: PASS; DEV normal-e2e dispatched once; sha=" + sha)


if __name__ == "__main__":
    try:
        main()
    except (Stop, OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired):
        print("NORMAL_INTEGRATION: STOP; auth/role/condition mismatch; raw details suppressed")
        raise SystemExit(1)
