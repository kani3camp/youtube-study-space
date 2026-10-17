import contextlib
import copy
from datetime import timedelta
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import shutil
import stat
import tempfile
import unittest
from unittest.mock import patch
import urllib.error

PATH=Path(__file__).resolve().parents[1]/'scripts'/'collect_runtime_read_scope.py'
spec=importlib.util.spec_from_file_location('scope_read',PATH)
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)

class Tests(unittest.TestCase):
    def setUp(self):
        number='111111111111'
        pool='projects/'+number+'/locations/global/workloadIdentityPools/aws-runtime'
        provider=pool+'/providers/aws-provider'
        email=m.PROJECT+'@appspot.gserviceaccount.com'
        self.source={'schema_version':1,'kind':'private-operator-runtime-metadata-capture',
            'capture_started_utc':'2030-01-01T00:00:00Z','capture_completed_utc':'2030-01-01T00:00:01Z',
            'complete':True,'lambda6_source_match':True,'ecs_source_match':False,
            'ci_identity_read_verified':False,'migration_ready':False,
            'metadata':{'project':{'projectId':m.PROJECT,'projectNumber':number,'lifecycleState':'ACTIVE'},
                'pool':{'name':pool,'state':'ACTIVE'},'provider':{'name':provider,'state':'ACTIVE'},
                'pool_attestation_rules':{},'iam_policy_resource':'projects/'+m.PROJECT+'/serviceAccounts/'+email,
                'runtime_config':{'audience':'//iam.googleapis.com/'+provider,'service_account_email':email},
                'iam_policy':{},'enabled_services':{'services':[]},
                'history_table':{'tableReference':{'projectId':m.PROJECT,'datasetId':'firestore_export','tableId':'user-activity-history'},
                                 'schema':{'fields':[{'name':'dummy_field_'+str(i)} for i in range(8)]}}}}
        self.raw=json.dumps(self.source).encode()
        self.capture_sha256=hashlib.sha256(self.raw).hexdigest()
        staged=tempfile.TemporaryDirectory();self.addCleanup(staged.cleanup)
        staged_root=Path(staged.name);staged_root.chmod(0o700)
        private=staged_root/'private';private.mkdir(mode=0o700)
        self.staged_capture=private/'runtime-metadata-capture.json'
        self.staged_capture.write_bytes(self.raw);self.staged_capture.chmod(0o600)
        self.capture_path=self.staged_capture
        self.review_time=m.timestamp(self.source['capture_completed_utc'])+timedelta(minutes=15)
        self.selectors=m.validate_capture(self.source,now=self.review_time)
        self.policy={'version':3,'etag':'private-fixture-etag','bindings':[{
            'role':'projects/dummy/roles/readOnly','members':['private-fixture-member'],
            'condition':{'title':'private-fixture-title','expression':'private-fixture-CEL','location':'private-fixture-location'}}],
            'auditConfigs':[{'service':'allServices','auditLogConfigs':[{'logType':'ADMIN_READ','exemptedMembers':['private-fixture-exempt']}]}]}
        self.calls=[]
    def request(self,path,body):
        self.calls.append((path,copy.deepcopy(body)))
        if path.endswith(':getIamPolicy'):
            self.assertEqual(body,{'options':{'requestedPolicyVersion':3}})
            return 200,copy.deepcopy(self.policy)
        self.assertEqual(path,m.QUERY_PATH)
        self.assertEqual(set(body),{'fullResourceName','pageSize'})
        self.assertEqual(body['pageSize'],1000)
        return 200,{'permissions':[{'name':'iam.googleapis.com/'+p[len('iam.'):]} for p in m.NEEDED]}
    def fresh(self):m.validate_capture(self.source,now=self.review_time)
    def test_pinned_hash_private_mode_and_1h_age(self):
        value=m.read_capture(self.staged_capture,self.capture_sha256)
        self.assertEqual(hashlib.sha256(self.raw).hexdigest(),self.capture_sha256)
        self.assertEqual(m.validate_capture(value,now=self.review_time),self.selectors)
        with self.assertRaises(ValueError):m.validate_capture(value,now=self.review_time+timedelta(hours=1))
        with self.assertRaises(ValueError):m.validate_capture(value,now=m.timestamp(value['capture_started_utc'])-timedelta(seconds=1))
        for key,bad in [('schema_version',True),('migration_ready',True),('kind','unexpected')]:
            changed=copy.deepcopy(value);changed[key]=bad
            with self.assertRaises(ValueError):m.validate_capture(changed,now=self.review_time)
    def test_input_mismatch_permissions_symlink_duplicates_size_and_unknown_shapes(self):
        with tempfile.TemporaryDirectory() as directory:
            outer=Path(directory);outer.chmod(0o700);root=outer/'private';root.mkdir(mode=0o700);file=root/'runtime-metadata-capture.json'
            file.write_bytes(self.raw);file.chmod(0o600)
            m.read_capture(file,self.capture_sha256)
            file.write_bytes(self.raw+b' ')
            with self.assertRaises(ValueError):m.read_capture(file,self.capture_sha256)
            file.write_bytes(self.raw);file.chmod(0o644)
            with self.assertRaises(ValueError):m.read_capture(file,self.capture_sha256)
            file.chmod(0o600);link=root/'link';link.symlink_to(file)
            with self.assertRaises(ValueError):m.read_capture(link,self.capture_sha256)
        for value in (b'{"a":1,"a":2}',b'[]',b'{"a":NaN}',b' '*(m.MAX_BYTES+1)):
            with self.assertRaises(ValueError):m.decode(value)
        for changed in ({'bindings':None},{'unknown':[]},{'version':True},{'version':1,'bindings':self.policy['bindings']}):
            with self.assertRaises(ValueError):m.check_policy(changed)
    def test_full_pool_policy_retained_and_scope_information_is_not_caller_grant(self):
        captured=m.collect(self.selectors,self.request,self.fresh,self.capture_sha256)
        self.assertTrue(captured['complete'])
        self.assertEqual(captured['pool_iam_policy']['policy'],self.policy)
        self.assertEqual(len(self.calls),3)
        self.assertEqual(self.calls[0][0],'v1/'+self.selectors['pool']+':getIamPolicy')
        self.assertFalse(any(captured['interpretation'].values()))
        for scope in captured['scopeTestable'].values():
            self.assertTrue(scope['complete'])
            self.assertTrue(all(item['testable_on_this_scope'] for item in scope['required_read_support'].values()))
    def test_empty_scope_is_complete_but_never_supports_required_permissions(self):
        result=m.query_scope(self.selectors['pool'],lambda p,b:(200,{}),self.fresh)
        self.assertTrue(result['complete']);self.assertTrue(result['empty'])
        self.assertFalse(any(x['testable_on_this_scope'] for x in result['required_read_support'].values()))
    def test_alias_and_omitted_supported_default_are_explicit(self):
        values=[{'name':'iam.googleapis.com/workloadIdentityPools.get','primaryPermission':'iam.workloadIdentityPools.get'}]
        result=m.support(values)[m.NEEDED[0]]
        self.assertTrue(result['testable_on_this_scope']);self.assertEqual(result['custom_role_support_levels'],['SUPPORTED'])
        for level in ('NOT_SUPPORTED','TESTING'):
            values[0]['customRolesSupportLevel']=level
            self.assertFalse(m.support(values)[m.NEEDED[0]]['supported_for_custom_role'])
    def test_complete_pagination_changes_only_body_token(self):
        seen=[]
        def pages(path,body):
            seen.append(copy.deepcopy(body))
            if len(seen)==1:return 200,{'permissions':[{'name':m.NEEDED[0]}],'nextPageToken':'page/private-token'}
            return 200,{'permissions':[{'name':m.NEEDED[1]}]}
        result=m.query_scope(self.selectors['pool'],pages,self.fresh)
        self.assertTrue(result['complete']);self.assertEqual(len(result['permissions']),2)
        self.assertNotIn('pageToken',seen[0]);self.assertEqual(seen[1]['pageToken'],'page/private-token')
        self.assertEqual(seen[0]['fullResourceName'],seen[1]['fullResourceName'])
    def test_http_unavailable_partial_failure_loop_duplicate_and_unknown_do_not_mean_success(self):
        for code in (400,403,404,501):
            result=m.query_scope(self.selectors['provider'],lambda p,b:(code,None),self.fresh)
            self.assertFalse(result['complete']);self.assertIsNone(result['required_read_support'])
            self.assertEqual(result['page_http_statuses'],[code])
        for body in ({'unknown':[]},{'permissions':None},{'permissions':[{'name':'p','futureField':True}]},
                     {'permissions':[{'name':'p'},{'name':'p'}]},{'nextPageToken':'loop'}):
            result=m.query_scope(self.selectors['pool'],lambda p,b:(200,copy.deepcopy(body)),self.fresh)
            self.assertFalse(result['complete']);self.assertIsNone(result['required_read_support'])
        count=[0]
        def unique_tokens(path,body):
            count[0]+=1;return 200,{'nextPageToken':str(count[0])}
        result=m.query_scope(self.selectors['pool'],unique_tokens,self.fresh)
        self.assertEqual(count[0],m.MAX_PAGES);self.assertFalse(result['complete'])
    def test_partial_policy_unavailable_cannot_be_empty_policy_success(self):
        def request(path,body):
            if path.endswith(':getIamPolicy'):return 403,None
            return 200,{}
        value=m.collect(self.selectors,request,self.fresh,self.capture_sha256)
        self.assertFalse(value['complete']);self.assertIsNone(value['pool_iam_policy']['policy'])
        self.assertFalse(value['pool_iam_policy']['available'])
    def test_transport_is_exact_read_only_post_and_redirect_http_errors_are_silent(self):
        for path,body in [('v1/projects/dummy:getIamPolicy',{'options':{'requestedPolicyVersion':3}}),
                          (m.QUERY_PATH,{'fullResourceName':'//cloudresourcemanager.googleapis.com/projects/dummy','pageSize':1000}),
                          ('v1/'+self.selectors['pool']+':getIamPolicy',{'options':{'requestedPolicyVersion':1}})]:
            with self.assertRaises(ValueError):m.transport('private-fixture-token',self.selectors,path,body)
        with self.assertRaises(ValueError):m.NoRedirect().redirect_request(None,None,302,'',{},'https://other.invalid')
        error=urllib.error.HTTPError('https://iam.googleapis.com/private-fixture-identity',403,
              'private-fixture-token',{},io.BytesIO(b'private-fixture-CEL private-fixture-member'))
        with patch.object(m.urllib.request,'build_opener') as opener:
            opener.return_value.open.side_effect=error
            public=io.StringIO()
            with contextlib.redirect_stdout(public),contextlib.redirect_stderr(public):
                self.assertEqual(m.transport('private-fixture-token',self.selectors,m.QUERY_PATH,
                    {'fullResourceName':'//iam.googleapis.com/'+self.selectors['pool'],'pageSize':1000}),(403,None))
            self.assertEqual(public.getvalue(),'')
            request=opener.return_value.open.call_args.args[0]
            self.assertEqual(request.method,'POST')
    def test_main_rejects_expired_before_token_and_preserves_private_output(self):
        with patch.object(m,'find_capture',return_value=(self.capture_path,self.source)), \
             patch.object(m,'utc_now',return_value=self.review_time+timedelta(hours=1)), \
             patch.object(m,'read_token') as auth,contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(m.main(['--capture-sha256',self.capture_sha256]),1);auth.assert_not_called()
        public=io.StringIO()
        def request(token,selectors,path,body):
            self.assertEqual(token,'private-fixture-token');return self.request(path,body)
        with patch.object(m,'find_capture',return_value=(self.capture_path,self.source)), \
             patch.object(m,'utc_now',return_value=self.review_time), \
             patch.object(m,'read_token',return_value='private-fixture-token'), \
             patch.object(m,'transport',side_effect=request), \
             contextlib.redirect_stdout(public),contextlib.redirect_stderr(public):
            self.assertEqual(m.main(['--capture-sha256',self.capture_sha256]),0)
        result_line=next(line for line in public.getvalue().splitlines() if line.startswith('RESULT_FILE='))
        target=Path(result_line.split('=',1)[1]);self.addCleanup(shutil.rmtree,target.parent)
        self.assertEqual(stat.S_IMODE(target.stat().st_mode),0o600)
        self.assertEqual(stat.S_IMODE(target.parent.stat().st_mode),0o700)
        for value in ('private-fixture-token','private-fixture-member','private-fixture-CEL','private-fixture-identity',
                      self.selectors['pool'],self.selectors['provider']):
            self.assertNotIn(value,public.getvalue())
        self.assertEqual(json.loads(target.read_text())['pool_iam_policy']['policy'],self.policy)
    def test_explicit_upload_is_pinned_and_tightened_without_requiring_private_parent(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);root.chmod(0o755)
            path=root/'runtime-metadata-capture.json';path.write_bytes(self.raw);path.chmod(0o644)
            self.assertEqual(m.read_uploaded_capture(path,self.capture_sha256),self.source)
            self.assertEqual(stat.S_IMODE(path.stat().st_mode),0o600)
            path.write_bytes(self.raw+b' ')
            with self.assertRaises(ValueError):m.read_uploaded_capture(path,self.capture_sha256)
            path.write_bytes(self.raw);path.chmod(0o664)
            with self.assertRaises(ValueError):m.read_uploaded_capture(path,self.capture_sha256)
            path.chmod(0o600);root.chmod(0o775)
            with self.assertRaises(ValueError):m.read_uploaded_capture(path,self.capture_sha256)
            root.chmod(0o755);link=root/'symlink';link.symlink_to(path)
            with self.assertRaises(ValueError):m.read_uploaded_capture(link,self.capture_sha256)
            for wrong in (Path('runtime-metadata-capture.json'),root/'nested'/'..'/path.name):
                with self.assertRaises((ValueError,OSError)):m.read_uploaded_capture(wrong,self.capture_sha256)
            path.write_bytes(b' '*(m.MAX_BYTES+1))
            with self.assertRaises(ValueError):m.read_uploaded_capture(path,self.capture_sha256)

    def test_explicit_upload_main_uses_only_the_selected_pinned_file(self):
        with patch.object(m,'find_capture') as fallback, \
             patch.object(m,'utc_now',return_value=self.review_time), \
             patch.object(m,'read_token',return_value='private-fixture-token'), \
             patch.object(m,'transport',side_effect=lambda token,selectors,path,body:self.request(path,body)), \
             patch.object(m,'write_capture',return_value=Path('/tmp/dummy-result.json')) as saved, \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(m.main(['--capture-sha256',self.capture_sha256,'--capture-file',str(self.staged_capture)]),0)
            fallback.assert_not_called()
            self.assertEqual(saved.call_args.args[0]['source_capture_sha256'],self.capture_sha256)

    def test_explicit_wrong_file_stops_before_auth_and_never_falls_back(self):
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'wrong.json';path.write_text('{}');path.chmod(0o600)
            public=io.StringIO()
            with patch.object(m,'find_capture') as fallback,patch.object(m,'read_token') as token, \
                 contextlib.redirect_stdout(public),contextlib.redirect_stderr(public):
                self.assertEqual(m.main(['--capture-sha256',self.capture_sha256,'--capture-file',str(path)]),1)
                token.assert_not_called();fallback.assert_not_called()
            self.assertNotIn(str(path),public.getvalue())

    def test_capture_hash_is_mandatory_hex_and_bad_arguments_are_private_before_auth(self):
        for args in ([], ['--capture-sha256','not-a-hash-private-value'],
                     ['--capture-sha256','a'*64,'--capture-file'], ['--capture-sha256','a'*63], ['--capture-sha256','g'*64],
                     ['--capture-sha256','a'*64,'--unexpected','private-fixture-token']):
            public=io.StringIO()
            with patch.object(m,'find_capture') as capture,patch.object(m,'read_token') as token, \
                 contextlib.redirect_stdout(public),contextlib.redirect_stderr(public):
                self.assertEqual(m.main(args),1)
                capture.assert_not_called();token.assert_not_called()
            self.assertNotIn('not-a-hash-private-value',public.getvalue())
            self.assertNotIn('private-fixture-token',public.getvalue())

if __name__=='__main__':unittest.main()
