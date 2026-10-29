"""DEV MyPage CD: push path routing and review-first release contract (offline only)."""
import base64
import json
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
        "MYPAGE_CD_IMAGE_DIGEST": cd.runtime.REPOSITORY + "@sha256:" + "c" * 64,
        "GITHUB_TOKEN": "synthetic-token",
    }


def release():
    return {
        "image_digest": cd.runtime.REPOSITORY + "@sha256:" + "1" * 64,
        "image_source_sha": "d" * 40,
        "hosting_backend_host": cd.HOST,
        "previous_image_digest": cd.runtime.REPOSITORY + "@sha256:" + "2" * 64,
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

    def test_paths_never_mix_image_source_and_release_declaration(self):
        self.assertEqual(cd.classify_paths(["system/Dockerfile.mypage"]), (True, False))
        self.assertEqual(cd.classify_paths([cd.RELEASE_PATH]), (False, True))
        self.assertEqual(cd.classify_paths(["docs/README.md"]), (False, False))
        with self.assertRaises(ValueError):
            cd.classify_paths(["system/go.mod", cd.RELEASE_PATH])
        with self.assertRaises(ValueError):
            cd.classify_paths(["system/poison\npath"])

    def test_classify_emits_only_bounded_booleans(self):
        class Completed:
            returncode = 0
            stdout = b"system/go.mod\x00docs/README.md\x00"
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "out"
            with patch.object(cd.subprocess, "run", return_value=Completed()) as command:
                cd.classify(env() | {"GITHUB_OUTPUT": str(output)})
            self.assertEqual(output.read_text(), "build=true\ndeploy=false\n")
            self.assertEqual(command.call_args.args[0][-2:], ["b" * 40, "a" * 40])
        with self.assertRaises(ValueError):
            cd.classify(env() | {"MYPAGE_CD_BEFORE": "0" * 40})

    def test_schema_rejects_wrong_registry_and_missing_baseline(self):
        self.assertEqual(cd.declaration(json.dumps(release()).encode()), release())
        for mutation in (
            {"image_digest": "bad"},
            {"previous_image_digest": ""},
            {"hosting_backend_host": "evil.run.app"},
            {"image_source_sha": "head"},
            {"extra": "secret"},
        ):
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                cd.declaration(json.dumps(release() | mutation).encode())

    def release_pr(self, *, stale=False, unchanged=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / cd.RELEASE_PATH
            target.parent.mkdir(parents=True)
            original = release()
            if unchanged:
                original["image_digest"] = env()["MYPAGE_CD_IMAGE_DIGEST"]
            raw = json.dumps(original).encode()
            target.write_bytes(raw)
            calls = []
            def api(context, method, path, body=None):
                calls.append((method, path, body))
                if path.startswith("/git/ref/heads/"):
                    return {"object": {"sha": "b" * 40 if stale else "a" * 40}}
                if path.startswith("/contents/"):
                    return {"encoding": "base64", "content": base64.b64encode(raw).decode(),
                            "sha": "synthetic-file-sha"}
                if path == "/pulls":
                    return {"html_url": "https://github.com/" + cd.c.REPOSITORY + "/pull/9999"}
                return {}
            with patch.object(cd, "ROOT", root):
                if stale:
                    with self.assertRaises(ValueError):
                        cd.release_pr(env(), api)
                    url = None
                else:
                    url = cd.release_pr(env(), api)
        return url, calls

    def test_review_pr_contains_exact_digest_previous_and_source(self):
        url, calls = self.release_pr()
        self.assertTrue(url.endswith("/9999"))
        mutations = calls[2:]
        self.assertEqual([call[1] for call in mutations],
                         ["/git/refs", "/contents/" + cd.RELEASE_PATH,
                          "/pulls", "/actions/workflows/ci.yml/dispatches"])
        commit = json.loads(base64.b64decode(mutations[1][2]["content"]))
        self.assertEqual(commit["image_digest"], env()["MYPAGE_CD_IMAGE_DIGEST"])
        self.assertEqual(commit["previous_image_digest"], release()["image_digest"])
        self.assertEqual(commit["image_source_sha"], env()["GITHUB_SHA"])
        self.assertEqual(commit["hosting_backend_host"], cd.HOST)
        self.assertEqual(mutations[2][2]["base"], cd.BRANCH)
        self.assertEqual(mutations[3][2], {"ref": "automation/mypage-dev-image-" + "a" * 12})
        self.assertFalse(any("terraform" in path or "run.googleapis" in path
                             for _, path, _ in calls))

    def test_stale_head_stops_before_writes(self):
        url, calls = self.release_pr(stale=True)
        self.assertIsNone(url)
        self.assertEqual([entry[0] for entry in calls], ["GET"])

    def test_identical_digest_needs_no_review_pr(self):
        url, calls = self.release_pr(unchanged=True)
        self.assertIsNone(url)
        self.assertEqual(calls, [])


if __name__ == "__main__":
    unittest.main()
