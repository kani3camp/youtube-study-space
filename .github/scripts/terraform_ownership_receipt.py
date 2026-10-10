#!/usr/bin/env python3
"""Source-pinned GitHub execution receipts. No cloud credential or mutation.

The empty reviewed issuer catalog deliberately authorizes no existing run.
Public receipts contain commitments, never private scope/state/identity values.
"""
from __future__ import annotations

from datetime import datetime
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import sys
from urllib.parse import urlparse
from urllib.request import Request, build_opener, HTTPRedirectHandler

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "infra/gcp/scripts"))
from terraform_history_table_metadata import stable_table_metadata
from prepare_user_activity_history_adoption import private_json, unique_object

REPOSITORY = "kani3camp/youtube-study-space"
REPOSITORY_ID, OWNER_ID = 340900071, 54093651
BRANCH = "feature/gcp-terraform-iac"
CALLER = ".github/workflows/ci.yml"
REUSABLE = ".github/workflows/gcp-terraform-authenticated.yml"
COMPANION = ".github/workflows/gcp-user-activity-schema-audit.yml"
WORKFLOW_REF = f"{REPOSITORY}/{CALLER}@refs/heads/{BRANCH}"
MARKER = "OWNERSHIP_RECEIPT_V1 "
RECORD = "history-ownership-receipt.json"
CHECKS = {"full_root_noop", "exact_state_delta", "metadata_unchanged", "lock_absent", "exact_guards"}
KEYS = {"schema_version", "repository_id", "source_sha", "run_id", "run_attempt", "job_role", "environment",
        "target", "root_contract", "operation", "wave", "phase", "prior_receipt_sha256",
        "prior_scope_commitment", "scope_commitment", "prior_state_commitment", "state_after_commitment", "resource_count", "import_before_count",
        "import_post_count", "checks", "accounting"}
MINIMUM_STEPS = {
    "plan": ["Checkout trusted commit", "Create saved plan without public output",
             "Sanitize and enforce the selected protected contract", "Cleanup sensitive temporary files"],
    "apply": ["Checkout the exact planned commit", "Assert plan/apply commit identity",
              "Apply the locally re-created saved plan", "Require post-apply no-op",
              "Emit verified ownership receipt", "Cleanup sensitive temporary files"],
}
HISTORY_STEPS = ["Check private history receipt binding before credentials", "Check fresh history receipt metadata binding", "Capture exact pre-import state and approved history plan", "Re-plan at the exact approved commit",
                 "Verify re-plan matches the sanitized approved projection", "Seal the one-shot history saved plan before apply",
                 "Verify post-import canonical metadata privately", "Verify one-shot history state and table receipt"]
RUNTIME_STEPS = {
    "plan": ["Stage reviewed private cumulative ownership inputs before credentials",
             "Verify authentic prior ownership receipts before cloud credentials",
             "Verify development identity boundaries without public identifiers", "Read canonical history metadata privately",
             "Bind fresh runtime state to authenticated predecessor", "Initialize remote backend without public output",
             "Recheck exact ownership receipts before protected validation",
             "Verify runtime plan safety and unchanged fresh state"],
    "apply": ["Stage reviewed private cumulative ownership inputs before credentials",
              "Verify authentic prior ownership receipts before cloud credentials",
              "Verify Function identity exact read permissions", "Re-read canonical history metadata privately",
              "Bind fresh runtime state to authenticated predecessor", "Initialize remote backend without public output",
              "Re-plan at the exact approved commit",
              "Recheck exact ownership receipts before protected validation", "Verify re-plan matches the sanitized approved projection",
              "Seal the runtime saved plan and fresh private inputs", "Verify post-import canonical metadata privately",
              "Verify runtime state-only delta and full-root post no-op"],
}
HISTORY_IDENTITY_STEP = "Verify exact AWS execution identity before GCP authentication"
REFERENCE_KEYS = {"run_id", "attempt", "job_id", "source_sha", "receipt_sha256"}


def need(value):
    if not value:
        raise ValueError("ownership receipt STOP")


def hex_value(value, length):
    return type(value) is str and re.fullmatch(r"[0-9a-f]{" + str(length) + "}", value) is not None


def positive(value):
    return type(value) is int and value > 0


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False).encode()


def decode(raw):
    need(type(raw) in {str, bytes} and len(raw) <= 1024 * 1024)
    def invalid(_):
        raise ValueError("ownership receipt STOP")
    return json.loads(raw, object_pairs_hook=unique_object, parse_constant=invalid)


def digest(value):
    return hashlib.sha256(canonical(value)).hexdigest()


def binding(value):
    need(type(value) is dict and set(value) == {"nonce", "backend", "history_metadata"})
    need(hex_value(value["nonce"], 64) and len(set(value["nonce"])) > 1)
    backend = value["backend"]
    need(type(backend) is dict and set(backend) == {"account_id", "bucket", "key", "region", "workspace"})
    need(type(backend["account_id"]) is str and re.fullmatch(r"\d{12}", backend["account_id"]))
    need(type(backend["bucket"]) is str and re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", backend["bucket"]))
    need(backend["key"] == "youtube-study-space/dev/terraform.tfstate"
         and backend["region"] == "ap-northeast-1" and backend["workspace"] == "default")
    stable_table_metadata(value["history_metadata"])
    return value


def history_scope(value):
    binding(value)
    return {"contract": "dev-history12-v1", "backend": value["backend"],
            "history": stable_table_metadata(value["history_metadata"])}


def check_current_binding(value, env, metadata=None):
    binding(value)
    need(value["backend"] == dict(account_id=env.get("STATE_ACCOUNT_ID"), bucket=env.get("STATE_BUCKET"),
                                  key=env.get("STATE_KEY"), region=env.get("STATE_AWS_REGION"),
                                  workspace=env.get("TF_WORKSPACE")))
    if metadata is not None:
        need(stable_table_metadata(value["history_metadata"]) == stable_table_metadata(metadata))


def commitment(value, scope):
    binding(value)
    return hmac.new(bytes.fromhex(value["nonce"]), b"ownership-scope-v1\0" + canonical(scope), hashlib.sha256).hexdigest()


def state_commitment(value, state):
    binding(value)
    return hmac.new(bytes.fromhex(value["nonce"]), b"ownership-state-v1\0" + canonical(state), hashlib.sha256).hexdigest()


def reference(value):
    need(type(value) is dict and set(value) == REFERENCE_KEYS)
    need(all(positive(value[k]) for k in ("run_id", "attempt", "job_id")) and value["attempt"] == 1)
    need(hex_value(value["source_sha"], 40) and hex_value(value["receipt_sha256"], 64))
    return value


def validate_receipt(value):
    need(type(value) is dict and set(value) == KEYS and type(value["schema_version"]) is int
         and value["schema_version"] == 1 and type(value["repository_id"]) is int
         and value["repository_id"] == REPOSITORY_ID)
    need(hex_value(value["source_sha"], 40) and positive(value["run_id"])
         and type(value["run_attempt"]) is int and value["run_attempt"] == 1)
    need(value["job_role"] == "apply" and value["environment"] == "terraform-dev-apply"
         and value["target"] == "dev" and value["root_contract"] == "gcp-dev-default"
         and value["operation"] == "apply" and value["phase"] == "post"
         and value["wave"] in {"history12", "pool", "provider", "grant", "api", "noop"})
    need(hex_value(value["scope_commitment"], 64) and hex_value(value["state_after_commitment"], 64))
    for key in ("prior_receipt_sha256", "prior_scope_commitment", "prior_state_commitment"):
        need(value[key] is None if value["wave"] == "history12" else hex_value(value[key], 64))
    need(positive(value["resource_count"]) and value["resource_count"] >= 12
         and type(value["import_before_count"]) is int
         and value["import_before_count"] == int(value["wave"] != "noop")
         and type(value["import_post_count"]) is int and value["import_post_count"] == 0)
    need(type(value["checks"]) is dict and set(value["checks"]) == CHECKS
         and all(v is True for v in value["checks"].values()))
    accounting = value["accounting"]
    need(type(accounting) is dict and set(accounting) == {"terraform_outcomes", "post_state_reads",
         "post_metadata_reads", "provider_requests", "actual_cost"})
    need(accounting["terraform_outcomes"] == {"apply": "success", "post_plan": "success"}
         and accounting["provider_requests"] == "unknown" and accounting["actual_cost"] == "unknown")
    need(all(positive(accounting[k]) and accounting[k] <= 1000 for k in ("post_state_reads", "post_metadata_reads")))
    if value["wave"] == "history12":
        need(value["resource_count"] == 12 and accounting["post_state_reads"] == 4 and accounting["post_metadata_reads"] == 1)
    need(len(canonical(value)) <= 16384)
    return value


def context(env):
    need(env.get("GITHUB_REPOSITORY") == REPOSITORY and env.get("GITHUB_REPOSITORY_ID") == str(REPOSITORY_ID)
         and env.get("GITHUB_REPOSITORY_OWNER_ID") == str(OWNER_ID)
         and env.get("GITHUB_EVENT_NAME") == "workflow_dispatch" and env.get("GITHUB_WORKFLOW_REF") == WORKFLOW_REF
         and env.get("GITHUB_REF") == "refs/heads/" + BRANCH and hex_value(env.get("GITHUB_SHA"), 40)
         and env.get("GITHUB_RUN_ATTEMPT") == "1" and type(env.get("GITHUB_RUN_ID")) is str
         and re.fullmatch(r"[1-9]\d*", env["GITHUB_RUN_ID"]))


def load_catalog(value):
    need(type(value) is dict and set(value) == {"schema_version", "issuers"}
         and type(value["schema_version"]) is int and value["schema_version"] == 1 and type(value["issuers"]) is dict)
    for sha, item in value["issuers"].items():
        need(hex_value(sha, 40) and type(item) is dict and set(item) == {
            "workflow_id", "job_names", "environments", "reviewer_ids", "receipt_step", "required_steps", "waves"})
        need(type(item["waves"]) is list and item["waves"] and all(type(w) is str for w in item["waves"]) and len(item["waves"]) == len(set(item["waves"]))
             and set(item["waves"]) <= {"history12", "pool", "provider", "grant", "api", "noop"})
        need(positive(item["workflow_id"]) and type(item["job_names"]) is dict and set(item["job_names"]) == {"plan", "apply"}
             and all(type(v) is str and v for v in item["job_names"].values())
             and len(set(item["job_names"].values())) == 2)
        need(type(item["environments"]) is dict and set(item["environments"]) == {"plan", "apply"})
        for role, e in item["environments"].items():
            need(type(e) is dict and set(e) == {"id", "name"} and positive(e["id"])
                 and e["name"] == "terraform-dev-" + role)
        need(len({e["id"] for e in item["environments"].values()}) == 2)
        need(type(item["reviewer_ids"]) is list and item["reviewer_ids"] and all(positive(i) for i in item["reviewer_ids"])
             and len(item["reviewer_ids"]) == len(set(item["reviewer_ids"])))
        need(item["receipt_step"] == "Emit verified ownership receipt"
             and type(item["required_steps"]) is dict and set(item["required_steps"]) == {"plan", "apply"})
        for role, names in item["required_steps"].items():
            need(type(names) is list and names and all(type(n) is str and n for n in names)
                 and len(names) == len(set(names)) and set(MINIMUM_STEPS[role]) <= set(names))
        need(item["receipt_step"] in item["required_steps"]["apply"])
        if "history12" in item["waves"]:
            need(set(HISTORY_STEPS) <= set(item["required_steps"]["apply"])
                 and set(HISTORY_STEPS[:2]) <= set(item["required_steps"]["plan"]))
            need(all(HISTORY_IDENTITY_STEP in item["required_steps"][role] for role in ("plan", "apply")))
        if set(item["waves"]) - {"history12"}:
            need(all(set(RUNTIME_STEPS[role]) <= set(item["required_steps"][role]) for role in ("plan", "apply")))
    return value


def timestamp(value):
    need(type(value) is str)
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    need(parsed.tzinfo is not None)
    return parsed


def check_run(run, ref, issuer):
    need(type(run) is dict and run.get("id") == ref["run_id"] and type(run.get("id")) is int
         and run.get("run_attempt") == 1 and type(run.get("run_attempt")) is int
         and run.get("status") == "completed" and run.get("conclusion") == "success"
         and run.get("event") == "workflow_dispatch" and run.get("head_sha") == ref["source_sha"]
         and run.get("head_branch") == BRANCH and run.get("path") in (CALLER, f"{CALLER}@{BRANCH}")
         and type(run.get("workflow_id")) is int and run.get("workflow_id") == issuer["workflow_id"])
    for key in ("repository", "head_repository"):
        repo = run.get(key)
        need(type(repo) is dict and type(repo.get("id")) is int and repo.get("id") == REPOSITORY_ID and repo.get("full_name") == REPOSITORY
             and type(repo.get("owner")) is dict and type(repo["owner"].get("id")) is int and repo["owner"].get("id") == OWNER_ID)
    # GitHub also reports @exact-SHA and the reviewed caller's schema-audit
    # companion, including when that job was skipped. Match every field and
    # each allowlisted path; never infer a ref or discard unknown references.
    workflows = run.get("referenced_workflows")
    need(type(workflows) is list and 1 <= len(workflows) <= 2)
    seen = set()
    for item in workflows:
        need(type(item) is dict and set(item) == {"path", "sha", "ref"} and type(item["path"]) is str
             and item["sha"] == ref["source_sha"] and item["ref"] == "refs/heads/" + BRANCH)
        matches = [workflow for workflow in (REUSABLE, COMPANION)
                   if item["path"] in {f"{REPOSITORY}/{workflow}@{suffix}" for suffix in
                                        (BRANCH, "refs/heads/" + BRANCH, ref["source_sha"])}]
        need(len(matches) == 1 and matches[0] not in seen)
        seen.add(matches[0])
    need(REUSABLE in seen)


def authenticate(ref, catalog, *, request):
    """GET exact run/attempt/jobs/reviews/step log, never operator-submitted logs."""
    reference(ref)
    issuer = load_catalog(catalog)["issuers"].get(ref["source_sha"])
    need(issuer is not None)
    base = f"/repos/{REPOSITORY}/actions/runs/{ref['run_id']}"
    initial = request(base)
    check_run(initial, ref, issuer)
    check_run(request(base + "/attempts/1"), ref, issuer)
    jobs, total = [], None
    for page in range(1, 11):
        response = request(base + f"/attempts/1/jobs?per_page=100&page={page}")
        need(type(response) is dict and set(response) == {"total_count", "jobs"}
             and type(response["total_count"]) is int and 0 < response["total_count"] <= 1000
             and type(response["jobs"]) is list and 0 < len(response["jobs"]) <= 100)
        if total is None:
            total = response["total_count"]
        need(total == response["total_count"])
        jobs.extend(response["jobs"])
        need(len(jobs) <= total)
        if len(jobs) == total:
            break
        need(len(response["jobs"]) == 100)
    need(len(jobs) == total and all(type(j) is dict and positive(j.get("id")) and positive(j.get("run_id"))
             and j["run_id"] == ref["run_id"] and j.get("head_sha") == ref["source_sha"] for j in jobs)
         and len({j["id"] for j in jobs}) == len(jobs))
    emitted_jobs = [j for j in jobs if j.get("name") == issuer["job_names"]["apply"]]
    need(len(emitted_jobs) == 1)
    job = emitted_jobs[0]
    need(job.get("id") == ref["job_id"])
    for role in ('plan', 'apply'):
        selected_jobs = [j for j in jobs if j.get('name') == issuer['job_names'][role]]
        need(len(selected_jobs) == 1)
        positions = []
        for name in MINIMUM_STEPS[role]:
            matches = [step for step in selected_jobs[0].get('steps', []) if step.get('name') == name]
            need(len(matches) == 1 and positive(matches[0].get('number'))
                 and matches[0].get('status') == 'completed' and matches[0].get('conclusion') == 'success')
            positions.append(matches[0]['number'])
        need(positions == sorted(positions))
    emitted_steps = [step for step in job.get("steps", []) if step.get("name") == issuer["receipt_step"]]
    need(len(emitted_steps) == 1 and positive(emitted_steps[0].get("number"))
         and emitted_steps[0].get("status") == "completed" and emitted_steps[0].get("conclusion") == "success")
    step = emitted_steps[0]
    # Job steps are numbered from 1; selective step-log endpoint uses index 0.
    raw = request(f"/repos/{REPOSITORY}/actions/jobs/{job['id']}/steps/{step['number'] - 1}/logs", log=True)
    need(type(raw) is bytes and 0 < len(raw) <= 1024 * 1024 and raw.endswith(b"\n"))
    lines = [re.sub(r"^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d+Z ", "", line)
             for line in raw.decode("utf-8").splitlines()]
    markers = [line[len(MARKER):] for line in lines if line.startswith(MARKER)]
    need(len(markers) == 1 and len(markers[0]) <= 16384)
    value = validate_receipt(decode(markers[0]))
    need(value["wave"] in issuer["waves"] and digest(value) == ref["receipt_sha256"] and value["run_id"] == ref["run_id"]
         and value["source_sha"] == ref["source_sha"])
    protected = {}
    for role, name in issuer["job_names"].items():
        matches = [j for j in jobs if j.get("name") == name]
        need(len(matches) == 1)
        j = matches[0]
        need(positive(j.get("run_id")) and j.get("run_id") == ref["run_id"] and j.get("head_sha") == ref["source_sha"]
             and j.get("head_branch") == BRANCH and j.get("status") == "completed" and j.get("conclusion") == "success")
        steps = j.get("steps")
        need(type(steps) is list and all(type(s) is dict and positive(s.get("number")) for s in steps)
             and len({s["number"] for s in steps}) == len(steps))
        positions = []
        inactive = (set(RUNTIME_STEPS[role]) - set(HISTORY_STEPS)) if value["wave"] == "history12" else (
            set(HISTORY_STEPS if role == "apply" else HISTORY_STEPS[:2]) - set(RUNTIME_STEPS[role]))
        for name in [name for name in issuer["required_steps"][role] if name not in inactive]:
            matches = [s for s in steps if s.get("name") == name]
            need(len(matches) == 1 and matches[0].get("status") == "completed" and matches[0].get("conclusion") == "success")
            positions.append(matches[0]["number"])
        need(positions == sorted(positions) and timestamp(j["started_at"]) <= timestamp(j["completed_at"]))
        protected[role] = j
    job = protected["apply"]
    need(job["id"] == ref["job_id"] and timestamp(protected["plan"]["completed_at"]) <= timestamp(job["started_at"]))
    reviews = request(base + "/approvals")
    need(type(reviews) is list and reviews and len(reviews) <= 100)
    for e in issuer["environments"].values():
        matches = []
        for review in reviews:
            need(type(review) is dict and type(review.get("environments")) is list)
            for actual in review["environments"]:
                need(type(actual) is dict)
                if actual.get("id") == e["id"] or actual.get("name") == e["name"]:
                    need(positive(actual.get("id")) and actual.get("id") == e["id"] and actual.get("name") == e["name"])
                    matches.append(review)
        need(matches and all(r.get("state") == "approved" and type(r.get("user")) is dict
                             and positive(r["user"].get("id")) and r["user"].get("id") in issuer["reviewer_ids"] for r in matches))
    final = request(base)
    check_run(final, ref, issuer)
    need(final == initial)
    return value, {"started_at": job["started_at"], "completed_at": job["completed_at"]}


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class GitHubRead:
    """Bounded GET only; strip credentials before a selective log redirect."""
    def __init__(self, token):
        need(type(token) is str and token and "\n" not in token and "\r" not in token)
        self.token = token
        self.opener = build_opener(NoRedirect)

    def __call__(self, path, *, log=False):
        from urllib.error import HTTPError
        need(type(path) is str and path.startswith(f"/repos/{REPOSITORY}/actions/") and not any(c in path for c in "\r\n#"))
        headers = {"Authorization": "Bearer " + self.token, "Accept": "application/vnd.github+json",
                   "X-GitHub-Api-Version": "2026-03-10"}
        try:
            response = self.opener.open(Request("https://api.github.com" + path, headers=headers), timeout=15)
        except HTTPError as error:
            need(log and error.code == 302)
            url = error.headers.get("Location", "")
            parsed = urlparse(url)
            need(parsed.scheme == "https" and parsed.hostname is not None and parsed.port in {None, 443}
                 and parsed.username is None and parsed.password is None
                 and (parsed.hostname.endswith(".actions.githubusercontent.com") or parsed.hostname.endswith(".blob.core.windows.net")))
            response = self.opener.open(Request(url), timeout=15)
        with response:
            need(response.status == 200)
            raw = response.read(1024 * 1024 + 1)
        need(len(raw) <= 1024 * 1024)
        return raw if log else decode(raw)


def new_nonce(env):
    from terraform_history_plan_receipt import write_private
    root = Path(env["RUNNER_TEMP"])
    need(root.is_absolute() and root.is_dir() and not root.is_symlink()
         and not root.resolve().is_relative_to(Path(__file__).resolve().parents[2]))
    write_private(root / "ownership-receipt-nonce.json", {"nonce": os.urandom(32).hex()})


def history_binding(env, *, metadata=None):
    context(env)
    need(env.get("MODE") == "apply" and env.get("HISTORY_TARGET") == "dev"
         and env.get("TF_VAR_project_id") == "test-youtube-study-space"
         and env.get("TF_VAR_manage_user_activity_history") == "true"
         and env.get("DEV_HISTORY_RECEIPT_EMITTER_ENABLED") == "true" and env.get("GITHUB_JOB") in {"plan", "apply"})
    envelope = decode(env.get("RUNTIME_OWNERSHIP_PACKET_JSON", ""))
    need(type(envelope) is dict and set(envelope) == {"schema_version", "receipt_binding"}
         and type(envelope["schema_version"]) is int and envelope["schema_version"] == 1)
    value = envelope["receipt_binding"]
    check_current_binding(value, env, metadata)
    scoped = commitment(value, history_scope(value))
    if env["GITHUB_JOB"] == "apply":
        need(hex_value(env.get("EXPECTED_HISTORY_RECEIPT_SCOPE"), 64)
             and env["EXPECTED_HISTORY_RECEIPT_SCOPE"] == scoped)
    return value


def check_history_binding(env, *, fresh=False):
    import stat
    value = history_binding(env, metadata=private_json(str(Path(env["RUNNER_TEMP"]) / "user-history-before.json")) if fresh else None)
    if not fresh and env["GITHUB_JOB"] == "plan":
        fd = os.open(env["GITHUB_OUTPUT"], os.O_WRONLY | os.O_APPEND | os.O_NOFOLLOW)
        with os.fdopen(fd, "w", encoding="utf-8") as output:
            info = os.fstat(output.fileno())
            need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
            output.write("history_receipt_scope=" + commitment(value, history_scope(value)) + "\n")


def emit(env):
    context(env)
    need(env.get("MODE") == "apply" and env.get("HISTORY_TARGET") == "dev" and env.get("GITHUB_JOB") == "apply"
         and env.get("DEV_HISTORY_RECEIPT_EMITTER_ENABLED") == "true")
    value = validate_receipt(private_json(str(Path(env["RUNNER_TEMP"]) / RECORD)))
    need(value["source_sha"] == env["GITHUB_SHA"] and value["run_id"] == int(env["GITHUB_RUN_ID"])
         and value["wave"] == "history12")
    print(MARKER + canonical(value).decode())


def emit_runtime(env):
    from terraform_runtime_execution import context as runtime_context, require_fresh_inputs, check_seal, state_shape, state_delta, RECORD as runtime_record
    from terraform_history_plan_receipt import private_bytes
    root, packet, metadata = runtime_context(env)
    need(env.get("GITHUB_JOB") == "apply" and env.get("MODE") == "apply")
    require_fresh_inputs(env, packet)
    check_seal(env)
    value = validate_receipt(private_json(str(root / runtime_record)))
    old = state_shape(private_bytes(root / "runtime-state-before.json"), packet, metadata, env, selection='adopted')
    state = state_shape(private_bytes(root / "runtime-state-after.json"), packet, metadata, env, selection='selected')
    state_delta(old, state, packet)
    need(value["source_sha"] == env["GITHUB_SHA"] and value["run_id"] == int(env["GITHUB_RUN_ID"])
         and value["wave"] == packet["wave"] and value["state_after_commitment"] == state_commitment(packet["receipt_binding"], state))
    print(MARKER + canonical(value).decode())


def main():
    try:
        if sys.argv[1:] in (["check-history-binding"], ["check-history-binding", "--fresh"]):
            check_history_binding(dict(os.environ), fresh="--fresh" in sys.argv)
            print("Private history receipt binding checked; values suppressed.")
        elif sys.argv[1:] == ["new-nonce"]:
            new_nonce(dict(os.environ))
            print("Private receipt binding nonce created; value suppressed.")
        elif sys.argv[1:] == ["emit-runtime"]:
            emit_runtime(dict(os.environ))
        else:
            need(sys.argv[1:] == ["emit-history"])
            emit(dict(os.environ))
    except Exception:
        print("STOP: ownership receipt rejected; diagnostic suppressed.", file=sys.stderr)
        return 3
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
