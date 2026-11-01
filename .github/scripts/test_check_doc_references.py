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


if __name__ == "__main__":
    unittest.main()
