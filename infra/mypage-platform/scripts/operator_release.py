"""ソースを固定したDEV maintenance operator Job image配備。

Cloud Run Job `mypage-maintenance-operator` のimageだけを更新する。API Service・Hosting・
Terraform・State・IAMには触れず、Jobを実行しない（実行はmaintenance modeを変更するため）。
"""
from copy import deepcopy
import json
import os
from pathlib import Path
import re
import sys
import time

import backend_release as backend
import foundation_contract as c
import hosting_release as hosting
import read_metrics_live as live
import runtime_contract as runtime

COMPONENT = c.IMAGE_COMPONENTS['maintenance-operator']
CALLER = COMPONENT['caller']
REUSABLE = c.workflow_ref('mypage-operator-authenticated.yml')
ENVIRONMENT = 'mypage-dev-operator'
PACKAGE = ('projects/' + c.PROJECT + '/locations/' + c.REGION + '/repositories/mypage/packages/' +
           COMPONENT['package'])
REGISTRY = 'https://artifactregistry.googleapis.com/v1/' + PACKAGE
DIGEST = re.escape(runtime.OPERATOR_REPOSITORY) + r'@sha256:[a-f0-9]{64}'
JOB_NAME = 'projects/' + c.PROJECT + '/locations/' + c.REGION + '/jobs/mypage-maintenance-operator'
JOB_URL = 'https://run.googleapis.com/v2/' + JOB_NAME
JOB_PATCH_URL = JOB_URL + '?allowMissing=false'
JOB_OPERATION = re.compile(r'projects/' + re.escape(c.PROJECT) + r'/locations/' + re.escape(c.REGION) +
                           r'/operations/[A-Za-z0-9_-]+')
JOB_OUTPUTS = frozenset({
    '@type', 'uid', 'generation', 'observedGeneration', 'createTime', 'updateTime',
    'deleteTime', 'expireTime', 'creator', 'lastModifier', 'terminalCondition',
    'conditions', 'executionCount', 'latestCreatedExecution', 'reconciling',
    'satisfiesPzs', 'startExecutionToken', 'runExecutionToken',
})


def context(env):
    """Reject every context except the owner's first manual run of the reviewed workflow."""
    c.need(c.TARGET == 'development', 'operator-target')
    expected = {
        'GITHUB_REPOSITORY': c.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
        'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_ACTOR_ID': '54093651',
        'GITHUB_ACTOR': 'kani3camp', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
        'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_RUN_ATTEMPT': '1',
        'GITHUB_REF': c.REF, 'GITHUB_WORKFLOW_REF': CALLER,
    }
    c.need(all(env.get(k) == v for k, v in expected.items()), 'operator-context')
    c.need(re.fullmatch('[a-f0-9]{40}', env.get('GITHUB_SHA', '')), 'operator-source-sha')
    c.need(env.get('MYPAGE_OPERATOR_EXPECTED_SHA') == env['GITHUB_SHA'], 'operator-expected-sha')
    c.need(re.fullmatch('[0-9]+', env.get('GITHUB_RUN_ID', '')), 'operator-run-id')
    c.need(not any(env.get(key) for key in ('AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY',
                                          'AWS_SESSION_TOKEN')), 'operator-no-state-credentials')
    digest = env.get('MYPAGE_OPERATOR_IMAGE_DIGEST', '')
    source = env.get('MYPAGE_OPERATOR_IMAGE_SOURCE_SHA', '')
    c.need(bool(digest) == bool(source), 'operator-image-pair')
    if digest:
        c.need(re.fullmatch(DIGEST, digest), 'operator-release-digest')
        c.need(re.fullmatch(r'[a-f0-9]{40}', source), 'operator-release-source')


def registry_tag(token, source):
    import urllib.request
    request = urllib.request.Request(REGISTRY + '/tags/' + source,
                                     headers={'Authorization': 'Bearer ' + token})
    with urllib.request.build_opener(live.NoRedirect()).open(request, timeout=30) as response:
        data = response.read(65537)
    c.need(len(data) <= 65536, 'operator-registry-size')
    return json.loads(data)


def declaration(env, token=None):
    """Resolve the exact-source tag of the operator package; never read a release JSON file."""
    source = env.get('MYPAGE_OPERATOR_IMAGE_SOURCE_SHA') or env['GITHUB_SHA']
    digest = env.get('MYPAGE_OPERATOR_IMAGE_DIGEST', '')
    if not digest and token:
        version = registry_tag(token, source).get('version', '')
        prefix = PACKAGE + '/versions/'
        c.need(re.fullmatch(re.escape(prefix) + r'sha256:[a-f0-9]{64}', version), 'operator-tag-digest')
        digest = runtime.OPERATOR_REPOSITORY + '@' + version.removeprefix(prefix)
    if digest:
        c.need(re.fullmatch(DIGEST, digest), 'operator-release-digest')
    return {'image_digest': digest, 'image_source_sha': source}


def registry(token, value):
    tag = registry_tag(token, value['image_source_sha'])
    c.need(tag.get('version') == PACKAGE + '/versions/' + value['image_digest'].split('@')[1],
           'operator-source-digest')


def job_mutable_configuration(job):
    """Return the complete writable Job resource while excluding only documented output metadata."""
    c.need(type(job) is dict and job.get('name') == JOB_NAME, 'operator-job-boundary')
    result = deepcopy({key: item for key, item in job.items()
                       if key not in JOB_OUTPUTS and key != 'etag'})
    c.need(result.get('name') == JOB_NAME and type(result.get('template')) is dict,
           'operator-job-boundary')
    return result


def job_configuration(job):
    """Stable comparison that ignores executions, reconciliation status and etag."""
    return live.compact(job_mutable_configuration(job))


def job_request(token, body=None, *, url=JOB_URL, missing_ok=False):
    import urllib.error
    import urllib.request
    operation = url.removeprefix('https://run.googleapis.com/v2/')
    allowed = url in (JOB_URL, JOB_PATCH_URL) or JOB_OPERATION.fullmatch(operation)
    c.need(bool(allowed), 'operator-job-url')
    c.need(type(token) is str and bool(token), 'operator-job-credential')
    c.need((body is None and url != JOB_PATCH_URL) or (body is not None and url == JOB_PATCH_URL),
           'operator-job-url')
    request = urllib.request.Request(
        url,
        data=None if body is None else c.canonical(body),
        method='GET' if body is None else 'PATCH',
        headers={'Authorization': 'Bearer ' + token,
                 'Content-Type': 'application/json; charset=utf-8'})
    try:
        with urllib.request.build_opener(live.NoRedirect()).open(request, timeout=30) as response:
            data = response.read(1024 * 1024 + 1)
    except urllib.error.HTTPError as error:
        if missing_ok and body is None and url == JOB_URL and error.code == 404:
            return None
        label = 'operator-job-conflict' if error.code in (409, 412) else 'operator-job-api-failed'
        raise c.ContractError(label) from None
    c.need(len(data) <= 1024 * 1024, 'operator-job-response-size')
    value = json.loads(data)
    c.need(type(value) is dict, 'operator-job-response')
    return value


def job_baseline(job, value, *, target=False):
    c.need(type(job) is dict and job.get('name') == JOB_NAME and
           type(job.get('uid')) is str and job['uid'] and
           not job.get('deleteTime') and not job.get('expireTime') and
           job.get('launchStage') == 'GA', 'operator-job-boundary')
    generation = job.get('generation')
    c.need(type(generation) is str and generation.isdigit() and int(generation) >= 1 and
           job.get('observedGeneration') == generation and
           job.get('reconciling', False) is False and
           job.get('terminalCondition', {}).get('state') == 'CONDITION_SUCCEEDED',
           'operator-job-ready')
    etag = job.get('etag')
    c.need(type(etag) is str and 0 < len(etag) <= 1024 and '\n' not in etag and '\r' not in etag,
           'operator-job-etag')
    template = job.get('template')
    c.need(type(template) is dict and template.get('taskCount') == 1 and template.get('parallelism') == 1,
           'operator-job-template')
    task = template.get('template')
    c.need(type(task) is dict and
           task.get('serviceAccount') == 'mypage-maintenance-job@' + c.PROJECT + '.iam.gserviceaccount.com' and
           task.get('maxRetries') == 0 and task.get('timeout') == '60s' and
           task.get('executionEnvironment') == 'EXECUTION_ENVIRONMENT_GEN2',
           'operator-job-task')
    container = runtime.one(task.get('containers'), 'operator-job-container')
    image = container.get('image')
    # The current image may still be the legacy API-bundled package; a release
    # target is always the dedicated operator package (checked in context()).
    c.need(type(image) is str and re.fullmatch(runtime.MAINTENANCE_JOB_IMAGE, image),
           'operator-job-image')
    if target:
        c.need(image == value['image_digest'], 'operator-job-target-image')
    c.need(container.get('command') == ['/app/mypage-maintenance-operator'] and
           container.get('args') in (None, []), 'operator-job-command')
    entries = container.get('env')
    c.need(type(entries) is list and all(type(entry) is dict for entry in entries), 'operator-job-env')
    env = {entry.get('name'): entry.get('value') for entry in entries}
    c.need(len(env) == len(entries) and env == {
        'GOOGLE_CLOUD_PROJECT': c.PROJECT,
        'MYPAGE_ENVIRONMENT': 'development',
    }, 'operator-job-env')
    resources = container.get('resources', {})
    c.need(resources.get('limits') == {'cpu': '1', 'memory': '512Mi'}, 'operator-job-resources')
    return image


def job_payload(before, value):
    current = job_baseline(before, value)
    c.need(current != value['image_digest'], 'operator-job-already-serving')
    result = job_mutable_configuration(before)
    result['etag'] = before['etag']
    runtime.one(result['template']['template'].get('containers'),
                'operator-job-container')['image'] = value['image_digest']
    return result


def validate_job_result(after, before, value):
    job_baseline(after, value, target=True)
    c.need(after['uid'] == before['uid'] and
           int(after['generation']) == int(before['generation']) + 1 and
           after['etag'] != before['etag'] and
           c.registry_timestamp(after['updateTime']) > c.registry_timestamp(before['updateTime']),
           'operator-job-result')
    expected = job_mutable_configuration(before)
    runtime.one(expected['template']['template'].get('containers'),
                'operator-job-container')['image'] = value['image_digest']
    c.need(job_configuration(after) == live.compact(expected), 'operator-job-result-config')


def same_job_configuration(current, expected):
    return (type(current) is dict and current.get('uid') == expected.get('uid') and
            job_configuration(current) == job_configuration(expected))


def start_job_update(token, before, value):
    operation = job_request(token, job_payload(before, value), url=JOB_PATCH_URL)
    name = operation.get('name')
    c.need(type(name) is str and JOB_OPERATION.fullmatch(name), 'operator-job-operation')
    c.need(not operation.get('error'), 'operator-job-operation')
    return operation


def wait_job_update(token, operation, before, value):
    deadline = time.monotonic() + 300
    current = operation
    while not current.get('done', False):
        c.need(time.monotonic() < deadline, 'operator-job-ready-timeout')
        time.sleep(2)
        current = job_request(token, url='https://run.googleapis.com/v2/' + operation['name'])
    c.need(not current.get('error'), 'operator-job-operation')
    while True:
        after = job_request(token)
        generation_advanced = (type(after.get('generation')) is str and after['generation'].isdigit() and
                               int(after['generation']) > int(before['generation']))
        if generation_advanced and not after.get('reconciling', False):
            validate_job_result(after, before, value)
            return after
        c.need(time.monotonic() < deadline, 'operator-job-ready-timeout')
        time.sleep(2)



def deploy(env):
    context(env)
    token = env['MYPAGE_OPERATOR_ACCESS_TOKEN']
    value = declaration(env, token)
    # Rollback may only select an immutable tag built from an ancestor commit.
    backend.verify_source(env, value)
    registry(token, value)
    before = job_request(token)
    current = job_baseline(before, value)
    unchanged = current == value['image_digest']
    record = {'schemaVersion': 1, 'environment': 'development', 'sourceSHA': env['GITHUB_SHA'],
              'workflowRun': 'https://github.com/' + c.REPOSITORY + '/actions/runs/' + env['GITHUB_RUN_ID'],
              'imageSourceSHA': value['image_source_sha'], 'imageDigest': value['image_digest'],
              'previousImageDigest': current, 'previousGeneration': before['generation'],
              'jobStatus': 'unchanged' if unchanged else 'pending', 'readback': 'pending'}
    path = Path(env['RUNNER_TEMP']) / 'mypage-operator-release-record.json'

    def save():
        path.write_bytes(c.canonical(record))

    save()  # Persist the recovery reference before the Job mutates.
    after = before
    if not unchanged:
        c.need(same_job_configuration(job_request(token), before), 'operator-job-moved')
        operation = start_job_update(token, before, value)
        record['jobStatus'] = 'accepted'
        save()
        after = wait_job_update(token, operation, before, value)
        record['jobStatus'] = 'confirmed'
        save()
    c.need(same_job_configuration(job_request(token), after), 'operator-job-superseded')
    hosting.verify_head(env)
    record['readback'] = 'passed'
    save()
    print('DEV maintenance operator Job uses the verified immutable image; not executed, API untouched.')


def main():
    os.umask(0o077)
    try:
        c.need(sys.argv[1:] in (['context'], ['deploy']), 'operator-command')
        if sys.argv[1] == 'context':
            context(os.environ)
            declaration(os.environ)
        else:
            deploy(os.environ)
        return 0
    except Exception as error:
        label = str(error) if type(error) in (ValueError, c.ContractError) and re.fullmatch('[a-z0-9-]+', str(error)) else 'execution'
        print('::error::DEV Operator STOP: ' + label + '. No automatic retry or rollback.', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
