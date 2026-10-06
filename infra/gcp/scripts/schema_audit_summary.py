#!/usr/bin/env python3
"""Project a private audit result into a fixed, count-free public decision."""

from __future__ import annotations

import json
import sys
from pathlib import Path

COUNTS = ("legacy_non_null", "legacy_only", "both_equal", "both_different")
FLAGS = ("canonical", "has_taken_at", "has_legacy_timestamp")
TARGET = {
    "environment": "development",
    "project_id": "test-youtube-study-space",
    "dataset": "firestore_export",
    "table": "user-activity-history",
}


def unique_object(pairs: list[tuple[str, object]]) -> dict:
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate field")
        result[key] = value
    return result


def summarize(payload: object) -> str:
    if type(payload) is not dict or set(payload) != {"mode", "target", "audit"}:
        raise ValueError("invalid envelope")
    if payload["mode"] != "read-only-aggregate" or payload["target"] != TARGET:
        raise ValueError("invalid scope")
    audit = payload["audit"]
    if type(audit) is not dict or set(audit) != set(COUNTS + FLAGS):
        raise ValueError("invalid audit fields")
    if any(type(audit[key]) is not bool for key in FLAGS):
        raise ValueError("invalid flags")
    if any(type(audit[key]) is not int or not 0 <= audit[key] <= 2**63 - 1 for key in COUNTS):
        raise ValueError("invalid counts")
    if not audit["has_taken_at"] or audit["canonical"] == audit["has_legacy_timestamp"]:
        raise ValueError("inconsistent schema flags")
    if audit["legacy_non_null"] != sum(audit[key] for key in COUNTS[1:]):
        raise ValueError("inconsistent aggregate partition")
    if not audit["has_legacy_timestamp"] and any(audit[key] for key in COUNTS):
        raise ValueError("counts without legacy field")

    lines = [
        "## Development user-activity schema audit",
        "",
        "- Scope: aggregate-only / read-only",
        f"- Legacy timestamp present: {'yes' if audit['has_legacy_timestamp'] else 'no'}",
    ]
    lines.extend(f"- {key} is zero: {'yes' if audit[key] == 0 else 'no'}" for key in COUNTS)
    lines.append("- Exact aggregate counts are intentionally not published.")
    return "\n".join(lines) + "\n"


def main() -> int:
    try:
        if len(sys.argv) != 3:
            raise ValueError("invalid arguments")
        with Path(sys.argv[1]).open("r", encoding="utf-8") as handle:
            raw = handle.read(65537)
        if len(raw) > 65536:
            raise ValueError("oversized input")
        summary = summarize(json.loads(raw, object_pairs_hook=unique_object))
        # Validate the entire projection before touching the public destination.
        with Path(sys.argv[2]).open("a", encoding="utf-8") as handle:
            handle.write(summary)
    except (ValueError, OSError, TypeError, RecursionError):
        print("::error::Schema audit summary rejected. Private output is intentionally not printed.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
