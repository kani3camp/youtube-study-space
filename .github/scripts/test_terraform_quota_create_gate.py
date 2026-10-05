import copy
import unittest
from test_terraform_protected_plan import post_fixture
from test_terraform_email_adoption_gate import EMAIL, NAME
from terraform_quota_create_gate import ADDRESSES, TYPES, expected, validate
from terraform_plan_summary import build_summary


def fixture(phase="before"):
    plan = post_fixture()
    for key in TYPES:
        value = expected(key, NAME)
        actions, before, unknown = ["create"], None, {"id": True, "name": True, "conditions": [{"name": True}]}
        if phase == "post":
            value["name"] = "projects/test-youtube-study-space/alertPolicies/fixture-" + key
            value["deletion_policy"] = "DELETE"
            actions, before, unknown = ["no-op"], copy.deepcopy(value), {}
        plan["resource_changes"].append({"address": f'module.youtube_quota_alerts.google_monitoring_alert_policy.quota["{key}"]',
            "mode": "managed", "type": "google_monitoring_alert_policy", "change": {
                "actions": actions, "before": before, "after": value, "after_unknown": unknown}})
    return plan


class QuotaGateTest(unittest.TestCase):
    def check(self, plan, phase="before"):
        validate(plan, channel_name=NAME, email=EMAIL, phase=phase)

    def test_three_fixed_creates_and_eight_post_noops(self):
        self.check(fixture())
        self.check(fixture("post"), "post")
        summary = build_summary(fixture(), environment="dev", git_sha="a" * 40, policy="import-only")
        self.assertEqual(summary["counts"]["create"], 3)
        self.assertEqual(summary["counts"]["no-op"], 5)
        self.assertFalse(summary["policy_passed"])
        self.assertNotIn(NAME, str(summary)); self.assertNotIn(EMAIL, str(summary))

    def test_wrong_semantics_channel_and_config_unknowns_stop(self):
        mutations = [
            lambda c: c["after"].update(project="youtube-study-space"),
            lambda c: c["after"].update(notification_channels=["projects/other/notificationChannels/foreign"]),
            lambda c: c["after"].update(enabled=False),
            lambda c: c["after"].update(severity="CRITICAL"),
            lambda c: c["after"]["conditions"][0]["condition_monitoring_query_language"][0].update(duration="120s"),
            lambda c: c["after"]["conditions"][0]["condition_monitoring_query_language"][0].update(query="unexpected"),
            lambda c: c["after_unknown"].update(notification_channels=[True]),
            lambda c: c["after_unknown"].update(enabled=True),
            lambda c: c.update(importing={"id": "unexpected"}),
            lambda c: c.update(actions=["update"]),
            lambda c: c["after"].update(unexpected_config="unexpected"),
        ]
        for mutate in mutations:
            plan = fixture(); mutate(plan["resource_changes"][-1]["change"])
            with self.subTest(mutate=mutate), self.assertRaises(ValueError): self.check(plan)

    def test_existing_resource_mutation_drift_duplicate_or_missing_policy_stop(self):
        for mutate in [
            lambda p: p["resource_changes"][0]["change"].update(actions=["update"]),
            lambda p: p.update(resource_drift=[{"address": "unexpected"}]),
            lambda p: p["resource_changes"].pop(),
            lambda p: p["resource_changes"].append(copy.deepcopy(p["resource_changes"][-1])),
            lambda p: p.update(output_changes={"unexpected": {"actions": ["create"]}}),
        ]:
            plan = fixture(); mutate(plan)
            with self.assertRaises(ValueError): self.check(plan)

    def test_post_requires_no_import_or_unknown_or_create(self):
        for mutate in [
            lambda c: c.update(actions=["create"]),
            lambda c: c.update(importing={"id": "unexpected"}),
            lambda c: c.update(after_unknown={"id": True}),
        ]:
            plan = fixture("post"); mutate(plan["resource_changes"][-1]["change"])
            with self.assertRaises(ValueError): self.check(plan, "post")


if __name__ == "__main__": unittest.main()
