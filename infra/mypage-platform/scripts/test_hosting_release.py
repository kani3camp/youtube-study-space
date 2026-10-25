"""Observable publication boundaries using an isolated filesystem and fake APIs."""
import copy
import json
from pathlib import Path
import tempfile
import time
import unittest
from unittest.mock import patch

import hosting_release as h


def environment():
    return {'GITHUB_REPOSITORY': h.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
            'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_ACTOR_ID': '54093651',
            'GITHUB_ACTOR': 'kani3camp', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
            'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_RUN_ATTEMPT': '1',
            'GITHUB_REF': h.REF, 'GITHUB_WORKFLOW_REF': h.CALLER,
            'MYPAGE_OTHER_OPERATION': 'false', 'GITHUB_SHA': 'a' * 40,
            'MYPAGE_HOSTING_EXPECTED_SHA': 'a' * 40, 'GITHUB_RUN_ID': '123',
            'MYPAGE_HOSTING_ACCESS_TOKEN': 'synthetic-token', 'GITHUB_TOKEN': 'synthetic-github'}


def hosting():
    return {'headers': [{'glob': '**', 'headers': {
        'Content-Security-Policy': "default-src 'none'", 'Cache-Control': 'no-store',
        'X-Content-Type-Options': 'nosniff', 'Referrer-Policy': 'no-referrer'}}],
        'rewrites': [{'glob': '/api/**', 'run': {'serviceId': 'mypage-api', 'region': 'asia-southeast2'}},
                     {'glob': '**', 'path': '/index.html'}]}


class ReleaseTest(unittest.TestCase):
    def test_rejects_wrong_context_before_cloud_access(self):
        h.context(environment())
        for key, value in [('GITHUB_REF', 'refs/heads/main'), ('GITHUB_EVENT_NAME', 'pull_request'),
                           ('GITHUB_RUN_ATTEMPT', '2'), ('GITHUB_ACTOR_ID', '123'),
                           ('MYPAGE_OTHER_OPERATION', 'true'), ('MYPAGE_HOSTING_EXPECTED_SHA', 'b' * 40),
                           ('GITHUB_WORKFLOW_REF', h.CALLER.replace('ci.yml', 'other.yml'))]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                h.context(environment() | {key: value})

    def make_dist(self, base):
        dist = base / 'dist'
        (dist / 'assets').mkdir(parents=True)
        for name, data in [('index.html', b'<!doctype html>'), ('assets/index-12345678.js', b'console.log(1)'),
                           ('assets/index-12345678.css', b'body{}')]:
            (dist / name).write_bytes(data)
        return dist

    def test_artifact_inventory_rejects_private_files_api_shadowing_and_maps(self):
        for name in ['.env', 'api/mypage', 'assets/index-12345678.js.map', 'private.json']:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temp:
                dist = self.make_dist(Path(temp))
                h.inventory(dist)
                target = dist / name
                target.parent.mkdir(exist_ok=True)
                target.write_bytes(b'synthetic-private')
                with self.assertRaises(ValueError):
                    h.inventory(dist)

    def test_artifact_binds_files_source_run_config_and_age(self):
        with tempfile.TemporaryDirectory() as temp:
            package = Path(temp)
            self.make_dist(package)
            config = {'synthetic': 'public-only'}
            rest = hosting()
            (package / 'public-config.json').write_bytes(h.canonical(config))
            (package / 'hosting.json').write_bytes(h.canonical(rest))
            (package / 'paths.json').write_bytes(h.canonical(sorted(h.inventory(package / 'dist'))))
            release = {'schemaVersion': 1, 'sourceSHA': 'a' * 40, 'runID': '123',
                       'createdAt': int(time.time()), 'files': h.inventory(package / 'dist'),
                       'publicConfigSHA256': h.sha(h.canonical(config)), 'hostingSHA256': h.sha(h.canonical(rest))}
            def save(value):
                (package / 'release.json').write_bytes(h.canonical(value))
                return environment() | {'MYPAGE_HOSTING_PACKAGE_DIGEST': h.sha(h.canonical(value))}
            env = save(release)
            with patch.object(h, 'translate', return_value=rest):
                h.validate_package(package, env)
                for key, value in [('sourceSHA', 'b' * 40), ('runID', '124'), ('createdAt', int(time.time()) - 1801)]:
                    with self.subTest(key=key), self.assertRaises(ValueError):
                        h.validate_package(package, save(release | {key: value}))
                save(release)
                (package / 'dist/index.html').write_bytes(b'changed')
                with self.assertRaises(ValueError):
                    h.validate_package(package, env)

    def test_authentication_never_follows_redirect_or_foreign_urls(self):
        with self.assertRaises(ValueError):
            h.http('https://attacker.invalid/', token='synthetic-private')
        with self.assertRaises(ValueError):
            h.NoRedirect().redirect_request(None, None, 307, '', {}, 'https://attacker.invalid/')

    def test_configuration_changes_are_not_hidden_by_asset_cache_updates(self):
        rest = hosting()
        more = copy.deepcopy(rest)
        more['headers'].append({'glob': '/assets/index-12345678.js', 'headers': {'Cache-Control': h.IMMUTABLE}})
        self.assertEqual(h.stable_config(rest), h.stable_config(more))
        more['headers'][0]['headers']['Content-Security-Policy'] += '; script-src *'
        self.assertNotEqual(h.stable_config(rest), h.stable_config(more))
        more['headers'][1]['headers']['Other'] = 'hidden'
        with self.assertRaises(ValueError):
            h.stable_config(more)

    def test_backend_accepts_omitted_false_but_requires_actual_ready_traffic(self):
        value = {'latestReadyRevision': 'projects/p/locations/l/services/s/revisions/r',
                 'latestCreatedRevision': 'projects/p/locations/l/services/s/revisions/r',
                 'terminalCondition': {'state': 'CONDITION_SUCCEEDED'},
                 'trafficStatuses': [{'revision': 'r', 'percent': 100}],
                 'uri': 'https://mypage-api-eu5mhkcewq-et.a.run.app'}
        with patch.object(h, 'cloud', return_value=value):
            self.assertEqual(h.backend('synthetic-token'), value)
        for change in [{'reconciling': True}, {'latestCreatedRevision': 'other'},
                       {'terminalCondition': {'state': 'CONDITION_FAILED'}},
                       {'trafficStatuses': [{'revision': 'foreign', 'percent': 100}]}]:
            with self.subTest(change=change), patch.object(h, 'cloud', return_value=value | change), self.assertRaises(ValueError):
                h.backend('synthetic-token')

    def publication(self, package, *, moved=False, upload_foreign=False, upload_failure=False, smoke_failure=False):
        env = environment() | {'RUNNER_TEMP': str(package.parent), 'MYPAGE_HOSTING_PACKAGE_DIGEST': 'b' * 64}
        rest = hosting()
        release = {'files': h.inventory(package / 'dist')}
        config = {'deployment': {'projectNumber': '123', 'privacyVersion': 'p', 'termsVersion': 't'}}
        api = {'latestReadyRevision': 'projects/p/locations/l/services/s/revisions/r',
               'template': {'containers': [{'env': [
                   {'name': 'MYPAGE_PRIVACY_VERSION', 'value': 'p'},
                   {'name': 'MYPAGE_TERMS_VERSION', 'value': 't'},
                   {'name': 'MYPAGE_PUBLIC_ORIGIN', 'value': h.ORIGIN}]}]}}
        old = {'name': 'sites/' + h.SITE + '/releases/old', 'version': {'name': 'sites/' + h.SITE + '/versions/old'}}
        name = 'sites/' + h.SITE + '/versions/new'
        calls = []
        state = {'current': old, 'reads': 0}
        def current(token):
            state['reads'] += 1
            if moved and state['reads'] == 2:
                return old | {'name': 'sites/' + h.SITE + '/releases/foreign'}
            return state['current']
        def request(url, token, *, method='GET', body=None):
            calls.append((method, url))
            if url.endswith('/sites/' + h.SITE):
                return {'name': 'projects/' + h.PROJECT + '/sites/' + h.SITE, 'defaultUrl': h.ORIGIN}
            if url.endswith('/versions/old'):
                return {'status': 'FINALIZED', 'config': rest}
            if url.endswith('/versions'):
                return {'name': name}
            if url.endswith(':populateFiles'):
                hashes = list(body['files'].values())
                return {'uploadUrl': 'https://attacker.invalid/' if upload_foreign else
                        'https://upload-firebasehosting.googleapis.com/upload/' + name + '/files',
                        'uploadRequiredHashes': hashes}
            if method == 'PATCH':
                return {'name': name, 'status': 'FINALIZED'}
            if '/releases?' in url:
                state['current'] = {'name': 'sites/' + h.SITE + '/releases/new', 'version': {'name': name}, 'type': 'DEPLOY'}
                return state['current']
            raise AssertionError('Unexpected fake cloud call')
        def upload(*args, **kwargs):
            if upload_failure:
                raise ValueError('cloud-http-503')
            return 200, {}, b''
        with patch.object(h, 'directory', return_value=package), patch.object(h, 'validate_package', return_value=(release, config, rest)), \
             patch.object(h, 'verify_head'), patch.object(h, 'current', side_effect=current), \
             patch.object(h, 'backend', return_value=api), patch.object(h, 'verify_public', side_effect=ValueError('smoke') if smoke_failure else None):
            try:
                h.deploy(env, request=request, transport=upload)
            except ValueError:
                self.assertTrue(moved or upload_foreign or upload_failure or smoke_failure)
            else:
                self.assertFalse(moved or upload_foreign or upload_failure or smoke_failure)
        return calls, package.parent / 'mypage-hosting-release-record.json'

    def test_stages_before_single_live_publish_and_preserves_rollback_receipt(self):
        with tempfile.TemporaryDirectory() as temp:
            package = Path(temp) / 'package'
            self.make_dist(package)
            calls, receipt = self.publication(package)
            writes = [url for method, url in calls if method == 'POST' and '/releases?' in url]
            self.assertEqual(len(writes), 1)
            self.assertEqual(json.loads(receipt.read_bytes())['publicReadback'], 'passed')
            self.assertEqual(json.loads(receipt.read_bytes())['previousVersion'], 'sites/' + h.SITE + '/versions/old')

    def test_moved_release_foreign_upload_or_failed_upload_cannot_publish(self):
        for option in ('moved', 'upload_foreign', 'upload_failure'):
            with self.subTest(option=option), tempfile.TemporaryDirectory() as temp:
                package = Path(temp) / 'package'
                self.make_dist(package)
                calls, receipt = self.publication(package, **{option: True})
                self.assertFalse(any('/releases?' in url for _, url in calls))
                self.assertFalse(receipt.exists())

    def test_smoke_failure_preserves_published_receipt_without_retry_or_rollback(self):
        with tempfile.TemporaryDirectory() as temp:
            package = Path(temp) / 'package'
            self.make_dist(package)
            calls, receipt = self.publication(package, smoke_failure=True)
            self.assertEqual(sum('/releases?' in url for _, url in calls), 1)
            self.assertEqual(json.loads(receipt.read_bytes())['publicReadback'], 'pending')


if __name__ == '__main__':
    unittest.main()
