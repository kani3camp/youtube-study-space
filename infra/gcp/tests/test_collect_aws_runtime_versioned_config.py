#!/usr/bin/env python3
"""Dummy-only boundaries for historical ECS/current version provenance."""
from __future__ import annotations

import contextlib
import copy
from datetime import datetime, timedelta, timezone
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / 'infra/gcp/scripts/collect_aws_runtime_versioned_config.py'
spec = importlib.util.spec_from_file_location('collect_aws_runtime_versioned_config', SCRIPT)
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)
collector.PROJECT = 'example-study-space-dev'
POINT = datetime(2026, 1, 2, 0, 30, tzinfo=timezone.utc)


def capture():
    return {
        'schema_version': 1, 'capture_type': 'aws-runtime-task-metadata-v1',
        'captured_at': '2025-12-01T01:02:03.123456+00:00',
        'environment': 'dev', 'aws_region': 'ap-northeast-1',
        'task_definition_arn': 'arn:aws:ecs:ap-northeast-1:222222222222:task-definition/DummyDailyBatch:7',
        'task_role_arn': 'arn:aws:iam::222222222222:role/DummyRuntimeTaskRole',
        'execution_role_arn': 'arn:aws:iam::222222222222:role/DummyExecutionRole',
        'runtime_auth': {'GOOGLE_CLOUD_PROJECT': 'example-study-space-dev',
            'GCP_WIF_AUDIENCE': '//iam.googleapis.com/projects/111111111111/locations/global/'
                'workloadIdentityPools/aws-runtime/providers/aws-provider',
            'GCP_WIF_SERVICE_ACCOUNT_EMAIL': 'example-study-space-dev@appspot.gserviceaccount.com'},
        'stack_reference_match': True, 'three_auth_env_match': True,
        'metadata_reads': 4, 'cloud_mutations': 0, 'readiness_attested': False,
    }


def raw_capture():
    # Deliberate formatting proves this is the reviewed original bytes digest,
    # not a new hash of the nested, reformatted historical body.
    return json.dumps(capture(), indent=4).encode() + b'\n'


def request():
    return {'schema_version': 1, 'kind': 'private-aws-runtime-versioned-config-request',
        'environment': 'dev', 'aws_account_id': '222222222222',
        'aws_region': 'ap-northeast-1', 'stack_name': 'AwsCdkStack',
        'readonly_role_arn': 'arn:aws:iam::222222222222:role/DummyReadOnlyRole',
        'readonly_session_name': 'dummy-reviewed-session',
        'owner_capture_sha256': hashlib.sha256(raw_capture()).hexdigest(),
        'issued_utc': '2026-01-02T00:00:00Z', 'expires_utc': '2026-01-02T01:00:00Z'}


def responses():
    source = capture()
    return [
        {'account': '222222222222',
         'arn': 'arn:aws:sts::222222222222:assumed-role/DummyReadOnlyRole/dummy-reviewed-session'},
        [{'name': 'AwsCdkStack', 'status': 'UPDATE_COMPLETE',
          'outputs': [{'OutputKey': 'DailyBatchTaskDefinitionArn',
                       'OutputValue': source['task_definition_arn']}],
          'parameters': [{'key': key, 'value': source['runtime_auth'][auth]}
                         for key, auth in collector.PARAMETERS.items()]}],
        [source['task_definition_arn']],
        *[{'name': name, 'arn': 'arn:aws:lambda:ap-northeast-1:222222222222:function:' + name,
           'role': 'arn:aws:iam::222222222222:role/DummyLambda-' + str(i),
           'auth': copy.deepcopy(source['runtime_auth']), 'version': '$LATEST',
           'revision_id': 'dummy-revision-' + str(i), 'last_modified': '2026-01-01T12:34:56.000+0000'}
          for i, name in enumerate(collector.CALLERS)],
    ]


class VersionedRead(unittest.TestCase):
    def run_capture(self, values=None, clock=lambda: POINT):
        values = iter(responses() if values is None else values)
        calls = []

        def read(call):
            calls.append(call)
            return next(values)

        result = collector.collect(request(), capture(), current=True, read=read, clock=clock)
        return result, calls

    def test_offline_default_has_no_reads_or_clock_and_no_fresh_claim(self):
        packet = collector.collect(request(), capture(),
            read=lambda _: self.fail('offline AWS call'), clock=lambda: self.fail('offline clock'))
        self.assertEqual(packet['mode'], 'offline')
        self.assertEqual(packet['metadata_reads'], 0)
        self.assertEqual(packet['observations'], [])
        self.assertFalse(any(packet['interpretation'].values()))
        self.assertNotIn('captured_at', packet)

    def test_live_replay_preserves_historical_body_and_only_claims_bounded_new_observations(self):
        old = capture()
        packet, calls = self.run_capture()
        self.assertEqual(len(calls), 9)
        self.assertEqual(packet['metadata_reads'], 9)
        self.assertEqual(packet['historical_capture']['body'], old)
        self.assertEqual(packet['historical_capture']['sha256'], hashlib.sha256(raw_capture()).hexdigest())
        self.assertNotEqual(packet['historical_capture']['sha256'],
                            hashlib.sha256(json.dumps(old, sort_keys=True).encode()).hexdigest())
        self.assertEqual(capture(), old)
        self.assertNotIn('captured_at', packet)
        self.assertNotEqual(packet['kind'], old['capture_type'])
        permitted = {'reader_identity_verified', 'fresh_stack_output_matches_historical_revision',
                     'fresh_active_family_head_matches_historical_revision',
                     'fresh_lambda_six_configuration_auth_matches_historical_auth'}
        self.assertEqual({key for key, value in packet['interpretation'].items() if value}, permitted)
        self.assertEqual(packet['cloud_mutations'], 0)
        self.assertEqual(packet['state_accesses'], 0)
        self.assertEqual([item['alias'] for item in packet['observations']], [c['alias'] for c in calls])
        self.assertEqual([item['value']['version'] for item in packet['observations'][3:]], ['$LATEST'] * 6)
        self.assertEqual([item['value']['revision_id'] for item in packet['observations'][3:]],
                         ['dummy-revision-' + str(i) for i in range(6)])

    def test_fixed_scope_has_no_task_describe_iam_role_assume_or_workload_calls(self):
        plan = collector.call_plan(request(), capture())
        self.assertEqual([(call['service'], call['operation']) for call in plan], [
            ('sts', 'get-caller-identity'), ('cloudformation', 'describe-stacks'),
            ('ecs', 'list-task-definitions'), *[('lambda', 'get-function-configuration')] * 6])
        self.assertEqual(plan[1]['arguments'], ['--stack-name', 'AwsCdkStack', '--no-paginate'])
        self.assertEqual(json.loads(plan[2]['arguments'][1]), {
            'familyPrefix': 'DummyDailyBatch', 'status': 'ACTIVE', 'sort': 'DESC', 'maxResults': 1})
        self.assertEqual(plan[2]['arguments'][-1], '--no-paginate')
        self.assertEqual(plan[2]['query'], 'taskDefinitionArns')
        for call, name in zip(plan[3:], collector.CALLERS):
            self.assertEqual(call['arguments'], ['--function-name',
                'arn:aws:lambda:ap-northeast-1:222222222222:function:' + name])
        self.assertNotIn('Secret', collector.FUNCTION_QUERY)

    def test_identity_mismatch_stops_before_cloud_metadata(self):
        for key, value in [('account', '333333333333'),
                           ('arn', 'arn:aws:sts::222222222222:assumed-role/Other/session'),
                           ('arn', 'arn:aws:iam::222222222222:user/DummyUser')]:
            values = responses(); values[0][key] = value
            calls = []
            with self.subTest(key=key, value=value), self.assertRaises(collector.Stop):
                collector.collect(request(), capture(), current=True,
                    read=lambda call: calls.append(call) or values[len(calls) - 1], clock=lambda: POINT)
            self.assertEqual(len(calls), 1)

    def test_stack_wrong_output_duplicate_params_or_wrong_auth_stop_before_family_list(self):
        candidates = []
        stack = responses()[1]
        bad = copy.deepcopy(stack); bad[0]['outputs'] = []; candidates.append(bad)
        bad = copy.deepcopy(stack); bad[0]['outputs'] *= 2; candidates.append(bad)
        bad = copy.deepcopy(stack); bad[0]['outputs'][0]['OutputValue'] += '0'; candidates.append(bad)
        bad = copy.deepcopy(stack); bad[0]['parameters'][1] = bad[0]['parameters'][0]; candidates.append(bad)
        bad = copy.deepcopy(stack); bad[0]['parameters'][0]['value'] = 'dummy-wrong-auth'; candidates.append(bad)
        bad = copy.deepcopy(stack); bad[0]['name'] = 'OtherStack'; candidates.append(bad)
        bad = copy.deepcopy(stack); bad *= 2; candidates.append(bad)
        for bad in candidates:
            calls = []; values = responses(); values[1] = bad
            with self.subTest(candidate=candidates.index(bad)), self.assertRaises(collector.Stop):
                collector.collect(request(), capture(), current=True,
                    read=lambda call: calls.append(call) or values[len(calls) - 1], clock=lambda: POINT)
            self.assertEqual(len(calls), 2)

    def test_active_head_empty_wrong_revision_family_account_or_multiple_stop(self):
        arn = capture()['task_definition_arn']
        for bad in ([], [arn[:-1] + '8'], [arn.replace('DummyDailyBatch', 'OtherFamily')],
                    [arn.replace('222222222222', '333333333333')], [arn, arn], None, {}):
            calls = []; values = responses(); values[2] = bad
            with self.subTest(candidate=bad), self.assertRaises(collector.Stop):
                collector.collect(request(), capture(), current=True,
                    read=lambda call: calls.append(call) or values[len(calls) - 1], clock=lambda: POINT)
            self.assertEqual(len(calls), 3)

    def test_lambda_exact_name_arn_role_auth_revision_and_mutable_version_are_required(self):
        mutations = [('name', 'other'), ('arn', 'arn:aws:lambda:ap-northeast-1:333333333333:function:other'),
            ('role', 'arn:aws:iam::333333333333:role/Other'), ('auth', {}), ('version', '2'),
            ('revision_id', ''), ('last_modified', None), ('extra', 'dummy-private-secret')]
        for key, bad in mutations:
            calls = []; values = responses(); values[3][key] = bad
            with self.subTest(key=key), self.assertRaises(collector.Stop):
                collector.collect(request(), capture(), current=True,
                    read=lambda call: calls.append(call) or values[len(calls) - 1], clock=lambda: POINT)
            self.assertEqual(len(calls), 4)

    def test_request_and_historical_schema_reject_before_reads(self):
        candidates = []
        for key, bad in [('environment', 'prod'), ('aws_region', 'us-east-1'), ('aws_account_id', 'bad'),
                         ('stack_name', 'OtherStack'), ('readonly_session_name', 'bad/session'),
                         ('owner_capture_sha256', 'bad'), ('expires_utc', '2026-01-02T01:00:01Z'),
                         ('issued_utc', '2026-01-02T00:00:00'), ('schema_version', True),
                         ('AWS_SECRET_ACCESS_KEY', 'dummy-private-secret')]:
            value = request(); value[key] = bad; candidates.append((value, capture()))
        for key, bad in [('environment', 'prod'), ('readiness_attested', True), ('schema_version', True),
                         ('cloud_mutations', 1), ('task_role_arn', 'arn:aws:iam::333333333333:role/Other'),
                         ('captured_at', '2026-01-02T00:00:01Z'), ('unknown', 'dummy-private-secret')]:
            value = capture(); value[key] = bad; candidates.append((request(), value))
        value = capture(); value['runtime_auth']['GOOGLE_CLOUD_PROJECT'] = 'other'; candidates.append((request(), value))
        for index, (source, old) in enumerate(candidates):
            with self.subTest(index=index), self.assertRaises((collector.Stop, ValueError)):
                collector.collect(source, old, current=True, read=lambda _: self.fail('pre-auth read'))

    def test_expired_not_yet_valid_and_backwards_clock_stop_before_or_between_calls(self):
        for point in (POINT - timedelta(hours=1), POINT + timedelta(hours=1)):
            with self.subTest(point=point), self.assertRaises(collector.Stop):
                self.run_capture(clock=lambda: point)
        times = iter([POINT, POINT, POINT + timedelta(hours=1)])
        calls = []
        with self.assertRaises(collector.Stop):
            collector.collect(request(), capture(), current=True,
                read=lambda call: calls.append(call) or responses()[0], clock=lambda: next(times))
        self.assertEqual(len(calls), 1)
        times = iter([POINT, POINT - timedelta(seconds=1)])
        with self.assertRaises(collector.Stop):
            collector.collect(request(), capture(), current=True,
                read=lambda _: self.fail('backwards clock call'), clock=lambda: next(times))

    def test_typed_observation_packet_is_not_accepted_as_legacy_ownership_inventory(self):
        packet, _ = self.run_capture()
        other = ROOT / 'infra/gcp/scripts/prepare_runtime_ownership.py'
        spec = importlib.util.spec_from_file_location('legacy_prepare_ownership', other)
        legacy = importlib.util.module_from_spec(spec); spec.loader.exec_module(legacy)
        with self.assertRaises((KeyError, ValueError)):
            legacy.prepare(packet)
        self.assertNotIn('runtime_grants', packet)
        self.assertNotIn('runtime_wif_inventory', packet)


class CliAndPrivateFiles(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='dummy-aws-versioned-')
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.directory.chmod(0o700)
        self.source = self.directory / 'request.json'
        self.old = self.directory / 'owner.json'
        self.output = self.directory / 'observations.json'
        self.old.write_bytes(raw_capture()); self.old.chmod(0o600)
        self.write_request(request())

    def write_request(self, value):
        self.source.write_text(json.dumps(value)); self.source.chmod(0o600)
        self.request_hash = hashlib.sha256(self.source.read_bytes()).hexdigest()

    def run_cli(self, live=False):
        stdout, stderr = io.StringIO(), io.StringIO()
        args = ['--request-file', str(self.source), '--request-sha256', self.request_hash,
                '--owner-capture-file', str(self.old), '--output', str(self.output)]
        if live:
            args.append('--read-current')
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            code = collector.main(args)
        return code, stdout.getvalue(), stderr.getvalue()

    def test_default_cli_exclusively_saves_private_output_without_auth_or_private_stdout(self):
        with patch.object(collector, 'aws_read', side_effect=AssertionError('unexpected auth')):
            code, stdout, stderr = self.run_cli()
        self.assertEqual(code, 0)
        self.assertEqual(self.output.stat().st_mode & 0o777, 0o600)
        self.assertNotIn('DummyRuntimeTaskRole', stdout + stderr)
        self.assertNotIn('222222222222', stdout + stderr)
        packet = json.loads(self.output.read_text())
        self.assertEqual(packet['historical_capture']['body'], capture())
        self.assertEqual(packet['observations'], [])
        self.assertEqual(self.run_cli()[0], 1)

    def test_live_cli_full_mock_replay_uses_only_nine_calls_and_private_fixed_output(self):
        values = iter(responses())
        commands = []

        def run(command, **kwargs):
            commands.append(command)
            self.assertEqual(kwargs['env']['AWS_MAX_ATTEMPTS'], '1')
            value = next(values)
            return subprocess.CompletedProcess(command, 0, json.dumps(value), '')

        with patch.object(collector.subprocess, 'run', side_effect=run), \
             patch.object(collector, 'now', return_value=POINT):
            code, stdout, stderr = self.run_cli(live=True)
        self.assertEqual(code, 0)
        self.assertEqual(len(commands), 9)
        self.assertEqual(json.loads(self.output.read_text())['metadata_reads'], 9)
        self.assertEqual(json.loads(self.output.read_text())['historical_capture']['body'], capture())
        self.assertNotIn('DummyRuntimeTaskRole', stdout + stderr)
        self.assertNotIn('222222222222', stdout + stderr)
        self.assertIn('--no-paginate', commands[2])
        input_index = commands[2].index('--cli-input-json') + 1
        self.assertEqual(json.loads(commands[2][input_index])['maxResults'], 1)

    def test_live_cli_partial_read_failure_does_not_publish_or_retry_private_diagnostics(self):
        outcomes = iter([subprocess.CompletedProcess('dummy', 0, json.dumps(responses()[0]), ''),
                         subprocess.CompletedProcess('dummy', 1, '', 'dummy-private-error-body')])
        with patch.object(collector.subprocess, 'run', side_effect=lambda *args, **kwargs: next(outcomes)) as run, \
             patch.object(collector, 'now', return_value=POINT):
            code, stdout, stderr = self.run_cli(live=True)
        self.assertEqual(code, 1)
        self.assertEqual(run.call_count, 2)
        self.assertFalse(self.output.exists())
        self.assertNotIn('dummy-private-error-body', stdout + stderr)

    def test_source_or_owner_hash_tamper_and_duplicate_unknown_keys_stop_before_auth(self):
        for path, raw in [(self.old, raw_capture() + b' '),
                          (self.source, self.source.read_bytes() + b' '),
                          (self.source, b'{"schema_version":1,"schema_version":1}')]:
            original = path.read_bytes(); path.write_bytes(raw)
            with patch.object(collector, 'collect') as collect:
                code, stdout, stderr = self.run_cli(live=True)
                self.assertEqual(code, 1); collect.assert_not_called()
                self.assertNotIn('DummyRuntimeTaskRole', stdout + stderr)
                self.assertFalse(self.output.exists())
            path.write_bytes(original)
        value = request(); value['AWS_SECRET_ACCESS_KEY'] = 'dummy-private-secret'; self.write_request(value)
        with patch.object(collector, 'collect') as collect:
            code, stdout, stderr = self.run_cli(live=True)
            self.assertEqual(code, 1); collect.assert_not_called()
            self.assertNotIn('dummy-private-secret', stdout + stderr)

    def test_symlinks_hardlinks_wrong_mode_checkout_and_empty_nonmanaged_git_marker_reject(self):
        old_source = self.source
        link = self.directory / 'link.json'; link.symlink_to(self.source); self.source = link
        with patch.object(collector, 'collect') as collect:
            self.assertEqual(self.run_cli()[0], 1); collect.assert_not_called()
        self.source = old_source
        link.unlink(); os.link(self.source, link)
        with patch.object(collector, 'collect') as collect:
            self.assertEqual(self.run_cli()[0], 1); collect.assert_not_called()
        link.unlink(); self.source.chmod(0o644)
        with patch.object(collector, 'collect') as collect:
            self.assertEqual(self.run_cli()[0], 1); collect.assert_not_called()
        self.source.chmod(0o600)
        self.output.symlink_to(self.old)
        with patch.object(collector, 'collect') as collect:
            self.assertEqual(self.run_cli()[0], 1); collect.assert_not_called()
        self.output.unlink()
        for git_kind in ('empty-dir', 'real-dir', 'file'):
            checkout = self.directory / git_kind; checkout.mkdir(mode=0o700)
            if git_kind == 'file':
                (checkout / '.git').write_text('gitdir: private-dummy')
            else:
                (checkout / '.git').mkdir()
                if git_kind == 'real-dir':
                    (checkout / '.git/HEAD').write_text('ref: refs/heads/dummy')
            with self.subTest(git_kind=git_kind), self.assertRaises(collector.Stop):
                collector.private_parent(checkout / 'output.json')
        with self.assertRaises(collector.Stop):
            collector.private_parent(ROOT / 'private-output.json')

    def test_cli_failure_or_expiry_cleans_reserved_output_and_suppresses_diagnostic(self):
        for error in (ValueError('dummy-private-secret'), subprocess.TimeoutExpired('dummy-private-command', 45)):
            with patch.object(collector, 'collect', side_effect=error):
                code, stdout, stderr = self.run_cli(live=True)
            self.assertEqual(code, 1); self.assertFalse(self.output.exists())
            self.assertNotIn('dummy-private', stdout + stderr)
        with patch.object(collector, 'collect', return_value={'dummy': 'secret'}), \
             patch.object(collector, 'now', return_value=POINT + timedelta(hours=1)):
            self.assertEqual(self.run_cli(live=True)[0], 1)
            self.assertFalse(self.output.exists())

    def test_cli_wrapper_uses_inherited_session_and_fixed_no_retry_allowlist(self):
        class Result:
            returncode = 0
            stdout = '{"account":"222222222222"}'

        with patch.dict(os.environ, {'AWS_ACCESS_KEY_ID': 'dummy-access', 'AWS_SESSION_TOKEN': 'dummy-token'}):
            original = os.environ.copy()
            call = collector.call_plan(request(), capture())[0]
            with patch.object(collector.subprocess, 'run', return_value=Result()) as run:
                collector.aws_read(call)
            args, kwargs = run.call_args
            self.assertEqual(args[0][:3], ['aws', 'sts', 'get-caller-identity'])
            self.assertEqual(kwargs['env']['AWS_ACCESS_KEY_ID'], 'dummy-access')
            self.assertEqual(kwargs['env']['AWS_SESSION_TOKEN'], 'dummy-token')
            self.assertEqual(kwargs['env']['AWS_MAX_ATTEMPTS'], '1')
            self.assertEqual(kwargs['timeout'], 45)
            self.assertEqual(os.environ.copy(), original)
        call['operation'] = 'assume-role'
        with patch.object(collector.subprocess, 'run') as run, self.assertRaises(collector.Stop):
            collector.aws_read(call)
        run.assert_not_called()


if __name__ == '__main__':
    unittest.main()
