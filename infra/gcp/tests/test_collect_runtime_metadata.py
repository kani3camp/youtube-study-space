"""The operator collector rejects incomplete reads and never publishes inputs."""
import contextlib
import copy
import io
import json
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest
from unittest.mock import patch
import urllib.error

SCRIPTS = Path(__file__).resolve().parents[1] / 'scripts'
sys.path.insert(0, str(SCRIPTS))
import collect_runtime_metadata as collector


def fixture():
    number = '111111111111'
    pool = 'projects/' + number + '/locations/global/workloadIdentityPools/aws-runtime'
    provider = pool + '/providers/aws-provider'
    email = collector.PROJECT + '@appspot.gserviceaccount.com'
    selectors = dict(schema_version=1, project_id=collector.PROJECT, project_number=number,
        runtime_pool_resource=pool, runtime_provider_resource=provider,
        runtime_service_account_email=email, runtime_iam_policy_resource='projects/' + collector.PROJECT + '/serviceAccounts/' + email,
        runtime_audience='//iam.googleapis.com/' + provider, lambda6_source_match=True, ecs_source_match=False)
    metadata = dict(
        project=dict(projectId=collector.PROJECT, projectNumber=number, lifecycleState='ACTIVE'),
        pool=dict(name=pool, state='ACTIVE', mode='FEDERATION_ONLY'),
        provider=dict(name=provider, state='ACTIVE', aws={'accountId': '222222222222'},
            attributeMapping={'google.subject': 'assertion.arn', 'attribute.account': 'assertion.account', 'attribute.aws_role': 'assertion.arn'},
            attributeCondition='assertion.account == "222222222222"'),
        attestation={},
        policy=dict(version=3, etag='dummy-etag', bindings=[dict(role='roles/iam.workloadIdentityUser',
            members=['private-fixture-member'], condition=dict(title='dummy-title', expression='true'))]),
        services=dict(services=[dict(name='projects/' + number + '/services/iam.googleapis.com',
            config={'name': 'iam.googleapis.com'}, state='ENABLED')]),
        history=dict(tableReference={'projectId': collector.PROJECT, 'datasetId': 'firestore_export', 'tableId': 'user-activity-history'},
            type='TABLE', location='asia-southeast2', schema={'fields': json.loads(collector.validate_history.__globals__['SCHEMA'].read_text())}))
    calls = []
    def request(host, path, method='GET'):
        calls.append((host, path, method))
        if host == 'cloudresourcemanager.googleapis.com': key = 'project'
        elif host == 'serviceusage.googleapis.com': key = 'services'
        elif host == 'bigquery.googleapis.com': key = 'history'
        elif ':listAttestationRules' in path: key = 'attestation'
        elif ':getIamPolicy' in path: key = 'policy'
        elif '/providers/' in path: key = 'provider'
        else: key = 'pool'
        return copy.deepcopy(metadata[key])
    return selectors, metadata, request, calls


class CollectionTests(unittest.TestCase):
    def test_complete_capture_keeps_full_policy_and_uses_only_seven_exact_interfaces(self):
        selectors, metadata, request, calls = fixture()
        capture = collector.collect(selectors, request)
        self.assertEqual(capture['iam_policy'], metadata['policy'])
        self.assertEqual(capture['history_table'], metadata['history'])
        self.assertEqual(len(calls), 7)
        self.assertEqual([method for _, _, method in calls].count('POST'), 1)
        self.assertTrue(next(path for _, path, method in calls if method == 'POST').endswith(
            ':getIamPolicy?options.requestedPolicyVersion=3'))
        self.assertFalse(any('testIamPermissions' in path or 'list?' in path for _, path, _ in calls))

    def test_attestation_unknown_nonempty_and_incomplete_shapes_stop(self):
        for value in ({'unknown': []}, {'attestationRules': [{}]}, {'nextPageToken': 'pending'},
                      {'attestationRules': None}, {'nextPageToken': False}, {'error': {}}):
            with self.subTest(value=value):
                selectors, metadata, request, _ = fixture()
                metadata['attestation'] = value
                with self.assertRaises(ValueError): collector.collect(selectors, request)

    def test_unknown_pool_provider_policy_and_incorrect_identity_stop(self):
        for kind, key, value in (('pool', 'futureTrust', {}), ('provider', 'oidc', {}),
                                 ('policy', 'unknown', True), ('project', 'projectNumber', '333333333333'),
                                 ('pool', 'disabled', True), ('provider', 'state', 'DELETED')):
            with self.subTest(kind=kind, key=key):
                selectors, metadata, request, _ = fixture()
                metadata[kind][key] = value
                with self.assertRaises(ValueError): collector.collect(selectors, request)

    def test_legacy_history_schema_cannot_become_success(self):
        selectors, metadata, request, _ = fixture()
        metadata['history']['schema']['fields'].append({'name': 'timestamp', 'type': 'TIMESTAMP'})
        with self.assertRaises(ValueError): collector.collect(selectors, request)

    def test_services_complete_pages_are_joined_and_loop_or_duplicates_stop(self):
        selectors, metadata, base, calls = fixture()
        pages = [{'nextPageToken': 'page /?1', 'services': metadata['services']['services']},
                 {'services': []}]
        def request(host, path, **kwargs):
            if host == 'serviceusage.googleapis.com':
                calls.append((host, path, 'GET'))
                return copy.deepcopy(pages.pop(0))
            return base(host, path, **kwargs)
        collector.collect(selectors, request)
        self.assertIn('pageToken=page%20%2F%3F1', calls[-2][1])
        for bad in ({'services': [], 'nextPageToken': 'loop'},
                    {'services': [metadata['services']['services'][0]]}, {'unknown': []}, {'nextPageToken': 1}):
            with self.subTest(bad=bad):
                def failure(host, path, **kwargs):
                    if host == 'serviceusage.googleapis.com': return copy.deepcopy(bad)
                    return base(host, path, **kwargs)
                if bad == {'services': [metadata['services']['services'][0]]}:
                    # A duplicate within one complete page is also rejected.
                    bad['services'] *= 2
                with self.assertRaises(ValueError): collector.collect(selectors, failure)

    def test_unique_token_pagination_ceiling_is_not_complete(self):
        selectors, _, base, _ = fixture()
        pages = iter(range(collector.MAX_PAGES))
        def request(host, path, **kwargs):
            if host == 'serviceusage.googleapis.com': return {'nextPageToken': str(next(pages))}
            return base(host, path, **kwargs)
        with self.assertRaises(ValueError): collector.collect(selectors, request)


class PrivateBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.root.chmod(0o700)
        self.selectors, self.metadata, self.request, _ = fixture()
        self.path = self.root / 'selectors.json'
        self.path.write_text(json.dumps(self.selectors))
        self.path.chmod(0o600)

    def run_main(self, request=None):
        out, err = io.StringIO(), io.StringIO()
        def transport(token, host, path, **kwargs):
            self.assertEqual(token, 'private-fixture-token')
            return (request or self.request)(host, path, **kwargs)
        with patch.object(collector, 'google_token', return_value='private-fixture-token') as auth, \
             patch.object(collector, 'transport', side_effect=transport), \
             contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            result = collector.main(['--private-root', str(self.root), '--selectors-file', str(self.path)])
        self.token_call_count = auth.call_count
        public = out.getvalue() + err.getvalue()
        for secret in ('private-fixture-token', 'private-fixture-member', 'dummy-etag', self.selectors['project_number']):
            self.assertNotIn(secret, public)
        return result, public

    def test_success_is_exclusive_private_and_never_readiness(self):
        self.assertEqual(self.run_main()[0], 0)
        target = self.root / collector.OUTPUT
        self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o600)
        value = json.loads(target.read_text())
        self.assertTrue(value['complete'])
        self.assertFalse(value['migration_ready'])
        self.assertFalse(value['ci_identity_read_verified'])
        original = target.read_bytes()
        self.assertEqual(self.run_main()[0], 1)
        self.assertEqual(target.read_bytes(), original)

    def test_failed_read_cleans_reserved_file_and_prints_no_dependency_body(self):
        def fail(*args, **kwargs): raise ValueError('private-fixture-token private-fixture-member')
        result, public = self.run_main(fail)
        self.assertEqual(result, 1)
        self.assertIn('STOP:', public)
        self.assertFalse((self.root / collector.OUTPUT).exists())

    def test_nonprivate_symlink_checkout_and_malformed_selectors_stop_before_auth(self):
        self.path.chmod(0o644)
        self.assertEqual(self.run_main()[0], 1)
        self.assertEqual(self.token_call_count, 0)
        self.path.chmod(0o600)
        self.root.chmod(0o755)
        self.assertEqual(self.run_main()[0], 1)
        self.assertEqual(self.token_call_count, 0)
        self.root.chmod(0o700)
        (self.root / '.git').write_text('gitdir: fixture')
        self.assertEqual(self.run_main()[0], 1)
        self.assertEqual(self.token_call_count, 0)
        (self.root / '.git').unlink()
        self.path.unlink()
        self.path.symlink_to('/dev/null')
        self.assertEqual(self.run_main()[0], 1)
        self.assertEqual(self.token_call_count, 0)

    def test_output_symlink_is_never_followed_or_unlinked(self):
        target = self.root / collector.OUTPUT
        target.symlink_to(self.path)
        self.assertEqual(self.run_main()[0], 1)
        self.assertTrue(target.is_symlink())
        self.assertEqual(json.loads(self.path.read_text()), self.selectors)

    def test_unknown_selectors_missing_input_and_symlink_root_reject_before_token(self):
        for change in ({'unexpected': True}, {'schema_version': True}, {'project_id': 'other-project'}):
            value = dict(self.selectors, **change)
            self.path.write_text(json.dumps(value))
            with patch.object(collector, 'google_token') as token:
                with contextlib.redirect_stderr(io.StringIO()):
                    self.assertEqual(collector.main(['--private-root', str(self.root), '--selectors-file', str(self.path)]), 1)
                token.assert_not_called()
        public = io.StringIO()
        with contextlib.redirect_stderr(public):
            self.assertEqual(collector.main(['--unexpected-secret-arg', 'private-fixture-token']), 1)
        self.assertNotIn('private-fixture-token', public.getvalue())
        link = self.root / 'link'; link.symlink_to(self.root, target_is_directory=True)
        with self.assertRaises(ValueError): collector.private_root(link)

    def test_duplicate_json_keys_oversize_and_nonobject_stop(self):
        for raw in (b'{"schema_version":1,"schema_version":1}', b'[]', b'{"value":NaN}', b' ' * (collector.MAX_BYTES + 1)):
            with self.subTest(raw_length=len(raw)):
                with self.assertRaises(ValueError): collector.decode(raw)

    def test_transport_forbids_redirects_nonread_methods_and_leaks_no_http_body(self):
        with self.assertRaises(ValueError):
            collector.NoRedirect().redirect_request(None, None, 302, '', {}, 'https://other.invalid')
        for host, path, method in [('other.invalid', 'v1/projects/dummy', 'GET'),
                                    ('iam.googleapis.com', 'v1/projects/dummy', 'DELETE')]:
            with self.assertRaises(ValueError): collector.transport('dummy', host, path, method)
        error = urllib.error.HTTPError('https://iam.googleapis.com/private-fixture-member', 403,
            'private-fixture-token', {}, io.BytesIO(b'private-fixture-token private-fixture-member'))
        with patch.object(collector.urllib.request, 'build_opener') as opener:
            opener.return_value.open.side_effect = error
            with self.assertRaisesRegex(ValueError, '^metadata HTTP STOP$'):
                collector.transport('dummy', 'iam.googleapis.com', 'v1/projects/dummy')

if __name__ == '__main__': unittest.main()
