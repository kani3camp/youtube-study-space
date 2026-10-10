#!/usr/bin/env python3
"""Private read-only receipt for a separately gated full-root history no-op."""
from __future__ import annotations

import hashlib
import os
from pathlib import Path
import sys

import terraform_history_plan_receipt as receipt
from terraform_history_table_metadata import stable_table_metadata
from terraform_identity_smoke import google
from terraform_plan_summary import COUNT_KEYS
from validate_user_activity_history_plan import TABLE, TABLE_ID

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "infra/gcp/scripts"))
from prepare_user_activity_history_adoption import normalize_field
from prepare_user_activity_history_workflow import PrivateArgumentParser, TABLE_PATH


BEFORE = "history-post-noop-before.json"
STATE_BEFORE = "history-post-noop-state-before.json"
STATE_AFTER = "history-post-noop-state-after.json"
EXPECTED_COUNTS = {key: (12 if key == "no-op" else 0) for key in COUNT_KEYS}


def context(env):
    receipt.require(env.get("HISTORY_POST_NOOP") == "true"
                    and env.get("DEV_HISTORY_POST_NOOP_ENABLED") == "true"
                    and env.get("HISTORY_TARGET") == "dev"
                    and env.get("GITHUB_REF") == "refs/heads/feature/gcp-terraform-iac",
                    "post-noop-context")
    return receipt.load_policy(env)


def snapshot(env, root, filename, metadata, maximum, *, request=None):
    first = receipt.head(env, maximum, request=request)
    path = root / filename
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    os.close(fd)
    got = receipt.s3(env, "get-object", "--key", receipt.STATE_KEY,
                     "--range", f"bytes=0-{maximum}", str(path), request=request)
    raw = receipt.private_bytes(path)
    receipt.require(got.get("VersionId") == first["VersionId"]
                    and got.get("ETag") == first["ETag"]
                    and got.get("ContentLength") == first["ContentLength"] == len(raw)
                    and receipt.head(env, maximum, request=request) == first,
                    "post-noop-state-snapshot")
    state = receipt.state_shape(raw, imported=True)
    tables = [r for r in state["resources"] if r.get("module") == "module.user_activity_history[0]"
              and r["type"] == "google_bigquery_table" and r["name"] == "retained"]
    receipt.require(len(tables) == 1 and len(tables[0]["instances"]) == 1, "post-noop-table-owner")
    attributes = tables[0]["instances"][0]["attributes"]
    receipt.require(all(attributes.get(key) == value for key, value in {
        "id": TABLE_ID, "project": "test-youtube-study-space", "dataset_id": "firestore_export",
        "table_id": "user-activity-history", "location": "asia-southeast2",
        "deletion_protection": True}.items()), "post-noop-table-identity")
    fields = receipt.decode(attributes.get("schema"))
    receipt.require(type(fields) is list and [normalize_field(field) for field in fields] ==
                    [normalize_field(field) for field in metadata["schema"]["fields"]],
                    "post-noop-table-schema")
    receipt.absent(env, receipt.STATE_KEY + ".tflock", request=request)
    receipt.absent(env, receipt.STATE_KEY.rsplit("/", 1)[0] + "/workspaces/", request=request)
    return first, raw


def exact_noop_summary(summary, sha):
    receipt.require(type(summary) is dict and summary.get("environment") == "dev"
                    and summary.get("git_sha") == sha and summary.get("policy") == "import-only"
                    and summary.get("policy_passed") is True and summary.get("violations") == []
                    and summary.get("drift") == [] and summary.get("counts") == EXPECTED_COUNTS,
                    "post-noop-summary")
    resources = summary.get("resources")
    receipt.require(type(resources) is list and len(resources) == 12
                    and {item.get("address") for item in resources} == receipt.BASELINE | {TABLE}
                    and all(type(item) is dict and item.get("mode") == "managed"
                            and item.get("action") == "no-op" and item.get("import") is False
                            for item in resources), "post-noop-resources")


def before(env, *, request=None):
    root, cost = context(env)
    metadata = receipt.private_json(str(root / "user-history-post.json"))
    stable = stable_table_metadata(metadata)
    head, raw = snapshot(env, root, STATE_BEFORE, metadata,
                         cost["profile"]["max_state_bytes"], request=request)
    receipt.write_private(root / BEFORE, {"git_sha": env["GITHUB_SHA"], "head": head,
                                         "state_sha256": hashlib.sha256(raw).hexdigest(),
                                         "stable_metadata": stable})


def after(env, *, request=None, metadata_request=None):
    root, cost = context(env)
    initial = receipt.private_json(str(root / BEFORE))
    receipt.require(initial["git_sha"] == env["GITHUB_SHA"], "post-noop-sha")
    head, raw = snapshot(env, root, STATE_AFTER,
                         receipt.private_json(str(root / "user-history-post.json")),
                         cost["profile"]["max_state_bytes"], request=request)
    receipt.require(head == initial["head"]
                    and hashlib.sha256(raw).hexdigest() == initial["state_sha256"]
                    and raw == receipt.private_bytes(root / STATE_BEFORE),
                    "post-noop-state-unchanged")
    status, metadata = (metadata_request or google)(TABLE_PATH, env["GCP_SMOKE_ACCESS_TOKEN"],
                                                    host="bigquery.googleapis.com")
    receipt.require(status == 200
                    and stable_table_metadata(metadata) == initial["stable_metadata"],
                    "post-noop-metadata-unchanged")
    receipt.write_private(root / "history-post-noop-table-after.json", metadata)
    exact_noop_summary(receipt.private_json(str(root / "sanitized-plan.json")), env["GITHUB_SHA"])
    receipt.require(env.get("BACKEND_INIT_OUTCOME") == "success"
                    and env.get("TERRAFORM_PLAN_OUTCOME") == "success"
                    and env.get("PLAN_VALIDATION_OUTCOME") == "success"
                    and env.get("PLAN_EXIT_CODE") == "0", "post-noop-plan-outcome")
    # Recheck the cost evidence's expiry after all read-only post checks.
    receipt.load_policy(env)
    with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as output:
        output.write("Independent history post-import no-op: PASS; import0/no-op12; "
                     "state and stable table metadata unchanged; native lock/workspaces absent; "
                     "volatile table observations recorded privately.\n")


def main():
    try:
        parser = PrivateArgumentParser()
        parser.add_argument("--phase", required=True, choices=("before", "after"))
        args = parser.parse_args()
        {"before": before, "after": after}[args.phase](dict(os.environ))
    except Exception:
        print("STOP: independent history post-import no-op rejected; private diagnostic suppressed.",
              file=sys.stderr)
        return 3
    print("Independent history post-import no-op " + args.phase + " passed; no private values emitted.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
