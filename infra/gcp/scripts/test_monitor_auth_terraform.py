#!/usr/bin/env python3
"""メール認証の維持と環境ごとの明示有効化をnative Terraformで検証する。"""
from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[3]


def run(directory, *args):
    result = subprocess.run(['terraform', f'-chdir={directory}', *args], capture_output=True, text=True)
    if result.returncode:
        # 認証情報を使わない合成fixtureだけを扱う。
        raise RuntimeError(result.stdout + result.stderr)


def main():
    with tempfile.TemporaryDirectory(prefix='monitor-auth-tf-', dir='/tmp') as tmp:
        root = Path(tmp)
        module = root / 'modules/monitor-auth'
        shutil.copytree(ROOT / 'infra/gcp/modules/monitor-auth', module)
        shutil.copy(ROOT / 'infra/gcp/environments/dev/.terraform.lock.hcl', module / '.terraform.lock.hcl')
        run(module, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
        run(module, 'validate')
        run(module, 'test', '-no-color')
        for target, project in [('dev', 'test-youtube-study-space'), ('prod', 'youtube-study-space')]:
            directory = root / 'environments' / target
            directory.mkdir(parents=True)
            shutil.copy(ROOT / f'infra/gcp/environments/{target}/monitor-auth.tf', directory)
            shutil.copy(ROOT / f'infra/gcp/environments/{target}/.terraform.lock.hcl', directory)
            (directory / 'main.tf').write_text('terraform {\n required_providers {\n google = { source = "hashicorp/google", version = "= 8.5.0" }\n }\n}\nprovider "google" {}\nvariable "project_id" { default = "' + project + '" }\n')
            (directory / 'tests').mkdir()
            (directory / 'tests/default.tftest.hcl').write_text('''mock_provider "google" {}
run "default_does_not_manage_auth" {
 command = plan
 assert {
  condition = length(module.monitor_auth) == 0
  error_message = "既定値では既存Auth設定を管理しません。"
 }
}
run "explicit_enable_manages_only_existing_auth" {
 command = plan
 variables { monitor_email_password_enabled = true }
 override_resource {
  target = module.monitor_auth[0].google_identity_platform_config.auth
  values = {
   id = "projects/__PROJECT__/config"
   project = "__PROJECT__"
   sign_in = { email = { enabled = true, password_required = true } }
  }
 }
 assert {
  condition = length(module.monitor_auth) == 1
  error_message = "明示有効化した環境だけで既存Auth設定を管理します。"
 }
}
'''.replace('__PROJECT__', project))
            run(directory, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
            run(directory, 'test', '-no-color')
    print('MONITOR_AUTH_TERRAFORM: PASS')


if __name__ == '__main__':
    main()
