"""受入・ネイティブPlan・独立バックエンド権限の観測可能な制御を検証する。"""
from copy import deepcopy
from contextlib import redirect_stderr, redirect_stdout
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'infra/mypage-platform/scripts'))
import foundation_contract as c
import foundation_ops as ops


def plan_fixture():
    changes = []
    for address, values in c.resource_contract().items():
        changes.append({'address': address, 'mode': 'managed', 'provider_name': 'registry.terraform.io/hashicorp/google',
                        'change': {'actions': ['create'], 'after': deepcopy(values), 'after_unknown': {}}})
    return {'format_version': '1.2', 'complete': True, 'errored': False, 'variables': {'stage': {'value': 'foundation'}},
            'resource_changes': changes}


def refreshed_metadata_fixture():
    plan = plan_fixture()
    plan['resource_drift'] = []
    for item in plan['resource_changes']:
        if item['address'].endswith('.google_service_account.runtime'):
            item['change']['actions'] = ['no-op']
        if '.google_secret_manager_secret.' not in item['address']:
            continue
        item['change']['actions'] = ['no-op']
        item['change']['after'].update(annotations={}, labels={}, version_aliases={})
        drift = deepcopy(item)
        drift['change']['actions'] = ['update']
        drift['change']['before'] = deepcopy(drift['change']['after'])
        drift['change']['before'].update(annotations=None, labels=None, version_aliases=None)
        plan['resource_drift'].append(drift)
    return plan


class FoundationTest(unittest.TestCase):
    def test_noop_provider_metadata_does_not_need_an_allowlist(self):
        plan = plan_fixture()
        resource = plan["resource_changes"][0]
        resource["change"]["actions"] = ["no-op"]
        resource["change"]["after"]["future_provider_metadata"] = {"revision": 42}
        observation = deepcopy(resource)
        observation["change"].update(actions=["update"], before={"old_metadata": None})
        plan["resource_drift"] = [observation]
        c.validate_plan(plan)
        for action in (["delete"], ["delete", "create"], ["update"]):
            rejected = deepcopy(plan)
            rejected["resource_changes"][0]["change"]["actions"] = action
            with self.subTest(action=action), self.assertRaises(c.ContractError):
                c.validate_plan(rejected)
        changed = deepcopy(plan)
        changed["resource_drift"][0]["change"]["before"] = {}
        self.assertNotEqual(c.commitment(plan, "v", "a" * 40, "k" * 32),
                            c.commitment(changed, "v", "a" * 40, "k" * 32))

    def test_desired_modes_use_the_reviewed_environment_stage(self):
        stage = (ROOT / 'infra/mypage-platform/environments' / c.CONFIG['state'] / 'desired-stage.txt').read_text().strip()
        self.assertEqual(c.stage_for_mode('desired-plan'), stage)
        self.assertEqual(c.stage_for_mode('desired-apply'), stage)

    def test_maintenance_creates_require_every_fixed_name_to_be_unowned(self):
        project = c.PROJECT
        expected = {
            ops.runtime_contract.MAINTENANCE_OPERATOR_SA: f'https://iam.googleapis.com/v1/projects/{project}/serviceAccounts/mypage-maintenance-job@{project}.iam.gserviceaccount.com',
            ops.runtime_contract.MAINTENANCE_INVOKER_SA: f'https://iam.googleapis.com/v1/projects/{project}/serviceAccounts/mypage-maintenance-invoker@{project}.iam.gserviceaccount.com',
            ops.runtime_contract.MAINTENANCE_ROLE: f'https://iam.googleapis.com/v1/projects/{project}/roles/mypageMaintenanceOperator',
            ops.runtime_contract.MAINTENANCE_POOL: f'https://iam.googleapis.com/v1/projects/{c.PROJECT_NUMBER}/locations/global/workloadIdentityPools/mypage-maintenance',
            ops.runtime_contract.MAINTENANCE_PROVIDER: f'https://iam.googleapis.com/v1/projects/{c.PROJECT_NUMBER}/locations/global/workloadIdentityPools/mypage-maintenance/providers/github',
            ops.runtime_contract.MAINTENANCE_JOB: f'https://run.googleapis.com/v2/projects/{project}/locations/asia-southeast2/jobs/mypage-maintenance-operator',
        }
        plan = {'resource_changes': [
            {'address': address, 'change': {'actions': ['create']}} for address in expected
        ]}

        def absent(url, _token):
            raise ops.urllib.error.HTTPError(url, 404, 'not found', None, None)

        with patch.object(ops, 'request', side_effect=absent) as request:
            ops.reject_unowned_resources({'MYPAGE_GCP_ACCESS_TOKEN': 'private-token'}, plan)
        self.assertEqual({call.args[0] for call in request.call_args_list}, set(expected.values()))

        for address in expected:
            with self.subTest(address=address), patch.object(ops, 'request', return_value={}):
                with self.assertRaises(ValueError):
                    ops.reject_unowned_resources(
                        {'MYPAGE_GCP_ACCESS_TOKEN': 'private-token'},
                        {'resource_changes': [{'address': address, 'change': {'actions': ['create']}}]},
                    )

    def test_new_maintenance_image_role_is_collision_checked_before_apply(self):
        address = ops.runtime_contract.MAINTENANCE_IMAGE_ROLE
        plan = {"resource_changes": [{"address": address, "change": {"actions": ["create"]}}]}
        env = {"MYPAGE_GCP_ACCESS_TOKEN": "private-token"}
        with patch.object(ops, "request", return_value={}) as request:
            with self.assertRaises(ValueError):
                ops.reject_unowned_resources(env, plan)
        self.assertTrue(request.call_args.args[0].endswith("/roles/mypageMaintenanceImageRelease"))

    def test_runtime_settings_do_not_consume_per_build_release_manifest(self):
        fixture = ROOT / 'infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json'
        value = json.loads(fixture.read_text())
        env = {'MYPAGE_RUNTIME_SETTINGS_JSON': json.dumps(value), 'MYPAGE_GCP_WIF_PROVIDER':
               'projects/' + value['project_number'] + '/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions'}
        with tempfile.TemporaryDirectory() as directory, patch.object(ops, 'REPO', Path(directory)):
            file = Path(directory) / 'infra/mypage-platform/environments/dev/runtime-release.json'
            file.parent.mkdir(parents=True)
            before = ops.runtime_settings(env)
            self.assertEqual(before['image_digest'], value['image_digest'])
            self.assertEqual(before['hosting_backend_host'], 'mypage-api-eu5mhkcewq-et.a.run.app')
            self.assertEqual(before['previous_image_digest'], '')
            file.write_text('{"image_digest":"bogus","image_source_sha":"bogus"}')
            self.assertEqual(ops.runtime_settings(env), before)

    def test_retired_maintenance_image_secret_keys_are_ignored_but_other_unknowns_fail(self):
        value = json.loads((ROOT / 'infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json').read_text())
        env = {'MYPAGE_GCP_WIF_PROVIDER':
               'projects/' + value['project_number'] + '/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions'}
        clean = ops.runtime_settings(env | {'MYPAGE_RUNTIME_SETTINGS_JSON': json.dumps(value)})
        legacy = value | {
            'maintenance_operator_enabled': True,
            'maintenance_operator_image_digest': 'private-retired-digest',
            'maintenance_operator_image_source_sha': 'private-retired-source',
        }
        self.assertEqual(ops.runtime_settings(env | {'MYPAGE_RUNTIME_SETTINGS_JSON': json.dumps(legacy)}), clean)
        with self.assertRaisesRegex(c.ContractError, '^runtime-settings-fields$'):
            ops.runtime_settings(env | {'MYPAGE_RUNTIME_SETTINGS_JSON': json.dumps(value | {'unreviewed': 'x'})})
    def test_missing_dev_maintenance_job_resolves_reviewed_bootstrap_tag_without_secret_digest(self):
        env = {'GITHUB_SHA': 'f' * 40, 'MYPAGE_GCP_ACCESS_TOKEN': 'private-token'}
        prefix = ('projects/' + c.PROJECT + '/locations/' + c.REGION +
                  '/repositories/mypage/packages/mypage-api')
        digest = 'd' * 64

        def cloud(url, _token):
            if url.endswith('/jobs/mypage-maintenance-operator'):
                raise ops.urllib.error.HTTPError(url, 404, 'not found', None, None)
            self.assertTrue(url.endswith('/tags/' + ops.MAINTENANCE_BOOTSTRAP_SOURCE_SHA))
            return {'version': prefix + '/versions/sha256:' + digest}

        dockerfile = ('go build -tags timetzdata -trimpath -ldflags="-s -w" '
                      '-o /out/mypage-maintenance-operator ./cmd/mypage-maintenance-operator\n'
                      'COPY --from=build /out/mypage-maintenance-operator /app/mypage-maintenance-operator\n')
        with patch.object(ops, 'request', side_effect=cloud), \
             patch.object(ops, 'run', side_effect=['', dockerfile]) as run:
            self.assertEqual(ops.maintenance_bootstrap_image(env, 'runtime_public'),
                             ops.runtime_contract.REPOSITORY + '@sha256:' + digest)
        self.assertIn('merge-base', run.call_args_list[0].args[0])
        self.assertEqual(run.call_args_list[1].args[0][:2], ['git', 'show'])

        with patch.object(ops, 'request', return_value={'name': 'existing-job'}), \
             patch.object(ops, 'run') as run:
            self.assertEqual(ops.maintenance_bootstrap_image(env, 'runtime_public'), '')
            run.assert_not_called()
    def test_maintenance_operator_stage_writes_admitted_runtime_settings(self):
        value = json.loads((ROOT / 'infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json').read_text())
        env = {
            'GITHUB_REPOSITORY': c.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
            'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_EVENT_NAME': 'workflow_dispatch',
            'GITHUB_ACTOR': 'kani3camp', 'GITHUB_ACTOR_ID': '54093651', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
            'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_REF': c.REF, 'GITHUB_WORKFLOW_REF': c.CALLER,
            'TF_WORKSPACE': 'default', 'GITHUB_SHA': 'a' * 40, 'MYPAGE_EXPECTED_SHA': 'a' * 40,
            'MYPAGE_MODE': 'maintenance-operator-plan', 'MYPAGE_ROLE': 'plan',
            'MYPAGE_STATE_ACCOUNT': '123456789012', 'MYPAGE_STATE_BUCKET': 'synthetic-state-bucket',
            'MYPAGE_BACKEND_ROLE_ARN': 'arn:aws:iam::123456789012:role/youtube-study-space-mypage-dev-plan',
            'MYPAGE_BACKEND_ROLE_ID': 'AROA' + 'A' * 17,
            'MYPAGE_GCP_SERVICE_ACCOUNT': 'mypage-foundation-plan@' + c.PROJECT + '.iam.gserviceaccount.com',
            'MYPAGE_GCP_WIF_PROVIDER': 'projects/' + value['project_number'] + '/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions',
            'MYPAGE_PLAN_HMAC_KEY': 'synthetic-test-key-with-more-than-32-bytes',
            'MYPAGE_RUNTIME_SETTINGS_JSON': json.dumps(value),
        }
        expected = ops.runtime_settings(env, 'maintenance_operator')
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'private'

            def copytree(_source, destination, **_kwargs):
                (destination / 'environments/dev').mkdir(parents=True)

            with patch.object(ops, 'directory', return_value=root), patch.object(ops.shutil, 'copytree', side_effect=copytree):
                ops.stage(env)
            written = json.loads((root / 'runtime-settings.json').read_text())
            self.assertEqual(written, {'runtime_settings': expected})

    def test_native_plan_failures_publish_only_fixed_diagnostic_labels(self):
        private = 'private-command-output-and-credential-marker'
        cases = (
            ("Permission 'run.services.get' denied", 'terraform-plan-permission-run.services.get'),
            ("Permission 'private.unreviewed.permission' denied", 'terraform-plan'),
            ('SERVICE_DISABLED', 'terraform-plan-api-disabled'),
            ('Error acquiring the state lock', 'terraform-plan-state-lock'),
            ('Error: Invalid value for variable', 'terraform-plan-variable-validation'),
            ('Error: Invalid function argument', 'terraform-plan-function-argument'),
            ('Error: Invalid provider configuration', 'terraform-plan-provider-configuration'),
            ('Error: Unsupported argument', 'terraform-plan-unsupported-argument'),
            ('unclassified native error', 'terraform-plan'),
        )
        for native, expected in cases:
            with self.subTest(native=native):
                result = ops.subprocess.CompletedProcess(['terraform', private], 1, private, native + '\n' + private)
                output = io.StringIO()
                with patch.object(sys, 'argv', ['foundation_ops.py', 'execute']), \
                        patch.object(ops.subprocess, 'run', return_value=result), \
                        patch.object(ops, 'prepare', side_effect=lambda env: ops.run(['terraform', private], check='terraform-plan')), \
                        redirect_stderr(output):
                    self.assertEqual(ops.main(), 1)
                self.assertIn('check=' + expected + '.', output.getvalue())
                self.assertNotIn(private, output.getvalue())
                self.assertNotIn('private.unreviewed.permission', output.getvalue())
        success = ops.subprocess.CompletedProcess(['terraform'], 0, private, '')
        with patch.object(ops.subprocess, 'run', return_value=success):
            self.assertEqual(ops.run(['terraform'], check='terraform-plan'), private)

    def test_runtime_inputs_match_exact_source_digest_and_enabled_metadata_only(self):
        value = json.loads((ROOT / 'infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json').read_text())
        env = {'GITHUB_SHA': 'a' * 40, 'MYPAGE_GCP_ACCESS_TOKEN': 'private-token-marker'}
        prefix = 'projects/' + c.PROJECT + '/locations/asia-southeast2/repositories/mypage/packages/mypage-api'
        tag = {'version': prefix + '/versions/sha256:' + value['image_digest'].split('@sha256:')[1]}
        with patch.object(ops, 'request', side_effect=[tag, *[{'state': 'ENABLED'} for _ in range(3)]]) as request:
            ops.verify_runtime_inputs(env, value, 'runtime_private')
        urls = [call.args[0] for call in request.call_args_list]
        self.assertEqual(urls[0], 'https://artifactregistry.googleapis.com/v1/' + prefix + '/tags/' + env['GITHUB_SHA'])
        self.assertEqual(urls[1:], [
            'https://secretmanager.googleapis.com/v1/projects/' + c.PROJECT + '/secrets/' + secret + '/versions/' + value[field]
            for secret, field in [('mypage-oauth-client-id', 'oauth_client_id_version'),
                                  ('mypage-oauth-client-secret', 'oauth_client_secret_version'),
                                  ('mypage-youtube-api-key', 'youtube_api_key_version')]])
        self.assertTrue(all(':access' not in url for url in urls))
        for responses, expected in (
            ([{'version': 'private-rejected-image-marker'}], 'runtime-source-image-digest'),
            ([tag, {'state': 'private-rejected-state-marker'}], 'runtime-secret-version-enabled'),
        ):
            output = io.StringIO()
            with patch.object(sys, 'argv', ['foundation_ops.py', 'execute']), \
                    patch.object(ops, 'request', side_effect=responses), \
                    patch.object(ops, 'prepare', side_effect=lambda ignored: ops.verify_runtime_inputs(env, value, 'runtime_private')), \
                    redirect_stderr(output):
                self.assertEqual(ops.main(), 1)
            self.assertIn('check=' + expected + '.', output.getvalue())
            self.assertNotIn('private-', output.getvalue())

    def test_terraform_bootstrap_image_source_may_be_an_ancestor(self):
        value = json.loads((ROOT / 'infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json').read_text())
        value['image_source_sha'] = 'b' * 40
        env = {'GITHUB_SHA': 'a' * 40, 'MYPAGE_GCP_ACCESS_TOKEN': 'private-token-marker'}
        prefix = 'projects/' + c.PROJECT + '/locations/asia-southeast2/repositories/mypage/packages/mypage-api'
        tag = {'version': prefix + '/versions/sha256:' + value['image_digest'].split('@sha256:')[1]}
        with patch.object(ops, 'run', side_effect=['', '']) as run, \
                patch.object(ops, 'request', side_effect=[tag, *[{'state': 'ENABLED'} for _ in range(3)]]) as request:
            ops.verify_runtime_inputs(env, value, 'runtime_private')
        self.assertEqual(run.call_args_list[0].args[0], ['git', 'fetch', '--no-tags', '--depth=1', 'origin', 'b' * 40])
        self.assertTrue(request.call_args_list[0].args[0].endswith('/tags/' + 'b' * 40))
        self.assertEqual(run.call_args_list[1].args[0],
                         ['git', 'merge-base', '--is-ancestor', 'b' * 40, env['GITHUB_SHA']])
        with patch.object(ops, 'run', side_effect=['', ValueError('unreviewed-ancestor')]), \
             patch.object(ops, 'request') as request:
            with self.assertRaises(ValueError):
                ops.verify_runtime_inputs(env, value, 'runtime_private')
            request.assert_not_called()
        for source in ['--upload-pack=private', 'b' * 39, 'B' * 40]:
            with self.subTest(source=source), patch.object(ops, 'run') as run:
                with self.assertRaisesRegex(c.ContractError, '^runtime-image-source-sha$'):
                    ops.verify_runtime_inputs(env, value | {'image_source_sha': source}, 'runtime_private')
                run.assert_not_called()

    def test_maintenance_operator_uses_the_shared_bootstrap_image_contract(self):
        value = json.loads((ROOT / 'infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json').read_text())
        operator = ops.runtime_contract.settings(value, 'maintenance_operator')
        self.assertEqual(operator['image_digest'], value['image_digest'])
        self.assertEqual(operator['image_source_sha'], '')
        self.assertNotIn('maintenance_operator_image_digest', operator)
        self.assertNotIn('maintenance_operator_image_source_sha', operator)

    def test_public_invoker_preflight_requires_the_exact_unconditional_owner_binding(self):
        env = {'MYPAGE_GCP_ACCESS_TOKEN': 'private-token-marker'}
        valid = {'bindings': [{'role': 'roles/run.invoker', 'members': ['allUsers']}]}
        with patch.object(ops, 'request', return_value=valid) as request:
            ops.verify_owner_public_invoker(env)
        self.assertTrue(request.call_args.args[0].endswith('/services/mypage-api:getIamPolicy?options.requestedPolicyVersion=3'))
        for policy in ({}, {'bindings': [{'role': 'roles/run.admin', 'members': ['allUsers']}]},
                       {'bindings': [{'role': 'roles/run.invoker', 'members': ['allUsers'], 'condition': {'expression': 'true'}}]},
                       {'bindings': valid['bindings'] + [{'role': 'roles/run.admin', 'members': ['allAuthenticatedUsers']}]}):
            with self.subTest(policy=policy), patch.object(ops, 'request', return_value=policy), self.assertRaisesRegex(c.ContractError, '^runtime-owner-public-invoker$'):
                ops.verify_owner_public_invoker(env)

    def test_registry_stage_adds_only_the_immutable_dev_repository(self):
        plan = plan_fixture()
        plan['variables']['stage']['value'] = 'registry'
        for item in plan['resource_changes']:
            item['change']['actions'] = ['no-op']
        address = 'module.platform[0].google_artifact_registry_repository.mypage[0]'
        plan['resource_changes'].append({
            'address': address, 'mode': 'managed', 'provider_name': 'registry.terraform.io/hashicorp/google',
            'change': {'actions': ['create'], 'after': deepcopy(c.resource_contract('registry')[address]), 'after_unknown': {}},
        })
        self.assertEqual(c.stage_for_mode('registry-plan'), 'registry')
        self.assertEqual(c.stage_for_mode('registry-apply'), 'registry')
        self.assertEqual(c.validate_plan(plan, 'registry'), {'resources': 9, 'creates': 1})
        for key, bad in (
            ('project', 'youtube-study-space'), ('location', 'asia-northeast2'), ('repository_id', 'gcf-artifacts'),
            ('format', 'NPM'), ('mode', 'REMOTE_REPOSITORY'), ('cleanup_policy_dry_run', False),
            ('docker_config', [{'immutable_tags': False}]), ('remote_repository_config', [{'description': 'unreviewed'}]),
            ('virtual_repository_config', [{'upstream_policies': []}]), ('cleanup_policies', [{'id': 'delete'}]),
        ):
            rejected = deepcopy(plan)
            rejected['resource_changes'][-1]['change']['after'][key] = bad
            with self.subTest(field=key), self.assertRaises(ValueError):
                c.validate_plan(rejected, 'registry')
        for actions in (['update'], ['delete'], ['delete', 'create']):
            rejected = deepcopy(plan)
            rejected['resource_changes'][-1]['change']['actions'] = actions
            with self.subTest(actions=actions), self.assertRaises(ValueError):
                c.validate_plan(rejected, 'registry')
        # 明示的に有効にする段階から、旧Foundationモードへリポジトリを混入させない。
        plan['variables']['stage']['value'] = 'foundation'
        with self.assertRaises(ValueError):
            c.validate_plan(plan)

    def test_post_apply_noop_accepts_only_the_same_secret_resource_name(self):
        plan = plan_fixture()
        for item in plan['resource_changes']:
            item['change']['actions'] = ['no-op']
            if '.google_secret_manager_secret_iam_member.' in item['address']:
                short = item['change']['after']['secret_id']
                item['change']['after']['secret_id'] = 'projects/' + c.PROJECT + '/secrets/' + short
        original = deepcopy(plan)
        self.assertEqual(c.validate_plan(plan), {'resources': 8, 'creates': 0})
        self.assertEqual(plan, original)
        iam = next(i for i, item in enumerate(plan['resource_changes'])
                   if '.google_secret_manager_secret_iam_member.' in item['address'])
        for rejected in (
            'projects/youtube-study-space/secrets/mypage-oauth-client-id',
            'projects/' + c.PROJECT + '/secrets/other-secret',
            'projects/' + c.PROJECT + '/secrets/mypage-oauth-client-id/versions/1',
            'untrusted/prefix/mypage-oauth-client-id',
            'projects/123456789/secrets/mypage-oauth-client-id',
        ):
            changed = deepcopy(plan)
            changed['resource_changes'][iam]['change']['after']['secret_id'] = rejected
            with self.subTest(rejected=rejected), self.assertRaises(ValueError):
                c.validate_plan(changed)
        # Secretメタデータ資源の短い名前の仕様を緩めない。
        metadata = next(item for item in plan['resource_changes'] if '.google_secret_manager_secret.' in item['address'])
        metadata['change']['after']['secret_id'] = 'projects/' + c.PROJECT + '/secrets/mypage-oauth-client-id'
        with self.assertRaises(ValueError):
            c.validate_plan(plan)

    def test_rejected_plan_values_never_enter_public_diagnostics(self):
        plan = plan_fixture()
        private = 'private-project-and-secret-payload-marker'
        plan['resource_changes'][0]['change']['after']['project'] = private
        output = io.StringIO()
        with patch.object(sys, 'argv', ['foundation_ops.py', 'execute']), \
                patch.object(ops, 'prepare', side_effect=lambda env: c.validate_plan(plan)), \
                redirect_stderr(output):
            self.assertEqual(ops.main(), 1)
        self.assertIn('check=resource-value-project', output.getvalue())
        self.assertNotIn(private, output.getvalue())

    def test_only_eight_expected_private_resources_are_admitted(self):
        self.assertEqual(c.validate_plan(plan_fixture()), {'resources': 8, 'creates': 8})
        changes = plan_fixture()['resource_changes']
        changes[0]['change']['actions'] = ['no-op']
        self.assertEqual(c.validate_plan(plan_fixture() | {'resource_changes': changes})['creates'], 7)

    def test_rejects_deletion_public_access_foreign_principals_secret_versions_and_unknowns(self):
        mutations = [
            lambda p: p['resource_changes'][0]['change'].update(actions=['delete', 'create']),
            lambda p: p['resource_changes'][0]['change'].update(actions=['update']),
            lambda p: p['resource_changes'].append({'address': 'google_cloud_run_v2_service_iam_member.public_invoker'}),
            lambda p: p['resource_changes'].append({'address': 'google_secret_manager_secret_version.raw_value'}),
            lambda p: p['resource_changes'][0]['change']['after'].update(project='youtube-study-space'),
            lambda p: p['resource_changes'][-1]['change']['after'].update(member='allUsers'),
            lambda p: p['resource_changes'][-1]['change']['after'].update(condition=[{'expression': 'true'}]),
            lambda p: p['resource_changes'][0]['change']['after_unknown'].update(account_id=True),
            lambda p: p['resource_changes'].pop(),
            lambda p: p.update(complete=False),
            lambda p: p.update(resource_drift=[{'address': 'unreviewed'}]),
            lambda p: p['variables']['stage'].update(value='runtime_public'),
            lambda p: p['resource_changes'][1]['change']['after']['replication'][0].update(user_managed=[{}]),
        ]
        for mutate in mutations:
            plan = plan_fixture()
            mutate(plan)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                c.validate_plan(plan)

    def test_private_plan_commitment_binds_changes_source_and_state(self):
        plan = plan_fixture()
        key = 'private-key-for-synthetic-tests-only'
        digest = c.commitment(plan, 'absent', 'a' * 40, key)
        self.assertEqual(digest, c.commitment(plan | {'timestamp': 'later'}, 'absent', 'a' * 40, key))
        for changed, version, sha in ((plan | {'applyable': False}, 'absent', 'a' * 40), (plan, 'new-version', 'a' * 40), (plan, 'absent', 'b' * 40), (plan | {'resource_drift': [{'address': 'changed'}]}, 'absent', 'a' * 40)):
            self.assertNotEqual(digest, c.commitment(changed, version, sha, key))

    def test_metadata_first_stage_cannot_grant_iam(self):
        plan = plan_fixture()
        plan['variables']['stage']['value'] = 'foundation_metadata'
        plan['resource_changes'] = [x for x in plan['resource_changes'] if '_iam_member.' not in x['address']]
        self.assertEqual(c.validate_plan(plan, 'foundation_metadata'), {'resources': 4, 'creates': 4})
        plan['resource_changes'].append(plan_fixture()['resource_changes'][-1])
        with self.assertRaises(ValueError):
            c.validate_plan(plan, 'foundation_metadata')

    def test_provider_actions_cannot_bypass_resource_boundary(self):
        plan = plan_fixture() | {'action_invocations': [{'address': 'action.google_arbitrary.call'}]}
        with self.assertRaises(ValueError):
            c.validate_plan(plan)

    def test_nested_false_unknown_masks_are_known_but_nested_true_is_rejected(self):
        plan = plan_fixture()
        secret = next(x for x in plan['resource_changes'] if '.google_secret_manager_secret.' in x['address'])
        secret['change']['after_unknown']['replication'] = [{'auto': [{'customer_managed_encryption': []}]}]
        c.validate_plan(plan)
        secret['change']['after_unknown']['replication'][0]['auto'][0]['customer_managed_encryption'] = True
        with self.assertRaises(ValueError):
            c.validate_plan(plan)

    def test_every_credentialed_operation_is_a_manual_dispatch_from_its_own_workflow(self):
        env = {
            'GITHUB_REPOSITORY': c.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
            'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_EVENT_NAME': 'workflow_dispatch',
            'GITHUB_ACTOR': 'kani3camp', 'GITHUB_ACTOR_ID': '54093651',
            'GITHUB_TRIGGERING_ACTOR': 'kani3camp', 'GITHUB_RUN_ATTEMPT': '1',
            'GITHUB_REF': c.REF, 'TF_WORKSPACE': 'default', 'GITHUB_SHA': 'a' * 40,
            'MYPAGE_EXPECTED_SHA': 'a' * 40, 'MYPAGE_MODE': 'image-build',
        }
        for component, item in c.IMAGE_COMPONENTS.items():
            image = env | {'MYPAGE_IMAGE_COMPONENT': component, 'GITHUB_WORKFLOW_REF': item['caller']}
            with self.subTest(component=component):
                c.context(image, 'apply', image=True)
                # No push or other event can build or deploy an application image.
                for event in ('push', 'pull_request', 'workflow_run'):
                    with self.assertRaises(ValueError):
                        c.context(image | {'GITHUB_EVENT_NAME': event}, 'apply', image=True)
                # Each image is built only by the Deploy workflow of its own use case.
                for other in [c.CALLER] + [x['caller'] for x in c.IMAGE_COMPONENTS.values() if x is not item]:
                    with self.assertRaises(ValueError):
                        c.context(image | {'GITHUB_WORKFLOW_REF': other}, 'apply', image=True)
        for bad in ('', 'mypage-api', 'all'):
            with self.subTest(component=bad), self.assertRaises(ValueError):
                c.context(env | {'MYPAGE_IMAGE_COMPONENT': bad, 'GITHUB_WORKFLOW_REF': c.CALLER}, 'apply', image=True)
        with self.assertRaises(ValueError):
            c.subject('apply', 'push')
        self.assertEqual(len(c.trust_policy('123456789012', 'apply')['Statement']), 1)
        self.assertEqual(c.trust_policy('123456789012', 'apply')['Statement'][0]['Condition']['StringEquals']['token.actions.githubusercontent.com:sub'],
                         c.subject('apply'))

    def test_provider_condition_pairs_each_environment_caller_and_exact_subject(self):
        condition = c.provider_condition()
        self.assertLessEqual(len(condition), c.CONDITION_LIMIT)
        prefix, listed = condition.split(' && assertion.sub in ')
        expected = [('plan', c.CALLER), ('apply', c.CALLER)] + [
            ('apply', item['caller']) for _, item in sorted(c.IMAGE_COMPONENTS.items())]
        # The clause list is exact, so image callers never reach the Plan Environment.
        self.assertEqual(json.loads(listed), [c.subject(kind, 'workflow_dispatch', workflow)
                                              for kind, workflow in expected])
        for clause in ('assertion.repository_id == "340900071"', 'assertion.actor_id == "54093651"',
                       'assertion.ref == ' + json.dumps(c.REF),
                       'assertion.job_workflow_ref == ' + json.dumps(c.REUSABLE),
                       'assertion.event_name == "workflow_dispatch"', 'assertion.run_attempt == "1"',
                       'assertion.workflow_sha == assertion.job_workflow_sha'):
            self.assertIn(clause, prefix)
        self.assertNotIn('||', condition)
        self.assertNotIn('push', condition)

    def test_exact_actor_ref_and_sha_before_credentials(self):
        env = {'GITHUB_REPOSITORY': c.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
               'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_EVENT_NAME': 'workflow_dispatch',
               'GITHUB_ACTOR': 'kani3camp', 'GITHUB_ACTOR_ID': '54093651', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
               'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_REF': c.REF, 'GITHUB_WORKFLOW_REF': c.CALLER,
               'TF_WORKSPACE': 'default', 'GITHUB_SHA': 'a' * 40, 'MYPAGE_EXPECTED_SHA': 'a' * 40,
               'MYPAGE_MODE': 'foundation-plan'}
        c.context(env, 'plan')
        for key, bad in (('GITHUB_EVENT_NAME', 'pull_request'), ('GITHUB_REF', 'refs/heads/main'),
                         ('GITHUB_ACTOR_ID', '1234'), ('GITHUB_RUN_ATTEMPT', '2'), ('MYPAGE_EXPECTED_SHA', 'b' * 40)):
            with self.subTest(key=key), self.assertRaises(ValueError):
                c.context(env | {key: bad}, 'plan')
        with self.assertRaises(ValueError):
            c.context(env, 'apply')

    def test_backend_actions_cannot_cross_state_or_delete_state(self):
        for kind in ('plan', 'apply'):
            policy = c.backend_policy('synthetic-state-bucket', kind)
            state, lock, listing = policy['Statement']
            self.assertEqual(state['Resource'], 'arn:aws:s3:::synthetic-state-bucket/' + c.STATE_KEY)
            self.assertNotIn('s3:DeleteObject', state['Action'])
            self.assertEqual('s3:PutObject' in state['Action'], kind == 'apply')
            self.assertEqual(lock['Resource'], state['Resource'] + '.tflock')
            self.assertFalse(any('*' in value for value in listing['Condition']['StringEquals']['s3:prefix']))
            conditions = c.trust_policy('123456789012', kind)['Statement'][0]['Condition']['StringEquals']
            self.assertEqual(conditions['token.actions.githubusercontent.com:actor_id'], '54093651')
            self.assertIn(c.REF, conditions['token.actions.githubusercontent.com:sub'])
            self.assertNotIn('*', json.dumps(conditions))

    def test_state_absence_requires_a_successful_exact_prefix_listing(self):
        env = {'MYPAGE_STATE_BUCKET': 'synthetic-state-bucket', 'MYPAGE_STATE_ACCOUNT': '123456789012'}
        listing = {'IsTruncated': False, 'Prefix': c.STATE_KEY, 'Contents': []}
        with patch.object(ops, 'run', return_value=json.dumps(listing)) as command:
            self.assertEqual(ops.state_version(env), 'absent')
            self.assertEqual(command.call_count, 1)
            self.assertIn('list-objects-v2', command.call_args.args[0])
        with patch.object(ops, 'run', side_effect=ValueError('AccessDenied')):
            with self.assertRaises(ValueError):
                ops.state_version(env)



class RuntimeDriftDiagnosticTest(unittest.TestCase):
    def test_drift_detail_shows_only_fixed_addresses_and_action_names(self):
        secret = 'synthetic-private-value'
        service = 'module.platform[0].google_cloud_run_v2_service.api[0]'
        plan = {
            'resource_drift': [
                {'address': service, 'change': {'actions': ['update'], 'before': {'image': secret}, 'after': {'image': secret}}},
                {'address': 'module.other["' + secret + '"].x.y', 'change': {'actions': ['update']}},
            ],
            'resource_changes': [
                {'address': service, 'change': {'actions': ['update'], 'before': {'env': secret + '-old'}, 'after': {'env': secret}}},
                {'address': 'module.platform[0].google_cloud_run_v2_job.maintenance_operator[0]',
                 'change': {'actions': ['create'], 'after': {'image': secret}}},
                {'address': 'module.platform[0].google_service_account.runtime', 'change': {'actions': ['no-op']}},
            ],
        }
        output = io.StringIO()
        with redirect_stdout(output):
            ops.describe_runtime_drift(plan)
        text = output.getvalue()
        self.assertNotIn(secret, text)
        lines = dict(line.split(': ', 1) for line in text.strip().splitlines())
        detail = json.loads(lines['Terraform drift detail'].removesuffix('.'))
        self.assertEqual(detail, [
            {'address': service, 'actions': ['update'], 'plannedActions': ['update']},
            {'address': '<withheld>', 'actions': ['update'], 'plannedActions': ['<withheld>']},
        ])
        writes = json.loads(lines['Terraform planned writes'].removesuffix('.'))
        self.assertEqual([entry['address'] for entry in writes],
                         ['module.platform[0].google_cloud_run_v2_job.maintenance_operator[0]', service])
        self.assertEqual(json.loads(lines['Terraform drift summary'].removesuffix('.')),
                         {'observedResources': 2, 'plannedWrites': 2})
        updates = json.loads(lines['Terraform planned update attributes'].removesuffix('.'))
        self.assertEqual(updates, [{'address': service, 'actions': ['update'], 'attributes': ['env'], 'enumValues': {}}])

    def test_enum_values_are_shown_only_for_fixed_provider_values(self):
        self.assertEqual(ops.public_enum_values({'before': {'deletion_policy': 'DELETE'}, 'after': {'deletion_policy': 'PREVENT'}}),
                         {'deletion_policy': {'before': 'DELETE', 'after': 'PREVENT'}})
        self.assertEqual(ops.public_enum_values({'before': {}, 'after': {'deletion_policy': 'PREVENT'}}),
                         {'deletion_policy': {'before': None, 'after': 'PREVENT'}})
        self.assertEqual(ops.public_enum_values({'before': {'deletion_policy': 'synthetic-secret'}, 'after': {'deletion_policy': 'PREVENT'}}),
                         {'deletion_policy': {'before': '<withheld>', 'after': 'PREVENT'}})
        self.assertEqual(ops.public_enum_values({'before': {'env': 'a'}, 'after': {'env': 'b'}}), {})

    def test_update_attributes_are_schema_names_only(self):
        change = {'before': {'deletion_policy': None, 'launch_stage': 'GA', 'template': [{'env': 'old-secret'}],
                             'labels': {'private-key': 'x'}},
                  'after': {'deletion_policy': 'PREVENT', 'launch_stage': 'GA', 'template': [{'env': 'new-secret'}],
                            'labels': {'private-key': 'x'}, 'Weird Key': 1},
                  'after_unknown': {'uri': True, 'etag': False}}
        names = ops.changed_attributes(change)
        self.assertEqual(names, ['<withheld>', 'deletion_policy', 'template', 'uri'])
        self.assertNotIn('secret', json.dumps(names))
        self.assertEqual(ops.changed_attributes({'before': None, 'after': {}}), ['<withheld>'])

    def test_known_nested_structures_in_after_unknown_are_not_changes(self):
        same = {'template': [{'containers': [{'image': 'x'}]}], 'launch_stage': 'GA'}
        change = {'before': same | {'launch_stage': 'BETA'}, 'after': same,
                  'after_unknown': {'template': [{}], 'scaling': {'containers': [{}]}, 'labels': [False],
                                    'traffic': [{'percent': False, 'revision': True}]}}
        self.assertEqual(ops.changed_attributes(change), ['launch_stage', 'traffic'])


if __name__ == "__main__":
    unittest.main()
