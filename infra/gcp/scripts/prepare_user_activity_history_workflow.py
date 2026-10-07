#!/usr/bin/env python3
"""Protected, separately activated dev history metadata preparation.

Use an already issued workflow token for one exact tables.get. No token minting,
query, data read, retry, IAM, schema, state or ownership activation is performed.
The workflow default-off gate and live prerequisites remain separate approvals.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import stat
import sys
from pathlib import Path

from prepare_user_activity_history_adoption import prepare, private_json, require

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / ".github/scripts"))
from terraform_identity_smoke import google

TABLE_PATH = "bigquery/v2/projects/test-youtube-study-space/datasets/firestore_export/tables/user-activity-history"


def prepare_workflow(env: dict[str, str], *, phase: str, request=google) -> None:
    require(phase in {"before", "post"})
    require(env.get("TF_VAR_project_id") == "test-youtube-study-space")
    require(env.get("TF_VAR_manage_user_activity_history") == "true")
    require(env.get("HISTORY_OPERATION") in {"plan", "apply"})
    require(env.get("HISTORY_STAGE") in {"plan", "apply"})
    require(phase != "post" or env["HISTORY_STAGE"] == "apply")
    require(all(env.get("TF_VAR_manage_export_" + kind) == "true" for kind in ("topic", "scheduler", "function")))
    require(bool(env.get("GCP_SMOKE_ACCESS_TOKEN")))
    status, metadata = request(TABLE_PATH, env["GCP_SMOKE_ACCESS_TOKEN"], host="bigquery.googleapis.com")
    require(status == 200)
    candidate = prepare(metadata)
    digest = hashlib.sha256(json.dumps(candidate, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    if env["HISTORY_STAGE"] == "apply":
        require(env.get("HISTORY_APPROVED_METADATA_DIGEST") == digest)
    directory = Path(env["RUNNER_TEMP"])
    require(directory.is_absolute())
    if phase == "post":
        # State-only adoption must not silently accept a concurrent field-order
        # change by feeding a different new order to the post-plan.
        require(candidate == prepare(private_json(str(directory / "user-history-before.json"))))
    target = directory / f"user-history-{phase}.json"
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(metadata, handle, separators=(",", ":"))
            handle.write("\n")
        order = json.dumps(candidate["user_activity_history_field_order"], separators=(",", ":"))
        # GITHUB_ENV is a runner-owned regular file, never a public Summary or
        # output. Serialize only fixed names, canonical field order and a path.
        require("\n" not in str(target) and "\r" not in str(target))
        env_fd = os.open(env["GITHUB_ENV"], os.O_WRONLY | os.O_APPEND | os.O_NOFOLLOW)
        with os.fdopen(env_fd, "w", encoding="utf-8") as handle:
            info = os.fstat(handle.fileno())
            require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
            handle.write(f"TF_VAR_user_activity_history_field_order={order}\n")
            handle.write(f"YSS_USER_ACTIVITY_HISTORY_METADATA_FILE={target}\n")
        output_fd = os.open(env["GITHUB_OUTPUT"], os.O_WRONLY | os.O_APPEND | os.O_NOFOLLOW)
        with os.fdopen(output_fd, "w", encoding="utf-8") as handle:
            info = os.fstat(handle.fileno())
            require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
            handle.write(f"history_metadata_digest={digest}\n")
    except Exception:
        target.unlink(missing_ok=True)
        raise


class PrivateArgumentParser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise ValueError("invalid protected preparation arguments")


def main() -> int:
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument("--phase", required=True, choices=("before", "post"))
        args = parser.parse_args()
        prepare_workflow(dict(os.environ), phase=args.phase)
    except Exception:
        print("STOP: protected history metadata rejected; private diagnostic suppressed.", file=sys.stderr)
        return 3
    print("Canonical history metadata prepared privately; no query or mutation performed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
