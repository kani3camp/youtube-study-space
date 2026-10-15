#!/usr/bin/env python3
"""Offline, SHA-bound private inputs for a single cumulative ownership wave.

Version 1 remains an offline contract. Protected staging requires version 2 or 3;
GitHub provenance is verified separately before cloud credentials. Gates remain
closed; apply uses the same private packet and separate protected approval.
"""
from __future__ import annotations

import base64
from datetime import datetime, timedelta, timezone
import hashlib
import json
import math
import os
from pathlib import Path
import re
import sys
import zlib

from prepare_runtime_ownership import prepare, require, unique_object
from prepare_user_activity_history_adoption import private_json

WAVES = {"pool", "provider", "grant", "api", "noop"}
SELECTION_KEYS = {"pool", "provider", "grants", "apis"}
ROOT_BASES = {2: "original-history-import", 3: "current-state-bootstrap"}
MAX_PACKET_BYTES = 4 * 1024 * 1024
MAX_REVIEWED_WRAPPER_BYTES = 48000
REVIEWED_CODEC_KIND = "runtime-reviewed-inputs-zlib-v1"
REVIEWED_WRAPPER_KEYS = {"schema_version", "kind", "encoding", "uncompressed_bytes", "sha256", "payload"}
BASE_KEYS = {"schema_version", "git_sha", "wave", "reviewed_utc", "expires_utc", "inventory", "adopted", "selected"}
REVIEWED_KEYS = BASE_KEYS | {"current_state_attestation", "adoption_receipts"}


def root_reference(packet):
    return packet["current_state_attestation" if packet["schema_version"] == 3 else "history_receipt"]


def require_root_basis(packet, env):
    require(packet.get("schema_version") in ROOT_BASES
            and env.get("OWNERSHIP_ROOT_BASIS", "original-history-import") == ROOT_BASES[packet["schema_version"]])


def invalid_constant(_):
    raise ValueError("ownership packet STOP")


def finite_float(raw):
    value = float(raw)
    require(math.isfinite(value))
    return value


def timestamp(value):
    require(type(value) is str and re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z", value))
    return datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)


def selection(value, candidate):
    require(type(value) is dict and set(value) == SELECTION_KEYS)
    require(type(value["pool"]) is bool and type(value["provider"]) is bool)
    for key, available in (("grants", candidate["runtime_wif_inventory"]["grants"]),
                           ("apis", candidate["api_classification"])):
        keys = value[key]
        require(type(keys) is list and all(type(k) is str for k in keys))
        require(len(keys) == len(set(keys)) and set(keys) <= set(available))
    require(not value["provider"] or value["pool"])
    require(not value["grants"] or value["provider"])
    require(not value["apis"] or (value["provider"] and
            set(value["grants"]) == set(candidate["runtime_wif_inventory"]["grants"])))
    for service in value["apis"]:
        item = candidate["api_classification"][service]
        require(item["classification"] == "Own" and item["state"] == "ENABLED")
    return value


def validate_packet(packet, *, wave, git_sha, now=None, check_chain_shape=True):
    require(wave in WAVES and type(packet) is dict)
    version = packet.get("schema_version")
    require(type(version) is int and version in {1, 2, 3})
    require(set(packet) == BASE_KEYS | ({"history_post_noop_run_id"} if version == 1 else
            {"receipt_binding", "current_state_attestation" if version == 3 else "history_receipt", "adoption_receipts"}))
    require(type(git_sha) is str and re.fullmatch(r"[0-9a-f]{40}", git_sha))
    require(packet["git_sha"] == git_sha and packet["wave"] == wave)
    reviewed, expires = timestamp(packet["reviewed_utc"]), timestamp(packet["expires_utc"])
    require(reviewed <= (now or datetime.now(timezone.utc)) < expires <= reviewed + timedelta(hours=24))
    if version == 1:
        require(type(packet["history_post_noop_run_id"]) is int and packet["history_post_noop_run_id"] > 0)
    else:
        sys.path.insert(0, str(Path(__file__).resolve().parents[3] / ".github/scripts"))
        import terraform_ownership_receipt as receipt
        receipt.binding(packet["receipt_binding"])
        if version == 3:
            import terraform_current_state_attestation as current_state
            current_state.reference(packet["current_state_attestation"])
        else:
            receipt.reference(packet["history_receipt"])
        chain = packet["adoption_receipts"]
        require(type(chain) is list and len(chain) <= 256)
        for item in chain:
            require(type(item) is dict and set(item) == {"reference", "wave", "selected"} and item["wave"] in WAVES - {"noop"})
            receipt.reference(item["reference"])
    candidate = prepare(packet["inventory"])
    old, new = (selection(packet[k], candidate) for k in ("adopted", "selected"))
    if version in {2, 3} and check_chain_shape:
        require(len(chain) == int(old["pool"]) + int(old["provider"]) + len(old["grants"]) + len(old["apis"]))
        for item in chain:
            selection(item["selected"], candidate)
        ids = [root_reference(packet)["run_id"]] + [i["reference"]["run_id"] for i in chain]
        require(len(ids) == len(set(ids)))
    # Preserve all adopted flags and keys; adoption adds one object, noop none.
    require(not old["pool"] or new["pool"])
    require(not old["provider"] or new["provider"])
    require(set(old["grants"]) <= set(new["grants"]) and set(old["apis"]) <= set(new["apis"]))
    delta = {"pool": int(new["pool"]) - int(old["pool"]),
             "provider": int(new["provider"]) - int(old["provider"]),
             "grant": len(set(new["grants"]) - set(old["grants"])),
             "api": len(set(new["apis"]) - set(old["apis"]))}
    require(delta == {key: int(key == wave) for key in delta})
    if wave == "pool":
        require(old == {"pool": False, "provider": False, "grants": [], "apis": []})
    elif wave == "provider":
        require(old["pool"] and not old["grants"] and not old["apis"])
    elif wave == "grant":
        require(old["provider"] and not old["apis"])
    elif wave == "api":
        require(old["provider"] and set(old["grants"]) == set(candidate["runtime_wif_inventory"]["grants"]))
    elif wave == "noop":
        # Every valid nonempty runtime selection includes the pool. The same
        # authenticated adoption chain and fresh full-state checks still apply.
        require(old["pool"])
    candidate.update(own_runtime_wif_pool=new["pool"], own_runtime_wif_provider=new["provider"],
                     runtime_wif_grant_keys=sorted(new["grants"]), owned_api_keys=sorted(new["apis"]))
    return candidate


def input_json(raw):
    require(type(raw) is str and 0 < len(raw.encode()) <= MAX_PACKET_BYTES)
    return json.loads(raw, object_pairs_hook=unique_object, parse_constant=invalid_constant,
                      parse_float=finite_float)


def reviewed_fragment(value):
    # No binding may enter this channel, including an explicit null binding.
    require(type(value) is dict and type(value.get("schema_version")) is int
            and value["schema_version"] == 3 and set(value) == REVIEWED_KEYS)
    return value


def reviewed_input(raw):
    """Decode only after the runner has checked the existing private binding."""
    value = input_json(raw)
    if type(value) is dict and value.get("kind") == REVIEWED_CODEC_KIND:
        require(len(raw.encode("utf-8")) <= MAX_REVIEWED_WRAPPER_BYTES
                and set(value) == REVIEWED_WRAPPER_KEYS
                and type(value["schema_version"]) is int and value["schema_version"] == 1
                and value["encoding"] == "zlib+base64"
                and type(value["uncompressed_bytes"]) is int
                and 1 <= value["uncompressed_bytes"] <= MAX_PACKET_BYTES
                and type(value["sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", value["sha256"])
                and type(value["payload"]) is str)
        payload = value["payload"]
        compressed = base64.b64decode(payload, validate=True)
        require(base64.b64encode(compressed).decode("ascii") == payload)
        stream = zlib.decompressobj()
        try:
            body = stream.decompress(compressed, MAX_PACKET_BYTES + 1)
        except zlib.error:
            raise ValueError("ownership packet STOP") from None
        # Never flush or request another chunk: over-limit output, another
        # stream, an incomplete stream, and any trailing byte are all STOP.
        require(len(body) <= MAX_PACKET_BYTES and stream.eof
                and not stream.unused_data and not stream.unconsumed_tail
                and len(body) == value["uncompressed_bytes"]
                and hashlib.sha256(body).hexdigest() == value["sha256"])
        value = input_json(body.decode("utf-8", errors="strict"))
    return reviewed_fragment(value)


def pack_reviewed_input(raw):
    """Losslessly encode exact reviewed bytes; this does not authenticate them."""
    require(type(raw) is bytes and 0 < len(raw) <= MAX_PACKET_BYTES)
    reviewed_fragment(input_json(raw.decode("utf-8", errors="strict")))
    value = dict(schema_version=1, kind=REVIEWED_CODEC_KIND, encoding="zlib+base64",
                 uncompressed_bytes=len(raw), sha256=hashlib.sha256(raw).hexdigest(),
                 payload=base64.b64encode(zlib.compress(raw, level=9)).decode("ascii"))
    wrapper = json.dumps(value, separators=(",", ":"), allow_nan=False).encode("utf-8")
    require(len(wrapper) <= MAX_REVIEWED_WRAPPER_BYTES)
    return wrapper


def workflow_packet(env):
    """Join only inside the runner; never recover or replace a private binding."""
    packet = input_json(env.get("RUNTIME_OWNERSHIP_PACKET_JSON", ""))
    reviewed = env.get("RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON", "")
    require(type(reviewed) is str and len(reviewed.encode()) <= MAX_PACKET_BYTES)
    enabled = env.get("DEV_RUNTIME_OWNERSHIP_SPLIT_INPUT_ENABLED", "false")
    require(enabled in {"true", "false"})
    if reviewed:
        require(enabled == "true" and env.get("OWNERSHIP_ROOT_BASIS") == "current-state-bootstrap")
        sys.path.insert(0, str(Path(__file__).resolve().parents[3] / ".github/scripts"))
        import terraform_current_state_attestation as current_state
        import terraform_ownership_receipt as receipt
        # A full packet and a second packet are ambiguous even if their values
        # agree. The existing Secret must contain one exact binding envelope.
        require(type(packet) is dict)
        if set(packet) == {"schema_version", "receipt_binding"}:
            require(type(packet["schema_version"]) is int and packet["schema_version"] == 1)
            packet = dict(packet, attestation_type=current_state.TYPE)
        current_state.validate_binding_envelope(packet)
        receipt.check_current_binding(packet["receipt_binding"], env)
        binding = packet["receipt_binding"]
        packet = reviewed_input(reviewed)
        packet = dict(packet, receipt_binding=binding)
    # Apply the same bound to the exact bytes written after the in-memory join.
    require(len(json.dumps(packet, separators=(",", ":"), allow_nan=False).encode()) + 1 <= MAX_PACKET_BYTES)
    return packet


def prepare_workflow(env):
    require(env.get("MODE") in {"plan", "apply"} and env.get("TF_VAR_project_id") == "test-youtube-study-space")
    require(env.get("TF_VAR_manage_user_activity_history") == "true")
    packet = workflow_packet(env)
    require_root_basis(packet, env)
    candidate = validate_packet(packet, wave=env.get("OWNERSHIP_WAVE"), git_sha=env.get("GITHUB_SHA"))
    import terraform_ownership_receipt as receipt
    receipt.check_current_binding(packet["receipt_binding"], env)
    root = Path(env["RUNNER_TEMP"])
    repository = Path(__file__).resolve().parents[3]
    require(root.is_absolute() and root.is_dir() and not root.is_symlink())
    require(not root.resolve().is_relative_to(repository))
    created = []
    try:
        for name, value in (("runtime-ownership-packet.json", packet), ("runtime-ownership.tfvars.json", candidate)):
            target = root / name
            fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
            created.append(target)
            with os.fdopen(fd, "w", encoding="utf-8") as handle:
                json.dump(value, handle, separators=(",", ":"), allow_nan=False)
                handle.write("\n")
        # Verify the same private-file policy used by the consumer.
        require(private_json(str(root / "runtime-ownership.tfvars.json")) == candidate)
    except Exception:
        for target in created:
            target.unlink(missing_ok=True)
        raise


def main():
    try:
        prepare_workflow(dict(os.environ))
    except Exception:
        print("STOP: private ownership packet rejected; diagnostic suppressed.", file=sys.stderr)
        return 3
    print("Reviewed ownership inputs staged privately; no cloud operation or approval performed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
