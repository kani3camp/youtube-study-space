"""New observations use reviewed addresses, without renewing old observations."""
import contextlib
import copy
from datetime import timedelta
import hashlib
import io
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import patch
import urllib.error

sys.path.insert(0,str(Path(__file__).resolve().parent))
import test_collect_runtime_read_scope as fixture_module
m=fixture_module.m


class ReviewedTargetsTests(unittest.TestCase):
    def setUp(self):
        # Reuse only synthetic setup, never an owner file or credential.
        fixture=fixture_module.Tests('test_pinned_hash_private_mode_and_1h_age')
        fixture.setUp();self.addCleanup(fixture.doCleanups)
        self.fixture=fixture
        self.source=copy.deepcopy(fixture.source)
        self.started=m.timestamp(self.source['capture_started_utc'])+timedelta(hours=2)
        self.project=copy.deepcopy(self.source['metadata']['project'])
        self.project['labels']={'fresh-fixture':'retained'}
        self.targets=m.reviewed_targets(self.source,now=self.started)
        self.path=fixture.staged_capture
        self.digest=fixture.capture_sha256
        self.args=['--capture-sha256',self.digest,'--reviewed-targets-file',str(self.path)]

    def fresh(self):m.observation_window(self.started,now=self.started)

    def test_old_evidence_is_not_used_or_copied_and_new_kind_has_new_window(self):
        old=self.source
        for key in ('complete','lambda6_source_match','ecs_source_match','ci_identity_read_verified','migration_ready'):
            old[key]=not old[key]
        metadata=old['metadata']
        metadata['project']['lifecycleState']='expired-never-copy'
        metadata['pool'].update(state='DELETED',disabled=True,mode='expired-never-copy')
        metadata['provider'].update(state='DELETED',disabled=True,attributeCondition='expired-never-copy')
        for key in ('pool_attestation_rules','iam_policy_resource','runtime_config','iam_policy','enabled_services','history_table'):
            metadata[key]={'expired-never-copy':True}
        targets=m.reviewed_targets(old,now=self.started)
        self.assertEqual(set(targets),{'project_id','project_number','pool','provider','source_capture_started_utc',
            'source_capture_completed_utc','source_capture_age_seconds_at_new_read'})
        with patch.object(m,'utc_now',return_value=self.started):
            value=m.collect_reviewed_targets(targets,self.fixture.request,lambda:(200,self.project),
                                             self.fresh,self.digest,self.started)
        self.assertEqual(value['schema_version'],2)
        self.assertEqual(value['kind'],'private-owner-runtime-read-scope-reviewed-targets')
        self.assertEqual(value['source_usage'],'reviewed-target-identifiers-only')
        self.assertEqual(value['source_capture_age_seconds_at_new_read'],7200)
        self.assertEqual(value['source_capture_started_utc'],old['capture_started_utc'])
        self.assertEqual(value['source_capture_completed_utc'],old['capture_completed_utc'])
        self.assertEqual(value['capture_started_utc'],self.started.isoformat().replace('+00:00','Z'))
        self.assertEqual(value['capture_expires_utc'],(self.started+timedelta(hours=1)).isoformat().replace('+00:00','Z'))
        self.assertEqual(value['fresh_project']['metadata'],self.project)
        self.assertNotIn('expired-never-copy',json.dumps(value))
        self.assertTrue(value['complete'])
        self.assertFalse(any(value['interpretation'].values()))
        self.assertNotIn('lambda6_source_match',value)
        self.assertNotIn('ecs_source_match',value)

    def test_incomplete_new_policy_cannot_inherit_old_complete(self):
        def request(path,body):
            if path.endswith(':getIamPolicy'):return 403,None
            return 200,{}
        with patch.object(m,'utc_now',return_value=self.started):
            value=m.collect_reviewed_targets(self.targets,request,lambda:(200,self.project),
                                             self.fresh,self.digest,self.started)
        self.assertFalse(value['complete'])
        self.assertTrue(self.source['complete'])
        self.assertIsNone(value['pool_iam_policy']['policy'])

    def test_project_must_be_fresh_dev_active_and_match_number_before_any_iam_request(self):
        for status,project in ((403,None),(200,dict(self.project,projectId='another-project')),
                (200,dict(self.project,projectNumber='333333333333')),
                (200,dict(self.project,lifecycleState='DELETE_REQUESTED')),(200,{'error':{}})):
            with self.subTest(status=status,shape=list(project) if project else None),patch.object(m,'transport') as iam:
                with self.assertRaises(ValueError):
                    m.collect_reviewed_targets(self.targets,iam,lambda:(status,project),self.fresh,self.digest,self.started)
                iam.assert_not_called()

    def test_target_schema_is_separate_and_rejects_other_projects_or_target_names(self):
        for kind,key,value in (('project','projectId','another-project'),('project','projectNumber','not-a-number'),
                               ('pool','name','projects/other/pool'),('provider','name','projects/other/provider')):
            changed=copy.deepcopy(self.source);changed['metadata'][kind][key]=value
            with self.assertRaises(ValueError):m.reviewed_targets(changed,now=self.started)
        for key,value in (('schema_version',True),('kind','different-kind'),('complete','true')):
            changed=copy.deepcopy(self.source);changed[key]=value
            with self.assertRaises(ValueError):m.reviewed_targets(changed,now=self.started)
        with self.assertRaises(ValueError):m.reviewed_targets(self.source,now=m.timestamp(self.source['capture_started_utc'])-timedelta(seconds=1))
        with self.assertRaises(ValueError):m.validate_capture(self.source,now=self.started)

    def test_mode_is_explicit_and_exclusive_and_default_stale_stops_before_auth(self):
        cases=[['--capture-sha256',self.digest,'--capture-file',str(self.path)],
               self.args+['--capture-file',str(self.path)],
               ['--capture-sha256',self.digest,'--reviewed-targets-file']]
        for args in cases:
            with patch.object(m,'utc_now',return_value=self.started),patch.object(m,'read_token') as auth, \
                 patch.object(m,'project_transport') as project,patch.object(m,'transport') as iam, \
                 contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(m.main(args),1)
                auth.assert_not_called();project.assert_not_called();iam.assert_not_called()

    def test_new_mode_checks_window_before_auth_after_project_and_before_save(self):
        with patch.object(m,'utc_now',side_effect=[self.started,self.started+timedelta(hours=1)]), \
             patch.object(m,'read_token') as auth,contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(m.main(self.args),1);auth.assert_not_called()
        now=[self.started]
        def expired_project(token):
            now[0]=self.started+timedelta(hours=1)
            return 200,self.project
        with patch.object(m,'utc_now',side_effect=lambda:now[0]),patch.object(m,'read_token',return_value='private-token'), \
             patch.object(m,'project_transport',side_effect=expired_project),patch.object(m,'transport') as iam, \
             patch.object(m,'write_capture') as output,contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(m.main(self.args),1);iam.assert_not_called();output.assert_not_called()
        now[0]=self.started
        def expired_result(*args):
            now[0]=self.started+timedelta(hours=1)
            return {'complete':True}
        with patch.object(m,'utc_now',side_effect=lambda:now[0]),patch.object(m,'read_token',return_value='private-token'), \
             patch.object(m,'collect_reviewed_targets',side_effect=expired_result),patch.object(m,'write_capture') as output, \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(m.main(self.args),1);output.assert_not_called()
        with self.assertRaises(ValueError):m.observation_window(self.started,now=self.started-timedelta(seconds=1))

    def test_window_expiry_during_iam_prevents_further_requests_and_save(self):
        now=[self.started]
        def expires(token,selectors,path,body):
            now[0]=self.started+timedelta(hours=1)
            return 200,copy.deepcopy(self.fixture.policy)
        with patch.object(m,'utc_now',side_effect=lambda:now[0]),patch.object(m,'read_token',return_value='private-token'), \
             patch.object(m,'project_transport',return_value=(200,self.project)) as project, \
             patch.object(m,'transport',side_effect=expires) as iam,patch.object(m,'write_capture') as output, \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(m.main(self.args),1)
            project.assert_called_once();iam.assert_called_once();output.assert_not_called()

    def test_new_mode_main_is_project_first_and_suppresses_private_values(self):
        order=[]
        def project(token):order.append('project');return 200,self.project
        def iam(token,selectors,path,body):order.append('iam');return self.fixture.request(path,body)
        public=io.StringIO()
        with patch.object(m,'utc_now',return_value=self.started),patch.object(m,'read_token',return_value='private-token'), \
             patch.object(m,'project_transport',side_effect=project),patch.object(m,'transport',side_effect=iam), \
             patch.object(m,'write_capture',return_value=Path('/tmp/dummy-result.json')) as saved, \
             contextlib.redirect_stdout(public),contextlib.redirect_stderr(public):
            self.assertEqual(m.main(self.args),0)
        self.assertEqual(order,['project','iam','iam','iam'])
        self.assertEqual(saved.call_args.args[0]['schema_version'],2)
        for secret in ('private-token','private-fixture-member','private-fixture-CEL',self.targets['pool'],self.targets['provider'],self.digest):
            self.assertNotIn(secret,public.getvalue())

    def test_fixed_project_get_transport_suppresses_error_bodies_and_redirects(self):
        error=urllib.error.HTTPError('https://private-fixture-identity',403,'private-token',{},io.BytesIO(b'private-fixture-CEL'))
        with patch.object(m.urllib.request,'build_opener') as opener:
            opener.return_value.open.side_effect=error
            public=io.StringIO()
            with contextlib.redirect_stdout(public),contextlib.redirect_stderr(public):
                self.assertEqual(m.project_transport('private-token'),(403,None))
            self.assertEqual(public.getvalue(),'')
            request=opener.return_value.open.call_args.args[0]
            self.assertEqual(request.method,'GET')
            self.assertEqual(request.full_url,'https://cloudresourcemanager.googleapis.com/v1/projects/'+m.PROJECT)
            self.assertIsNone(request.data)


if __name__=='__main__':unittest.main()
