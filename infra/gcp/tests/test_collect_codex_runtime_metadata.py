"""Dummy-only authority/transport tests; no real keys, authentication or HTTP."""
import contextlib
import copy
from datetime import datetime, timedelta, timezone
import io
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import urllib.error
import urllib.parse

SCRIPTS = Path(__file__).resolve().parents[1] / 'scripts'
sys.path.insert(0, str(SCRIPTS))
import collect_codex_runtime_metadata as collector
from test_collect_runtime_metadata import fixture

DUMMY_ENV = {'AWS_ACCESS_KEY_ID': 'AKIA' + '0' * 16, 'AWS_SECRET_ACCESS_KEY': 'A' * 40}
SECRET = 'PRIVATE_AUTH_SENTINEL_DO_NOT_PRINT'


def raw(value):
    return json.dumps(value, ensure_ascii=False).encode()


def auth_fixture():
    selectors, metadata, request, calls = fixture()
    auth = {'schema_version': 1, 'kind': 'canonical-dedicated-codex-dev-metadata-identity-selector',
            'service_account_email': 'dummy-metadata-reader@' + collector.PROJECT + '.iam.gserviceaccount.com',
            'project_number': selectors['project_number'], 'pool_id': 'codex-cloud',
            'provider_id': 'aws-state-reader', 'source_page': '3d3357a8d0ce8191a8b1d8876b6840a9',
            'source_section': 'Codex Cloud：開発・インフラ調査アクセス（2026-10-12）'}
    identity = {'account_id': '222222222222', 'arn': 'arn:aws:iam::222222222222:user/DummyOriginal',
                'user_id': 'AIDA' + '0' * 17}
    return auth, selectors, identity, metadata, request


def xml(identity):
    return ('<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">'
            '<GetCallerIdentityResult><Account>' + identity['account_id'] + '</Account><Arn>'
            + identity['arn'] + '</Arn><UserId>' + identity['user_id']
            + '</UserId></GetCallerIdentityResult><ResponseMetadata><RequestId>dummy</RequestId>'
            '</ResponseMetadata></GetCallerIdentityResponse>').encode()


class Reply:
    status = 200
    def __init__(self, body):
        self.body, self.headers = body, {'Content-Length': str(len(body))}
    def __enter__(self): return self
    def __exit__(self, *args): pass
    def read(self, limit): return self.body[:limit]


class Opener:
    def __init__(self, auth, identity, metadata):
        self.auth, self.identity, self.metadata = auth, identity, metadata
        self.calls, self.inject = [], None
    def open(self, request, timeout):
        self.calls.append(request)
        if self.inject is not None:
            result = self.inject(request)
            if result is not None: return result
        url = request.full_url
        if url == collector.AWS_STS: return Reply(xml(self.identity))
        if url == collector.GOOGLE_STS:
            return Reply(raw({'access_token': 'dummy-federated-token', 'issued_token_type': collector.ACCESS_TYPE,
                              'token_type': 'Bearer', 'expires_in': 900}))
        if url == collector.impersonation(self.auth):
            expires = (datetime.now(timezone.utc) + timedelta(minutes=15)).strftime('%Y-%m-%dT%H:%M:%SZ')
            return Reply(raw({'accessToken': 'dummy-impersonated-token', 'expireTime': expires}))
        host = urllib.parse.urlsplit(url).netloc
        if host == 'cloudresourcemanager.googleapis.com': key = 'project'
        elif host == 'serviceusage.googleapis.com': key = 'services'
        elif host == 'bigquery.googleapis.com': key = 'history'
        elif ':listAttestationRules' in url: key = 'attestation'
        elif ':getIamPolicy' in url: key = 'policy'
        elif '/providers/' in url: key = 'provider'
        else: key = 'pool'
        return Reply(raw(self.metadata[key]))


def fake_sdk(auth):
    created = []
    class Supplier: pass
    class Signer:
        def __init__(self, region): assert region == collector.REGION
        def get_request_options(self, keys, url, method):
            return dict(url=url, method=method, headers={'Authorization': 'dummy-signed-authorization'})
    class Credentials:
        def __init__(self, **kwargs):
            self.kwargs, self.token = kwargs, None
            created.append(self)
            assert kwargs['credential_source'] is None
            assert kwargs['audience'] == collector.audience(auth)
            assert kwargs['service_account_impersonation_url'] == collector.impersonation(auth)
        def refresh(self, request):
            context = SimpleNamespace(audience=collector.audience(auth), subject_token_type=collector.SUBJECT_TYPE)
            supplier = self.kwargs['aws_security_credentials_supplier']
            assert supplier.get_aws_region(context, None) == collector.REGION
            value = supplier.get_aws_security_credentials(context, None)
            assert value[0] == DUMMY_ENV['AWS_ACCESS_KEY_ID'] and value[2] is None
            request(collector.GOOGLE_STS, method='POST', body=urllib.parse.urlencode({
                'grant_type': 'urn:ietf:params:oauth:grant-type:token-exchange',
                'requested_token_type': collector.ACCESS_TYPE, 'subject_token_type': collector.SUBJECT_TYPE,
                'subject_token': 'dummy-subject', 'audience': collector.audience(auth), 'scope': collector.IAM_SCOPE}))
            request(collector.impersonation(auth), method='POST',
                    body=raw({'delegates': None, 'scope': [collector.SCOPE], 'lifetime': '900s'}))
            self.token = 'dummy-impersonated-token'
    return SimpleNamespace(AwsSecurityCredentialsSupplier=Supplier, AwsSecurityCredentials=lambda *args: args,
                           RequestSigner=Signer, Credentials=Credentials, created=created)


class CollectionTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='dummy-codex-collector-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.root.chmod(0o700)
        self.auth, self.selectors, self.identity, self.metadata, _ = auth_fixture()
        self.sdk = fake_sdk(self.auth)
        self.opener = Opener(self.auth, self.identity, self.metadata)
        self.dep_root = self.root / 'dependencies'; self.dep_root.mkdir(mode=0o700)
        (self.dep_root/'dummy.py').write_text('# public dummy dependency\n')
        self.dependency = {'schema_version':1,'kind':'private-isolated-codex-metadata-dependencies',
                           'root':str(self.dep_root),'tree_sha256':collector.dependency_tree(self.dep_root),
                           'python_version':'3.12','platform':'linux-x86_64'}
        current = datetime.now(timezone.utc).replace(microsecond=0)
        self.manifest = {'schema_version':1,'kind':'private-codex-dev-runtime-metadata-read',
            'source_revision':'1'*40,'script_sha256':'2'*64,'collector_sha256':'3'*64,
            'requirements_sha256':collector.digest(collector.REQUIREMENTS.read_bytes()),
            'auth_selector_sha256':collector.digest(raw(self.auth)),
            'runtime_selector_sha256':collector.digest(raw(self.selectors)),
            'dependency_selector_sha256':collector.digest(raw(self.dependency)),
            'issued_utc':(current-timedelta(seconds=1)).strftime('%Y-%m-%dT%H:%M:%SZ'),
            'expires_utc':(current+timedelta(minutes=14)).strftime('%Y-%m-%dT%H:%M:%SZ'),
            'source_identity':self.identity,'execute_approved':True}
        self.args = SimpleNamespace(manifest_file=self.root/'manifest.json',manifest_sha256='',
            auth_selector_file=self.root/'auth.json',runtime_selector_file=self.root/'runtime.json',
            dependency_selector_file=self.root/'dependencies.json',output=self.root/'capture.json',execute=True)
        self.write_inputs()
    def write_inputs(self):
        for path,value in ((self.args.auth_selector_file,self.auth),(self.args.runtime_selector_file,self.selectors),
                           (self.args.dependency_selector_file,self.dependency)):
            path.write_bytes(raw(value));path.chmod(0o600)
        for key,path in (('auth_selector_sha256',self.args.auth_selector_file),
                         ('runtime_selector_sha256',self.args.runtime_selector_file),
                         ('dependency_selector_sha256',self.args.dependency_selector_file)):
            self.manifest[key]=collector.digest(path.read_bytes())
        self.args.manifest_file.write_bytes(raw(self.manifest));self.args.manifest_file.chmod(0o600)
        self.args.manifest_sha256=collector.digest(self.args.manifest_file.read_bytes())
    def run_core(self, env=None, clock=collector.now):
        sources={name:(SCRIPTS/(name+'.py')).read_bytes() for name in collector.COMPANIONS}
        with patch.object(collector,'validate_dependencies',return_value=self.dep_root):
            return collector.run(self.args,env=DUMMY_ENV if env is None else env,sdk_loader=lambda _:self.sdk,
                                 opener=self.opener,check_source=lambda _:sources,clock=clock)
    def reject(self, env=None):
        with self.assertRaises(Exception):self.run_core(env=env)
        self.assertFalse(self.args.output.exists())

    def test_default_offline_never_imports_sdk_reads_keys_or_opens_http_and_creates_no_capture(self):
        self.args.execute=False
        self.manifest['execute_approved']=False;self.write_inputs()
        class NoEnv:
            def get(self,*args):raise AssertionError('env keys read in offline mode')
        with patch.object(collector,'validate_dependencies',return_value=self.dep_root), \
             patch.object(collector,'load_sdk') as sdk,patch.object(collector,'source_keys') as keys, \
             patch.object(collector,'load_legacy') as companion:
            self.assertFalse(collector.run(self.args,env=NoEnv(),check_source=lambda _:None))
        sdk.assert_not_called();keys.assert_not_called();companion.assert_not_called()
        self.assertEqual(self.opener.calls,[]);self.assertFalse(self.args.output.exists())

    def test_loading_public_entrypoint_does_not_execute_companions_or_sdk(self):
        code=('import sys; sys.path.insert(0,'+repr(str(SCRIPTS))+'); '
              'import collect_codex_runtime_metadata as c; '
              'assert not any(name in sys.modules for name in (*c.COMPANIONS,"google.auth.aws"))')
        result=subprocess.run([sys.executable,'-B','-c',code],capture_output=True,check=False)
        self.assertEqual(result.returncode,0)
        self.assertEqual(result.stdout,b'');self.assertEqual(result.stderr,b'')

    def test_companion_execution_uses_verified_bytes_not_cached_or_ambient_modules(self):
        sources={name:(SCRIPTS/(name+'.py')).read_bytes() for name in collector.COMPANIONS}
        bad=SimpleNamespace(__file__=str(SCRIPTS/'collect_runtime_metadata.py'),collect=lambda *_:SECRET)
        with patch.dict(sys.modules,{'collect_runtime_metadata':bad}):
            module=collector.load_legacy(sources)
            self.assertIsNot(module,bad)
            self.assertEqual(module.collect(self.selectors,fixture()[2])['pool'],self.metadata['pool'])
        with self.assertRaises(collector.Stop):collector.load_legacy({})

    def test_explicit_dummy_read_is_source_sts_then_exact_wif_then_seven_types_with_false_readiness(self):
        self.assertTrue(self.run_core())
        self.assertEqual([r.full_url for r in self.opener.calls[:3]],
                         [collector.AWS_STS,collector.GOOGLE_STS,collector.impersonation(self.auth)])
        self.assertEqual(len(self.opener.calls),10)
        value=collector.decode(self.args.output.read_bytes())
        self.assertEqual(value['metadata']['iam_policy'],self.metadata['policy'])
        self.assertEqual(value['metadata']['history_table'],self.metadata['history'])
        self.assertTrue(value['complete']);self.assertTrue(value['source_identity_verified'])
        for key in ('migration_ready','ci_identity_read_verified','caller_scope_verified','root_admission_verified'):
            self.assertIs(value[key],False)
        self.assertEqual(stat.S_IMODE(self.args.output.stat().st_mode),0o600)
        self.assertIsNone(self.sdk.created[0].token)
        value_text=self.args.output.read_text()
        for secret in (*DUMMY_ENV.values(),'dummy-federated-token','dummy-impersonated-token','dummy-signed-authorization'):
            self.assertNotIn(secret,value_text)
        original=self.args.output.read_bytes()
        with self.assertRaises(Exception):self.run_core()
        self.assertEqual(self.args.output.read_bytes(),original)

    def test_each_original_source_identity_mismatch_stops_before_google_sts(self):
        for key,wrong in (('account_id','333333333333'),('arn','arn:aws:sts::222222222222:assumed-role/Dummy/session'),
                          ('user_id','AIDA'+'1'*17)):
            with self.subTest(key=key):
                self.opener.identity={**self.identity,key:wrong};self.opener.calls=[]
                self.reject();self.assertEqual(len(self.opener.calls),1)
                self.assertEqual(self.sdk.created,[])

    def test_missing_session_or_profile_credentials_stop_before_any_request(self):
        cases=[{}, {'AWS_ACCESS_KEY_ID':DUMMY_ENV['AWS_ACCESS_KEY_ID']},
               {**DUMMY_ENV,'AWS_ACCESS_KEY_ID':'ASIA'+'0'*16}]
        cases += [{**DUMMY_ENV,key:SECRET} for key in collector.FORBIDDEN_ENV]
        for env in cases:
            with self.subTest(keys=set(env)):
                self.reject(env);self.assertEqual(self.opener.calls,[])

    def test_source_credentials_are_snapshotted_once_and_region_is_not_read_from_environment(self):
        class Count(dict):
            def get(self,key,default=None):
                seen[key]=seen.get(key,0)+1
                return super().get(key,default)
        seen={}
        self.assertTrue(self.run_core(env=Count({**DUMMY_ENV,'AWS_REGION':'untrusted-region'})))
        self.assertEqual(seen['AWS_ACCESS_KEY_ID'],1);self.assertEqual(seen['AWS_SECRET_ACCESS_KEY'],1)
        self.assertNotIn('AWS_REGION',seen)

    def test_selector_pins_and_wrong_project_audience_sa_source_shape_reject_without_keys(self):
        cases=[('auth',lambda:self.auth.update(provider_id='other-provider')),
               ('auth',lambda:self.auth.update(service_account_email='terraform-dev-plan@'+collector.PROJECT+'.iam.gserviceaccount.com')),
               ('auth',lambda:self.auth.update(service_account_email=collector.PROJECT+'@appspot.gserviceaccount.com')),
               ('runtime',lambda:self.selectors.update(project_id='other-project')),
               ('runtime',lambda:self.selectors.update(runtime_audience='//untrusted'))]
        for name,mutate in cases:
            with self.subTest(name=name):
                old_auth,old_runtime=copy.deepcopy(self.auth),copy.deepcopy(self.selectors)
                mutate();self.write_inputs()
                self.reject();self.assertEqual(self.opener.calls,[])
                self.auth,self.selectors=old_auth,old_runtime
        self.write_inputs();self.args.auth_selector_file.write_bytes(raw({**self.auth,'unknown':SECRET}))
        self.reject();self.assertEqual(self.opener.calls,[])

    def test_expired_overlong_not_approved_window_and_tampered_pins_stop_before_auth(self):
        original=copy.deepcopy(self.manifest)
        for override in ({'expires_utc':'2000-01-01T00:00:00Z'},
                         {'expires_utc':'2999-01-01T00:00:00Z'},{'execute_approved':False},
                         {'source_revision':'__FINAL_SOURCE__'},{'script_sha256':'0'*64}):
            with self.subTest(override=override):
                self.manifest={**original,**override};self.write_inputs()
                self.reject();self.assertEqual(self.opener.calls,[])
        self.manifest=original;self.write_inputs();self.args.manifest_sha256='f'*64
        self.reject();self.assertEqual(self.opener.calls,[])

    def test_unsafe_paths_permissions_symlink_hardlink_and_checkout_stop_before_auth(self):
        path=self.args.auth_selector_file
        path.chmod(0o644);self.reject();path.chmod(0o600)
        alias=self.root/'alias.json';alias.symlink_to(path)
        self.args.auth_selector_file=alias;self.reject();self.args.auth_selector_file=path;alias.unlink()
        os.link(path,alias);self.reject();alias.unlink()
        (self.root/'.git').write_text('gitdir: dummy');self.reject();(self.root/'.git').unlink()
        self.root.chmod(0o755);self.reject();self.root.chmod(0o700)
        self.assertEqual(self.opener.calls,[])

    def test_public_cli_parser_and_dependency_exceptions_never_echo_secret_values(self):
        for args in (['--'+SECRET],[SECRET]):
            stream=io.StringIO()
            with contextlib.redirect_stdout(stream),contextlib.redirect_stderr(stream):
                self.assertEqual(collector.main(args),1)
            self.assertNotIn(SECRET,stream.getvalue())
        stream=io.StringIO()
        argv=['--manifest-file',str(self.args.manifest_file),'--manifest-sha256',self.args.manifest_sha256,
              '--auth-selector-file',str(self.args.auth_selector_file),'--runtime-selector-file',str(self.args.runtime_selector_file),
              '--dependency-selector-file',str(self.args.dependency_selector_file),'--output',str(self.args.output)]
        with patch.object(collector,'run',side_effect=ValueError(SECRET)),contextlib.redirect_stdout(stream):
            self.assertEqual(collector.main(argv),1)
        self.assertNotIn(SECRET,stream.getvalue());self.assertEqual(len(stream.getvalue().splitlines()),4)

    def test_http_error_body_never_read_published_or_persisted_and_cleanup_occurs(self):
        class Body(io.BytesIO):
            def read(self,*args):raise AssertionError('error body read')
        def poison(request):
            raise urllib.error.HTTPError(request.full_url,403,SECRET,{},Body(SECRET.encode()))
        self.opener.inject=poison
        self.reject();self.assertEqual(len(self.opener.calls),1)

    def test_each_auth_failure_stops_without_retry_fallback_or_partial_output(self):
        for target in (collector.AWS_STS,collector.GOOGLE_STS,collector.impersonation(self.auth)):
            for status in (403,503,200):
                with self.subTest(target=target==collector.AWS_STS,status=status):
                    self.opener.calls=[]
                    def poison(request):
                        if request.full_url!=target:return None
                        if status==200:return Reply(raw({'unknown':SECRET}))
                        raise urllib.error.HTTPError(request.full_url,status,SECRET,{},io.BytesIO(SECRET.encode()))
                    self.opener.inject=poison
                    self.reject()
                    self.assertEqual(len(self.opener.calls),(collector.AWS_STS,collector.GOOGLE_STS,
                                     collector.impersonation(self.auth)).index(target)+1)

    def test_sdk_exception_does_not_expose_dependency_body_or_token(self):
        def fail(credentials,request):raise ValueError(SECRET)
        with patch.object(self.sdk.Credentials,'refresh',fail):
            with self.assertRaises(collector.Stop) as stopped:self.run_core()
        self.assertNotIn(SECRET,str(stopped.exception))
        self.assertEqual(len(self.opener.calls),1);self.assertFalse(self.args.output.exists())
        self.assertIsNone(self.sdk.created[-1].token)

    def test_source_pin_failure_precedes_environment_sdk_and_auth(self):
        class NoEnv:
            def get(self,*args):raise AssertionError('keys read before reviewed source')
        with patch.object(collector,'load_sdk') as sdk, \
             patch.object(collector,'source_keys') as keys, \
             patch.object(collector,'load_legacy') as companion:
            with self.assertRaises(collector.Stop):
                collector.run(self.args,env=NoEnv(),check_source=lambda _:collector.need(False))
        sdk.assert_not_called();keys.assert_not_called();companion.assert_not_called()
        self.assertFalse(self.args.output.exists());self.assertEqual(self.opener.calls,[])

    def test_output_replacement_or_mode_change_cannot_be_accepted(self):
        def replace(request):
            if 'cloudresourcemanager.googleapis.com' in request.full_url:
                self.args.output.unlink();self.args.output.write_text(SECRET);self.args.output.chmod(0o600)
        self.opener.inject=replace
        with self.assertRaises(collector.Stop):self.run_core()
        # A replacement file is not the reserved collector file; do not erase it.
        self.assertEqual(self.args.output.read_text(),SECRET);self.args.output.unlink()
        def permissions(request):
            if 'cloudresourcemanager.googleapis.com' in request.full_url:self.args.output.chmod(0o644)
        self.opener.inject=permissions;self.reject()

    def test_redirect_unknown_host_auth_method_and_extra_scope_are_rejected(self):
        with self.assertRaises(collector.Stop):collector.NoRedirect().redirect_request(None,None,302,SECRET,{},'https://evil.invalid')
        admission=SimpleNamespace(check=lambda:None)
        wire=collector.Wire(admission,self.auth,self.selectors,opener=self.opener)
        for url,method in (('http://169.254.169.254/latest','GET'),('https://evil.invalid','GET'),(collector.AWS_STS,'GET')):
            with self.assertRaises(collector.Stop):wire.call(url,method)
        self.assertEqual(self.opener.calls,[])
        wire.auth_calls=1
        body=urllib.parse.urlencode({'grant_type':'urn:ietf:params:oauth:grant-type:token-exchange',
            'requested_token_type':collector.ACCESS_TYPE,'subject_token_type':collector.SUBJECT_TYPE,
            'subject_token':'dummy','audience':'//untrusted','scope':collector.IAM_SCOPE})
        with self.assertRaises(collector.Stop):wire.sdk_request(collector.GOOGLE_STS,method='POST',body=body)
        self.assertEqual(self.opener.calls,[])

    def test_wrong_fresh_project_stops_before_pool_and_failed_metadata_has_no_output(self):
        self.metadata['project']['projectNumber']='333333333333'
        self.reject();self.assertEqual(len(self.opener.calls),4)

    def test_complete_pagination_preserves_full_data_and_rejects_loop_unknown_or_duplicate(self):
        calls=0
        def pages(request):
            nonlocal calls
            if 'serviceusage.googleapis.com' in request.full_url:
                calls+=1
                return Reply(raw({'nextPageToken':'private /?page','services':self.metadata['services']['services']} if calls==1 else {'services':[]}))
        self.opener.inject=pages
        self.assertTrue(self.run_core());self.assertEqual(calls,2)
        self.assertIn('pageToken=private%20%2F%3Fpage',self.opener.calls[-2].full_url)
        self.args.output.unlink()
        for bad in ({'services':[],'nextPageToken':'same'},{'unknown':[]},
                    {'services':self.metadata['services']['services']*2}):
            with self.subTest(bad=bad):
                self.opener.inject=lambda request: Reply(raw(bad)) if 'serviceusage.googleapis.com' in request.full_url else None
                self.reject()

    def test_expiry_after_response_or_before_save_does_not_create_capture(self):
        start=datetime.now(timezone.utc)
        ticks=[start]
        def clock():return ticks[0]
        def expire(request):
            if 'cloudresourcemanager.googleapis.com' in request.full_url:ticks[0]=start+timedelta(minutes=20)
        self.opener.inject=expire
        with self.assertRaises(Exception):self.run_core(clock=clock)
        self.assertFalse(self.args.output.exists());self.assertEqual(len(self.opener.calls),4)

    def test_pin_io_crossing_deadline_stops_before_opener_and_auth_attempt(self):
        ticks=[datetime.now(timezone.utc)]
        def checks():ticks[0]=collector.stamp(self.manifest['expires_utc'])
        admission=collector.Admission(self.manifest,checks,clock=lambda:ticks[0])
        wire=collector.Wire(admission,self.auth,self.selectors,opener=self.opener)
        with self.assertRaises(collector.Stop):wire.call(collector.AWS_STS,'POST')
        self.assertEqual(self.opener.calls,[])
        self.assertEqual(wire.auth_calls,0);self.assertEqual(wire.metadata_calls,0)

    def test_capture_completion_sample_must_be_monotonic_and_inside_window(self):
        start=datetime.now(timezone.utc)
        sources={name:(SCRIPTS/(name+'.py')).read_bytes() for name in collector.COMPANIONS}
        for invalid in (start-timedelta(seconds=1),start+timedelta(minutes=20)):
            with self.subTest(direction=invalid>start):
                queued=[];calls=0
                def check_source(_):
                    nonlocal calls
                    calls+=1
                    if calls==2:queued.append(invalid)
                    return sources
                def clock():return queued.pop(0) if queued else start
                with patch.object(collector,'validate_dependencies',return_value=self.dep_root):
                    with self.assertRaises(collector.Stop):
                        collector.run(self.args,env=DUMMY_ENV,sdk_loader=lambda _:self.sdk,
                            opener=self.opener,check_source=check_source,clock=clock)
                self.assertEqual(calls,2);self.assertFalse(self.args.output.exists())

    def test_supplier_rejects_unexpected_audience_subject_and_has_no_network_methods(self):
        supplier=collector.official_supplier(self.sdk,self.auth,tuple(DUMMY_ENV.values()))
        for context in (SimpleNamespace(audience='//wrong',subject_token_type=collector.SUBJECT_TYPE),
                        SimpleNamespace(audience=collector.audience(self.auth),subject_token_type='other')):
            with self.assertRaises(collector.Stop):supplier.get_aws_region(context,None)
            with self.assertRaises(collector.Stop):supplier.get_aws_security_credentials(context,None)

    def test_dependency_hash_or_unknown_distribution_changes_are_rejected_before_sdk(self):
        bad={**self.dependency,'tree_sha256':'f'*64}
        with self.assertRaises(collector.Stop):collector.validate_dependencies(bad,collector.REQUIREMENTS.read_bytes())
        with self.assertRaises(collector.Stop):collector.validate_dependencies(self.dependency,collector.REQUIREMENTS.read_bytes())
        symlink=self.dep_root/'alias.py';symlink.symlink_to(self.dep_root/'dummy.py')
        with self.assertRaises(collector.Stop):collector.dependency_tree(self.dep_root)

    def test_preloaded_ambient_sdk_dependency_is_rejected(self):
        ambient=SimpleNamespace(__file__=str(self.root/'unreviewed.py'))
        with patch.dict(sys.modules,{'cryptography':ambient}),self.assertRaises(collector.Stop):
            collector.load_sdk(self.dep_root)

    def test_reviewed_git_source_helper_and_lock_pins_are_exact(self):
        root=self.root/'reviewed-checkout';root.mkdir()
        scripts=root/'infra/gcp/scripts';scripts.mkdir(parents=True)
        helper=scripts/'collect_codex_runtime_metadata.py';helper.write_text('# reviewed transport\n')
        legacy=scripts/'collect_runtime_metadata.py';legacy.write_text('# reviewed collector\n')
        for name in collector.COMPANIONS[1:]:(scripts/(name+'.py')).write_text('# reviewed validator\n')
        lock=root/'infra/gcp/requirements.txt';lock.write_text('# reviewed dependencies\n')
        def git(*args):
            result=subprocess.run(['git','-C',str(root),'-c','user.name=Dummy',
                '-c','user.email=dummy@example.invalid','-c','commit.gpgsign=false',*args],
                stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=True)
            return result.stdout.decode().strip()
        git('init','-q');git('add','infra');git('commit','-qm','Dummy reviewed source')
        manifest={'source_revision':git('rev-parse','HEAD'),'script_sha256':collector.digest(helper.read_bytes()),
                  'collector_sha256':collector.digest(legacy.read_bytes()),'requirements_sha256':collector.digest(lock.read_bytes())}
        with patch.object(collector,'ROOT',root),patch.object(collector,'__file__',str(helper)), \
             patch.object(collector,'REQUIREMENTS',lock):
            collector.source_pins(manifest)
            for key in manifest:
                with self.subTest(pin=key),self.assertRaises(collector.Stop):
                    collector.source_pins({**manifest,key:'f'*len(manifest[key])})
            legacy.write_text('# unreviewed modification\n')
            with self.assertRaises(collector.Stop):collector.source_pins(manifest)

    def test_xml_duplicates_dtd_and_malformed_identity_cannot_be_authenticated(self):
        for data in (b'<!DOCTYPE x><x/>',b'<x/>',xml(self.identity).replace(b'</GetCallerIdentityResult>',b'<UserId>extra</UserId></GetCallerIdentityResult>')):
            with self.assertRaises(Exception):collector.source_identity(data)


class OfficialSDKDummyTest(unittest.TestCase):
    def test_pinned_real_sdk_import_supplier_and_official_dummy_signing_without_network(self):
        path=os.environ.get('CODEX_METADATA_TEST_DEPENDENCIES')
        if not path:self.skipTest('optional isolated pinned SDK not selected')
        root=Path(path).resolve()
        dependency={'schema_version':1,'kind':'private-isolated-codex-metadata-dependencies',
                    'root':str(root),'tree_sha256':collector.dependency_tree(root),
                    'python_version':'3.12','platform':'linux-x86_64'}
        self.assertEqual(collector.validate_dependencies(dependency,collector.REQUIREMENTS.read_bytes()),root)
        sdk=collector.load_sdk(root)
        auth,selectors,identity,metadata,_=auth_fixture()
        supplier=collector.official_supplier(sdk,auth,tuple(DUMMY_ENV.values()))
        context=SimpleNamespace(audience=collector.audience(auth),subject_token_type=collector.SUBJECT_TYPE)
        keys=supplier.get_aws_security_credentials(context,None)
        with patch('urllib.request.OpenerDirector.open',side_effect=AssertionError('network forbidden')):
            signed=sdk.RequestSigner(supplier.get_aws_region(context,None)).get_request_options(keys,collector.AWS_STS,'POST')
        self.assertEqual(signed['url'],collector.AWS_STS)
        self.assertEqual(signed['method'],'POST')
        self.assertTrue(signed['headers']['Authorization'].startswith('AWS4-HMAC-SHA256 '))
        self.assertNotIn('x-amz-security-token',{key.lower() for key in signed['headers']})
        # Real SDK protocol objects, exclusively invented keys and in-memory
        # dummy STS/IAM responses: this does not contact any auth endpoint.
        opener=Opener(auth,identity,metadata)
        wire=collector.Wire(SimpleNamespace(check=lambda:None),auth,selectors,opener=opener)
        with patch('urllib.request.OpenerDirector.open',side_effect=AssertionError('network forbidden')):
            self.assertEqual(collector.token(sdk,auth,tuple(DUMMY_ENV.values()),identity,wire),'dummy-impersonated-token')
        self.assertEqual(len(opener.calls),3)
        self.assertEqual(importlib_version(root),'2.56.2')


def importlib_version(root):
    return next(d.version for d in collector.importlib.metadata.distributions(path=[str(root)])
                if d.metadata['Name'].lower().replace('_','-')=='google-auth')


if __name__=='__main__':unittest.main()
