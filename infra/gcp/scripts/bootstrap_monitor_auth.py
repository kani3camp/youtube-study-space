#!/usr/bin/env python3
"""認証用Terraformの一回限りの権限準備。既定は予定表示だけで、実行には--applyが必要。"""
import argparse
import json
import re
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
    # Cloud Functionsは元ZIPを管理バケットへ複製する。別関数のソースは許可しない。
    # 実行時に対象プロジェクトから取得した番号へ置換し、番号をコードへ固定しない。
    staged_source = ("resource.type == 'storage.googleapis.com/Object' && "
                     f"resource.name.startsWith('projects/_/buckets/gcf-v2-sources-PROJECT_NUMBER-{region}/objects/monitor-password-signup-guard/')")
    roles = {
        'monitorAuthConfigRead': ['firebaseauth.configs.get'],
        'monitorAuthConfigWrite': ['firebaseauth.configs.get', 'firebaseauth.configs.update'],
    }
    grants = [(plan, f'projects/{project}/roles/monitorAuthConfigRead', None),
              (apply, f'projects/{project}/roles/monitorAuthConfigWrite', None),
              (plan, 'roles/storage.objectViewer', source),
              (apply, 'roles/storage.objectViewer', source),
              (apply, 'roles/storage.objectCreator', source),
              (builder, 'roles/storage.objectViewer', source),
              (builder, 'roles/logging.logWriter', None),
              (builder, 'roles/storage.objectViewer', staged_source)]
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
    metadata = command(['projects', 'describe', project])
    project_number = str(metadata.get('projectNumber', ''))
    if metadata.get('projectId') != project or not re.fullmatch(r'[0-9]+', project_number):
        raise ValueError('対象プロジェクトの番号を確認できません。権限を変更せず停止します。')
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
    for email, role, expression in spec['grants']:
        if expression is not None:
            expression = expression.replace('PROJECT_NUMBER', project_number)
        title = 'monitor-auth-' + email.split('@')[0] + '-' + role.rsplit('/', 1)[-1].replace('.', '-')
        if expression is not None and 'gcf-v2-sources-' in expression:
            title += '-staged'
        condition = 'None' if expression is None else f'title={title},expression={expression}'
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
