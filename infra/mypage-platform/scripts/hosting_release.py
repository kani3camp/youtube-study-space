"""MyPageのWebページ(静的ファイル)をDEVのFirebase Hostingへ公開し、公開後に読み直して確かめる。

目的: 想定外のファイル(非公開ファイルやAPIを隠すもの)が公開される、設定が黙って変わる、公開が競合する、といった事故を防ぐ。
使われる場面: MyPage DEV Hosting Release ワークフロー(.github/workflows/mypage-hosting-authenticated.yml)の context・build・deploy の各ステップ。CI Frontend の mypage-check でも、ビルド結果の一覧検査(inventory)だけが読み込まれる。
失敗したとき: Hosting配備が中断される。自動の再試行・巻き戻しはしない。CI Frontend では inventory の失敗でPRをマージできない。
なくした場合: DEV Hosting配備ワークフローが動かなくなり、CI Frontend のビルド成果物の検査も失われる。

補足: 非特権ビルド、制限付きREST公開、再読み取りを行う。Firebase CLI探索・SA鍵・Terraform/IAM書き込み・バックエンド更新・ランタイムの秘密値・変更の自動再試行・自動ロールバックは扱わない。
"""
from __future__ import annotations

import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request
from urllib.parse import urljoin

ROOT = Path(__file__).resolve().parents[3]
PROJECT = 'test-youtube-study-space'
SITE = 'test--study-space'
ORIGIN = 'https://' + SITE + '.web.app'
REPOSITORY = 'kani3camp/youtube-study-space'
REF = 'refs/heads/integration/mypage-canon-20261006'
WORKFLOWS = REPOSITORY + '/.github/workflows/'
CALLER = WORKFLOWS + 'deploy-mypage-frontend.yml@' + REF
# Deploy workflows whose release scripts share this owner/ref/run admission.
CALLERS = frozenset({CALLER, WORKFLOWS + 'deploy-mypage-api.yml@' + REF})
API = 'https://firebasehosting.googleapis.com/v1beta1/'
RUN = 'https://run.googleapis.com/v2/projects/' + PROJECT + '/locations/asia-southeast2/services/mypage-api'
MAX_FILE = 4 * 1024 * 1024
MAX_TOTAL = 20 * 1024 * 1024
IMMUTABLE = 'public, max-age=31536000, immutable'
POLICY_PAGES = {'/privacy': 'privacy.html', '/terms': 'terms.html'}
POLICY_REWRITES = [{'glob': route + '{,/}', 'path': '/' + filename}
                   for route, filename in POLICY_PAGES.items()]
POLICY_REDIRECTS = [{'glob': '/privacy_policy{,/}', 'statusCode': 302, 'location': '/privacy'}]
DEV_GA4_ID = 'G-E31NHW939V'
GA4_TAG = 'https://www.googletagmanager.com/gtag/js'
GA4_COLLECTION = frozenset({
    'https://www.google-analytics.com', 'https://region1.google-analytics.com',
})
CONSENT_HEADER = 'X-MyPage-Consent-Versions'
POLICY_SOURCE = ROOT / 'mypage/src/public-policy-config.ts'
GA4_SOURCES = {
    'script-src': frozenset({GA4_TAG}),
    'connect-src': GA4_COLLECTION,
    'img-src': GA4_COLLECTION,
}


def need(value, label='contract'):
    if not value:
        raise ValueError(label)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()


def sha(data):
    return hashlib.sha256(data).hexdigest()


def context(env, *, caller=CALLER):
    """Admit only the owner's first manual run of the use-case Deploy workflow."""
    need(caller in CALLERS, 'execution-caller')
    expected = {
        'GITHUB_REPOSITORY': REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
        'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_ACTOR_ID': '54093651',
        'GITHUB_ACTOR': 'kani3camp', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
        'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_RUN_ATTEMPT': '1',
        'GITHUB_REF': REF, 'GITHUB_WORKFLOW_REF': caller,
    }
    need(all(env.get(k) == v for k, v in expected.items()), 'execution-context')
    need(re.fullmatch('[a-f0-9]{40}', env.get('GITHUB_SHA', '')), 'source-sha')
    need(env.get('MYPAGE_HOSTING_EXPECTED_SHA') == env['GITHUB_SHA'], 'expected-sha')
    need(re.fullmatch('[0-9]+', env.get('GITHUB_RUN_ID', '')), 'run-id')


def directory(env):
    base = Path(env['RUNNER_TEMP']).resolve()
    need(not base.is_relative_to(ROOT), 'private-workspace')
    return base / 'mypage-hosting-release'


def command(args, *, env=None, cwd=ROOT):
    result = subprocess.run(args, env=env, cwd=cwd, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=600)
    need(result.returncode == 0, 'local-command')
    return result.stdout


def allowed_path(name):
    return name in {'index.html', 'favicon.png', 'fonts/mplus-rounded-1c/OFL.txt', *POLICY_PAGES.values()} or bool(
        re.fullmatch(r'assets/[A-Za-z0-9][A-Za-z0-9._-]*-[A-Za-z0-9_-]{8,}\.(js|css|woff2)', name))


def inventory(dist):
    need(dist.is_dir() and not dist.is_symlink(), 'dist')
    files = {}
    for p in sorted(dist.rglob('*')):
        need(not p.is_symlink(), 'symlink')
        if p.is_dir():
            continue
        name = p.relative_to(dist).as_posix()
        need(p.is_file() and allowed_path(name), 'asset-path')
        need(0 < p.stat().st_size <= MAX_FILE, 'asset-size')
        data = p.read_bytes()
        files[name] = {'bytes': len(data), 'sha256': sha(data)}
    need(len(files) <= 64 and {'index.html', *POLICY_PAGES.values()} <= files.keys(), 'asset-count')
    need(sum(f['bytes'] for f in files.values()) <= MAX_TOTAL, 'release-size')
    need(sum(n.endswith('.js') for n in files) == 1 and
         sum(n.endswith('.css') for n in files) == 1, 'entry-assets')
    return files


def translate(package):
    files = inventory(package / 'dist')
    (package / 'paths.json').write_bytes(canonical(sorted(files)))
    command(['node', 'mypage/scripts/hosting-release-config.mjs',
             str(package / 'public-config.json'), str(package / 'paths.json'),
             str(package / 'hosting.json')])
    return json.loads((package / 'hosting.json').read_bytes())


def frontend_consent_versions():
    # 配備する画面と同じソースの定数から、APIに送る同意版を読む。
    match = re.search(r"^export const POLICY_VERSIONS = \{ privacy: '([A-Za-z0-9._-]+)', terms: '([A-Za-z0-9._-]+)' \} as const$",
                      POLICY_SOURCE.read_text(encoding='utf-8'), re.M)
    need(match, 'frontend-consent-versions')
    return 'privacy=' + match[1] + ', terms=' + match[2]


def served_consent_versions():
    # 稼働中のAPIが匿名リクエストにも返す、実際に照合する同意版。
    _, headers, _ = http(ORIGIN + '/api/mypage')
    return {k.lower(): v for k, v in headers.items()}.get(CONSENT_HEADER.lower())


def drop_retired_policy_settings(config):
    # 同意版と公開文面の値はソースの定数になった。既存のGitHub Environment変数に残る
    # 旧項目だけを移行期間として取り除き、それ以外の未知項目は標準の検証処理で拒否する。
    config.pop('publicPolicy', None)
    for section in ('deployment', 'backend'):
        for key in ('privacyVersion', 'termsVersion'):
            config.get(section, {}).pop(key, None)
    for key in ('VITE_PRIVACY_POLICY_VERSION', 'VITE_TERMS_VERSION'):
        config.get('frontend', {}).pop(key, None)


def configure_dev_analytics(config):
    frontend = config['frontend']
    # Accept the existing GitHub Environment manifest during migration.
    frontend.pop('VITE_MYPAGE_ANALYTICS_READY', None)
    frontend.update(VITE_MYPAGE_GA4_ID=DEV_GA4_ID, VITE_MYPAGE_PUBLIC_ORIGIN=ORIGIN)
    inventory = config['hosting']['inventory']
    inventory['analyticsApproved'] = True
    for key, directive in (('script', 'script-src'), ('connect', 'connect-src'), ('image', 'img-src')):
        sources = GA4_SOURCES[directive]
        inventory[key] = list(dict.fromkeys(inventory[key] + sorted(sources)))


def build(env):
    context(env)
    package = directory(env)
    package.mkdir(mode=0o700)  # Never replace a previous attempt's artifact.
    raw = env['MYPAGE_HOSTING_PUBLIC_CONFIG_JSON'].encode()
    need(len(raw) <= 65536, 'public-config-size')
    config = json.loads(raw)
    drop_retired_policy_settings(config)
    configure_dev_analytics(config)
    # 値をビルド環境変数に使う前に、標準の検証処理を実行する。
    (package / 'public-config.json').write_bytes(canonical(config))
    # 候補への変換時に、DEVの識別情報とアクセス解析の設定も検証する。
    (package / 'paths.json').write_bytes(canonical([]))
    command(['node', 'mypage/scripts/hosting-release-config.mjs',
             str(package / 'public-config.json'), str(package / 'paths.json'),
             str(package / 'hosting.json')])
    check_ga4_csp(json.loads((package / 'hosting.json').read_bytes()))
    build_env = {k: v for k, v in env.items() if not k.startswith('VITE_') and
                 k not in {'MYPAGE_HOSTING_PUBLIC_CONFIG_JSON', 'GITHUB_TOKEN',
                           'GH_TOKEN', 'GOOGLE_APPLICATION_CREDENTIALS'}}
    build_env.update(config['frontend'])
    need(not list((ROOT / 'mypage').glob('.env*')), 'ambient-vite-config')
    command(['pnpm', 'build'], cwd=ROOT / 'mypage', env=build_env)
    shutil.copytree(ROOT / 'mypage/dist', package / 'dist', dirs_exist_ok=True)
    hosting = translate(package)
    files = inventory(package / 'dist')
    release = {'schemaVersion': 1, 'sourceSHA': env['GITHUB_SHA'],
               'runID': env['GITHUB_RUN_ID'], 'createdAt': int(time.time()),
               'publicConfigSHA256': sha(canonical(config)),
               'hostingSHA256': sha(canonical(hosting)), 'files': files}
    (package / 'release.json').write_bytes(canonical(release))
    digest = sha(canonical(release))
    with open(env['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
        output.write('digest=' + digest + '\n')
    print('DEV Hosting package prepared; no cloud credentials or deployment.')


def validate_package(package, env):
    need(not package.is_symlink(), 'package-symlink')
    need({p.name for p in package.iterdir()} ==
         {'dist', 'release.json', 'hosting.json', 'paths.json', 'public-config.json'}, 'package-files')
    for p in package.iterdir():
        need(not p.is_symlink(), 'package-symlink')
        if p.is_file():
            need(p.stat().st_size <= 65536, 'metadata-size')
    release = json.loads((package / 'release.json').read_bytes())
    need(sha(canonical(release)) == env['MYPAGE_HOSTING_PACKAGE_DIGEST'], 'package-digest')
    need(set(release) == {'schemaVersion', 'sourceSHA', 'runID', 'createdAt',
                         'publicConfigSHA256', 'hostingSHA256', 'files'}, 'package-schema')
    need(release['schemaVersion'] == 1 and release['sourceSHA'] == env['GITHUB_SHA'] and
         release['runID'] == env['GITHUB_RUN_ID'], 'package-origin')
    need(type(release['createdAt']) is int and 0 <= time.time() - release['createdAt'] <= 1800, 'package-age')
    need(inventory(package / 'dist') == release['files'], 'package-content')
    config = json.loads((package / 'public-config.json').read_bytes())
    need(sha(canonical(config)) == release['publicConfigSHA256'], 'package-config')
    previous = (package / 'hosting.json').read_bytes()
    hosting = translate(package)
    need(canonical(json.loads(previous)) == canonical(hosting) and
         sha(canonical(hosting)) == release['hostingSHA256'], 'package-hosting')
    return release, config, hosting


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        # 匿名の応答検査で転送先を確認できるよう、追従せずに応答を返す。
        return None


def http(url, *, token=None, method='GET', body=None, binary=False, github=False):
    if token:
        need(url.startswith(API) or url == RUN or
             url.startswith('https://upload-firebasehosting.googleapis.com/upload/sites/' + SITE + '/versions/') or
             (github and url == 'https://api.github.com/repos/' + REPOSITORY + '/git/ref/heads/' + REF.removeprefix('refs/heads/')),
             'credential-destination')
    else:
        need(url.startswith(ORIGIN + '/'), 'public-destination')
    headers = {'Accept-Encoding': 'identity', 'User-Agent': 'mypage-hosting-ci'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
        if not github:
            headers['x-goog-user-project'] = PROJECT
    if body is not None:
        headers['Content-Type'] = 'application/octet-stream' if binary else 'application/json'
        body = body if binary else canonical(body)
    request = urllib.request.Request(url, data=body, method=method, headers=headers)
    try:
        with urllib.request.build_opener(NoRedirect()).open(request, timeout=45) as response:
            data = response.read(MAX_FILE + 1)
            need(len(data) <= MAX_FILE, 'http-size')
            return response.status, dict(response.headers), data
    except urllib.error.HTTPError as error:
        # プロバイダーの応答・認証ヘッダー・URL・ユーザーデータは出力しない。
        if not token and method == 'GET':
            data = error.read(MAX_FILE + 1)
            need(len(data) <= MAX_FILE, 'http-size')
            return error.code, dict(error.headers), data
        raise ValueError('cloud-http-' + str(error.code)) from None


def cloud(url, token, *, method='GET', body=None):
    status, _, raw = http(url, token=token, method=method, body=body)
    need(200 <= status < 300, 'cloud-status')
    return json.loads(raw) if raw else {}


def current(token):
    listing = cloud(API + 'sites/' + SITE + '/releases?pageSize=1', token)
    releases = listing.get('releases', [])
    need(len(releases) == 1 and releases[0].get('type') == 'DEPLOY', 'current-release')
    item = releases[0]
    need(re.fullmatch('sites/' + SITE + '/releases/[A-Za-z0-9_-]+', item.get('name', '')), 'release-name')
    name = item.get('version', {}).get('name', '')
    need(re.fullmatch('sites/' + SITE + '/versions/[A-Za-z0-9_-]+', name), 'version-name')
    return item


def backend(token):
    value = cloud(RUN, token)
    # 前後比較のため、サイズ制限付きの応答全体を保持する。公開はしない。
    ready = value.get('latestReadyRevision', '')
    need(isinstance(ready, str) and ready and ready == value.get('latestCreatedRevision') and
         value.get('reconciling', False) is False and
         value.get('terminalCondition', {}).get('state') == 'CONDITION_SUCCEEDED' and
         len(value.get('trafficStatuses', [])) == 1 and
         value['trafficStatuses'][0].get('percent') == 100 and
         value['trafficStatuses'][0].get('revision') == ready.rsplit('/', 1)[-1], 'backend-ready')
    need(value.get('uri') == 'https://mypage-api-eu5mhkcewq-et.a.run.app', 'backend-uri')
    return value


def verify_head(env):
    url = 'https://api.github.com/repos/' + REPOSITORY + '/git/ref/heads/' + REF.removeprefix('refs/heads/')
    _, _, data = http(url, token=env['GITHUB_TOKEN'], github=True)
    need(json.loads(data)['object']['sha'] == env['GITHUB_SHA'], 'branch-moved')


def stable_config(config):
    # 不変アセットのキャッシュ規則と、DEV GA4の固定送信先だけを比較から除く。
    need(set(config) in ({'headers', 'rewrites'}, {'headers', 'rewrites', 'redirects'}),
         'hosting-config-scope')
    headers = config['headers']
    need(isinstance(headers, list) and sum(x.get('glob') == '**' for x in headers) == 1, 'hosting-headers')
    for h in headers:
        need(set(h) == {'glob', 'headers'}, 'hosting-header-schema')
        if h['glob'] != '**':
            need(h['glob'].startswith('/assets/') and allowed_path(h['glob'][1:]) and
                 h['headers'] == {'Cache-Control': IMMUTABLE}, 'hosting-cache-rule')
    global_headers = next(h['headers'] for h in headers if h['glob'] == '**').copy()
    csp = global_headers['Content-Security-Policy']
    directives = []
    for item in csp.split('; '):
        directive, *sources = item.split(' ')
        allowed = GA4_SOURCES.get(directive, ())
        directives.append(' '.join([directive, *(source for source in sources if source not in allowed)]))
    global_headers['Content-Security-Policy'] = '; '.join(directives)
    return {'headers': global_headers, 'rewrites': config['rewrites'],
            'redirects': config.get('redirects', [])}


def hosting_settings_match(current_config, candidate_config):
    current_settings = stable_config(current_config)
    candidate_settings = stable_config(candidate_config)
    rewrites = candidate_settings['rewrites']
    # 初回の公開時だけ、規約ルール追加前の設定も許可する。
    need(candidate_settings['redirects'] == POLICY_REDIRECTS and
         rewrites[-3:] == [*POLICY_REWRITES, {'glob': '**', 'path': '/index.html'}],
         'hosting-policy-routes')
    legacy = candidate_settings | {'redirects': [], 'rewrites': rewrites[:-3] + rewrites[-1:]}
    return current_settings in (candidate_settings, legacy)


def check_ga4_csp(hosting):
    csp = next(h['headers']['Content-Security-Policy'] for h in hosting['headers'] if h['glob'] == '**')
    directives = dict((parts[0], set(parts[1:])) for item in csp.split('; ') if (parts := item.split()))
    for directive, sources in GA4_SOURCES.items():
        need(sources <= directives.get(directive, set()), 'hosting-ga4-csp')


def deploy(env, *, request=cloud, transport=http):
    context(env)
    package = directory(env)
    release, config, hosting = validate_package(package, env)
    check_ga4_csp(hosting)
    verify_head(env)
    token = env['MYPAGE_HOSTING_ACCESS_TOKEN']
    site = request(API + 'projects/' + PROJECT + '/sites/' + SITE, token)
    need(site.get('defaultUrl') == ORIGIN and site.get('name', '').split('/')[1] in
         {PROJECT, config['deployment']['projectNumber']}, 'site-project')
    before = current(token)
    version = request(API + before['version']['name'], token)
    need(version.get('status') == 'FINALIZED' and
         hosting_settings_match(version['config'], hosting), 'hosting-settings-changed')
    api_before = backend(token)
    public_env = {x['name']: x.get('value') for x in api_before['template']['containers'][0]['env']
                  if x.get('name') == 'MYPAGE_PUBLIC_ORIGIN'}
    need(public_env == {'MYPAGE_PUBLIC_ORIGIN': ORIGIN}, 'backend-public-config')
    # 画面と稼働中APIの同意版が異なるとログインを開始できないため、一致しない組み合わせは公開しない。
    need(served_consent_versions() == frontend_consent_versions(), 'backend-consent-versions')
    created = request(API + 'sites/' + SITE + '/versions', token, method='POST', body={
        'config': hosting, 'labels': {'deployment-tool': 'github-actions', 'source-sha': env['GITHUB_SHA'],
                                     'workflow-run': env['GITHUB_RUN_ID']}})
    name = created.get('name', '')
    need(re.fullmatch('sites/' + SITE + '/versions/[A-Za-z0-9_-]+', name), 'created-version')
    compressed = {}
    upload_files = {}
    for path in release['files']:
        data = gzip.compress((package / 'dist' / path).read_bytes(), mtime=0)
        digest = sha(data)
        compressed[digest] = data
        upload_files['/' + path] = digest
    populated = request(API + name + ':populateFiles', token, method='POST', body={'files': upload_files})
    upload_url = 'https://upload-firebasehosting.googleapis.com/upload/' + name + '/files'
    need(populated.get('uploadUrl') == upload_url, 'upload-destination')
    required = populated.get('uploadRequiredHashes', [])
    need(isinstance(required, list) and len(required) == len(set(required)) and
         all(h in compressed for h in required), 'upload-hashes')
    for digest in required:
        status, _, _ = transport(upload_url + '/' + digest, token=token, method='POST',
                                 body=compressed[digest], binary=True)
        need(200 <= status < 300, 'upload-status')
    finalized = request(API + name + '?updateMask=status', token, method='PATCH', body={'status': 'FINALIZED'})
    need(finalized.get('name') == name and finalized.get('status') == 'FINALIZED', 'finalize-readback')
    need(current(token)['name'] == before['name'], 'hosting-moved')
    need(backend(token) == api_before, 'backend-moved')
    verify_head(env)
    record = {'schemaVersion': 1, 'environment': 'development', 'sourceSHA': env['GITHUB_SHA'],
              'workflowRun': 'https://github.com/' + REPOSITORY + '/actions/runs/' + env['GITHUB_RUN_ID'],
              'packageSHA256': env['MYPAGE_HOSTING_PACKAGE_DIGEST'], 'hostingRelease': None,
              'hostingVersion': name, 'previousRelease': before['name'], 'previousVersion': before['version']['name'],
              'backendRevision': api_before['latestReadyRevision'].rsplit('/', 1)[-1],
              'analyticsEnabled': True, 'realGA4Verified': False,
              'publishStatus': 'unknown', 'publicReadback': 'pending'}
    record_path = Path(env['RUNNER_TEMP']) / 'mypage-hosting-release-record.json'
    # 実際の切替前に復旧用参照を永続化する。応答消失時も公開が成功した可能性がある。
    # unknownを維持し、この書き込みは再試行しない。
    record_path.write_bytes(canonical(record))
    published = request(API + 'sites/' + SITE + '/releases?versionName=' + name, token,
                        method='POST', body={'message': 'CI DEV MyPage ' + env['GITHUB_SHA']})
    need(published.get('version', {}).get('name') == name and published.get('type') == 'DEPLOY', 'release-response')
    record.update(hostingRelease=published.get('name'), publishStatus='confirmed')
    record_path.write_bytes(canonical(record))
    need(current(token)['version']['name'] == name and backend(token) == api_before, 'post-release-state')
    verify_public(release, hosting)
    record['publicReadback'] = 'passed'
    record_path.write_bytes(canonical(record))
    print('DEV Hosting release verified: ' + name + '; backend unchanged.')


def verify_public(release, hosting):
    expected = next(h['headers'] for h in hosting['headers'] if h['glob'] == '**')
    for name, info in release['files'].items():
        status, headers, data = http(ORIGIN + '/' + name)
        need(status == 200 and sha(data) == info['sha256'], 'public-file-body')
        headers = {k.lower(): v for k, v in headers.items()}
        need(headers.get('cache-control') == (IMMUTABLE if name.startswith('assets/') else 'no-store'), 'public-cache')
        for key in ('Content-Security-Policy', 'X-Content-Type-Options', 'Referrer-Policy'):
            need(headers.get(key.lower()) == expected[key], 'public-security-header')
    for route in ('/', '/mypage'):
        status, headers, data = http(ORIGIN + route)
        need(status == 200 and sha(data) == release['files']['index.html']['sha256'], 'public-route')
    for route, filename in POLICY_PAGES.items():
        status, headers, data = http(ORIGIN + route)
        headers = {k.lower(): v for k, v in headers.items()}
        need(status == 200 and sha(data) == release['files'][filename]['sha256'] and
             headers.get('content-type', '').split(';', 1)[0] == 'text/html' and
             headers.get('cache-control') == 'no-store', 'public-policy-html')
    status, headers, _ = http(ORIGIN + '/privacy_policy')
    location = {k.lower(): v for k, v in headers.items()}.get('location', '')
    need(status == 302 and urljoin(ORIGIN, location) == ORIGIN + '/privacy', 'public-policy-redirect')
    status, headers, data = http(ORIGIN + '/api/mypage')
    cache = {k.lower(): v for k, v in headers.items()}.get('cache-control', '')
    need(status == 403 and json.loads(data).get('error', {}).get('code') == 'APP_CHECK_REQUIRED' and
         {part.strip().lower() for part in cache.split(',')} == {'no-store', 'private'}, 'anonymous-api')
    consent = {k.lower(): v for k, v in headers.items()}.get(CONSENT_HEADER.lower())
    need(consent == frontend_consent_versions(), 'backend-consent-versions')


def main():
    try:
        need(len(sys.argv) == 2 and sys.argv[1] in {'context', 'build', 'deploy'}, 'arguments')
        {'context': context, 'build': build, 'deploy': deploy}[sys.argv[1]](dict(os.environ))
    except Exception as error:
        label = str(error) if type(error) is ValueError and re.fullmatch('[a-z0-9-]+', str(error)) else 'execution'
        print('::error::DEV Hosting stopped: ' + label + '. No automatic retry or rollback.', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
