#!/usr/bin/env python3
import contextlib
import importlib.util
import io
import subprocess
import unittest
from unittest.mock import patch
from pathlib import Path

spec = importlib.util.spec_from_file_location("smoke", Path(__file__).with_name("terraform_identity_smoke.py"))
smoke = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)


class IdentitySmokeTest(unittest.TestCase):
    def test_dependency_failures_are_not_treated_as_permission_denial(self):
        for error in ("(NoSuchKey)", "(404)", "(PreconditionFailed)", "timeout PRIVATE_SENTINEL"):
            with self.subTest(error=error), self.assertRaises(smoke.SmokeFailure):
                smoke.denied_aws(subprocess.CompletedProcess([], 1, "", error), "denied-operation")

    def test_only_explicit_aws_denials_pass(self):
        for error in ("(AccessDenied) PRIVATE_SENTINEL", "(403) Forbidden PRIVATE_SENTINEL"):
            smoke.denied_aws(subprocess.CompletedProcess([], 1, "", error), "denied-operation")
        with self.assertRaises(smoke.SmokeFailure):
            smoke.denied_aws(subprocess.CompletedProcess([], 0, "", "(AccessDenied)"), "denied-operation")

    def run_gcp(self, *, permissions=None, prod_permissions=None, impersonation_status=403):
        calls = []
        def request(path, token, body=None, **kwargs):
            calls.append(path)
            if "generateAccessToken" in path:
                return impersonation_status, {"error": {"status": "PERMISSION_DENIED", "message": "PRIVATE_SENTINEL"}}
            if path == "v3/projects/youtube-study-space:testIamPermissions":
                return 200, {"permissions": prod_permissions or []}
            if path.endswith(":testIamPermissions"):
                return 200, {"permissions": permissions or []}
            return 200, {"projectId": "development-fixture", "state": "ACTIVE"}
        result = smoke.verify_google("PRIVATE_TOKEN", "terraform-dev-plan@development-fixture.iam.gserviceaccount.com", request, project="development-fixture")
        return result, calls

    def test_empty_graph_smoke_performs_read_and_denied_impersonation(self):
        result, calls = self.run_gcp()
        self.assertEqual(len(result), 4)
        self.assertEqual(calls[0], "v3/projects/development-fixture")
        self.assertIn("terraform-dev-apply@", calls[-1])
        self.assertNotIn("PRIVATE", " ".join(result))

    def test_unexpected_mutation_or_production_grant_stops_smoke(self):
        for overrides in ({"permissions": ["iam.serviceAccounts.create"]}, {"prod_permissions": ["resourcemanager.projects.get"]}, {"impersonation_status": 200}, {"impersonation_status": 404}):
            with self.subTest(overrides=overrides), self.assertRaises(smoke.SmokeFailure) as caught:
                self.run_gcp(**overrides)
            self.assertNotIn("PRIVATE", str(caught.exception))

    def test_unexpected_dependency_error_does_not_leak_publicly(self):
        output = io.StringIO()
        with patch.object(smoke, "verify_oidc", side_effect=RuntimeError("PRIVATE_TOKEN_AND_IDENTIFIER")), contextlib.redirect_stderr(output):
            self.assertEqual(smoke.main(), 1)
        self.assertNotIn("PRIVATE", output.getvalue())
        self.assertIn("raw error suppressed", output.getvalue())


if __name__ == "__main__":
    unittest.main()
