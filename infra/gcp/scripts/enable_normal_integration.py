#!/usr/bin/env python3
"""DEVの本人起動を維持し、信頼する実行元をEnvironmentへ明示追加する。

GitHubの本人認証で既存DEV両EnvironmentのSelected branchesを管理する。
AWS/GCP trustの監査は読み取り専用の任意操作で、本番は扱わない。
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
DEFAULT_BRANCH = "dev"
ISSUER = "token.actions.githubusercontent.com"
STATE_REGION = "ap-northeast-1"
LEGACY_AWS_REF = "refs/heads/*"
CLAIM_KEYS = ("repository_id", "repository_owner_id", "environment", "ref",
              "workflow_ref", "job_workflow_ref", "event_name")
CALLERS = ('ci.yml', 'terraform.yml')
BRANCH = re.compile(r"[A-Za-z0-9][A-Za-z0-9._/-]*")
PHASE = "start"


class Stop(ValueError):
    pass


def need(value: bool) -> None:
    if not value:
        raise Stop("branch authorization precondition not met")


def branch_name(value: str) -> str:
    need(type(value) is str and BRANCH.fullmatch(value) is not None
         and value != "main" and not value.startswith("-")
         and not value.startswith("refs/"))
    return value


def custom_subject(role: str, ref: str, caller: str = 'ci.yml') -> str:
    need(caller in CALLERS)
    claims = {
        "repository_id": "340900071",
        "repository_owner_id": "54093651",
        "environment": "terraform-dev-" + role,
        "ref": ref,
        "workflow_ref": REPO + "/.github/workflows/" + caller + "@" + ref,
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
    """Audit every WebIdentity Allow; owner claim is enforced by AWS itself."""
    candidates = []
    for statement in _statements(document):
        if statement.get("Effect") != "Allow":
            continue
        # NotAction / wildcard grants can authorize WebIdentity without naming
        # it. These are not part of this role's reviewed trust contract.
        need("NotAction" not in statement and "NotPrincipal" not in statement)
        actions = _actions(statement.get("Action", []))
        need(actions == ["sts:AssumeRoleWithWebIdentity"])
        principal = statement.get("Principal")
        need(isinstance(principal, dict) and set(principal) == {"Federated"})
        federated = principal["Federated"]
        federated = [federated] if isinstance(federated, str) else federated
        need(isinstance(federated, list) and len(federated) == 1
             and isinstance(federated[0], str)
             and re.fullmatch(r"arn:aws:iam::[0-9]{12}:oidc-provider/" + re.escape(ISSUER),
                              federated[0]) is not None)
        candidates.append(statement)
    need(len(candidates) == 1)
    condition = candidates[0].get("Condition")
    need(isinstance(condition, dict))
    equals = condition.get("StringEquals", {})
    likes = condition.get("StringLike", {})
    need(isinstance(equals, dict) and isinstance(likes, dict)
         and equals.get(ISSUER + ":aud") == "sts.amazonaws.com"
         and equals.get(ISSUER + ":actor_id") == "54093651")
    # 任意ブランチを許可するが、本人のimmutable IDと他のsubject claimsは固定する。
    subjects = likes.get(ISSUER + ':sub')
    subjects = [subjects] if isinstance(subjects, str) else subjects
    allowed = {custom_subject(role, LEGACY_AWS_REF, caller) for caller in CALLERS}
    need(isinstance(subjects, list) and all(isinstance(s, str) for s in subjects)
         and len(subjects) == len(set(subjects)) and set(subjects) <= allowed
         and custom_subject(role, LEGACY_AWS_REF) in subjects)


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
    # 本人起動・非mainのbranch ref・同ref/SHAのworkflowをWIF自身が検証する。
    # 既存のTerraform専用入口が併存するDEVも、他の条件は完全一致のまま読み取る。
    caller = "assertion.workflow_ref == '" + REPO + "/.github/workflows/ci.yml@' + assertion.ref"
    callers = 'assertion.workflow_ref in [' + ', '.join(
        "'" + REPO + '/.github/workflows/' + name + "@' + assertion.ref" for name in CALLERS) + ']'
    need(value.get('attributeCondition') in (config.CONDITION, config.CONDITION.replace(callers, caller)))
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


def verify_operator() -> None:
    operator = gh('user')
    need(operator.get('login') == 'kani3camp' and operator.get('id') == 54093651)


def dev_environment(role: str) -> tuple[str, dict]:
    need(role in ('plan', 'apply'))
    endpoint = f"repos/{REPO}/environments/terraform-dev-{role}"
    environment = gh(endpoint)
    need(environment.get('name') == f'terraform-dev-{role}'
         and type(environment.get('id')) is int
         and 'deployment_branch_policy' in environment)
    policy = environment.get("deployment_branch_policy")
    need(policy is None or (isinstance(policy, dict)
         and policy.get('custom_branch_policies') is True
         and policy.get('protected_branches') is False))
    # PUTが他の保護を暗黙に変更しないよう、未知のreviewer/timer等があれば停止する。
    rules = environment.get('protection_rules')
    need(isinstance(rules, list)
         and all(isinstance(rule, dict) and rule.get('type') == 'branch_policy' for rule in rules))
    return endpoint, environment


def authorize_control_branch(endpoint: str, current: dict, target: str) -> None:
    need(endpoint in {f'repos/{REPO}/environments/terraform-dev-{role}' for role in ('plan', 'apply')})
    branch_name(target)
    policy = {'protected_branches': False, 'custom_branch_policies': True}
    if current['deployment_branch_policy'] is None:
        # No restriction時は一覧APIが404になる。先に制限を復旧し、アクセスを閉じる。
        gh('-X', 'PUT', endpoint, '--input', '-', data={'deployment_branch_policy': policy})
    path = endpoint + '/deployment-branch-policies'
    values = gh(path + '?per_page=100')
    need(type(values.get('total_count')) is int and values['total_count'] <= 100
         and isinstance(values.get('branch_policies'), list))
    rules = values['branch_policies']
    need(all(type(r.get('id')) is int and type(r.get('name')) is str for r in rules))
    need(all(rule.get('type', 'branch') == 'branch' for rule in rules))
    previous = {branch_name(rule['name']) for rule in rules}
    if target not in previous:
        gh('-X', 'POST', path, '--input', '-', data={'name': target, 'type': 'branch'})
    latest = gh(path + '?per_page=100')
    latest_rules = latest['branch_policies']
    need(latest['total_count'] == len(previous | {target}) == len(latest_rules)
         and all(rule.get('type', 'branch') == 'branch' for rule in latest_rules)
         and {branch_name(rule['name']) for rule in latest_rules} == previous | {target})


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
        "-f", "terraform_source_branch=" + target,
    ], capture_output=True, text=True, timeout=60, check=False)
    need(result.returncode == 0)
    print("NORMAL_BRANCH_ACCESS: PASS; normal-e2e dispatched; sha=" + sha)


def main() -> None:
    global PHASE
    PHASE = "arguments"
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--branch", default=DEFAULT_BRANCH,
                        help="Environmentへ明示追加する信頼済みの実行元ブランチ")
    parser.add_argument("--apply", action="store_true",
                        help="DEV両Environmentの既存許可を残して--branchを明示追加します")
    parser.add_argument("--dispatch-e2e", action="store_true",
                        help="--applyの検証後にnormal-e2eを1回起動します")
    parser.add_argument("--verify-aws", action="store_true",
                        help="任意でDEV AWS OIDC trustを読み取り監査します")
    parser.add_argument("--verify-gcp", action="store_true",
                        help="任意でDEV GCP WIFを読み取り監査します")
    flags = parser.parse_args()
    target = branch_name(flags.branch)
    need(not flags.dispatch_e2e or flags.apply)

    PHASE = 'github_operator'
    verify_operator()
    PHASE = 'github_oidc_template'
    verify_oidc_template()
    # Secretのref制限と、クラウド側の本人制限を別々の境界として維持する。
    if flags.verify_aws:
        verify_aws()
    if flags.verify_gcp:
        verify_gcp()
    PHASE = "github_environments"
    current = {role: dev_environment(role) for role in ("plan", "apply")}

    print("NORMAL_BRANCH_ACCESS: PASS; phase=preflight; scope=dev;"
          " desired_selected_branches_only=true; cloud_mutation=0")
    if flags.apply:
        PHASE = "github_environment_update"
        for role in ("plan", "apply"):
            endpoint, previous = current[role]
            authorize_control_branch(endpoint, previous, target)
            _, updated = dev_environment(role)
            need(updated['deployment_branch_policy'] == {'protected_branches': False, 'custom_branch_policies': True}
                 and updated['id'] == previous['id']
                 and updated.get('can_admins_bypass') == previous.get('can_admins_bypass'))
        print("NORMAL_BRANCH_ACCESS: PASS; trusted DEV control branch authorized;"
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
