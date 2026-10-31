"""DEV control planeとTerraform対象の分離を、実Git checkoutとモックAPIで検証する。"""
from contextlib import redirect_stdout
import io
import base64
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import MagicMock, patch

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
                     'NORMAL_EXPECTED_SHA': source_sha, 'NORMAL_SOURCE_SHA': source_sha}
        return control, source, e

    def test_owner_dispatch_control_ref_is_checked_without_a_branch_name_allowlist(self):
        e = env() | {'NORMAL_SOURCE_BRANCH': 'feature/source', 'NORMAL_EXPECTED_SHA': 'b' * 40, 'NORMAL_SOURCE_SHA': 'b' * 40}
        self.assertEqual(ops.context(e, 'plan'), 'dev')
        for ref in ('refs/heads/main', 'refs/tags/dev', 'refs/pull/1/merge'):
            with self.subTest(ref=ref), self.assertRaises(ValueError):
                ops.context(e | {'GITHUB_REF': ref, 'GITHUB_WORKFLOW_REF': ops.CALLER + ref}, 'plan')
        for control in ('dev', 'integration/mypage-canon-20261006', 'feature/trusted-controller'):
            for caller in ops.DEV_CALLERS:
                ref = 'refs/heads/' + control
                context = e | {'GITHUB_REF': ref, 'GITHUB_WORKFLOW_REF': caller + ref}
                self.assertEqual(ops.context(context, 'plan'), 'dev')
                self.assertEqual(selected.branch(context | {'NORMAL_SOURCE_BRANCH': ''}), '')
        for key, value in (('GITHUB_ACTOR', 'other'), ('GITHUB_ACTOR_ID', '0'),
                           ('GITHUB_TRIGGERING_ACTOR', 'other'), ('GITHUB_RUN_ATTEMPT', '2'),
                           ('GITHUB_EVENT_NAME', 'pull_request')):
            with self.subTest(key=key), self.assertRaises(ValueError):
                ops.context(e | {key: value}, 'plan')

    def test_selector_is_same_repository_branch_and_exact_sha(self):
        e = env() | {'NORMAL_SOURCE_BRANCH': 'feature/source', 'NORMAL_EXPECTED_SHA': 'b' * 40, 'NORMAL_SOURCE_SHA': 'b' * 40}
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

    def test_branch_only_is_resolved_once_and_shared_by_both_roles(self):
        raw = env() | {'NORMAL_SOURCE_BRANCH': 'feature/source', 'NORMAL_EXPECTED_SHA': '',
                       'GITHUB_TOKEN': 'FAKE_TOKEN'}
        response = MagicMock()
        response.__enter__.return_value.read.return_value = json.dumps({
            'ref': 'refs/heads/feature/source', 'object': {'type': 'commit', 'sha': 'b' * 40}}).encode()
        with patch.object(selected.urllib.request, 'urlopen', return_value=response) as request:
            resolved = selected.resolve(raw)
            self.assertEqual(resolved, {'branch': 'feature/source', 'sha': 'b' * 40})
            # preflight後にbranch HEADが進んでも、Plan/Applyは解決済みSHAだけを使う。
            response.__enter__.return_value.read.return_value = json.dumps({
                'ref': 'refs/heads/feature/source', 'object': {'type': 'commit', 'sha': 'c' * 40}}).encode()
            for role in ('plan', 'apply'):
                pinned = env(role) | {'NORMAL_EXPECTED_SHA': '', 'NORMAL_SOURCE_BRANCH': resolved['branch'],
                                      'NORMAL_SOURCE_SHA': resolved['sha']}
                self.assertEqual(ops.context(pinned, role), 'dev')
                self.assertEqual(selected.sha(pinned), 'b' * 40)
            request.assert_called_once()

    def test_sha_only_selects_a_commit_without_inferring_a_branch(self):
        raw = env() | {'NORMAL_SOURCE_BRANCH': '', 'NORMAL_EXPECTED_SHA': 'b' * 40, 'GITHUB_TOKEN': 'FAKE_TOKEN'}
        with patch.object(selected, 'require_branch_commit') as proof:
            resolved = selected.resolve(raw)
        self.assertEqual(resolved, {'branch': '', 'sha': 'b' * 40})
        proof.assert_called_once_with(raw, 'b' * 40)
        self.assertEqual(selected.identity(raw | {'NORMAL_SOURCE_SHA': resolved['sha']})['branch'], '')
        with patch.object(selected, 'require_branch_commit', side_effect=ValueError), self.assertRaises(ValueError):
            selected.resolve(raw)

    def test_sha_only_requires_a_normal_branch_ancestor_not_pr_or_tag_objects(self):
        with tempfile.TemporaryDirectory() as temporary:
            remote = Path(temporary) / 'remote'
            base = self.repository(remote, {'file': 'fixture'})
            self.git(remote, 'branch', '-M', 'main')
            def commit(message, *parents):
                args = ['-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid',
                        'commit-tree', base + '^{tree}', '-m', message]
                for parent in parents:
                    args += ['-p', parent]
                return self.git(remote, *args)
            main = commit('main', base)
            feature = commit('feature', base)
            fork = commit('fork-only', base)
            merge = commit('GitHub synthetic PR merge', main, fork)
            tag = commit('tag-only', base)
            self.git(remote, '-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid',
                     'tag', '--no-sign', '-a', 'reachable', '-m', 'fixture', main)
            annotated_tag = self.git(remote, 'rev-parse', 'refs/tags/reachable')
            refs = {'refs/heads/main': main, 'refs/heads/feature/nested': feature,
                    'refs/pull/1522/head': fork, 'refs/pull/1522/merge': merge, 'refs/tags/only': tag}
            for ref, sha in refs.items():
                self.git(remote, 'update-ref', ref, sha)
            for sha in (base, main, feature, fork, merge, tag):
                self.git(remote, 'cat-file', '-e', sha + '^{commit}')  # 全て存在しても許可しない。
            self.git(remote, 'config', 'uploadpack.allowFilter', 'true')
            original_run = subprocess.run
            fetches = []
            def local_transport(args, **kwargs):
                if 'fetch' in args:
                    self.assertEqual(args[-2:], ['https://github.com/' + selected.REPOSITORY + '.git',
                                                '+refs/heads/*:refs/heads/*'])
                    self.assertIn('--no-tags', args)
                    self.assertFalse(any(arg.startswith('--depth') for arg in args))
                    self.assertNotIn('FAKE_TOKEN', ' '.join(args))
                    fetches.append(args)
                    args = [remote.as_uri() if value == args[-2] else value for value in args]
                result = original_run(args, **kwargs)
                if 'fetch' in args and result.returncode == 0:
                    # 存在確認だけでは通せないよう、サーバーが余分なobjectを送った状況も再現する。
                    # FETCH_HEADやtag objectは通常branchへの到達性の証明に使えない。
                    extra = original_run(['git', '-C', args[2], 'fetch', '--no-tags', remote.as_uri(),
                        'refs/pull/1522/merge', 'refs/tags/only', 'refs/tags/reachable'], **kwargs)
                    self.assertEqual(extra.returncode, 0)
                    for sha in (fork, merge, tag, annotated_tag):
                        exists = original_run(['git', '-C', args[2], 'cat-file', '-e', sha], **kwargs)
                        self.assertEqual(exists.returncode, 0)
                    self.assertNotIn('AUTHORIZATION', (Path(args[2]) / 'config').read_text())
                return result
            with patch.object(selected.subprocess, 'run', side_effect=local_transport), \
                    patch.object(selected, 'github', side_effect=AssertionError('存在APIへのfallbackは禁止')):
                for sha in (base, main, feature):
                    with self.subTest(allowed=sha):
                        resolved = selected.resolve(env() | {'NORMAL_EXPECTED_SHA': sha, 'GITHUB_TOKEN': 'FAKE_TOKEN'})
                        self.assertEqual(resolved, {'branch': '', 'sha': sha})
                for sha in (fork, merge, tag, annotated_tag):
                    with self.subTest(rejected=sha), self.assertRaises(ValueError):
                        selected.resolve(env() | {'NORMAL_EXPECTED_SHA': sha, 'GITHUB_TOKEN': 'FAKE_TOKEN'})
            self.assertEqual(len(fetches), 7)

    def test_branch_proof_fetch_failure_rejects_instead_of_falling_back_to_existence(self):
        raw = env() | {'NORMAL_EXPECTED_SHA': 'b' * 40, 'GITHUB_TOKEN': 'FAKE_TOKEN'}
        with patch.object(selected.subprocess, 'run', side_effect=[
                subprocess.CompletedProcess([], 0, stdout=b'', stderr=b''),
                subprocess.CompletedProcess([], 1, stdout=b'', stderr=b'PRIVATE_API_ERROR')]), \
                patch.object(selected, 'github') as existence, self.assertRaises(ValueError):
            selected.resolve(raw)
        existence.assert_not_called()

    def test_empty_selection_uses_immutable_control_commit_without_api_lookup(self):
        raw = env() | {'NORMAL_SOURCE_BRANCH': '', 'NORMAL_EXPECTED_SHA': '',
                       'GITHUB_REF': 'refs/heads/integration/selected'}
        with patch.object(selected.urllib.request, 'urlopen') as request:
            self.assertEqual(selected.resolve(raw), {'branch': 'integration/selected', 'sha': raw['GITHUB_SHA']})
            self.assertEqual(selected.resolve(raw | {'NORMAL_EXPECTED_SHA': raw['GITHUB_SHA']}),
                             {'branch': '', 'sha': raw['GITHUB_SHA']})
        request.assert_not_called()

    def test_bad_sha_is_rejected_before_metadata_request_or_secret_use(self):
        for value in ('short', 'b' * 39, 'B' * 40, 'b' * 40 + '\n'):
            with patch.object(selected.urllib.request, 'urlopen') as request, self.assertRaises(ValueError):
                selected.resolve(env() | {'NORMAL_EXPECTED_SHA': value})
            request.assert_not_called()

    def test_credentialed_jobs_require_the_pinned_output_not_a_new_resolution(self):
        for role in ('plan', 'apply'):
            for value in ('', 'short', 'c' * 40):
                with self.subTest(role=role, pin=value), self.assertRaises(ValueError):
                    ops.context(env(role) | {'NORMAL_SOURCE_SHA': value}, role)

    def test_resolved_commit_is_published_for_checkout_and_later_apply(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'output'
            summary = Path(temporary) / 'summary'
            e = env() | {'NORMAL_EXPECTED_SHA': '', 'GITHUB_OUTPUT': str(output),
                         'GITHUB_STEP_SUMMARY': str(summary)}
            selected.publish(e, {'branch': 'feature/source', 'sha': 'b' * 40})
            self.assertEqual(output.read_text(), 'source_sha=' + 'b' * 40 + '\nsource_branch=feature/source\n')
            self.assertIn('`' + 'b' * 40 + '`', summary.read_text())
            self.assertNotIn('FAKE_TOKEN', summary.read_text())

    def test_oidc_binds_allowed_control_ref_and_caller_not_source_ref(self):
        ref = 'refs/heads/integration/mypage-canon-20261006'
        e = env() | {'GITHUB_REF': ref, 'GITHUB_WORKFLOW_REF': ops.DEV_CALLERS[1] + ref,
                     'NORMAL_SOURCE_BRANCH': 'feature/data', 'NORMAL_EXPECTED_SHA': 'b' * 40, 'NORMAL_SOURCE_SHA': 'b' * 40,
                     'ACTIONS_ID_TOKEN_REQUEST_URL': 'https://fixture.invalid/token?x=1',
                     'ACTIONS_ID_TOKEN_REQUEST_TOKEN': 'FAKE_REQUEST_TOKEN'}
        fields = {'repository_id': '340900071', 'repository_owner_id': '54093651',
                  'environment': 'terraform-dev-plan', 'ref': ref,
                  'workflow_ref': e['GITHUB_WORKFLOW_REF'], 'job_workflow_ref': ops.REUSABLE + ref,
                  'event_name': 'workflow_dispatch'}
        claims = fields | {'actor_id': '54093651', 'actor': 'kani3camp', 'run_attempt': '1',
                           'run_id': e['GITHUB_RUN_ID'], 'workflow_sha': e['GITHUB_SHA'],
                           'job_workflow_sha': e['GITHUB_SHA']}
        claims['sub'] = ':'.join(f'{key}:{value.replace(":", "%3A")}' for key, value in fields.items())
        response = MagicMock()
        def reply(value):
            payload = base64.urlsafe_b64encode(json.dumps(value).encode()).decode().rstrip('=')
            response.__enter__.return_value.read.return_value = json.dumps({'value': 'header.' + payload + '.signature'}).encode()
            with patch.object(ops.common.urllib.request, 'urlopen', return_value=response):
                ops.oidc(e)
        reply(claims)
        for change in ({'ref': 'refs/heads/feature/data'}, {'workflow_sha': 'b' * 40},
                       {'actor_id': '0'}, {'job_workflow_ref': ops.REUSABLE + 'refs/heads/dev'}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                reply(claims | change)

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

    def test_sha_only_uses_selected_checkout_instead_of_control_hcl(self):
        with tempfile.TemporaryDirectory() as temporary:
            control, source, e = self.fixture(temporary)
            e['NORMAL_SOURCE_BRANCH'] = ''
            with patch.object(ops, 'REPO', control):
                tfroot = ops.source(e, Path(temporary) / 'private')
            self.assertEqual((tfroot / 'main.tf').read_text(), '# selected source')

    def test_wrong_repository_sha_dirty_file_and_symlink_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            control, source, e = self.fixture(temporary)
            self.git(source, 'remote', 'set-url', 'origin', 'https://github.com/attacker/fork')
            with self.assertRaises(ValueError):
                selected.verify_checkout(e, control, ops.common.run)
            self.git(source, 'remote', 'set-url', 'origin', 'https://github.com/' + selected.REPOSITORY)
            with self.assertRaises(ValueError):
                selected.verify_checkout(e | {'NORMAL_SOURCE_SHA': 'f' * 40}, control, ops.common.run)
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
        e = env() | {'NORMAL_SOURCE_BRANCH': 'feature/source', 'NORMAL_EXPECTED_SHA': 'b' * 40, 'NORMAL_SOURCE_SHA': 'b' * 40}
        with patch.object(ops, 'review', return_value={}):
            original = ops.commitment(e, {}, {}, plan(), b'k' * 32)
            for change in ({'GITHUB_SHA': 'c' * 40}, {'NORMAL_SOURCE_SHA': 'd' * 40},
                           {'NORMAL_SOURCE_BRANCH': 'feature/other'}):
                self.assertNotEqual(original, ops.commitment(e | change, {}, {}, plan(), b'k' * 32))
                before = ops.commitment_components(e, {}, {}, plan(), b'k' * 32)
                after = ops.commitment_components(e | change, {}, {}, plan(), b'k' * 32)
                self.assertEqual([k for k in before if before[k] != after[k]], ['source_run_mode'])

    def test_source_sha_change_cannot_apply_a_prepared_plan(self):
        with tempfile.TemporaryDirectory() as temporary:
            e = env('apply') | {'RUNNER_TEMP': temporary, 'NORMAL_SOURCE_BRANCH': 'feature/source',
                                'NORMAL_EXPECTED_SHA': 'b' * 40, 'NORMAL_SOURCE_SHA': 'b' * 40}
            root = ops.private(e); root.mkdir(mode=0o700)
            prepared = {'source_sha': 'c' * 40, 'source_branch': 'feature/source', 'control_sha': e['GITHUB_SHA'],
                        'run_id': e['GITHUB_RUN_ID'], 'mode': e['NORMAL_MODE'], 'digest': 'a' * 64,
                        'binary_sha256': 'b' * 64, 'before': {}, 'counts': {}}
            ops.common.write(root / 'normal-prepared.json', ops.common.canonical(prepared))
            with patch.object(ops, 'apply_saved') as apply, self.assertRaises(ValueError):
                ops.apply_prepared(e)
            apply.assert_not_called()

    def test_prod_cannot_select_another_source(self):
        e = env() | {'NORMAL_TARGET': 'prod', 'GITHUB_REF': 'refs/heads/main',
                     'GITHUB_WORKFLOW_REF': ops.CALLER + 'refs/heads/main', 'NORMAL_MODE': 'normal-plan'}
        e.pop('NORMAL_SOURCE_SHA', None)
        self.assertEqual(ops.context(e), 'prod')
        for change in ({'NORMAL_SOURCE_BRANCH': 'feature/source'}, {'NORMAL_SOURCE_SHA': e['GITHUB_SHA']}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                ops.context(e | change)

    def test_preflight_never_prints_metadata_api_errors_or_tokens(self):
        output = io.StringIO()
        with patch.dict(ops.os.environ, env(), clear=True), patch.object(sys, 'argv', ['ops', 'preflight']), \
                patch.object(selected, 'resolve', side_effect=RuntimeError('PRIVATE_ERROR_TOKEN')), redirect_stdout(output):
            self.assertEqual(ops.main(), 1)
        self.assertNotIn('PRIVATE_ERROR_TOKEN', output.getvalue())


if __name__ == '__main__':
    unittest.main()
