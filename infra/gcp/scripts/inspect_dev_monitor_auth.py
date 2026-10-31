"""DEVのAuth構成を、秘密値を取得・表示せずに確認する。変更操作は行わない。"""
import argparse
import json
import subprocess
import sys
import urllib.parse
import urllib.request

PROJECT = 'test-youtube-study-space'
FIELDS = 'subtype,signIn.email.enabled,signIn.email.passwordRequired,blockingFunctions.triggers'


def summarize(config):
    subtype = config.get('subtype')
    email = config.get('signIn', {}).get('email', {})
    triggers = config.get('blockingFunctions', {}).get('triggers', {})
    if (subtype not in ('IDENTITY_PLATFORM', 'FIREBASE_AUTH')
            or not isinstance(triggers, dict)
            or not set(triggers) <= {'beforeCreate', 'beforeSignIn'}
            or any(type(email.get(key, False)) is not bool for key in ('enabled', 'passwordRequired'))):
        raise ValueError('Auth構成の形式を確認できません')
    # サーバーが余分なフィールドを返しても、選んだ非秘密項目だけを出力する。
    return {'project': PROJECT, 'subtype': subtype,
            'email_enabled': email.get('enabled', False),
            'password_required': email.get('passwordRequired', False),
            'blocking_triggers': sorted(triggers)}


def inspect():
    credential = subprocess.run(['gcloud', 'auth', 'print-access-token'],
                                capture_output=True, text=True, check=True, timeout=30)
    token = credential.stdout.strip()
    if not token:
        raise ValueError('認証情報を確認できません')
    query = urllib.parse.urlencode({'fields': FIELDS})
    request = urllib.request.Request(
        f'https://identitytoolkit.googleapis.com/admin/v2/projects/{PROJECT}/config?{query}',
        headers={'Authorization': 'Bearer ' + token, 'X-Goog-User-Project': PROJECT})
    with urllib.request.urlopen(request, timeout=30) as response:
        config = json.loads(response.read(1_000_001))
    return summarize(config)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', choices=('dev',), required=True)
    parser.parse_args()
    try:
        print(json.dumps(inspect(), ensure_ascii=False))
    except Exception:
        # HTTPエラー本文・gcloudの標準エラー・例外には資格情報が含まれ得る。
        print('DEVのAuth確認に失敗しました。秘密保護のため応答・例外の内容は表示しません。', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
