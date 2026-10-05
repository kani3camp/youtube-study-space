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
        plan = json.load(sys.stdin)
        if args.operation == "email-adoption":
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
            if os.environ.get("TF_VAR_manage_export_topic") == "true":
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
