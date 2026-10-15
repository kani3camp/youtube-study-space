#!/usr/bin/env python3
"""Dummy-only lossless transport and unchanged protected admission contracts."""
from __future__ import annotations

import base64
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch
import zlib

sys.path.insert(0, str(Path(__file__).parent))
import test_runtime_ownership_package as fixtures

codec = fixtures.packet_module
import pack_runtime_reviewed_inputs as packer


def compact(value):
    return json.dumps(value, separators=(',', ':'), ensure_ascii=False, allow_nan=False).encode('utf-8')


def fragment():
    with tempfile.TemporaryDirectory() as directory:
        return fixtures.PackageContracts().split_env(directory)[3]


def wrapper(body):
    return dict(schema_version=1, kind=codec.REVIEWED_CODEC_KIND, encoding='zlib+base64',
                uncompressed_bytes=len(body), sha256=hashlib.sha256(body).hexdigest(),
                payload=base64.b64encode(zlib.compress(body)).decode('ascii'))


class CodecContracts(unittest.TestCase):
    def setUp(self):
        self.value = fragment()
        self.raw = compact(self.value)
        self.wrapped = wrapper(self.raw)

    def decode(self, value):
        return codec.reviewed_input(compact(value).decode('utf-8'))

    def test_exact_bytes_hash_unicode_and_plain_compatibility(self):
        self.value['inventory']['pool']['description'] = '非公開テスト'
        for raw in (compact(self.value), json.dumps(self.value, indent=2, ensure_ascii=False).encode() + b'\n'):
            encoded = codec.pack_reviewed_input(raw)
            metadata = json.loads(encoded)
            self.assertFalse(encoded.endswith((b'\r', b'\n')))
            self.assertEqual(zlib.decompress(base64.b64decode(metadata['payload'])), raw)
            self.assertEqual(metadata['sha256'], hashlib.sha256(raw).hexdigest())
            self.assertEqual(metadata['uncompressed_bytes'], len(raw))
            self.assertEqual(codec.reviewed_input(encoded.decode()), self.value)
            self.assertEqual(codec.reviewed_input(raw.decode()), self.value)

    def test_wrapper_exact_types_keys_lengths_and_digest(self):
        mutations = [
            lambda v: v.update(schema_version=True), lambda v: v.update(schema_version=3),
            lambda v: v.update(kind='unknown'), lambda v: v.update(encoding='gzip+base64'),
            lambda v: v.update(uncompressed_bytes=True), lambda v: v.update(uncompressed_bytes=0),
            lambda v: v.update(uncompressed_bytes=codec.MAX_PACKET_BYTES + 1),
            lambda v: v.update(uncompressed_bytes=len(self.raw) + 1),
            lambda v: v.update(sha256='A' * 64), lambda v: v.update(sha256='0' * 64),
            lambda v: v.update(sha256=None), lambda v: v.update(payload=None),
            lambda v: v.update(extra='DUMMY_PRIVATE'), lambda v: v.pop('encoding'),
            lambda v: v.update(receipt_binding=None),
        ]
        for index, mutate in enumerate(mutations):
            with self.subTest(case=index), self.assertRaises((ValueError, TypeError)):
                value = copy.deepcopy(self.wrapped)
                mutate(value)
                self.decode(value)

    def test_canonical_base64_no_whitespace_and_no_alternate_padding_bits(self):
        for payload in ('', self.wrapped['payload'] + '\n', ' ' + self.wrapped['payload'],
                        self.wrapped['payload'] + '=', '====', '!!!!', 'é', 'eB=='):
            with self.subTest(payload=payload), self.assertRaises(ValueError):
                self.decode(dict(self.wrapped, payload=payload))

    def test_zlib_eof_trailing_concatenation_raw_deflate_and_gzip_rejected(self):
        data = base64.b64decode(self.wrapped['payload'])
        alternatives = [data[:-1], data + b'\0', data + zlib.compress(self.raw), b'',
                        b'not-zlib', zlib.compress(self.raw)[2:-4]]
        compressor = zlib.compressobj(wbits=31)
        alternatives.append(compressor.compress(self.raw) + compressor.flush())
        for alternative in alternatives:
            with self.subTest(case=len(alternative)), self.assertRaises(ValueError):
                self.decode(dict(self.wrapped, payload=base64.b64encode(alternative).decode()))

    def test_bomb_is_bounded_and_does_not_accept_claimed_small_length(self):
        compressed = zlib.compress(b'x' * (codec.MAX_PACKET_BYTES + 1000), level=9)
        bomb = dict(self.wrapped, uncompressed_bytes=1, payload=base64.b64encode(compressed).decode())
        with self.assertRaises(ValueError):
            self.decode(bomb)
        with self.assertRaises(ValueError):
            codec.pack_reviewed_input(b'x' * (codec.MAX_PACKET_BYTES + 1))

    def test_decompressed_json_is_strict_utf8_unique_finite_and_fragment_only(self):
        bodies = [b'\xff', b'\xef\xbb\xbf{}', b'{} {}', b'[]', b'null',
                  self.raw.replace(b'"schema_version":3', b'"schema_version":3,"schema_version":3'),
                  self.raw.replace(b'"schema_version":3', b'"schema_version":NaN'),
                  self.raw.replace(b'"schema_version":3', b'"schema_version":Infinity'),
                  self.raw.replace(b'"schema_version":3', b'"schema_version":1e999')]
        for key, value in (('receipt_binding', None), ('receipt_binding', {'nonce': 'DUMMY_PRIVATE'}),
                           ('schema_version', True), ('schema_version', 2), ('extra', 'DUMMY_PRIVATE')):
            bodies.append(compact(dict(self.value, **{key: value})))
        for index, body in enumerate(bodies):
            with self.subTest(case=index), self.assertRaises((ValueError, TypeError)):
                self.decode(wrapper(body))
            with self.subTest(encode=index), self.assertRaises((ValueError, TypeError)):
                codec.pack_reviewed_input(body)

    def test_wrapper_json_duplicate_nonfinite_and_utf8_byte_ceiling(self):
        raw = compact(self.wrapped).decode()
        for bad in (raw.replace('"schema_version":1', '"schema_version":1,"schema_version":1'),
                    raw.replace('"schema_version":1', '"schema_version":NaN'),
                    raw.replace('"schema_version":1', '"schema_version":1e999')):
            with self.assertRaises(ValueError):
                codec.reviewed_input(bad)
        at_limit = raw + ' ' * (codec.MAX_REVIEWED_WRAPPER_BYTES - len(raw.encode()))
        self.assertEqual(codec.reviewed_input(at_limit), self.value)
        with self.assertRaises(ValueError):
            codec.reviewed_input(at_limit + ' ')
        encoded_length = len(codec.pack_reviewed_input(self.raw))
        with patch.object(codec, 'MAX_REVIEWED_WRAPPER_BYTES', encoded_length - 1), self.assertRaises(ValueError):
            codec.pack_reviewed_input(self.raw)
        # Plain fragments retain the existing 4 MiB input allowance.
        self.assertEqual(codec.reviewed_input(self.raw.decode() + ' ' * 48000), self.value)

    def test_incompressible_large_wrapper_is_not_emitted(self):
        # Deterministic bytes, not random credentials or private fixtures.
        noise = ''.join(hashlib.sha256(str(i).encode()).hexdigest() for i in range(2000))
        value = copy.deepcopy(self.value)
        value['inventory']['pool']['description'] = noise
        with self.assertRaises(ValueError):
            codec.pack_reviewed_input(compact(value))


class ProtectedJoinContracts(unittest.TestCase):
    def setUp(self):
        self.project = patch.object(fixtures.preparer, 'PROJECT', fixtures.PROJECT)
        self.project.start()
        self.addCleanup(self.project.stop)

    def env(self, directory):
        env, value, envelope, reviewed = fixtures.PackageContracts().split_env(directory)
        env['RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON'] = codec.pack_reviewed_input(compact(reviewed)).decode()
        return env, value, envelope, reviewed

    def test_join_preserves_binding_and_stages_through_existing_full_validator(self):
        for mode in ('plan', 'apply'):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                env, value, _, _ = self.env(directory)
                env['MODE'] = mode
                original_binding = copy.deepcopy(value['receipt_binding'])
                with patch.object(codec, 'validate_packet', side_effect=lambda p, **kw:
                                  fixtures.packet_module_validate(p, now=fixtures.NOW, **kw)) as validate:
                    codec.prepare_workflow(env)
                stored = json.loads((Path(directory) / 'runtime-ownership-packet.json').read_text())
                self.assertEqual(stored, value)
                self.assertEqual(stored['receipt_binding'], original_binding)
                self.assertEqual(validate.call_count, 1)
                self.assertEqual({p.name for p in Path(directory).iterdir()},
                                 {'runtime-ownership-packet.json', 'runtime-ownership.tfvars.json'})

    def test_closed_gate_wrong_root_full_packet_or_bad_binding_never_decompresses(self):
        mutations = [lambda e, v, b: e.update(DEV_RUNTIME_OWNERSHIP_SPLIT_INPUT_ENABLED='false'),
                     lambda e, v, b: e.pop('DEV_RUNTIME_OWNERSHIP_SPLIT_INPUT_ENABLED'),
                     lambda e, v, b: e.update(OWNERSHIP_ROOT_BASIS='original-history-import'),
                     lambda e, v, b: e.update(RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(v)),
                     lambda e, v, b: e.update(STATE_BUCKET='wrong-private-bucket'),
                     lambda e, v, b: e.update(RUNTIME_OWNERSHIP_PACKET_JSON=e['RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON'])]
        for index, mutate in enumerate(mutations):
            with self.subTest(case=index), tempfile.TemporaryDirectory() as directory:
                env, value, binding, _ = self.env(directory)
                mutate(env, value, binding)
                with patch.object(codec.zlib, 'decompressobj') as decompress, self.assertRaises(ValueError):
                    codec.prepare_workflow(env)
                decompress.assert_not_called()
                self.assertEqual(list(Path(directory).iterdir()), [])

    def test_compressed_full_packet_is_never_accepted_on_existing_channel(self):
        with tempfile.TemporaryDirectory() as directory:
            env, value, _, _ = self.env(directory)
            env['RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON'] = ''
            env['RUNTIME_OWNERSHIP_PACKET_JSON'] = compact(wrapper(compact(value))).decode()
            with patch.object(codec.zlib, 'decompressobj') as decompress, self.assertRaises(ValueError):
                codec.prepare_workflow(env)
            decompress.assert_not_called()
            self.assertEqual(list(Path(directory).iterdir()), [])

    def test_codec_does_not_bypass_scope_freshness_reference_or_inventory_validation(self):
        mutations = [lambda p: p.update(git_sha='b' * 40),
                     lambda p: p.update(expires_utc='2026-10-10T03:59:59Z'),
                     lambda p: p['selected'].update(provider=True),
                     lambda p: p['current_state_attestation'].update(transport='artifact-v1'),
                     lambda p: p['inventory']['pool_attestation_rules'].update(nextPageToken='remaining'),
                     lambda p: p['inventory']['runtime_grants']['grant-01'].update(member='wrong-member')]
        for index, mutate in enumerate(mutations):
            with self.subTest(case=index), tempfile.TemporaryDirectory() as directory:
                env, _, _, reviewed = self.env(directory)
                mutate(reviewed)
                env['RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON'] = codec.pack_reviewed_input(compact(reviewed)).decode()
                with patch.object(codec, 'validate_packet', side_effect=lambda p, **kw:
                                  fixtures.packet_module_validate(p, now=fixtures.NOW, **kw)), self.assertRaises(ValueError):
                    codec.prepare_workflow(env)
                self.assertEqual(list(Path(directory).iterdir()), [])

    def test_private_rejection_never_prints_payload_or_binding(self):
        with tempfile.TemporaryDirectory() as directory:
            env, _, _, _ = self.env(directory)
            bad = dict(wrapper(b'DUMMY_PRIVATE_PAYLOAD'), payload='DUMMY_PRIVATE_PAYLOAD')
            env['RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON'] = compact(bad).decode()
            with patch.dict(codec.os.environ, env, clear=True), patch('sys.stdout', io.StringIO()) as out, \
                 patch('sys.stderr', io.StringIO()) as err:
                self.assertEqual(codec.main(), 3)
            for secret in ('DUMMY_PRIVATE', 'nonce', 'dummy-state-bucket', env['RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON']):
                self.assertNotIn(secret, out.getvalue() + err.getvalue())

    def test_joined_serialization_still_has_the_original_size_limit(self):
        with tempfile.TemporaryDirectory() as directory:
            env, _, _, reviewed = self.env(directory)
            with patch.object(codec, 'MAX_PACKET_BYTES', len(compact(reviewed))), self.assertRaises(ValueError):
                codec.prepare_workflow(env)
            self.assertEqual(list(Path(directory).iterdir()), [])


class OfflinePackerContracts(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / 'reviewed-fragment.json'
        self.output = self.root / 'reviewed-wrapper.json'
        self.value = fragment()
        self.raw = compact(self.value)
        self.source.write_bytes(self.raw)
        self.source.chmod(0o600)
        from test_terraform_current_state_attestation import bootstrap_catalog
        catalog = self.root / 'dummy-catalog.json'
        catalog.write_bytes(compact(bootstrap_catalog()))
        for context in (patch.object(packer, 'CATALOG', catalog),
                        patch.object(packer, 'now', return_value=fixtures.NOW),
                        patch.object(fixtures.preparer, 'PROJECT', fixtures.PROJECT),
                        patch('socket.socket', side_effect=AssertionError('network forbidden')),
                        patch('subprocess.run', side_effect=AssertionError('child forbidden'))):
            context.start()
            self.addCleanup(context.stop)

    def args(self, write=False):
        args = ['--fragment', str(self.source), '--fragment-sha256', hashlib.sha256(self.source.read_bytes()).hexdigest(),
                '--git-sha', fixtures.SHA, '--issuer-sha', self.value['current_state_attestation']['source_sha']]
        return args + (['--pack', '--output', str(self.output)] if write else [])

    def run_private(self, args, expected):
        with patch('sys.stdout', io.StringIO()) as out, patch('sys.stderr', io.StringIO()) as err:
            self.assertEqual(packer.main(args), expected)
        text = out.getvalue() + err.getvalue()
        for private in ('DUMMY_PRIVATE', str(self.source), str(self.output), 'dummy-state-bucket', 'attributeMapping'):
            self.assertNotIn(private, text)

    def test_default_off_then_explicit_exclusive_private_output_is_lossless(self):
        self.run_private(self.args(), 0)
        self.assertFalse(self.output.exists())
        self.run_private(self.args(write=True), 0)
        encoded = self.output.read_bytes()
        self.assertEqual(self.output.stat().st_mode & 0o777, 0o600)
        self.assertFalse(encoded.endswith((b'\r', b'\n')))
        self.assertEqual(codec.reviewed_input(encoded.decode()), self.value)
        metadata = json.loads(encoded)
        self.assertEqual(zlib.decompress(base64.b64decode(metadata['payload'])), self.raw)
        self.assertEqual(metadata['sha256'], hashlib.sha256(self.raw).hexdigest())
        self.run_private(self.args(write=True), 3)
        self.assertEqual(self.output.read_bytes(), encoded)

    def test_bad_private_arguments_are_suppressed_and_create_nothing(self):
        for args in ([], ['--unknown-DUMMY_PRIVATE', 'DUMMY_PRIVATE_VALUE'],
                     self.args() + ['--output', str(self.output)], self.args() + ['--pack']):
            with self.subTest(args=len(args)):
                self.run_private(args, 3)
                self.assertFalse(self.output.exists())

    def test_input_and_output_private_modes_links_and_checkout_boundaries(self):
        self.source.chmod(0o644)
        self.run_private(self.args(write=True), 3)
        self.source.chmod(0o600)
        self.root.chmod(0o755)
        self.run_private(self.args(write=True), 3)
        self.root.chmod(0o700)
        link = self.root / 'link'
        os.link(self.source, link)
        self.run_private(self.args(write=True), 3)
        link.unlink()
        self.source.rename(link)
        self.source.symlink_to(link)
        self.run_private(self.args(write=True), 3)
        self.source.unlink()
        link.rename(self.source)
        marker = self.root / '.git'
        marker.write_text('gitdir: DUMMY_PRIVATE\n')
        self.run_private(self.args(write=True), 3)
        marker.unlink()
        marker.mkdir()
        (marker / 'HEAD').write_text('ref: refs/heads/dummy\n')
        self.run_private(self.args(write=True), 3)
        (marker / 'HEAD').unlink()
        marker.rmdir()
        self.output.symlink_to(self.source)
        self.run_private(self.args(write=True), 3)
        self.assertEqual(self.source.read_bytes(), self.raw)

    def test_header_scope_inventory_root_and_age_reject_without_output(self):
        mutations = [lambda v: v.update(receipt_binding=None), lambda v: v.update(schema_version=True),
                     lambda v: v.update(wave='provider'), lambda v: v.update(git_sha='b' * 40),
                     lambda v: v.update(expires_utc='2026-10-11T05:30:00Z'),
                     lambda v: v.update(expires_utc='2026-10-10T03:59:59Z'),
                     lambda v: v['selected'].update(provider=True),
                     lambda v: v['adopted'].update(pool=True),
                     lambda v: v.update(adoption_receipts=[{}]),
                     lambda v: v['current_state_attestation'].update(source_sha='b' * 40),
                     lambda v: v['current_state_attestation'].update(transport='artifact-v1'),
                     lambda v: v['inventory']['pool_attestation_rules'].update(nextPageToken='remaining')]
        for index, mutate in enumerate(mutations):
            with self.subTest(case=index):
                value = copy.deepcopy(self.value)
                mutate(value)
                self.source.write_bytes(compact(value))
                self.run_private(self.args(write=True), 3)
                self.assertFalse(self.output.exists())

    def test_hash_issuer_and_unapproved_catalog_reject_without_output(self):
        for flag, wrong in (('--fragment-sha256', '0' * 64), ('--git-sha', 'b' * 40), ('--issuer-sha', 'b' * 40)):
            args = self.args(write=True)
            args[args.index(flag) + 1] = wrong
            self.run_private(args, 3)
        packer.CATALOG.write_bytes(compact(dict(schema_version=1, issuers={})))
        self.run_private(self.args(write=True), 3)
        self.assertFalse(self.output.exists())

    def test_pretty_input_is_preserved_and_never_silently_reformatted(self):
        original = json.dumps(self.value, indent=2).encode() + b'\r\n'
        self.source.write_bytes(original)
        self.run_private(self.args(write=True), 0)
        metadata = json.loads(self.output.read_bytes())
        self.assertEqual(metadata['sha256'], hashlib.sha256(original).hexdigest())
        self.assertEqual(zlib.decompress(base64.b64decode(metadata['payload'])), original)
        self.assertEqual(self.source.read_bytes(), original)

    def test_partial_write_failure_removes_only_new_output(self):
        with patch.object(packer.os, 'fsync', side_effect=OSError('DUMMY_PRIVATE')):
            self.run_private(self.args(write=True), 3)
        self.assertFalse(self.output.exists())
        self.assertEqual(self.source.read_bytes(), self.raw)


if __name__ == '__main__':
    unittest.main()
