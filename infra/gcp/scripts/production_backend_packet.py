#!/usr/bin/env python3
"""Offline private production backend proposal; never reads or changes a cloud.

The caller supplies observed private identifiers, not credentials. This proposal
does not attest inventory, effective permissions, development exit, or approval.
"""
from __future__ import annotations

from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import stat
import sys

from prepare_user_activity_history_workflow import PrivateArgumentParser

ROOT = Path(__file__).resolve().parents[3]
STATE_KEY = "youtube-study-space/prod/terraform.tfstate"
WORKSPACE_PREFIX = "youtube-study-space/prod/workspaces"
CLAIM_KEYS = (
    "repository_id", "repository_owner_id", "environment", "ref",
    "workflow_ref", "job_workflow_ref", "event_name",
)
FIELDS = {
    "schema_version", "source_git_sha", "observed_at", "state_account_id",
    "state_bucket", "state_region", "state_status", "bucket_encryption",
    "production_plan_role_name", "production_apply_role_name",
    "development_plan_role_name", "development_apply_role_name",
    "oidc_include_claim_keys",
}


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("Duplicate field")
        result[key] = value
    return result


def validate(inventory):
    if type(inventory) is not dict or set(inventory) != FIELDS:
        raise ValueError("Exact noncredential inventory fields required")
    if inventory["schema_version"] != "production-backend-private-inventory-v1":
        raise ValueError("Unsupported inventory contract")
    patterns = {
        "source_git_sha": r"[0-9a-f]{40}", "state_account_id": r"[0-9]{12}",
        "state_bucket": r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]",
        "state_region": r"[a-z]{2}-[a-z]+-[1-9][0-9]*",
    }
    for key, pattern in patterns.items():
        if type(inventory[key]) is not str or not re.fullmatch(pattern, inventory[key]):
            raise ValueError("Invalid private target")
    bucket = inventory["state_bucket"]
    if ".." in bucket or ".-" in bucket or "-." in bucket or re.fullmatch(r"[0-9]+(?:\.[0-9]+){3}", bucket):
        raise ValueError("Invalid bucket")
    role_fields = [f"{environment}_{kind}_role_name" for environment in ("production", "development") for kind in ("plan", "apply")]
    for key in role_fields:
        if type(inventory[key]) is not str or not re.fullmatch(r"[A-Za-z0-9+=,.@_-]{1,64}", inventory[key]):
            raise ValueError("Exact role name required")
    if len({inventory[key] for key in role_fields}) != len(role_fields):
        raise ValueError("Production and development identities must be distinct")
    if inventory["oidc_include_claim_keys"] != list(CLAIM_KEYS):
        raise ValueError("Observed OIDC subject template must match; never change it here")
    if inventory["state_status"] not in {"absent", "empty", "existing"}:
        raise ValueError("State status must distinguish observed absence from failure")
    # Existing SSE-S3 control plane only. KMS migration/grants are a different unit.
    if inventory["bucket_encryption"] != "AES256":
        raise ValueError("Existing encryption requires a separate reviewed contract")
    observed = datetime.fromisoformat(inventory["observed_at"].replace("Z", "+00:00"))
    if observed.tzinfo is None or observed > datetime.now(timezone.utc):
        raise ValueError("Observed inventory timestamp required")
    return inventory


def trust_policy(account_id, kind):
    claims = {
        "repository_id": "340900071", "repository_owner_id": "54093651",
        "environment": f"terraform-prod-{kind}", "ref": "refs/heads/main",
        "workflow_ref": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/main",
        "job_workflow_ref": "kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/main",
        "event_name": "workflow_dispatch",
    }
    subject = ":".join(f"{key}:{claims[key].replace(':', '%3A')}" for key in CLAIM_KEYS)
    return {"Version": "2012-10-17", "Statement": [{
        "Sid": "ExactProductionEnvironment", "Effect": "Allow",
        "Principal": {"Federated": f"arn:aws:iam::{account_id}:oidc-provider/token.actions.githubusercontent.com"},
        "Action": "sts:AssumeRoleWithWebIdentity",
        "Condition": {"StringEquals": {
            "token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
            "token.actions.githubusercontent.com:sub": subject,
        }},
    }]}


def permissions_policy(bucket, kind):
    bucket_arn = f"arn:aws:s3:::{bucket}"
    state_arn = f"{bucket_arn}/{STATE_KEY}"
    return {"Version": "2012-10-17", "Statement": [
        {"Sid": "ExactDefaultState", "Effect": "Allow",
         "Action": ["s3:GetObject"] + (["s3:PutObject"] if kind == "apply" else []),
         "Resource": state_arn},
        {"Sid": "ExactNativeLock", "Effect": "Allow",
         "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],
         "Resource": state_arn + ".tflock"},
        {"Sid": "ProductionDiscoveryOnly", "Effect": "Allow", "Action": "s3:ListBucket",
         "Resource": bucket_arn, "Condition": {"StringEquals": {
             "s3:prefix": [STATE_KEY, STATE_KEY + ".tflock", WORKSPACE_PREFIX + "/"],
         }}},
    ]}


def build_packet(inventory):
    inventory = validate(inventory)
    return {
        "schema_version": "production-backend-proposal-v1", "cloud_mutations": 0,
        "live_readiness_attested": False, "inventory": inventory,
        "backend_config": {
            "bucket": inventory["state_bucket"], "key": STATE_KEY,
            "region": inventory["state_region"], "encrypt": True, "use_lockfile": True,
            "workspace_key_prefix": WORKSPACE_PREFIX,
            "allowed_account_ids": [inventory["state_account_id"]],
        },
        "roles": {kind: {
            "role_name": inventory[f"production_{kind}_role_name"],
            "trust_policy": trust_policy(inventory["state_account_id"], kind),
            "permissions_policy": permissions_policy(inventory["state_bucket"], kind),
        } for kind in ("plan", "apply")},
    }


def private_path(path):
    resolved = path.resolve()
    if (not path.is_absolute() or path.is_symlink() or resolved.is_relative_to(ROOT)
            or any((parent / ".git").exists() for parent in resolved.parents)):
        raise ValueError("Private file outside the checkout required")


def private_file(path):
    private_path(path)
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor) as source:
        metadata = os.fstat(source.fileno())
        if not stat.S_ISREG(metadata.st_mode) or metadata.st_uid != os.getuid() or metadata.st_mode & 0o077 or metadata.st_size > 65536:
            raise ValueError("Owner-only bounded private file required")
        value = source.read(65537)
        if len(value) > 65536:
            raise ValueError("Bounded private inventory required")
    return json.loads(value, object_pairs_hook=unique_object)


def main(argv=None):
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument("--inventory", type=Path, required=True)
        parser.add_argument("--output", type=Path, required=True)
        args = parser.parse_args(argv)
        packet = build_packet(private_file(args.inventory))
        destination = args.output
        private_path(destination)
        # O_EXCL also rejects a raced symlink; never overwrite an older packet.
        descriptor = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "w") as output:
            os.fchmod(output.fileno(), 0o600)
            output.write(json.dumps(packet, sort_keys=True, indent=2) + "\n")
        print("Production backend proposal prepared privately; live readiness remains unverified.")
        return 0
    except Exception:
        print("Production backend proposal STOP; private diagnostic suppressed.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
