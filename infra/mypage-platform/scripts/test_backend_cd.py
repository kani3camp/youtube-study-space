"""DEV MyPage CD: only trusted integration source changes trigger the image path."""
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import backend_cd as cd


def env():
    return {
        "GITHUB_REPOSITORY": cd.c.REPOSITORY,
        "GITHUB_REPOSITORY_ID": "340900071",
        "GITHUB_REPOSITORY_OWNER_ID": "54093651",
        "GITHUB_EVENT_NAME": "push",
        "GITHUB_ACTOR": "kani3camp",
        "GITHUB_ACTOR_ID": "54093651",
        "GITHUB_TRIGGERING_ACTOR": "kani3camp",
        "GITHUB_RUN_ATTEMPT": "1",
        "GITHUB_REF": "refs/heads/" + cd.BRANCH,
        "GITHUB_WORKFLOW_REF": cd.c.CALLER,
        "GITHUB_SHA": "a" * 40,
        "MYPAGE_CD_BEFORE": "b" * 40,
    }


class CDTest(unittest.TestCase):
    def test_owner_only_integration_push(self):
        cd.push_context(env())
        for key, value in (
            ("GITHUB_EVENT_NAME", "workflow_dispatch"),
            ("GITHUB_EVENT_NAME", "pull_request"),
            ("GITHUB_REF", "refs/heads/main"),
            ("GITHUB_REF", "refs/heads/dev"),
            ("GITHUB_ACTOR_ID", "0"),
            ("GITHUB_RUN_ATTEMPT", "2"),
            ("GITHUB_SHA", "invalid"),
        ):
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                cd.push_context(env() | {key: value})

    def test_only_system_sources_trigger_build(self):
        self.assertTrue(cd.classify_paths(["system/Dockerfile.mypage-maintenance-operator"]))
        self.assertTrue(cd.classify_paths(["system/go.mod", "docs/README.md"]))
        self.assertFalse(cd.classify_paths(["docs/README.md"]))
        self.assertFalse(cd.classify_paths(["infra/mypage-platform/environments/dev/runtime-release.json"]))
        with self.assertRaises(ValueError):
            cd.classify_paths(["system/poison\npath"])

    def test_classify_does_not_need_mutable_github_credentials_or_release_pr(self):
        class Completed:
            returncode = 0
            stdout = b"system/go.mod\x00docs/README.md\x00"
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "out"
            with patch.object(cd.subprocess, "run", return_value=Completed()) as command:
                cd.classify(env() | {"GITHUB_OUTPUT": str(output)})
            self.assertEqual(output.read_text(), "build=true\n")
            self.assertEqual(command.call_args.args[0][-2:], ["b" * 40, "a" * 40])
        with self.assertRaises(ValueError):
            cd.classify(env() | {"MYPAGE_CD_BEFORE": "0" * 40})


if __name__ == "__main__":
    unittest.main()
