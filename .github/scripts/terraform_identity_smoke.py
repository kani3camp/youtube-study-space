#!/usr/bin/env python3
"""Development-only identity smoke. Emit fixed labels, never API responses.

plan-read-only performs only dev reads and dev testIamPermissions. Legacy modes
retain negative credential/write probes; omitted checks are not DENY evidence.
No credentials, raw state, identifiers, or dependency errors go to public output.
"""
from __future__ import annotations

import json
import base64
import os
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request
from pathlib import Path


class SmokeFailure(Exception):
    pass


FORBIDDEN_PERMISSIONS = (
    "resourcemanager.projects.update", "resourcemanager.projects.delete",
    "resourcemanager.projects.setIamPolicy", "iam.serviceAccounts.create",
    "iam.serviceAccounts.setIamPolicy", "iam.serviceAccounts.getAccessToken",
    "secretmanager.versions.access", "storage.buckets.create", "bigquery.datasets.create",
    "storage.buckets.update", "storage.objects.get", "storage.objects.list",
    "storage.objects.create", "storage.objects.delete",
    "monitoring.notificationChannels.create", "monitoring.notificationChannels.update",
    "monitoring.notificationChannels.delete", "monitoring.alertPolicies.create",
    "monitoring.alertPolicies.update", "monitoring.alertPolicies.delete",
    "monitoring.alertPolicies.list", "monitoring.notificationChannels.list",
    "serviceusage.services.enable", "serviceusage.services.disable",
    "bigquery.datasets.update", "bigquery.datasets.delete", "bigquery.tables.create",
    "bigquery.tables.update", "bigquery.tables.delete", "bigquery.tables.getData",
    "bigquery.jobs.create",
)


def aws(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["aws", *args, "--output", "json"], capture_output=True, text=True, timeout=60)


def denied_aws(result: subprocess.CompletedProcess[str], label: str) -> None:
    # Network errors, missing objects, and precondition failures are not DENY evidence.
    if result.returncode == 0 or not any(code in result.stderr for code in ("(AccessDenied)", "(403)")):
        raise SmokeFailure(label)


def verify_oidc(env: dict[str, str], *, plan_read_only: bool = False) -> list[str]:
    request = urllib.request.Request(
        env["ACTIONS_ID_TOKEN_REQUEST_URL"] + "&audience=sts.amazonaws.com",
        headers={"Authorization": "Bearer " + env["ACTIONS_ID_TOKEN_REQUEST_TOKEN"]},
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        token = json.load(response)["value"]
    encoded = token.split(".")[1]
    claims = json.loads(base64.urlsafe_b64decode(encoded + "=" * (-len(encoded) % 4)))
    expected = {
        "repository_id": "340900071", "repository_owner_id": "54093651",
        "environment": "terraform-dev-plan", "ref": "refs/heads/feature/gcp-terraform-iac",
        "workflow_ref": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
        "job_workflow_ref": "kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/feature/gcp-terraform-iac",
        "event_name": "workflow_dispatch",
    }
    subject = ":".join(f"{key}:{value.replace(':', '%3A')}" for key, value in expected.items())
    if claims.get("sub") != subject or any(claims.get(k) != v for k, v in expected.items()):
        raise SmokeFailure("github-exact-oidc-claims")
    if claims.get("job_workflow_sha") != env["GITHUB_SHA"] or claims.get("workflow_sha") != env["GITHUB_SHA"]:
        raise SmokeFailure("github-same-commit-oidc-claims")
    plan_role = env["BACKEND_ROLE_ARN"]
    if not plan_role.endswith("-plan"):
        raise SmokeFailure("aws-plan-role-target")
    checks = ["GitHub exact immutable/ref/workflow OIDC claims", "GitHub caller/reusable workflow same SHA"]
    if plan_read_only:
        return checks
    with tempfile.TemporaryDirectory(dir=env["RUNNER_TEMP"]) as directory:
        token_file = Path(directory) / "token"
        token_file.write_text(token)
        denied_aws(aws(
            "sts", "assume-role-with-web-identity", "--role-arn", plan_role[:-5] + "-apply",
            "--role-session-name", "terraform-wrong-environment-smoke",
            "--web-identity-token", f"file://{token_file}", "--duration-seconds", "900",
        ), "aws-wrong-environment-assume-role-denied")
    return checks + ["AWS wrong Environment AssumeRole denied"]


def google(path: str, token: str, body: dict | None = None, *, host: str = "cloudresourcemanager.googleapis.com") -> tuple[int, dict]:
    request = urllib.request.Request(
        f"https://{host}/{path}",
        data=json.dumps(body).encode() if body is not None else None,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as error:
        # Consume privately; never print dependency error text or resource identifiers.
        return error.code, json.loads(error.read())


def verify_google(token: str, service_account: str, request=google, *, project: str = "test-youtube-study-space", plan_read_only: bool = False) -> list[str]:
    if not service_account.endswith(f"@{project}.iam.gserviceaccount.com"):
        raise SmokeFailure("development-service-account-target")
    if plan_read_only and service_account != f"terraform-dev-plan@{project}.iam.gserviceaccount.com":
        raise SmokeFailure("gcp-read-only-plan-identity")
    status, data = request(f"v3/projects/{project}", token)
    if status != 200 or data.get("projectId") != project or data.get("state") != "ACTIVE":
        raise SmokeFailure("gcp-plan-service-account-read")
    permissions = list(FORBIDDEN_PERMISSIONS)
    status, data = request(f"v3/projects/{project}:testIamPermissions", token, {"permissions": permissions})
    if status != 200 or data.get("permissions", []):
        raise SmokeFailure("gcp-plan-workload-mutation-denied")
    checks = ["GCP WIF + plan SA harmless read", "GCP plan mutation permissions absent"]
    if plan_read_only:
        return checks
    status, data = request("v3/projects/youtube-study-space:testIamPermissions", token, {"permissions": [
        "resourcemanager.projects.get", "bigquery.datasets.get", "bigquery.tables.get", *permissions
    ]})
    if not ((status == 200 and not data.get("permissions", [])) or (status == 403 and data.get("error", {}).get("status") == "PERMISSION_DENIED")):
        raise SmokeFailure("gcp-development-to-production-denied")
    # Plan SA must not mint a token for even the separately provisioned apply SA.
    unrelated = service_account.replace("terraform-dev-plan@", "terraform-dev-apply@")
    if unrelated == service_account:
        raise SmokeFailure("gcp-unrelated-service-account-target")
    status, data = request(
        f"v1/projects/-/serviceAccounts/{unrelated}:generateAccessToken", token,
        {"scope": ["https://www.googleapis.com/auth/cloud-platform"], "lifetime": "600s"},
        host="iamcredentials.googleapis.com",
    )
    if status != 403 or data.get("error", {}).get("status") != "PERMISSION_DENIED":
        raise SmokeFailure("gcp-unrelated-service-account-impersonation-denied")
    return checks + ["GCP dev to prod permissions absent", "GCP unrelated SA impersonation denied"]


def verify_quota_google(token: str, service_account: str, request=google, *, identity: str, project: str = "test-youtube-study-space", plan_read_only: bool = False) -> list[str]:
    """Only metadata GET and, for the existing apply identity, policy CREATE."""
    if identity not in {"plan", "apply"} or service_account != f"terraform-dev-{identity}@{project}.iam.gserviceaccount.com":
        raise SmokeFailure("quota-development-identity-target")
    if plan_read_only and identity != "plan":
        raise SmokeFailure("quota-read-only-plan-identity")
    requested = ["monitoring.alertPolicies.get", *FORBIDDEN_PERMISSIONS]
    expected = {"monitoring.alertPolicies.get"}
    if identity == "apply":
        expected.add("monitoring.alertPolicies.create")
    status, data = request(f"v3/projects/{project}:testIamPermissions", token, {"permissions": requested})
    if status != 200 or set(data.get("permissions", [])) != expected:
        raise SmokeFailure("quota-exact-development-permissions")
    checks = [f"GCP quota {identity} exact GET" + (" + CREATE" if identity == "apply" else " only"),
              "GCP quota update/delete/list/data/IAM/API grants absent"]
    if plan_read_only:
        return checks
    status, data = request("v3/projects/youtube-study-space:testIamPermissions", token, {"permissions": [
        "resourcemanager.projects.get", "storage.buckets.get", "monitoring.notificationChannels.get",
        "bigquery.datasets.get", "bigquery.tables.get", "datastore.backupSchedules.get", *requested
    ]})
    if not ((status == 200 and not data.get("permissions", [])) or (status == 403 and data.get("error", {}).get("status") == "PERMISSION_DENIED")):
        raise SmokeFailure("quota-production-permissions-denied")
    return checks + ["GCP quota production grants absent"]


FUNCTION_FORBIDDEN_PERMISSIONS = tuple("cloudfunctions.functions." + suffix for suffix in (
    "list", "create", "update", "delete", "call", "invoke", "sourceCodeGet", "sourceCodeSet", "getIamPolicy", "setIamPolicy"))


EXPORT_TOPIC_PERMISSIONS = (
    "pubsub.topics.get", "pubsub.topics.publish", "pubsub.topics.create",
    "pubsub.topics.update", "pubsub.topics.delete", "pubsub.topics.list",
    "pubsub.topics.getIamPolicy", "pubsub.topics.setIamPolicy",
    "pubsub.subscriptions.get", "pubsub.subscriptions.list",
    "pubsub.subscriptions.create", "pubsub.subscriptions.update",
    "pubsub.subscriptions.delete", "pubsub.subscriptions.consume",
    "cloudscheduler.jobs.get", "cloudfunctions.functions.get",
)


def verify_export_topic_google(token, service_account, request=google, *, project="test-youtube-study-space", scheduler=False, function=False, plan_read_only=False):
    if service_account not in {f"terraform-dev-{kind}@{project}.iam.gserviceaccount.com" for kind in ("plan", "apply")}:
        raise SmokeFailure("topic-development-identity-target")
    if plan_read_only and service_account != f"terraform-dev-plan@{project}.iam.gserviceaccount.com":
        raise SmokeFailure("topic-read-only-plan-identity")
    if function and not scheduler:
        raise SmokeFailure("function-scheduler-dependency")
    requested = list(EXPORT_TOPIC_PERMISSIONS) + ["cloudscheduler.jobs.list", "cloudscheduler.jobs.create",
        "cloudscheduler.jobs.update", "cloudscheduler.jobs.delete", "cloudscheduler.jobs.run",
        "cloudscheduler.jobs.pause", "cloudscheduler.jobs.enable", "cloudscheduler.jobs.fullView",
        *FUNCTION_FORBIDDEN_PERMISSIONS]
    expected = {"pubsub.topics.get"} | ({"cloudscheduler.jobs.get"} if scheduler else set()) | ({"cloudfunctions.functions.get"} if function else set())
    status, data = request(f"v3/projects/{project}:testIamPermissions", token, {"permissions": requested})
    if status != 200 or set(data.get("permissions", [])) != expected:
        raise SmokeFailure("topic-exact-get-only-permissions")
    production = []
    if not plan_read_only:
        status, data = request("v3/projects/youtube-study-space:testIamPermissions", token, {"permissions": requested})
        if not ((status == 200 and not data.get("permissions", [])) or (status == 403 and data.get("error", {}).get("status") == "PERMISSION_DENIED")):
            raise SmokeFailure("topic-production-permissions-denied")
        production = ["GCP export production grants absent"]
    if function:
        return ["GCP topic + Scheduler + Function exact GET only", "GCP Function list/mutation/call/invoke/sourceCode/IAM grants absent",
                "GCP export trigger/publish/subscription grants absent"] + production
    if scheduler:
        return ["GCP topic + Scheduler exact GET only", "GCP Scheduler list/mutation/run/pause/resume/publish grants absent",
                "GCP Function GET remains ungranted"] + production
    return ["GCP topic exact GET only", "GCP topic publish/subscription/mutation/list/IAM grants absent", "GCP Scheduler/Function GET remains ungranted"] + (["GCP topic production grants absent"] if not plan_read_only else [])


def configure_function_execution_identity(token, env, request=google):
    # Exact metadata GET only; never source download, build lookup or invocation.
    project = "test-youtube-study-space"
    path = f"v1/projects/{project}/locations/asia-southeast2/functions/firestoreCollectionsExport"
    status, data = request(path, token, host="cloudfunctions.googleapis.com")
    email = data.get("serviceAccountEmail", "")
    if status != 200 or email != f"{project}@appspot.gserviceaccount.com" or any(data.get(k) != v for k, v in {
            "name": path[3:], "runtime": "nodejs22", "status": "ACTIVE", "versionId": "8"}.items()):
        raise SmokeFailure("function-fresh-execution-identity")
    # Register masks before introducing the private Terraform input to later steps.
    for value in (email, email.split("@")[0]):
        print(f"::add-mask::{value}")
    with open(env["GITHUB_ENV"], "a", encoding="utf-8") as output:
        output.write(f"TF_VAR_export_function_execution_service_account_email={email}\n")
    return ["GCP exact Function metadata GET and preserved execution identity"]


def state_counts(state: dict) -> str:
    """Expose numeric integrity metadata only, never state values/identifiers."""
    if state.get("version") != 4 or not isinstance(state.get("serial"), int):
        raise SmokeFailure("aws-state-format")
    resources = state.get("resources")
    if not isinstance(resources, list) or not isinstance(state.get("outputs"), dict):
        raise SmokeFailure("aws-state-structure")
    count = sum(len(r["instances"]) for r in resources if r.get("mode") == "managed")
    return f"AWS current state resource count {count}, serial {state['serial']}, output count {len(state['outputs'])}"


def verify_aws(env: dict[str, str], *, plan_read_only: bool = False) -> list[str]:
    identity = aws("sts", "get-caller-identity")
    if identity.returncode or json.loads(identity.stdout).get("Account") != env["STATE_ACCOUNT_ID"]:
        raise SmokeFailure("aws-dedicated-state-account")
    bucket, key = env["STATE_BUCKET"], env["STATE_KEY"]
    if key != "youtube-study-space/dev/terraform.tfstate":
        raise SmokeFailure("aws-development-state-key")
    with tempfile.TemporaryDirectory(dir=env["RUNNER_TEMP"]) as directory:
        state = Path(directory) / "state.json"
        result = aws("s3api", "get-object", "--bucket", bucket, "--key", key, str(state))
        if result.returncode or not state.is_file():
            raise SmokeFailure("aws-development-state-read")
        counts = state_counts(json.loads(state.read_text()))
        checks = ["AWS OIDC + dedicated STS identity", "AWS development state read", counts]
        if plan_read_only:
            return checks
        # Legacy probe: an existing current object prevents overwrite, but a
        # concurrent deletion plus a mistaken grant could create an empty version.
        # plan-read-only never performs this write request.
        empty = Path(directory) / "empty"
        empty.write_bytes(b"")
        result = aws("s3api", "put-object", "--bucket", bucket, "--key", key, "--body", str(empty), "--if-none-match", "*")
        denied_aws(result, "aws-plan-state-write-denied")
    for prefix, label in [("youtube-study-space/prod/", "aws-production-prefix-denied"), ("unrelated-product/dev/", "aws-other-product-prefix-denied")]:
        denied_aws(aws("s3api", "list-objects-v2", "--bucket", bucket, "--prefix", prefix, "--max-keys", "1"), label)
        denied_aws(aws("s3api", "head-object", "--bucket", bucket, "--key", prefix + "terraform.tfstate"), label)
    return checks + ["AWS plan state PutObject denied", "AWS production/other-product prefixes denied"]


def main(argv: list[str] | None = None) -> int:
    os.umask(0o077)
    try:
        env = os.environ
        args = sys.argv[1:] if argv is None else argv
        if args == ["quota-apply"]:
            checks = verify_quota_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"], identity="apply")
        elif args == ["export-function"]:
            checks = verify_export_topic_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"], scheduler=True, function=True)
            checks += configure_function_execution_identity(env["GCP_SMOKE_ACCESS_TOKEN"], env)
        elif args == ["export-scheduler"]:
            checks = verify_export_topic_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"], scheduler=True)
        elif args == ["export-topic"]:
            checks = verify_export_topic_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"])
        elif not args or args == ["plan-read-only"]:
            plan_read_only = args == ["plan-read-only"]
            checks = verify_oidc(env, plan_read_only=plan_read_only)
            checks += verify_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"], plan_read_only=plan_read_only)
            if env.get("QUOTA_IDENTITY_REQUIRED") == "true":
                checks += verify_quota_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"], identity="plan", plan_read_only=plan_read_only)
            if env.get("EXPORT_TOPIC_IDENTITY_REQUIRED") == "true":
                checks += verify_export_topic_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"],
                    scheduler=env.get("EXPORT_SCHEDULER_IDENTITY_REQUIRED") == "true",
                    function=env.get("EXPORT_FUNCTION_IDENTITY_REQUIRED") == "true", plan_read_only=plan_read_only)
                if env.get("EXPORT_FUNCTION_IDENTITY_REQUIRED") == "true":
                    checks += configure_function_execution_identity(env["GCP_SMOKE_ACCESS_TOKEN"], env)
            checks += verify_aws(env, plan_read_only=plan_read_only)
        else:
            raise SmokeFailure("unsupported-identity-smoke-mode")
        title = "Development plan read-only checks" if args == ["plan-read-only"] else "Development identity smoke"
        summary = f"### {title}\n\n" + "".join(f"- PASS: {label}\n" for label in checks)
        if args == ["plan-read-only"]:
            summary += "".join(f"- SKIPPED (plan-read-only): {label}\n" for label in (
                "AWS apply-role AssumeRole DENY probe",
                "GCP apply-SA generateAccessToken DENY probe",
                "AWS state-body PutObject DENY probe",
                "AWS production/other-product prefix read/list DENY probes",
                "GCP production permission checks",
            ))
            summary += "\nFull security gate: NOT VERIFIED by this read-only plan.\n"
        with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as output:
            output.write(summary)
        print("Development plan read-only checks: PASS; full security gate: NOT VERIFIED"
              if args == ["plan-read-only"] else "Development identity smoke: PASS")
        return 0
    except SmokeFailure as error:
        print(f"::error::Development identity smoke failed: {error}", file=sys.stderr)
    except Exception:
        # Do not expose URLs, API response bodies, env values, or subprocess errors.
        print("::error::Development identity smoke could not complete; raw error suppressed.", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
