"""DEV maintenance operator Job配備の観測可能な制御を検証する。クラウド認証情報は使わない。"""
from copy import deepcopy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import operator_identity as identity
import operator_release as o


def environment():
    return {'GITHUB_REPOSITORY': o.c.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
            'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_ACTOR_ID': '54093651',
            'GITHUB_ACTOR': 'kani3camp', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
            'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_RUN_ATTEMPT': '1',
            'GITHUB_REF': o.c.REF, 'GITHUB_WORKFLOW_REF': o.CALLER,
            'GITHUB_SHA': 'a' * 40, 'GITHUB_RUN_ID': '123',
            'MYPAGE_OPERATOR_EXPECTED_SHA': 'a' * 40, 'MYPAGE_OPERATOR_ACCESS_TOKEN': 'synthetic-token'}


def release():
    return {'image_source_sha': 'a' * 40,
            'image_digest': o.runtime.OPERATOR_REPOSITORY + '@sha256:' + 'b' * 64}


def legacy_image():
    # Before the first operator release, the Job still points at the API-bundled package.
    return o.runtime.REPOSITORY + '@sha256:' + 'c' * 64


def job(*, after=False, image=None):
    container = {
        'image': release()['image_digest'] if after else (image or legacy_image()),
        'command': ['/app/mypage-maintenance-operator'],
        'args': [],
        'resources': {'limits': {'cpu': '1', 'memory': '512Mi'}},
        'env': [
            {'name': 'GOOGLE_CLOUD_PROJECT', 'value': o.c.PROJECT},
            {'name': 'MYPAGE_ENVIRONMENT', 'value': 'development'},
        ],
    }
    return {
        'name': o.JOB_NAME,
        'uid': 'synthetic-job-uid',
        'etag': 'new-job-etag' if after else 'old-job-etag',
        'generation': '2' if after else '1',
        'observedGeneration': '2' if after else '1',
        'updateTime': '2026-10-26T01:00:01Z' if after else '2026-10-26T01:00:00Z',
        'launchStage': 'GA',
        'labels': {'goog-terraform-provisioned': 'true', 'preserve': 'label'},
        'annotations': {'preserve': 'annotation'},
        'client': 'terraform',
        'clientVersion': '8.5.0',
        'executionCount': 8 if after else 7,
        'latestCreatedExecution': {'name': 'executions/new' if after else 'executions/old'},
        'reconciling': False,
        'terminalCondition': {'state': 'CONDITION_SUCCEEDED'},
        'template': {
            'taskCount': 1,
            'parallelism': 1,
            'template': {
                'serviceAccount': 'mypage-maintenance-job@' + o.c.PROJECT + '.iam.gserviceaccount.com',
                'maxRetries': 0,
                'timeout': '60s',
                'executionEnvironment': 'EXECUTION_ENVIRONMENT_GEN2',
                'containers': [container],
            },
        },
    }


class OperatorReleaseTest(unittest.TestCase):
    def test_rejects_unauthorized_context_before_any_cloud_access(self):
        o.context(environment())
        for key, value in [('GITHUB_REF', 'refs/heads/main'), ('GITHUB_ACTOR_ID', '0'),
                           ('GITHUB_RUN_ATTEMPT', '2'), ('GITHUB_EVENT_NAME', 'push'),
                           ('GITHUB_EVENT_NAME', 'pull_request'),
                           ('GITHUB_WORKFLOW_REF', o.c.CALLER),
                           ('GITHUB_WORKFLOW_REF', o.c.workflow_ref('deploy-mypage-api.yml')),
                           ('MYPAGE_OPERATOR_EXPECTED_SHA', 'b' * 40),
                           ('AWS_ACCESS_KEY_ID', 'synthetic-private')]:
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                o.context(environment() | {key: value})

    def test_identity_is_bound_to_the_manual_operator_workflow_only(self):
        condition = identity.condition()
        for text in ('assertion.ref == ' + json.dumps(o.c.REF),
                     'assertion.workflow_ref == ' + json.dumps(o.CALLER),
                     'assertion.job_workflow_ref == ' + json.dumps(o.REUSABLE),
                     'assertion.actor_id == "54093651"', 'assertion.run_attempt == "1"',
                     'assertion.event_name == "workflow_dispatch"',
                     'assertion.environment == "mypage-dev-operator"',
                     'assertion.workflow_sha == assertion.job_workflow_sha'):
            self.assertIn(text, condition)
        for absent in ('push', 'pull_request', 'mypage-backend-authenticated.yml', ' || '):
            self.assertNotIn(absent, condition)
        workflow = Path(o.__file__).resolve().parents[3] / '.github/workflows/mypage-operator-authenticated.yml'
        self.assertTrue(o.REUSABLE.startswith(o.c.WORKFLOWS + workflow.name + '@'))

    def test_release_target_must_be_the_dedicated_operator_package(self):
        env = environment() | {'MYPAGE_OPERATOR_IMAGE_SOURCE_SHA': 'f' * 40}
        o.context(env | {'MYPAGE_OPERATOR_IMAGE_DIGEST': release()['image_digest']})
        for digest in (legacy_image(), '', o.runtime.OPERATOR_REPOSITORY + ':latest'):
            with self.subTest(digest=digest), self.assertRaises(ValueError):
                o.context(env | {'MYPAGE_OPERATOR_IMAGE_DIGEST': digest})

    def test_manual_release_resolves_exact_source_tag_of_operator_package(self):
        given = environment()
        self.assertEqual(o.declaration(given)['image_digest'], '')
        prefix = o.PACKAGE + '/versions/'
        with patch.object(o, 'registry_tag', return_value={'version': prefix + 'sha256:' + 'b' * 64}):
            self.assertEqual(o.declaration(given, 'synthetic-token')['image_digest'], release()['image_digest'])
        api_package = prefix.replace('/mypage-maintenance-operator/', '/mypage-api/')
        for version in (prefix + 'sha256:bad', api_package + 'sha256:' + 'b' * 64):
            with self.subTest(version=version), \
                 patch.object(o, 'registry_tag', return_value={'version': version}), \
                 self.assertRaises(ValueError):
                o.declaration(given, 'synthetic-token')

    def test_job_payload_preserves_all_writable_config_and_changes_only_image(self):
        before = job()
        payload = o.job_payload(before, release())
        expected = o.job_mutable_configuration(before)
        expected['etag'] = 'old-job-etag'
        expected['template']['template']['containers'][0]['image'] = release()['image_digest']
        self.assertEqual(payload, expected)
        self.assertEqual(payload['labels'], before['labels'])
        self.assertEqual(payload['annotations'], before['annotations'])
        for output in ('executionCount', 'latestCreatedExecution', 'generation',
                       'startExecutionToken', 'runExecutionToken'):
            self.assertNotIn(output, payload)
        self.assertEqual(before, job())

        # Output-only execution metadata may change independently of Job config.
        o.validate_job_result(job(after=True), before, release())
        for mutate in (
            lambda x: x['template']['template'].update(serviceAccount='other'),
            lambda x: x['template']['template']['containers'][0]['env'].pop(),
            lambda x: x['labels'].update(preserve='changed'),
            lambda x: x.update(generation='3'),
            lambda x: x.update(uid='other-job'),
        ):
            after = job(after=True)
            mutate(after)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                o.validate_job_result(after, before, release())

    def test_current_job_image_must_be_a_known_package(self):
        o.job_baseline(job(), release())
        o.job_baseline(job(image=o.runtime.OPERATOR_REPOSITORY + '@sha256:' + 'd' * 64), release())
        with self.assertRaises(ValueError):
            o.job_baseline(job(image=o.runtime.IMAGE_REGISTRY + 'other@sha256:' + 'd' * 64), release())

    def test_job_wait_tolerates_stale_get_until_generation_advances(self):
        operation = {'name': 'projects/' + o.c.PROJECT + '/locations/' + o.c.REGION + '/operations/synthetic',
                     'done': True}
        with patch.object(o, 'job_request', side_effect=[job(), job(after=True)]) as request, \
             patch.object(o.time, 'sleep'):
            self.assertEqual(o.wait_job_update('token', operation, job(), release()), job(after=True))
        self.assertEqual(request.call_count, 2)

    def execute(self, *, moved=False, unchanged=False, superseded=False):
        before_job = job(after=unchanged)
        after_job = deepcopy(before_job) if unchanged else job(after=True)
        job_calls, gets = [], 0
        with tempfile.TemporaryDirectory() as directory:
            env = environment() | {'RUNNER_TEMP': directory}
            path = Path(directory) / 'mypage-operator-release-record.json'

            def jobrequest(token, body=None, *, url=o.JOB_URL, missing_ok=False):
                nonlocal gets
                if body is not None:
                    self.assertEqual(url, o.JOB_PATCH_URL)
                    self.assertEqual(json.loads(path.read_bytes())['jobStatus'], 'pending')
                    job_calls.append(deepcopy(body))
                    return {'name': 'projects/' + o.c.PROJECT + '/locations/' + o.c.REGION +
                            '/operations/synthetic', 'done': True}
                if url != o.JOB_URL:
                    return {'name': url.removeprefix('https://run.googleapis.com/v2/'), 'done': True}
                gets += 1
                if moved and gets == 2:
                    return before_job | {'labels': {'foreign': 'change'}}
                value = deepcopy(after_job if job_calls else before_job)
                if superseded and job_calls and gets >= 4:
                    value['labels'] = {'foreign': 'change'}
                # A maintenance execution may change output-only counters.
                value['executionCount'] += 10
                value['latestCreatedExecution'] = {'name': 'executions/concurrent'}
                return value

            with patch.object(o, 'declaration', return_value=release()), \
                 patch.object(o.backend, 'verify_source'), patch.object(o, 'registry'), \
                 patch.object(o.hosting, 'verify_head'), \
                 patch.object(o, 'job_request', side_effect=jobrequest), patch.object(o.time, 'sleep'):
                if moved or superseded:
                    with self.assertRaises(ValueError):
                        o.deploy(env)
                else:
                    o.deploy(env)
            record = json.loads(path.read_bytes()) if path.exists() else None
        return job_calls, record

    def test_release_updates_job_once_and_records_previous_image(self):
        calls, record = self.execute()
        self.assertEqual(len(calls), 1)
        self.assertEqual(record['schemaVersion'], 1)
        self.assertEqual(record['jobStatus'], 'confirmed')
        self.assertEqual(record['readback'], 'passed')
        self.assertEqual(record['previousImageDigest'], legacy_image())
        self.assertEqual(record['imageDigest'], release()['image_digest'])

    def test_unchanged_job_is_recorded_without_write(self):
        calls, record = self.execute(unchanged=True)
        self.assertEqual(calls, [])
        self.assertEqual(record['jobStatus'], 'unchanged')
        self.assertEqual(record['readback'], 'passed')

    def test_competing_job_change_prevents_write_and_late_change_fails_readback(self):
        calls, record = self.execute(moved=True)
        self.assertEqual(calls, [])
        self.assertEqual(record['jobStatus'], 'pending')
        calls, record = self.execute(superseded=True)
        self.assertEqual(len(calls), 1)
        self.assertEqual(record['readback'], 'pending')


if __name__ == '__main__':
    unittest.main()
