#!/usr/bin/env python3
"""Value-free output for the separately approved development Email adoption.

Ordinary execution delegates to the unchanged global import-only policy. The
exception is available only through the protected workflow's explicit mode.
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

from terraform_email_adoption_gate import CHANNEL, EXISTING, canonical, has_unknown, validate
from terraform_plan_summary import build_summary, render_markdown, write_private
from terraform_quota_create_gate import validate as validate_quota
from terraform_quota_refresh_gate import validate as validate_quota_refresh
from terraform_export_topic_gate import validate as validate_export_topic
from terraform_export_scheduler_gate import validate as validate_export_scheduler
from terraform_export_function_gate import validate as validate_export_function

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "infra/gcp/scripts"))
from prepare_user_activity_history_adoption import prepare, private_json, unique_object
from validate_user_activity_history_plan import validate as validate_history


def adoption_summary(plan, *, environment, git_sha, phase, email, channel_name):
    if environment != "dev" or not email or not channel_name:
        raise ValueError("Development adoption target required")
    if phase == "before":
        validate(plan, email=email, channel_name=channel_name)
    else:
        changes = plan.get("resource_changes", [])
        if len(changes) != 5 or {r.get("address") for r in changes} != EXISTING | {CHANNEL}:
            raise ValueError("Post-adoption resource graph mismatch")
        if plan.get("resource_drift"):
            raise ValueError("Post-adoption drift")
        for resource in changes:
            change = resource.get("change", {})
            if resource.get("mode") != "managed" or change.get("actions") != ["no-op"] or change.get("importing"):
                raise ValueError("Post-adoption action mismatch")
            if has_unknown(change.get("after_unknown", {})) or canonical(change.get("before")) != canonical(change.get("after")):
                raise ValueError("Post-adoption value mismatch")
        for output in plan.get("output_changes", {}).values():
            if output.get("actions") != ["no-op"] or has_unknown(output.get("after_unknown", {})):
                raise ValueError("Post-adoption output change")
        channel = next(r["change"]["after"] for r in changes if r["address"] == CHANNEL)
        if channel.get("name") != channel_name or channel.get("labels") != {"email_address": email}:
            raise ValueError("Post-adoption identity mismatch")
    # plan-only is used solely to serialize an already validated exception.
    # Its permissive result never authorizes execution without validate above.
    summary = build_summary(plan, environment=environment, git_sha=git_sha, policy="plan-only")
    summary["policy"] = "development-email-adoption-" + phase
    return summary


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--operation", required=True, choices=("plan", "apply", "email-adoption", "quota-plan", "quota-create", "quota-refresh"))
    parser.add_argument("--phase", required=True, choices=("before", "post"))
    parser.add_argument("--environment", required=True, choices=("dev", "prod"))
    parser.add_argument("--git-sha", required=True)
    parser.add_argument("--policy", required=True, choices=("import-only",))
    parser.add_argument("--json-output", required=True, type=Path)
    parser.add_argument("--markdown-output", required=True, type=Path)
    args = parser.parse_args()
    try:
        plan = json.load(sys.stdin, object_pairs_hook=unique_object)
        history = os.environ.get("TF_VAR_manage_user_activity_history", "false")
        if history not in {"false", "true"}:
            raise ValueError("History ownership flag required")
        if history == "true":
            if args.environment != "dev" or args.operation not in {"plan", "apply"}:
                raise ValueError("History adoption cannot share an exceptional or production wave")
            if any(os.environ.get("TF_VAR_manage_export_" + kind) != "true" for kind in ("function", "scheduler", "topic")):
                raise ValueError("Adopted history dependencies required")
            directory = Path(os.environ["RUNNER_TEMP"])
            if not directory.is_absolute():
                raise ValueError("Private runner directory required")
            metadata = private_json(str(directory / f"user-history-{args.phase}.json"))
            inputs = private_json(str(directory / f"user-history-{args.phase}.tfvars.json"))
            expected_inputs = {key: value for key, value in prepare(metadata).items()
                               if key != "manage_user_activity_history"}
            if type(inputs) is not dict or inputs != expected_inputs:
                raise ValueError("Fresh history field order and descriptions required")
            validate_history(plan, metadata=metadata, phase=args.phase,
                             allow_adopted=not (args.operation == "plan" and args.phase == "before"),
                             execution_email=os.environ.get("TF_VAR_export_function_execution_service_account_email", ""))
            summary = build_summary(plan, environment=args.environment, git_sha=args.git_sha, policy=args.policy)
        elif args.operation == "email-adoption":
            summary = adoption_summary(plan, environment=args.environment, git_sha=args.git_sha, phase=args.phase,
                                       email=os.environ.get("TF_VAR_primary_email_address"),
                                       channel_name=os.environ.get("TF_VAR_primary_email_channel_name"))
        elif args.operation == "quota-refresh":
            if args.environment != "dev": raise ValueError("Development quota only")
            values = dict(channel_name=os.environ.get("TF_VAR_primary_email_channel_name", ""),
                          email=os.environ.get("TF_VAR_primary_email_address", ""))
            if args.phase == "before":
                validate_quota_refresh(plan, **values)
            else:
                validate_quota(plan, phase="post", **values)
            summary = build_summary(plan, environment=args.environment, git_sha=args.git_sha, policy="plan-only")
            summary["policy"] = "development-quota-state-only-refresh-" + args.phase
        elif args.operation in {"quota-plan", "quota-create"}:
            if args.environment != "dev": raise ValueError("Development quota only")
            validate_quota(plan, channel_name=os.environ.get("TF_VAR_primary_email_channel_name", ""),
                           email=os.environ.get("TF_VAR_primary_email_address", ""), phase=args.phase)
            summary = build_summary(plan, environment=args.environment, git_sha=args.git_sha, policy="plan-only")
            summary["policy"] = "development-quota-three-create-" + args.phase
        else:
            summary = build_summary(plan, environment=args.environment, git_sha=args.git_sha, policy=args.policy)
            if os.environ.get("TF_VAR_manage_export_function") == "true":
                if args.environment != "dev" or any(os.environ.get("TF_VAR_manage_export_" + kind) != "true" for kind in ("topic", "scheduler")):
                    raise ValueError("Development Function dependencies required")
                validate_export_function(plan, phase=args.phase, execution_email=os.environ.get("TF_VAR_export_function_execution_service_account_email"))
            elif os.environ.get("TF_VAR_manage_export_scheduler") == "true":
                if args.environment != "dev" or os.environ.get("TF_VAR_manage_export_topic") != "true":
                    raise ValueError("Development Scheduler topic dependency required")
                validate_export_scheduler(plan, phase=args.phase)
            elif os.environ.get("TF_VAR_manage_export_topic") == "true":
                if args.environment != "dev": raise ValueError("Development topic only")
                validate_export_topic(plan, phase=args.phase)
        write_private(args.json_output, json.dumps(summary, sort_keys=True, separators=(",", ":")) + "\n")
        write_private(args.markdown_output, render_markdown(summary))
        return 0 if summary["policy_passed"] else 3
    except Exception:
        print("Protected Terraform plan STOP; private diagnostic suppressed.", file=sys.stderr)
        return 3


if __name__ == "__main__":
    raise SystemExit(main())
