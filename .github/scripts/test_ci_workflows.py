"""振り分け漏れ・必須検査の迂回・認証情報の漏えいだけを対象にする。

YAMLと呼び出しの構文はactionlint、アプリケーションの仕様は各呼び出し先で検証する。
"""
import importlib.util
from pathlib import Path
import re
import unittest

import test_ci_results

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('codeql_source', ROOT / '.github/codeql/validate-source.py')
yaml_source = importlib.util.module_from_spec(spec)
spec.loader.exec_module(yaml_source)
load = yaml_source.load_yaml
gate = test_ci_results.gate

def selected(expression, flags):
    """小さなORだけの振り分け式を評価し、Actionsのジョブグラフは評価しない。"""
    terms = expression.removeprefix('${{').removesuffix('}}').strip().split(' || ')
    groups = [re.fullmatch(r"needs.changes.outputs.(\w+) == 'true'", term) for term in terms]
    if not all(groups):
        raise ValueError('Unsupported quality routing expression')
    return any(flags[g[1]] for g in groups)


class WorkflowSafetyTest(unittest.TestCase):
    def setUp(self):
        self.caller = load(ROOT / '.github/workflows/ci.yml')

    def test_representative_path_flags_reach_the_required_domains_and_leaf_inputs(self):
        cases = [set(), *({g} for g in gate.GROUPS), {'system', 'mypage'},
                 {'gcp_terraform', 'firestore_integration'}, set(gate.GROUPS)]
        for enabled in cases:
            flags = {g: g in enabled for g in gate.GROUPS}
            for name, groups in gate.DOMAIN_GROUPS.items():
                call = self.caller['jobs'][name]
                expected = flags['all'] or any(flags[g] for g in groups)
                with self.subTest(flags=enabled, domain=name):
                    self.assertEqual(selected(call['if'], flags), expected)
                    for key, expression in call.get('with', {}).items():
                        node = key in {'youtube_monitor', 'mypage', 'docs_site',
                                       'menu_image_generator', 'video_maker_simulator', 'figma_plugin'}
                        self.assertEqual(selected(expression, flags),
                                         flags[key] or flags['all'] or (node and flags['node_projects']))
                    if call.get('with'):
                        module = load(ROOT / call['uses'].removeprefix('./'))
                        used = set()
                        for leaf in module['jobs'].values():
                            match = re.fullmatch(r'\$\{\{ inputs\.(\w+) \}\}', leaf.get('if', ''))
                            self.assertIsNotNone(match, 'A leaf must use its routed domain input')
                            used.add(match[1])
                        self.assertEqual(used, set(call['with']))


    def test_required_gate_always_receives_every_callee_and_runs_the_result_policy(self):
        jobs = self.caller['jobs']
        check = jobs['ci-gate']
        self.assertEqual(check['if'], 'always()')
        self.assertEqual(set(check['needs']), set(jobs) - {'ci-gate'})
        self.assertEqual(set(check['needs']), gate.DEPENDENCIES)
        step = next(s for s in check['steps'] if 'check-ci-results.py' in s.get('run', ''))
        self.assertNotIn('if', step)
        self.assertNotIn('continue-on-error', step)
        self.assertEqual(step['env']['CI_NEEDS'], '${{ toJSON(needs) }}')

    def test_ci_never_receives_credentials_or_operation_inputs(self):
        self.assertEqual(self.caller['permissions'], {'contents': 'read'})
        # CI is validation only: no push trigger, no dispatch inputs, no operations job.
        self.assertEqual(set(self.caller['on']), {'pull_request', 'workflow_dispatch'})
        self.assertIsNone(self.caller['on']['workflow_dispatch'])
        self.assertNotIn('operations', self.caller['jobs'])
        for path in (ROOT / '.github/workflows').glob('ci*.yml'):
            workflow = load(path)
            self.assertNotEqual(workflow.get('permissions', {}).get('id-token'), 'write')
            for name, job in workflow['jobs'].items():
                with self.subTest(workflow=path.name, job=name):
                    self.assertNotEqual(job.get('permissions', {}).get('id-token'), 'write')
                    self.assertNotIn('secrets', job)
                    self.assertNotIn('environment', job)
