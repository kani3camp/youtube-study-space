#!/usr/bin/env python3
"""Feed provider-generated metrics-only update Plans into real admission."""
import json
from pathlib import Path
import subprocess

from foundation_contract import need
import runtime_contract as r

root = Path(__file__).resolve().parents[1] / "environments/dev"
result = subprocess.run(["terraform", "test", "-filter=tests/read-metrics-transition.tftest.hcl", "-verbose", "-json"],
                        cwd=root, capture_output=True, text=True, timeout=180)
need(result.returncode == 0, "metrics-native-tests")
events = [json.loads(line) for line in result.stdout.splitlines()]
plans = [event["test_plan"] for event in events if event.get("type") == "test_plan"]
need(len(plans) == 2, "metrics-native-plan-count")
for rendered, enabled in zip(plans, (True, False)):
    settings = r.settings(json.loads((root / "tests/runtime-settings.fixture.json").read_text()) | {
        "read_metrics_enabled": enabled, "read_metrics_from_revision": "mypage-api-before",
    })
    # As in the existing native checker, test renderer metadata is adapted.
    # Provider-generated actions, before/after and unknowns are NOT rewritten.
    envelope = {"format_version": rendered["plan_format_version"], "complete": True, "errored": False,
                "variables": {"stage": {"value": "runtime_private"}, "runtime_settings": {"value": settings}},
                "resource_changes": rendered.get("resource_changes", []),
                "resource_drift": rendered.get("resource_drift", []),
                "deferred_changes": rendered.get("deferred_changes", [])}
    need(r.validate_plan(envelope, "runtime_private") == {"resources": 29, "creates": 0, "updates": 1},
         "metrics-native-admission")
print("Native mocked metrics enable/disable Plans each admitted exactly one update; no cloud I/O.")
