#!/usr/bin/env python3
"""Create a value-free Terraform plan summary for public CI output.

The input is the JSON emitted by `terraform show -json <saved-plan>` on stdin.
Only resource addresses, normalized action classes, counts, target metadata, and
policy results are emitted. Resource values, outputs, provider config, import IDs,
and raw state fields are intentionally ignored.
"""

from __future__ import annotations

import argparse
import hashlib
import html
import json
import sys
from pathlib import Path
from typing import Any

KNOWN_ACTIONS = {
    ("no-op",): "no-op",
    ("read",): "read",
    ("create",): "create",
    ("update",): "update",
    ("delete",): "delete",
    ("delete", "create"): "replace",
    ("create", "delete"): "replace",
}

COUNT_KEYS = ("import", "create", "update", "delete", "replace", "read", "no-op", "other", "drift")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--environment", required=True, choices=("dev", "prod"))
    parser.add_argument("--git-sha", required=True)
    parser.add_argument("--policy", required=True, choices=("import-only", "no-destroy", "plan-only"))
    parser.add_argument("--json-output", required=True, type=Path)
    parser.add_argument("--markdown-output", required=True, type=Path)
    return parser.parse_args()


def normalize_action(actions: Any) -> str:
    if not isinstance(actions, list) or not all(isinstance(item, str) for item in actions):
        return "other"
    return KNOWN_ACTIONS.get(tuple(actions), "other")


def is_importing(change: Any) -> bool:
    return isinstance(change, dict) and isinstance(change.get("importing"), dict)


def collect_resource_changes(plan: dict[str, Any]) -> list[dict[str, Any]]:
    resources: list[dict[str, Any]] = []
    raw_changes = plan.get("resource_changes", [])
    if not isinstance(raw_changes, list):
        return resources

    for item in raw_changes:
        if not isinstance(item, dict):
            continue
        address = item.get("address")
        if not isinstance(address, str):
            address = "(unknown-address)"
        mode = item.get("mode") if item.get("mode") in ("managed", "data") else "unknown"
        change = item.get("change") if isinstance(item.get("change"), dict) else {}
        action = normalize_action(change.get("actions"))
        importing = is_importing(change)
        resources.append(
            {
                "address": address,
                "mode": mode,
                "action": action,
                "import": importing,
            }
        )
    return sorted(resources, key=lambda item: (item["address"], item["action"], item["import"]))


def collect_drift(plan: dict[str, Any]) -> list[dict[str, str]]:
    drift: list[dict[str, str]] = []
    raw_drift = plan.get("resource_drift", [])
    if not isinstance(raw_drift, list):
        return drift
    for item in raw_drift:
        if not isinstance(item, dict):
            continue
        address = item.get("address")
        if not isinstance(address, str):
            address = "(unknown-address)"
        change = item.get("change") if isinstance(item.get("change"), dict) else {}
        drift.append({"address": address, "action": normalize_action(change.get("actions"))})
    return sorted(drift, key=lambda item: (item["address"], item["action"]))


def build_summary(plan: dict[str, Any], *, environment: str, git_sha: str, policy: str) -> dict[str, Any]:
    resources = collect_resource_changes(plan)
    drift = collect_drift(plan)
    counts = {key: 0 for key in COUNT_KEYS}

    for resource in resources:
        action = resource["action"]
        counts[action if action in counts else "other"] += 1
        if resource["import"]:
            counts["import"] += 1
    counts["drift"] = len(drift)

    violations = policy_violations(resources, drift, policy)
    return {
        "schema_version": 1,
        "environment": environment,
        "git_sha": git_sha,
        "terraform_version": plan.get("terraform_version") if isinstance(plan.get("terraform_version"), str) else "unknown",
        "policy": policy,
        "policy_passed": not violations,
        "violations": violations,
        "counts": counts,
        "resources": resources,
        "drift": drift,
    }


def policy_violations(resources: list[dict[str, Any]], drift: list[dict[str, str]], policy: str) -> list[str]:
    violations: list[str] = []
    if policy == "plan-only":
        return violations

    for resource in resources:
        action = resource["action"]
        address = resource["address"]
        importing = resource["import"]

        if action == "other":
            violations.append(f"unknown action at {address}")
            continue

        if policy == "import-only":
            if action in {"create", "update", "delete", "replace"}:
                violations.append(f"{action} at {address}")
            elif importing and action not in {"no-op", "read"}:
                violations.append(f"import with unsupported action at {address}")
        elif policy == "no-destroy" and action in {"delete", "replace"}:
            violations.append(f"{action} at {address}")

    if policy == "import-only" and drift:
        for item in drift:
            violations.append(f"drift at {item['address']}")

    return sorted(set(violations))


def markdown_escape(value: str) -> str:
    return html.escape(value, quote=True).replace("|", "&#124;")


def render_markdown(summary: dict[str, Any]) -> str:
    counts = summary["counts"]
    lines = [
        "### Terraform sanitized plan summary",
        "",
        f"- Environment: `{markdown_escape(summary['environment'])}`",
        f"- Git SHA: `{markdown_escape(summary['git_sha'])}`",
        f"- Terraform: `{markdown_escape(summary['terraform_version'])}`",
        f"- Policy: `{markdown_escape(summary['policy'])}`",
        f"- Policy result: **{'PASS' if summary['policy_passed'] else 'STOP'}**",
        "",
        "| import | create | update | delete | replace | read | no-op | drift | other |",
        "| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |",
        f"| {counts['import']} | {counts['create']} | {counts['update']} | {counts['delete']} | {counts['replace']} | {counts['read']} | {counts['no-op']} | {counts['drift']} | {counts['other']} |",
    ]

    if summary["resources"]:
        lines.extend(["", "#### Resource actions", "", "| Address | Mode | Action | Import |", "| --- | --- | --- | --- |"])
        for resource in summary["resources"]:
            lines.append(
                "| <code>{}</code> | {} | {} | {} |".format(
                    markdown_escape(resource["address"]),
                    markdown_escape(resource["mode"]),
                    markdown_escape(resource["action"]),
                    "yes" if resource["import"] else "no",
                )
            )

    if summary["drift"]:
        lines.extend(["", "#### Resource drift", "", "| Address | Drift action |", "| --- | --- |"])
        for item in summary["drift"]:
            lines.append(f"| <code>{markdown_escape(item['address'])}</code> | {markdown_escape(item['action'])} |")

    if summary["violations"]:
        lines.extend(["", "#### Stop reasons"])
        for violation in summary["violations"]:
            lines.append(f"- {markdown_escape(violation)}")

    return "\n".join(lines) + "\n"


def write_private(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    path.chmod(0o600)


def main() -> int:
    args = parse_args()
    try:
        plan = json.load(sys.stdin)
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        print(f"sanitizer: invalid Terraform JSON input: {exc}", file=sys.stderr)
        return 2
    if not isinstance(plan, dict):
        print("sanitizer: Terraform JSON root must be an object", file=sys.stderr)
        return 2

    summary = build_summary(plan, environment=args.environment, git_sha=args.git_sha, policy=args.policy)
    canonical = json.dumps(summary, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n"
    write_private(args.json_output, canonical)
    write_private(args.markdown_output, render_markdown(summary))

    digest = hashlib.sha256(canonical.encode("utf-8")).hexdigest()
    print(f"summary_sha256={digest}")
    if not summary["policy_passed"]:
        for violation in summary["violations"]:
            print(f"sanitizer stop: {violation}", file=sys.stderr)
        return 3
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
