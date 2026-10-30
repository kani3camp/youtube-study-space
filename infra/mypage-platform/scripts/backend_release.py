"""ソースを固定したDEVイメージ配備。秘密取得・IAM書き込み・Stateアクセスは行わない。"""
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
JOB_NAME = 'projects/' + c.PROJECT + '/locations/asia-southeast2/jobs/mypage-maintenance-operator'
JOB_URL = 'https://run.googleapis.com/v2/' + JOB_NAME


def context(env):
    event = env.get('GITHUB_EVENT_NAME')
    c.need(event in ('workflow_dispatch', 'push'), 'backend-event')
    hosting.context(env | {'MYPAGE_HOSTING_EXPECTED_SHA': env.get('MYPAGE_BACKEND_EXPECTED_SHA', '')},
                    event_name=event)
    c.need(not any(env.get(key) for key in ('AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY',
                                          'AWS_SESSION_TOKEN')), 'backend-no-state-credentials')
    digest = env.get('MYPAGE_BACKEND_IMAGE_DIGEST', '')
    source = env.get('MYPAGE_BACKEND_IMAGE_SOURCE_SHA', '')
    c.need(bool(digest) == bool(source), 'backend-image-pair')
    c.need(event != 'push' or (digest and source == env['GITHUB_SHA']),
           'backend-push-source')
    if digest:
        c.need(type(digest) is str and re.fullmatch(
            re.escape(runtime.REPOSITORY) + r'@sha256:[a-f0-9]{64}', digest),
            'backend-release-digest')
        c.need(type(source) is str and re.fullmatch(r'[a-f0-9]{40}', source),
               'backend-release-source')

def declaration(env, token=None):
    """Resolve a verified registry tag; never read or update a per-release JSON file."""
    source = env.get('MYPAGE_BACKEND_IMAGE_SOURCE_SHA') or env['GITHUB_SHA']
    digest = env.get('MYPAGE_BACKEND_IMAGE_DIGEST', '')
    if not digest and token:
        tag = registry_tag(token, source)
        version = tag.get('version', '')
        prefix = REGISTRY.removeprefix('https://artifactregistry.googleapis.com/v1/') + '/versions/'
        c.need(version.startswith(prefix + 'sha256:') and
               re.fullmatch(r'[a-f0-9]{64}', version.removeprefix(prefix + 'sha256:')),
               'backend-tag-digest')
        digest = runtime.REPOSITORY + '@' + version.removeprefix(prefix)
    if digest:
        c.need(re.fullmatch(re.escape(runtime.REPOSITORY) + r'@sha256:[a-f0-9]{64}', digest),
               'backend-release-digest')
    return {'image_digest': digest, 'image_source_sha': source,
            'hosting_backend_host': 'mypage-api-eu5mhkcewq-et.a.run.app'}

def verify_source(env, value):
    hosting.verify_head(env)
    source = value['image_source_sha']
    if source != env['GITHUB_SHA']:
        # An operator-approved rollback must use an immutable tag from an
        # ancestor commit, never an arbitrary image from another branch.
        hosting.command(['git', 'fetch', '--no-tags', '--depth=1', 'origin', source])
        hosting.command(['git', 'merge-base', '--is-ancestor', source, env['GITHUB_SHA']])

def registry_tag(token, source):
    import urllib.request
    request = urllib.request.Request(REGISTRY + '/tags/' + source,
                                     headers={'Authorization': 'Bearer ' + token})
    with urllib.request.build_opener(live.NoRedirect()).open(request, timeout=30) as response:
        data = response.read(65537)
    c.need(len(data) <= 65536, 'backend-registry-size')
    return json.loads(data)


def registry(token, value):
    tag = registry_tag(token, value['image_source_sha'])
    c.need(tag.get('version') == REGISTRY.removeprefix('https://artifactregistry.googleapis.com/v1/') +
           '/versions/' + value['image_digest'].split('@')[1], 'backend-source-digest')

def baseline(service, value, *, target=False):
    revision = live.ready(service)
    c.need(service.get('uri') == 'https://' + value['hosting_backend_host'], 'backend-uri')
    container = runtime.one(service['template'].get('containers'), 'backend-container')
    image = container.get('image')
    c.need(type(image) is str and re.fullmatch(
        re.escape(runtime.REPOSITORY) + r'@sha256:[a-f0-9]{64}', image),
        'backend-serving-image')
    if target:
        c.need(image == value['image_digest'], 'backend-target-image')
    entries = container.get('env', [])
    c.need(type(entries) is list and all(type(x) is dict and type(x.get('name')) is str for x in entries)
           and len({x['name'] for x in entries}) == len(entries), 'backend-env')
    values = {x['name']: x.get('value') for x in entries}
    c.need(values.get('MYPAGE_PROJECT_NUMBER') == c.PROJECT_NUMBER, 'backend-project-number')
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
    current = runtime.one(before['template'].get('containers'), 'backend-v2-container').get('image')
    c.need(container.get('image') == current, 'backend-v1-image')
    c.need(current != value['image_digest'], 'backend-already-serving')
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


def job_request(token, body=None):
    import urllib.request
    request = urllib.request.Request(
        JOB_URL,
        data=None if body is None else c.canonical(body),
        method='GET' if body is None else 'PATCH',
        headers={'Authorization': 'Bearer ' + token,
                 'Content-Type': 'application/json; charset=utf-8'})
    with urllib.request.build_opener(live.NoRedirect()).open(request, timeout=30) as response:
        data = response.read(65537)
    c.need(len(data) <= 65536, 'backend-job-response-size')
    return json.loads(data)


def job_baseline(job, value, *, target=False):
    c.need(job.get('name') == JOB_NAME and type(job.get('uid')) is str and job['uid'] and
           job.get('deletionProtection') is True and job.get('launchStage') == 'GA',
           'backend-job-boundary')
    template = job.get('template')
    c.need(type(template) is dict and template.get('taskCount') == 1 and template.get('parallelism') == 1,
           'backend-job-template')
    task = template.get('template')
    c.need(type(task) is dict and
           task.get('serviceAccount') == 'mypage-maintenance-job@' + c.PROJECT + '.iam.gserviceaccount.com' and
           task.get('maxRetries') == 0 and task.get('timeout') == '60s' and
           task.get('executionEnvironment') == 'EXECUTION_ENVIRONMENT_GEN2',
           'backend-job-task')
    container = runtime.one(task.get('containers'), 'backend-job-container')
    image = container.get('image')
    c.need(type(image) is str and re.fullmatch(
        re.escape(runtime.REPOSITORY) + r'@sha256:[a-f0-9]{64}', image),
        'backend-job-image')
    if target:
        c.need(image == value['image_digest'], 'backend-job-target-image')
    c.need(container.get('command') == ['/app/mypage-maintenance-operator'] and
           container.get('args') in (None, []), 'backend-job-command')
    entries = container.get('env')
    c.need(type(entries) is list and all(type(entry) is dict for entry in entries), 'backend-job-env')
    env = {entry.get('name'): entry.get('value') for entry in entries}
    c.need(len(env) == len(entries) and env == {
        'GOOGLE_CLOUD_PROJECT': c.PROJECT,
        'MYPAGE_ENVIRONMENT': 'development',
    }, 'backend-job-env')
    resources = container.get('resources', {})
    c.need(resources.get('limits') == {'cpu': '1', 'memory': '512Mi'}, 'backend-job-resources')
    return image


def job_payload(before, value):
    current = job_baseline(before, value)
    c.need(current != value['image_digest'], 'backend-job-already-serving')
    result = {'name': JOB_NAME, 'etag': before.get('etag'), 'template': deepcopy(before['template'])}
    c.need(type(result['etag']) is str and result['etag'], 'backend-job-etag')
    runtime.one(result['template']['template'].get('containers'), 'backend-job-container')['image'] = value['image_digest']
    return result


def validate_job_result(after, before, value):
    job_baseline(after, value, target=True)
    c.need(after['uid'] == before['uid'] and int(after['generation']) == int(before['generation']) + 1 and
           str(after.get('observedGeneration')) == str(after['generation']) and
           after.get('reconciling') is False and
           after.get('terminalCondition', {}).get('state') == 'CONDITION_SUCCEEDED' and
           after.get('etag') != before.get('etag') and
           c.registry_timestamp(after['updateTime']) > c.registry_timestamp(before['updateTime']),
           'backend-job-result')
    expected = deepcopy(before['template'])
    runtime.one(expected['template'].get('containers'), 'backend-job-container')['image'] = value['image_digest']
    c.need(after.get('template') == expected, 'backend-job-result-config')


def update_job(token, before, value):
    wanted = job_payload(before, value)
    job_request(token, wanted)  # etag-bound PATCH; no retry.
    deadline = time.monotonic() + 300
    while True:
        after = job_request(token)
        if not after.get('reconciling', False):
            validate_job_result(after, before, value)
            return after
        c.need(time.monotonic() < deadline, 'backend-job-ready-timeout')
        time.sleep(2)


def public_get(url):
    # リダイレクトを追わず、アプリケーションデータを保持せず、実認証情報を使わない。
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
    # 直接URLはApp Checkより前のHost検証で拒否される。両方のアプリケーション拒否を確認する。
    # 上流から返る任意の400/403だけでは証拠にならない。
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
    token = env['MYPAGE_BACKEND_ACCESS_TOKEN']
    value = declaration(env, token)
    verify_source(env, value)
    registry(token, value)
    before = live.request(token, live.SERVICE_URL)
    before_job = job_request(token)
    current_image = runtime.one(before['template'].get('containers'), 'backend-container').get('image')
    current_job_image = job_baseline(before_job, value)
    unchanged = current_image == value['image_digest']
    job_unchanged = current_job_image == value['image_digest']
    previous, number = baseline(before, value, target=unchanged)
    hosting_before = hosting.current(token)
    c.need(live.request(token, live.SERVICE_URL) == before, 'backend-moved')
    c.need(job_request(token) == before_job, 'backend-maintenance-job-moved')
    hosting.verify_head(env)
    c.need(hosting.current(token) == hosting_before, 'backend-hosting-moved')
    record = {'schemaVersion': 3, 'environment': 'development', 'sourceSHA': env['GITHUB_SHA'],
              'workflowRun': 'https://github.com/' + c.REPOSITORY + '/actions/runs/' + env['GITHUB_RUN_ID'],
              'imageSourceSHA': value['image_source_sha'], 'imageDigest': value['image_digest'],
              'previousImageDigest': current_image, 'previousRevision': previous,
              'previousMaintenanceJobImageDigest': current_job_image,
              'maintenanceJobImageDigest': current_job_image,
              'maintenanceJobStatus': 'unchanged' if job_unchanged else 'pending',
              'hostingRelease': hosting_before['name'], 'hostingVersion': hosting_before['version']['name'],
              'backendRevision': None, 'publishStatus': 'unknown', 'publicReadback': 'pending',
              'authenticatedE2E': 'unverified'}
    path = Path(env['RUNNER_TEMP']) / 'mypage-backend-release-record.json'

    def save():
        path.write_bytes(c.canonical(record))

    save()  # Persist recovery references before either Cloud Run resource mutates.
    after = before
    if unchanged:
        record.update(backendRevision=previous, publishStatus='unchanged')
        save()
    else:
        original = v1.request(token)
        wanted = payload(original, before, value)
        c.need(live.request(token, live.SERVICE_URL) == before, 'backend-moved')
        result = v1.request(token, wanted)  # Original resourceVersion; no retry.
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

    after_job = before_job
    if not job_unchanged:
        c.need(job_request(token) == before_job, 'backend-maintenance-job-moved')
        record['maintenanceJobStatus'] = 'accepted'
        save()
        after_job = update_job(token, before_job, value)
        record.update(maintenanceJobImageDigest=value['image_digest'], maintenanceJobStatus='confirmed')
        save()

    smoke()
    c.need(live.request(token, live.SERVICE_URL) == after, 'backend-superseded')
    c.need(job_request(token) == after_job, 'backend-maintenance-job-superseded')
    c.need(hosting.current(token) == hosting_before, 'backend-hosting-moved')
    hosting.verify_head(env)
    record['publicReadback'] = 'passed'
    save()
    print('DEV Backend Service and maintenance Job serve the verified immutable image; no Terraform operation.')


def main():
    os.umask(0o077)
    try:
        c.need(sys.argv[1:] in (['context'], ['deploy']), 'backend-command')
        if sys.argv[1] == 'context':
            context(os.environ)
            declaration(os.environ)
        else:
            deploy(os.environ)
        return 0
    except Exception as error:
        label = str(error) if type(error) in (ValueError, c.ContractError) and re.fullmatch('[a-z0-9-]+', str(error)) else 'execution'
        print('::error::DEV Backend STOP: ' + label + '. No automatic retry or rollback.', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
