#!/usr/bin/env python3
"""認証用Terraformの一回限りの権限準備。既定は予定表示だけで、実行には--applyが必要。"""
import argparse
import json
import subprocess

TARGETS = {
    'dev': ('test-youtube-study-space', 'asia-southeast2'),
    'prod': ('youtube-study-space', 'asia-northeast2'),
}


def desired(target):
    project, region = TARGETS[target]
    runtime = f'monitor-auth-runtime@{project}.iam.gserviceaccount.com'
    builder = f'monitor-auth-build@{project}.iam.gserviceaccount.com'
    plan = f'terraform-{target}-normal-plan@{project}.iam.gserviceaccount.com'
    apply = f'terraform-{target}-normal-apply@{project}.iam.gserviceaccount.com'
    bucket = f'projects/_/buckets/{project}-monitor-auth-source'
    source = f"resource.name == '{bucket}' || resource.name.startsWith('{bucket}/objects/')"
    repository = f"resource.name == 'projects/{project}/locations/{region}/repositories/monitor-auth' || resource.name.startsWith('projects/{project}/locations/{region}/repositories/monitor-auth/')"
    invoker = f"resource.name.endsWith('/locations/{region}/services/monitor-password-signup-guard')"
    roles = {
        'monitorAuthConfigRead': ['firebaseauth.configs.get'],
        'monitorAuthConfigWrite': ['firebaseauth.configs.get', 'firebaseauth.configs.update'],
        'monitorAuthFunctionInvokerPolicy': ['run.services.getIamPolicy', 'run.services.setIamPolicy'],
    }
    grants = [(plan, f'projects/{project}/roles/monitorAuthConfigRead', None),
              (apply, f'projects/{project}/roles/monitorAuthConfigWrite', None),
              (apply, f'projects/{project}/roles/monitorAuthFunctionInvokerPolicy', invoker),
              (plan, 'roles/storage.objectViewer', source),
              (apply, 'roles/storage.objectViewer', source),
              (apply, 'roles/storage.objectCreator', source),
              (builder, 'roles/storage.objectViewer', source),
              (builder, 'roles/artifactregistry.writer', repository),
              (builder, 'roles/logging.logWriter', None)]
    return {'project': project, 'region': region, 'runtime': runtime, 'builder': builder,
            'plan': plan, 'apply': apply, 'roles': roles, 'grants': grants}


def command(args, missing=False):
    result = subprocess.run(['gcloud', *args, '--format=json', '--quiet'], capture_output=True, text=True, timeout=120)
    if result.returncode:
        if missing and ('NOT_FOUND' in result.stderr or 'does not exist' in result.stderr):
            return None
        raise RuntimeError('認証用権限の準備に失敗しました。管理端末で権限と対象環境を確認してください。')
    return json.loads(result.stdout) if result.stdout.strip() else {}


def provision(spec):
    project = spec['project']
    for email in (spec['plan'], spec['apply']):
        command(['iam', 'service-accounts', 'describe', email, '--project=' + project])
    for email in (spec['runtime'], spec['builder']):
        if command(['iam', 'service-accounts', 'describe', email, '--project=' + project], missing=True) is None:
            command(['iam', 'service-accounts', 'create', email.split('@')[0], '--project=' + project])
    policy = command(['projects', 'get-iam-policy', project])
    if any('serviceAccount:' + spec['runtime'] in binding.get('members', []) for binding in policy.get('bindings', [])):
        raise ValueError('実行用SAに既存のプロジェクト権限があります。専用・無権限の実行主体を確認してください。')
    for name, permissions in spec['roles'].items():
        existing = command(['iam', 'roles', 'describe', name, '--project=' + project], missing=True)
        if existing is not None:
            if sorted(existing.get('includedPermissions', [])) != sorted(permissions):
                raise ValueError('同名カスタムロールの内容が異なります。上書きせず確認してください。')
        else:
            command(['iam', 'roles', 'create', name, '--project=' + project,
                     '--title=' + name, '--stage=GA', '--permissions=' + ','.join(permissions)])
    for index, (email, role, expression) in enumerate(spec['grants']):
        condition = 'None' if expression is None else f'title=monitor-auth-{index},expression={expression}'
        command(['projects', 'add-iam-policy-binding', project, '--member=serviceAccount:' + email,
                 '--role=' + role, '--condition=' + condition])
    for email in (spec['runtime'], spec['builder']):
        command(['iam', 'service-accounts', 'add-iam-policy-binding', email, '--project=' + project,
                 '--member=serviceAccount:' + spec['apply'], '--role=roles/iam.serviceAccountUser'])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', required=True, choices=TARGETS)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()
    spec = desired(args.target)
    if not args.apply:
        print(json.dumps(spec, ensure_ascii=False, indent=2))
        return
    provision(spec)
    print('MONITOR_AUTH_BOOTSTRAP: PASS')


if __name__ == '__main__':
    main()
