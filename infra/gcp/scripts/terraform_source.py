"""信頼済み実行器と、同一repoのTerraform対象ソースを分離する。"""
import json
import re
import subprocess
import urllib.parse
import urllib.request
import zipfile

REPOSITORY = 'kani3camp/youtube-study-space'
CHECKOUT = '.terraform-source'


def need(value):
    if not value:
        raise ValueError('Terraform対象ソースの検証に失敗しました')


def branch(env):
    value = env.get('NORMAL_SOURCE_BRANCH', '')
    if not value:
        return env['GITHUB_REF'].removeprefix('refs/heads/') if env['NORMAL_TARGET'] == 'dev' else 'main'
    need(env['NORMAL_TARGET'] == 'dev' and type(value) is str
         and re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._/-]*', value) is not None
         and not value.startswith('refs/'))
    result = subprocess.run(['git', 'check-ref-format', 'refs/heads/' + value], capture_output=True, timeout=10)
    need(result.returncode == 0)
    return value


def sha(env):
    value = env.get('NORMAL_EXPECTED_SHA', '') if env.get('NORMAL_SOURCE_BRANCH') else env.get('GITHUB_SHA', '')
    need(type(value) is str and re.fullmatch(r'[0-9a-f]{40}', value) is not None)
    return value


def identity(env):
    return {'repository': REPOSITORY, 'branch': branch(env), 'sha': sha(env)}


def resolve(env):
    """Environmentなしのpreflightで、同一repoのbranch HEADと指定SHAを照合する。"""
    if not env.get('NORMAL_SOURCE_BRANCH'):
        return
    source = identity(env)
    ref = urllib.parse.quote(source['branch'], safe='')
    request = urllib.request.Request(f'https://api.github.com/repos/{REPOSITORY}/git/ref/heads/{ref}',
        headers={'Authorization': 'Bearer ' + env['GITHUB_TOKEN'], 'Accept': 'application/vnd.github+json'})
    with urllib.request.urlopen(request, timeout=30) as response:
        result = json.loads(response.read(1_000_001))
    need(result.get('ref') == 'refs/heads/' + source['branch']
         and result.get('object', {}).get('type') == 'commit'
         and result['object'].get('sha') == source['sha'])


def repository(env, control):
    # ディレクトリは入力で指定できない。actions/checkoutの固定pathと一致させる。
    return control / CHECKOUT if env.get('NORMAL_SOURCE_BRANCH') else control


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
    if env.get('NORMAL_SOURCE_BRANCH'):
        result = subprocess.run(['git', 'show', sha(env) + ':' + relative], cwd=root,
                                capture_output=True, timeout=20)
        need(result.returncode == 0 and data == result.stdout)
    return data


def verify_checkout(env, control, run):
    root = repository(env, control)
    need(root.is_dir() and not root.is_symlink())
    need(run(['git', 'rev-parse', 'HEAD'], cwd=root).stdout.decode().strip() == sha(env))
    need(not run(['git', 'status', '--porcelain', '--untracked-files=no'], cwd=root).stdout)
    if env.get('NORMAL_SOURCE_BRANCH'):
        need(run(['git', 'remote', 'get-url', 'origin'], cwd=root).stdout.decode().strip()
             in ('https://github.com/' + REPOSITORY, 'https://github.com/' + REPOSITORY + '.git'))
    return root


def data_paths(env, root, run):
    paths = run(['git', 'ls-files', 'infra/gcp'], cwd=root).stdout.decode().splitlines()
    if env['NORMAL_TARGET'] == 'prod':
        return paths  # 本番の従来のsource/実行方式は変更しない。
    prefixes = ('infra/gcp/environments/dev/', 'infra/gcp/modules/')
    return [name for name in paths if name.startswith(prefixes)]


def package_data(tfroot, tfvars):
    """配備用の固定4ファイルをデータとしてZIP化し、source側のスクリプトは実行しない。"""
    if tfvars.get('monitor_auth_registration') is None:
        return
    directory = tfroot.parents[1] / 'modules/monitor-auth-registration/function'
    with zipfile.ZipFile(directory / 'source.zip', 'w', compression=zipfile.ZIP_STORED) as archive:
        for name in ('guard.cjs', 'index.cjs', 'package.json', 'package-lock.json'):
            path = directory / name
            need(path.is_file() and not path.is_symlink())
            info = zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0))
            info.create_system = 3
            info.external_attr = 0o100644 << 16
            archive.writestr(info, path.read_bytes())
