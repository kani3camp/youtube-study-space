#!/usr/bin/env python3
"""Prepare default-off Terraform inputs from private canonical table metadata.

This tool has no cloud client and performs no query, mutation, import or approval.
Fresh, complete tables.get provenance remains the operator's responsibility.
"""

from __future__ import annotations

import json
import os
import stat
import sys
from pathlib import Path

REFERENCE = {
    "projectId": "test-youtube-study-space",
    "datasetId": "firestore_export",
    "tableId": "user-activity-history",
}
LOCATION = "asia-southeast2"
SCHEMA = Path(__file__).parents[1] / "modules/retained-user-activity-history/schema.json"
ALIASES = {"INT64": "INTEGER", "BOOL": "BOOLEAN", "STRUCT": "RECORD"}


def require(condition: bool) -> None:
    if not condition:
        raise ValueError("metadata does not match the canonical adoption contract")


def unique_object(pairs: list[tuple[str, object]]) -> dict:
    result = {}
    for key, value in pairs:
        require(key not in result)
        result[key] = value
    return result


def normalize_field(field: object) -> dict:
    require(type(field) is dict)
    require({"name", "type"} <= set(field) <= {"name", "type", "mode", "fields", "description"})
    require(type(field["name"]) is str and type(field["type"]) is str)
    require(field.get("description", "") == "")
    mode = field.get("mode", "NULLABLE")
    require(type(mode) is str)
    result = {"name": field["name"], "type": ALIASES.get(field["type"], field["type"]), "mode": mode}
    if "fields" in field:
        require(type(field["fields"]) is list)
        if field["fields"]:
            result["fields"] = [normalize_field(child) for child in field["fields"]]
    return result


def prepare(metadata: object) -> dict:
    require(type(metadata) is dict)
    require(metadata.get("tableReference") == REFERENCE)
    require(metadata.get("type") == "TABLE" and metadata.get("location") == LOCATION)
    require(type(metadata.get("schema")) is dict and set(metadata["schema"]) == {"fields"})
    fields = metadata["schema"]["fields"]
    require(type(fields) is list)
    actual = [normalize_field(field) for field in fields]
    expected = [normalize_field(field) for field in json.loads(SCHEMA.read_text())]
    expected_by_name = {field["name"]: field for field in expected}
    require(len(expected) == len(expected_by_name) == 8)
    require("taken_at" in expected_by_name and "timestamp" not in expected_by_name)
    names = [field["name"] for field in actual]
    # Never silently strip timestamp, guess a field order, or relax nested modes.
    require(len(names) == len(expected) and set(names) == set(expected_by_name))
    require(all(field == expected_by_name[field["name"]] for field in actual))
    for key in ("description", "friendlyName", "maxStaleness"):
        require(metadata.get(key, "") == "")
    for key in ("labels", "resourceTags"):
        require(metadata.get(key, {}) == {})
    for key in (
        "timePartitioning", "rangePartitioning", "clustering", "expirationTime",
        "encryptionConfiguration", "externalDataConfiguration", "view", "materializedView",
        "snapshotDefinition", "cloneDefinition", "tableConstraints", "biglakeConfiguration",
    ):
        require(metadata.get(key) is None)
    require(metadata.get("requirePartitionFilter", False) is False)
    return {"manage_user_activity_history": False, "user_activity_history_field_order": names}


def private_json(path: str) -> object:
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, "rb") as handle:
        info = os.fstat(handle.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
        require(info.st_mode & 0o077 == 0)
        raw = handle.read(4 * 1024 * 1024 + 1)
    require(len(raw) <= 4 * 1024 * 1024)
    return json.loads(raw.decode("utf-8"), object_pairs_hook=unique_object)


def main() -> int:
    try:
        require(len(sys.argv) == 3)
        candidate = prepare(private_json(sys.argv[1]))
        fd = os.open(sys.argv[2], os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(candidate, handle, indent=2)
            handle.write("\n")
    except (ValueError, OSError, TypeError, KeyError, RecursionError):
        print("STOP: canonical metadata preparation rejected; no cloud operation performed.", file=sys.stderr)
        return 1
    print("Offline preparation passed. Adoption remains disabled; this is not approval or a live plan.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
