#!/usr/bin/env python3
"""DEV両Environmentのbranch制限を外し、境界をクラウドtrustの本人検査に一本化する。

DEVはownerの任意の非main branchの手動実行と、ownerの同一repository PRのPR Planを許可する。
Environmentの値は資格情報ではなく、GCP/AWS trustがactor・repository・初回実行を検査する。
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
ISSUER = "token.actions.githubusercontent.com"
STATE_REGION = "ap-northeast-1"
LEGACY_AWS_REF = "refs/heads/*"
PR_AWS_REF = "refs/pull/*/merge"
CLAIM_KEYS = ("repository_id", "repository_owner_id", "environment", "ref",
              "workflow_ref", "job_workflow_ref", "event_name")
# terraform.yml is the only caller (#1500); DEV trust may still list ci.yml until the old claims are removed.
# The retired ci.yml caller was removed from the DEV trust on 2026-11-01.
CALLERS = ('terraform.yml',)
PHASE = "start"


class Stop(ValueError):
    pass


def need(value: bool) -> None:
    if not value:
        raise Stop("branch authorization precondition not met")


def custom_subject(role: str, ref: str, caller: str = 'terraform.yml',
                   event_name: str = "workflow_dispatch") -> str:
    need(caller in CALLERS and event_name in ("workflow_dispatch", "pull_request"))
    # ownerのPR Planは手動PlanのEnvironment/identityを使い、terraform.ymlからだけ呼ぶ。
    need(event_name == "workflow_dispatch" or (role == "plan" and caller == "terraform.yml"))
    claims = {
        "repository_id": "340900071",
        "repository_owner_id": "54093651",
        "environment": "terraform-dev-" + role,
        "ref": ref,
        "workflow_ref": REPO + "/.github/workflows/" + caller + "@" + ref,
        "job_workflow_ref":
            REPO + "/.github/workflows/gcp-terraform-authenticated.yml@" + ref,
        "event_name": event_name,
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
    if role == "plan":
        allowed.add(custom_subject(role, PR_AWS_REF, event_name="pull_request"))
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
    # 本人起動・非mainのbranch ref・同ref/SHAのworkflow、または本人のPR PlanをWIF自身が検証する。
    # 旧ci.yml入口が併存するDEVも、他の条件は完全一致のまま読み取る。
    need(value.get('attributeCondition') == config.CONDITION)
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


def open_environment(endpoint: str, current: dict) -> None:
    """branch制限だけを外す。reviewer/timer等はdev_environmentが存在を拒否済み。"""
    need(endpoint in {f'repos/{REPO}/environments/terraform-dev-{role}' for role in ('plan', 'apply')})
    if current['deployment_branch_policy'] is not None:
        gh('-X', 'PUT', endpoint, '--input', '-', data={'deployment_branch_policy': None})


def main() -> None:
    global PHASE
    PHASE = "arguments"
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true",
                        help="DEV両Environmentのbranch制限を外します（No restriction）")
    parser.add_argument("--verify-aws", action="store_true",
                        help="任意でDEV AWS OIDC trustを読み取り監査します")
    parser.add_argument("--verify-gcp", action="store_true",
                        help="任意でDEV GCP WIFを読み取り監査します")
    flags = parser.parse_args()

    PHASE = 'github_operator'
    verify_operator()
    PHASE = 'github_oidc_template'
    verify_oidc_template()
    # 資格情報の境界はクラウド側の本人制限。Environmentはbranchで制限しない。
    if flags.verify_aws:
        verify_aws()
    if flags.verify_gcp:
        verify_gcp()
    PHASE = "github_environments"
    current = {role: dev_environment(role) for role in ("plan", "apply")}

    print("NORMAL_BRANCH_ACCESS: PASS; phase=preflight; scope=dev;"
          " desired_branch_restriction=none; cloud_mutation=0")
    if flags.apply:
        PHASE = "github_environment_update"
        for role in ("plan", "apply"):
            endpoint, previous = current[role]
            open_environment(endpoint, previous)
            _, updated = dev_environment(role)
            need(updated['deployment_branch_policy'] is None and updated['protection_rules'] == []
                 and updated['id'] == previous['id']
                 and updated.get('can_admins_bypass') == previous.get('can_admins_bypass'))
        print("NORMAL_BRANCH_ACCESS: PASS; DEV Environments have no branch restriction;"
              " AWS/GCP mutation=0")


if __name__ == "__main__":
    try:
        main()
    except (Stop, OSError, ValueError, KeyError, TypeError,
            subprocess.TimeoutExpired):
        print("NORMAL_BRANCH_ACCESS: STOP; phase=" + PHASE
              + "; raw provider output suppressed")
        raise SystemExit(1)
