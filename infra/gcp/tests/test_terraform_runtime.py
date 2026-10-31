#!/usr/bin/env python3
"""Offline contracts for private inputs and State reads used by normal operations."""
from contextlib import ExitStack, redirect_stdout
from copy import deepcopy
import base64
import json
import io
from pathlib import Path
import random
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import Mock, patch
import urllib.error
import zlib

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
import terraform_runtime as runtime

CANARY = "PRIVATE_RUNTIME_CANARY_NEVER_PUBLIC"


def environment(*, target="dev"):
    return {"NORMAL_TARGET": target, "STATE_BUCKET": "fixture-state-bucket",
            "STATE_ACCOUNT_ID": "111111111111"}


class OfflineTest(unittest.TestCase):
    def setUp(self):
        self.stack = ExitStack()
        self.addCleanup(self.stack.close)
        self.stack.enter_context(patch.object(runtime.subprocess, "run", side_effect=AssertionError("External process forbidden")))
        self.stack.enter_context(patch.object(runtime.urllib.request, "urlopen", side_effect=AssertionError("External HTTP forbidden")))
        self.temp = tempfile.TemporaryDirectory(prefix="terraform-runtime-contract-")
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)


class InputTransportTest(OfflineTest):
    def raw(self, target="dev"):
        return runtime.canonical({"schema_version": runtime.INPUT_SCHEMA,
            "tfvars": {"project_id": "test-youtube-study-space" if target == "dev" else "youtube-study-space"},
            "manifest": {"resources": []}, "plan_integrity_key": "b" * 64})

    def packed(self, compressed):
        return runtime.canonical({"schema_version": runtime.PACKED_INPUT_SCHEMA,
                                    "codec": "zlib+base64",
                                    "payload": base64.b64encode(compressed).decode()})

    def assert_rejected(self, wire):
        with self.assertRaises(ValueError) as rejected:
            runtime.decode_input_transport(wire)
        self.assertEqual(str(rejected.exception), "Terraform runtime contract rejected")

    def test_plain_backward_compatibility_and_packed_roundtrip_preserve_every_input_field(self):
        for target in ("dev", "prod"):
            raw = self.raw(target)
            expected = json.loads(raw)
            wire = runtime.encode_input_transport(raw)
            self.assertNotIn(b"\n", wire)
            self.assertNotIn(b"\r", wire)
            self.assertLessEqual(len(wire), runtime.INPUT_WIRE_BYTES)
            decoded = runtime.decode_input_transport(wire)
            self.assertEqual(decoded, expected)
            payload = json.loads(wire)["payload"]
            self.assertEqual(zlib.decompress(base64.b64decode(payload)), raw)

    def test_large_reviewed_input_roundtrips_at_decoded_limit_without_reformatting(self):
        value = json.loads(self.raw())
        value["tfvars"]["private_value"] = ""
        overhead = len(runtime.canonical(value))
        value["tfvars"]["private_value"] = "x" * (runtime.MAX_BYTES - overhead)
        raw = runtime.canonical(value)
        self.assertEqual(len(raw), runtime.MAX_BYTES)
        original = bytes(raw)
        wire = runtime.encode_input_transport(raw)
        self.assertEqual(raw, original)
        self.assertEqual(runtime.decode_input_transport(wire), value)
        self.assertEqual(zlib.decompress(base64.b64decode(json.loads(wire)["payload"])), original)
        self.assertLessEqual(len(wire), runtime.INPUT_WIRE_BYTES)

    def test_wire_limit_is_utf8_bytes_and_encoder_rejects_incompressible_oversize(self):
        raw = self.raw()
        boundary = raw + b" " * (runtime.INPUT_WIRE_BYTES - len(raw))
        self.assertEqual(runtime.decode_input_transport(boundary), json.loads(raw))
        self.assert_rejected(boundary + b" ")
        self.assert_rejected(b" " * runtime.INPUT_WIRE_BYTES + "é".encode())
        value = json.loads(raw)
        value["tfvars"]["private_value"] = base64.b64encode(random.Random(0).randbytes(60000)).decode()
        with self.assertRaises(ValueError):
            runtime.encode_input_transport(runtime.canonical(value))

    def test_packed_exact_schema_codec_one_line_and_canonical_base64_are_required(self):
        valid = json.loads(runtime.encode_input_transport(self.raw()))
        edits = (
            lambda value: value.update(extra=CANARY),
            lambda value: value.update(schema_version="unknown"),
            lambda value: value.update(codec="gzip+base64"),
            lambda value: value.update(payload=None),
            lambda value: value.update(payload=valid["payload"] + "\n"),
            lambda value: value.update(payload=valid["payload"] + "="),
            lambda value: value.update(payload="!" + valid["payload"]),
            lambda value: value.update(payload="é"),
        )
        for edit in edits:
            bad = deepcopy(valid)
            edit(bad)
            with self.subTest(edit=edit):
                self.assert_rejected(runtime.canonical(bad))
        self.assert_rejected(json.dumps(valid, indent=2).encode())
        self.assert_rejected(b'{"schema_version":"gcp-migration-inputs-packed-v1",'
                             b'"codec":"zlib+base64","codec":"zlib+base64","payload":""}')
        raw = self.raw()
        while len(zlib.compress(raw)) % 3 == 0:
            raw += b" "
        compressed = zlib.compress(raw)
        payload = base64.b64encode(compressed).decode()
        padding = len(payload) - len(payload.rstrip("="))
        alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
        pos = len(payload) - padding - 1
        altered = payload[:pos] + alphabet[alphabet.index(payload[pos]) + 1] + payload[pos + 1:]
        self.assertEqual(base64.b64decode(altered), compressed)
        self.assertEqual(zlib.decompress(base64.b64decode(altered)), raw)
        self.assert_rejected(runtime.canonical(valid | {"payload": altered}))

    def test_only_one_complete_zlib_stream_with_bounded_decoded_bytes_is_accepted(self):
        compressed = zlib.compress(self.raw())
        invalid = (b"", compressed[:-1], compressed + b"x", compressed + compressed,
                   zlib.compress(b""), zlib.compress(b"x" * (runtime.MAX_BYTES + 1)),
                   zlib.compress(b"\xff"), zlib.compress(self.raw().decode().encode("utf-16")))
        for candidate in invalid:
            with self.subTest(size=len(candidate)):
                self.assert_rejected(self.packed(candidate))

    def test_nested_wrapper_duplicate_nonfinite_overflow_and_unknown_decoded_shape_are_rejected(self):
        invalid = (runtime.encode_input_transport(self.raw()), b'{"duplicate":1,"duplicate":2}',
                   b'{"value":NaN}', b'{"value":Infinity}', b'{"value":1e999}',
                   b'[]', runtime.canonical(json.loads(self.raw()) | {"extra": CANARY}))
        for raw in invalid:
            with self.subTest(size=len(raw)):
                self.assert_rejected(self.packed(zlib.compress(raw)))
        value = json.loads(self.raw())
        value["tfvars"]["private_value"] = CANARY
        for literal in (b"NaN", b"Infinity", b"-Infinity", b"1e999"):
            raw = json.dumps(value).encode().replace(json.dumps(CANARY).encode(), literal)
            self.assert_rejected(raw)
            self.assert_rejected(self.packed(zlib.compress(raw)))
        deeply_nested = b"[" * 2000 + b"0" + b"]" * 2000
        self.assert_rejected(self.packed(zlib.compress(deeply_nested)))




class SnapshotTest(OfflineTest):
    def setup_snapshot(self, *, target="dev", absent=False):
        env = environment(target=target)
        key = f"youtube-study-space/{target}/terraform.tfstate"
        state = {"version": 4, "terraform_version": runtime.VERSION, "serial": 2,
                 "lineage": "12345678-1234-1234-1234-123456789abc", "resources": [], "outputs": {}}
        raw = runtime.canonical(state)
        head = {"ContentLength": len(raw), "VersionId": "fixture-version", "ETag": '"fixture-etag"', "ServerSideEncryption": "AES256"}
        settings = {"head": head, "get": deepcopy(head), "after_head": deepcopy(head), "raw": raw,
                    "listing_edit": None, "get_error": None}
        heads = [0]

        def aws(_env, service, action, *args):
            self.assertEqual(service, "s3api")
            if action == "list-objects-v2":
                prefix = args[args.index("--prefix") + 1]
                listing = {"Name": env["STATE_BUCKET"], "Prefix": prefix, "MaxKeys": 2,
                           "KeyCount": 0, "IsTruncated": False}
                if prefix == key and not absent:
                    listing.update(KeyCount=1, Contents=[{"Key": key, "ETag": head["ETag"], "Size": len(raw)}])
                if settings["listing_edit"]:
                    settings["listing_edit"](prefix, listing)
                return listing
            if action == "head-object":
                heads[0] += 1
                return deepcopy(settings["head"] if heads[0] == 1 else settings["after_head"])
            if action == "get-object":
                if settings["get_error"]:
                    raise settings["get_error"]
                Path(args[-1]).write_bytes(settings["raw"])
                return deepcopy(settings["get"])
            raise AssertionError("Unexpected offline AWS operation")

        return env, settings, Mock(side_effect=aws)

    def test_current_object_get_is_bounded_and_version_etag_head_get_head_must_match(self):
        env, settings, aws = self.setup_snapshot()
        with patch.object(runtime, "aws", aws):
            result = runtime.snapshot(env, self.directory, "before")
        self.assertEqual(result["head"], settings["head"])
        self.assertEqual(result["raw"], settings["raw"].decode())
        actions = [call.args[2] for call in aws.call_args_list]
        self.assertEqual(actions, ["list-objects-v2"] * 3 + ["head-object", "get-object", "head-object"])
        arguments = aws.call_args_list[4].args
        self.assertNotIn("--version-id", arguments)
        self.assertEqual(arguments[arguments.index("--range") + 1], f"bytes=0-{runtime.MAX_BYTES - 1}")
        self.assertEqual((self.directory / "before.tfstate").stat().st_mode & 0o777, 0o600)

    def test_missing_state_stops_without_initialization_in_both_environments(self):
        for target in ("dev", "prod"):
            env, _, aws = self.setup_snapshot(target=target, absent=True)
            with self.subTest(target=target), patch.object(runtime, "aws", aws):
                with self.assertRaises(ValueError):
                    runtime.snapshot(env, self.directory, "empty")
            self.assertFalse(any(call.args[2] == "get-object" for call in aws.call_args_list))

    def test_forbidden_and_unknown_aws_failures_never_become_absence(self):
        for error in (urllib.error.HTTPError("https://fixture.invalid", 403, CANARY, {}, io.BytesIO(b"dummy")),
                      subprocess.TimeoutExpired([CANARY], 1, output=CANARY), ValueError(CANARY)):
            if isinstance(error, urllib.error.HTTPError):
                self.addCleanup(error.close)
            aws = Mock(side_effect=error)
            with self.subTest(error=type(error)), patch.object(runtime, "aws", aws), self.assertRaises(type(error)):
                runtime.snapshot(environment(target="prod"), self.directory, "absent")
            self.assertEqual(aws.call_count, 1)

    def test_lock_workspace_or_malformed_absence_listing_stops_before_download(self):
        edits = (
            lambda prefix, value: value.update(KeyCount=1) if prefix.endswith(".tflock") else None,
            lambda prefix, value: value.update(KeyCount=1) if prefix.endswith("workspaces/") else None,
            lambda prefix, value: value.update(IsTruncated=True),
            lambda prefix, value: value.update(Contents=[{"Key": "unexpected"}]),
            lambda prefix, value: value.pop("IsTruncated"),
            lambda prefix, value: value.update(KeyCount=False),
        )
        for number, edit in enumerate(edits):
            env, settings, aws = self.setup_snapshot(target="prod", absent=True)
            settings["listing_edit"] = edit
            with self.subTest(case=number), patch.object(runtime, "aws", aws), self.assertRaises(ValueError):
                runtime.snapshot(env, self.directory, "invalid")
            self.assertFalse(any(call.args[2] == "get-object" for call in aws.call_args_list))

    def test_size_encryption_version_and_races_are_rejected(self):
        for field, bad in (("ContentLength", runtime.MAX_BYTES + 1), ("ContentLength", 0),
                           ("ContentLength", True), ("ServerSideEncryption", "aws:kms"), ("VersionId", "null")):
            env, settings, aws = self.setup_snapshot()
            settings["head"][field] = bad
            with self.subTest(field=field, bad=bad), patch.object(runtime, "aws", aws), self.assertRaises(ValueError):
                runtime.snapshot(env, self.directory, "invalid")
            self.assertFalse(any(call.args[2] == "get-object" for call in aws.call_args_list))
        for name, edit in (
            ("version", lambda settings: settings["get"].update(VersionId="changed")),
            ("etag", lambda settings: settings["get"].update(ETag="changed")),
            ("length", lambda settings: settings["get"].update(ContentLength=1)),
            ("body", lambda settings: settings.update(raw=b"{}")),
            ("head-race", lambda settings: settings["after_head"].update(VersionId="changed")),
        ):
            env, settings, aws = self.setup_snapshot()
            edit(settings)
            with self.subTest(case=name), patch.object(runtime, "aws", aws), self.assertRaises(ValueError):
                runtime.snapshot(env, self.directory, name)


class PrivateProcessTest(OfflineTest):
    def test_process_failure_and_oversized_output_are_bounded_without_retry(self):
        for result in (subprocess.CompletedProcess([], 1, CANARY.encode(), CANARY.encode()),
                       subprocess.CompletedProcess([], 0, b"x" * 129, b""),
                       subprocess.CompletedProcess([], 0, b"", b"x" * 9)):
            with self.subTest(code=result.returncode, size=len(result.stdout)), patch.object(runtime, "MAX_BYTES", 8), patch.object(runtime.subprocess, "run", return_value=result) as command, self.assertRaises(ValueError) as rejected:
                runtime.run(["fixture"])
            self.assertEqual(command.call_count, 1)
            self.assertNotIn(CANARY, str(rejected.exception))
        with patch.object(runtime.subprocess, "run", side_effect=subprocess.TimeoutExpired([CANARY], 1)) as command, self.assertRaises(subprocess.TimeoutExpired):
            runtime.run(["fixture"])
        self.assertEqual(command.call_count, 1)


    def test_raw_terraform_output_is_private_and_unknown_failure_text_is_suppressed(self):
        root = self.directory / "raw"
        root.mkdir(mode=0o700)
        result = subprocess.CompletedProcess([], 0, CANARY.encode(), CANARY.encode())
        stream = io.StringIO()
        with patch.object(runtime, "run", return_value=result) as command, redirect_stdout(stream):
            runtime.terraform(environment(), root, root, "fixture", ["plan"])
        self.assertEqual(stream.getvalue(), "")
        self.assertEqual(command.call_count, 1)
        for path in (root / "fixture.stdout", root / "fixture.stderr"):
            self.assertEqual(path.read_bytes(), CANARY.encode())
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)


if __name__ == '__main__':
    unittest.main()
