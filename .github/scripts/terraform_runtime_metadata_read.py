#!/usr/bin/env python3
"""Protected exact runtime metadata observations; no Terraform, State or receipts.

Six exact metadata kinds are observed twice through one Environment identity.
Snapshots stay in memory. Equality observes a bounded interval, not historical
provenance, caller ownership, all conditional IAM or migration readiness.
"""
from __future__ import annotations

import base64
from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import re
import stat
import sys
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "infra/gcp/scripts"))
from prepare_user_activity_history_workflow import PrivateArgumentParser
from terraform_identity_smoke import WIF_MUTATION_PERMISSIONS, verify_oidc_claims
import prepare_runtime_ownership as ownership

PROJECT = ownership.PROJECT
MAX_BYTES = 4 * 1024 * 1024
MAX_PAGES = 20
MAX_SERVICES = 4000
MAX_TOKEN = 2048
ALIASES = ("runtime-pool", "runtime-attestation", "runtime-provider", "runtime-sa-policy",
           "runtime-project", "runtime-enabled-services", "runtime-stable",
           "runtime-project-permissions", "runtime-window", "runtime-cleanup")
REASONS = {"PASS", "未実行", "HTTP403", "HTTP_OTHER", "RESPONSE_REJECTED", "TRANSPORT_FAILED", "ADMISSION_STOP", "BOUNDARY_REJECTED"}
REASON_KEYS = tuple(alias + "-" + phase + "-reason" for phase in ("before", "after") for alias in ALIASES[:6]) + ("runtime-project-permissions-reason",)
MUTATIONS = (*WIF_MUTATION_PERMISSIONS, "iam.serviceAccounts.setIamPolicy",
             "serviceusage.services.enable", "serviceusage.services.disable")


class RuntimeAdmission(ValueError):
    pass


class RuntimeLimit(RuntimeAdmission):
    pass


class ProbeFailure(ValueError):
    def __init__(self, reason):
        self.reason = reason
        super().__init__("runtime metadata read STOP")


def need(value):
    if not value:
        raise ValueError("runtime metadata read STOP")


def window(env, *, now=None):
    stamps = []
    for key in ("DEV_RUNTIME_METADATA_READ_ISSUED_UTC", "DEV_RUNTIME_METADATA_READ_EXPIRES_UTC"):
        value = env.get(key, "")
        need(type(value) is str and re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ", value))
        stamps.append(datetime.fromisoformat(value.replace("Z", "+00:00")))
    issued, expires = stamps
    need(issued <= (now or datetime.now(timezone.utc)) < expires <= issued + timedelta(hours=8))


def read_window(env):
    try:
        window(env)
    except Exception:
        raise RuntimeAdmission("runtime metadata read STOP") from None


def context(env, *, gate=True, token=False):
    identity = env.get("RUNTIME_READ_IDENTITY")
    need(identity in {"plan", "apply"} and env.get("MODE") == "runtime-metadata-read"
         and env.get("RUNTIME_TARGET") == "dev"
         and env.get("GCP_SMOKE_SERVICE_ACCOUNT") == f"terraform-dev-{identity}@{PROJECT}.iam.gserviceaccount.com")
    if gate:
        need(env.get("DEV_RUNTIME_METADATA_READ_ENABLED") == "true")
        window(env)
    expected = {
        "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_REPOSITORY": "kani3camp/youtube-study-space",
        "GITHUB_REPOSITORY_ID": "340900071", "GITHUB_REPOSITORY_OWNER_ID": "54093651",
        "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac", "GITHUB_RUN_ATTEMPT": "1",
        "GITHUB_JOB": f"runtime_metadata_read_{identity}",
        "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
    }
    need(all(env.get(k) == v for k, v in expected.items())
         and re.fullmatch(r"[0-9a-f]{40}", env.get("GITHUB_SHA", "")))
    need(all(not env.get(k, "") for k in ("PLAN_SAFETY_EVIDENCE", "PLAN_COST_EVIDENCE", "HISTORY_IMPORT_APPROVAL"))
         and env.get("OWNERSHIP_WAVE", "none") == "none"
         and env.get("HISTORY_POST_NOOP", "false") == "false"
         and env.get("CURRENT_STATE_ATTESTATION", "false") == "false"
         and env.get("OWNERSHIP_ROOT_BASIS", "original-history-import") == "original-history-import")
    provider = env.get("RUNTIME_READ_WIF_PROVIDER", "")
    match = re.fullmatch(r"projects/([0-9]+)/locations/global/workloadIdentityPools/[a-z0-9-]+/providers/[a-z0-9-]+", provider)
    need(match is not None)
    directory = Path(env["RUNNER_TEMP"])
    need(directory.is_absolute() and directory.is_dir() and not directory.is_symlink()
         and directory.stat().st_uid == os.geteuid() and directory.resolve() == directory
         and not directory.resolve().is_relative_to(ROOT))
    summary_path = Path(env["GITHUB_STEP_SUMMARY"])
    need(summary_path.is_absolute() and not summary_path.is_symlink()
         and summary_path.parent.resolve() == summary_path.parent)
    if summary_path.exists():
        info = summary_path.stat()
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
    need(not token or bool(env.get("GCP_SMOKE_ACCESS_TOKEN")))
    return match[1]


def load(raw):
    need(type(raw) is bytes and 0 < len(raw) <= MAX_BYTES)
    value = json.loads(raw.decode("utf-8"), object_pairs_hook=ownership.unique_object,
                       parse_constant=lambda _: need(False))
    need(type(value) is dict)
    return value


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


def response_json(response, limit=MAX_BYTES):
    length = response.headers.get("Content-Length")
    need(length is None or re.fullmatch(r"[0-9]+", length))
    if length is not None and int(length) > limit:
        raise RuntimeLimit("runtime metadata read STOP")
    raw = response.read(limit + 1)
    if len(raw) > limit:
        raise RuntimeLimit("runtime metadata read STOP")
    need(0 < len(raw) and (length is None or len(raw) == int(length)))
    return load(raw)


def google(path, token, body=None, *, host, method):
    # Call sites construct every host/path from fixed development scope. Redirects
    # are forbidden so a bearer token cannot reach another endpoint.
    need(host in {"iam.googleapis.com", "cloudresourcemanager.googleapis.com", "serviceusage.googleapis.com"}
         and method in {"GET", "POST"})
    request = urllib.request.Request("https://" + host + "/" + path,
        data=None if body is None else json.dumps(body).encode(),
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"}, method=method)
    try:
        with urllib.request.build_opener(NoRedirect()).open(request, timeout=30) as response:
            if response.status != 200:
                return response.status, {}
            try:
                return response.status, response_json(response)
            except RuntimeAdmission:
                raise
            except Exception:
                # The HTTP success was actually observed before parsing failed.
                raise ProbeFailure("RESPONSE_REJECTED") from None
    except urllib.error.HTTPError as error:
        # HTTP status alone is public-safe. Never consume or emit error bodies.
        error.close()
        return error.code, {}


def exact_response(request, path, token, body=None, *, host, method):
    try:
        status, value = request(path, token, body, host=host, method=method)
    except (ProbeFailure, RuntimeAdmission):
        raise
    except Exception:
        # No HTTP status was established here. Never label an unknown exception
        # as an observed denial or a rejected HTTP200 response.
        raise ProbeFailure("TRANSPORT_FAILED") from None
    if type(status) is not int or not 100 <= status <= 599:
        raise ProbeFailure("TRANSPORT_FAILED")
    if status != 200:
        raise ProbeFailure("HTTP403" if status == 403 else "HTTP_OTHER")
    return value


def identity(env):
    context(env)
    request = urllib.request.Request(env["ACTIONS_ID_TOKEN_REQUEST_URL"] + "&audience=sts.amazonaws.com",
        headers={"Authorization": "Bearer " + env["ACTIONS_ID_TOKEN_REQUEST_TOKEN"]})
    with urllib.request.build_opener(NoRedirect()).open(request, timeout=30) as response:
        need(response.status == 200)
        token = response_json(response, limit=128 * 1024)["value"]
    need(type(token) is str and len(token) <= 128 * 1024 and len(token.split(".")) == 3)
    encoded = token.split(".")[1]
    claims = load(base64.urlsafe_b64decode(encoded + "=" * (-len(encoded) % 4)))
    verify_oidc_claims(claims, env, expected_identity=env["RUNTIME_READ_IDENTITY"])
    need(claims.get("run_attempt") == "1")


def shape(value, allowed, required=()):
    need(type(value) is dict and set(required) <= set(value) <= set(allowed))


def strings(value):
    need(type(value) is dict and all(type(k) is str and type(v) is str for k, v in value.items()))


def validate_pool(value, number, *, provider=False):
    base = {"name", "state", "displayName", "description", "disabled"}
    extra = {"aws", "attributeMapping", "attributeCondition"} if provider else {"mode"}
    required = {"name", "state"} | ({"aws", "attributeMapping", "attributeCondition"} if provider else set())
    shape(value, base | extra, required)
    pool_name = f"projects/{number}/locations/global/workloadIdentityPools/aws-runtime"
    need(value["name"] == pool_name + ("/providers/aws-provider" if provider else "")
         and value["state"] == "ACTIVE" and value.get("disabled", False) is False)
    for key in ("name", "state", "displayName", "description"):
        need(key not in value or type(value[key]) is str)
    need("disabled" not in value or type(value["disabled"]) is bool)
    if provider:
        shape(value["aws"], {"accountId"}, {"accountId"})
        need(type(value["aws"]["accountId"]) is str and re.fullmatch(r"[0-9]{12}", value["aws"]["accountId"]))
        strings(value["attributeMapping"])
        for key in ("google.subject", "attribute.aws_role", "attribute.account"):
            ownership.text(value["attributeMapping"][key])
        ownership.text(value["attributeCondition"])
    else:
        need(value.get("mode") in (None, "", "FEDERATION_ONLY"))


def validate_project(project, number):
    shape(project, {"projectId", "projectNumber", "lifecycleState", "name", "createTime", "labels", "parent"},
          {"projectId", "projectNumber", "lifecycleState"})
    if project["projectId"] != PROJECT or project["projectNumber"] != number:
        raise RuntimeAdmission("runtime metadata read STOP")
    need(project["lifecycleState"] == "ACTIVE")
    for key in ("projectId", "projectNumber", "lifecycleState", "name", "createTime"):
        need(key not in project or type(project[key]) is str)
    if "labels" in project:
        strings(project["labels"])
    if "parent" in project:
        shape(project["parent"], {"type", "id"}, {"type", "id"})
        strings(project["parent"])


def validate_policy(policy):
    shape(policy, {"version", "bindings", "etag", "auditConfigs"}, {"bindings"})
    need(type(policy.get("version", 1)) is int and policy.get("version", 1) in {1, 3})
    need(type(policy["bindings"]) is list and ("etag" not in policy or type(policy["etag"]) is str))
    for binding in policy["bindings"]:
        shape(binding, {"role", "members", "condition"}, {"role", "members"})
        need(type(binding["role"]) is str and bool(binding["role"]) and type(binding["members"]) is list
             and all(type(m) is str and bool(m) for m in binding["members"])
             and len(set(binding["members"])) == len(binding["members"]))
        if "condition" in binding:
            shape(binding["condition"], {"title", "description", "expression"}, {"title", "expression"})
            strings(binding["condition"])
            need(policy.get("version") == 3 and bool(binding["condition"]["expression"]) and bool(binding["condition"]["title"]))
    if "auditConfigs" in policy:
        need(type(policy["auditConfigs"]) is list)
        for config in policy["auditConfigs"]:
            shape(config, {"service", "auditLogConfigs"}, {"service", "auditLogConfigs"})
            need(type(config["service"]) is str and type(config["auditLogConfigs"]) is list)
            for log in config["auditLogConfigs"]:
                shape(log, {"logType", "exemptedMembers"}, {"logType"})
                need(log["logType"] in {"ADMIN_READ", "DATA_WRITE", "DATA_READ"})
                need(type(log.get("exemptedMembers", [])) is list
                     and all(type(m) is str for m in log.get("exemptedMembers", [])))


def validate_service(service, number):
    shape(service, {"name", "parent", "config", "state"}, {"name", "config", "state"})
    need("parent" not in service or service["parent"] == "projects/" + number)
    # Service Usage ServiceConfig has these documented fields. Preserve every
    # returned byte-valued JSON field in comparisons, including nested config.
    shape(service["config"], {"name", "title", "apis", "documentation", "quota", "authentication", "usage", "endpoints", "monitoredResources", "monitoring"}, {"name"})
    name = service["config"]["name"]
    need(type(name) is str and re.fullmatch(r"[a-z][a-z0-9-]*\.googleapis\.com", name))
    need(service["name"] == f"projects/{number}/services/{name}" and service["state"] == "ENABLED")
    return name


def validate(snapshot, number):
    """Reuse the existing offline runtime shape/completed inventory contract."""
    enabled = snapshot["enabled_services"]
    shape(enabled, {"services"}, {"services"})
    need(type(enabled["services"]) is list and 0 < len(enabled["services"]) <= MAX_SERVICES)
    classification = {}
    for service in enabled["services"]:
        name = validate_service(service, number)
        need(name not in classification)
        classification[name] = {"classification": "Investigate", "dependency_addresses": [], "reason": "Observation only"}
    # Empty grant selection and Investigate are internal validation inputs only;
    # no ownership candidate, grant selection, inventory file or receipt is made.
    ownership.prepare(snapshot | {"runtime_grants": {}, "api_classification": classification})


def observe(env, number, results, reasons, *, request):
    total = 0
    def get(host, path, validate_item=None, *, method="GET"):
        nonlocal total
        read_window(env)
        value = exact_response(request, path, env["GCP_SMOKE_ACCESS_TOKEN"], host=host, method=method)
        need(type(value) is dict)
        size = len(json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode())
        if not 0 < size <= MAX_BYTES or total + size > MAX_BYTES:
            raise RuntimeLimit("runtime metadata read STOP")
        total += size
        if validate_item is not None:
            validate_item(value)
        return value

    def probe(alias, operation):
        # Admission failures do not become evidence of a denied endpoint. Once
        # admitted, one failed interface cannot fabricate another interface's
        # response or suppress its independent success/failure observation.
        read_window(env)
        try:
            value = operation()
        except Exception as error:
            results[alias] = "STOP"
            if isinstance(error, RuntimeAdmission):
                reasons[alias] = "ADMISSION_STOP"
                raise
            reasons[alias] = error.reason if isinstance(error, ProbeFailure) else "RESPONSE_REJECTED"
            return None
        reasons[alias] = "PASS"
        results[alias] = "PASS"
        return value

    # Establish the fixed development project number before any number-scoped
    # IAM request, including when the existing provider secret is misconfigured.
    project = probe("runtime-project", lambda: get("cloudresourcemanager.googleapis.com", "v1/projects/" + PROJECT,
                    lambda value: validate_project(value, number)))
    if project is None:
        return None
    pool_name = f"projects/{number}/locations/global/workloadIdentityPools/aws-runtime"
    provider_name = pool_name + "/providers/aws-provider"
    sa_email = PROJECT + "@appspot.gserviceaccount.com"
    sa_name = f"projects/{PROJECT}/serviceAccounts/{sa_email}"
    snapshot = {
        "project": project,
        "iam_policy_resource": sa_name,
        "runtime_config": {"audience": "//iam.googleapis.com/" + provider_name, "service_account_email": sa_email},
    }
    operations = (
        ("runtime-pool", "pool", lambda: get("iam.googleapis.com", "v1/" + pool_name, lambda value: validate_pool(value, number))),
        ("runtime-attestation", "pool_attestation_rules", lambda: get("iam.googleapis.com", "v1/" + pool_name + ":listAttestationRules", ownership.require_empty_attestation_response)),
        ("runtime-provider", "provider", lambda: get("iam.googleapis.com", "v1/" + provider_name, lambda value: validate_pool(value, number, provider=True))),
        ("runtime-sa-policy", "iam_policy", lambda: get("iam.googleapis.com", "v1/" + sa_name + ":getIamPolicy?options.requestedPolicyVersion=3", validate_policy, method="POST")),
    )
    for alias, key, operation in operations:
        value = probe(alias, operation)
        if value is not None:
            snapshot[key] = value

    def enabled_services():
        services, tokens, token, names = [], set(), "", set()
        for _ in range(MAX_PAGES):
            path = f"v1/projects/{number}/services?filter=state:ENABLED&pageSize=200"
            if token:
                path += "&pageToken=" + urllib.parse.quote(token, safe="")
            page = get("serviceusage.googleapis.com", path)
            shape(page, {"services", "nextPageToken"})
            items, next_token = page.get("services", []), page.get("nextPageToken", "")
            need(type(items) is list and len(items) <= 200 and type(next_token) is str and len(next_token) <= MAX_TOKEN)
            for item in items:
                name = validate_service(item, number)
                need(name not in names)
                names.add(name)
            services.extend(items)
            need(len(services) <= MAX_SERVICES)
            if not next_token:
                break
            need(next_token not in tokens)
            tokens.add(next_token)
            token = next_token
        else:
            need(False)
        need(bool(services))
        return {"services": services}

    enabled = probe("runtime-enabled-services", enabled_services)
    if enabled is not None:
        snapshot["enabled_services"] = enabled
    if all(results[alias] == "PASS" for alias in ALIASES[:6]):
        validate(snapshot, number)
    return snapshot


def permissions(env, *, request):
    read_window(env)
    value = exact_response(request, "v3/projects/" + PROJECT + ":testIamPermissions", env["GCP_SMOKE_ACCESS_TOKEN"],
        {"permissions": list(MUTATIONS)}, host="cloudresourcemanager.googleapis.com", method="POST")
    shape(value, {"permissions"})
    granted = value.get("permissions", [])
    need(type(granted) is list and all(type(item) is str and item in MUTATIONS for item in granted)
         and len(granted) == len(set(granted)))
    if granted:
        raise ProbeFailure("BOUNDARY_REJECTED")


def summary(env, checks, reasons=None):
    need(set(checks) == set(ALIASES) and all(value in {"PASS", "STOP", "未実行"} for value in checks.values()))
    lines = ["runtime-metadata-read: " + ("PASS" if all(value == "PASS" for value in checks.values()) else "STOP")]
    lines += [alias + ": " + checks[alias] for alias in ALIASES]
    if reasons is not None:
        need(set(reasons) == set(REASON_KEYS) and all(value in REASONS for value in reasons.values()))
        lines += [key + ": " + reasons[key] for key in REASON_KEYS]
    try:
        path = Path(env["GITHUB_STEP_SUMMARY"])
        need(path.is_absolute() and not path.is_symlink())
        fd = os.open(path, os.O_WRONLY | os.O_APPEND | os.O_CREAT | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            info = os.fstat(handle.fileno())
            need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
            handle.write("\n".join(lines) + "\n")
    except Exception:
        lines[0] = "runtime-metadata-read: STOP"
        raise
    finally:
        # Both public sinks receive only the validated fixed projection. Keep
        # per-interface observations available when Step Summary is unavailable.
        print("\n".join(lines))


def read(env, *, request=None):
    number = context(env, token=True)
    checks = dict.fromkeys(ALIASES, "未実行")
    observations = {phase: dict.fromkeys(ALIASES[:6], "未実行") for phase in ("before", "after")}
    reasons = {phase: dict.fromkeys(ALIASES[:6], "未実行") for phase in ("before", "after")}
    permission_reason = "未実行"
    snapshots = {}
    checks["runtime-window"] = "STOP"
    try:
        for phase in ("before", "after"):
            snapshots[phase] = observe(env, number, observations[phase], reasons[phase], request=request or google)
            if observations[phase]["runtime-project"] != "PASS":
                break
        for alias in ALIASES[:6]:
            states = [observations[phase][alias] for phase in ("before", "after")]
            checks[alias] = "STOP" if "STOP" in states else ("PASS" if states == ["PASS", "PASS"] else "未実行")
        complete = all(checks[alias] == "PASS" for alias in ALIASES[:6])
        if complete:
            checks["runtime-stable"] = "PASS" if snapshots["before"] == snapshots["after"] else "STOP"
        if observations["after"]["runtime-project"] == "PASS":
            read_window(env)
            checks["runtime-project-permissions"] = "STOP"
            try:
                permissions(env, request=request or google)
            except RuntimeAdmission:
                permission_reason = "ADMISSION_STOP"
                raise
            except Exception as error:
                permission_reason = error.reason if isinstance(error, ProbeFailure) else "RESPONSE_REJECTED"
                raise
            permission_reason = "PASS"
            checks["runtime-project-permissions"] = "PASS"
        window(env)
        checks["runtime-window"] = "PASS"
    except Exception:
        # Preserve completed individual observations when a later window/read
        # fails. Missing observations remain explicit; no response is invented.
        for alias in ALIASES[:6]:
            states = [observations[phase][alias] for phase in ("before", "after")]
            checks[alias] = "STOP" if "STOP" in states else ("PASS" if states == ["PASS", "PASS"] else "未実行")
    finally:
        # No private snapshots, token files, candidates or receipts reach disk.
        snapshots.clear()
        checks["runtime-cleanup"] = "PASS"
        public_reasons = {alias + "-" + phase + "-reason": reasons[phase][alias]
                          for phase in ("before", "after") for alias in ALIASES[:6]}
        public_reasons["runtime-project-permissions-reason"] = permission_reason
        summary(env, checks, public_reasons)
    need(all(value == "PASS" for value in checks.values()))


def main(argv=None):
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument("--phase", required=True, choices=("preflight", "identity", "read", "cleanup"))
        args = parser.parse_args(argv)
        env = dict(os.environ)
        if args.phase == "read":
            read(env)
        elif args.phase == "identity":
            identity(env)
        else:
            context(env, gate=args.phase != "cleanup")
    except Exception:
        print("runtime-metadata-read: STOP", file=sys.stderr)
        return 3
    print("runtime-metadata-read: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
