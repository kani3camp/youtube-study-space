"""Shared, value-free comparison contract for history tables.get snapshots."""
from __future__ import annotations

import re

from prepare_user_activity_history_adoption import prepare

# Only documented output-only observations may vary with independent writers.
# Unknown fields stay in the exact stable comparison.
VOLATILE_TABLE_FIELDS = frozenset({
    "etag", "lastModifiedTime", "streamingBuffer", "numRows", "numBytes",
    "numLongTermBytes", "numTimeTravelPhysicalBytes", "numTotalLogicalBytes",
    "numActiveLogicalBytes", "numLongTermLogicalBytes", "numTotalPhysicalBytes",
    "numActivePhysicalBytes", "numLongTermPhysicalBytes", "numPartitions",
})
VOLATILE_METRICS = VOLATILE_TABLE_FIELDS - {"etag", "streamingBuffer"}


def stable_table_metadata(metadata):
    """Validate the full snapshot, then retain every configuration field."""
    prepare(metadata)
    if type(metadata) is not dict:
        raise ValueError("history table metadata STOP")
    for key in VOLATILE_METRICS & metadata.keys():
        if type(metadata[key]) is not str or not re.fullmatch(r"[0-9]+", metadata[key]):
            raise ValueError("history table metadata STOP")
    if "etag" in metadata and (type(metadata["etag"]) is not str or not metadata["etag"]):
        raise ValueError("history table metadata STOP")
    if "streamingBuffer" in metadata:
        buffer = metadata["streamingBuffer"]
        if (type(buffer) is not dict or set(buffer) - {"estimatedRows", "estimatedBytes", "oldestEntryTime"}
                or any(type(value) is not str or not re.fullmatch(r"[0-9]+", value) for value in buffer.values())):
            raise ValueError("history table metadata STOP")
    return {key: value for key, value in metadata.items() if key not in VOLATILE_TABLE_FIELDS}


def volatile_table_metadata(metadata):
    return {key: value for key, value in metadata.items() if key in VOLATILE_TABLE_FIELDS}
