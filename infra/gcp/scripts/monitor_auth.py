"""メール・パスワード認証の採用範囲だけを検証する。登録制限は管理しない。"""
ADDRESS = 'module.monitor_auth[0].google_identity_platform_config.auth'


def validate_import(item, project, enabled):
    importing = item.get('change', {}).get('importing')
    if not importing:
        return
    if not (enabled and item.get('address') == ADDRESS
            and item.get('type') == 'google_identity_platform_config'
            and item.get('mode') == 'managed'
            and item.get('provider_name') == 'registry.terraform.io/hashicorp/google'
            and importing == {'id': f'projects/{project}/config'}
            and item['change']['actions'] in (['no-op'], ['update'])):
        raise ValueError('Auth設定以外の取り込みは許可されていません')


def preflight(enabled, project, read):
    if not enabled:
        return
    current = read('identitytoolkit.googleapis.com', f'admin/v2/projects/{project}/config?fields=subtype')
    if current.get('subtype') not in ('IDENTITY_PLATFORM', 'FIREBASE_AUTH'):
        raise ValueError('既存Auth構成を確認できません')
