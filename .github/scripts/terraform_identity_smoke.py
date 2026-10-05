#!/usr/bin/env python3
"""Development-only identity smoke. Emit fixed labels, never API responses.

All probes are reads, testIamPermissions, or safely conditional denied requests.
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


def aws(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["aws", *args, "--output", "json"], capture_output=True, text=True, timeout=60)


def denied_aws(result: subprocess.CompletedProcess[str], label: str) -> None:
    # Network errors, missing objects, and precondition failures are not DENY evidence.
    if result.returncode == 0 or not any(code in result.stderr for code in ("(AccessDenied)", "(403)")):
        raise SmokeFailure(label)


def verify_oidc(env: dict[str, str]) -> list[str]:
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
    with tempfile.TemporaryDirectory(dir=env["RUNNER_TEMP"]) as directory:
        token_file = Path(directory) / "token"
        token_file.write_text(token)
        denied_aws(aws(
            "sts", "assume-role-with-web-identity", "--role-arn", plan_role[:-5] + "-apply",
            "--role-session-name", "terraform-wrong-environment-smoke",
            "--web-identity-token", f"file://{token_file}", "--duration-seconds", "900",
        ), "aws-wrong-environment-assume-role-denied")
    return ["GitHub exact immutable/ref/workflow OIDC claims", "GitHub caller/reusable workflow same SHA", "AWS wrong Environment AssumeRole denied"]


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


def verify_google(token: str, service_account: str, request=google, *, project: str = "test-youtube-study-space") -> list[str]:
    if not service_account.endswith(f"@{project}.iam.gserviceaccount.com"):
        raise SmokeFailure("development-service-account-target")
    status, data = request(f"v3/projects/{project}", token)
    if status != 200 or data.get("projectId") != project or data.get("state") != "ACTIVE":
        raise SmokeFailure("gcp-plan-service-account-read")
    permissions = [
        "resourcemanager.projects.update", "resourcemanager.projects.delete",
        "resourcemanager.projects.setIamPolicy", "iam.serviceAccounts.create",
        "iam.serviceAccounts.setIamPolicy", "iam.serviceAccounts.getAccessToken",
        "secretmanager.versions.access", "storage.buckets.create", "bigquery.datasets.create",
        "bigquery.datasets.update", "bigquery.datasets.delete", "bigquery.tables.create",
        "bigquery.tables.update", "bigquery.tables.delete", "bigquery.tables.getData",
        "bigquery.jobs.create",
    ]
    status, data = request(f"v3/projects/{project}:testIamPermissions", token, {"permissions": permissions})
    if status != 200 or data.get("permissions", []):
        raise SmokeFailure("gcp-plan-workload-mutation-denied")
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
    return ["GCP WIF + plan SA harmless read", "GCP plan mutation permissions absent", "GCP dev to prod permissions absent", "GCP unrelated SA impersonation denied"]


def verify_aws(env: dict[str, str]) -> list[str]:
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
        # Existing-object precondition makes an unexpected grant harmless: even a
        # mistaken PutObject allow cannot overwrite the verified existing state.
        empty = Path(directory) / "empty"
        empty.write_bytes(b"")
        result = aws("s3api", "put-object", "--bucket", bucket, "--key", key, "--body", str(empty), "--if-none-match", "*")
        denied_aws(result, "aws-plan-state-write-denied")
    for prefix, label in [("youtube-study-space/prod/", "aws-production-prefix-denied"), ("unrelated-product/dev/", "aws-other-product-prefix-denied")]:
        denied_aws(aws("s3api", "list-objects-v2", "--bucket", bucket, "--prefix", prefix, "--max-keys", "1"), label)
        denied_aws(aws("s3api", "head-object", "--bucket", bucket, "--key", prefix + "terraform.tfstate"), label)
    return ["AWS OIDC + dedicated STS identity", "AWS development state read", "AWS plan state PutObject denied", "AWS production/other-product prefixes denied"]


def main() -> int:
    os.umask(0o077)
    try:
        env = os.environ
        checks = verify_oidc(env)
        checks += verify_google(env["GCP_SMOKE_ACCESS_TOKEN"], env["GCP_SMOKE_SERVICE_ACCOUNT"])
        checks += verify_aws(env)
        summary = "### Development identity smoke\n\n" + "".join(f"- PASS: {label}\n" for label in checks)
        with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as output:
            output.write(summary)
        print("Development identity smoke: PASS")
        return 0
    except SmokeFailure as error:
        print(f"::error::Development identity smoke failed: {error}", file=sys.stderr)
    except Exception:
        # Do not expose URLs, API response bodies, env values, or subprocess errors.
        print("::error::Development identity smoke could not complete; raw error suppressed.", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
