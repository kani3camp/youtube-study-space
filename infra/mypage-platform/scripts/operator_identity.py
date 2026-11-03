"""DEV maintenance operator配備用の独立した権限を所有者が初期設定する。既定はプレビュー。

Job IAM（`run.jobs.get/update`）と実行SAへのactAsはTerraform（runtime module）が付与する。
このスクリプトはSA・WIF pool/provider・Environmentだけを作り、Cloud Run Job・API Service・
Terraform State・本番対象には触れない。既存名があれば停止し、部分実行後は復旧前に状態を調べる。
"""
import json
import sys

import backend_identity as backend
import foundation_contract as c
import operator_release as release

ACCOUNT_ID = 'mypage-operator-deploy'
ACCOUNT = ACCOUNT_ID + '@' + c.PROJECT + '.iam.gserviceaccount.com'
ENVIRONMENT = release.ENVIRONMENT
POOL = 'mypage-operator'
VARIABLE = 'MYPAGE_OPERATOR_WIF_PROVIDER'
PROJECT_ROLES = ('roles/serviceusage.serviceUsageConsumer',)


def condition():
    # Only the owner's first manual run of the reviewed Deploy workflow on the
    # integration ref. Pushes, PRs, other refs, identities, retries and prod are denied.
    values = {'repository_id': '340900071', 'repository_owner_id': '54093651',
              'actor_id': '54093651', 'actor': 'kani3camp', 'repository': c.REPOSITORY,
              'ref': c.REF, 'workflow_ref': release.CALLER, 'job_workflow_ref': release.REUSABLE,
              'event_name': 'workflow_dispatch', 'run_attempt': '1', 'environment': ENVIRONMENT}
    values['sub'] = ':'.join(key + ':' + values[key].replace(':', '%3A') for key in c.CLAIM_KEYS)
    return '(' + ' && '.join('assertion.' + key + ' == ' + json.dumps(value) for key, value in values.items()) + \
        ' && assertion.workflow_sha == assertion.job_workflow_sha)'


def member(number):
    return ('principal://iam.googleapis.com/projects/' + number + '/locations/global/workloadIdentityPools/' +
            POOL + '/subject/340900071:' + ENVIRONMENT + ':54093651')


def apply():
    command = backend.command
    c.need(c.TARGET == 'development', 'operator-bootstrap-target')
    common = ['--project=' + c.PROJECT, '--quiet', '--format=json']
    number = str(command('gcloud', ['projects', 'describe', c.PROJECT, '--format=json'])['projectNumber'])
    c.need(number == c.PROJECT_NUMBER, 'operator-bootstrap-project')
    # 最初の書き込み前に、候補の名前空間がすべて未使用であることを確認する。
    command('gcloud', ['iam', 'service-accounts', 'list', '--filter=email:' + ACCOUNT, *common], absent=True)
    command('gcloud', ['iam', 'workload-identity-pools', 'list', '--location=global',
                       '--filter=name:/' + POOL, *common], absent=True)
    envs = command('gh', ['api', 'repos/' + c.REPOSITORY + '/environments?per_page=100'])
    c.need(envs['total_count'] <= 100 and not any(e['name'] == ENVIRONMENT for e in envs['environments']),
           'operator-bootstrap-environment-collision')
    command('gcloud', ['iam', 'service-accounts', 'create', ACCOUNT_ID,
                       '--display-name=MyPage DEV maintenance operator CI', *common])
    command('gcloud', ['iam', 'workload-identity-pools', 'create', POOL, '--location=global',
                       '--display-name=MyPage DEV operator CI', *common])
    mapping = 'google.subject=assertion.repository_id+":"+assertion.environment+":"+assertion.actor_id'
    command('gcloud', ['iam', 'workload-identity-pools', 'providers', 'create-oidc', 'github-actions',
                       '--location=global', '--workload-identity-pool=' + POOL,
                       '--issuer-uri=https://token.actions.githubusercontent.com',
                       '--attribute-mapping=' + mapping, '--attribute-condition=' + condition(), *common])
    command('gcloud', ['iam', 'service-accounts', 'add-iam-policy-binding', ACCOUNT,
                       '--role=roles/iam.workloadIdentityUser', '--member=' + member(number), *common])
    sa_member = '--member=serviceAccount:' + ACCOUNT
    command('gcloud', ['artifacts', 'repositories', 'add-iam-policy-binding', 'mypage',
                       '--location=' + c.REGION, '--role=roles/artifactregistry.reader', sa_member, *common])
    for role in PROJECT_ROLES:
        command('gcloud', ['projects', 'add-iam-policy-binding', c.PROJECT,
                           '--role=' + role, sa_member, '--condition=None', *common])
    env_url = 'repos/' + c.REPOSITORY + '/environments/' + ENVIRONMENT
    command('gh', ['api', '--method', 'PUT', env_url, '--input', '-'], body={'deployment_branch_policy': {
        'protected_branches': False, 'custom_branch_policies': True}})
    command('gh', ['api', '--method', 'POST', env_url + '/deployment-branch-policies', '--input', '-'],
            body={'name': c.REF.removeprefix('refs/heads/'), 'type': 'branch'})
    provider = 'projects/' + number + '/locations/global/workloadIdentityPools/' + POOL + '/providers/github-actions'
    command('gh', ['api', '--method', 'POST', env_url + '/variables', '--input', '-'],
            body={'name': VARIABLE, 'value': provider})
    readback(number)
    print('DEV operator identity/Environment created and read back. Apply MyPage Terraform to grant Job access.')


def readback(number):
    command = backend.command
    common = ['--project=' + c.PROJECT, '--format=json']
    provider = command('gcloud', ['iam', 'workload-identity-pools', 'providers', 'describe',
                                  'github-actions', '--location=global', '--workload-identity-pool=' + POOL, *common])
    c.need(provider.get('attributeCondition') == condition() and provider.get('state') == 'ACTIVE' and
           provider.get('oidc', {}).get('issuerUri') == 'https://token.actions.githubusercontent.com' and
           not provider.get('disabled', False), 'operator-bootstrap-provider-readback')
    keys = command('gcloud', ['iam', 'service-accounts', 'keys', 'list', '--iam-account=' + ACCOUNT,
                              '--managed-by=user', *common])
    c.need(keys == [], 'operator-bootstrap-keys')
    sa_member = 'serviceAccount:' + ACCOUNT
    registry = command('gcloud', ['artifacts', 'repositories', 'get-iam-policy', 'mypage',
                                  '--location=' + c.REGION, *common])
    bindings = [b for b in registry.get('bindings', []) if sa_member in b.get('members', [])]
    c.need(len(bindings) == 1 and bindings[0].get('role') == 'roles/artifactregistry.reader'
           and not bindings[0].get('condition'), 'operator-bootstrap-binding-readback')
    project = command('gcloud', ['projects', 'get-iam-policy', c.PROJECT, '--format=json'])
    bindings = [b for b in project.get('bindings', []) if sa_member in b.get('members', [])]
    c.need({b['role'] for b in bindings} == set(PROJECT_ROLES) and len(bindings) == len(PROJECT_ROLES)
           and not any(b.get('condition') for b in bindings), 'operator-bootstrap-project-readback')
    policy = command('gcloud', ['iam', 'service-accounts', 'get-iam-policy', ACCOUNT, *common])
    c.need(policy.get('bindings') == [{'role': 'roles/iam.workloadIdentityUser', 'members': [member(number)]}],
           'operator-bootstrap-federation-readback')
    env_url = 'repos/' + c.REPOSITORY + '/environments/' + ENVIRONMENT
    environment = command('gh', ['api', env_url])
    c.need(environment.get('deployment_branch_policy') == {'protected_branches': False, 'custom_branch_policies': True},
           'operator-bootstrap-environment-readback')
    policies = command('gh', ['api', env_url + '/deployment-branch-policies'])
    c.need(policies['total_count'] == 1 and policies['branch_policies'][0]['name'] == c.REF.removeprefix('refs/heads/')
           and policies['branch_policies'][0]['type'] == 'branch', 'operator-bootstrap-branch-readback')


if __name__ == '__main__':
    try:
        c.need(sys.argv[1:] in ([], ['--apply']), 'operator-bootstrap-arguments')
        if sys.argv[1:] == ['--apply']:
            apply()
        else:
            print('Preview: new DEV-only SA/WIF pool+provider/Environment ' + ENVIRONMENT + '; no API calls.')
            print('Caller: ' + release.CALLER)
            print('Reusable: ' + release.REUSABLE)
            print('Grants here: DEV mypage registry reader; quota consumer. Job get/update and actAs: Terraform.')
    except Exception:
        print('DEV operator bootstrap stopped. No automatic retry; inspect any partial execution.', file=sys.stderr)
        sys.exit(1)
