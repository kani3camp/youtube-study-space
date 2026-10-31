"""DEV control planeとTerraform対象の分離を、実Git checkoutとモックAPIで検証する。"""
from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import MagicMock, patch
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
import terraform_normal_ops as ops
import terraform_source as selected
from test_terraform_normal_ops import env, plan


class SourceBoundary(unittest.TestCase):
    def git(self, root, *args):
        return subprocess.run(['git', *args], cwd=root, capture_output=True, check=True).stdout.decode().strip()

    def repository(self, root, files):
        root.mkdir(parents=True, exist_ok=True)
        self.git(root, 'init', '-q')
        self.git(root, 'remote', 'add', 'origin', 'https://github.com/' + selected.REPOSITORY)
        for relative, content in files.items():
            path = root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content)
        self.git(root, 'add', '.')
        self.git(root, '-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid',
                 '-c', 'core.hooksPath=/dev/null', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture')
        return self.git(root, 'rev-parse', 'HEAD')

    def fixture(self, temporary):
        control = Path(temporary) / 'control'
        control_sha = self.repository(control, {
            'infra/gcp/environments/dev/main.tf': '# trusted control data',
            'infra/gcp/environments/dev/normal.tfvars.json': '{"marker":"control"}',
            'infra/gcp/environments/dev/normal-operations.json': '{"destructive_changes":{}}'})
        source = control / selected.CHECKOUT
        source_sha = self.repository(source, {
            'infra/gcp/environments/dev/main.tf': '# selected source',
            'infra/gcp/environments/dev/normal.tfvars.json': '{"marker":"source"}',
            'infra/gcp/environments/dev/normal-operations.json': '{"destructive_changes":{"google_pubsub_topic.reviewed":["delete"]}}',
            'infra/gcp/modules/example/main.tf': '# selected module',
            'infra/gcp/environments/prod/main.tf': '# must not be copied',
            'infra/gcp/scripts/terraform_normal_ops.py': 'raise RuntimeError("SOURCE_SCRIPT_EXECUTED")',
            '.github/workflows/ci.yml': 'SOURCE_WORKFLOW_MUST_NOT_RUN'})
        e = env() | {'GITHUB_SHA': control_sha, 'NORMAL_SOURCE_BRANCH': 'feature/selected',
                     'NORMAL_EXPECTED_SHA': source_sha}
        return control, source, e

    def test_owner_dispatch_runs_only_from_dev_but_source_sha_can_differ(self):
        e = env() | {'NORMAL_SOURCE_BRANCH': 'feature/source', 'NORMAL_EXPECTED_SHA': 'b' * 40}
        self.assertEqual(ops.context(e, 'plan'), 'dev')
        for ref in ('refs/heads/feature/source', 'refs/heads/main', 'refs/tags/dev', 'refs/pull/1/merge'):
            with self.subTest(ref=ref), self.assertRaises(ValueError):
                ops.context(e | {'GITHUB_REF': ref, 'GITHUB_WORKFLOW_REF': ops.CALLER + ref}, 'plan')
        for key, value in (('GITHUB_ACTOR', 'other'), ('GITHUB_ACTOR_ID', '0'),
                           ('GITHUB_TRIGGERING_ACTOR', 'other'), ('GITHUB_RUN_ATTEMPT', '2'),
                           ('GITHUB_EVENT_NAME', 'pull_request')):
            with self.subTest(key=key), self.assertRaises(ValueError):
                ops.context(e | {key: value}, 'plan')

    def test_selector_is_same_repository_branch_and_exact_sha(self):
        e = env() | {'NORMAL_SOURCE_BRANCH': 'feature/source', 'NORMAL_EXPECTED_SHA': 'b' * 40}
        response = MagicMock()
        good = {'ref': 'refs/heads/feature/source', 'object': {'type': 'commit', 'sha': 'b' * 40}}
        response.__enter__.return_value.read.return_value = json.dumps(good).encode()
        with patch.object(selected.urllib.request, 'urlopen', return_value=response) as request:
            selected.resolve(e | {'GITHUB_TOKEN': 'FAKE_TOKEN'})
        sent = request.call_args.args[0]
        self.assertEqual(sent.full_url, 'https://api.github.com/repos/kani3camp/youtube-study-space/git/ref/heads/feature%2Fsource')
        self.assertEqual(sent.get_method(), 'GET')
        for change in ({'ref': 'refs/heads/other'}, {'object': {'type': 'tag', 'sha': 'b' * 40}},
                       {'object': {'type': 'commit', 'sha': 'c' * 40}}):
            response.__enter__.return_value.read.return_value = json.dumps(good | change).encode()
            with patch.object(selected.urllib.request, 'urlopen', return_value=response), self.assertRaises(ValueError):
                selected.resolve(e | {'GITHUB_TOKEN': 'FAKE_TOKEN'})
        for value in ('https://github.com/fork/repo', 'refs/pull/1/merge', '../x', 'x/../y', '-option', 'branch\nname'):
            with self.subTest(value=value), self.assertRaises(ValueError):
                selected.identity(e | {'NORMAL_SOURCE_BRANCH': value})

    def test_selected_hcl_inputs_review_and_assets_are_data_only(self):
        with tempfile.TemporaryDirectory() as temporary:
            control, source, e = self.fixture(temporary)
            root = Path(temporary) / 'private'
            with patch.object(ops, 'REPO', control):
                tfroot = ops.source(e, root)
                self.assertEqual((tfroot / 'main.tf').read_text(), '# selected source')
                self.assertTrue((root / 'checkout/infra/gcp/modules/example/main.tf').is_file())
                self.assertFalse((root / 'checkout/infra/gcp/scripts').exists())
                self.assertFalse((root / 'checkout/.github').exists())
                self.assertFalse((root / 'checkout/infra/gcp/environments/prod').exists())
                baseline = {'tfvars': {'project_id': ops.PROJECT}, 'plan_integrity_key': 'a' * 64}
                with patch.object(ops.common, 'decode_input_transport', return_value=baseline):
                    tfvars, _ = ops.inputs(e)
                self.assertEqual(tfvars['marker'], 'source')
                self.assertEqual(ops.review('dev', e), {'google_pubsub_topic.reviewed': ['delete']})

    def test_wrong_repository_sha_dirty_file_and_symlink_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            control, source, e = self.fixture(temporary)
            self.git(source, 'remote', 'set-url', 'origin', 'https://github.com/attacker/fork')
            with self.assertRaises(ValueError):
                selected.verify_checkout(e, control, ops.common.run)
            self.git(source, 'remote', 'set-url', 'origin', 'https://github.com/' + selected.REPOSITORY)
            with self.assertRaises(ValueError):
                selected.verify_checkout(e | {'NORMAL_EXPECTED_SHA': 'f' * 40}, control, ops.common.run)
            path = source / 'infra/gcp/environments/dev/main.tf'
            path.write_text('dirty')
            with self.assertRaises(ValueError):
                selected.verify_checkout(e, control, ops.common.run)
            relative = 'infra/gcp/environments/dev/normal.tfvars.json'
            path = source / relative
            path.unlink(); path.symlink_to(control / relative)
            with self.assertRaises(ValueError):
                selected.read_file(e, control, relative)

    def test_untracked_source_input_cannot_override_commit(self):
        with tempfile.TemporaryDirectory() as temporary:
            control, source, e = self.fixture(temporary)
            relative = 'infra/gcp/environments/dev/untracked.tfvars.json'
            (source / relative).write_text('{"marker":"untracked"}')
            with self.assertRaises(ValueError):
                selected.read_file(e, control, relative)

    def test_hmac_binds_control_sha_source_sha_and_source_branch(self):
        e = env() | {'NORMAL_SOURCE_BRANCH': 'feature/source', 'NORMAL_EXPECTED_SHA': 'b' * 40}
        with patch.object(ops, 'review', return_value={}):
            original = ops.commitment(e, {}, {}, plan(), b'k' * 32)
            for change in ({'GITHUB_SHA': 'c' * 40}, {'NORMAL_EXPECTED_SHA': 'd' * 40},
                           {'NORMAL_SOURCE_BRANCH': 'feature/other'}):
                self.assertNotEqual(original, ops.commitment(e | change, {}, {}, plan(), b'k' * 32))
                before = ops.commitment_components(e, {}, {}, plan(), b'k' * 32)
                after = ops.commitment_components(e | change, {}, {}, plan(), b'k' * 32)
                self.assertEqual([k for k in before if before[k] != after[k]], ['source_run_mode'])

    def test_source_sha_change_cannot_apply_a_prepared_plan(self):
        with tempfile.TemporaryDirectory() as temporary:
            e = env('apply') | {'RUNNER_TEMP': temporary, 'NORMAL_SOURCE_BRANCH': 'feature/source',
                                'NORMAL_EXPECTED_SHA': 'b' * 40}
            root = ops.private(e); root.mkdir(mode=0o700)
            prepared = {'source_sha': 'c' * 40, 'source_branch': 'feature/source', 'control_sha': e['GITHUB_SHA'],
                        'run_id': e['GITHUB_RUN_ID'], 'mode': e['NORMAL_MODE'], 'digest': 'a' * 64,
                        'binary_sha256': 'b' * 64, 'before': {}, 'counts': {}}
            ops.common.write(root / 'normal-prepared.json', ops.common.canonical(prepared))
            with patch.object(ops, 'apply_saved') as apply, self.assertRaises(ValueError):
                ops.apply_prepared(e)
            apply.assert_not_called()

    def test_deployment_archive_is_deterministic_without_source_script_execution(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            directory = root / 'modules/monitor-auth-registration/function'
            directory.mkdir(parents=True)
            names = ('guard.cjs', 'index.cjs', 'package.json', 'package-lock.json')
            for name in names:
                (directory / name).write_text('SOURCE_AS_DATA')
            (directory / 'untracked-secret.json').write_text('DO_NOT_PACKAGE')
            tfroot = root / 'environments/dev'
            tfvars = {'monitor_auth_registration': {}}
            selected.package_data(tfroot, tfvars)
            original = (directory / 'source.zip').read_bytes()
            selected.package_data(tfroot, tfvars)
            self.assertEqual(original, (directory / 'source.zip').read_bytes())
            with zipfile.ZipFile(directory / 'source.zip') as archive:
                self.assertEqual(archive.namelist(), list(names))

    def test_prod_cannot_select_another_source(self):
        e = env() | {'NORMAL_TARGET': 'prod', 'GITHUB_REF': 'refs/heads/main',
                     'GITHUB_WORKFLOW_REF': ops.CALLER + 'refs/heads/main', 'NORMAL_MODE': 'normal-plan'}
        self.assertEqual(ops.context(e), 'prod')
        with self.assertRaises(ValueError):
            ops.context(e | {'NORMAL_SOURCE_BRANCH': 'feature/source'})

    def test_preflight_never_prints_metadata_api_errors_or_tokens(self):
        output = io.StringIO()
        with patch.dict(ops.os.environ, env(), clear=True), patch.object(sys, 'argv', ['ops', 'preflight']), \
                patch.object(selected, 'resolve', side_effect=RuntimeError('PRIVATE_ERROR_TOKEN')), redirect_stdout(output):
            self.assertEqual(ops.main(), 1)
        self.assertNotIn('PRIVATE_ERROR_TOKEN', output.getvalue())


if __name__ == '__main__':
    unittest.main()
