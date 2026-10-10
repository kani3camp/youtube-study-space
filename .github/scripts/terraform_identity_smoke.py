#!/usr/bin/env python3
"""Development-only identity smoke. Emit fixed labels, never API responses.

plan-read-only and apply-read-only perform only bounded dev reads and dev
testIamPermissions. The separately gated security-probe performs negative
tests with private before/after state checks; omitted checks are not DENY evidence.
No credentials, raw state, identifiers, or dependency errors go to public output.
"""
from __future__ import annotations

import json
import base64
import hashlib
import os
import re
import secrets
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request
from pathlib import Path

from terraform_identity_diagnostics import SmokeFailure, StageFailure, at_stage


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
    if (result.returncode == 0 or re.search(r"\((?:412|404|409|PreconditionFailed|NoSuchKey|ConditionalRequestConflict)\)", result.stderr)
            or not re.search(r"\((?:AccessDenied|403)\)", result.stderr)):
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


def verify_export_topic_google(token, service_account, request=google, *, project="test-youtube-study-space", scheduler=False, function=False, plan_read_only=False, dev_only=False):
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
    if not (plan_read_only or dev_only):
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
    return ["GCP topic exact GET only", "GCP topic publish/subscription/mutation/list/IAM grants absent", "GCP Scheduler/Function GET remains ungranted"] + (["GCP topic production grants absent"] if not (plan_read_only or dev_only) else [])


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


def capture_exact_state_and_lock(env: dict[str, str], path: Path) -> tuple[dict, str]:
    """Private exact-version body and lock snapshot; never print state or headers."""
    from terraform_history_plan_receipt import MAX_BYTES, STATE_KEY, absent, head, private_bytes, s3

    first = head(env, MAX_BYTES, request=aws)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    os.close(fd)
    result = s3(env, "get-object", "--key", STATE_KEY, "--range", f"bytes=0-{MAX_BYTES}", str(path), request=aws)
    raw = private_bytes(path)
    if (result.get("VersionId") != first["VersionId"] or result.get("ETag") != first["ETag"]
            or result.get("ContentLength") != first["ContentLength"] or len(raw) != first["ContentLength"]
            or head(env, MAX_BYTES, request=aws) != first):
        raise SmokeFailure("aws-state-probe-snapshot")
    state_counts(json.loads(raw))
    absent(env, STATE_KEY + ".tflock", request=aws)
    return first, hashlib.sha256(raw).hexdigest()


def verify_aws_identity(env: dict[str, str], *, require_role_id: bool = False) -> None:
    identity = aws("sts", "get-caller-identity")
    if identity.returncode:
        raise SmokeFailure("aws-dedicated-state-account")
    caller = json.loads(identity.stdout)
    if caller.get("Account") != env["STATE_ACCOUNT_ID"]:
        raise SmokeFailure("aws-dedicated-state-account")
    if require_role_id:
        role_id = env.get("BACKEND_ROLE_ID", "")
        if not role_id or caller.get("UserId", "").split(":", 1)[0] != role_id:
            raise SmokeFailure("aws-plan-role-id")


def verify_aws(env: dict[str, str], *, plan_read_only: bool = False, apply_read_only: bool = False) -> list[str]:
    if plan_read_only and apply_read_only:
        raise SmokeFailure("aws-identity-mode")
    at_stage("aws-sts", lambda: verify_aws_identity(env, require_role_id=not (plan_read_only or apply_read_only)))
    bucket, key = env["STATE_BUCKET"], env["STATE_KEY"]
    if key != "youtube-study-space/dev/terraform.tfstate":
        raise StageFailure("aws-state-read", "check-failed")
    if (plan_read_only or apply_read_only) and env.get("OWNERSHIP_WAVE", "none") != "none":
        # Runtime performs its full-state/commitment capture after fresh history
        # metadata. Never route an adopted runtime graph through baseline11.
        sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "infra/gcp/scripts"))
        from runtime_ownership_provenance import load_inputs
        load_inputs(env)
        return ["AWS OIDC + dedicated STS identity", "Runtime state verification follows fresh private metadata"]
    if plan_read_only and env.get("TF_VAR_manage_user_activity_history") == "true":
        # Reuse this initial read for the complete private receipt. The cost
        # policy is validated before authentication and before any native lock.
        from terraform_history_plan_receipt import snapshot_before
        snapshot_before(env, request=aws)
        return ["AWS OIDC + dedicated STS identity", "AWS exact persistent eleven-resource state verified privately",
                "Owner-evidenced cost bound and native lock/workspace absence prechecks"]
    with tempfile.TemporaryDirectory(dir=env["RUNNER_TEMP"]) as directory:
        if apply_read_only:
            # MODE=apply has no history plan-cost receipt. The one-shot verifier
            # separately checks exact eleven-resource state before any apply.
            at_stage("aws-state-read", lambda: capture_exact_state_and_lock(env, Path(directory) / "apply-state.json"))
            return ["AWS OIDC + dedicated STS identity", "AWS exact state and native lock read privately"]
        if plan_read_only:
            state = Path(directory) / "state.json"
            def read_state():
                result = aws("s3api", "get-object", "--bucket", bucket, "--key", key, str(state))
                if result.returncode or not state.is_file():
                    raise SmokeFailure("aws-development-state-read")
                return state_counts(json.loads(state.read_text()))
            counts = at_stage("aws-state-read", read_state)
            return ["AWS OIDC + dedicated STS identity", "AWS development state read", counts]
        # A random ETag unequal to a fresh HEAD cannot create an absent key.
        # If permission were unexpectedly granted and an object raced to that
        # ETag, a write is still theoretically possible. The gate stays closed
        # until owner review. Reuse the current body to limit that residual harm.
        before, before_digest = at_stage("aws-state-read", lambda: capture_exact_state_and_lock(env, Path(directory) / "probe-before.json"))
        candidate = secrets.token_hex(16)
        if candidate == before["ETag"].strip('"'):
            raise StageFailure("aws-state-probe-request", "check-failed")
        result = at_stage("aws-state-probe-request", lambda: aws(
            "s3api", "put-object", "--bucket", bucket, "--expected-bucket-owner", env["STATE_ACCOUNT_ID"],
            "--key", key, "--body", str(Path(directory) / "probe-before.json"), "--if-match", f'"{candidate}"'))
        after, after_digest = at_stage("aws-state-probe-post", lambda: capture_exact_state_and_lock(
            env, Path(directory) / "probe-after.json"))
        try:
            denied_aws(result, "aws-plan-state-write-denied")
            put_denied = True
        except SmokeFailure:
            put_denied = False
        from terraform_history_plan_receipt import write_private
        at_stage("aws-state-probe-receipt", lambda: write_private(Path(directory) / "probe-private-receipt.json", {
            "before_head": before, "before_body_sha256": before_digest, "before_lock_absent": True,
            "after_head": after, "after_body_sha256": after_digest, "after_lock_absent": True,
            "if_match": f'"{candidate}"', "put_denied_403": put_denied,
        }))
        if after != before or after_digest != before_digest:
            raise StageFailure("aws-state-probe-invariant", "check-failed")
        if not put_denied:
            raise StageFailure("aws-state-probe-deny", "check-failed")
    for prefix, label in [("youtube-study-space/prod/", "aws-production-prefix-denied"), ("unrelated-product/dev/", "aws-other-product-prefix-denied")]:
        denied_aws(aws("s3api", "list-objects-v2", "--bucket", bucket, "--prefix", prefix, "--max-keys", "1"), label)
        denied_aws(aws("s3api", "head-object", "--bucket", bucket, "--key", prefix + "terraform.tfstate"), label)
    return ["AWS OIDC + dedicated STS identity", "AWS exact state and lock snapshots unchanged",
            "AWS plan state PutObject denied for mismatched If-Match request",
            "AWS production/other-product prefixes denied"]


def main(argv: list[str] | None = None) -> int:
    os.umask(0o077)
    try:
        env = os.environ
        args = sys.argv[1:] if argv is None else argv
        if args == ["quota-apply"]:
            checks = at_stage("gcp-quota", lambda: verify_quota_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"], identity="apply"))
        elif args in (["export-function"], ["export-function-apply-read-only"]):
            project = "test-youtube-study-space"
            if args == ["export-function-apply-read-only"] and (env.get("MODE") != "apply" or
                    env.get("GCP_SMOKE_SERVICE_ACCOUNT") != f"terraform-dev-apply@{project}.iam.gserviceaccount.com"):
                raise StageFailure("identity-mode", "check-failed")
            checks = at_stage("gcp-export", lambda: verify_export_topic_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"],
                scheduler=True, function=True, dev_only=args == ["export-function-apply-read-only"]))
            checks += at_stage("gcp-function", lambda: configure_function_execution_identity(env["GCP_SMOKE_ACCESS_TOKEN"], env))
        elif args == ["export-scheduler"]:
            checks = at_stage("gcp-export", lambda: verify_export_topic_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"], scheduler=True))
        elif args == ["export-topic"]:
            checks = at_stage("gcp-export", lambda: verify_export_topic_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"]))
        elif args in (["plan-read-only"], ["apply-read-only"], ["post-noop-read-only"], ["security-probe"]):
            if (args == ["security-probe"] and
                    (env.get("MODE") != "security-probe" or env.get("DEV_TERRAFORM_SECURITY_PROBE_ENABLED") != "true")):
                raise StageFailure("identity-mode", "check-failed")
            project = "test-youtube-study-space"
            if (args == ["security-probe"] and
                    env.get("GCP_SMOKE_SERVICE_ACCOUNT") !=
                    f"terraform-dev-plan@{project}.iam.gserviceaccount.com"):
                raise StageFailure("gcp-plan-service-account-target", "check-failed")
            if (args == ["apply-read-only"] and
                    env.get("MODE") not in {"apply", "email-adoption", "quota-create", "quota-refresh"}):
                raise StageFailure("identity-mode", "check-failed")
            if (args == ["post-noop-read-only"] and
                    (env.get("MODE") != "plan" or env.get("HISTORY_POST_NOOP") != "true"
                     or env.get("DEV_HISTORY_POST_NOOP_ENABLED") != "true"
                     or env.get("TF_VAR_manage_user_activity_history") != "true")):
                raise StageFailure("identity-mode", "check-failed")
            plan_read_only = args == ["plan-read-only"]
            apply_read_only = args in (["apply-read-only"], ["post-noop-read-only"])
            positive_only = plan_read_only or apply_read_only
            if args == ["security-probe"]:
                # Verify the live role before any negative STS or Google request.
                at_stage("aws-sts", lambda: verify_aws_identity(env, require_role_id=True))
            checks = at_stage("oidc", lambda: verify_oidc(env, plan_read_only=positive_only))
            checks += at_stage("gcp-project", lambda: verify_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"], plan_read_only=positive_only))
            if env.get("QUOTA_IDENTITY_REQUIRED") == "true":
                checks += at_stage("gcp-quota", lambda: verify_quota_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"], identity="plan", plan_read_only=positive_only))
            if env.get("EXPORT_TOPIC_IDENTITY_REQUIRED") == "true":
                checks += at_stage("gcp-export", lambda: verify_export_topic_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"],
                    scheduler=env.get("EXPORT_SCHEDULER_IDENTITY_REQUIRED") == "true",
                    function=env.get("EXPORT_FUNCTION_IDENTITY_REQUIRED") == "true", plan_read_only=positive_only))
                if env.get("EXPORT_FUNCTION_IDENTITY_REQUIRED") == "true":
                    checks += at_stage("gcp-function", lambda: configure_function_execution_identity(env["GCP_SMOKE_ACCESS_TOKEN"], env))
            checks += verify_aws(env, plan_read_only=plan_read_only, apply_read_only=apply_read_only)
        else:
            raise StageFailure("identity-mode", "check-failed")
        title = ("Development plan read-only checks" if args == ["plan-read-only"] else
                 "Development apply read-only checks" if args == ["apply-read-only"] else
                 "Development post-import read-only checks" if args == ["post-noop-read-only"] else
                 "Development conditional security probe" if args == ["security-probe"] else "Development identity smoke")
        summary = f"### {title}\n\n" + "".join(f"- PASS: {label}\n" for label in checks)
        if args in (["plan-read-only"], ["apply-read-only"], ["post-noop-read-only"]):
            mode = args[0]
            summary += "".join(f"- SKIPPED ({mode}): {label}\n" for label in (
                "AWS apply-role AssumeRole DENY probe",
                "GCP apply-SA generateAccessToken DENY probe",
                "AWS state-body PutObject DENY probe",
                "AWS production/other-product prefix read/list DENY probes",
                "GCP production permission checks",
            ))
            summary += "\nFull security gate: NOT VERIFIED by this read-only identity check.\n"
        if args == ["security-probe"]:
            summary += "\nConditional PutObject denial is evidence for this request/header only; owner review of role, bucket, session policy and SCP remains required.\n"
        def write_summary():
            with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as output:
                output.write(summary)
        at_stage("identity-output", write_summary)
        print("Development plan read-only checks: PASS; full security gate: NOT VERIFIED"
              if args == ["plan-read-only"] else
              "Development apply read-only checks: PASS; full security gate: NOT VERIFIED"
              if args == ["apply-read-only"] else
              "Development post-import read-only checks: PASS; full security gate: NOT VERIFIED"
              if args == ["post-noop-read-only"] else
              "Development conditional security probe: PASS; request/header only"
              if args == ["security-probe"] else "Development identity smoke: PASS")
        return 0
    except StageFailure as error:
        reason = f"; reason={error.reason}" if error.reason is not None else ""
        print(f"::error::Development identity smoke STOP; stage={error.stage}; category={error.category}{reason}.", file=sys.stderr)
    except SmokeFailure:
        print("::error::Development identity smoke STOP; stage=identity-mode; category=check-failed.", file=sys.stderr)
    except Exception:
        # Do not expose URLs, API response bodies, env values, or subprocess errors.
        print("::error::Development identity smoke STOP; stage=identity-mode; category=dependency-error.", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
