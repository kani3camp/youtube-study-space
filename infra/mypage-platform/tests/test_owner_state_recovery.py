"""One exact stale-State refresh; no remote write, version drift or live race."""
from copy import deepcopy
import hashlib
import json
from pathlib import Path
import tempfile
import time
import unittest
from unittest.mock import patch

from test_runtime import fixture as base, service
from test_read_metrics_live import api_service, VERSION
import foundation_contract as c
import foundation_ops as ops
import runtime_contract as r
import read_metrics_live as live


def fixture():
    plan = base('runtime_public')
    for item in plan['resource_changes']:
        item['change'].update(actions=['no-op'])
        item['change'].pop('importing', None)
    value = plan['variables']['runtime_settings']['value']
    first = value['image_digest']
    value.update(previous_image_digest=r.REPOSITORY + '@sha256:' + 'b' * 64,
                 image_digest=r.REPOSITORY + '@sha256:' + 'c' * 64,
                 hosting_backend_host='mypage-api-observed-et.a.run.app')
    before = deepcopy(service(plan)['after'])
    before['template'][0]['revision'] = ''
    before['template'][0]['containers'][0]['env'].append({'name': 'MYPAGE_HOSTING_BACKEND_HOST', 'value': value['hosting_backend_host']})
    before.update(uid='synthetic-service-uid', etag='source-etag', generation='2', observed_generation='2',
                  update_time='2026-10-21T00:00:00Z', latest_ready_revision='mypage-api-old', latest_created_revision='mypage-api-old',
                  terminal_condition=[{'state': 'CONDITION_SUCCEEDED'}], reconciling=False,
                  traffic_statuses=[{'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 100, 'revision': 'mypage-api-old'}])
    after = deepcopy(before)
    after['template'][0]['containers'][0]['image'] = value['image_digest']
    after.update(etag='result-etag', generation='4', observed_generation='4', update_time='2026-10-24T00:00:00Z',
                 latest_ready_revision='mypage-api-new', latest_created_revision='mypage-api-new',
                 traffic_statuses=[{'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 100, 'revision': 'mypage-api-new'}])
    service(plan)['after'] = after
    drift = deepcopy(next(x for x in plan['resource_changes'] if x['address'] == r.SERVICE))
    drift['change'].update(actions=['update'], before=before)
    plan['resource_drift'] = [drift]
    receipt = dict(schema_version=1, project=c.PROJECT, service='mypage-api', observed_at='2026-10-24T01:00:00Z',
                   revision_configuration_equal_except_image=True, state_serial=8,
                   state_version_sha256=hashlib.sha256(c.canonical(VERSION)).hexdigest(), state_lineage_sha256='a' * 64,
                   service_uid_sha256=hashlib.sha256(before['uid'].encode()).hexdigest(),
                   live_etag_sha256=hashlib.sha256(after['etag'].encode()).hexdigest(),
                   chain=[dict(generation='2', revision='mypage-api-old', image=first),
                          dict(generation='3', revision='mypage-api-middle', image=value['previous_image_digest']),
                          dict(generation='4', revision='mypage-api-new', image=value['image_digest'])])
    return plan, receipt


class OwnerRecoveryTest(unittest.TestCase):
    def test_exact_refresh_preserves_full_plan_and_is_closed_without_receipt(self):
        plan, receipt = fixture()
        original = deepcopy(plan)
        with self.assertRaises(ValueError):
            r.validate_plan(plan, 'runtime_public')
        self.assertEqual(r.validate_plan(plan, 'runtime_public', receipt), {'resources': 12, 'creates': 0})
        self.assertEqual(plan, original)

    def test_checkpoint_and_configuration_mismatches_are_rejected(self):
        for mutate in (
            lambda p: p['resource_drift'][0]['change']['before'].update(generation='3'),
            lambda p: p['resource_drift'][0]['change']['before'].update(uid='other-service'),
            lambda p: p['resource_drift'][0]['change']['before'].update(latest_ready_revision='mypage-api-other'),
            lambda p: service(p)['after'].update(generation='5'),
            lambda p: service(p)['after']['template'][0].update(timeout='60s'),
            lambda p: service(p)['after'].update(ingress='INGRESS_TRAFFIC_INTERNAL_ONLY'),
            lambda p: service(p)['after'].update(traffic_statuses=[{'percent': 50}]),
            lambda p: service(p).update(actions=['update']),
            lambda p: service(p).update(after_unknown={'template': True}),
        ):
            p, receipt = fixture(); mutate(p)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                r.validate_plan(p, 'runtime_public', receipt)
        for mutate in (
            lambda x: x.update(project='other-project'),
            lambda x: x.update(extra=True),
            lambda x: x['chain'][1].update(generation='9'),
            lambda x: x['chain'][1].update(image=x['chain'][0]['image']),
            lambda x: x['chain'][2].update(revision='mypage-api-other'),
            lambda x: x.update(service_uid_sha256='a' * 64),
        ):
            p, receipt = fixture(); mutate(receipt)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                r.validate_plan(p, 'runtime_public', receipt)

    def test_fresh_live_and_exact_state_version_required_before_apply_and_live_after(self):
        plan, receipt = fixture(); api = api_service(service(plan)['after'])
        env = {'MYPAGE_GCP_ACCESS_TOKEN': 'synthetic-token'}
        with patch.object(live, 'request', return_value=api) as request:
            self.assertTrue(ops.verify_owner_recovery(env, plan, receipt, version=VERSION))
            request.assert_called_once_with('synthetic-token', live.SERVICE_URL)
        with patch.object(live, 'request') as request, self.assertRaises(ValueError):
            ops.verify_owner_recovery(env, plan, receipt, version=VERSION | {'VersionId': 'changed'})
        request.assert_not_called()
        for field, value in (('etag', 'raced'), ('uid', 'other'), ('generation', '5'),
                             ('latestReadyRevision', live.SERVICE_NAME + '/revisions/mypage-api-raced'),
                             ('trafficStatuses', [{'percent': 50}])):
            changed = deepcopy(api); changed[field] = value
            with self.subTest(field=field), patch.object(live, 'request', return_value=changed), self.assertRaises(ValueError):
                ops.verify_owner_recovery(env, plan, receipt, version=VERSION)
        state = {'values': {'root_module': {'resources': [{'address':r.SERVICE, 'values':service(plan)['after']}]}}}
        with patch.object(live, 'request', return_value=api):
            self.assertTrue(ops.verify_owner_recovery(env, plan, receipt, state=state))
        with patch.object(live, 'request', return_value=api | {'etag':'raced'}), self.assertRaises(ValueError):
            ops.verify_owner_recovery(env, plan, receipt, state=state)

    def test_saved_state_only_apply_brackets_live_checks_without_cloud_patch(self):
        plan, recovery = fixture(); api = api_service(service(plan)['after'])
        state = {'values': {'root_module': {'resources': [{'address':x['address'], 'values':x['change']['after']} for x in plan['resource_changes']]}}}
        events = []
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); saved = root / 'foundation.tfplan'; saved.write_bytes(b'saved-plan')
            (root / 'prepared.json').write_text(json.dumps({'sha256':hashlib.sha256(saved.read_bytes()).hexdigest(), 'source':'a' * 40}))
            def run(args, **kwargs):
                if args[1] == 'apply':
                    events.append('state-only-apply'); return ''
                if args[-1] == str(saved):
                    return json.dumps(plan)
                return json.dumps(state)
            def get(token, url):
                self.assertEqual(url, live.SERVICE_URL)
                events.append('live-get'); return deepcopy(api)
            env = {'MYPAGE_ROLE':'apply', 'MYPAGE_PLAN_TIME':str(time.time()), 'MYPAGE_MODE':'runtime-public-apply',
                   'GITHUB_SHA':'a' * 40, 'MYPAGE_GCP_ACCESS_TOKEN':'synthetic-token'}
            with patch.object(c, 'context'), patch.object(ops, 'directory', return_value=root), patch.object(ops, 'run', side_effect=run), \
                 patch.object(ops, 'owner_recovery_receipt', return_value=recovery), patch.object(ops, 'state_version', return_value=VERSION), \
                 patch.object(live, 'request', side_effect=get):
                ops.apply_saved(env)
        self.assertEqual(events, ['live-get', 'state-only-apply', 'live-get'])


if __name__ == '__main__':
    unittest.main()
