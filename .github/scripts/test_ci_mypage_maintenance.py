#!/usr/bin/env python3
"""Contracts for the manually-dispatched MyPage maintenance workflow."""
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/mypage-maintenance.yml"


class MyPageMaintenanceWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.text = WORKFLOW.read_text(encoding="utf-8")

    def test_checkout_precedes_google_auth_and_gcloud(self):
        checkout = self.text.index("uses: actions/checkout@v7")
        auth = self.text.index("uses: google-github-actions/auth@")
        gcloud = self.text.index("gcloud run jobs execute mypage-maintenance-operator")
        self.assertLess(checkout, auth)
        self.assertLess(auth, gcloud)

        checkout_block = self.text[checkout:self.text.index(
            "- name: Authenticate the dedicated job invoker", checkout
        )]
        self.assertIn("persist-credentials: false", checkout_block)

    def test_authentication_remains_wif_only(self):
        self.assertIn("id-token: write", self.text)
        self.assertIn("MYPAGE_MAINTENANCE_WIF_PROVIDER", self.text)
        self.assertNotIn("credentials_json:", self.text)
        self.assertNotIn("service_account_key", self.text)


if __name__ == "__main__":
    unittest.main()
