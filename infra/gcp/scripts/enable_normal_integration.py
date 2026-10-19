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
import time
from urllib.parse import unquote, quote

import bootstrap_normal_ops as config

REPO = "kani3camp/youtube-study-space"
PROJECT = "test-youtube-study-space"
BRANCH = "integration/gcp-terraform-normal-ops"
GIT_REF = "refs/heads/" + BRANCH
ISSUER = "token.actions.githubusercontent.com"
STATE_REGION = "ap-northeast-1"
OLD_REF = "refs/heads/dev"
LEGACY_AWS_REF = "refs/heads/*"  # Existing dev backend roles also serve migration workflows.
PHASE = "start"  # Fixed classification only; never embed raw CLI output.
CLAIM_KEYS = ("repository_id", "repository_owner_id", "environment", "ref",
              "workflow_ref", "job_workflow_ref", "event_name")


def custom_subject(role: str, ref: str) -> str:
    claims = {
        "repository_id": "340900071", "repository_owner_id": "54093651",
        "environment": "terraform-dev-" + role,
        "ref": ref,
        "workflow_ref": REPO + "/.github/workflows/ci.yml@" + ref,
        "job_workflow_ref": REPO + "/.github/workflows/gcp-terraform-authenticated.yml@" + ref,
        "event_name": "workflow_dispatch",
    }
    return ":".join(f"{key}:{claims[key].replace(':', '%3A')}"
                    for key in CLAIM_KEYS)

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
    conditions = matches[0].get("Condition")
    need(isinstance(conditions, dict))
    need(conditions.get("StringEquals", {}).get(ISSUER + ":aud") == "sts.amazonaws.com")
    subjects = [(family, values[ISSUER + ":sub"])
                for family in ("StringEquals", "StringLike")
                for values in [conditions.get(family, {})]
                if isinstance(values, dict) and ISSUER + ":sub" in values]
    need(len(subjects) == 1)
    family, actual = subjects[0]
    original = custom_subject(role, OLD_REF)
    extended = custom_subject(role, GIT_REF)
    # A verified pre-existing wildcard is compatible with the integration ref.
    # It is shared with migration flows and MUST NOT be silently narrowed.
    # Only the exact seven-claim subject is accepted, not a generic wildcard.
    legacy = custom_subject(role, LEGACY_AWS_REF)
    need(actual == original or actual == [original, extended] or
         (family == "StringLike" and actual == legacy))
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
        for family in ("StringEquals", "StringLike"):
            rules = conditions.get(family, {})
            need(isinstance(rules, dict))
            sub_key = ISSUER + ":sub"
            if sub_key in rules and rules[sub_key] == custom_subject(role, OLD_REF):
                # A customized GitHub OIDC 'sub' binds environment + caller +
                # reusable path + ref together, so OR exactly two complete subs.
                rules[sub_key] = [custom_subject(role, OLD_REF),
                                  custom_subject(role, GIT_REF)]
            for claim, suffix in (
                ("ref", ""), ("workflow_ref", REPO + "/.github/workflows/ci.yml@"),
                ("job_workflow_ref", REPO + "/.github/workflows/gcp-terraform-authenticated.yml@"),
            ):
                key = ISSUER + ":" + claim
                if key not in rules:
                    continue
                one, two = suffix + OLD_REF, suffix + GIT_REF
                actual = rules[key]
                need(actual == one or actual == [one, two])
                rules[key] = [one, two]
    # Preserve every other trust condition (audience, actor, owner, account,
    # OIDC principal and IAM permission policies) verbatim.
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


def verify_oidc_template() -> None:
    settings = gh(f"repos/{REPO}/actions/oidc/customization/sub")
    need(settings.get("use_default") is False and
         settings.get("include_claim_keys") == list(CLAIM_KEYS))


def verify_role_readback(role_name: str, role_id: str, desired: dict) -> None:
    # IAM is eventually consistent: compare the actual policy, not a projected
    # post-transformation document. Retry a bounded number of times.
    for attempt in range(6):
        role = aws("iam", "get-role", "--role-name", role_name)["Role"]
        need(role.get("RoleId") == role_id)
        actual = role.get("AssumeRolePolicyDocument")
        if isinstance(actual, str):
            actual = json.loads(unquote(actual))
        if actual == desired:
            return
        if attempt < 5:
            time.sleep(2)
    raise Stop("AWS trust propagation verification failed")


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
    global PHASE
    PHASE = "arguments"
    parser = argparse.ArgumentParser()
    parser.add_argument("--apply", action="store_true",
                        help="Apply exact DEV integration trust; default is read-only inspection")
    parser.add_argument("--dispatch-e2e", action="store_true",
                        help="After verified --apply, dispatch one DEV normal-e2e from the reviewed integration SHA")
    parser.add_argument("--verify-aws", action="store_true",
                        help="Optional read-only AWS role audit; requires an authenticated AWS CLI. No IAM changes.")
    flags = parser.parse_args()
    need(not flags.dispatch_e2e or flags.apply)
    if flags.verify_aws:
        PHASE = "aws_identity"
        caller = aws("sts", "get-caller-identity")
        need(isinstance(caller.get("Account"), str))
        PHASE = "aws_roles"
        aws_roles = aws("iam", "list-roles")
        need(isinstance(aws_roles.get("Roles"), list) and not aws_roles.get("IsTruncated"))
        roles = eligible_roles(aws_roles["Roles"])
        PHASE = "aws_trust_preflight"
        for kind, role in roles.items():
            need(caller["Account"] in role["Arn"])
            document = role["AssumeRolePolicyDocument"]
            if isinstance(document, str):
                document = json.loads(unquote(document))
            # Auditing only: normal setup never changes these shared backend roles.
            need(extended_trust(document, kind) == document)
        print("NORMAL_INTEGRATION: AWS_TRUST_READBACK_PASS; no mutation")
    else:
        # Verified independently through the connected Terraform State AWS account:
        # pre-existing DEV role trusts already accept the exact 7-claim
        # refs/heads/* subject. We never write AWS IAM here. If trust changes,
        # GitHub Actions will fail closed when assuming its backend role.
        print("NORMAL_INTEGRATION: AWS_CLI_NOT_REQUIRED; AWS_IAM_MUTATION=0")
    PHASE = "github_oidc_template"
    verify_oidc_template()
    PHASE = "gcp_wif_preflight"
    provider = provider_details()
    check_provider(provider)
    PHASE = "github_environments"
    gh_envs = {kind: branch_policies(kind) for kind in ("plan", "apply")}
    for _, pol in gh_envs.values():
        integration_present(pol)
    print("NORMAL_INTEGRATION: PASS; phase=preflight; target=DEV; environments=2; AWS_IAM_MUTATION=0")
    if not flags.apply:
        print("NORMAL_INTEGRATION: CHECK_ONLY; rerun with --apply to change exact DEV trust")
        return
    # Existing AWS backend roles are shared by migration and normal flows.
    # Do not alter their trust, permissions, or state merely to add an
    # integration branch. Only GCP WIF and GitHub deployment restrictions
    # are adjusted below.
    PHASE = "gcp_wif_update"
    if provider["attributeCondition"] != config.CONDITION:
        gcloud("iam", "workload-identity-pools", "providers", "update-oidc", "github-actions",
               "--workload-identity-pool=terraform-normal", "--location=global",
               "--attribute-condition=" + config.CONDITION)
    latest = provider_details()
    check_provider(latest)
    need(latest["attributeCondition"] == config.CONDITION)
    PHASE = "github_environment_update"
    for kind in ("plan", "apply"):
        endpoint, policy = gh_envs[kind]
        if not integration_present(policy):
            gh("-X", "POST", endpoint + "/deployment-branch-policies",
               "-H", "Accept: application/vnd.github+json",
               "--input", "-", data={"name": BRANCH})
        need(integration_present(branch_policies(kind)[1]))
    print("NORMAL_INTEGRATION: PASS; GCP/GitHub DEV trust readback verified; AWS not modified")
    print("NORMAL_INTEGRATION: Production, State and role resource grants untouched")
    if flags.dispatch_e2e:
        PHASE = "github_dispatch_preflight"
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
        PHASE = "github_dispatch"
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
        print("NORMAL_INTEGRATION: STOP; phase=" + PHASE +
              "; auth/role/condition mismatch; raw details suppressed")
        raise SystemExit(1)
