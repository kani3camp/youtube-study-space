#!/usr/bin/env python3
"""Offline, SHA-bound private inputs for a single cumulative ownership wave.

Version 1 remains an offline contract. Protected staging requires version 2 or 3;
GitHub provenance is verified separately before cloud credentials. Gates remain
closed; apply uses the same private packet and separate protected approval.
"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import re
import sys

from prepare_runtime_ownership import prepare, require, unique_object
from prepare_user_activity_history_adoption import private_json

WAVES = {"pool", "provider", "grant", "api", "noop"}
SELECTION_KEYS = {"pool", "provider", "grants", "apis"}
ROOT_BASES = {2: "original-history-import", 3: "current-state-bootstrap"}
MAX_PACKET_BYTES = 4 * 1024 * 1024
BASE_KEYS = {"schema_version", "git_sha", "wave", "reviewed_utc", "expires_utc", "inventory", "adopted", "selected"}


def root_reference(packet):
    return packet["current_state_attestation" if packet["schema_version"] == 3 else "history_receipt"]


def require_root_basis(packet, env):
    require(packet.get("schema_version") in ROOT_BASES
            and env.get("OWNERSHIP_ROOT_BASIS", "original-history-import") == ROOT_BASES[packet["schema_version"]])


def invalid_constant(_):
    raise ValueError("ownership packet STOP")


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
    return json.loads(raw, object_pairs_hook=unique_object, parse_constant=invalid_constant)


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
        packet = input_json(reviewed)
        require(type(packet) is dict and type(packet.get("schema_version")) is int
                and packet["schema_version"] == 3
                and set(packet) == BASE_KEYS | {"current_state_attestation", "adoption_receipts"})
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
