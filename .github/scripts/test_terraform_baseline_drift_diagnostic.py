#!/usr/bin/env python3
"""Dummy-only drift projections never admit a plan or publish dynamic data."""
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import terraform_baseline_drift_diagnostic as drift
import terraform_baseline_drift_fields as fields
import terraform_plan_diagnostic as legacy
import test_terraform_plan_diagnostic as fixtures

ROOT = Path(__file__).resolve().parents[2]
CANARY = fixtures.CANARY


class BaselineDriftTest(unittest.TestCase):
    def scope(self):
        return dict(env=fixtures.BaselineDiagnosticTest().selectors(), operation='plan', environment='dev', phase='post')

    def plan(self):
        graph = [dict(address=address, mode='managed', type=kind, provider_name=fields.PROVIDER,
            change={'actions': ['no-op']}) for _, address, kind in fields.ALIASES]
        return dict(complete=True, errored=False, resource_changes=graph, resource_drift=[])

    def entry(self, index=11, field='schema'):
        alias, address, resource_type = fields.ALIASES[index]
        return dict(address=address, mode='managed', type=resource_type, provider_name=fields.PROVIDER,
            change=dict(actions=['update'], before={field: CANARY}, after={field: CANARY + '_changed'},
                after_unknown={}, before_sensitive={}, after_sensitive={}))

    def value(self, plan):
        return dict(schema_version=2, outcome='STOP', stage='BASELINE_TOPIC', field='EMPTY_LIST',
            counts=legacy.counts(plan), drift=drift.project(plan))

    def output(self, value):
        text = drift.render(value, **self.scope())
        self.assertNotIn(CANARY, text)
        self.assertNotIn('google_bigquery_table', text)
        self.assertNotIn('module.', text)
        return text

    def test_static_map_matches_all_source_owned_addresses_types_and_locked_provider(self):
        from terraform_email_adoption_gate import CHANNEL, EXISTING
        from terraform_quota_create_gate import ADDRESSES
        from terraform_export_topic_gate import TOPIC
        from terraform_export_scheduler_gate import SCHEDULER
        from terraform_export_function_gate import FUNCTION
        from validate_user_activity_history_plan import TABLE
        addresses = EXISTING | {CHANNEL, TOPIC, SCHEDULER, FUNCTION, TABLE} | ADDRESSES
        self.assertEqual({a for _, a, _ in fields.ALIASES}, addresses)
        self.assertEqual(len(fields.ALIASES), 12)
        self.assertEqual(len({alias for alias, _, _ in fields.ALIASES}), 12)
        for alias, address, kind in fields.ALIASES:
            self.assertIn('.' + kind + '.', '.' + address)
            self.assertRegex(alias, r'^[A-Z][A-Z_0-9]*$')
            self.assertEqual(tuple(sorted(set(fields.FIELDS[kind]))), fields.FIELDS[kind])
            for field in fields.FIELDS[kind]: self.assertRegex(field, r'^[a-z][a-z_0-9]*$')
        self.assertEqual(fields.PROVIDER_VERSION, '8.5.0')
        self.assertEqual(fields.PROVIDER, 'registry.terraform.io/hashicorp/google')
        for root in ('dev', 'prod'):
            lock = (ROOT/'infra/gcp/environments'/root/'.terraform.lock.hcl').read_text()
            self.assertIn('version     = "8.5.0"', lock)
            self.assertIn('zh:' + fields.PROVIDER_ARCHIVE_SHA256, lock)
        self.assertEqual(fields.PUBLIC_SCHEMA_SHA256, 'b8da54b7d8799d9f668a1d2c81ff17d7e776fe1fd3d0bf9b95353959e994e347')
        normalized = dict(aliases=[dict(alias=alias, address=address, type=kind) for alias, address, kind in fields.ALIASES],
            resource_schemas={kind: dict(resource_schema_version=fields.SCHEMA_VERSIONS[kind], fields=list(names))
                for kind, names in fields.FIELDS.items()})
        raw = json.dumps(normalized, sort_keys=True, separators=(',', ':')).encode()
        self.assertEqual(hashlib.sha256(raw).hexdigest(), '75ec694c44aef5e4aca94784817f67d9eb1ac3fef2bb06c0723710d1b5a9968a')

    def test_all_twelve_targets_and_public_top_fields_are_complete(self):
        plan = self.plan()
        for index, (_, _, kind) in enumerate(fields.ALIASES):
            item = self.entry(index, fields.FIELDS[kind][0])
            item['change']['before'] = dict.fromkeys(fields.FIELDS[kind], CANARY)
            item['change']['after'] = dict.fromkeys(fields.FIELDS[kind], CANARY + '_changed')
            plan['resource_drift'].append(item)
        value = self.value(plan)
        self.assertEqual(len(value['drift']['items']), 12)
        text = self.output(value)
        for alias, _, kind in fields.ALIASES:
            line = next(line for line in text.splitlines() if 'role=' + alias + ' ' in line)
            self.assertIn('VALUE=' + ','.join(field.upper() for field in fields.FIELDS[kind]), line)
        self.assertEqual(len(text.splitlines()), 14)

    def test_presence_null_value_masks_replacement_and_nested_data_have_finite_codes(self):
        item = self.entry()
        item['change'].update(before={'schema': None, 'labels': {CANARY: 'before'}},
            after={'schema': CANARY, 'labels': {CANARY: 'after'}, 'description': None},
            after_unknown={'schema': True, 'labels': {CANARY: [False, True]}},
            before_sensitive={'schema': False}, after_sensitive={'schema': True},
            replace_paths=[['schema'], ['labels', CANARY]])
        plan = self.plan(); plan['resource_drift'] = [item]
        text = self.output(self.value(plan))
        for part in ('PRESENCE=DESCRIPTION', 'NULL=DESCRIPTION', 'VALUE=DESCRIPTION',
                     'UNKNOWN=LABELS,SCHEMA', 'SENSITIVITY=SCHEMA', 'REPLACE=LABELS,SCHEMA'):
            self.assertIn(part, text)
        self.assertNotIn('before', text); self.assertNotIn('after', text)

    def test_unknown_null_absent_and_nested_placeholders_never_invent_known_value_differences(self):
        for after, mask in (({'description': None}, True), ({}, True),
                            ({'description': {'dummy': None}}, {'dummy': True}),
                            ({'description': [None]}, [True])):
            with self.subTest(shape=type(mask).__name__):
                plan = self.plan(); item = self.entry()
                item['change'].update(before={'description': CANARY}, after=after,
                    after_unknown={'description': mask}, before_sensitive={'description': False},
                    after_sensitive={'description': True}, replace_paths=[['description']])
                plan['resource_drift'] = [item]
                text = self.output(self.value(plan))
                for kind in ('PRESENCE', 'NULL', 'VALUE'):
                    self.assertIn(' ' + kind + '=NONE', text)
                for kind in ('UNKNOWN', 'SENSITIVITY', 'REPLACE'):
                    self.assertIn(' ' + kind + '=DESCRIPTION', text)
        # An all-false nested mask does not suppress an actually known change.
        item['change'].update(after={'description': None}, after_unknown={'description': {'dummy': [False]}})
        text = self.output(self.value(plan))
        self.assertIn('NULL=DESCRIPTION', text); self.assertIn('VALUE=DESCRIPTION', text)
        self.assertIn('UNKNOWN=NONE', text)

    def test_missing_versus_null_and_typed_values_are_not_normalized(self):
        item = self.entry(); item['change'].update(before={'schema': None, 'description': 1}, after={'description': True})
        plan = self.plan(); plan['resource_drift'] = [item]
        text = self.output(self.value(plan))
        self.assertIn('PRESENCE=SCHEMA', text); self.assertIn('NULL=SCHEMA', text)
        self.assertIn('VALUE=DESCRIPTION,SCHEMA', text)
        item['change'].update(before=None, after={'schema': CANARY})
        self.assertIn('object_presence=AFTER', self.output(self.value(plan)))

    def test_supported_empty_and_nested_masks_and_replacement_remain_top_level_only(self):
        plan = self.plan(); item = self.entry(); plan['resource_drift'] = [item]
        item['change'].update(after_unknown={'schema': [False, {'dummy': True}], 'labels': []},
            before_sensitive=False, after_sensitive={'schema': True}, replace_paths=[['schema']])
        text = self.output(self.value(plan)); self.assertIn('UNKNOWN=SCHEMA', text)
        self.assertIn('SENSITIVITY=SCHEMA', text)
        self.assertIn('REPLACE=SCHEMA', text)
        for key in ('after_unknown', 'before_sensitive', 'after_sensitive'):
            item['change'][key] = True
            self.assertIsNone(drift.project(plan)); item['change'][key] = False
        self.assertIsNotNone(drift.project(plan))

    def test_full_unique_known_resource_change_graph_is_required_before_details(self):
        mutations = [lambda p: p['resource_changes'].pop(), lambda p: p['resource_changes'].append(self.entry()),
            lambda p: p['resource_changes'].__setitem__(0, copy.deepcopy(p['resource_changes'][1])),
            lambda p: p['resource_changes'][0].update(address=CANARY),
            lambda p: p['resource_changes'][0].update(previous_address=CANARY),
            lambda p: p['resource_changes'][0].update(mode='data'),
            lambda p: p['resource_changes'][0].update(type=CANARY),
            lambda p: p['resource_changes'][0].update(provider_name=CANARY)]
        for mutate in mutations:
            plan = self.plan(); plan['resource_drift'] = [self.entry()]; mutate(plan)
            self.assertIsNone(drift.project(plan)); self.assertNotIn('role=', self.output(self.value(plan)))

    def test_strict_json_depth_node_text_type_and_finite_budgets_reject_whole_projection(self):
        deep = True
        for _ in range(drift.MAX_DEPTH + 1): deep = [deep]
        values = [deep, [False] * (drift.MAX_NODES + 1), CANARY + 'x' * drift.MAX_TEXT_BYTES,
            {1: CANARY}, (CANARY,), float('nan'), float('inf')]
        for value in values:
            plan = self.plan(); item = self.entry(); item['change']['after']['schema'] = value
            plan['resource_drift'] = [self.entry(2), item]
            self.assertIsNone(drift.project(plan))
            self.assertNotIn('role=', self.output(self.value(plan)))

    def test_any_unknown_target_shape_or_public_field_rejects_whole_projection(self):
        changes = [lambda x: x.update(address=CANARY), lambda x: x.update(type=CANARY),
            lambda x: x.update(provider_name=CANARY), lambda x: x.update(mode='data'),
            lambda x: x.update(previous_address=CANARY), lambda x: x.update(deposed=CANARY),
            lambda x: x.update(name=CANARY), lambda x: x.update(module_address=CANARY),
            lambda x: x.update(index=True), lambda x: x.update(private=CANARY),
            lambda x: x['change']['before'].update({CANARY: True}),
            lambda x: x['change']['after'].update({CANARY: True}),
            lambda x: x['change'].update(actions=[CANARY]),
            lambda x: x['change'].update(before=CANARY), lambda x: x['change'].update(after=[]),
            lambda x: x['change'].update(after_unknown={CANARY: True}),
            lambda x: x['change'].update(after_unknown={'schema': CANARY}),
            lambda x: x['change'].update(before_sensitive={'schema': {CANARY: CANARY}}),
            lambda x: x['change'].update(after_sensitive=None),
            lambda x: x['change'].update(replace_paths=[[CANARY]]),
            lambda x: x['change'].update(replace_paths=[[]]),
            lambda x: x['change'].update(replace_paths=[['schema', True]]),
            lambda x: x['change'].update(importing={'id': CANARY}),
            lambda x: x.pop('change')]
        for mutate in changes:
            with self.subTest(case=changes.index(mutate)):
                plan = self.plan(); good = self.entry(2); bad = self.entry()
                mutate(bad); plan['resource_drift'] = [good, bad]
                self.assertIsNone(drift.project(plan))
                text = self.output(self.value(plan))
                self.assertIn('Baseline drift diagnostic: NOT_OBSERVED', text)
                self.assertNotIn('role=', text)

    def test_incomplete_duplicate_oversized_and_malformed_inputs_are_unobserved(self):
        cases = [None, {}, self.plan() | {'complete': False}, self.plan() | {'errored': True},
            self.plan() | {'deferred_changes': [{}]}, self.plan() | {'action_invocations': [{}]},
            self.plan() | {'resource_drift': None}, self.plan() | {'resource_drift': [self.entry()]*2},
            self.plan() | {'resource_drift': [self.entry()]*13}, self.plan() | {'resource_drift': [None]}]
        for plan in cases:
            self.assertIsNone(drift.project(plan))
        self.assertEqual(drift.project(self.plan()), {'map_version': fields.MAP_VERSION, 'items': []})

    def test_worst_case_all_fields_all_classes_twelve_targets_fit_existing_two_kib(self):
        items = [[index, 'replace', 3, *[format((1 << len(fields.FIELDS[kind])) - 1, 'x')]*len(drift.KINDS)]
            for index, (_, _, kind) in enumerate(fields.ALIASES)]
        value = dict(schema_version=2, outcome='STOP', stage=max(legacy.STAGES, key=len),
            field=max(legacy.FIELDS, key=len), counts=dict.fromkeys(legacy.COUNT_KEYS, 10000),
            drift={'map_version': fields.MAP_VERSION, 'items': items})
        value['counts']['drift'] = 12
        drift.validate(value)
        raw = (json.dumps(value, sort_keys=True, separators=(',', ':')) + '\n').encode()
        self.assertLessEqual(len(raw), legacy.MAX_BYTES)
        self.assertNotIn(CANARY.encode(), raw)

    def test_record_read_render_are_all_baseline_scoped_and_legacy_v1_is_accepted_only_inside_scope(self):
        plan = self.plan(); plan['resource_drift'] = [self.entry()]
        with tempfile.TemporaryDirectory() as directory:
            context = legacy.Context(); context.mark('BASELINE_TOPIC'); context.note_failure('EMPTY_LIST')
            drift.record(directory, plan, context, ValueError(CANARY), **self.scope())
            value = drift.read(directory, **self.scope())
            self.assertEqual(value['schema_version'], 2); self.output(value)
            for key in self.scope()['env']:
                bad = self.scope(); bad['env'][key] = 'wrong'
                for call in (lambda: drift.record(directory, plan, context, ValueError(CANARY), **bad),
                             lambda: drift.read(directory, **bad), lambda: drift.render(value, **bad)):
                    with self.assertRaises(ValueError): call()
            for key, wrong in [('operation', 'apply'), ('environment', 'prod'), ('phase', 'before')]:
                bad = self.scope() | {key: wrong}
                with self.assertRaises(ValueError): drift.read(directory, **bad)
                with self.assertRaises(ValueError): drift.render(value, **bad)
                with self.assertRaises(ValueError): drift.record(directory, plan, context, ValueError(CANARY), **bad)
            with self.assertRaises(ValueError): legacy.read(directory)
        with tempfile.TemporaryDirectory() as directory:
            legacy.record(directory, None, legacy.Context(), ValueError(CANARY))
            value = drift.read(directory, **self.scope())
            self.assertEqual(drift.render(value, **self.scope()), legacy.render(legacy.read(directory)))

    def test_strict_v2_validator_rejects_unknown_enums_fields_bits_and_partial_success(self):
        plan = self.plan(); plan['resource_drift'] = [self.entry()]
        good = self.value(plan)
        changes = [lambda x: x.update(private=CANARY), lambda x: x.update(outcome='PASS'),
            lambda x: x.update(schema_version=True), lambda x: x['drift'].update(map_version=CANARY),
            lambda x: x['drift']['items'][0].__setitem__(0, 12),
            lambda x: x['drift']['items'][0].__setitem__(0, True),
            lambda x: x['drift']['items'][0].__setitem__(1, CANARY),
            lambda x: x['drift']['items'][0].__setitem__(2, 4),
            lambda x: x['drift']['items'][0].__setitem__(3, CANARY),
            lambda x: x['drift']['items'][0].__setitem__(3, '00'),
            lambda x: x['drift']['items'][0].__setitem__(3, 'f'*20),
            lambda x: x['drift']['items'].append(copy.deepcopy(x['drift']['items'][0])),
            lambda x: x['counts'].__setitem__('drift', 2), lambda x: x.update(counts=None)]
        for mutate in changes:
            bad = copy.deepcopy(good); mutate(bad)
            with self.assertRaises(ValueError): drift.render(bad, **self.scope())

    def test_cli_file_bounds_unsafe_files_and_false_scope_never_expose_raw_contents(self):
        plan = self.plan(); plan['resource_drift'] = [self.entry()]
        for kind in ('good', 'partial', 'oversized', 'unknown', 'mode', 'link', 'hardlink', 'duplicate', 'nan'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                path = Path(directory)/legacy.FILE
                raw = json.dumps(self.value(plan)).encode()
                if kind == 'partial': raw = ('{"' + CANARY).encode()
                if kind == 'oversized': raw = b'x'*2049
                if kind == 'unknown': raw = json.dumps(self.value(plan) | {'message': CANARY}).encode()
                if kind == 'duplicate': raw = b'{"schema_version":2,"schema_version":2}'
                if kind == 'nan': raw = b'{"counts":NaN}'
                path.write_bytes(raw); path.chmod(0o600)
                if kind == 'mode': path.chmod(0o644)
                if kind == 'link': path.rename(Path(directory)/'target'); path.symlink_to(Path(directory)/'target')
                if kind == 'hardlink': os.link(path, Path(directory)/'other')
                with patch.dict(os.environ, self.scope()['env'] | {'RUNNER_TEMP': directory}, clear=True), patch('sys.stdout', io.StringIO()) as out:
                    result = drift.main()
                self.assertEqual(result, 0 if kind == 'good' else 3)
                self.assertNotIn(CANARY, out.getvalue())
                if kind != 'good': self.assertIn('Baseline drift diagnostic: NOT_OBSERVED', out.getvalue())
                with patch.dict(os.environ, {'RUNNER_TEMP': directory}, clear=True), patch('sys.stdout', io.StringIO()) as out:
                    self.assertEqual(drift.main(), 3)
                self.assertEqual(out.getvalue(), '')
        with tempfile.TemporaryDirectory() as directory:
            drift.record(directory, plan, legacy.Context(), ValueError(CANARY), **self.scope())
            with self.assertRaises(FileExistsError): drift.record(directory, plan, legacy.Context(), ValueError(CANARY), **self.scope())

    def test_real_entrypoint_drift_failure_preserves_rc_guards_and_no_approval(self):
        item = fixtures.BaselineDiagnosticTest()
        plan, metadata = item.source()
        types = {address: kind for _, address, kind in fields.ALIASES}
        for resource in plan['resource_changes']:
            # Older validator fixtures omitted public identity keys that real
            # Terraform JSON supplies. Populate them for the strict projection.
            resource.setdefault('type', types[resource['address']])
            resource.setdefault('provider_name', fields.PROVIDER)
        drift_item = self.entry(); plan['resource_drift'] = [drift_item]
        result = item.execute(plan, metadata)
        self.assertEqual(result['rc'], 3); self.assertIsNone(result['json']); self.assertIsNone(result['markdown'])
        value = result['diagnostic']
        self.assertEqual((value['stage'], value['field']), ('BASELINE_TOPIC', 'EMPTY_LIST'))
        self.assertEqual((value['counts']['no-op'], value['counts']['drift']), (12, 1))
        self.assertIn('role=USER_ACTIVITY_HISTORY', self.output(value))
        self.assertIn('VALUE=SCHEMA', self.output(value))
        # Synthetic canary only: this does not fill in actual run881's target.

    def test_legacy_runtime_module_bytes_and_guard_sources_remain_pinned(self):
        # Only the unreachable Attestation post-check renderer was removed.
        # The remaining diagnostic AST and all guard sources are unchanged.
        pins = {
            '.github/scripts/terraform_plan_diagnostic.py': '90956dabe89c3c75d21076adfc1fa30576b5f166391cbbd1b3fe3a23c7c587cc',
            '.github/scripts/terraform_export_function_gate.py': 'f1dda0f59fde3f2184542fe6d880357ab51bcd4154a1a40486b39787da2443bf',
            '.github/scripts/terraform_export_scheduler_gate.py': '9fbf44c3c26b866595e61425dc82bb57ff12aa5f913b95aa912bc8f2081f4e2f',
            '.github/scripts/terraform_export_topic_gate.py': 'd325f9f58c95793ef9b1e1a6a18a568f6482b4cfa3a66dc8ad8595e769c4b6bd',
            'infra/gcp/scripts/validate_user_activity_history_plan.py': '444658265a9b101a394c8435443ea34f97b4bf40fead9b51a6bf59d9d43b3fbd'}
        for name, expected in pins.items():
            self.assertEqual(hashlib.sha256((ROOT/name).read_bytes()).hexdigest(), expected)


if __name__ == '__main__':
    unittest.main()
