#!/usr/bin/env python3
"""Owner bootstrap for the Core Terraform PR Plan identity. Preview by default.

PR Plan runs the PR author's workflow code, so the identity itself is the blast
radius: a read-only GCP SA (the existing `terraformNormalPlan` read role) in a
dedicated WIF pool, and an AWS role that can only read the State object. It is
separate from both the manual Plan and the Apply identities.

`--apply --target dev|prod` creates or updates only the PR Plan pool/provider,
SA, its two bindings and the GitHub Environment `terraform-<target>-pr-plan`.
AWS is never mutated here: `--aws-policies` prints the reviewed trust and
permission documents for the owner to apply. No keys are created.
"""
import argparse
import json
import re
import subprocess
import sys

REPO = 'kani3camp/youtube-study-space'
WORKFLOWS = REPO + '/.github/workflows/'
PROJECTS = {'dev': 'test-youtube-study-space', 'prod': 'youtube-study-space'}
POOL = 'terraform-pr-plan'
PROVIDER = 'github-pr-plan'
PR_REF = r'^refs/pull/[1-9][0-9]{0,8}/merge$'
CLAIM_KEYS = ('repository_id', 'repository_owner_id', 'environment', 'ref',
              'workflow_ref', 'job_workflow_ref', 'event_name')
MAPPING = {
    'google.subject': "assertion.repository_id + ':' + assertion.repository_owner_id + ':' + assertion.environment",
    **{'attribute.' + k: 'assertion.' + k for k in ('environment', 'event_name', 'job_workflow_ref',
       'ref', 'repository_id', 'repository_owner_id', 'workflow_ref')},
}
CONDITION_LIMIT = 4096


def need(value):
    if not value:
        raise ValueError('PR Plan bootstrap rejected')


def environment(target):
    need(target in PROJECTS)
    return 'terraform-' + target + '-pr-plan'


def account(target):
    return 'terraform-' + target + '-normal-pr-plan@' + PROJECTS[target] + '.iam.gserviceaccount.com'


def condition(target):
    # The ref differs per PR, so callers are bound to the token's own ref
    # instead of a list of exact subjects. Push, dispatch, forks (no OIDC),
    # other actors, retries and the other target's Environment are denied.
    condition = ' && '.join([
        "assertion.repository_id == '340900071'", "assertion.repository_owner_id == '54093651'",
        "assertion.actor_id == '54093651'", "assertion.actor == 'kani3camp'",
        "assertion.event_name == 'pull_request'", "assertion.run_attempt == '1'",
        "assertion.ref.matches('" + PR_REF + "')",
        "assertion.workflow_ref == '" + WORKFLOWS + "terraform.yml@' + assertion.ref",
        "assertion.job_workflow_ref == '" + WORKFLOWS + "terraform-pr-plan.yml@' + assertion.ref",
        "assertion.environment == '" + environment(target) + "'",
        'assertion.workflow_sha == assertion.sha', 'assertion.job_workflow_sha == assertion.sha',
    ])
    need(len(condition) <= CONDITION_LIMIT)
    return condition


def aws_trust_policy(target, state_account):
    need(re.fullmatch(r'[0-9]{12}', state_account) is not None)
    claims = {'repository_id': '340900071', 'repository_owner_id': '54093651',
              'environment': environment(target), 'ref': 'refs/pull/*/merge',
              'workflow_ref': WORKFLOWS + 'terraform.yml@refs/pull/*/merge',
              'job_workflow_ref': WORKFLOWS + 'terraform-pr-plan.yml@refs/pull/*/merge',
              'event_name': 'pull_request'}
    subject = ':'.join(f"{key}:{claims[key].replace(':', '%3A')}" for key in CLAIM_KEYS)
    return {'Version': '2012-10-17', 'Statement': [{
        'Sid': 'OwnerSameRepositoryPullRequestPlan', 'Effect': 'Allow',
        'Principal': {'Federated': f'arn:aws:iam::{state_account}:oidc-provider/token.actions.githubusercontent.com'},
        'Action': 'sts:AssumeRoleWithWebIdentity',
        'Condition': {
            'StringEquals': {'token.actions.githubusercontent.com:aud': 'sts.amazonaws.com',
                             'token.actions.githubusercontent.com:actor_id': '54093651'},
            # Only the PR number is a wildcard; every other claim is fixed.
            'StringLike': {'token.actions.githubusercontent.com:sub': subject},
        },
    }]}


def aws_state_read_policy(target, bucket):
    need(target in PROJECTS and re.fullmatch(r'[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]', bucket) is not None)
    key = f'youtube-study-space/{target}/terraform.tfstate'
    arn = 'arn:aws:s3:::' + bucket
    # Read the State only: no PutObject/DeleteObject on the State or its lock
    # (PR Plan runs with -lock=false); listing proves the lock is absent.
    return {'Version': '2012-10-17', 'Statement': [
        {'Sid': 'StateRead', 'Effect': 'Allow', 'Action': ['s3:GetObject'], 'Resource': arn + '/' + key},
        {'Sid': 'StateDiscovery', 'Effect': 'Allow', 'Action': 's3:ListBucket', 'Resource': arn,
         'Condition': {'StringEquals': {'s3:prefix': [key, key + '.tflock',
                                                      key.rsplit('/', 1)[0] + '/workspaces/']}}},
    ]}


def run(args, *, data=None, missing=False):
    result = subprocess.run(args, input=data, capture_output=True, text=True, timeout=120)
    if missing and result.returncode != 0 and 'NOT_FOUND' in result.stderr:
        return None
    need(result.returncode == 0)
    return json.loads(result.stdout) if result.stdout.strip() else {}


def gcloud(target, *args, missing=False):
    return run(['gcloud', *args, '--project=' + PROJECTS[target], '--format=json', '--quiet'], missing=missing)


def apply(target):
    project = gcloud(target, 'projects', 'describe', PROJECTS[target])
    need(project.get('projectId') == PROJECTS[target] and project.get('lifecycleState') == 'ACTIVE')
    number = project['projectNumber']
    role = f'projects/{PROJECTS[target]}/roles/terraformNormalPlan'
    # Reuse the reviewed read-only Plan role; refuse if it gained any write verb.
    permissions = gcloud(target, 'iam', 'roles', 'describe', 'terraformNormalPlan')['includedPermissions']
    need(permissions and all(p.rsplit('.', 1)[-1] in {'get', 'list', 'getIamPolicy', 'getAttestationRules'}
                             for p in permissions))
    if not gcloud(target, 'iam', 'workload-identity-pools', 'describe', POOL, '--location=global', missing=True):
        gcloud(target, 'iam', 'workload-identity-pools', 'create', POOL, '--location=global',
               '--display-name=Terraform PR Plan')
    existing = gcloud(target, 'iam', 'workload-identity-pools', 'providers', 'describe', PROVIDER,
                      '--workload-identity-pool=' + POOL, '--location=global', missing=True)
    gcloud(target, 'iam', 'workload-identity-pools', 'providers', 'update-oidc' if existing else 'create-oidc',
           PROVIDER, '--workload-identity-pool=' + POOL, '--location=global',
           '--issuer-uri=https://token.actions.githubusercontent.com',
           '--attribute-mapping=' + ','.join(k + '=' + v for k, v in MAPPING.items()),
           '--attribute-condition=' + condition(target))
    provider = gcloud(target, 'iam', 'workload-identity-pools', 'providers', 'describe', PROVIDER,
                      '--workload-identity-pool=' + POOL, '--location=global')
    need(provider['attributeCondition'] == condition(target) and provider['attributeMapping'] == MAPPING)
    # A principalSet trusts the whole pool, so the pool must hold only this provider.
    providers = gcloud(target, 'iam', 'workload-identity-pools', 'providers', 'list',
                       '--workload-identity-pool=' + POOL, '--location=global')
    need([item.get('name') for item in providers] == [provider['name']])
    email = account(target)
    if not gcloud(target, 'iam', 'service-accounts', 'describe', email, missing=True):
        gcloud(target, 'iam', 'service-accounts', 'create', email.split('@')[0],
               '--display-name=Terraform ' + target + ' PR Plan (read-only)')
    keys = gcloud(target, 'iam', 'service-accounts', 'keys', 'list', '--iam-account=' + email)
    need(all(key.get('keyType') != 'USER_MANAGED' for key in keys))
    principal = (f'principalSet://iam.googleapis.com/projects/{number}/locations/global/'
                 f'workloadIdentityPools/{POOL}/attribute.environment/{environment(target)}')
    gcloud(target, 'iam', 'service-accounts', 'add-iam-policy-binding', email,
           '--member=' + principal, '--role=roles/iam.workloadIdentityUser')
    gcloud(target, 'projects', 'add-iam-policy-binding', PROJECTS[target],
           '--member=serviceAccount:' + email, '--role=' + role, '--condition=None')
    policy = gcloud(target, 'projects', 'get-iam-policy', PROJECTS[target])
    need({b['role'] for b in policy['bindings'] if 'serviceAccount:' + email in b['members']} == {role})
    trust = gcloud(target, 'iam', 'service-accounts', 'get-iam-policy', email)
    need(trust.get('bindings') == [{'role': 'roles/iam.workloadIdentityUser', 'members': [principal]}])
    env_url = f'repos/{REPO}/environments/{environment(target)}'
    # The boundary is the WIF condition (owner, PR ref, exact workflows), not a
    # branch policy: PR refs are not branches.
    run(['gh', 'api', '-X', 'PUT', env_url, '--input', '-'], data=json.dumps({'deployment_branch_policy': None}))
    run(['gh', 'secret', 'set', 'GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT', '--repo', REPO,
         '--env', environment(target)], data=email)
    run(['gh', 'secret', 'set', 'GCP_TERRAFORM_NORMAL_WIF_PROVIDER', '--repo', REPO,
         '--env', environment(target)], data=provider['name'])
    print('PR_PLAN_BOOTSTRAP: PASS; target=' + target + '; GCP/Environment read back. '
          'Set the AWS role and managed inputs (with its own plan_integrity_key) separately.')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--target', choices=sorted(PROJECTS), required=True)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--aws-policies', nargs=2, metavar=('STATE_ACCOUNT_ID', 'STATE_BUCKET'))
    flags = parser.parse_args()
    if flags.aws_policies:
        state_account, bucket = flags.aws_policies
        print(json.dumps({'role_name': 'youtube-study-space-terraform-' + flags.target + '-pr-plan',
                          'trust_policy': aws_trust_policy(flags.target, state_account),
                          'permission_policy': aws_state_read_policy(flags.target, bucket)}, indent=2))
    elif flags.apply:
        apply(flags.target)
    else:
        print('Preview: pool=' + POOL + ' provider=' + PROVIDER + ' sa=' + account(flags.target) +
              ' environment=' + environment(flags.target) + '; no API calls.')
        print('Condition: ' + condition(flags.target))


if __name__ == '__main__':
    try:
        main()
    except Exception:
        print('PR_PLAN_BOOTSTRAP: STOP; nothing is retried automatically; inspect partial state.', file=sys.stderr)
        raise SystemExit(1)
