import copy
import unittest

from terraform_email_adoption_gate import CHANNEL, EXISTING, validate
from terraform_plan_summary import build_summary

EMAIL = "fixture@example.invalid"
NAME = "projects/test-youtube-study-space/notificationChannels/private-fixture"


def fixture():
    value = {"project": "test-youtube-study-space", "name": NAME, "type": "email", "enabled": True,
             "force_delete": False, "labels": {"email_address": EMAIL}, "verification_status": ""}
    return {"resource_changes": [
        {"address": address, "mode": "managed", "change": {"actions": ["no-op"]}}
        for address in sorted(EXISTING)
    ] + [{"address": CHANNEL, "type": "google_monitoring_notification_channel", "mode": "managed", "change": {
        "actions": ["update"], "importing": {"id": NAME}, "before": value, "after": copy.deepcopy(value),
        "after_unknown": {}, "before_sensitive": {}, "after_sensitive": {"labels": {"email_address": True}}}}]}


class EmailAdoptionTest(unittest.TestCase):
    def test_equal_value_sensitivity_adoption_is_reviewable_but_not_import_only(self):
        plan = fixture()
        validate(plan, email=EMAIL, channel_name=NAME)
        summary = build_summary(plan, environment="dev", git_sha="0" * 40, policy="import-only")
        self.assertFalse(summary["policy_passed"])
        self.assertEqual(summary["counts"]["update"], 1)
        self.assertEqual(summary["counts"]["import"], 1)
        self.assertNotIn(EMAIL, str(summary))
        self.assertNotIn(NAME, str(summary))

    def test_unexpected_mutations_unknowns_imports_and_drift_are_rejected(self):
        def channel(plan): return plan["resource_changes"][-1]["change"]
        mutations = [
            lambda p: channel(p)["after"].update(enabled=False),
            lambda p: channel(p)["after_unknown"].update(labels={"email_address": True}),
            lambda p: channel(p).update(actions=["create"]),
            lambda p: channel(p)["after_sensitive"].update(name=True),
            lambda p: p.update(resource_drift=[{"address": CHANNEL}]),
            lambda p: p["resource_changes"][0]["change"].update(actions=["update"]),
            lambda p: p["resource_changes"][0]["change"].update(importing={"id": "other"}),
            lambda p: p["resource_changes"].pop(0),
            lambda p: p.update(output_changes={"unexpected": {"actions": ["create"]}}),
        ]
        for mutate in mutations:
            with self.subTest(mutate=mutate):
                plan = fixture(); mutate(plan)
                with self.assertRaises(ValueError): validate(plan, email=EMAIL, channel_name=NAME)

    def test_foreign_channel_and_mailbox_mismatch_are_rejected(self):
        for email, name in [("other@example.invalid", NAME), (EMAIL, NAME.replace("test-youtube-study-space", "youtube-study-space"))]:
            with self.assertRaises(ValueError): validate(fixture(), email=email, channel_name=name)


if __name__ == "__main__": unittest.main()
