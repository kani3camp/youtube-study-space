#!/usr/bin/env python3
"""One-time PRODUCTION normal GCP identity setup; never mutates AWS or State.

Requires an operator's short-lived gcloud/gh/AWS credentials. Fail closed
unless the pre-existing production backend trust is already main + actor_id.
Existing migration WIF, GCP SA, State, workloads and AWS roles are untouched.
"""
from __future__ import annotations

import json
import re
import sys

import bootstrap_normal_ops as dev

PROJECT = "youtube-study-space"
REPO = dev.REPO
POOL = "terraform-normal"
ISSUER = "token.actions.githubusercontent.com"
CLAIMS = ("repository_id", "repository_owner_id", "environment", "ref",
          "workflow_ref", "job_workflow_ref", "event_name")
MAIN = "refs/heads/main"
BASELINE = {"resourcemanager.projects.get",
            "iam.workloadIdentityPools.get",
            "iam.workloadIdentityPoolProviders.get"}
MAPPING = dev.MAPPING
CONDITION = " && ".join([
    "assertion.repository_id == '340900071'",
    "assertion.repository_owner_id == '54093651'",
    "assertion.ref == 'refs/heads/main'",
    "assertion.event_name == 'workflow_dispatch'",
    "assertion.workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/main'",
    "assertion.job_workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/main'",
    "assertion.environment in ['terraform-prod-plan', 'terraform-prod-apply']",
    "assertion.actor_id == '54093651'",
    "assertion.actor == 'kani3camp'",
    "assertion.run_attempt == '1'",
    "assertion.workflow_sha == assertion.sha",
    "assertion.job_workflow_sha == assertion.sha",
])


def need(value):
    dev.need(value)


def gcloud(*args, missing=False):
    scope = [] if args[:3] == ("iam", "roles", "describe") and args[3].startswith("roles/") else ["--project=" + PROJECT]
    return dev.run(["gcloud", *args, *scope, "--format=json", "--quiet"], missing=missing)


def aws(*args):
    return dev.run(["aws", *args, "--region", "ap-northeast-1", "--output", "json"])


def gh(*args):
    return dev.run(["gh", "api", *args])


def subject(role):
    ref = MAIN
    values = {
        "repository_id": "340900071", "repository_owner_id": "54093651",
        "environment": "terraform-prod-" + role, "ref": ref,
        "workflow_ref": REPO + "/.github/workflows/ci.yml@" + ref,
        "job_workflow_ref": REPO + "/.github/workflows/gcp-terraform-authenticated.yml@" + ref,
        "event_name": "workflow_dispatch",
    }
    return ":".join(f"{key}:{values[key].replace(':', '%3A')}" for key in CLAIMS)


def verify_role(document, account, role):
    if isinstance(document, str):
        document = json.loads(document)
    need(isinstance(document, dict))
    statements = document.get("Statement", [])
    if isinstance(statements, dict):
        statements = [statements]
    need(type(statements) is list and all(type(x) is dict for x in statements))
    allows = [s for s in statements if s.get("Effect") == "Allow"]
    need(len(allows) == 1)
    statement = allows[0]
    need("NotAction" not in statement and "NotPrincipal" not in statement)
    need(statement.get("Action") in ("sts:AssumeRoleWithWebIdentity", ["sts:AssumeRoleWithWebIdentity"]))
    principal = statement.get("Principal")
    need(type(principal) is dict and set(principal) == {"Federated"})
    expected_arn = f"arn:aws:iam::{account}:oidc-provider/{ISSUER}"
    need(principal["Federated"] in (expected_arn, [expected_arn]))
    conditions = statement.get("Condition")
    need(type(conditions) is dict and set(conditions) == {"StringEquals", "StringLike"})
    need(conditions["StringEquals"] == {
        ISSUER + ":aud": "sts.amazonaws.com",
        ISSUER + ":actor_id": "54093651",
    })
    need(conditions["StringLike"] == {ISSUER + ":sub": subject(role)})


def preflight_cloud():
    identity = aws("sts", "get-caller-identity")
    account = identity.get("Account")
    need(type(account) is str and re.fullmatch("[0-9]{12}", account) is not None)
    for role in ("plan", "apply"):
        value = aws("iam", "get-role", "--role-name", f"youtube-study-space-terraform-prod-{role}")["Role"]
        need(value.get("Arn") == f"arn:aws:iam::{account}:role/youtube-study-space-terraform-prod-{role}")
        verify_role(value.get("AssumeRolePolicyDocument"), account, role)
    return account


def preflight_github():
    template = gh(f"repos/{REPO}/actions/oidc/customization/sub")
    need(template.get("use_default") is False
         and template.get("include_claim_keys") == list(CLAIMS))
    actor = gh("user")
    need(actor.get("login") == "kani3camp" and actor.get("id") == 54093651)
    for role in ("plan", "apply"):
        env = f"terraform-prod-{role}"
        base = f"repos/{REPO}/environments/{env}"
        settings = gh(base)
        policy = settings.get("deployment_branch_policy", {})
        need(type(policy) is dict and policy.get("custom_branch_policies") is True
             and policy.get("protected_branches") is False)
        response = gh(base + "/deployment-branch-policies?per_page=100")
        rules = response.get("branch_policies")
        need(type(rules) is list and type(response.get("total_count")) is int
             and response["total_count"] <= 100
             and all(item.get("type") in ("branch", None) for item in rules))
        names = [item.get("name") for item in rules]
        need(names == ["main"])  # Main is the sole PROD Environment branch.
        secrets = gh(base + "/secrets")
        need({"GCP_TERRAFORM_MANAGED_INPUTS_JSON",
              "AWS_TERRAFORM_BACKEND_ROLE_ARN", "AWS_TERRAFORM_BACKEND_ROLE_ID",
              "AWS_TERRAFORM_STATE_BUCKET", "AWS_TERRAFORM_STATE_ACCOUNT_ID"}
             <= {s.get("name") for s in secrets.get("secrets", [])})


def production_permissions(catalog):
    """Allow general infrastructure changes but never delegate resource IAM."""
    roles = dev.selected_permissions(catalog, BASELINE)
    roles["apply"] = [p for p in roles["apply"] if not p.endswith(".setIamPolicy")]
    need(not any(p.endswith(".setIamPolicy") for p in roles["apply"]))
    return roles


def permissions():
    project = gcloud("projects", "describe", PROJECT)
    need(project.get("projectId") == PROJECT and project.get("lifecycleState") == "ACTIVE")
    number = project.get("projectNumber")
    need(type(number) in (str, int) and re.fullmatch(r"[0-9]+", str(number)) is not None)
    catalog = set()
    for role in dev.CATALOGS:
        catalog.update(gcloud("iam", "roles", "describe", role)["includedPermissions"])
    allowed = production_permissions(catalog)
    return str(number), allowed


def provider_details():
    return gcloud("iam", "workload-identity-pools", "providers", "describe",
                  "github-actions", "--workload-identity-pool=" + POOL, "--location=global")


def verify_provider(details):
    need(details.get("state") == "ACTIVE"
         and details.get("oidc", {}).get("issuerUri") == "https://" + ISSUER
         and details.get("attributeMapping") == MAPPING
         and details.get("attributeCondition") == CONDITION)


def check_policy(policy, approved):
    """Only a single narrow principal may be bound at the target SA scope.

    Never ignore conditioned/extra grants when checking for privilege
    escalation. Empty policies are acceptable before initial provisioning.
    """
    need(type(policy) is dict and type(policy.get("bindings", [])) is list)
    actual = set()
    for binding in policy.get("bindings", []):
        need(type(binding) is dict and "condition" not in binding
             and set(binding) == {"role", "members"})
        role, members = binding["role"], binding["members"]
        need(type(role) is str and type(members) is list and len(members) == 1
             and type(members[0]) is str)
        actual.add((role, members[0]))
    need(actual <= approved)
    return actual


def check_project_grants(policy, member, expected_role):
    """Reject even conditional extra project IAM grants on a normal SA."""
    need(type(policy) is dict and type(policy.get("bindings", [])) is list)
    allowed = []
    for binding in policy["bindings"]:
        need(type(binding) is dict and type(binding.get("members", [])) is list)
        if member in binding["members"]:
            need("condition" not in binding and binding.get("role") == expected_role)
            allowed.append(binding)
    need(len(allowed) <= 1)
    return bool(allowed)


def bootstrap(number, allowed):
    # Validate *all existing* target resources before writing anything. A
    # bootstrap rerun must not overwrite an unexpected production role or WIF.
    roles = {}
    for kind, included in allowed.items():
        name = "terraformNormal" + kind.title()
        current = gcloud("iam", "roles", "describe", name, missing=True)
        if current is not None:
            need(sorted(current.get("includedPermissions", [])) == included
                 and current.get("stage") == "GA")
        roles[kind] = (name, current)
    runtime = "terraform-prod-workload@" + PROJECT + ".iam.gserviceaccount.com"
    runtime_exists = gcloud("iam", "service-accounts", "describe", runtime, missing=True)
    pool = gcloud("iam", "workload-identity-pools", "describe", POOL,
                  "--location=global", missing=True)
    if pool is not None:
        need(pool.get("state") == "ACTIVE")
    current_provider = gcloud("iam", "workload-identity-pools", "providers", "describe",
                             "github-actions", "--workload-identity-pool=" + POOL,
                             "--location=global", missing=True)
    if current_provider is not None:
        verify_provider(current_provider)
    project_policy = gcloud("projects", "get-iam-policy", PROJECT)
    need(type(project_policy) is dict and type(project_policy.get("bindings", [])) is list
         and not any("serviceAccount:" + runtime in b.get("members", [])
                     for b in project_policy["bindings"]))
    runtime_grant = ("roles/iam.serviceAccountUser",
                     "serviceAccount:terraform-prod-normal-apply@" + PROJECT + ".iam.gserviceaccount.com")
    if runtime_exists is not None:
        check_policy(gcloud("iam", "service-accounts", "get-iam-policy", runtime), {runtime_grant})
    accounts = {}
    for kind in ("plan", "apply"):
        email = "terraform-prod-normal-" + kind + "@" + PROJECT + ".iam.gserviceaccount.com"
        exists = gcloud("iam", "service-accounts", "describe", email, missing=True)
        principal = (f"principalSet://iam.googleapis.com/projects/{number}/locations/global/"
                     f"workloadIdentityPools/{POOL}/attribute.environment/terraform-prod-{kind}")
        approved_grant = ("roles/iam.workloadIdentityUser", principal)
        project_role = f"projects/{PROJECT}/roles/terraformNormal" + kind.title()
        check_project_grants(project_policy, "serviceAccount:" + email, project_role)
        if exists is not None:
            need(exists.get("email") == email)
            keys = gcloud("iam", "service-accounts", "keys", "list", "--iam-account=" + email)
            need(all(key.get("keyType") != "USER_MANAGED" for key in keys))
            check_policy(gcloud("iam", "service-accounts", "get-iam-policy", email), {approved_grant})
        accounts[kind] = (email, exists, principal, approved_grant, project_role)

    # Only absent resources are created; matching existing resources stay intact.
    for kind, included in allowed.items():
        name, existing = roles[kind]
        if existing is None:
            gcloud("iam", "roles", "create", name,
                   "--permissions=" + ",".join(included), "--stage=GA",
                   "--title=Terraform PROD normal " + kind)
        updated = gcloud("iam", "roles", "describe", name)
        need(sorted(updated.get("includedPermissions", [])) == included
             and updated.get("stage") == "GA")

    if runtime_exists is None:
        gcloud("iam", "service-accounts", "create", "terraform-prod-workload",
               "--display-name=Terraform PROD workload runtime")
    if pool is None:
        gcloud("iam", "workload-identity-pools", "create", POOL,
               "--location=global", "--display-name=Terraform PROD normal")
    if current_provider is None:
        gcloud("iam", "workload-identity-pools", "providers",
               "create-oidc", "github-actions",
               "--workload-identity-pool=" + POOL, "--location=global",
               "--issuer-uri=https://" + ISSUER,
               "--attribute-mapping=" + ",".join(k + "=" + v for k, v in MAPPING.items()),
               "--attribute-condition=" + CONDITION)
    provider = provider_details()
    verify_provider(provider)

    for kind in ("plan", "apply"):
        account, exists, principal, approved_grant, project_role = accounts[kind]
        if exists is None:
            gcloud("iam", "service-accounts", "create", "terraform-prod-normal-" + kind,
                   "--display-name=Terraform PROD normal " + kind)
        need(gcloud("iam", "service-accounts", "describe", account)["email"] == account)
        keys = gcloud("iam", "service-accounts", "keys", "list", "--iam-account=" + account)
        need(all(key.get("keyType") != "USER_MANAGED" for key in keys))
        trust = gcloud("iam", "service-accounts", "get-iam-policy", account)
        if approved_grant not in check_policy(trust, {approved_grant}):
            gcloud("iam", "service-accounts", "add-iam-policy-binding", account,
                   "--member=" + principal, "--role=roles/iam.workloadIdentityUser")
        need(check_policy(gcloud("iam", "service-accounts", "get-iam-policy", account),
                          {approved_grant}) == {approved_grant})
        member = "serviceAccount:" + account
        project_policy = gcloud("projects", "get-iam-policy", PROJECT)
        if not check_project_grants(project_policy, member, project_role):
            gcloud("projects", "add-iam-policy-binding", PROJECT,
                   "--member=" + member, "--role=" + project_role, "--condition=None")
        need(check_project_grants(gcloud("projects", "get-iam-policy", PROJECT),
                                  member, project_role))
        if kind == "apply":
            runtime_policy = gcloud("iam", "service-accounts", "get-iam-policy", runtime)
            if runtime_grant not in check_policy(runtime_policy, {runtime_grant}):
                gcloud("iam", "service-accounts", "add-iam-policy-binding", runtime,
                       "--member=" + member, "--role=roles/iam.serviceAccountUser")
            need(check_policy(gcloud("iam", "service-accounts", "get-iam-policy", runtime),
                              {runtime_grant}) == {runtime_grant})
        env = "terraform-prod-" + kind
        dev.run(["gh", "secret", "set", "GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT",
                 "--repo", REPO, "--env", env], data=account)
        dev.run(["gh", "secret", "set", "GCP_TERRAFORM_NORMAL_WIF_PROVIDER",
                 "--repo", REPO, "--env", env], data=provider["name"])
        secrets = gh(f"repos/{REPO}/environments/{env}/secrets")
        need({"GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT", "GCP_TERRAFORM_NORMAL_WIF_PROVIDER"}
             <= {s.get("name") for s in secrets.get("secrets", [])})
        print("PROD_NORMAL_BOOTSTRAP: PASS; role=" + kind + "; provider_verified=true")


def main():
    need(sys.argv[1:] in (["--check"], ["--apply"]))
    preflight_cloud()
    preflight_github()
    number, roles = permissions()
    if sys.argv[1] == "--check":
        print(json.dumps({"project": PROJECT, "normal_roles": roles,
                          "ref": MAIN, "environment": ["terraform-prod-plan", "terraform-prod-apply"],
                          "state_mutations": 0, "cloud_mutations": 0}, sort_keys=True))
        return
    bootstrap(number, roles)
    print("PROD_NORMAL_BOOTSTRAP: PASS; migration_and_state_untouched=true; production_apply_not_run")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("PROD_NORMAL_BOOTSTRAP: STOP; raw provider output suppressed", file=sys.stderr)
        raise SystemExit(1)
