"""Observable DEV release/competition/failure boundaries; no cloud credentials."""
from copy import deepcopy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import backend_release as b
import backend_identity as identity


def environment():
    return {'GITHUB_REPOSITORY': b.c.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
            'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_ACTOR_ID': '54093651',
            'GITHUB_ACTOR': 'kani3camp', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
            'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_RUN_ATTEMPT': '1',
            'GITHUB_REF': b.c.REF, 'GITHUB_WORKFLOW_REF': b.c.CALLER,
            'GITHUB_SHA': 'a' * 40, 'GITHUB_RUN_ID': '123', 'MYPAGE_OTHER_OPERATION': 'false',
            'MYPAGE_BACKEND_EXPECTED_SHA': 'a' * 40, 'MYPAGE_BACKEND_ACCESS_TOKEN': 'synthetic-token'}


def release():
    return {'image_source_sha': 'a' * 40, 'image_digest': b.runtime.REPOSITORY + '@sha256:' + 'b' * 64,
            'previous_image_digest': b.runtime.REPOSITORY + '@sha256:' + 'c' * 64,
            'hosting_backend_host': 'mypage-api-eu5mhkcewq-et.a.run.app'}


def service(*, after=False):
    value = release()
    revision = 'mypage-api-00002-new' if after else 'mypage-api-00001-old'
    env = {'GOOGLE_CLOUD_PROJECT': b.c.PROJECT, 'MYPAGE_ENVIRONMENT': 'development',
           'MYPAGE_PROJECT_NUMBER': '123456789', 'MYPAGE_PUBLIC_ORIGIN': b.hosting.ORIGIN,
           'MYPAGE_HOSTING_BACKEND_HOST': value['hosting_backend_host']}
    return {'name': b.live.SERVICE_NAME, 'uid': 'synthetic-uid', 'etag': 'new' if after else 'old',
            'generation': '2' if after else '1', 'observedGeneration': '2' if after else '1',
            'updateTime': '2026-10-26T01:00:01Z' if after else '2026-10-26T01:00:00Z',
            'uri': 'https://' + value['hosting_backend_host'],
            'terminalCondition': {'state': 'CONDITION_SUCCEEDED'},
            'latestReadyRevision': revision, 'latestCreatedRevision': revision,
            'traffic': [{'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 100}],
            'trafficStatuses': [{'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 100,
                                 'revision': revision}],
            'futureConfig': {'preserve': True}, 'ingress': 'INGRESS_TRAFFIC_ALL',
            'template': {'serviceAccount': b.c.EMAIL, 'scaling': {'maxInstanceCount': 1},
                         'containers': [{'image': value['image_digest' if after else 'previous_image_digest'],
                                         'env': [{'name': k, 'value': v} for k, v in env.items()] +
                                         [{'name': 'MYPAGE_OAUTH_CLIENT_SECRET', 'valueSource': {
                                             'secretKeyRef': {'secret': 'synthetic-secret-ref', 'version': '1'}}}]}]}}


def v1service(*, after=False):
    api = service(after=after)
    return {'apiVersion': 'serving.knative.dev/v1', 'kind': 'Service',
            'metadata': {'uid': api['uid'], 'generation': int(api['generation']), 'name': 'mypage-api',
                         'namespace': b.c.PROJECT, 'resourceVersion': 'new-rv' if after else 'old-rv'},
            'spec': {'template': {'spec': {'containers': deepcopy(api['template']['containers'])}},
                     'preserve': {'future': True}},
            'status': {'latestReadyRevisionName': api['latestReadyRevision'],
                       'latestCreatedRevisionName': api['latestCreatedRevision'],
                       'conditions': [{'type': 'Ready', 'status': 'True'}]}}


class BackendReleaseTest(unittest.TestCase):
    def test_new_identity_is_bound_to_exact_dev_source_actor_and_environment(self):
        condition = identity.condition()
        for text in ('assertion.ref == ' + json.dumps(b.c.REF),
                     'assertion.job_workflow_ref == ' + json.dumps(identity.REUSABLE),
                     'assertion.actor_id == "54093651"', 'assertion.run_attempt == "1"',
                     'assertion.event_name == "workflow_dispatch"',
                     'assertion.environment == "mypage-dev-backend"',
                     'assertion.workflow_sha == assertion.job_workflow_sha'):
            self.assertIn(text, condition)
        self.assertEqual(set(identity.PERMISSIONS), {'run.services.get', 'run.services.update'})
        self.assertNotIn('mypage-foundation', condition)

    def test_rejects_unauthorized_context_before_any_cloud_access(self):
        b.context(environment())
        for key, value in [('GITHUB_REF', 'refs/heads/main'), ('GITHUB_ACTOR_ID', '0'),
                           ('GITHUB_RUN_ATTEMPT', '2'), ('GITHUB_EVENT_NAME', 'pull_request'),
                           ('MYPAGE_OTHER_OPERATION', 'true'), ('MYPAGE_BACKEND_EXPECTED_SHA', 'b' * 40),
                           ('AWS_ACCESS_KEY_ID', 'synthetic-private')]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                b.context(environment() | {key: value})

    def test_only_image_changes_and_original_resource_version_is_preserved(self):
        original = v1service()
        payload = b.payload(original, service(), release())
        expected = deepcopy(original)
        expected.pop('status')
        expected['spec']['template']['spec']['containers'][0]['image'] = release()['image_digest']
        self.assertEqual(payload, expected)
        self.assertEqual(original, v1service())
        self.assertEqual(payload['metadata']['resourceVersion'], 'old-rv')

    def test_new_revision_must_preserve_all_config_including_unknown_fields(self):
        b.validate_result(service(after=True), service(), release())
        changes = [lambda x: x['template']['containers'][0]['env'].pop(),
                   lambda x: x['template']['scaling'].update(maxInstanceCount=3),
                   lambda x: x['futureConfig'].update(preserve=False),
                   lambda x: x['template'].update(serviceAccount='other'),
                   lambda x: x.update(generation='3'),
                   lambda x: x['traffic'][0].update(percent=90)]
        for mutate in changes:
            after = service(after=True)
            mutate(after)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                b.validate_result(after, service(), release())

    def execute(self, *, moved=False, conflict=False, lost=False, unhealthy=False, smoke=False):
        before, after = service(), service(after=True)
        if unhealthy:
            after['terminalCondition']['state'] = 'CONDITION_FAILED'
        calls = []
        with tempfile.TemporaryDirectory() as directory:
            env = environment() | {'RUNNER_TEMP': directory}
            path = Path(directory) / 'mypage-backend-release-record.json'
            def v1request(token, body=None):
                if body is None:
                    return v1service(after=bool(calls))
                calls.append(deepcopy(body))
                self.assertTrue(path.is_file())
                self.assertEqual(json.loads(path.read_bytes())['publishStatus'], 'unknown')
                if conflict or lost:
                    raise ValueError('synthetic-conflict' if conflict else 'synthetic-lost-response')
                return v1service(after=True)
            host = {'name': 'sites/test--study-space/releases/old',
                    'version': {'name': 'sites/test--study-space/versions/old'}}
            sequence = [before, before | {'etag': 'foreign'} if moved else before, after, after]
            with patch.object(b, 'declaration', return_value=release()), patch.object(b, 'verify_source'), \
                 patch.object(b, 'registry'), patch.object(b.hosting, 'verify_head'), \
                 patch.object(b.hosting, 'current', return_value=host), \
                 patch.object(b.live, 'request', side_effect=sequence), \
                 patch.object(b.v1, 'request', side_effect=v1request), \
                 patch.object(b, 'smoke', side_effect=ValueError('synthetic-smoke') if smoke else None):
                if any((moved, conflict, lost, unhealthy, smoke)):
                    with self.assertRaises(ValueError):
                        b.deploy(env)
                else:
                    b.deploy(env)
            record = json.loads(path.read_bytes()) if path.exists() else None
        return calls, record

    def test_release_updates_once_and_records_revision_digest_pair(self):
        calls, record = self.execute()
        self.assertEqual(len(calls), 1)
        self.assertEqual(record['publishStatus'], 'confirmed')
        self.assertEqual(record['publicReadback'], 'passed')
        self.assertEqual(record['stateReadback'], 'pending')
        self.assertEqual(record['previousImageDigest'], release()['previous_image_digest'])

    def test_competing_configuration_prevents_write(self):
        calls, record = self.execute(moved=True)
        self.assertEqual(calls, [])
        self.assertIsNone(record)

    def test_conflict_and_lost_response_never_retry_and_preserve_recovery_record(self):
        for option in ('conflict', 'lost'):
            with self.subTest(option=option):
                calls, record = self.execute(**{option: True})
                self.assertEqual(len(calls), 1)
                self.assertEqual(record['publishStatus'], 'unknown')
                self.assertEqual(record['previousRevision'], 'mypage-api-00001-old')

    def test_unhealthy_or_failed_smoke_stops_without_rollback(self):
        for option in ('unhealthy', 'smoke'):
            with self.subTest(option=option):
                calls, record = self.execute(**{option: True})
                self.assertEqual(len(calls), 1)
                self.assertEqual(record['publicReadback'], 'pending')
                self.assertEqual(record['stateReadback'], 'pending')


if __name__ == '__main__':
    unittest.main()
