"""Plan-only policy: data boundaries, fail-closed inventory, and bounded reflection."""
from copy import deepcopy
from datetime import datetime, timezone, timedelta
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
from contextlib import redirect_stdout

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
import plan_read_access as op
import bootstrap_normal_ops as bootstrap


def fixture(migrated=False):
    members = list(op.TARGETS)
    bindings = [{'role': name, 'members': [member, 'serviceAccount:unrelated-apply@example.invalid']}
                for member, roles in op.TARGETS.items() for name in sorted(roles)]
    roles = {name: {'name': name, 'etag': 'role-etag', 'stage': 'GA',
                   'includedPermissions': ['resourcemanager.projects.get']}
             for names in op.TARGETS.values() for name in names}
    roles[op.ROLE] = None
    if migrated:
        bindings = [{'role': op.ROLE, 'members': members}]
        roles[op.ROLE] = {'name': op.ROLE, 'etag': 'role-etag', **op.definition()}
    result = {'project': {'projectId': op.PROJECT, 'projectNumber': '123', 'lifecycleState': 'ACTIVE'},
              'policy': {'version': 3, 'etag': 'project-etag', 'bindings': bindings,
                         'auditConfigs': [{'service': 'allServices', 'auditLogConfigs': [{'logType': 'ADMIN_READ'}]}]},
              'roles': roles,
              'testable_permissions': [{'name': p, 'stage': 'GA', 'customRolesSupportLevel': 'SUPPORTED'}
                                       for p in op.definition()['includedPermissions']],
              'accounts': {}, 'analysis': {}}
    for member, legacy in op.TARGETS.items():
        result['accounts'][member] = {'email': member.split(':', 1)[1], 'keys': []}
        current = {op.ROLE} if migrated else legacy
        result['analysis'][member] = {'fullyExplored': True, 'mainAnalysis': {
            'fullyExplored': True, 'analysisQuery': {'scope': 'projects/' + op.PROJECT,
                'identitySelector': {'identity': member}, 'options': {'expandRoles': True}},
            'analysisResults': [{'fullyExplored': True,
                'attachedResourceFullName': '//cloudresourcemanager.googleapis.com/projects/123',
                'iamBinding': {'role': name, 'members': [member]}} for name in current]}}
    return result


def review(packet):
    return {'confirmation': packet['confirmation'], 'reviewed_at': datetime.now(timezone.utc).isoformat(),
            'evidence_reference': 'private operator inventory record',
            **{key: True for key in ('resource_iam_review_complete', 'cross_project_grants_review_complete',
               'plaintext_secrets_absent', 'no_other_members_of_plan_role', 'legacy_paths_reviewed')}}


class PlanAccessTest(unittest.TestCase):
    def test_manifest_contains_unused_families_and_preserves_existing_read_requirements(self):
        permissions = set(op.definition()['includedPermissions'])
        self.assertGreater(len(permissions), 100)
        self.assertTrue({'compute.instances.get', 'compute.networks.get', 'dns.managedZones.get',
                         'container.clusters.get', 'datastore.databases.getMetadata',
                         'datastore.indexes.list', 'artifactregistry.tags.get', 'iam.roles.get',
                         'firebaseauth.configs.get', 'resourcemanager.projects.getIamPolicy'} <= permissions)
        self.assertTrue(set(op.TARGETS).isdisjoint({'serviceAccount:mypage-foundation-apply@' + op.PROJECT}))

    def test_forbidden_and_unclassified_permissions_cannot_be_mixed_into_any_family(self):
        manifest = op.policy.load(op.MANIFEST)
        for permission in ('secretmanager.versions.access', 'storage.objects.get', 'bigquery.tables.getData',
                'logging.logEntries.list', 'billing.accounts.get', 'iam.serviceAccounts.getAccessToken',
                'iam.serviceAccountKeys.create', 'container.clusters.getCredentials', 'datastore.databases.get',
                'compute.instances.getSerialPortOutput', 'compute.instances.getFutureSecret',
                'cloudsql.instances.get', 'compute.networks.update', '*'):
            for family in manifest['families']:
                bad = deepcopy(manifest)
                item = next(f for f in bad['families'] if f['id'] == family['id'])
                item['permissions'] = sorted(item['permissions'] + [permission])
                with self.subTest(permission=permission, family=family['id']), self.assertRaises(ValueError):
                    op.policy.validate(bad)

    def test_scope_schema_duplicates_and_incomplete_classification_are_rejected(self):
        manifest = op.policy.load(op.MANIFEST)
        for key, value in (('schema_version', 2), ('project', 'youtube-study-space'),
                           ('role_id', 'Apply'), ('exclusions', []), ('families', manifest['families'][:-1])):
            with self.subTest(key=key), self.assertRaises(ValueError):
                op.policy.validate(manifest | {key: value})
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / 'manifest.json';path.write_text('{"project":"a","project":"b"}')
            with self.assertRaises(ValueError):op.policy.load(path)

    def test_plan_generation_is_static_and_does_not_expand_apply_permissions(self):
        catalog = {'pubsub.topics.get', 'pubsub.topics.create', 'compute.instances.get',
                   'compute.instances.create', 'secretmanager.versions.access'}
        actual = bootstrap.selected_permissions(catalog, ['resourcemanager.projects.get'])
        self.assertEqual(actual['plan'], op.definition()['includedPermissions'])
        self.assertEqual(actual['apply'], ['pubsub.topics.create', 'pubsub.topics.get', 'resourcemanager.projects.get'])

    def test_only_two_plan_bindings_change_and_other_shared_members_audit_configs_survive(self):
        snapshot = fixture();before = deepcopy(snapshot)
        packet = op.prepare(snapshot)
        self.assertEqual(snapshot, before)
        for member in op.TARGETS:self.assertEqual(op.grants(packet['project_policy'], member), {op.ROLE})
        unrelated = 'serviceAccount:unrelated-apply@example.invalid'
        self.assertEqual(op.grants(snapshot['policy'], unrelated), op.grants(packet['project_policy'], unrelated))
        self.assertEqual(packet['project_policy']['etag'], 'project-etag')
        self.assertEqual(packet['project_policy']['auditConfigs'], snapshot['policy']['auditConfigs'])
        self.assertTrue(all(not change['remove_permissions'] for change in packet['changes'].values()))
        self.assertEqual(op.normalized(op.prepare(fixture(True))['project_policy']), op.normalized(fixture(True)['policy']))

    def test_unexpected_grants_unknown_permissions_parent_keys_and_partial_analysis_fail_closed(self):
        first = next(iter(op.TARGETS));role = next(iter(op.TARGETS[first]))
        mutations = [
            lambda s:s['project'].update(parent={'type': 'organization', 'id': '1'}),
            lambda s:s['project'].update(projectId='youtube-study-space'),
            lambda s:s['policy'].pop('etag'),
            lambda s:s['policy']['bindings'].append({'role': 'roles/owner', 'members': [first]}),
            lambda s:s['policy']['bindings'][0].update(condition={'expression': 'true'}),
            lambda s:s['roles'][role]['includedPermissions'].append('secretmanager.versions.access'),
            lambda s:s['accounts'][first]['keys'].append({'keyType': 'USER_MANAGED'}),
            lambda s:s['accounts'][first].update(disabled=True),
            lambda s:s['analysis'][first].update(fullyExplored=False),
            lambda s:s['analysis'][first]['mainAnalysis'].update(nonCriticalErrors=[{'code': 'PERMISSION_DENIED'}]),
            lambda s:s['analysis'][first]['mainAnalysis']['analysisQuery'].update(resourceSelector={'fullResourceName': 'x'}),
            lambda s:s['analysis'][first]['mainAnalysis']['analysisResults'][0].update(attachedResourceFullName='//run.googleapis.com/child'),
            lambda s:s['analysis'][first]['mainAnalysis']['analysisResults'].clear(),
            lambda s:s['testable_permissions'][0].update(customRolesSupportLevel='NOT_SUPPORTED'),
            lambda s:s['testable_permissions'].pop(),
        ]
        for i, mutate in enumerate(mutations):
            snapshot = fixture();mutate(snapshot)
            with self.subTest(mutation=i), self.assertRaises((ValueError,KeyError)):op.prepare(snapshot)

    def test_existing_common_role_cannot_have_extra_members_permissions_or_be_deleted(self):
        for mutate in (
            lambda s:s['policy']['bindings'][0]['members'].append('serviceAccount:apply@example.invalid'),
            lambda s:s['roles'][op.ROLE]['includedPermissions'].append('secretmanager.versions.access'),
            lambda s:s['roles'][op.ROLE].update(deleted=True)):
            snapshot = fixture(True);mutate(snapshot)
            with self.assertRaises(ValueError):op.prepare(snapshot)

    def test_missing_support_level_is_supported_but_unknown_or_testing_is_not(self):
        snapshot=fixture()
        for p in snapshot['testable_permissions']:p.pop('customRolesSupportLevel');p['apiDisabled']=True
        op.prepare(snapshot)  # Disabled unused services are never enabled by this tool.
        snapshot['testable_permissions'][0]['customRolesSupportLevel']='TESTING'
        with self.assertRaises(ValueError):op.prepare(snapshot)

    def test_readonly_collection_uses_no_write_secret_state_or_impersonation_commands(self):
        snapshot=fixture();calls=[]
        def cloud(*args, **kw):
            calls.append(args)
            if args[:2]==('projects','describe'):return snapshot['project']
            if args[:2]==('projects','get-iam-policy'):return snapshot['policy']
            if args[:3]==('iam','roles','describe'):
                return snapshot['roles'][f'projects/{op.PROJECT}/roles/{args[3]}']
            if args[:3]==('iam','service-accounts','describe'):
                return {k:v for k,v in snapshot['accounts']['serviceAccount:'+args[3]].items() if k!='keys'}
            if args[:4]==('iam','service-accounts','keys','list'):return []
            if args[:2]==('asset','analyze-iam-policy'):
                return snapshot['analysis'][next(a.removeprefix('--identity=') for a in args if a.startswith('--identity='))]
            if args[:2]==('iam','list-testable-permissions'):return snapshot['testable_permissions']
            raise AssertionError('Unexpected or mutating command')
        with patch.object(op,'gcloud',side_effect=cloud),patch.object(op,'apply',side_effect=AssertionError('write')):
            collected=op.collect();op.prepare(collected)
        self.assertEqual(collected,snapshot)
        self.assertTrue(calls)

    def test_offline_dryrun_never_calls_cloud_or_apply_and_omits_raw_iam(self):
        with tempfile.TemporaryDirectory() as temp:
            path=Path(temp)/'snapshot.json';path.write_text(json.dumps(fixture()))
            with patch.object(op,'gcloud',side_effect=AssertionError('cloud')),patch.object(op,'apply',side_effect=AssertionError('write')),redirect_stdout(io.StringIO()) as output:
                op.main(['--snapshot',str(path)])
            summary=json.loads(output.getvalue())
            self.assertFalse(summary['environment_updated'])
            self.assertTrue(summary['blocked_pending_operator_review'])
            self.assertNotIn('unrelated-apply',output.getvalue())
            with self.assertRaises(ValueError):op.main(['--snapshot',str(path),'--apply'])

    def test_confirmation_binds_every_snapshot_field_and_manifest_review(self):
        snapshot=fixture();packet=op.prepare(snapshot)
        evidence=review(packet);op.review_evidence(evidence,packet['confirmation'])
        for key,bad in (('confirmation','bad'),('plaintext_secrets_absent',False),
                        ('legacy_paths_reviewed',False),('cross_project_grants_review_complete',False),
                        ('reviewed_at',(datetime.now(timezone.utc)-timedelta(days=2)).isoformat())):
            with self.subTest(key=key),self.assertRaises(ValueError):
                op.review_evidence(evidence|{key:bad},packet['confirmation'])
        snapshot['policy']['etag']='changed'
        self.assertNotEqual(packet['confirmation'],op.prepare(snapshot)['confirmation'])

    def test_apply_requires_live_fresh_matching_review_and_private_backup_before_write(self):
        snapshot=fixture();packet=op.prepare(snapshot)
        with tempfile.TemporaryDirectory() as temp:
            record=Path(temp)/'review.json';record.write_text(json.dumps(review(packet)))
            with patch.object(op,'collect',return_value=snapshot),patch.object(op,'apply') as apply,redirect_stdout(io.StringIO()):
                op.main(['--live','--apply','--confirm',packet['confirmation'],'--review',str(record),'--backup',str(Path(temp)/'backup')])
                apply.assert_called_once()
            with patch.object(op,'collect',return_value=snapshot),patch.object(op,'apply',side_effect=AssertionError('write')):
                with self.assertRaises(ValueError):
                    op.main(['--live','--apply','--confirm','wrong','--review',str(record),'--backup',str(Path(temp)/'backup')])

    def test_reflection_writes_only_common_role_and_exact_etag_policy_after_backup(self):
        snapshot=fixture();packet=op.prepare(snapshot);calls=[]
        with tempfile.TemporaryDirectory() as temp:
            backup=Path(temp)/'backup'
            def cloud(*args, **kw):
                self.assertTrue((backup/'before.json').exists())
                self.assertEqual((backup/'before.json').stat().st_mode & 0o777,0o600)
                calls.append(args)
                if args[:3]==('iam','roles','create'):
                    document=json.loads(Path(args[-1].removeprefix('--file=')).read_text())
                    self.assertEqual(document,packet['role']);self.assertEqual(args[3],op.ROLE_ID);return {}
                if args[:3]==('iam','roles','describe'):return packet['role']
                if args[:2]==('projects','set-iam-policy'):
                    self.assertEqual(json.loads(Path(args[-1]).read_text()),packet['project_policy']);return {}
                if args[:2]==('projects','get-iam-policy'):return packet['project_policy']|{'etag':'new'}
                raise AssertionError('Unexpected write')
            with patch.object(op,'gcloud',side_effect=cloud):op.apply(snapshot,packet,backup)
        self.assertEqual(len(calls),4)

    def test_bootstrap_refuses_to_write_before_the_plan_only_migration(self):
        def read(*args,**kwargs):
            if args[:2]==('projects','describe'):
                return {'projectId':op.PROJECT,'lifecycleState':'ACTIVE','projectNumber':'123'}
            if args[:3]==('iam','roles','describe'):
                name=args[3]
                if name==op.ROLE_ID:return {'stage':'GA','includedPermissions':op.definition()['includedPermissions']}
                return {'includedPermissions':['resourcemanager.projects.get']}
            if args[:2]==('projects','get-iam-policy'):return fixture()['policy']
            raise AssertionError('No writes allowed before migrated Plan grant')
        with patch.object(bootstrap,'gcloud',side_effect=read),patch.object(sys,'argv',['bootstrap']):
            with self.assertRaises(ValueError):bootstrap.bootstrap()

    def test_bootstrap_checks_apply_grants_before_any_write_after_plan_migration(self):
        for apply_roles in ([], ['roles/owner'], ['projects/'+op.PROJECT+'/roles/terraformNormalApply']):
            def read(*args,**kwargs):
                if args[:2]==('projects','describe'):
                    return {'projectId':op.PROJECT,'lifecycleState':'ACTIVE','projectNumber':'123'}
                if args[:3]==('iam','roles','describe'):
                    if args[3]==op.ROLE_ID:return {'stage':'GA','includedPermissions':op.definition()['includedPermissions']}
                    return {'includedPermissions':['resourcemanager.projects.get']}
                if args[:2]==('projects','get-iam-policy'):
                    return {'bindings':[{'role':op.ROLE,'members':['serviceAccount:terraform-dev-normal-plan@'+op.PROJECT+'.iam.gserviceaccount.com']},
                            *({'role':role,'members':['serviceAccount:terraform-dev-normal-apply@'+op.PROJECT+'.iam.gserviceaccount.com']} for role in apply_roles)]}
                raise AssertionError('Write before Apply grant inventory completed')
            with self.subTest(roles=apply_roles),patch.object(bootstrap,'gcloud',side_effect=read),patch.object(sys,'argv',['bootstrap']):
                with self.assertRaises(ValueError):bootstrap.bootstrap()


if __name__=='__main__':unittest.main()
