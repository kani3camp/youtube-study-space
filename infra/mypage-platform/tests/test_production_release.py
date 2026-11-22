"""本番(MYPAGE_TARGET=production)の配備経路が、本番の固定値と main だけを受け入れるかをテストする。

目的: 配備スクリプトと配備用本人情報の契約が環境ごとの値(ブランチ・Environment・SA・Registry・WIF 条件)を取り違えて、DEV の経路で本番を、本番の経路で DEV を操作できてしまう事故を防ぐ。クラウド認証情報は使わない。
使われる場面: infra/mypage-platform/scripts/check-local.sh の `unittest discover`。CI GCP Terraform の terraform-tests(validate.sh 経由)で実行される。
失敗したとき: CI GCP Terraform が失敗し、PRをマージできない。
なくした場合: 本番を対象にした配備スクリプトの入口検査と、本番の配備用本人情報の Plan 契約が確かめられなくなる。

補足: 対象環境は import 時に決まるため、本番の検査は MYPAGE_TARGET=production の別プロセスで行う。
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import textwrap
import unittest

SCRIPTS = Path(__file__).resolve().parents[1] / "scripts"
SITE = "study-space"
HOST = "mypage-api-abc123def4-dt.a.run.app"


def run_production(code, **extra):
    env = {key: value for key, value in os.environ.items() if not key.startswith(("MYPAGE_", "GITHUB_"))}
    env |= {"MYPAGE_TARGET": "production", "PYTHONPATH": str(SCRIPTS)} | extra
    result = subprocess.run([sys.executable, "-c", textwrap.dedent(code)], env=env,
                            capture_output=True, text=True, timeout=60)
    if result.returncode != 0:
        raise AssertionError(result.stderr[-2000:])
    return json.loads(result.stdout)


CONTEXT = """
    import json
    import foundation_contract as c
    import backend_release as b
    import hosting_release as h
    import operator_release as o
    import image_ops
    import read_metrics_v1 as v1

    def base(caller):
        return {'GITHUB_REPOSITORY': c.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
                'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_ACTOR_ID': '54093651',
                'GITHUB_ACTOR': 'kani3camp', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
                'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_RUN_ATTEMPT': '1',
                'GITHUB_REF': c.REF, 'GITHUB_WORKFLOW_REF': caller, 'GITHUB_SHA': 'a' * 40,
                'GITHUB_RUN_ID': '1', 'MYPAGE_BACKEND_EXPECTED_SHA': 'a' * 40,
                'MYPAGE_HOSTING_EXPECTED_SHA': 'a' * 40, 'MYPAGE_OPERATOR_EXPECTED_SHA': 'a' * 40}

    def admitted(check, env):
        try:
            check(env)
        except ValueError as error:
            return str(error)
        return 'admitted'

    dev_ref = 'refs/heads/dev'
    result = {}
    for name, check, caller in (('backend', b.context, b.CALLER), ('operator', o.context, o.CALLER),
                                ('hosting', h.context, h.CALLER)):
        env = base(caller)
        result[name] = {
            'main': admitted(check, env),
            'dev_ref': admitted(check, env | {'GITHUB_REF': dev_ref}),
            'dev_caller': admitted(check, env | {'GITHUB_WORKFLOW_REF': caller.replace('@refs/heads/main', '@' + dev_ref)}),
        }
    result['targets'] = {'site': h.SITE, 'host': h.BACKEND_HOST, 'origin': h.ORIGIN, 'run': h.RUN,
                         'registry': b.REGISTRY, 'operator_environment': o.ENVIRONMENT, 'ga4': h.GA4_ID,
                         'backend_account': c.release_account('backend'), 'max': c.MAX_INSTANCES,
                         'run_v1': v1.URL, 'job': o.JOB_NAME, 'image_prefix': image_ops.PREFIX}
    print(json.dumps(result))
"""


class ProductionReleaseTest(unittest.TestCase):
    def test_release_scripts_admit_only_main_and_production_values(self):
        result = run_production(CONTEXT, MYPAGE_HOSTING_BACKEND_HOST=HOST)
        for name in ("backend", "operator", "hosting"):
            with self.subTest(release=name):
                self.assertEqual(result[name]["main"], "admitted")
                self.assertNotEqual(result[name]["dev_ref"], "admitted")
                self.assertNotEqual(result[name]["dev_caller"], "admitted")
        self.assertEqual(result["targets"], {
            "site": SITE, "host": HOST, "origin": "https://moku.work",
            "run": "https://run.googleapis.com/v2/projects/youtube-study-space/locations/asia-northeast2/services/mypage-api",
            "registry": ("https://artifactregistry.googleapis.com/v1/projects/youtube-study-space/locations/"
                         "asia-northeast2/repositories/mypage/packages/mypage-api"),
            "operator_environment": "mypage-prod-operator", "ga4": "G-ZJ1VPVT2L2",
            "backend_account": "mypage-backend-deploy@youtube-study-space.iam.gserviceaccount.com", "max": 3,
            "run_v1": ("https://asia-northeast2-run.googleapis.com/apis/serving.knative.dev/v1/namespaces/"
                       "youtube-study-space/services/mypage-api"),
            "job": "projects/youtube-study-space/locations/asia-northeast2/jobs/mypage-maintenance-operator",
            "image_prefix": "projects/youtube-study-space/locations/asia-northeast2/repositories/mypage"})

    def test_production_host_must_be_supplied_and_well_formed(self):
        for extra in ({}, {"MYPAGE_HOSTING_BACKEND_HOST": "evil.example.com"}):
            with self.subTest(extra=extra):
                result = run_production(CONTEXT, **extra)
                self.assertEqual(result["backend"]["main"], "hosting-backend-host")
                self.assertEqual(result["hosting"]["main"], "hosting-backend-host")
                # The operator release never reads the Service host.
                self.assertEqual(result["operator"]["main"], "admitted")

    def test_production_site_is_the_reviewed_source_value_not_an_input(self):
        result = run_production(CONTEXT, MYPAGE_HOSTING_SITE="another-site", MYPAGE_HOSTING_BACKEND_HOST=HOST)
        self.assertEqual(result["targets"]["site"], SITE)

    def test_production_revisions_are_read_in_the_production_region(self):
        result = run_production("""
            import json
            import foundation_contract as c
            import read_metrics_live as live
            import read_metrics_transition as m

            def full(region, name):
                return "projects/" + c.PROJECT + "/locations/" + region + "/services/mypage-api/revisions/" + name

            revision = full(c.REGION, "mypage-api-00002-abc")
            service = {"name": live.SERVICE_NAME, "terminalCondition": {"state": "CONDITION_SUCCEEDED"},
                       "generation": "2", "observedGeneration": "2", "etag": "etag-1", "uid": "uid-1",
                       "latestReadyRevision": revision, "latestCreatedRevision": revision,
                       "trafficStatuses": [{"type": "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", "percent": 100,
                                            "revision": revision}]}
            result = {"ready": live.ready(service)}
            try:
                m.revision_name(full("asia-southeast2", "mypage-api-00002-abc"))
                result["dev_region"] = "admitted"
            except ValueError as error:
                result["dev_region"] = str(error)
            print(json.dumps(result))
        """)
        self.assertEqual(result, {"ready": "mypage-api-00002-abc", "dev_region": "metrics-transition-revision"})

    def test_production_wif_conditions_pin_main_and_each_units_own_environment(self):
        result = run_production("""
            import json
            import foundation_contract as c
            print(json.dumps({name: [c.release_condition(name), c.release_member(name)] for name in c.RELEASES}))
        """)
        for name, (condition, member) in result.items():
            with self.subTest(release=name):
                self.assertIn('assertion.ref == "refs/heads/main"', condition)
                self.assertIn('assertion.environment == "mypage-prod-' + name + '"', condition)
                self.assertIn('assertion.event_name == "workflow_dispatch"', condition)
                self.assertIn('assertion.run_attempt == "1"', condition)
                self.assertNotIn("integration", condition)
                self.assertNotIn("refs/heads/dev", condition)
                self.assertNotIn("mypage-dev-", condition)
                self.assertNotIn("pull_request", condition)
                self.assertTrue(member.endswith("/subject/340900071:mypage-prod-" + name + ":54093651"))
                self.assertIn("/projects/906336399194/", member)

    def test_production_release_identities_are_exact_and_never_touch_service_iam(self):
        result = run_production("""
            import json
            import runtime_contract as r
            contract = r.release_resource_contract()
            print(json.dumps({'contract': contract,
                              'runtime': sorted(r.resource_contract(runtime=True, public=False))}))
        """)
        contract = result["contract"]
        self.assertEqual(len(contract), 21)
        self.assertTrue(set(contract) <= set(result["runtime"]))
        self.assertFalse(any("cloud_run_v2_service_iam" in address for address in contract))
        self.assertFalse(any("condition" in values for values in contract.values()))
        for address, values in contract.items():
            member = values.get("member")
            if member is None:
                continue
            with self.subTest(address=address):
                component = next(name for name in ("backend", "operator", "hosting") if "." + name + "_" in address)
                if values["role"] == "roles/iam.workloadIdentityUser":
                    self.assertIn(":mypage-prod-" + component + ":", member)
                else:
                    self.assertEqual(member, "serviceAccount:mypage-" + component +
                                     "-deploy@youtube-study-space.iam.gserviceaccount.com")
        roles = {values["role"] for values in contract.values() if "role" in values}
        self.assertNotIn("roles/run.admin", roles)
        self.assertNotIn("roles/iam.serviceAccountTokenCreator", roles)

    def test_dev_bootstrap_scripts_refuse_production(self):
        result = run_production("""
            import json
            import backend_identity as b
            import operator_identity as o
            outcome = {}
            for name, module in (('backend', b), ('operator', o)):
                try:
                    module.condition()
                except ValueError as error:
                    outcome[name] = str(error)
            print(json.dumps(outcome))
        """)
        self.assertEqual(result, {"backend": "backend-bootstrap-target", "operator": "operator-bootstrap-target"})

    def test_receipts_reject_a_record_of_the_other_environment(self):
        result = run_production("""
            import json, os, tempfile
            from pathlib import Path
            import backend_release as b
            import operator_release as o
            import runtime_contract as runtime
            outcome = {}
            with tempfile.TemporaryDirectory() as temp:
                for name, module, record in (
                    ('backend', b, {'schemaVersion': 4, 'sourceSHA': 'a' * 40, 'publishStatus': 'confirmed',
                                    'publicReadback': 'passed', 'imageDigest': runtime.REPOSITORY + '@sha256:' + 'b' * 64}),
                    ('operator', o, {'schemaVersion': 1, 'sourceSHA': 'a' * 40, 'jobStatus': 'confirmed',
                                     'readback': 'passed', 'imageDigest': runtime.OPERATOR_REPOSITORY + '@sha256:' + 'b' * 64}),
                ):
                    for environment in ('production', 'development'):
                        path = Path(temp) / (name + environment)
                        path.write_text(json.dumps(record | {'environment': environment}))
                        try:
                            module.receipt({'RECORD': str(path), 'GITHUB_SHA': 'a' * 40})
                            outcome[name + ':' + environment] = 'accepted'
                        except ValueError as error:
                            outcome[name + ':' + environment] = str(error)
            print(json.dumps(outcome))
        """)
        self.assertEqual(result, {"backend:production": "accepted", "backend:development": "backend-receipt",
                                  "operator:production": "accepted", "operator:development": "operator-receipt"})


if __name__ == "__main__":
    unittest.main()
