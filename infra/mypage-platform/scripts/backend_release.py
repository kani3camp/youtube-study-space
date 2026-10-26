"""Source-pinned DEV image release. No secret reads, IAM writes or State access."""
from copy import deepcopy
import json
import os
from pathlib import Path
import re
import sys
import time

import foundation_contract as c
import hosting_release as hosting
import read_metrics_live as live
import read_metrics_v1 as v1
import runtime_contract as runtime

ROOT = Path(__file__).resolve().parents[3]
REGISTRY = ('https://artifactregistry.googleapis.com/v1/projects/' + c.PROJECT +
            '/locations/asia-southeast2/repositories/mypage/packages/mypage-api')
RELEASE = ROOT / 'infra/mypage-platform/environments/dev/runtime-release.json'


def context(env):
    hosting.context(env | {'MYPAGE_HOSTING_EXPECTED_SHA': env.get('MYPAGE_BACKEND_EXPECTED_SHA', '')})
    c.need(not any(env.get(key) for key in ('AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY',
                                          'AWS_SESSION_TOKEN')), 'backend-no-state-credentials')


def declaration():
    c.need(RELEASE.is_file() and not RELEASE.is_symlink() and RELEASE.stat().st_size <= 4096,
           'backend-release-file')
    value = json.loads(RELEASE.read_bytes())
    c.need(type(value) is dict and set(value) == {'image_digest', 'image_source_sha',
           'hosting_backend_host', 'previous_image_digest'}, 'backend-release-schema')
    pattern = re.escape(runtime.REPOSITORY) + r'@sha256:[a-f0-9]{64}'
    c.need(all(type(value[key]) is str and re.fullmatch(pattern, value[key])
               for key in ('image_digest', 'previous_image_digest')), 'backend-release-digest')
    c.need(value['image_digest'] != value['previous_image_digest'] and
           re.fullmatch('[a-f0-9]{40}', value['image_source_sha']) and
           value['hosting_backend_host'] == 'mypage-api-eu5mhkcewq-et.a.run.app',
           'backend-release-values')
    return value


def verify_source(env, value):
    hosting.verify_head(env)
    source = value['image_source_sha']
    hosting.command(['git', 'fetch', '--no-tags', '--depth=1', 'origin', source])
    # Compare the complete Docker context, not only the Go entrypoint.
    import subprocess
    def tree(ref):
        result = subprocess.run(['git', 'rev-parse', ref + ':system'], cwd=ROOT,
                                capture_output=True, text=True, timeout=30)
        c.need(result.returncode == 0 and re.fullmatch('[a-f0-9]{40}', result.stdout.strip()),
               'backend-source-tree')
        return result.stdout.strip()
    c.need(tree(source) == tree(env['GITHUB_SHA']), 'backend-source-tree')


def registry(token, value):
    import urllib.request
    request = urllib.request.Request(REGISTRY + '/tags/' + value['image_source_sha'],
                                     headers={'Authorization': 'Bearer ' + token})
    with urllib.request.build_opener(live.NoRedirect()).open(request, timeout=30) as response:
        data = response.read(65537)
    c.need(len(data) <= 65536, 'backend-registry-size')
    tag = json.loads(data)
    c.need(tag.get('version') == REGISTRY.removeprefix('https://artifactregistry.googleapis.com/v1/') +
           '/versions/' + value['image_digest'].split('@')[1], 'backend-source-digest')


def baseline(service, value, *, target=False):
    revision = live.ready(service)
    c.need(service.get('uri') == 'https://' + value['hosting_backend_host'], 'backend-uri')
    container = runtime.one(service['template'].get('containers'), 'backend-container')
    c.need(container.get('image') == value['image_digest' if target else 'previous_image_digest'],
           'backend-previous-image')
    entries = container.get('env', [])
    c.need(type(entries) is list and all(type(x) is dict and type(x.get('name')) is str for x in entries)
           and len({x['name'] for x in entries}) == len(entries), 'backend-env')
    values = {x['name']: x.get('value') for x in entries}
    c.need(re.fullmatch('[0-9]{6,20}', values.get('MYPAGE_PROJECT_NUMBER', '')), 'backend-project-number')
    c.need(values.get('MYPAGE_ENVIRONMENT') == 'development' and
           values.get('GOOGLE_CLOUD_PROJECT') == c.PROJECT and
           values.get('MYPAGE_PUBLIC_ORIGIN') == hosting.ORIGIN and
           values.get('MYPAGE_HOSTING_BACKEND_HOST') == value['hosting_backend_host'] and
           values.get('MYPAGE_READ_METRICS') in (None, 'false'), 'backend-runtime-boundary')
    c.need(service['template'].get('serviceAccount') == c.EMAIL and
           service['template'].get('scaling', {}).get('maxInstanceCount') == 1 and
           service['template'].get('scaling', {}).get('minInstanceCount', 0) == 0,
           'backend-runtime-boundary')
    c.need(service.get('scaling', {}).get('maxInstanceCount', 1) == 1 and
           service.get('scaling', {}).get('minInstanceCount', 0) == 0, 'backend-service-scaling')
    traffic = runtime.one(service.get('traffic'), 'backend-traffic')
    c.need(traffic == {'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 100},
           'backend-traffic')
    return revision, values.get('MYPAGE_PROJECT_NUMBER')


def payload(original, before, value):
    _, number = baseline(before, value)
    v1.identity(original, before | {'projectNumber': number})
    result = deepcopy(original)
    result.pop('status', None)
    template = result['spec']['template']
    c.need(not template.get('metadata', {}).get('name'), 'backend-fixed-revision')
    container = runtime.one(template['spec'].get('containers'), 'backend-v1-container')
    c.need(container.get('image') == value['previous_image_digest'], 'backend-v1-image')
    container['image'] = value['image_digest']
    return result


def validate_result(after, before, value):
    c.need(baseline(after, value, target=True)[0] != baseline(before, value)[0], 'backend-new-revision')
    c.need(after['uid'] == before['uid'] and int(after['generation']) == int(before['generation']) + 1
           and after['etag'] != before['etag'] and
           c.registry_timestamp(after['updateTime']) > c.registry_timestamp(before['updateTime']),
           'backend-result-generation')
    expected = deepcopy(before)
    expected['template']['containers'][0]['image'] = value['image_digest']
    c.need(live.api_configuration(after) == live.api_configuration(expected), 'backend-result-config')


def public_get(url):
    # Never follow redirects, retain application data or use real credentials.
    import urllib.error
    import urllib.request
    c.need(url in (hosting.ORIGIN + '/api/mypage',
                   'https://mypage-api-eu5mhkcewq-et.a.run.app/api/mypage'), 'backend-smoke-url')
    try:
        with urllib.request.build_opener(live.NoRedirect()).open(url, timeout=30) as response:
            return response.status, response.headers, response.read(65537)
    except urllib.error.HTTPError as error:
        return error.code, error.headers, error.read(65537)


def smoke():
    # The direct URL rejects at the Host boundary before App Check. Preserve
    # both application rejections; an arbitrary upstream 400/403 is not proof.
    for url, wanted_status, wanted_code in (
        (hosting.ORIGIN + '/api/mypage', 403, 'APP_CHECK_REQUIRED'),
        ('https://mypage-api-eu5mhkcewq-et.a.run.app/api/mypage', 400, 'INVALID_REQUEST'),
    ):
        status, headers, data = public_get(url)
        c.need(len(data) <= 65536 and status == wanted_status and
               json.loads(data).get('error', {}).get('code') == wanted_code and
               'no-store' in {part.strip().lower() for part in headers.get('Cache-Control', '').split(',')},
               'backend-anonymous-smoke')
    for route in ('/', '/mypage'):
        status, _, data = hosting.http(hosting.ORIGIN + route)
        c.need(status == 200 and b'<html' in data.lower(), 'backend-hosting-smoke')


def deploy(env):
    context(env)
    value = declaration()
    verify_source(env, value)
    token = env['MYPAGE_BACKEND_ACCESS_TOKEN']
    registry(token, value)
    before = live.request(token, live.SERVICE_URL)
    previous, number = baseline(before, value)
    original = v1.request(token)
    wanted = payload(original, before, value)
    hosting_before = hosting.current(token)
    # A second GET binds the v1 resourceVersion to the observed v2 configuration.
    c.need(live.request(token, live.SERVICE_URL) == before, 'backend-moved')
    hosting.verify_head(env)
    c.need(hosting.current(token) == hosting_before, 'backend-hosting-moved')
    record = {'schemaVersion': 1, 'environment': 'development', 'sourceSHA': env['GITHUB_SHA'],
              'workflowRun': 'https://github.com/' + c.REPOSITORY + '/actions/runs/' + env['GITHUB_RUN_ID'],
              'imageSourceSHA': value['image_source_sha'], 'imageDigest': value['image_digest'],
              'previousImageDigest': value['previous_image_digest'], 'previousRevision': previous,
              'hostingRelease': hosting_before['name'], 'hostingVersion': hosting_before['version']['name'],
              'backendRevision': None, 'publishStatus': 'unknown', 'publicReadback': 'pending',
              'stateReadback': 'pending', 'authenticatedE2E': 'unverified'}
    path = Path(env['RUNNER_TEMP']) / 'mypage-backend-release-record.json'
    def save():
        path.write_bytes(c.canonical(record))
    save()  # Persist recovery references before the only live mutation.
    result = v1.request(token, wanted)  # Original resourceVersion, no retry.
    c.need(result.get('metadata', {}).get('uid') == before['uid'] and
           str(result['metadata'].get('generation')) == str(int(before['generation']) + 1),
           'backend-update-response')
    record['publishStatus'] = 'accepted'
    save()
    deadline = time.monotonic() + 300
    while True:
        after = live.request(token, live.SERVICE_URL)
        if not after.get('reconciling', False):
            validate_result(after, before, value)
            break
        c.need(time.monotonic() < deadline, 'backend-ready-timeout')
        time.sleep(2)
    final_v1 = v1.request(token)
    c.need(v1.configuration(final_v1) == v1.configuration(wanted), 'backend-v1-result-config')
    v1.identity(final_v1, after | {'projectNumber': number})
    record.update(backendRevision=live.ready(after), publishStatus='confirmed')
    save()
    smoke()
    c.need(live.request(token, live.SERVICE_URL) == after, 'backend-superseded')
    c.need(hosting.current(token) == hosting_before, 'backend-hosting-moved')
    hosting.verify_head(env)
    record['publicReadback'] = 'passed'
    save()
    print('DEV Backend image release and anonymous smoke passed; State synchronization follows.')


def main():
    os.umask(0o077)
    try:
        c.need(sys.argv[1:] in (['context'], ['deploy']), 'backend-command')
        if sys.argv[1] == 'context':
            context(os.environ)
            declaration()
        else:
            deploy(os.environ)
        return 0
    except Exception as error:
        label = str(error) if type(error) in (ValueError, c.ContractError) and re.fullmatch('[a-z0-9-]+', str(error)) else 'execution'
        print('::error::DEV Backend STOP: ' + label + '. No automatic retry or rollback.', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
