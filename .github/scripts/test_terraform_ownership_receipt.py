#!/usr/bin/env python3
"""Public dummy GitHub metadata/logs; no token, network, state or cloud calls."""
from __future__ import annotations

import copy
from io import StringIO
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import terraform_ownership_receipt as gate
import test_gcp_user_activity_schema_audit_workflow as history_fixture

SHA = 'a' * 40
NONCE = 'ab' * 32  # Public fixture only; real helper uses os.urandom(32).


def dummy_binding():
    fixture = history_fixture.UserActivityHistoryPlanTest()
    fixture.setUp()
    _, metadata = fixture.fixture()
    return dict(nonce=NONCE, backend=dict(account_id='111111111111', bucket='dummy-state-bucket',
        key='youtube-study-space/dev/terraform.tfstate', region='ap-northeast-1', workspace='default'), history_metadata=metadata)


def dummy_context():
    return dict(GITHUB_REPOSITORY=gate.REPOSITORY, GITHUB_REPOSITORY_ID=str(gate.REPOSITORY_ID),
                GITHUB_REPOSITORY_OWNER_ID=str(gate.OWNER_ID), GITHUB_REF='refs/heads/' + gate.BRANCH,
                GITHUB_EVENT_NAME='workflow_dispatch', GITHUB_WORKFLOW_REF=gate.WORKFLOW_REF,
                GITHUB_SHA=SHA, GITHUB_RUN_ID='101', GITHUB_RUN_ATTEMPT='1', GITHUB_JOB='apply',
                MODE='apply', HISTORY_TARGET='dev', TF_VAR_project_id='test-youtube-study-space', DEV_HISTORY_RECEIPT_EMITTER_ENABLED='true')


def dummy_catalog():
    steps = copy.deepcopy(gate.MINIMUM_STEPS)
    steps['plan'] = ['Checkout trusted commit'] + gate.HISTORY_STEPS[:2] + gate.MINIMUM_STEPS['plan'][1:]
    steps['apply'] = ['Checkout the exact planned commit', 'Assert plan/apply commit identity',
        *gate.HISTORY_STEPS[:6],
        'Apply the locally re-created saved plan', gate.HISTORY_STEPS[6], 'Require post-apply no-op',
        gate.HISTORY_STEPS[7], 'Emit verified ownership receipt', 'Cleanup sensitive temporary files']
    return dict(schema_version=1, issuers={SHA: dict(workflow_id=19,
        job_names=dict(plan='dummy protected plan', apply='dummy protected apply'),
        environments=dict(plan=dict(id=31, name='terraform-dev-plan'), apply=dict(id=32, name='terraform-dev-apply')),
        reviewer_ids=[17], receipt_step='Emit verified ownership receipt', required_steps=steps,
        waves=['history12', 'pool', 'provider', 'grant', 'api', 'noop'])})


def dummy_receipt(run_id=101, wave='history12', scope=None, prior=None, prior_scope=None, count=12):
    return dict(schema_version=1, repository_id=gate.REPOSITORY_ID, source_sha=SHA, run_id=run_id, run_attempt=1,
        job_role='apply', environment='terraform-dev-apply', target='dev', root_contract='gcp-dev-default',
        operation='apply', wave=wave, phase='post', prior_receipt_sha256=prior, prior_scope_commitment=prior_scope,
        scope_commitment=scope or gate.commitment(dummy_binding(), gate.history_scope(dummy_binding())),
        prior_state_commitment=None if wave == 'history12' else 'c'*64, state_after_commitment='c'*64,
        resource_count=count, import_before_count=int(wave != 'noop'), import_post_count=0,
        checks={key: True for key in gate.CHECKS}, accounting=dict(terraform_outcomes=dict(apply='success', post_plan='success'),
            post_state_reads=4, post_metadata_reads=1, provider_requests='unknown', actual_cost='unknown'))


class GitHubFixture:
    def __init__(self, value=None, *, minute=0):
        self.value = value or dummy_receipt()
        self.ref = dict(run_id=self.value['run_id'], attempt=1, job_id=self.value['run_id'] + 1000,
                        source_sha=SHA, receipt_sha256=gate.digest(self.value))
        repo = dict(id=gate.REPOSITORY_ID, full_name=gate.REPOSITORY, owner=dict(id=gate.OWNER_ID))
        run = dict(id=self.ref['run_id'], run_attempt=1, status='completed', conclusion='success',
            event='workflow_dispatch', head_sha=SHA, head_branch=gate.BRANCH, path=gate.CALLER, workflow_id=19,
            repository=copy.deepcopy(repo), head_repository=copy.deepcopy(repo), referenced_workflows=[dict(
                path=f'{gate.REPOSITORY}/{gate.REUSABLE}@refs/heads/{gate.BRANCH}', sha=SHA, ref='refs/heads/' + gate.BRANCH)])
        self.base = f"/repos/{gate.REPOSITORY}/actions/runs/{self.ref['run_id']}"
        policy = dummy_catalog()['issuers'][SHA]
        jobs = []
        for role, job_id, start, end in [('plan', self.ref['job_id'] - 1, minute, minute + 1),
                                         ('apply', self.ref['job_id'], minute + 2, minute + 3)]:
            jobs.append(dict(id=job_id, run_id=self.ref['run_id'], name=policy['job_names'][role], head_sha=SHA,
                head_branch=gate.BRANCH, status='completed', conclusion='success',
                started_at=f'2026-10-10T01:{start:02d}:00Z', completed_at=f'2026-10-10T01:{end:02d}:00Z',
                steps=[dict(name=name, number=i, status='completed', conclusion='success')
                       for i, name in enumerate(policy['required_steps'][role], 1)]))
        emit_index = len(jobs[1]['steps']) - 2
        self.log_path = f"/repos/{gate.REPOSITORY}/actions/jobs/{self.ref['job_id']}/steps/{emit_index}/logs"
        self.responses = {self.base: run, self.base + '/attempts/1': copy.deepcopy(run),
            self.base + '/attempts/1/jobs?per_page=100&page=1': dict(total_count=2, jobs=jobs),
            self.base + '/approvals': [dict(state='approved', user=dict(id=17), environments=list(policy['environments'].values()))],
            self.log_path: b'2026-10-10T01:03:00.123Z ' + gate.MARKER.encode() + gate.canonical(self.value) + b'\n'}
        self.calls = []

    def __call__(self, path, *, log=False):
        self.calls.append((path, log))
        value = self.responses[path]
        if isinstance(value, Exception):
            raise value
        return copy.deepcopy(value)

    @property
    def jobs(self):
        return self.responses[self.base + '/attempts/1/jobs?per_page=100&page=1']['jobs']


class OwnershipReceiptTest(unittest.TestCase):
    def test_exact_authenticated_metadata_step_log_and_double_read(self):
        fake = GitHubFixture()
        value, timing = gate.authenticate(fake.ref, dummy_catalog(), request=fake)
        self.assertEqual(value, fake.value)
        self.assertEqual(timing['completed_at'], '2026-10-10T01:03:00Z')
        self.assertEqual(fake.calls[0], fake.calls[-1])
        self.assertEqual(sum(log for _, log in fake.calls), 1)
        self.assertIn('/attempts/1/jobs?', fake.calls[2][0])
        self.assertEqual(len(fake.calls), 6)

    def test_empty_catalog_rejects_before_transport(self):
        fake = GitHubFixture()
        with self.assertRaises(ValueError):
            gate.authenticate(fake.ref, dict(schema_version=1, issuers={}), request=fake)
        self.assertEqual(fake.calls, [])

    def test_shape_only_run_ids_cannot_substitute_for_receipts(self):
        for value in [1, dict(run_id=1), dict(GitHubFixture().ref, attempt=2), dict(GitHubFixture().ref, run_id=True)]:
            with self.subTest(value=value), self.assertRaises(ValueError):
                gate.authenticate(value, dummy_catalog(), request=lambda *_: self.fail('no transport'))

    def test_wrong_source_repository_owner_event_branch_caller_and_reusable_rejected(self):
        changes = [dict(id=102), dict(run_attempt=2), dict(status='in_progress'), dict(conclusion='skipped'),
            dict(event='pull_request'), dict(head_sha='b'*40), dict(head_branch='dev'), dict(path='other.yml'), dict(workflow_id=20),
            dict(repository=dict(id=99, full_name=gate.REPOSITORY, owner=dict(id=gate.OWNER_ID))),
            dict(head_repository=dict(id=gate.REPOSITORY_ID, full_name=gate.REPOSITORY, owner=dict(id=99))),
            dict(referenced_workflows=[]), dict(referenced_workflows=[dict(path='wrong', sha=SHA, ref='refs/heads/' + gate.BRANCH)])]
        for mutation in changes:
            fake = GitHubFixture()
            fake.responses[fake.base].update(mutation)
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                gate.authenticate(fake.ref, dummy_catalog(), request=fake)

    def test_wrong_attempt_and_rerun_race_rejected(self):
        fake = GitHubFixture()
        fake.responses[fake.base + '/attempts/1']['head_sha'] = 'b'*40
        with self.assertRaises(ValueError):
            gate.authenticate(fake.ref, dummy_catalog(), request=fake)
        fake = GitHubFixture()
        def race(path, **kw):
            value = fake(path, **kw)
            if path == fake.base and len(fake.calls) > 1:
                value['run_attempt'] = 2
            return value
        with self.assertRaises(ValueError):
            gate.authenticate(fake.ref, dummy_catalog(), request=race)

    def test_wrong_job_step_failed_skipped_cleanup_and_order_rejected(self):
        mutations = [lambda f: f.jobs[1].update(id=999), lambda f: f.jobs[1].update(conclusion='failure'),
            lambda f: f.jobs[0].update(head_sha='b'*40), lambda f: f.jobs[1].update(name='security probe'),
            lambda f: f.jobs[1]['steps'][-1].update(conclusion='failure'),
            lambda f: f.jobs[1]['steps'][-2].update(conclusion='skipped'),
            lambda f: f.jobs[1]['steps'][-2].update(number=99),
            lambda f: f.jobs.append(copy.deepcopy(f.jobs[0])),
            lambda f: f.jobs[1].update(started_at='2026-10-10T00:00:00Z')]
        for mutate in mutations:
            fake = GitHubFixture()
            mutate(fake)
            with self.subTest(case=mutations.index(mutate)), self.assertRaises(ValueError):
                gate.authenticate(fake.ref, dummy_catalog(), request=fake)

    def test_environment_approvals_require_exact_ids_names_authorized_reviewer(self):
        changes = [[], [dict(state='rejected', user=dict(id=17), environments=[dict(id=31, name='terraform-dev-plan')])],
            [dict(state='approved', user=dict(id=99), environments=[dict(id=31, name='terraform-dev-plan'), dict(id=32, name='terraform-dev-apply')])],
            [dict(state='approved', user=dict(id=17), environments=[dict(id=999, name='terraform-dev-plan')])],
            [dict(state='approved', user=dict(id=17), environments=[dict(id=31, name='terraform-dev-plan')])]]
        for response in changes:
            fake = GitHubFixture()
            fake.responses[fake.base + '/approvals'] = response
            with self.subTest(response=response), self.assertRaises(ValueError):
                gate.authenticate(fake.ref, dummy_catalog(), request=fake)

    def test_private_scope_nonce_context_and_metadata_bound_without_values_in_receipt(self):
        value = dummy_binding()
        committed = gate.commitment(value, gate.history_scope(value))
        for mutation in [lambda v: v['backend'].update(bucket='dummy-other-bucket'),
                         lambda v: v['history_metadata']['schema']['fields'].reverse(), lambda v: v.update(nonce='cd'*32)]:
            other = copy.deepcopy(value)
            mutation(other)
            self.assertNotEqual(gate.commitment(other, gate.history_scope(other)), committed)
        public = gate.canonical(dummy_receipt()).decode()
        for sentinel in [NONCE, 'dummy-state-bucket', '111111111111', json.dumps(value['history_metadata'])]:
            self.assertNotIn(sentinel, public)

    def test_receipt_schema_action_counts_flags_and_digests_fail_closed(self):
        changes = [dict(operation='plan'), dict(target='prod'), dict(phase='before'), dict(root_contract='other'),
                   dict(import_post_count=1), dict(import_before_count=0), dict(resource_count=11), dict(run_attempt=True),
                   dict(checks=dict.fromkeys(gate.CHECKS, False)), dict(private_member='DUMMY_PRIVATE_MEMBER'),
                   dict(prior_receipt_sha256='b'*64), dict(accounting={})]
        for mutation in changes:
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                gate.validate_receipt(dummy_receipt() | mutation)
        fake = GitHubFixture()
        fake.ref['receipt_sha256'] = 'b'*64
        with self.assertRaises(ValueError):
            gate.authenticate(fake.ref, dummy_catalog(), request=fake)

    def test_log_duplicate_json_marker_truncation_and_oversize_rejected(self):
        fake = GitHubFixture()
        good = fake.responses[fake.log_path]
        for raw in [good + good, good[:-1], b'PASS\n', b'x' * (1024*1024+1),
                    good.replace(b'"target":"dev"', b'"target":"dev","target":"dev"'),
                    good.replace(b'"import_post_count":0', b'"import_post_count":NaN')]:
            fake = GitHubFixture()
            fake.responses[fake.log_path] = raw
            with self.subTest(length=len(raw)), self.assertRaises(ValueError):
                gate.authenticate(fake.ref, dummy_catalog(), request=fake)

    def test_incomplete_pagination_and_api_failures_have_no_fallback(self):
        for response in [dict(total_count=3, jobs=GitHubFixture().jobs), dict(total_count=0, jobs=[]),
                         dict(total_count=True, jobs=GitHubFixture().jobs), dict(total_count=1001, jobs=[])]:
            fake = GitHubFixture()
            fake.responses[fake.base + '/attempts/1/jobs?per_page=100&page=1'] = response
            with self.assertRaises(ValueError):
                gate.authenticate(fake.ref, dummy_catalog(), request=fake)
        for endpoint in GitHubFixture().responses:
            fake = GitHubFixture()
            fake.responses[endpoint] = RuntimeError('DUMMY_PRIVATE_API_ERROR')
            with self.subTest(endpoint=endpoint), self.assertRaises(RuntimeError):
                gate.authenticate(fake.ref, dummy_catalog(), request=fake)

    def test_catalog_requires_critical_steps_and_issuer_wave_approval(self):
        catalog = dummy_catalog()
        catalog['issuers'][SHA]['required_steps']['apply'].remove('Require post-apply no-op')
        with self.assertRaises(ValueError):
            gate.load_catalog(catalog)
        fake = GitHubFixture()
        catalog = dummy_catalog()
        catalog['issuers'][SHA]['waves'] = ['pool']
        with self.assertRaises(ValueError):
            gate.authenticate(fake.ref, catalog, request=fake)

    def test_selective_log_transport_strips_auth_and_limits_redirects(self):
        from urllib.error import HTTPError
        class Response:
            status = 200
            def __enter__(self): return self
            def __exit__(self, *args): pass
            def read(self, limit): return b'DUMMY_LOG\n'
        for host, allowed in [('dummy.actions.githubusercontent.com', True), ('dummy.blob.core.windows.net', True),
                              ('attacker.invalid', False), ('actions.githubusercontent.com.attacker.invalid', False)]:
            calls = []
            class Opener:
                def open(self, request, timeout):
                    calls.append(request)
                    if len(calls) == 1:
                        raise HTTPError(request.full_url, 302, 'redirect', {'Location': 'https://' + host + '/dummy'}, None)
                    return Response()
            transport = gate.GitHubRead('DUMMY_GITHUB_TOKEN')
            transport.opener = Opener()
            if allowed:
                self.assertEqual(transport(f'/repos/{gate.REPOSITORY}/actions/jobs/1101/steps/10/logs', log=True), b'DUMMY_LOG\n')
                self.assertIsNone(calls[1].get_header('Authorization'))
            else:
                with self.assertRaises(ValueError):
                    transport(f'/repos/{gate.REPOSITORY}/actions/jobs/1101/steps/10/logs', log=True)
                self.assertEqual(len(calls), 1)

    def test_nonce_generation_is_private_exclusive_and_value_free(self):
        with tempfile.TemporaryDirectory() as root:
            gate.new_nonce(dict(RUNNER_TEMP=root))
            path = Path(root) / 'ownership-receipt-nonce.json'
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertTrue(gate.hex_value(json.loads(path.read_text())['nonce'], 64))
            with self.assertRaises(Exception):
                gate.new_nonce(dict(RUNNER_TEMP=root))

    def test_emitter_requires_closed_gate_native_context_and_sanitized_private_record(self):
        import terraform_history_plan_receipt as private
        with tempfile.TemporaryDirectory() as root:
            env = dummy_context() | dict(RUNNER_TEMP=root)
            private.write_private(Path(root) / gate.RECORD, dummy_receipt())
            with patch('sys.stdout', StringIO()) as output:
                gate.emit(env)
            self.assertEqual(output.getvalue(), gate.MARKER + gate.canonical(dummy_receipt()).decode() + '\n')
            for change in [dict(DEV_HISTORY_RECEIPT_EMITTER_ENABLED='false'), dict(GITHUB_SHA='b'*40), dict(MODE='plan'),
                           dict(GITHUB_RUN_ATTEMPT='2'), dict(GITHUB_JOB='security-probe'), dict(GITHUB_RUN_ID='102')]:
                with self.subTest(change=change), patch('sys.stdout', StringIO()) as output, self.assertRaises(ValueError):
                    gate.emit(env | change)
                self.assertEqual(output.getvalue(), '')


    def test_complete_pagination_uses_every_exact_attempt_page(self):
        fake = GitHubFixture()
        extras = [dict(id=5000+i, run_id=fake.ref['run_id'], head_sha=SHA, name='dummy unrelated job') for i in range(100)]
        fake.responses[fake.base + '/attempts/1/jobs?per_page=100&page=1'] = dict(total_count=102, jobs=fake.jobs+extras[:98])
        fake.responses[fake.base + '/attempts/1/jobs?per_page=100&page=2'] = dict(total_count=102, jobs=extras[98:])
        gate.authenticate(fake.ref, dummy_catalog(), request=fake)
        self.assertEqual(len(fake.calls), 7)
        self.assertIn((fake.base + '/attempts/1/jobs?per_page=100&page=2', False), fake.calls)


    def test_state_commitment_is_private_and_preserves_lineage_serial_and_values(self):
        value = dummy_binding()
        state = dict(lineage='DUMMY_PRIVATE_LINEAGE', serial=12, resources=[dict(member='DUMMY_PRIVATE_MEMBER')])
        committed = gate.state_commitment(value, state)
        for mutation in [dict(lineage='DUMMY_OTHER_LINEAGE'), dict(serial=13), dict(resources=[])]:
            self.assertNotEqual(gate.state_commitment(value, state | mutation), committed)
        self.assertNotEqual(committed, gate.commitment(value, state))
        for sentinel in ['DUMMY_PRIVATE_LINEAGE', 'DUMMY_PRIVATE_MEMBER', value['nonce']]:
            self.assertNotIn(sentinel, committed)


    def test_history_binding_denies_before_auth_and_joins_plan_apply_and_fresh_metadata(self):
        import terraform_history_plan_receipt as private
        value = dummy_binding()
        with tempfile.TemporaryDirectory() as root:
            path = Path(root)/'output'
            path.write_text('')
            backend = value['backend']
            env = dummy_context() | dict(GITHUB_JOB='plan', RUNNER_TEMP=root, GITHUB_OUTPUT=str(path),
                TF_VAR_manage_user_activity_history='true', STATE_ACCOUNT_ID=backend['account_id'], STATE_BUCKET=backend['bucket'],
                STATE_KEY=backend['key'], STATE_AWS_REGION=backend['region'], TF_WORKSPACE=backend['workspace'],
                RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(dict(schema_version=1, receipt_binding=value)))
            gate.check_history_binding(env)
            scoped = path.read_text().strip().split('=', 1)[1]
            self.assertEqual(scoped, gate.commitment(value, gate.history_scope(value)))
            private.write_private(Path(root)/'user-history-before.json', value['history_metadata'])
            gate.check_history_binding(env, fresh=True)
            apply = env | dict(GITHUB_JOB='apply', EXPECTED_HISTORY_RECEIPT_SCOPE=scoped)
            gate.check_history_binding(apply)
            gate.check_history_binding(apply, fresh=True)
            for change in [dict(RUNTIME_OWNERSHIP_PACKET_JSON='{}'), dict(STATE_BUCKET='dummy-other-state'),
                           dict(EXPECTED_HISTORY_RECEIPT_SCOPE='b'*64), dict(GITHUB_RUN_ATTEMPT='2')]:
                with self.subTest(change=change), self.assertRaises(ValueError):
                    gate.check_history_binding(apply | change)
            changed = copy.deepcopy(value['history_metadata'])
            changed['schema']['fields'].reverse()
            (Path(root)/'user-history-before.json').write_text(json.dumps(changed))
            with self.assertRaises(ValueError):
                gate.check_history_binding(apply, fresh=True)
            self.assertNotIn(value['nonce'], path.read_text())
            self.assertNotIn('dummy-state-bucket', path.read_text())


if __name__ == '__main__':
    unittest.main()
