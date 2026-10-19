#!/usr/bin/env python3
"""DEV-only operator bootstrap; uses gcloud login, creates no credential keys."""
import json
import subprocess
import sys

PROJECT = 'test-youtube-study-space'
REPO = 'kani3camp/youtube-study-space'
# Catalogs are read from GCP on every bootstrap; grant only the explicit
# metadata/control-plane intersection, never these broad predefined roles.
CATALOGS = ['roles/run.admin', 'roles/cloudfunctions.developer', 'roles/cloudscheduler.admin',
    'roles/pubsub.editor', 'roles/artifactregistry.admin', 'roles/storage.admin',
    'roles/bigquery.admin', 'roles/monitoring.editor', 'roles/serviceusage.serviceUsageAdmin',
    'roles/logging.configWriter', 'roles/secretmanager.admin', 'roles/iam.serviceAccountAdmin']
ROOTS = {
    'run.services', 'run.jobs', 'run.configurations', 'run.revisions', 'run.routes',
    'run.locations', 'run.operations', 'cloudfunctions.functions', 'cloudfunctions.locations',
    'cloudfunctions.operations', 'cloudscheduler.jobs', 'cloudscheduler.locations',
    'cloudscheduler.operations', 'pubsub.topics', 'pubsub.subscriptions', 'pubsub.schemas',
    'pubsub.snapshots', 'artifactregistry.repositories', 'artifactregistry.locations',
    'artifactregistry.operations', 'storage.buckets', 'bigquery.datasets', 'bigquery.tables',
    'bigquery.routines', 'monitoring.alertPolicies', 'monitoring.notificationChannels',
    'monitoring.dashboards', 'monitoring.uptimeCheckConfigs', 'monitoring.services',
    'monitoring.slos', 'serviceusage.services', 'serviceusage.operations', 'logging.sinks',
    'logging.logMetrics', 'logging.buckets', 'logging.views', 'logging.settings',
    'secretmanager.secrets', 'secretmanager.locations', 'secretmanager.versions',
    'iam.serviceAccounts', 'iam.workloadIdentityPools', 'iam.workloadIdentityPoolProviders',
}
READ = {'get', 'list', 'getIamPolicy', 'getAttestationRules'}
WRITE = {'create', 'update', 'delete', 'setIamPolicy', 'enable', 'disable', 'use'}
MAPPING = {
    'google.subject': "assertion.repository_id + ':' + assertion.repository_owner_id + ':' + assertion.environment",
    **{'attribute.' + k: 'assertion.' + k for k in ('environment', 'event_name', 'job_workflow_ref',
        'ref', 'repository_id', 'repository_owner_id', 'workflow_ref')},
}
CONDITION = " && ".join([
    "assertion.repository_id == '340900071'", "assertion.repository_owner_id == '54093651'",
    # Branch names are intentionally NOT allowlisted in GCP. GitHub Environment
    # Selected branches/tags is the sole per-branch authorization surface.
    "assertion.ref.startsWith('refs/heads/')", "assertion.ref != 'refs/heads/main'",
    "assertion.event_name == 'workflow_dispatch'",
    "assertion.workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/ci.yml@' + assertion.ref",
    "assertion.job_workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@' + assertion.ref",
    "assertion.environment in ['terraform-dev-plan', 'terraform-dev-apply']",
    "assertion.actor_id == '54093651'", "assertion.actor == 'kani3camp'", "assertion.run_attempt == '1'",
    "assertion.workflow_sha == assertion.sha", "assertion.job_workflow_sha == assertion.sha",
])



def need(value):
    if not value:
        raise ValueError("bootstrap verification rejected")


def selected_permissions(catalog, baseline):
    need(all(p.rsplit('.', 1)[-1] in READ for p in baseline))
    read, write = set(baseline), set(baseline)
    for permission in catalog:
        root, verb = permission.rsplit('.', 1)
        if root not in ROOTS:
            continue
        if verb in READ:
            read.add(permission)
            write.add(permission)
        if verb in WRITE:
            # Authentication bootstrap stays outside normal Terraform. A
            # shared unprivileged runtime SA supplies bounded actAs below.
            if root in {'iam.workloadIdentityPools', 'iam.workloadIdentityPoolProviders'}:
                continue
            if root == 'iam.serviceAccounts' and verb not in {'create', 'update', 'delete'}:
                continue
            if root == 'secretmanager.versions':
                continue
            write.add(permission)
    forbidden = {'iam.serviceAccounts.getAccessToken', 'iam.serviceAccountKeys.create',
        'resourcemanager.projects.setIamPolicy', 'secretmanager.versions.access',
        'secretmanager.versions.add', 'storage.objects.get', 'storage.objects.delete',
        'bigquery.tables.getData', 'bigquery.tables.updateData', 'bigquery.jobs.create'}
    need(not (write & forbidden) and read <= write)
    return {'plan': sorted(read), 'apply': sorted(write)}


def run(args, *, data=None, missing=False):
    result = subprocess.run(args, input=data, capture_output=True, text=True, timeout=120)
    if missing and result.returncode and ('NOT_FOUND' in result.stderr or 'does not exist' in result.stderr):
        return None
    if result.returncode:
        raise RuntimeError('bootstrap command failed')
    return json.loads(result.stdout) if result.stdout.strip() else {}


def gcloud(*args, missing=False):
    scope = [] if args[:3] == ('iam', 'roles', 'describe') and args[3].startswith('roles/') else ['--project=' + PROJECT]
    return run(['gcloud', *args, *scope, '--format=json', '--quiet'], missing=missing)


def bootstrap():
    project = gcloud('projects', 'describe', PROJECT)
    need(project['projectId'] == PROJECT and project['lifecycleState'] == 'ACTIVE')
    number = project['projectNumber']
    provider = gcloud('iam', 'workload-identity-pools', 'providers', 'describe', 'github-actions',
        '--workload-identity-pool=terraform-ci', '--location=global')
    condition = provider['attributeCondition']
    need(all(fragment in condition for fragment in ["assertion.repository_id == '340900071'",
        "assertion.actor_id == '54093651'", "assertion.event_name == 'workflow_dispatch'",
        'assertion.workflow_sha == assertion.sha', 'assertion.job_workflow_sha == assertion.sha']))
    baseline = []
    for role in ('terraformCiRead', 'terraformDevRuntimePoolReadV1'):
        baseline.extend(gcloud('iam', 'roles', 'describe', role)['includedPermissions'])
    catalog = set()
    for role in CATALOGS:
        catalog.update(gcloud('iam', 'roles', 'describe', role)['includedPermissions'])
    permissions = selected_permissions(catalog, baseline)
    if '--check' in sys.argv:
        print(json.dumps({'project': PROJECT, 'roles': permissions,
            'wif_ref': 'refs/heads/dev', 'runtime_sa': 'terraform-dev-workload',
            'github_environments': ['terraform-dev-plan', 'terraform-dev-apply']}, indent=2))
        return
    for kind, allowed in permissions.items():
        role = 'terraformNormal' + kind.title()
        existing = gcloud('iam', 'roles', 'describe', role, missing=True)
        gcloud('iam', 'roles', 'update' if existing else 'create', role,
            '--permissions=' + ','.join(allowed), '--stage=GA', '--title=Terraform DEV normal ' + kind)
        need(sorted(gcloud('iam', 'roles', 'describe', role)['includedPermissions']) == allowed)
    runtime = 'terraform-dev-workload@' + PROJECT + '.iam.gserviceaccount.com'
    if not gcloud('iam', 'service-accounts', 'describe', runtime, missing=True):
        gcloud('iam', 'service-accounts', 'create', 'terraform-dev-workload', '--display-name=Terraform DEV workload runtime')
    pool = 'terraform-normal'
    existing = gcloud('iam', 'workload-identity-pools', 'describe', pool, '--location=global', missing=True)
    if not existing:
        gcloud('iam', 'workload-identity-pools', 'create', pool, '--location=global', '--display-name=Terraform DEV normal')
    normal_condition = CONDITION
    mapping = ','.join(k + '=' + v for k, v in MAPPING.items())
    existing = gcloud('iam', 'workload-identity-pools', 'providers', 'describe', 'github-actions',
        '--workload-identity-pool=' + pool, '--location=global', missing=True)
    gcloud('iam', 'workload-identity-pools', 'providers', 'update-oidc' if existing else 'create-oidc',
        'github-actions', '--workload-identity-pool=' + pool, '--location=global',
        '--issuer-uri=https://token.actions.githubusercontent.com', '--attribute-mapping=' + mapping,
        '--attribute-condition=' + normal_condition)
    readback = gcloud('iam', 'workload-identity-pools', 'providers', 'describe', 'github-actions',
        '--workload-identity-pool=' + pool, '--location=global')
    need(readback['attributeCondition'] == normal_condition and readback['attributeMapping'] == MAPPING)
    runtime_policy = gcloud('projects', 'get-iam-policy', PROJECT)
    need(not any('serviceAccount:' + runtime in b['members'] for b in runtime_policy['bindings']))
    for kind in ('plan', 'apply'):
        roles = [f'projects/{PROJECT}/roles/terraformNormal' + kind.title()]
        name = 'terraform-dev-normal-' + kind
        email = name + '@' + PROJECT + '.iam.gserviceaccount.com'
        existing = gcloud('iam', 'service-accounts', 'describe', email, missing=True)
        if not existing:
            gcloud('iam', 'service-accounts', 'create', name, '--display-name=Terraform DEV normal ' + kind)
        need(gcloud('iam', 'service-accounts', 'describe', email)['email'] == email)
        keys = gcloud('iam', 'service-accounts', 'keys', 'list', '--iam-account=' + email)
        need(all(k.get('keyType') != 'USER_MANAGED' for k in keys))
        principal = f'principalSet://iam.googleapis.com/projects/{number}/locations/global/workloadIdentityPools/{pool}/attribute.environment/terraform-dev-{kind}'
        gcloud('iam', 'service-accounts', 'add-iam-policy-binding', email,
            '--member=' + principal, '--role=roles/iam.workloadIdentityUser')
        trust = gcloud('iam', 'service-accounts', 'get-iam-policy', email)
        need(any(b['role'] == 'roles/iam.workloadIdentityUser' and b['members'] == [principal] for b in trust['bindings']))
        for role_name in roles:
            gcloud('projects', 'add-iam-policy-binding', PROJECT, '--member=serviceAccount:' + email,
                   '--role=' + role_name, '--condition=None')
        if kind == 'apply':
            gcloud('iam', 'service-accounts', 'add-iam-policy-binding', runtime,
                '--member=serviceAccount:' + email, '--role=roles/iam.serviceAccountUser')
            policy = gcloud('iam', 'service-accounts', 'get-iam-policy', runtime)
            need(any(b['role'] == 'roles/iam.serviceAccountUser' and b['members'] == ['serviceAccount:' + email]
                       for b in policy['bindings']))
        policy = gcloud('projects', 'get-iam-policy', PROJECT)
        actual = {b['role'] for b in policy['bindings'] if 'serviceAccount:' + email in b['members'] and 'condition' not in b}
        need(actual == set(roles))
        environment = 'terraform-dev-' + kind
        run(['gh', 'secret', 'set', 'GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT', '--repo', REPO, '--env', environment], data=email)
        run(['gh', 'secret', 'set', 'GCP_TERRAFORM_NORMAL_WIF_PROVIDER', '--repo', REPO, '--env', environment], data=readback['name'])
        names = run(['gh', 'api', f'repos/{REPO}/environments/{environment}/secrets'])
        need({'GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT', 'GCP_TERRAFORM_NORMAL_WIF_PROVIDER'} <= {s['name'] for s in names['secrets']})
        print('BOOTSTRAP: PASS; DEV normal ' + kind + '; IAM/WIF readback and Environment secret names verified')
    print('BOOTSTRAP: PASS; no keys; production and migration identities unchanged')


if __name__ == '__main__':
    try:
        bootstrap()
    except Exception:
        print('BOOTSTRAP: STOP; operator authentication/permissions or readback failed', file=sys.stderr)
        raise SystemExit(1)
