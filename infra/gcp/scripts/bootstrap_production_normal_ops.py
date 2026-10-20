#!/usr/bin/env python3
"""One-time PRODUCTION normal GCP identity setup; never mutates AWS or State.

Requires an operator's short-lived gcloud/gh/AWS credentials. Fail closed
unless the pre-existing production backend trust is already main + actor_id.
Existing migration WIF, GCP SA, State, workloads and AWS roles are untouched.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import sys
import subprocess
import time

import bootstrap_normal_ops as dev
import production_backend_packet as backend

PROJECT = "youtube-study-space"
REPO = dev.REPO
POOL = "terraform-normal"
ISSUER = "token.actions.githubusercontent.com"
CLAIMS = ("repository_id", "repository_owner_id", "environment", "ref",
          "workflow_ref", "job_workflow_ref", "event_name")
MAIN = "refs/heads/main"
BASELINE_ROLES = ("terraformCiRead", "terraformProdRuntimePoolReadV1",
                  "terraformProdRuntimeSaPolicyReadV1")
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
    expected = {ISSUER + ":aud": "sts.amazonaws.com", ISSUER + ":actor_id": "54093651"}
    need(type(conditions) is dict)
    # AWS supports GitHub's native actor_id condition. Preserve the existing
    # seven-claim subject template; customising it would affect every workflow.
    need(conditions in (
        {"StringEquals": expected | {ISSUER + ":sub": subject(role)}},
        {"StringEquals": expected, "StringLike": {ISSUER + ":sub": subject(role)}},
    ))


def load_backend(path):
    value = backend.private_file(path)
    need(type(value) is dict and set(value) == {
        "state_account_id", "state_bucket", "production_plan_role_name", "production_apply_role_name"})
    need(re.fullmatch(r"[0-9]{12}", value["state_account_id"]) is not None)
    bucket = value["state_bucket"]
    need(re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", bucket) is not None
         and ".." not in bucket and ".-" not in bucket and "-." not in bucket
         and re.fullmatch(r"[0-9]+(?:\.[0-9]+){3}", bucket) is None)
    names = [value[f"production_{kind}_role_name"] for kind in ("plan", "apply")]
    need(len(set(names)) == 2 and all(re.fullmatch(r"[A-Za-z0-9+=,.@_-]{1,64}", n) for n in names))
    # Match the normal controller's identity() role-purpose contract.
    need(all(name.endswith("-" + kind) for name, kind in zip(names, ("plan", "apply"))))
    return value


def policy_projection(document):
    """Ignore policy presentation, never actions/resources/conditions."""
    need(type(document) is dict and set(document) <= {"Version", "Id", "Statement"}
         and document.get("Version") == "2012-10-17")
    statements = document.get("Statement")
    if type(statements) is dict:
        statements = [statements]
    need(type(statements) is list)
    projected = []
    for statement in statements:
        need(type(statement) is dict)
        item = {key: value for key, value in statement.items() if key != "Sid"}
        for key in ("Action", "Resource"):
            if type(item.get(key)) is str:
                item[key] = [item[key]]
            need(type(item.get(key)) is list and all(type(x) is str for x in item[key]))
            item[key] = sorted(item[key])
        if "Condition" in item:
            condition = item["Condition"]
            need(type(condition) is dict)
            item["Condition"] = {}
            for operator, entries in condition.items():
                need(type(entries) is dict)
                item["Condition"][operator] = {}
                for key, values in entries.items():
                    values = [values] if type(values) is str else values
                    need(type(values) is list and all(type(v) is str for v in values))
                    item["Condition"][operator][key] = sorted(values)
        projected.append(item)
    return sorted(projected, key=lambda x: json.dumps(x, sort_keys=True))


def verify_backend_permissions(inline, attached, bucket, kind):
    # An attached managed policy is acceptable only if it is the sole exact
    # backend policy. Never silently ignore either source of permission grants.
    documents = inline + attached
    need(len(documents) == 1)
    need(policy_projection(documents[0]) == policy_projection(backend.permissions_policy(bucket, kind)))


def preflight_cloud(config):
    account = config["state_account_id"]
    need(aws("sts", "get-caller-identity").get("Account") == account)
    for kind in ("plan", "apply"):
        name = config[f"production_{kind}_role_name"]
        value = aws("iam", "get-role", "--role-name", name)["Role"]
        need(value.get("Arn") == f"arn:aws:iam::{account}:role/{name}")
        verify_role(value.get("AssumeRolePolicyDocument"), account, kind)
        inline, attached = [], []
        for policy in aws("iam", "list-role-policies", "--role-name", name)["PolicyNames"]:
            inline.append(aws("iam", "get-role-policy", "--role-name", name,
                              "--policy-name", policy)["PolicyDocument"])
        for policy in aws("iam", "list-attached-role-policies", "--role-name", name)["AttachedPolicies"]:
            arn = policy["PolicyArn"]
            version = aws("iam", "get-policy", "--policy-arn", arn)["Policy"]["DefaultVersionId"]
            attached.append(aws("iam", "get-policy-version", "--policy-arn", arn,
                                "--version-id", version)["PolicyVersion"]["Document"])
        verify_backend_permissions(inline, attached, config["state_bucket"], kind)
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


def production_permissions(catalog, baseline):
    """Allow general infrastructure changes but never delegate resource IAM."""
    roles = dev.selected_permissions(catalog, baseline)
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
    baseline = set()
    for role in BASELINE_ROLES:
        baseline.update(gcloud("iam", "roles", "describe", role)["includedPermissions"])
    allowed = production_permissions(catalog, baseline)
    return str(number), allowed


def provider_details():
    return gcloud("iam", "workload-identity-pools", "providers", "describe",
                  "github-actions", "--workload-identity-pool=" + POOL, "--location=global")


def verify_provider(details):
    need(details.get("state") == "ACTIVE"
         and details.get("disabled", False) is False
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


def verify_account(account, email):
    need(type(account) is dict and account.get("email") == email
         and account.get("disabled", False) is False)


def verify_no_user_keys(email):
    keys = gcloud("iam", "service-accounts", "keys", "list", "--iam-account=" + email)
    need(type(keys) is list and all(key.get("keyType") != "USER_MANAGED" for key in keys))


def wait_for_account(email):
    # Newly created IAM accounts can briefly return NOT_FOUND. Retry only
    # absence; permission failures and identity/disabled drift stop immediately.
    for attempt in range(12):
        account = gcloud("iam", "service-accounts", "describe", email, missing=True)
        if account is not None:
            verify_account(account, email)
            return
        need(attempt < 11)
        time.sleep(5)


def verify_pool_providers(number, expected):
    # A principalSet grants trust across the entire pool, not one provider.
    # Include soft-deleted providers so an unreviewed identity cannot be restored.
    providers = gcloud("iam", "workload-identity-pools", "providers", "list",
                       "--workload-identity-pool=" + POOL, "--location=global",
                       "--show-deleted")
    name = (f"projects/{number}/locations/global/workloadIdentityPools/{POOL}"
            "/providers/github-actions")
    need(type(providers) is list and len(providers) == (1 if expected is not None else 0))
    if expected is not None:
        need(providers[0].get("name") == name)
        verify_provider(providers[0])


def policy_analysis_scope(number):
    ancestors = gcloud("projects", "get-ancestors", PROJECT)
    need(type(ancestors) is list and ancestors
         and ancestors[0].get("type") == "project"
         and str(ancestors[0].get("id")) in (PROJECT, str(number)))
    for ancestor in ancestors[1:]:
        need(ancestor.get("type") in ("folder", "organization")
             and re.fullmatch(r"[0-9]+", str(ancestor.get("id"))) is not None)
    root = ancestors[-1]
    return root["type"], str(root["id"])


def verify_effective_grants(response, member, scope, approved, required):
    """Accept only fully explored, unconditioned grants at exact resources."""
    need(type(response) is dict and response.get("fullyExplored") is True)
    main = response.get("mainAnalysis", {})
    need(main.get("fullyExplored") is True and not main.get("nonCriticalErrors"))
    query = main.get("analysisQuery", {})
    need(query.get("scope") == scope
         and query.get("identitySelector") == {"identity": member}
         and query.get("options", {}).get("expandGroups") is True
         and not query.get("resourceSelector") and not query.get("accessSelector"))
    results = main.get("analysisResults", [])
    need(type(results) is list)
    observed = set()
    for result in results:
        need(result.get("fullyExplored") is True)
        binding = result.get("iamBinding", {})
        role = binding.get("role")
        need("condition" not in binding and member in binding.get("members", [])
             and role in approved
             and result.get("attachedResourceFullName") in approved[role])
        observed.add(role)
    need(required <= observed)


def audit_effective_grants(email, scope, approved, required=frozenset()):
    kind, identifier = scope
    member = "serviceAccount:" + email
    response = dev.run(["gcloud", "asset", "analyze-iam-policy", "--" + kind + "=" + identifier,
                        "--billing-project=" + PROJECT, "--identity=" + member,
                        "--expand-groups", "--show-response", "--format=json", "--quiet"])
    verify_effective_grants(response, member, kind + "s/" + identifier, approved, required)


def allowed_account_grants(number, kind, runtime=None):
    role = f"projects/{PROJECT}/roles/terraformNormal" + kind.title()
    approved = {role: {"//cloudresourcemanager.googleapis.com/projects/" + value
                       for value in (PROJECT, str(number))}}
    if kind == "apply" and runtime is not None:
        email = "terraform-prod-workload@" + PROJECT + ".iam.gserviceaccount.com"
        verify_account(runtime, email)
        identifiers = {email, runtime.get("uniqueId", email)}
        approved["roles/iam.serviceAccountUser"] = {
            f"//iam.googleapis.com/projects/{project}/serviceAccounts/{identity}"
            for project in (PROJECT, str(number)) for identity in identifiers}
    return approved


def bootstrap(number, allowed, *, enable_apply=False, check=False):
    kinds = ("plan", "apply") if enable_apply else ("plan",)
    # Validate *all existing* target resources before writing anything. A
    # bootstrap rerun must not overwrite an unexpected production role or WIF.
    roles = {}
    for kind in kinds:
        included = allowed[kind]
        name = "terraformNormal" + kind.title()
        current = gcloud("iam", "roles", "describe", name, missing=True)
        if current is not None:
            need(sorted(current.get("includedPermissions", [])) == included
                 and current.get("stage") == "GA")
        roles[kind] = (name, current)
    runtime = "terraform-prod-workload@" + PROJECT + ".iam.gserviceaccount.com"
    runtime_exists = gcloud("iam", "service-accounts", "describe", runtime, missing=True)
    if runtime_exists is not None:
        verify_account(runtime_exists, runtime)
    pool = gcloud("iam", "workload-identity-pools", "describe", POOL,
                  "--location=global", missing=True)
    if pool is not None:
        need(pool.get("state") == "ACTIVE" and pool.get("disabled", False) is False)
    current_provider = gcloud("iam", "workload-identity-pools", "providers", "describe",
                             "github-actions", "--workload-identity-pool=" + POOL,
                             "--location=global", missing=True)
    if current_provider is not None:
        verify_provider(current_provider)
    if pool is not None:
        verify_pool_providers(number, current_provider)
    project_policy = gcloud("projects", "get-iam-policy", PROJECT)
    need(type(project_policy) is dict and type(project_policy.get("bindings", [])) is list
         and not any("serviceAccount:" + runtime in b.get("members", [])
                     for b in project_policy["bindings"]))
    runtime_grant = ("roles/iam.serviceAccountUser",
                     "serviceAccount:terraform-prod-normal-apply@" + PROJECT + ".iam.gserviceaccount.com")
    if runtime_exists is not None:
        verify_no_user_keys(runtime)
        check_policy(gcloud("iam", "service-accounts", "get-iam-policy", runtime),
                     {runtime_grant} if enable_apply else set())
    accounts = {}
    for kind in ("plan", "apply"):
        email = "terraform-prod-normal-" + kind + "@" + PROJECT + ".iam.gserviceaccount.com"
        exists = gcloud("iam", "service-accounts", "describe", email, missing=True)
        principal = (f"principalSet://iam.googleapis.com/projects/{number}/locations/global/"
                     f"workloadIdentityPools/{POOL}/attribute.environment/terraform-prod-{kind}")
        approved_grant = ("roles/iam.workloadIdentityUser", principal)
        project_role = f"projects/{PROJECT}/roles/terraformNormal" + kind.title()
        granted = check_project_grants(project_policy, "serviceAccount:" + email, project_role)
        need(enable_apply or kind == "plan" or not granted)
        if exists is not None:
            verify_account(exists, email)
            verify_no_user_keys(email)
            check_policy(gcloud("iam", "service-accounts", "get-iam-policy", email),
                         {approved_grant} if kind in kinds else set())
        accounts[kind] = (email, exists, principal, approved_grant, project_role)

    # Analyze from the highest ancestor so resource-level, inherited and
    # group-derived grants cannot bypass the narrow Project policy check.
    scope = policy_analysis_scope(number)
    audit_effective_grants(runtime, scope, {})
    for kind, (email, _, _, _, project_role) in accounts.items():
        approved = allowed_account_grants(number, kind, runtime_exists if enable_apply else None) if kind in kinds else {}
        required = {project_role} if check_project_grants(project_policy, "serviceAccount:" + email, project_role) else set()
        audit_effective_grants(email, scope, approved, required)

    if check:
        return

    # Only absent resources are created; matching existing resources stay intact.
    for kind in kinds:
        included = allowed[kind]
        name, existing = roles[kind]
        if existing is None:
            gcloud("iam", "roles", "create", name,
                   "--permissions=" + ",".join(included), "--stage=GA",
                   "--title=Terraform PROD normal " + kind)
        updated = gcloud("iam", "roles", "describe", name)
        need(sorted(updated.get("includedPermissions", [])) == included
             and updated.get("stage") == "GA")

    if enable_apply and runtime_exists is None:
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
    verify_pool_providers(number, provider)
    if enable_apply:
        wait_for_account(runtime)
        verify_no_user_keys(runtime)
        runtime_exists = gcloud("iam", "service-accounts", "describe", runtime)
        audit_effective_grants(runtime, scope, {})

    for kind in kinds:
        account, exists, principal, approved_grant, project_role = accounts[kind]
        if exists is None:
            gcloud("iam", "service-accounts", "create", "terraform-prod-normal-" + kind,
                   "--display-name=Terraform PROD normal " + kind)
        wait_for_account(account)
        verify_no_user_keys(account)
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
        required = {project_role} | ({"roles/iam.serviceAccountUser"} if kind == "apply" else set())
        audit_effective_grants(account, scope, allowed_account_grants(number, kind, runtime_exists), required)
        env = "terraform-prod-" + kind
        dev.run(["gh", "secret", "set", "GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT",
                 "--repo", REPO, "--env", env], data=account)
        dev.run(["gh", "secret", "set", "GCP_TERRAFORM_NORMAL_WIF_PROVIDER",
                 "--repo", REPO, "--env", env], data=provider["name"])
        secrets = gh(f"repos/{REPO}/environments/{env}/secrets")
        need({"GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT", "GCP_TERRAFORM_NORMAL_WIF_PROVIDER"}
             <= {s.get("name") for s in secrets.get("secrets", [])})
        print("PROD_NORMAL_BOOTSTRAP: PASS; role=" + kind + "; provider_verified=true")


def verify_initial_noop(run_id):
    """Verify a successful owner-dispatched normal Plan on current main.

    Only a reviewed main workflow can emit this marker after native full-root
    no-op/output/30-managed-resource and unchanged-State verification. Migration
    jobs, stale revisions, reruns and copied/failed logs are not evidence.
    """
    need(re.fullmatch(r"[1-9][0-9]*", run_id) is not None)
    run = gh(f"repos/{REPO}/actions/runs/{run_id}")
    sha = run.get("head_sha")
    need(type(sha) is str and re.fullmatch(r"[0-9a-f]{40}", sha) is not None)
    need(run.get("id") == int(run_id) and run.get("run_attempt") == 1
         and run.get("head_branch") == "main" and run.get("event") == "workflow_dispatch"
         and run.get("status") == "completed" and run.get("conclusion") == "success"
         and run.get("path") == ".github/workflows/ci.yml"
         and run.get("repository", {}).get("id") == 340900071
         and run.get("head_repository", {}).get("id") == 340900071
         and all(run.get(key, {}).get("id") == 54093651 for key in ("actor", "triggering_actor")))
    need(gh(f"repos/{REPO}/branches/main").get("commit", {}).get("sha") == sha)
    jobs = gh(f"repos/{REPO}/actions/runs/{run_id}/jobs?per_page=100")
    need(type(jobs.get("total_count")) is int and jobs["total_count"] <= 100
         and jobs["total_count"] == len(jobs.get("jobs", [])))
    selected = [job for job in jobs["jobs"] if job.get("name", "").endswith(" / Terraform Plan (prod)")]
    need(len(selected) == 1 and selected[0].get("conclusion") == "success")
    job = selected[0]
    need(job.get("run_id") == int(run_id) and job.get("head_sha") == sha)
    marker = f"GCP_PROD_INITIAL_NOOP: PASS; source={sha}; run={run_id}; managed=30; state_unchanged=true"
    logs = subprocess.run(["gh", "api", f"repos/{REPO}/actions/jobs/{job['id']}/logs"],
                              capture_output=True, text=True, timeout=120)
    need(logs.returncode == 0)
    logs = logs.stdout
    need(type(logs) is str and sum(line.endswith(marker) for line in logs.splitlines()) == 1)
    return sha


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--check", action="store_true")
    mode.add_argument("--apply", action="store_true")
    parser.add_argument("--backend-config", type=Path, required=True)
    parser.add_argument("--enable-apply", action="store_true")
    parser.add_argument("--noop-run-id")
    args = parser.parse_args()
    need(args.enable_apply == (args.noop_run_id is not None))
    config = load_backend(args.backend_config)
    preflight_cloud(config)
    preflight_github()
    if args.enable_apply:
        verify_initial_noop(args.noop_run_id)
    number, roles = permissions()
    bootstrap(number, roles, enable_apply=args.enable_apply, check=args.check)
    print(json.dumps({"project": PROJECT, "phase": "apply" if args.enable_apply else "plan",
                      "read_only": args.check, "permission_counts": {k: len(v) for k, v in roles.items()},
                      "state_mutations": 0, "production_apply_not_run": True}, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("PROD_NORMAL_BOOTSTRAP: STOP; raw provider output suppressed", file=sys.stderr)
        raise SystemExit(1)
