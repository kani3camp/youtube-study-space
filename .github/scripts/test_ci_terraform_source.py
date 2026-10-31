"""検査対象SHAとPlan/Apply対象SHAの一致、および資格情報の分離を検証する。"""
from contextlib import redirect_stdout
import importlib.util
import io
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

from test_ci_workflows import ROOT, TERRAFORM_MODE, load

sys.path.insert(0, str(ROOT / 'infra/gcp/tests'))
import test_terraform_source as fixtures

spec = importlib.util.spec_from_file_location('ci_terraform_source', ROOT / '.github/scripts/ci-terraform-source.py')
resolver = importlib.util.module_from_spec(spec)
spec.loader.exec_module(resolver)


class SelectedValidationSourceTest(unittest.TestCase):
    def environment(self, **changes):
        env = fixtures.env() | {'CI_EXECUTION_MODE': '検査＋plan', 'NORMAL_MODE': 'normal-plan',
                                'NORMAL_EXPECTED_SHA': '', 'GITHUB_TOKEN': 'FIXTURE_TOKEN'}
        env.pop('NORMAL_SOURCE_SHA')
        return env | changes

    def test_branch_moves_after_selection_but_validation_and_plan_stay_on_selected_commit(self):
        fixture = fixtures.SourceBoundary()
        with tempfile.TemporaryDirectory() as temporary:
            control, target, env = fixture.fixture(temporary)
            selected_sha = fixture.git(target, 'rev-parse', 'HEAD')
            output = Path(temporary) / 'output'
            env = self.environment(GITHUB_SHA=env['GITHUB_SHA'], NORMAL_SOURCE_BRANCH='feature/selected',
                                   GITHUB_OUTPUT=str(output))
            reply = {'ref': 'refs/heads/feature/selected', 'object': {'type': 'commit', 'sha': selected_sha}}
            with patch.object(resolver.source, 'github', return_value=reply) as api:
                checked = resolver.resolve(env)
                # The target HEAD moves while checks are running.
                (target / 'infra/gcp/environments/dev/main.tf').write_text('# later commit, not checked')
                fixture.git(target, 'add', '.')
                fixture.git(target, '-c', 'user.name=fixture', '-c', 'user.email=fixture@example.invalid',
                            '-c', 'core.hooksPath=/dev/null', '-c', 'commit.gpgsign=false', 'commit', '-qm', 'later')
                self.assertNotEqual(checked, fixture.git(target, 'rev-parse', 'HEAD'))
                validation = Path(temporary) / 'validation'
                fixture.git(Path(temporary), 'clone', '--local', str(target), str(validation))
                fixture.git(validation, 'checkout', '--detach', checked)
                self.assertEqual(fixture.git(validation, 'rev-parse', 'HEAD'), selected_sha)
                self.assertEqual((validation / 'infra/gcp/environments/dev/main.tf').read_text(), '# selected source')
                self.assertNotEqual((validation / 'infra/gcp/environments/dev/main.tf').read_text(),
                                    (control / 'infra/gcp/environments/dev/main.tf').read_text())
                for mode in ('normal-plan', 'normal-apply'):
                    pinned = env | {'NORMAL_EXPECTED_SHA': checked, 'NORMAL_SOURCE_BRANCH': '', 'NORMAL_MODE': mode}
                    resolver.ops.context(pinned)
                    with patch.object(resolver.source, 'require_branch_commit') as proof:
                        planned = resolver.source.resolve(pinned)
                    proof.assert_called_once_with(pinned, checked)
                    self.assertEqual(planned['sha'], checked)
                api.assert_called_once()  # Branch HEAD is never resolved a second time.

    def test_explicit_sha_and_default_source_are_passed_through_without_branch_lookup(self):
        with tempfile.TemporaryDirectory() as temporary:
            for requested in ('', 'b' * 40):
                env = self.environment(NORMAL_EXPECTED_SHA=requested, GITHUB_OUTPUT=str(Path(temporary) / 'output'))
                with patch.object(resolver.source, 'require_branch_commit') as proof, \
                        patch.object(resolver.source, 'github', side_effect=AssertionError('no branch lookup')):
                    self.assertEqual(resolver.resolve(env), requested or env['GITHUB_SHA'])
                self.assertEqual(proof.call_count, int(bool(requested)))

    def test_prod_still_requires_exact_controller_main_sha(self):
        env = self.environment(NORMAL_TARGET='prod', GITHUB_REF='refs/heads/main',
            GITHUB_WORKFLOW_REF=resolver.ops.CALLER + 'refs/heads/main', NORMAL_EXPECTED_SHA='a' * 40)
        with patch.object(resolver.source, 'resolve', side_effect=AssertionError('prod is controller SHA')):
            self.assertEqual(resolver.resolve(env), env['GITHUB_SHA'])
        for change in ({'NORMAL_EXPECTED_SHA': ''}, {'NORMAL_EXPECTED_SHA': 'b' * 40},
                       {'NORMAL_SOURCE_BRANCH': 'feature/source'}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                resolver.resolve(env | change)

    def test_pr_and_validation_only_keep_controller_commit_and_ignore_terraform_selectors(self):
        for event, mode in (('pull_request', ''), ('pull_request', '検査＋apply'),
                            ('workflow_dispatch', '全検査のみ')):
            env = self.environment(GITHUB_EVENT_NAME=event, CI_EXECUTION_MODE=mode,
                                   NORMAL_SOURCE_BRANCH='ignored', NORMAL_EXPECTED_SHA='ignored')
            with patch.object(resolver.ops, 'context', side_effect=AssertionError('not authenticated')), \
                    patch.object(resolver.source, 'resolve', side_effect=AssertionError('no target lookup')):
                self.assertEqual(resolver.resolve(env), env['GITHUB_SHA'])

    def test_untrusted_context_and_selector_failure_publish_no_validation_sha(self):
        for changes in ({'GITHUB_ACTOR_ID': '0'}, {'NORMAL_EXPECTED_SHA': 'invalid'},
                        {'NORMAL_SOURCE_BRANCH': 'feature/source', 'NORMAL_EXPECTED_SHA': 'b' * 40}):
            with self.subTest(changes=changes), tempfile.TemporaryDirectory() as temporary:
                output = Path(temporary) / 'output'
                env = self.environment(GITHUB_OUTPUT=str(output), **changes)
                reply = {'ref': 'refs/heads/feature/source', 'object': {'type': 'commit', 'sha': 'c' * 40}}
                with patch.dict(os.environ, env, clear=True), patch.object(resolver.source, 'github', return_value=reply), \
                        redirect_stdout(io.StringIO()) as log:
                    self.assertEqual(resolver.main(), 1)
                self.assertFalse(output.exists())
                self.assertNotIn('FIXTURE_TOKEN', log.getvalue())

    def test_resolver_publishes_one_sha_for_the_entire_pipeline(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / 'output'
            with patch.dict(os.environ, self.environment(GITHUB_OUTPUT=str(output)), clear=True):
                self.assertEqual(resolver.main(), 0)
            outputs = dict(line.split('=', 1) for line in output.read_text().splitlines())
            self.assertEqual(outputs['validation_sha'], 'a' * 40)

    def test_workflow_checks_and_operations_share_sha_but_keep_separate_credentials(self):
        ci = load(ROOT / '.github/workflows/ci.yml')
        jobs = ci['jobs']
        selection = next(step for step in jobs['changes']['steps'] if step.get('id') == 'source')
        self.assertEqual(jobs['changes']['steps'][0]['with']['ref'], '${{ github.sha }}')
        self.assertEqual(jobs['ci-gate']['steps'][0]['with'],
                         {'ref': '${{ github.sha }}', 'persist-credentials': False})
        self.assertEqual(selection['run'], 'python3 .github/scripts/ci-terraform-source.py')
        self.assertEqual(selection['env'], {
            'CI_EXECUTION_MODE': '${{ inputs.execution_mode }}',
            'NORMAL_TARGET': '${{ inputs.terraform_target }}', 'NORMAL_MODE': TERRAFORM_MODE,
            'NORMAL_EXPECTED_SHA': '${{ inputs.terraform_expected_sha }}',
            'NORMAL_SOURCE_BRANCH': '${{ inputs.terraform_source_branch }}', 'GITHUB_TOKEN': '${{ github.token }}'})
        pinned = '${{ needs.changes.outputs.validation_sha }}'
        checkout = jobs['gcp-terraform-validate']['steps'][0]
        self.assertEqual(checkout['with'], {'ref': pinned, 'persist-credentials': False})
        self.assertEqual(jobs['repository']['with'], {'source_sha': pinned})
        repository = load(ROOT / '.github/workflows/ci-repository.yml')
        for job in repository['jobs'].values():
            self.assertEqual(job['steps'][0]['with'],
                             {'ref': '${{ inputs.source_sha }}', 'persist-credentials': False})
        for name in ('changes', 'gcp-terraform-validate', 'repository'):
            self.assertNotIn('id-token', jobs[name].get('permissions', ci['permissions']))
            self.assertNotIn('environment', jobs[name])
            self.assertNotIn('secrets', jobs[name])
        authenticated = jobs['gcp-terraform-authenticated']
        self.assertEqual(authenticated['with']['expected_sha'], pinned)
        self.assertEqual(authenticated['with']['source_branch'], '')
        self.assertIn('ci-gate', authenticated['needs'])
        reusable = load(ROOT / '.github/workflows/gcp-terraform-authenticated.yml')
        for role in ('normal_plan', 'normal_apply'):
            steps = reusable['jobs'][role]['steps']
            self.assertEqual(steps[0]['with']['ref'], '${{ github.sha }}')
            self.assertEqual(steps[1]['with']['ref'], '${{ needs.preflight.outputs.source_sha }}')


if __name__ == '__main__':
    unittest.main()
