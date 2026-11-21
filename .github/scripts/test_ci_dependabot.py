"""Dependabot 設定がリポジトリ内の依存マニフェストを漏れなく・重複なく監視しているかをテストする。

目的: 新しく追加した package.json / go.mod / Dockerfile* / .terraform.lock.hcl が Dependabot の監視から黙って漏れること、削除済みディレクトリや重複設定が残ることを防ぐ。
使われる場面: CI Repository ワークフローの contracts 検査（.github/ci-validation/validate.sh が `unittest discover -p 'test_ci_*.py'` で実行）。ローカルでも同じ方法で実行できる。
失敗したとき: CI Repository が失敗し、PR をマージできない。
なくした場合: パッケージや Dockerfile を追加・移動したときに .github/dependabot.yml の更新漏れや参照切れに自動で気づけなくなる。
"""
import os
from pathlib import Path
import unittest

from test_ci_workflows import ROOT, load

DEPENDABOT_CONFIG = ROOT / '.github/dependabot.yml'

IGNORED_DIRS = frozenset({
    '.git',
    '.worktrees',
    '.ci-source',
    '.next',
    '.terraform',
    '.docusaurus',
    'node_modules',
    'dist',
    'build',
    'cdk.out',
    '__pycache__',
    '.venv',
    'venv',
})

ECOSYSTEM_MANIFEST_MATCHERS = {
    'npm': lambda name: name == 'package.json',
    'gomod': lambda name: name == 'go.mod',
    'docker': lambda name: name.startswith('Dockerfile'),
    'terraform': lambda name: name == '.terraform.lock.hcl',
}

_TERRAFORM_MANUAL_REASON = (
    'マルチプラットフォーム lockfile（linux_amd64 / darwin_arm64）と dev/prod・ルート間一致を '
    'CI で強制しており、保護された環境別 Plan/Apply レビューおよびネイティブテスト更新を伴うため手動 PR で一括更新する'
)

# 意図的に Dependabot 対象外とする (package-ecosystem, directory) と理由。
# 対象外を追加する場合は、必ず妥当な理由を明記し .github/dependabot.yml にもコメントを残す。
EXCLUDED_DIRECTORIES = {
    ('terraform', '/infra/gcp/environments/dev'): _TERRAFORM_MANUAL_REASON,
    ('terraform', '/infra/gcp/environments/prod'): _TERRAFORM_MANUAL_REASON,
    ('terraform', '/infra/mypage-platform/environments/dev'): _TERRAFORM_MANUAL_REASON,
    ('terraform', '/infra/mypage-platform/environments/prod'): _TERRAFORM_MANUAL_REASON,
    ('terraform', '/infra/mypage-oauth-ttl/environments/dev'): _TERRAFORM_MANUAL_REASON,
    ('terraform', '/infra/mypage-oauth-ttl/environments/prod'): _TERRAFORM_MANUAL_REASON,
}


def to_dependabot_dir(path: Path) -> str:
    rel = path.relative_to(ROOT)
    return '/' if rel == Path('.') else '/' + rel.as_posix()


def from_dependabot_dir(dependabot_dir: str) -> Path:
    if dependabot_dir == '/':
        return ROOT
    return ROOT / dependabot_dir.lstrip('/')


def discover_manifest_directories() -> dict[str, set[str]]:
    discovered = {ecosystem: set() for ecosystem in ECOSYSTEM_MANIFEST_MATCHERS}
    for current, dirnames, filenames in os.walk(ROOT):
        dirnames[:] = sorted(d for d in dirnames if d not in IGNORED_DIRS)
        current_path = Path(current)
        for ecosystem, matcher in ECOSYSTEM_MANIFEST_MATCHERS.items():
            if any(matcher(name) for name in filenames):
                discovered[ecosystem].add(to_dependabot_dir(current_path))
    return discovered


def entry_directories(entry: dict) -> list[str]:
    has_single = 'directory' in entry
    has_multi = 'directories' in entry
    if has_single == has_multi:
        raise AssertionError(
            f"Each update entry must set exactly one of 'directory' or 'directories': {entry!r}"
        )
    if has_single:
        directory = entry['directory']
        if not isinstance(directory, str) or not directory:
            raise AssertionError(f"Invalid 'directory' value: {directory!r}")
        return [directory]
    directories = entry['directories']
    if not isinstance(directories, list) or not directories:
        raise AssertionError(f"Invalid 'directories' value: {directories!r}")
    for directory in directories:
        if not isinstance(directory, str) or not directory:
            raise AssertionError(f"Invalid directory entry in 'directories': {directory!r}")
    return directories


class DependabotContractTest(unittest.TestCase):
    def setUp(self):
        self.config = load(DEPENDABOT_CONFIG)
        self.assertEqual(self.config.get('version'), 2)
        self.updates = self.config.get('updates')
        self.assertIsInstance(self.updates, list)
        self.assertTrue(self.updates, 'Dependabot updates list must not be empty')

    def configured_directories_by_ecosystem(self) -> dict[str, list[str]]:
        by_ecosystem: dict[str, list[str]] = {}
        for entry in self.updates:
            ecosystem = entry.get('package-ecosystem')
            self.assertIsInstance(ecosystem, str)
            self.assertTrue(ecosystem)
            by_ecosystem.setdefault(ecosystem, []).extend(entry_directories(entry))
        return by_ecosystem

    def test_every_manifest_directory_is_covered_or_explicitly_excluded(self):
        discovered = discover_manifest_directories()
        configured = {
            ecosystem: set(directories)
            for ecosystem, directories in self.configured_directories_by_ecosystem().items()
        }

        for ecosystem, directories in discovered.items():
            covered = configured.get(ecosystem, set())
            for directory in sorted(directories):
                with self.subTest(ecosystem=ecosystem, directory=directory):
                    if directory in covered:
                        continue
                    reason = EXCLUDED_DIRECTORIES.get((ecosystem, directory))
                    self.assertTrue(
                        isinstance(reason, str) and reason.strip(),
                        f'{ecosystem}:{directory} is not covered by .github/dependabot.yml '
                        'and has no justified entry in EXCLUDED_DIRECTORIES',
                    )

        for (ecosystem, directory), reason in sorted(EXCLUDED_DIRECTORIES.items()):
            with self.subTest(stale_exclusion=(ecosystem, directory)):
                self.assertTrue(
                    isinstance(reason, str) and reason.strip(),
                    f'Exclusion for {ecosystem}:{directory} must include a non-empty justification',
                )
                self.assertIn(
                    directory,
                    discovered.get(ecosystem, set()),
                    f'Stale exclusion: {ecosystem}:{directory} no longer has matching manifests',
                )
                self.assertNotIn(
                    directory,
                    configured.get(ecosystem, set()),
                    f'Redundant exclusion: {ecosystem}:{directory} is already configured in .github/dependabot.yml',
                )

    def test_configured_directories_exist_and_contain_expected_manifests(self):
        for ecosystem, directories in self.configured_directories_by_ecosystem().items():
            for directory in directories:
                with self.subTest(ecosystem=ecosystem, directory=directory):
                    self.assertTrue(
                        directory.startswith('/'),
                        f'Dependabot directory must start with "/": {directory}',
                    )
                    if directory != '/':
                        self.assertFalse(
                            directory.endswith('/'),
                            f'Dependabot directory must not have a trailing "/": {directory}',
                        )
                    target = from_dependabot_dir(directory)
                    self.assertTrue(
                        target.is_dir(),
                        f'Configured Dependabot directory does not exist: {directory}',
                    )
                    if ecosystem in ECOSYSTEM_MANIFEST_MATCHERS:
                        matcher = ECOSYSTEM_MANIFEST_MATCHERS[ecosystem]
                        files = [p.name for p in target.iterdir() if p.is_file()]
                        self.assertTrue(
                            any(matcher(name) for name in files),
                            f'Configured {ecosystem} directory {directory} has no expected manifest file',
                        )
                    elif ecosystem == 'github-actions':
                        workflows_dir = target / '.github/workflows'
                        self.assertTrue(
                            workflows_dir.is_dir(),
                            f'Configured github-actions directory {directory} has no .github/workflows/',
                        )
                        workflow_files = [
                            p for p in workflows_dir.iterdir()
                            if p.is_file() and p.suffix in ('.yml', '.yaml')
                        ]
                        self.assertTrue(
                            workflow_files,
                            f'Configured github-actions directory {directory} has no workflow YAML files',
                        )

    def test_no_duplicate_directories_within_same_ecosystem(self):
        for ecosystem, directories in self.configured_directories_by_ecosystem().items():
            with self.subTest(ecosystem=ecosystem):
                seen = set()
                duplicates = []
                for directory in directories:
                    if directory in seen:
                        duplicates.append(directory)
                    seen.add(directory)
                self.assertEqual(
                    duplicates,
                    [],
                    f'Duplicate directories configured for {ecosystem}: {duplicates}',
                )


if __name__ == '__main__':
    unittest.main()
