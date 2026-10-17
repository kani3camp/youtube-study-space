#!/usr/bin/env python3
"""Protected development metadata reads; no Terraform, writes or attestations.

This observes two bounded State/metadata snapshots through existing credentials.
Absence before and after does not establish writer quiescence, a full-root no-op,
historical import evidence or an ownership/admission receipt.
"""
from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone
import os
from pathlib import Path
import re
import stat
import sys

import terraform_history_plan_receipt as private
import terraform_history_post_noop_receipt as adopted
from terraform_history_table_metadata import stable_table_metadata
from terraform_identity_smoke import google, verify_aws_identity, verify_oidc
from prepare_user_activity_history_workflow import PrivateArgumentParser

ROOT = Path(__file__).resolve().parents[2]
MAX_BYTES = 4 * 1024 * 1024
PROJECT = "test-youtube-study-space"
PROJECT_PATH = "v1/projects/" + PROJECT
FILES = tuple("current-state-read-" + kind + "-" + phase + ".json"
              for phase in ("before", "after") for kind in ("state", "table", "project"))
LABELS = (
    "Exact twelve-resource State and canonical history owner",
    "Canonical eight-column table metadata",
    "Development project metadata",
    "State bytes and storage metadata unchanged",
    "Stable table and project metadata unchanged",
    "Native lock absent before and after",
    "Workspace absent before and after",
    "Private temporary files removed",
)


def need(value):
    if not value:
        raise ValueError("current-state read STOP")


def execution_window(env, *, now=None):
    def parse(key):
        value = env.get(key)
        need(type(value) is str and re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ", value) is not None)
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    issued = parse("DEV_CURRENT_STATE_READ_ISSUED_UTC")
    expires = parse("DEV_CURRENT_STATE_READ_EXPIRES_UTC")
    now = now or datetime.now(timezone.utc)
    need(issued <= now < expires <= issued + timedelta(hours=8))


def context(env, *, gate=True, token=False):
    need(env.get("MODE") == "current-state-read" and env.get("HISTORY_TARGET") == "dev"
         and env.get("TF_VAR_project_id") == PROJECT and env.get("TF_VAR_manage_user_activity_history") == "true"
         and env.get("TF_WORKSPACE") == "default"
         and env.get("GCP_SMOKE_SERVICE_ACCOUNT") == f"terraform-dev-plan@{PROJECT}.iam.gserviceaccount.com")
    need(not gate or env.get("DEV_CURRENT_STATE_READ_ENABLED") == "true")
    if gate:
        execution_window(env)
    expected = {
        "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_REPOSITORY": "kani3camp/youtube-study-space",
        "GITHUB_REPOSITORY_ID": "340900071", "GITHUB_REPOSITORY_OWNER_ID": "54093651",
        "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac", "GITHUB_RUN_ATTEMPT": "1",
        "GITHUB_JOB": "current_state_read",
        "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
    }
    need(all(env.get(key) == value for key, value in expected.items())
         and re.fullmatch(r"[0-9a-f]{40}", env.get("GITHUB_SHA", "")) is not None)
    need(env.get("STATE_KEY") == private.STATE_KEY and env.get("STATE_AWS_REGION") == "ap-northeast-1"
         and re.fullmatch(r"\d{12}", env.get("STATE_ACCOUNT_ID", "")) is not None
         and re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", env.get("STATE_BUCKET", "")) is not None
         and env.get("AWS_MAX_ATTEMPTS") == "1" and env.get("AWS_RETRY_MODE") == "standard")
    # Read mode never consumes or fabricates the separate plan/issuer admission.
    need(all(not env.get(key, "") for key in ("PLAN_SAFETY_EVIDENCE", "PLAN_COST_EVIDENCE", "HISTORY_IMPORT_APPROVAL"))
         and env.get("OWNERSHIP_WAVE", "none") == "none"
         and env.get("HISTORY_POST_NOOP", "false") == "false"
         and env.get("CURRENT_STATE_ATTESTATION", "false") == "false"
         and env.get("OWNERSHIP_ROOT_BASIS", "original-history-import") == "original-history-import")
    directory = Path(env["RUNNER_TEMP"])
    need(directory.is_absolute() and directory.is_dir() and not directory.is_symlink()
         and directory.stat().st_uid == os.geteuid() and not directory.resolve().is_relative_to(ROOT))
    need(not token or bool(env.get("GCP_SMOKE_ACCESS_TOKEN")))
    return directory


def metadata(env, root, phase, *, request):
    def get(host, path):
        status, value = request(path, env["GCP_SMOKE_ACCESS_TOKEN"], host=host, method="GET")
        need(status == 200 and type(value) is dict)
        raw = json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()
        need(0 < len(raw) <= MAX_BYTES)
        return value
    table = stable = project = None
    try:
        observed = get("bigquery.googleapis.com", adopted.TABLE_PATH)
        compared = stable_table_metadata(observed)  # Exact canonical-eight/schema contract.
        private.write_private(root / ("current-state-read-table-" + phase + ".json"), observed)
        table, stable = observed, compared
    except Exception:
        pass
    try:
        observed = get("cloudresourcemanager.googleapis.com", PROJECT_PATH)
        need(observed.get("projectId") == PROJECT
             and type(observed.get("projectNumber")) is str
             and re.fullmatch(r"\d+", observed["projectNumber"]) is not None
             and observed.get("lifecycleState") == "ACTIVE")
        private.write_private(root / ("current-state-read-project-" + phase + ".json"), observed)
        project = observed
    except Exception:
        pass
    return table, stable, project


def snapshot(env, root, phase, table, *, request):
    # The low-level helper does not invoke its no-op/admission context. It checks
    # HEAD size before GET, version/ETag/body/HEAD equality and exact adopted12
    # State shape plus the canonical table identity/schema/deletion protection.
    head, raw = adopted.snapshot(env, root, "current-state-read-state-" + phase + ".json",
                                 table, MAX_BYTES, request=request, check_absence=False)
    need(private.state_shape(raw, imported=True)["terraform_version"] == "1.16.4")
    return head, raw


def cleanup(root):
    for name in FILES:
        path = root / name
        try:
            info = path.lstat()
        except FileNotFoundError:
            continue
        need(info.st_uid == os.geteuid() and (stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode)))
        path.unlink()


def identity(env):
    context(env)
    # The existing exact OIDC checker performs no wrong-Environment assume-role
    # probe in this mode. STS verifies the configured account and immutable RoleId.
    verify_oidc(env, plan_read_only=True)
    verify_aws_identity(env, require_role_id=True)


def summary(env, checks):
    # Only fixed labels, fixed counts after complete success and PASS/STOP.
    passed = all(checks.values())
    lines = ["Current-state metadata read: " + ("PASS" if passed else "STOP")]
    lines.extend("- " + label + ": " + ("PASS" if checks[label] else "STOP") for label in LABELS)
    lines.extend(("- Managed resources: " + ("12" if passed else "STOP"),
                  "- Canonical columns: " + ("8" if passed else "STOP"),
                  "- Terraform executions: 0", "- State writes: 0", "- Native lock creations: 0",
                  "- Artifacts published: 0"))
    fd = os.open(env["GITHUB_STEP_SUMMARY"], os.O_WRONLY | os.O_APPEND | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        info = os.fstat(handle.fileno())
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
        handle.write("\n".join(lines) + "\n")


def read(env, *, request=None, metadata_request=None):
    root = context(env, token=True)
    request, metadata_request = request or private.aws, metadata_request or google
    # Refuse pre-existing/symlink staging paths instead of clobbering them.
    need(all(not (root / name).exists() and not (root / name).is_symlink() for name in FILES))
    checks = dict.fromkeys(LABELS, False)
    states, tables, projects = {}, {}, {}
    absences = {"lock": [], "workspace": []}
    try:
        for phase in ("before", "after"):
            try:
                table, stable, project = metadata(env, root, phase, request=metadata_request)
                if stable is not None:
                    tables[phase] = stable
                if project is not None:
                    projects[phase] = project
                checks[LABELS[1]] = len(tables) == 2
                checks[LABELS[2]] = len(projects) == 2
                try:
                    need(table is not None)
                    states[phase] = snapshot(env, root, phase, table, request=request)
                    checks[LABELS[0]] = len(states) == 2
                except Exception:
                    pass
            except Exception:
                pass
            # Each absence read is attempted independently, including after a
            # failed/oversized State or malformed metadata response.
            for name, prefix in (("lock", private.STATE_KEY + ".tflock"),
                                 ("workspace", private.STATE_KEY.rsplit("/", 1)[0] + "/workspaces/")):
                try:
                    private.absent(env, prefix, request=request)
                    absences[name].append(True)
                except Exception:
                    absences[name].append(False)
        checks[LABELS[3]] = len(states) == 2 and states["before"] == states["after"]
        checks[LABELS[4]] = (len(tables) == len(projects) == 2 and tables["before"] == tables["after"]
                             and projects["before"] == projects["after"])
        checks[LABELS[5]] = absences["lock"] == [True, True]
        checks[LABELS[6]] = absences["workspace"] == [True, True]
    finally:
        try:
            cleanup(root)
            checks[LABELS[7]] = True
        except Exception:
            pass
    summary(env, checks)
    need(all(checks.values()))


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
        elif args.phase == "cleanup":
            cleanup(context(env, gate=False))
        else:
            context(env)
    except Exception:
        print("STOP: current-state metadata read rejected; private diagnostic suppressed.", file=sys.stderr)
        return 3
    print("Current-state metadata read checks passed; values suppressed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
