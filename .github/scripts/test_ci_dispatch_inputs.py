"""手動実行（workflow_dispatch）の選択値が、Run 名と Job Summary に安全に残ることをテストする。

目的: Actions の一覧で主要な選択値を見分けられ、各 Run の Summary で全入力値を確認できる状態を保つ。
入力値を Markdown 表へ書くときに表崩れ・HTML・シェル展開・式展開が起きないこと、記録が資格情報の取得前に行われること、
記録の失敗や追加が既存の実行可否・認証境界を変えないことも確認する。
使われる場面: CI Repository ワークフローの contracts 検査(.github/ci-validation/validate.sh が `unittest discover -p 'test_ci_*.py'` で実行)。ローカルでも同じ方法で実行できる。
失敗したとき: CI Repository が失敗し、PRをマージできない。入力を追加・改名したワークフローでは、記録ステップの env も同じ PR で更新する。
"""
import html
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

from test_ci_workflows import ROOT, load

WORKFLOWS = ROOT / '.github/workflows'
STEP_NAME = 'Record dispatch inputs'
# Run 名に載せてよいのは選択肢が固定された入力だけ（任意文字列は長さ・内容が無制限）。
TITLE_INPUT_TYPES = {'choice', 'boolean', 'environment'}
INPUT_NAME = r'^[a-z][a-z0-9_]*$'
CONTEXT = ('event_name', 'ref', 'sha', 'actor', 'triggering_actor', 'run_attempt')


def dispatch_workflows():
    """選択入力を持つ workflow_dispatch のワークフローを列挙する（入力のない手動実行は対象外）。"""
    found = {}
    for path in sorted(WORKFLOWS.glob('*.yml')):
        workflow = load(path)
        dispatch = (workflow.get('on') or {}).get('workflow_dispatch') or {}
        if dispatch.get('inputs'):
            found[path.name] = workflow
    return found


def record_steps(workflow):
    return [(name, job, index, step) for name, job in workflow['jobs'].items()
            for index, step in enumerate(job.get('steps', [])) if step.get('name') == STEP_NAME]


def top_level_or(expression):
    """`${{ ... }}` 内の式を、括弧と文字列の外にある最後の `||` で左右に分ける。"""
    body = expression.removeprefix('${{').removesuffix('}}')
    depth, quoted, split = 0, False, None
    for index, char in enumerate(body):
        if char == "'":
            quoted = not quoted
        elif not quoted and char in '([':
            depth += 1
        elif not quoted and char in ')]':
            depth -= 1
        elif not quoted and depth == 0 and body.startswith('||', index):
            split = index
    return None if split is None else (body[:split].strip(), body[split + 2:].strip())


def expected_default(spec):
    if 'default' not in spec:
        return None
    default = spec['default']
    return str(default).lower() if isinstance(default, bool) else str(default)


class DispatchInputRecordTest(unittest.TestCase):
    def setUp(self):
        self.workflows = dispatch_workflows()

    def test_known_dispatch_workflows_are_covered(self):
        # 動的に列挙するため、新しい手動ワークフローも自動で以下の契約の対象になる。
        self.assertTrue({'terraform.yml', 'deploy-mypage-api.yml', 'deploy-mypage-maintenance.yml',
                         'deploy-mypage-frontend.yml', 'mypage-maintenance.yml'} <= set(self.workflows))

    def test_run_name_shows_only_fixed_choice_inputs_and_is_gated_for_other_events(self):
        for name, workflow in self.workflows.items():
            with self.subTest(workflow=name):
                run_name = workflow.get('run-name')
                self.assertIsInstance(run_name, str)
                inputs = workflow['on']['workflow_dispatch']['inputs']
                referenced = set(re.findall(r'inputs\.([A-Za-z0-9_-]+)', run_name))
                self.assertTrue(referenced, 'run-name must show the main selection')
                for key in referenced:
                    self.assertIn(inputs[key]['type'], TITLE_INPUT_TYPES, key)
                # inputs はドット記法の個別参照だけを許す（inputs['x'] や toJSON(inputs) を通さない）。
                self.assertEqual(len(re.findall(r'\binputs\b', run_name)), len(re.findall(r'\binputs\.\w', run_name)))
                self.assertNotIn('secrets', run_name)
                if set(workflow['on']) != {'workflow_dispatch'}:
                    # PR などでは inputs が空になるため、手動実行時だけ入力値を表示し、他のイベントは別の名前にする。
                    self.assertEqual(run_name.count('${{'), 1, run_name)
                    self.assertTrue(run_name.startswith("${{ github.event_name == 'workflow_dispatch' && "), run_name)
                    self.assertTrue(run_name.endswith(' }}'), run_name)
                    branches = top_level_or(run_name)
                    self.assertIsNotNone(branches, 'run-name needs a fallback for other events')
                    self.assertNotIn(branches[1], ('', "''"), 'the fallback must name the run')
                    self.assertNotIn('inputs', branches[1])

    def test_top_level_or_ignores_nested_and_quoted_operators(self):
        self.assertEqual(top_level_or("${{ a && format('x || y', (b || c)) || d }}"), ("a && format('x || y', (b || c))", 'd'))
        self.assertIsNone(top_level_or("${{ a && format('{0}', (b || c)) }}"))

    def test_record_step_lists_every_input_explicitly_through_env(self):
        for name, workflow in self.workflows.items():
            with self.subTest(workflow=name):
                steps = record_steps(workflow)
                self.assertEqual(len(steps), 1)
                step = steps[0][3]
                inputs = workflow['on']['workflow_dispatch']['inputs']
                expected = {'DISPATCH_INPUTS': ' '.join(inputs)}
                for key, spec in inputs.items():
                    expected['DISPATCH_VALUE_' + key.upper()] = '${{ inputs.' + key + ' }}'
                    if (default := expected_default(spec)) is not None:
                        expected['DISPATCH_DEFAULT_' + key.upper()] = default
                actual = {key: value if isinstance(value, str) else str(value).lower()
                          for key, value in step['env'].items()}
                self.assertEqual(actual, expected)
                for key in inputs:
                    self.assertRegex(key, INPUT_NAME, 'the recorder accepts only these input names')
                self.assertNotIn('${{', step['run'])
                self.assertIs(step['continue-on-error'], True)

    def test_every_workflow_uses_the_same_recorder(self):
        runs = {name: record_steps(workflow)[0][3]['run'] for name, workflow in self.workflows.items()}
        self.assertEqual(len(set(runs.values())), 1, sorted(runs))

    def test_record_runs_first_in_a_credentialless_job_that_does_not_wait_for_others(self):
        for name, workflow in self.workflows.items():
            with self.subTest(workflow=name):
                job_name, job, index, step = record_steps(workflow)[0]
                self.assertEqual(index, 0)
                self.assertNotIn('needs', job)
                self.assertNotIn('uses', job)
                self.assertNotIn('environment', job)
                self.assertNotIn('if', step)
                # 明示した読み取り権限だけを持ち、リポジトリ既定のトークン権限を継承しない。
                permissions = job.get('permissions', workflow.get('permissions'))
                self.assertIsInstance(permissions, dict)
                self.assertTrue(set(permissions.items()) <= {('contents', 'read')}, permissions)
                self.assertNotIn('secrets', str(job))
                if set(workflow['on']) != {'workflow_dispatch'}:
                    self.assertEqual(job['if'], "${{ github.event_name == 'workflow_dispatch' }}", job_name)
                else:
                    self.assertNotIn('if', job, job_name)

    def test_maintenance_record_job_does_not_gate_the_transition(self):
        jobs = load(WORKFLOWS / 'mypage-maintenance.yml')['jobs']
        self.assertEqual(set(jobs), {'record', 'transition'})
        self.assertEqual(jobs['record']['permissions'], {})
        # 記録ジョブ自体が失敗・タイムアウトしても Run の結論を変えない。
        self.assertIs(jobs['record']['continue-on-error'], True)
        self.assertNotIn('needs', jobs['transition'])

    def render(self, env, previous='existing summary\n', workflow='terraform.yml'):
        script = record_steps(self.workflows[workflow])[0][3]['run']
        with tempfile.TemporaryDirectory() as temporary:
            summary = Path(temporary) / 'summary'
            summary.write_text(previous)
            sentinel = Path(temporary) / 'pwned'
            base = {'PATH': os.environ.get('PATH', os.defpath), 'GITHUB_STEP_SUMMARY': str(summary),
                    'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_REF': 'refs/heads/main', 'GITHUB_SHA': 'a' * 40,
                    'GITHUB_ACTOR': 'owner', 'GITHUB_TRIGGERING_ACTOR': 'owner', 'GITHUB_RUN_ATTEMPT': '1',
                    'SENTINEL': str(sentinel)}
            result = subprocess.run(['bash', '-eo', 'pipefail', '-c', script], cwd=temporary,
                                    env=base | env, capture_output=True, text=True, timeout=30)
            self.assertFalse(sentinel.exists(), 'input was executed')
            return result, summary.read_text()

    @staticmethod
    def rows(summary):
        return {match[1]: match[2] for match in re.finditer(r'^\| `([a-z][a-z0-9_]*)` \| (.*) \|$', summary, re.M)}

    @staticmethod
    def decoded(cell):
        """<code> の中身を表示上の文字列へ戻す。HTML として解釈される素の記号は残らない。"""
        match = re.fullmatch(r'<code>([^<>`|*_\[\]\\]*)</code>', cell)
        assert match, cell
        return html.unescape(match[1])

    def test_summary_shows_values_defaults_and_context_without_dropping_existing_output(self):
        result, summary = self.render({
            'DISPATCH_INPUTS': 'environment allow_mypage_deletions source_branch enabled',
            'DISPATCH_VALUE_ENVIRONMENT': 'production', 'DISPATCH_DEFAULT_ENVIRONMENT': 'development',
            'DISPATCH_VALUE_ALLOW_MYPAGE_DELETIONS': 'false', 'DISPATCH_DEFAULT_ALLOW_MYPAGE_DELETIONS': 'false',
            'DISPATCH_VALUE_SOURCE_BRANCH': '', 'DISPATCH_DEFAULT_SOURCE_BRANCH': '',
            'DISPATCH_VALUE_ENABLED': 'true'})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(summary.startswith('existing summary\n'))
        rows = self.rows(summary)
        values = {key: cell.split(' | ') for key, cell in rows.items()}
        self.assertEqual(values['environment'], ['<code>production</code>', '<code>development</code>'])
        self.assertEqual(values['allow_mypage_deletions'], ['<code>false</code>', '<code>false</code>'])
        self.assertEqual(values['source_branch'], ['（空文字）', '（空文字）'])
        self.assertEqual(values['enabled'], ['<code>true</code>', '（定義なし）'])
        self.assertEqual({key: values[key] for key in CONTEXT}, {
            'event_name': ['<code>workflow&#95;dispatch</code>'], 'ref': ['<code>refs&#47;heads&#47;main</code>'],
            'sha': ['<code>' + 'a' * 40 + '</code>'], 'actor': ['<code>owner</code>'],
            'triggering_actor': ['<code>owner</code>'], 'run_attempt': ['<code>1</code>']})
        self.assertIn('実行コンテキスト（選択入力ではありません）', summary)
        self.assertLess(summary.index('`enabled`'), summary.index('`event_name`'))

    def test_each_workflow_records_every_input_with_its_own_env(self):
        for name, workflow in self.workflows.items():
            with self.subTest(workflow=name):
                inputs = workflow['on']['workflow_dispatch']['inputs']
                samples = {key: spec['options'][-1] if spec['type'] == 'choice' else
                           'true' if spec['type'] == 'boolean' else 'sample-' + key for key, spec in inputs.items()}
                env = {}
                for key, value in record_steps(workflow)[0][3]['env'].items():
                    value = value if isinstance(value, str) else str(value).lower()
                    match = re.fullmatch(r'\$\{\{ inputs\.([a-z0-9_]+) \}\}', value)
                    env[key] = samples[match[1]] if match else value
                result, summary = self.render(env, workflow=name)
                self.assertEqual(result.returncode, 0, result.stderr)
                rows = self.rows(summary)
                self.assertEqual(list(rows), [*inputs, *CONTEXT])
                for key, spec in inputs.items():
                    value, default = rows[key].split(' | ')
                    self.assertEqual(self.decoded(value), samples[key])
                    expected = expected_default(spec)
                    self.assertEqual(default if expected in (None, '') else self.decoded(default),
                                     {None: '（定義なし）', '': '（空文字）'}.get(expected, expected))

    def test_hostile_strings_cannot_break_the_table_inject_markup_or_run_commands(self):
        hostile = ('x | y\n| `forged` | row |\r\t<script>alert(1)</script> **b** [l](u) '
                   '$(touch "$SENTINEL") `touch "$SENTINEL"` ${{ secrets.X }} \\ \u202e\u2028\x01\x7f')
        result, summary = self.render({'DISPATCH_INPUTS': 'source_branch', 'DISPATCH_VALUE_SOURCE_BRANCH': hostile})
        self.assertEqual(result.returncode, 0, result.stderr)
        rows = self.rows(summary)
        self.assertNotIn('forged', rows)
        value, default = rows['source_branch'].split(' | ')
        self.assertEqual(default, '（定義なし）')
        visible = (hostile.replace('\n', '\\n').replace('\r', '\\r').replace('\t', '\\t')
                   .replace('\u202e', '\\u{202e}').replace('\u2028', '\\u{2028}')
                   .replace('\x01', '\\u{0001}').replace('\x7f', '\\u{007f}'))
        self.assertEqual(self.decoded(value), visible)
        table = [line for line in summary.splitlines() if line.startswith('| `')]
        self.assertTrue(all(line.count('|') == len(line.split(' | ')) + 1 for line in table))

    def test_unlisted_or_malformed_input_names_are_rejected(self):
        for env in ({'DISPATCH_INPUTS': 'environment'}, {'DISPATCH_INPUTS': 'bad|name'}):
            with self.subTest(env=env):
                result, summary = self.render(env)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(summary, 'existing summary\n')


if __name__ == '__main__':
    unittest.main()
