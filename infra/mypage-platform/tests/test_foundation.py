"""Observable admission, native Plan and isolated backend permission boundaries."""
from copy import deepcopy
from contextlib import redirect_stderr, redirect_stdout
import importlib.util
import io
import json
from pathlib import Path
import re
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

    def test_public_release_manifest_overrides_only_reviewed_release_fields(self):
        fixture = ROOT / 'infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json'
        value = json.loads(fixture.read_text())
        env = {'MYPAGE_RUNTIME_SETTINGS_JSON': json.dumps(value), 'MYPAGE_GCP_WIF_PROVIDER':
               'projects/' + value['project_number'] + '/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions'}
        release = {'image_digest': ops.runtime_contract.REPOSITORY + '@sha256:' + 'b' * 64,
                   'image_source_sha': 'c' * 40, 'hosting_backend_host': 'mypage-api-observed-et.a.run.app',
                   'previous_image_digest': value['image_digest']}
        with tempfile.TemporaryDirectory() as directory, patch.object(ops, 'REPO', Path(directory)):
            file = Path(directory) / 'infra/mypage-platform/environments/dev/runtime-release.json'
            file.parent.mkdir(parents=True)
            self.assertEqual(ops.runtime_settings(env)['image_digest'], value['image_digest'])
            file.write_text(json.dumps(release))
            actual = ops.runtime_settings(env)
            self.assertEqual({k: actual[k] for k in release}, release)
            self.assertEqual(actual['privacy_version'], value['privacy_version'])
            self.assertEqual(actual['maintenance_operator_image_digest'], value['maintenance_operator_image_digest'])
            self.assertEqual(actual['maintenance_operator_image_source_sha'], value['maintenance_operator_image_source_sha'])
            for invalid in (release | {'privacy_version': 'other'}, release | {'image_source_sha': ''},
                            release | {'hosting_backend_host': 'other.example.test'},
                            release | {'image_digest': release['previous_image_digest']}):
                file.write_text(json.dumps(invalid))
                with self.assertRaises(ValueError):
                    ops.runtime_settings(env)
            file.write_text(json.dumps(release)[:-1] + ',"image_digest":"duplicate"}')
            with self.assertRaises(ValueError):
                ops.runtime_settings(env)

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
        value['maintenance_operator_image_source_sha'] = env['GITHUB_SHA']
        prefix = 'projects/' + c.PROJECT + '/locations/asia-southeast2/repositories/mypage/packages/mypage-api'
        tag = {'version': prefix + '/versions/sha256:' + value['image_digest'].split('@sha256:')[1]}
        operator_tag = {'version': prefix + '/versions/sha256:' + value['maintenance_operator_image_digest'].split('@sha256:')[1]}
        with patch.object(ops, 'request', side_effect=[tag, operator_tag, *[{'state': 'ENABLED'} for _ in range(3)]]) as request:
            ops.verify_runtime_inputs(env, value, 'runtime_private')
        urls = [call.args[0] for call in request.call_args_list]
        self.assertEqual(urls[0], 'https://artifactregistry.googleapis.com/v1/' + prefix + '/tags/' + env['GITHUB_SHA'])
        self.assertEqual(urls[1], 'https://artifactregistry.googleapis.com/v1/' + prefix + '/tags/' + env['GITHUB_SHA'])
        self.assertEqual(urls[2:], [
            'https://secretmanager.googleapis.com/v1/projects/' + c.PROJECT + '/secrets/' + secret + '/versions/' + value[field]
            for secret, field in [('mypage-oauth-client-id', 'oauth_client_id_version'),
                                  ('mypage-oauth-client-secret', 'oauth_client_secret_version'),
                                  ('mypage-youtube-api-key', 'youtube_api_key_version')]])
        self.assertTrue(all(':access' not in url for url in urls))
        for responses, expected in (
            ([{'version': 'private-rejected-image-marker'}], 'runtime-source-image-digest'),
            ([tag, operator_tag, {'state': 'private-rejected-state-marker'}], 'runtime-secret-version-enabled'),
        ):
            output = io.StringIO()
            with patch.object(sys, 'argv', ['foundation_ops.py', 'execute']), \
                    patch.object(ops, 'request', side_effect=responses), \
                    patch.object(ops, 'prepare', side_effect=lambda ignored: ops.verify_runtime_inputs(env, value, 'runtime_private')), \
                    redirect_stderr(output):
                self.assertEqual(ops.main(), 1)
            self.assertIn('check=' + expected + '.', output.getvalue())
            self.assertNotIn('private-', output.getvalue())

    def test_retained_image_requires_identical_entire_build_context(self):
        value = json.loads((ROOT / 'infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json').read_text())
        value['image_source_sha'] = 'b' * 40
        env = {'GITHUB_SHA': 'a' * 40, 'MYPAGE_GCP_ACCESS_TOKEN': 'private-token-marker'}
        value['maintenance_operator_image_source_sha'] = env['GITHUB_SHA']
        prefix = 'projects/' + c.PROJECT + '/locations/asia-southeast2/repositories/mypage/packages/mypage-api'
        tag = {'version': prefix + '/versions/sha256:' + value['image_digest'].split('@sha256:')[1]}
        operator_tag = {'version': prefix + '/versions/sha256:' + value['maintenance_operator_image_digest'].split('@sha256:')[1]}
        with patch.object(ops, 'run', side_effect=['', 'c' * 40, 'c' * 40]) as run, \
                patch.object(ops, 'request', side_effect=[tag, operator_tag, *[{'state': 'ENABLED'} for _ in range(3)]]) as request:
            ops.verify_runtime_inputs(env, value, 'runtime_private')
        self.assertEqual(run.call_args_list[0].args[0], ['git', 'fetch', '--no-tags', '--depth=1', 'origin', 'b' * 40])
        self.assertTrue(request.call_args_list[0].args[0].endswith('/tags/' + 'b' * 40))
        with patch.object(ops, 'run', side_effect=['', 'c' * 40, 'd' * 40]), patch.object(ops, 'request') as request:
            with self.assertRaisesRegex(c.ContractError, '^runtime-image-source-tree$'):
                ops.verify_runtime_inputs(env, value, 'runtime_private')
            request.assert_not_called()
        for source in ['--upload-pack=private', 'b' * 39, 'B' * 40]:
            with self.subTest(source=source), patch.object(ops, 'run') as run:
                with self.assertRaisesRegex(c.ContractError, '^runtime-image-source-sha$'):
                    ops.verify_runtime_inputs(env, value | {'image_source_sha': source}, 'runtime_private')
                run.assert_not_called()

    def test_operator_image_source_must_be_ancestor_and_resolve_to_its_own_digest(self):
        value = json.loads((ROOT / 'infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json').read_text())
        env = {'GITHUB_SHA': 'a' * 40, 'MYPAGE_GCP_ACCESS_TOKEN': 'private-token-marker'}
        prefix = 'projects/' + c.PROJECT + '/locations/asia-southeast2/repositories/mypage/packages/mypage-api'
        api_tag = {'version': prefix + '/versions/sha256:' + value['image_digest'].split('@sha256:')[1]}
        operator_tag = {'version': prefix + '/versions/sha256:' + value['maintenance_operator_image_digest'].split('@sha256:')[1]}
        with patch.object(ops, 'run', side_effect=['', '']) as run, patch.object(
                ops, 'request', side_effect=[api_tag, operator_tag, *[{'state': 'ENABLED'} for _ in range(3)]]) as request:
            ops.verify_runtime_inputs(env, value, 'runtime_private')
        self.assertEqual(run.call_args_list[0].args[0], ['git', 'fetch', '--no-tags', '--depth=1', 'origin', 'c' * 40])
        self.assertEqual(run.call_args_list[1].args[0], ['git', 'merge-base', '--is-ancestor', 'c' * 40, 'a' * 40])
        self.assertTrue(request.call_args_list[1].args[0].endswith('/tags/' + 'c' * 40))
        with patch.object(ops, 'run', side_effect=['', '']), patch.object(
                ops, 'request', side_effect=[api_tag, {'version': 'private-wrong-digest'}]):
            with self.assertRaisesRegex(c.ContractError, '^maintenance-operator-source-image-digest$'):
                ops.verify_runtime_inputs(env, value, 'runtime_private')

    def test_runtime_drift_diagnostic_does_not_publish_values_or_unreviewed_field_names(self):
        plan = {'resource_changes': [{'address': ops.runtime_contract.SERVICE, 'change': {'actions': ['no-op']}}],
                'resource_drift': [{'address': ops.runtime_contract.SERVICE, 'change': {
                    'before': {'etag': 'private-before', 'private-field-name': 'private-before'},
                    'after': {'etag': 'private-after', 'private-field-name': 'private-after', 'template': [{'annotations': {}, 'labels': {}}]}, 'actions': ['update']}}]}
        original = deepcopy(plan)
        output = io.StringIO()
        with redirect_stdout(output):
            ops.describe_runtime_drift(plan)
        self.assertIn('"fields": ["etag", "template"]', output.getvalue())
        self.assertIn('"after": "array"', output.getvalue())
        self.assertIn('"otherFields": 1', output.getvalue())
        self.assertIn('"plannedNoop": true', output.getvalue())
        self.assertNotIn('private-', output.getvalue())
        self.assertEqual(plan, original)

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

    def test_registry_artifact_push_refresh_preserves_configuration_and_time_order(self):
        plan = refreshed_metadata_fixture()
        plan['variables']['stage']['value'] = 'registry'
        for item in plan['resource_changes']:
            item['change']['actions'] = ['no-op']
        address = 'module.platform[0].google_artifact_registry_repository.mypage[0]'
        after = deepcopy(c.resource_contract('registry')[address]) | {
            'labels': {}, 'update_time': '2000-01-01T01:00:00.123456789Z'}
        item = {'address': address, 'mode': 'managed', 'provider_name': 'registry.terraform.io/hashicorp/google',
                'change': {'actions': ['no-op'], 'after': after, 'after_unknown': {}}}
        plan['resource_changes'].append(item)
        drift = deepcopy(item)
        drift['change'].update(actions=['update'], before=deepcopy(after) | {
            'labels': None, 'update_time': '2000-01-01T00:00:00Z'})
        plan['resource_drift'].append(drift)
        original = deepcopy(plan)
        self.assertEqual(c.validate_plan(plan, 'registry'), {'resources': 9, 'creates': 0})
        self.assertEqual(plan, original)
        timestamp_only = deepcopy(plan)
        timestamp_only['resource_drift'][-1]['change']['before']['labels'] = {}
        self.assertEqual(c.validate_plan(timestamp_only, 'registry'), {'resources': 9, 'creates': 0})
        for bad in ('1999-12-31T23:59:59Z', '2000-13-01T01:00:00Z', '2000-01-01T01:00:00+00:00',
                    '2000-01-01T01:00:00.1234567890Z', None, 'private-invalid-value'):
            rejected = deepcopy(plan)
            rejected['resource_drift'][-1]['change']['after']['update_time'] = bad
            with self.subTest(timestamp=bad), self.assertRaises(ValueError):
                c.validate_plan(rejected, 'registry')
        nanos = deepcopy(plan)
        nanos['resource_drift'][-1]['change']['before']['update_time'] = '2000-01-01T01:00:00.123456790Z'
        with self.assertRaises(ValueError):
            c.validate_plan(nanos, 'registry')
        for mutate in (
            lambda p: p['resource_drift'][-1]['change']['after'].update(labels={'unexpected': 'value'}),
            lambda p: p['resource_drift'][-1]['change']['after'].update(docker_config=[{'immutable_tags': False}]),
            lambda p: p['resource_drift'][-1]['change']['after_unknown'].update(update_time=True),
            lambda p: p['resource_changes'][-1]['change'].update(actions=['update']),
        ):
            rejected = deepcopy(plan)
            mutate(rejected)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                c.validate_plan(rejected, 'registry')
        key = 'private-key-for-synthetic-tests-only'
        self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                            c.commitment(plan | {'resource_drift': []}, 'state-version', 'a' * 40, key))

    def test_registry_first_read_accepts_only_absent_label_representation(self):
        plan = refreshed_metadata_fixture()
        plan['variables']['stage']['value'] = 'registry'
        for item in plan['resource_changes']:
            item['change']['actions'] = ['no-op']
        address = 'module.platform[0].google_artifact_registry_repository.mypage[0]'
        item = {'address': address, 'mode': 'managed', 'provider_name': 'registry.terraform.io/hashicorp/google',
                'change': {'actions': ['no-op'], 'after': deepcopy(c.resource_contract('registry')[address]) | {'labels': {}},
                           'after_unknown': {}}}
        plan['resource_changes'].append(item)
        drift = deepcopy(item)
        drift['change'].update(actions=['update'], before=deepcopy(drift['change']['after']) | {'labels': None})
        plan['resource_drift'].append(drift)
        original = deepcopy(plan)
        self.assertEqual(c.validate_plan(plan, 'registry'), {'resources': 9, 'creates': 0})
        self.assertEqual(plan, original)
        for mutate in (
            lambda p: p['resource_drift'][-1]['change']['after'].update(labels={'changed': 'value'}),
            lambda p: p['resource_drift'][-1]['change']['before'].update(labels={}),
            lambda p: p['resource_drift'][-1]['change']['after'].update(update_time='synthetic-new-time'),
            lambda p: p['resource_drift'][-1]['change']['after_unknown'].update(labels=True),
            lambda p: p['resource_changes'][-1]['change'].update(actions=['update']),
            lambda p: p['resource_drift'][-1].update(address='unreviewed-repository'),
            lambda p: (p['resource_drift'][-1]['change']['before'].update(repository_id='other'),
                       p['resource_drift'][-1]['change']['after'].update(repository_id='other')),
        ):
            rejected = deepcopy(plan)
            mutate(rejected)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                c.validate_plan(rejected, 'registry')
        key = 'private-key-for-synthetic-tests-only'
        self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                            c.commitment(plan | {'resource_drift': []}, 'state-version', 'a' * 40, key))

    def test_shared_policy_etag_refresh_preserves_the_exact_managed_member(self):
        plan = refreshed_metadata_fixture()
        for item in plan['resource_changes']:
            item['change']['actions'] = ['no-op']
            if '_iam_member.' not in item['address']:
                continue
            item['change']['after']['etag'] = 'synthetic-new-etag'
            drift = deepcopy(item)
            drift['change']['actions'] = ['update']
            drift['change']['before'] = deepcopy(drift['change']['after'])
            drift['change']['before']['etag'] = 'synthetic-old-etag'
            plan['resource_drift'].append(drift)
        original = deepcopy(plan)
        self.assertEqual(c.validate_plan(plan), {'resources': 8, 'creates': 0})
        self.assertEqual(plan, original)
        key = 'private-key-for-synthetic-tests-only'
        self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                            c.commitment(plan | {'resource_drift': []}, 'state-version', 'a' * 40, key))
        index = next(i for i, item in enumerate(plan['resource_drift']) if '_iam_member.' in item['address'])
        for mutate in (
            lambda p: p['resource_drift'][index]['change']['after'].update(member='allUsers'),
            lambda p: p['resource_drift'][index]['change']['after'].update(role='roles/owner'),
            lambda p: (p['resource_drift'][index]['change']['before'].update(member='allUsers'),
                       p['resource_drift'][index]['change']['after'].update(member='allUsers')),
            lambda p: p['resource_drift'][index]['change']['after'].update(condition=[{'expression': 'true'}]),
            lambda p: p['resource_drift'][index]['change']['after'].update(etag=''),
            lambda p: p['resource_drift'][index]['change']['before'].update(etag=None),
            lambda p: p['resource_drift'][index]['change']['after_unknown'].update(etag=True),
            lambda p: p['resource_drift'][index].update(address='unreviewed-iam-member'),
            lambda p: next(x for x in p['resource_changes'] if x['address'] == p['resource_drift'][index]['address'])['change'].update(actions=['update']),
        ):
            rejected = deepcopy(plan)
            mutate(rejected)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                c.validate_plan(rejected)

    def test_bootstrap_apply_modes_stay_internal_and_reach_the_independent_apply_job(self):
        spec = importlib.util.spec_from_file_location('workflow_yaml', ROOT / '.github/codeql/validate-source.py')
        loader = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(loader)

        dispatch = loader.load_yaml(ROOT / '.github/workflows/ci.yml')['on']['workflow_dispatch']['inputs']
        self.assertNotIn('mypage_foundation_mode', dispatch)
        self.assertNotIn('mypage_foundation_target', dispatch)

        jobs = loader.load_yaml(ROOT / '.github/workflows/mypage-foundation-authenticated.yml')['jobs']
        expression = jobs['foundation_apply']['if'].removeprefix('${{').removesuffix('}}').strip()
        admitted = set()
        for term in expression.split(' || '):
            match = re.fullmatch(r"inputs.mode == '([^']+)'", term)
            self.assertIsNotNone(match)
            admitted.add(match[1])

        expected_apply = {
            'desired-apply',
            'foundation-metadata-apply',
            'foundation-apply',
            'registry-apply',
            'maintenance-operator-apply',
            'maintenance-upgrade-apply',
            'runtime-private-apply',
            'runtime-public-apply',
        }
        self.assertEqual(admitted, expected_apply)
        for mode in expected_apply | {m.removesuffix('-apply') + '-plan' for m in expected_apply}:
            c.stage_for_mode(mode)

        self.assertEqual(c.stage_for_mode('desired-plan'), 'runtime_public')
        self.assertEqual(c.stage_for_mode('desired-apply'), 'runtime_public')
        self.assertEqual(
            (ROOT / 'infra/mypage-platform/environments/prod/desired-stage.txt').read_text().strip(),
            'disabled',
        )

        operations = loader.load_yaml(ROOT / '.github/workflows/ci-operations.yml')
        self.assertEqual(
            operations['jobs']['mypage-image-build']['with']['mode'],
            'image-build',
        )

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
        # An opt-in stage cannot leak the repository into the old Foundation mode.
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
        # Do not broaden the Secret metadata resource's short-name contract.
        metadata = next(item for item in plan['resource_changes'] if '.google_secret_manager_secret.' in item['address'])
        metadata['change']['after']['secret_id'] = 'projects/' + c.PROJECT + '/secrets/mypage-oauth-client-id'
        with self.assertRaises(ValueError):
            c.validate_plan(plan)

    def test_dependency_source_order_does_not_change_independent_plan_commitment(self):
        plan = plan_fixture() | {'relevant_attributes': [
            {'resource': 'module.platform[0].google_service_account.runtime', 'attribute': ['account_id']},
            {'resource': 'module.platform[0].google_secret_manager_secret.oauth_client_id', 'attribute': ['secret_id']},
        ]}
        original = deepcopy(plan)
        reordered = deepcopy(plan)
        reordered['relevant_attributes'].reverse()
        key = 'private-key-for-synthetic-tests-only'
        self.assertEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                         c.commitment(reordered, 'state-version', 'a' * 40, key))
        self.assertEqual(plan, original)

    def test_dependency_source_contents_and_multiplicity_remain_bound(self):
        plan = plan_fixture() | {'relevant_attributes': [
            {'resource': 'synthetic-resource', 'attribute': ['items', 0, 'name']},
        ]}
        key = 'private-key-for-synthetic-tests-only'
        expected = c.commitment(plan, 'state-version', 'a' * 40, key)
        for change in (
            lambda p: p['relevant_attributes'][0].update(resource='other-resource'),
            lambda p: p['relevant_attributes'].append(deepcopy(p['relevant_attributes'][0])),
            lambda p: p['relevant_attributes'].clear(),
            lambda p: p['relevant_attributes'][0]['attribute'].reverse(),
        ):
            mutated = deepcopy(plan)
            change(mutated)
            self.assertNotEqual(c.commitment(mutated, 'state-version', 'a' * 40, key), expected)
        with self.assertRaises(ValueError):
            c.commitment(plan | {'relevant_attributes': {}}, 'state-version', 'a' * 40, key)

    def test_action_sequence_order_is_not_canonicalized(self):
        plan = plan_fixture()
        plan['resource_changes'][0]['change']['actions'] = ['delete', 'create']
        other = deepcopy(plan)
        other['resource_changes'][0]['change']['actions'].reverse()
        key = 'private-key-for-synthetic-tests-only'
        self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                            c.commitment(other, 'state-version', 'a' * 40, key))

    def test_only_empty_map_refreshes_on_existing_noop_secrets_are_admitted(self):
        self.assertEqual(c.validate_plan(refreshed_metadata_fixture()), {'resources': 8, 'creates': 4})
        metadata = refreshed_metadata_fixture()
        metadata['variables']['stage']['value'] = 'foundation_metadata'
        metadata['resource_changes'] = [x for x in metadata['resource_changes'] if '_iam_member.' not in x['address']]
        self.assertEqual(c.validate_plan(metadata, 'foundation_metadata'), {'resources': 4, 'creates': 0})
        no_op = refreshed_metadata_fixture()
        for item in no_op['resource_changes']:
            item['change']['actions'] = ['no-op']
        self.assertEqual(c.validate_plan(no_op), {'resources': 8, 'creates': 0})

    def test_real_drift_unknowns_foreign_resources_and_planned_writes_still_stop(self):
        mutations = [
            lambda p: p['resource_drift'][0]['change']['before'].update(labels={'retained': 'value'}),
            lambda p: p['resource_drift'][0]['change']['after'].update(labels={'unexpected': 'value'}),
            lambda p: (p['resource_drift'][0]['change']['before'].update(labels={}),
                       p['resource_drift'][0]['change']['after'].update(labels=None)),
            lambda p: p['resource_drift'][0]['change']['after'].update(project='youtube-study-space'),
            lambda p: p['resource_drift'][0]['change']['after'].update(replication=[]),
            lambda p: p['resource_drift'][0]['change']['after'].update(secret_id='other-secret'),
            lambda p: p['resource_drift'][0]['change']['after'].update(new_field={}),
            lambda p: p['resource_drift'][0]['change']['after_unknown'].update(labels=True),
            lambda p: p['resource_drift'][0].update(address='module.platform[0].google_service_account.runtime'),
            lambda p: p['resource_drift'][0].update(address='unreviewed-secret'),
            lambda p: p['resource_drift'][0].update(provider_name='registry.terraform.io/unreviewed/provider'),
            lambda p: p['resource_drift'][0].update(mode='data'),
            lambda p: p['resource_drift'][0]['change'].update(actions=['delete', 'create']),
            lambda p: p['resource_changes'][1]['change'].update(actions=['update']),
            lambda p: p['resource_changes'][1]['change'].update(actions=['create']),
            lambda p: p['resource_drift'].append(deepcopy(p['resource_drift'][0])),
        ]
        for mutate in mutations:
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                plan = refreshed_metadata_fixture()
                mutate(plan)
                c.validate_plan(plan)

    def test_empty_metadata_refresh_remains_bound_to_independent_plan_commitment(self):
        plan = refreshed_metadata_fixture()
        key = 'private-key-for-synthetic-tests-only'
        self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                            c.commitment(plan | {'resource_drift': []}, 'state-version', 'a' * 40, key))

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
        for changed, version, sha in ((plan | {'applyable': False}, 'absent', 'a' * 40), (plan, 'new-version', 'a' * 40), (plan, 'absent', 'b' * 40)):
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

    def test_workflow_apply_requires_independent_plan_agreement_and_cleanup(self):
        # Reuse the repository's YAML-1.2 parser when its CI dependency is installed.
        spec = importlib.util.spec_from_file_location('workflow_source', ROOT / '.github/codeql/validate-source.py')
        module = importlib.util.module_from_spec(spec)
        try:
            spec.loader.exec_module(module)
        except ModuleNotFoundError:
            self.skipTest('PyYAML checked by source-contract CI')
        workflow = module.load_yaml(ROOT / '.github/workflows/mypage-foundation-authenticated.yml')
        self.assertEqual(set(workflow['on']), {'workflow_call'})
        self.assertFalse(workflow['concurrency']['cancel-in-progress'])
        for kind in ('plan', 'apply'):
            job = workflow['jobs']['foundation_' + kind]
            self.assertIn("inputs.target == 'production'", job['environment'])
            self.assertIn('mypage-prod-' + kind, job['environment'])
            self.assertIn('mypage-dev-' + kind, job['environment'])
            self.assertEqual(job['steps'][-1]['if'], '${{ always() }}')
            self.assertEqual(job['steps'][1]['run'].split()[-1], 'stage')
        apply = workflow['jobs']['foundation_apply']
        step = next(x for x in apply['steps'] if x.get('id') == 'applied')
        self.assertIn("steps.prepared.outputs.commitment == needs.foundation_plan.outputs.commitment", step['if'])
        self.assertIn("steps.prepared.outputs.commitment != ''", step['if'])
        self.assertNotIn('commitment', json.dumps(step.get('env', {})))


if __name__ == '__main__':
    unittest.main()
