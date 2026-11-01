"""CI選択Summaryの短さ・詳細表示と、テスト出力のjobからの隔離を検証する。"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / '.github/scripts'


class PathSummaryTest(unittest.TestCase):
    def run_detector(self, *args, **environment):
        with tempfile.TemporaryDirectory() as temporary:
            summary = Path(temporary) / 'summary'
            output = Path(temporary) / 'output'
            result = subprocess.run(['bash', str(SCRIPTS / 'detect-ci-paths.sh'), *args], cwd=ROOT,
                env=os.environ | environment | {'GITHUB_STEP_SUMMARY': str(summary), 'GITHUB_OUTPUT': str(output)},
                capture_output=True, text=True, check=True, timeout=30)
            self.assertEqual(output.read_text(), result.stdout)
            return summary.read_text(), result.stdout

    def test_path_tests_do_not_modify_parent_job_summary_or_outputs(self):
        with tempfile.TemporaryDirectory() as temporary:
            files = {name: Path(temporary) / name for name in ('GITHUB_STEP_SUMMARY', 'GITHUB_OUTPUT')}
            for file in files.values():
                file.write_text('parent job sentinel\n')
            result = subprocess.run(['bash', str(SCRIPTS / 'test-detect-ci-paths.sh')], cwd=ROOT,
                env=os.environ | {name: str(path) for name, path in files.items()},
                capture_output=True, text=True, check=True, timeout=60)
            self.assertIn('tests passed', result.stdout)
            for file in files.values():
                self.assertEqual(file.read_text(), 'parent job sentinel\n')

    def test_only_selected_groups_are_visible_and_paths_are_collapsed(self):
        summary, output = self.run_detector('--paths', 'system/core/app.go', 'README.md')
        visible, details = summary.split('<details>', 1)
        self.assertIn('`system`、`firestore_integration`', visible)
        self.assertNotIn('false', summary)
        self.assertNotIn('youtube_monitor', summary)
        self.assertNotIn('system/core/app.go', visible)
        self.assertIn('変更ファイル: 2件 / グループ未対応: 1件', details)
        self.assertIn('<code>system/core/app.go</code>', details)
        self.assertIn('**グループ未対応のファイル**\n\n- <code>README.md</code>', details)
        self.assertEqual(summary.count('### '), 1)
        self.assertLess(len(visible.splitlines()), 8)
        self.assertIn('youtube_monitor=false\n', output)

    def test_ci_changes_select_all_without_listing_every_group(self):
        summary, output = self.run_detector('--paths', '.github/workflows/ci.yml')
        self.assertIn('**選択グループ:** 全グループ', summary)
        self.assertIn('CI設定の変更', summary)
        self.assertNotIn('`system`', summary)
        self.assertIn('all=true\n', output)

    def test_manual_runs_explain_scope_without_empty_path_sections(self):
        summary, _ = self.run_detector(GITHUB_EVENT_NAME='workflow_dispatch')
        self.assertIn('**選択グループ:** 全グループ', summary)
        self.assertIn('手動実行', summary)
        self.assertNotIn('<details>', summary)
        self.assertLessEqual(len(summary.splitlines()), 5)

    def test_unmatched_and_empty_changes_explain_common_checks(self):
        for args in ((), ('README.md',)):
            with self.subTest(paths=args):
                summary, _ = self.run_detector('--paths', *args)
                self.assertIn('追加チェックなし', summary)
                self.assertIn('共通のリポジトリチェックは常に実行', summary)
                self.assertEqual('<details>' in summary, bool(args))

    def test_path_markup_cannot_break_out_of_details(self):
        summary, _ = self.run_detector('--paths', 'docs/</details><script>&unsafe`.md')
        self.assertEqual(summary.count('</details>'), 1)
        self.assertNotIn('<script>', summary)
        self.assertIn('docs/&lt;/details&gt;&lt;script&gt;&amp;unsafe`.md', summary)

    def test_control_characters_are_visible_without_splitting_path_rows(self):
        summary, _ = self.run_detector('--paths', 'docs/a\nb\r\t.md', r'docs/a\n.md')
        visible, details = summary.split('**変更ファイル**', 1)
        changed, _ = details.split('**グループ未対応のファイル**', 1)
        self.assertIn('変更ファイル: 2件 / グループ未対応: 2件', visible)
        self.assertEqual(changed.count('- <code>'), 2)
        self.assertIn(r'<code>docs/a\nb\r\t.md</code>', changed)
        self.assertIn(r'<code>docs/a\\n.md</code>', changed)

    def test_known_document_checkers_are_matched_without_extra_groups(self):
        summary, output = self.run_detector('--paths', '.github/scripts/check-doc-references.py',
                                          '.github/scripts/test_check_doc_references.py')
        self.assertIn('追加チェックなし', summary)
        self.assertIn('グループ未対応: 0件', summary)
        self.assertIn('all=false\n', output)
        self.assertIn('gcp_terraform=false\n', output)


if __name__ == '__main__':
    unittest.main()
