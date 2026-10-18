import copy
import unittest

from test_terraform_email_adoption_gate import EMAIL, NAME, fixture
from terraform_protected_plan import adoption_summary


def post_fixture():
    plan = fixture()
    change = plan["resource_changes"][-1]["change"]
    change["actions"] = ["no-op"]
    change.pop("importing")
    change["before_sensitive"] = copy.deepcopy(change["after_sensitive"])
    return plan


class ProtectedPlanTest(unittest.TestCase):
    def summary(self, plan, phase="before", environment="dev"):
        return adoption_summary(plan, environment=environment, git_sha="a" * 40,
                                phase=phase, email=EMAIL, channel_name=NAME)

    def test_fixed_adoption_and_complete_post_noop_are_value_free(self):
        before = self.summary(fixture())
        self.assertEqual(before["counts"]["import"], 1)
        self.assertEqual(before["counts"]["update"], 1)
        self.assertEqual(before["counts"]["no-op"], 4)
        after = self.summary(post_fixture(), phase="post")
        self.assertEqual(after["counts"]["no-op"], 5)
        self.assertEqual(after["counts"]["import"], 0)
        for summary in (before, after):
            self.assertTrue(summary["policy_passed"])
            self.assertNotIn(EMAIL, str(summary))
            self.assertNotIn(NAME, str(summary))

    def test_all_resources_unknowns_and_changed_values_are_rejected(self):
        for phase, factory in [("before", fixture), ("post", post_fixture)]:
            for mutate in [
                lambda p: p["resource_changes"][0]["change"].update(after_unknown={"id": True}),
                lambda p: p["resource_changes"][0]["change"].update(after={"unexpected": True}),
                lambda p: p["resource_changes"][-1]["change"].update(actions=["create"]),
                lambda p: p["resource_changes"].pop(0),
                lambda p: p.update(resource_drift=[{"address": "unexpected"}]),
            ]:
                plan = factory(); mutate(plan)
                with self.subTest(phase=phase, mutate=mutate), self.assertRaises(ValueError):
                    self.summary(plan, phase=phase)

    def test_production_and_post_import_or_update_are_rejected(self):
        with self.assertRaises(ValueError): self.summary(fixture(), environment="prod")
        for mutation in [{"importing": {"id": NAME}}, {"actions": ["update"]}]:
            plan = post_fixture(); plan["resource_changes"][-1]["change"].update(mutation)
            with self.assertRaises(ValueError): self.summary(plan, phase="post")


if __name__ == "__main__": unittest.main()
