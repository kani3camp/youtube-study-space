"""既存Auth設定の採用と、beforeCreateの管理範囲を検証する。"""
import re

ADDRESS = 'module.monitor_auth_registration[0].google_identity_platform_config.auth'


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


def preflight(settings, project, region, read):
    if settings is None:
        return
    for field in ('runtime_service_account_email', 'build_service_account_email'):
        if not re.fullmatch(r'[a-z][a-z0-9-]{4,28}[a-z0-9]@' + re.escape(project) + r'\.iam\.gserviceaccount\.com', settings[field]):
            raise ValueError('認証用の実行主体が環境に一致しません')
    if settings['runtime_service_account_email'] == settings['build_service_account_email']:
        raise ValueError('実行用SAとビルド用SAを分離してください')
    current = read('identitytoolkit.googleapis.com', f'admin/v2/projects/{project}/config')
    if current.get('subtype') != 'IDENTITY_PLATFORM':
        raise ValueError('既存プロジェクトのIdentity Platform有効化を先に確認してください')
    triggers = current.get('blockingFunctions', {}).get('triggers', {})
    if not set(triggers) <= {'beforeCreate', 'beforeSignIn'}:
        raise ValueError('未対応の既存Authトリガーがあります')
    if triggers.get('beforeSignIn', {}).get('functionUri', '') != settings.get('preserved_before_sign_in_uri', ''):
        raise ValueError('既存beforeSignInの維持設定が一致しません')
    if 'beforeCreate' in triggers:
        function = read('cloudfunctions.googleapis.com',
                        f'v2/projects/{project}/locations/{region}/functions/monitor-password-signup-guard')
        uri = function.get('serviceConfig', {}).get('uri')
        if not uri or uri != triggers['beforeCreate'].get('functionUri'):
            raise ValueError('既存beforeCreateを上書きできません')
