"""信頼済み実行器と、同一repoのTerraform対象ソースを分離する。"""
import base64
import json
import os
import re
import subprocess
import tempfile
import urllib.parse
import urllib.request

REPOSITORY = 'kani3camp/youtube-study-space'
CHECKOUT = '.terraform-source'


def need(value):
    if not value:
        raise ValueError('Terraform対象ソースの検証に失敗しました')


def branch(env):
    value = env.get('NORMAL_SOURCE_BRANCH', '')
    if not value:
        if env['NORMAL_TARGET'] == 'prod':
            return 'main'
        # SHA直接指定に、実行元のbranch名を誤って付けない。
        if env.get('NORMAL_SOURCE_SHA') or env.get('NORMAL_EXPECTED_SHA'):
            return ''
        return env['GITHUB_REF'].removeprefix('refs/heads/')
    need(env['NORMAL_TARGET'] == 'dev' and type(value) is str
         and re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._/-]*', value) is not None
         and not value.startswith('refs/'))
    result = subprocess.run(['git', 'check-ref-format', 'refs/heads/' + value], capture_output=True, timeout=10)
    need(result.returncode == 0)
    return value


def sha(env):
    value = env.get('GITHUB_SHA', '')
    if env['NORMAL_TARGET'] == 'dev':
        value = env.get('NORMAL_SOURCE_SHA') or env.get('NORMAL_EXPECTED_SHA') or value
    need(type(value) is str and re.fullmatch(r'[0-9a-f]{40}', value) is not None)
    return value


def identity(env):
    return {'repository': REPOSITORY, 'branch': branch(env), 'sha': sha(env)}


def validate_selector(env):
    branch(env)
    value = env.get('NORMAL_EXPECTED_SHA', '')
    need(type(value) is str and (not value or re.fullmatch(r'[0-9a-f]{40}', value) is not None))


def require_pinned(env):
    """資格情報を使うjobでは、preflightが確定したSHAの欠落を拒否する。"""
    value = env.get('NORMAL_SOURCE_SHA', '')
    need(type(value) is str and re.fullmatch(r'[0-9a-f]{40}', value) is not None)
    need(not env.get('NORMAL_EXPECTED_SHA') or env['NORMAL_EXPECTED_SHA'] == value)


def github(env, path):
    request = urllib.request.Request(f'https://api.github.com/repos/{REPOSITORY}/' + path,
        headers={'Authorization': 'Bearer ' + env['GITHUB_TOKEN'], 'Accept': 'application/vnd.github+json'})
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.loads(response.read(1_000_001))


def require_branch_commit(env, commit):
    """存在確認だけで許可せず、通常branchの履歴からの到達性を検証する。"""
    need(type(commit) is str and re.fullmatch(r'[0-9a-f]{40}', commit) is not None)
    # 元checkoutのshallow境界・PR refs・tags・古いremote refsを証明に使わない。
    git_env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
    authorization = base64.b64encode(('x-access-token:' + env['GITHUB_TOKEN']).encode()).decode()
    git_env.update({'GIT_CONFIG_GLOBAL': os.devnull, 'GIT_CONFIG_SYSTEM': os.devnull,
        'GIT_TERMINAL_PROMPT': '0', 'GIT_CONFIG_COUNT': '1',
        'GIT_CONFIG_KEY_0': 'http.https://github.com/.extraheader',
        'GIT_CONFIG_VALUE_0': 'AUTHORIZATION: basic ' + authorization})
    with tempfile.TemporaryDirectory(prefix='terraform-source-graph-') as directory:
        def git(*args):
            result = subprocess.run(['git', '-C', directory, *args], env=git_env,
                                    capture_output=True, timeout=120)
            need(result.returncode == 0)
            return result.stdout
        git('init', '--bare', '--template=')
        # blob本文は不要。depthを指定せず全branch履歴を取得する。PR refs/tagsは取得しない。
        git('fetch', '--no-tags', '--no-recurse-submodules', '--filter=blob:none',
            'https://github.com/' + REPOSITORY + '.git', '+refs/heads/*:refs/heads/*')
        need(git('cat-file', '-t', commit).strip() == b'commit')
        need(git('for-each-ref', '--contains=' + commit, '--format=%(refname)', 'refs/heads/').strip())


def resolve(env):
    """Environmentなしのpreflightで一度だけ解決し、後続jobへ固定値を渡す。"""
    if env['NORMAL_TARGET'] != 'dev':
        return None
    validate_selector(env)
    requested_branch = env.get('NORMAL_SOURCE_BRANCH', '')
    requested_sha = env.get('NORMAL_EXPECTED_SHA', '')
    if requested_branch:
        ref = urllib.parse.quote(branch(env), safe='')
        result = github(env, 'git/ref/heads/' + ref)
        need(result.get('ref') == 'refs/heads/' + requested_branch
             and result.get('object', {}).get('type') == 'commit')
        pinned = result['object'].get('sha')
        need(not requested_sha or requested_sha == pinned)
    elif requested_sha:
        # 実行元SHAは本人dispatchのbranch contextで既知。それ以外は通常branchへの到達性を要求する。
        if requested_sha != env['GITHUB_SHA']:
            require_branch_commit(env, requested_sha)
        pinned = requested_sha
    else:
        requested_branch = env['GITHUB_REF'].removeprefix('refs/heads/')
        pinned = env['GITHUB_SHA']
    need(type(pinned) is str and re.fullmatch(r'[0-9a-f]{40}', pinned) is not None)
    return {'branch': requested_branch, 'sha': pinned}


def publish(env, source):
    if env['NORMAL_TARGET'] != 'dev':
        return
    require_pinned(env | {'NORMAL_SOURCE_SHA': source['sha']})
    with open(env['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
        output.write('source_sha=' + source['sha'] + '\nsource_branch=' + source['branch'] + '\n')
    if env.get('GITHUB_STEP_SUMMARY'):
        with open(env['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as summary:
            summary.write('### Terraform対象ソース\n\n'
                          + '- リポジトリ: `' + REPOSITORY + '`\n'
                          + '- ブランチ: ' + ('`' + source['branch'] + '`' if source['branch'] else 'SHA直接指定') + '\n'
                          + '- SHA: `' + source['sha'] + '`\n')


def separate_checkout(env):
    return env['NORMAL_TARGET'] == 'dev' and bool(env.get('NORMAL_SOURCE_BRANCH') or env.get('NORMAL_SOURCE_SHA'))


def repository(env, control):
    # ディレクトリは入力で指定できない。actions/checkoutの固定pathと一致させる。
    return control / CHECKOUT if separate_checkout(env) else control


def read_file(env, control, relative):
    root = repository(env, control)
    path = root / relative
    need(path.resolve().is_relative_to(root.resolve()))
    for parent in (path, *path.parents):
        if parent == root:
            break
        need(not parent.is_symlink())
    if not path.exists():
        return None
    data = path.read_bytes()
    if separate_checkout(env):
        result = subprocess.run(['git', 'show', sha(env) + ':' + relative], cwd=root,
                                capture_output=True, timeout=20)
        need(result.returncode == 0 and data == result.stdout)
    return data


def verify_checkout(env, control, run):
    root = repository(env, control)
    need(root.is_dir() and not root.is_symlink())
    need(run(['git', 'rev-parse', 'HEAD'], cwd=root).stdout.decode().strip() == sha(env))
    need(not run(['git', 'status', '--porcelain', '--untracked-files=no'], cwd=root).stdout)
    if separate_checkout(env):
        need(run(['git', 'remote', 'get-url', 'origin'], cwd=root).stdout.decode().strip()
             in ('https://github.com/' + REPOSITORY, 'https://github.com/' + REPOSITORY + '.git'))
    return root


def data_paths(env, root, run):
    paths = run(['git', 'ls-files', 'infra/gcp'], cwd=root).stdout.decode().splitlines()
    if env['NORMAL_TARGET'] == 'prod':
        return paths  # 本番の従来のsource/実行方式は変更しない。
    prefixes = ('infra/gcp/environments/dev/', 'infra/gcp/modules/')
    return [name for name in paths if name.startswith(prefixes)]
