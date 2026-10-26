"""Owner bootstrap for the isolated DEV deployer; preview is the default.

No runtime secret values, existing Terraform identities or production targets.
Existing names stop the bootstrap; inspect partial execution before recovery.
"""
import json
import os
import shutil
import subprocess
import sys

import foundation_contract as c

ACCOUNT = 'mypage-backend-deploy@' + c.PROJECT + '.iam.gserviceaccount.com'
ENVIRONMENT = 'mypage-dev-backend'
POOL = 'mypage-backend'
ROLE = 'mypageBackendRelease'
PERMISSIONS = ['run.services.get', 'run.services.update']
REUSABLE = c.REPOSITORY + '/.github/workflows/mypage-backend-authenticated.yml@' + c.REF


def condition():
    values = {'repository_id': '340900071', 'repository_owner_id': '54093651',
              'actor_id': '54093651', 'actor': 'kani3camp', 'repository': c.REPOSITORY,
              'ref': c.REF, 'workflow_ref': c.CALLER, 'job_workflow_ref': REUSABLE,
              'event_name': 'workflow_dispatch', 'run_attempt': '1', 'environment': ENVIRONMENT}
    subject = ':'.join(key + ':' + value.replace(':', '%3A') for key, value in (
        ('repository_id', '340900071'), ('repository_owner_id', '54093651'),
        ('environment', ENVIRONMENT), ('ref', c.REF), ('workflow_ref', c.CALLER),
        ('job_workflow_ref', REUSABLE), ('event_name', 'workflow_dispatch')))
    values['sub'] = subject
    return ' && '.join('assertion.' + key + ' == ' + json.dumps(value) for key, value in values.items()) + \
        ' && assertion.workflow_sha == assertion.job_workflow_sha'


def command(tool, arguments, *, body=None, absent=False):
    executable = (shutil.which(tool + '.cmd') or shutil.which(tool)) if os.name == 'nt' else shutil.which(tool)
    c.need(executable is not None, 'backend-bootstrap-tool')
    result = subprocess.run([executable, *arguments], input=json.dumps(body) if body is not None else None,
                            text=True, capture_output=True, timeout=120)
    if absent:
        # Absence must be proven by a completed list, never inferred from 403.
        c.need(result.returncode == 0 and json.loads(result.stdout) == [], 'backend-bootstrap-collision')
        return
    c.need(result.returncode == 0, 'backend-bootstrap-command')
    return json.loads(result.stdout) if result.stdout.strip() else None


def apply():
    common = ['--project=' + c.PROJECT, '--quiet', '--format=json']
    number = str(command('gcloud', ['projects', 'describe', c.PROJECT, '--format=json'])['projectNumber'])
    c.need(number.isdigit(), 'backend-bootstrap-project')
    # Verify each proposed namespace is free before the first write.
    command('gcloud', ['iam', 'service-accounts', 'list', '--filter=email:' + ACCOUNT, *common], absent=True)
    command('gcloud', ['iam', 'workload-identity-pools', 'list', '--location=global',
                       '--filter=name:/' + POOL, *common], absent=True)
    command('gcloud', ['iam', 'roles', 'list', '--filter=name:/' + ROLE, *common], absent=True)
    envs = command('gh', ['api', 'repos/' + c.REPOSITORY + '/environments?per_page=100'])
    c.need(envs['total_count'] <= 100 and not any(e['name'] == ENVIRONMENT for e in envs['environments']),
           'backend-bootstrap-environment-collision')
    command('gcloud', ['iam', 'service-accounts', 'create', 'mypage-backend-deploy',
                       '--display-name=MyPage DEV Backend CI', *common])
    command('gcloud', ['iam', 'roles', 'create', ROLE, '--title=MyPage DEV Backend release',
                       '--permissions=' + ','.join(PERMISSIONS), '--stage=GA', *common])
    command('gcloud', ['iam', 'workload-identity-pools', 'create', POOL, '--location=global',
                       '--display-name=MyPage DEV Backend CI', *common])
    mapping = 'google.subject=assertion.repository_id+":"+assertion.environment+":"+assertion.actor_id'
    command('gcloud', ['iam', 'workload-identity-pools', 'providers', 'create-oidc', 'github-actions',
                       '--location=global', '--workload-identity-pool=' + POOL,
                       '--issuer-uri=https://token.actions.githubusercontent.com',
                       '--attribute-mapping=' + mapping, '--attribute-condition=' + condition(), *common])
    member = ('principal://iam.googleapis.com/projects/' + number + '/locations/global/workloadIdentityPools/' +
              POOL + '/subject/340900071:' + ENVIRONMENT + ':54093651')
    command('gcloud', ['iam', 'service-accounts', 'add-iam-policy-binding', ACCOUNT,
                       '--role=roles/iam.workloadIdentityUser', '--member=' + member, *common])
    sa_member = '--member=serviceAccount:' + ACCOUNT
    command('gcloud', ['run', 'services', 'add-iam-policy-binding', 'mypage-api',
                       '--region=asia-southeast2', '--role=projects/' + c.PROJECT + '/roles/' + ROLE,
                       sa_member, *common])
    command('gcloud', ['iam', 'service-accounts', 'add-iam-policy-binding', c.EMAIL,
                       '--role=roles/iam.serviceAccountUser', sa_member, *common])
    command('gcloud', ['artifacts', 'repositories', 'add-iam-policy-binding', 'mypage',
                       '--location=asia-southeast2', '--role=roles/artifactregistry.reader', sa_member, *common])
    for role in ('roles/firebasehosting.viewer', 'roles/serviceusage.serviceUsageConsumer'):
        command('gcloud', ['projects', 'add-iam-policy-binding', c.PROJECT,
                           '--role=' + role, sa_member, '--condition=None', *common])
    command('gh', ['api', '--method', 'PUT', 'repos/' + c.REPOSITORY + '/environments/' + ENVIRONMENT,
                   '--input', '-'], body={'deployment_branch_policy': {
                       'protected_branches': False, 'custom_branch_policies': True}})
    command('gh', ['api', '--method', 'POST', 'repos/' + c.REPOSITORY + '/environments/' + ENVIRONMENT +
                   '/deployment-branch-policies', '--input', '-'], body={'name': c.REF.removeprefix('refs/heads/'),
                                                                       'type': 'branch'})
    provider = ('projects/' + number + '/locations/global/workloadIdentityPools/' + POOL + '/providers/github-actions')
    command('gh', ['api', '--method', 'POST', 'repos/' + c.REPOSITORY + '/environments/' + ENVIRONMENT +
                   '/variables', '--input', '-'], body={'name': 'MYPAGE_BACKEND_WIF_PROVIDER', 'value': provider})
    readback(number)
    print('DEV Backend identity/Environment created and read back. No Cloud Run image update performed.')


def readback(number):
    common = ['--project=' + c.PROJECT, '--format=json']
    provider = command('gcloud', ['iam', 'workload-identity-pools', 'providers', 'describe',
                                  'github-actions', '--location=global', '--workload-identity-pool=' + POOL, *common])
    c.need(provider.get('attributeCondition') == condition() and provider.get('state') == 'ACTIVE' and
           provider.get('oidc', {}).get('issuerUri') == 'https://token.actions.githubusercontent.com' and
           not provider.get('disabled', False), 'backend-bootstrap-provider-readback')
    role = command('gcloud', ['iam', 'roles', 'describe', ROLE, *common])
    c.need(set(role['includedPermissions']) == set(PERMISSIONS), 'backend-bootstrap-role-readback')
    keys = command('gcloud', ['iam', 'service-accounts', 'keys', 'list', '--iam-account=' + ACCOUNT,
                              '--managed-by=user', *common])
    c.need(keys == [], 'backend-bootstrap-keys')
    member = 'serviceAccount:' + ACCOUNT
    checks = [(['run', 'services', 'get-iam-policy', 'mypage-api', '--region=asia-southeast2'],
               'projects/' + c.PROJECT + '/roles/' + ROLE),
              (['iam', 'service-accounts', 'get-iam-policy', c.EMAIL], 'roles/iam.serviceAccountUser'),
              (['artifacts', 'repositories', 'get-iam-policy', 'mypage', '--location=asia-southeast2'],
               'roles/artifactregistry.reader')]
    for arguments, expected in checks:
        policy = command('gcloud', [*arguments, *common])
        bindings = [b for b in policy.get('bindings', []) if member in b.get('members', [])]
        c.need(len(bindings) == 1 and bindings[0].get('role') == expected and not bindings[0].get('condition'),
               'backend-bootstrap-binding-readback')
    project = command('gcloud', ['projects', 'get-iam-policy', c.PROJECT, '--format=json'])
    bindings = [b for b in project.get('bindings', []) if member in b.get('members', [])]
    c.need({b['role'] for b in bindings} == {'roles/firebasehosting.viewer', 'roles/serviceusage.serviceUsageConsumer'}
           and len(bindings) == 2 and not any(b.get('condition') for b in bindings),
           'backend-bootstrap-project-readback')
    policy = command('gcloud', ['iam', 'service-accounts', 'get-iam-policy', ACCOUNT, *common])
    expected_member = ('principal://iam.googleapis.com/projects/' + number + '/locations/global/workloadIdentityPools/' +
                       POOL + '/subject/340900071:' + ENVIRONMENT + ':54093651')
    c.need(policy.get('bindings') == [{'role': 'roles/iam.workloadIdentityUser', 'members': [expected_member]}],
           'backend-bootstrap-federation-readback')
    env_url = 'repos/' + c.REPOSITORY + '/environments/' + ENVIRONMENT
    environment = command('gh', ['api', env_url])
    c.need(environment.get('deployment_branch_policy') == {'protected_branches': False, 'custom_branch_policies': True}
           and environment.get('protection_rules') == [environment['protection_rules'][0]]
           and environment['protection_rules'][0].get('type') == 'branch_policy',
           'backend-bootstrap-environment-readback')
    policies = command('gh', ['api', env_url + '/deployment-branch-policies'])
    c.need(policies['total_count'] == 1 and policies['branch_policies'][0]['name'] == c.REF.removeprefix('refs/heads/')
           and policies['branch_policies'][0]['type'] == 'branch', 'backend-bootstrap-branch-readback')


if __name__ == '__main__':
    try:
        c.need(sys.argv[1:] in ([], ['--apply']), 'backend-bootstrap-arguments')
        if sys.argv[1:] == ['--apply']:
            apply()
        else:
            print('Preview: new DEV-only SA/custom role/WIF pool+provider/Environment; no API calls.')
            print('Service: mypage-api; permissions: ' + ', '.join(PERMISSIONS))
            print('Runtime SA: actAs; DEV mypage registry: reader; DEV Hosting: viewer; quota consumer.')
            print('IAM can change more than an image: trusted workflow enforces the image-only contract.')
    except Exception:
        print('DEV Backend bootstrap stopped. No automatic retry; inspect any partial execution.', file=sys.stderr)
        sys.exit(1)
