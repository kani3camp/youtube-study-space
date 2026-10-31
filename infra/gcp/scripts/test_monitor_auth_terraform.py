#!/usr/bin/env python3
"""認証用モジュールと環境側の既定無効・取り込み境界をnative Terraformで検査する。"""
from pathlib import Path
import shutil
import subprocess
import tempfile
import sys

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
from package_monitor_auth import package


def run(directory, *args):
    result = subprocess.run(['terraform', f'-chdir={directory}', *args], capture_output=True, text=True)
    if result.returncode:
        # 合成入力のみ。認証付きplan/Stateはこのテストでは扱わない。
        raise RuntimeError(result.stdout + result.stderr)


def main():
    with tempfile.TemporaryDirectory(prefix='monitor-auth-tf-', dir='/tmp') as tmp:
        root = Path(tmp)
        module = root / 'modules/monitor-auth-registration'
        shutil.copytree(ROOT / 'infra/gcp/modules/monitor-auth-registration', module,
                        ignore=shutil.ignore_patterns('node_modules', '.terraform', '.terraform.lock.hcl', 'source.zip', '*debug.log'))
        shutil.copy(ROOT / 'infra/gcp/environments/dev/.terraform.lock.hcl', module / '.terraform.lock.hcl')
        package(root)
        run(module, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
        run(module, 'validate')
        run(module, 'test', '-no-color')
        # 無効な環境では、生成物がなくても通常のPlanが成立することを確認する。
        (module / 'function/source.zip').unlink()
        (module / 'tests/missing-source.tftest.hcl').write_text('''mock_provider "google" {}
variables {
 project_id = "demo-monitor-auth"
 region = "asia-southeast2"
 runtime_service_account_email = "monitor-auth-runtime@demo-monitor-auth.iam.gserviceaccount.com"
 build_service_account_email = "monitor-auth-build@demo-monitor-auth.iam.gserviceaccount.com"
}
run "enabled_requires_a_packaged_source" {
 command = plan
 expect_failures = [google_storage_bucket.source]
}
''')
        run(module, 'test', '-filter=tests/missing-source.tftest.hcl', '-no-color')
        for target, project in [('dev', 'test-youtube-study-space'), ('prod', 'youtube-study-space')]:
            directory = root / 'environments' / target
            directory.mkdir(parents=True)
            shutil.copy(ROOT / f'infra/gcp/environments/{target}/monitor-auth-registration.tf', directory)
            shutil.copy(ROOT / f'infra/gcp/environments/{target}/.terraform.lock.hcl', directory)
            (directory / 'main.tf').write_text('terraform {\n required_providers {\n google = { source = "hashicorp/google", version = "= 8.5.0" }\n }\n}\nprovider "google" {}\nvariable "project_id" { default = "' + project + '" }\n')
            (directory / 'tests').mkdir()
            (directory / 'tests/default.tftest.hcl').write_text('mock_provider "google" {}\nrun "default_does_not_manage_auth_or_functions" {\n command = plan\n assert {\n condition = length(module.monitor_auth_registration) == 0\n error_message = "既定値ではAuth設定と関数を変更してはいけません。"\n }\n}\n')
            run(directory, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
            run(directory, 'test', '-no-color')
    print('MONITOR_AUTH_TERRAFORM: PASS')


if __name__ == '__main__':
    main()
