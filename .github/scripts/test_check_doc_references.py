#!/usr/bin/env python3
"""Guard against silently dropping cross-directory README link coverage."""
from __future__ import annotations

from contextlib import redirect_stderr, redirect_stdout
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).with_name("check-doc-references.py")
spec = importlib.util.spec_from_file_location("check_doc_references", SCRIPT)
assert spec is not None and spec.loader is not None
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class DocumentationReferencesTests(unittest.TestCase):
    def test_aws_cdk_readme_is_mandatory(self):
        self.assertIn("aws-cdk/README.md", checker.TARGETS)

    def test_deleted_target_is_rejected_even_if_link_contains_heading(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "aws-cdk" / "README.md"
            source.parent.mkdir()
            source.write_text(
                "[historical packet](../infra/gcp/docs/deleted.md#old-heading)\n",
                encoding="utf-8",
            )
            with patch.object(checker, "ROOT", root), patch.object(
                checker, "TARGETS", ("aws-cdk/README.md",)
            ), redirect_stderr(io.StringIO()) as err:
                self.assertEqual(checker.main(), 1)
            self.assertIn("broken relative link", err.getvalue())
            self.assertIn("aws-cdk/README.md", err.getvalue())

    def test_existing_local_target_is_accepted(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "aws-cdk" / "README.md"
            target = root / "infra/gcp/docs/current.md"
            source.parent.mkdir()
            target.parent.mkdir(parents=True)
            source.write_text(
                "[current guide](../infra/gcp/docs/current.md#section)\n",
                encoding="utf-8",
            )
            target.write_text("# Section\n", encoding="utf-8")
            with patch.object(checker, "ROOT", root), patch.object(
                checker, "TARGETS", ("aws-cdk/README.md",)
            ), redirect_stdout(io.StringIO()):
                self.assertEqual(checker.main(), 0)

    def test_docs_markdown_files_are_checked_automatically(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "docs/privacy/cleanup.md"
            source.parent.mkdir(parents=True)
            source.write_text("[runbook](missing.md#gate)\n", encoding="utf-8")
            with patch.object(checker, "ROOT", root), patch.object(
                checker, "TARGETS", ()
            ), redirect_stderr(io.StringIO()) as err:
                self.assertEqual(checker.main(), 1)
            self.assertIn("docs/privacy/cleanup.md", err.getvalue())
            self.assertIn("broken relative link", err.getvalue())

    def test_ephemeral_pr_phrasing_in_governed_doc_is_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "docs/development/guide.md"
            source.parent.mkdir(parents=True)
            source.write_text("このPRでは暫定的に無効化する。\n", encoding="utf-8")
            with patch.object(checker, "ROOT", root), patch.object(
                checker, "TARGETS", ()
            ), redirect_stderr(io.StringIO()) as err:
                self.assertEqual(checker.main(), 1)
            self.assertIn("docs/development/guide.md:1", err.getvalue())
            self.assertIn("ephemeral PR-scoped phrasing", err.getvalue())

    def test_review_only_marker_in_source_and_rules_is_rejected(self):
        marker = "[NOTE" + " FOR REVIEW]"
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            go_file = root / "system/core/sample.go"
            go_file.parent.mkdir(parents=True)
            go_file.write_text(f"// {marker} temporary explanation\n", encoding="utf-8")
            rule_file = root / "system/.cursor/rules/edit-rule.mdc"
            rule_file.parent.mkdir(parents=True)
            rule_file.write_text(f"- {marker}\n", encoding="utf-8")
            with patch.object(checker, "ROOT", root), patch.object(
                checker, "TARGETS", ()
            ), redirect_stderr(io.StringIO()) as err:
                self.assertEqual(checker.main(), 1)
            output = err.getvalue()
            self.assertIn("system/core/sample.go:1", output)
            self.assertIn("system/.cursor/rules/edit-rule.mdc:1", output)

    def test_permanent_note_comment_and_clean_doc_are_accepted(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            go_file = root / "system/core/sample.go"
            go_file.parent.mkdir(parents=True)
            go_file.write_text("// NOTE: preserve invariant across retries\n", encoding="utf-8")
            doc_file = root / "docs/development/guide.md"
            doc_file.parent.mkdir(parents=True)
            doc_file.write_text("現行仕様の説明。\n", encoding="utf-8")
            with patch.object(checker, "ROOT", root), patch.object(
                checker, "TARGETS", ()
            ), redirect_stdout(io.StringIO()):
                self.assertEqual(checker.main(), 0)


if __name__ == "__main__":
    unittest.main()
