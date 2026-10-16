#!/usr/bin/env python3
"""Dummy end-to-end sync controller: actual auth/cloud/process writes are denied."""
import contextlib
import copy
from datetime import timedelta
import io
import json
import os
import re
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import terraform_history_observation_sync as controller
import test_terraform_history_observation_sync_profile as requests

sys.path.insert(0,str(controller.ROOT/'infra/gcp/tests'))
import test_history_observation_sync_archive as fixtures

gate=controller.gate
CANARY=fixtures.fixture.CANARY
REAL_RUN=subprocess.run
# One immutable window per test process: preview/apply share exact scope even
# when the suite crosses a second/day boundary. Runtime TTL rules are unchanged.
FIXTURE_NOW=controller.now().replace(microsecond=0)
FIXTURE_ISSUED=FIXTURE_NOW-timedelta(minutes=5)
FIXTURE_EXPIRES=FIXTURE_ISSUED+timedelta(hours=24)
DUMMY_ISSUED_UTC=FIXTURE_ISSUED.strftime('%Y-%m-%dT%H:%M:%SZ')
DUMMY_EXPIRES_UTC=FIXTURE_EXPIRES.strftime('%Y-%m-%dT%H:%M:%SZ')


def environment(root,operation='preview'):
    env=requests.dummy_env(operation)
    sha=REAL_RUN(['git','rev-parse','HEAD'],cwd=controller.ROOT,capture_output=True,text=True,check=True).stdout.strip()
    value=json.loads(env['HISTORY_OBSERVATION_SYNC_PROFILE']);value['git_sha']=sha
    value.update(issued_utc=DUMMY_ISSUED_UTC,expires_utc=DUMMY_EXPIRES_UTC)
    env.update(DEV_HISTORY_OBSERVATION_SYNC_ISSUED_UTC=DUMMY_ISSUED_UTC,
        DEV_HISTORY_OBSERVATION_SYNC_EXPIRES_UTC=DUMMY_EXPIRES_UTC)
    if operation=='synchronize':value['preview_reference']['source_sha']=sha
    env.update(GITHUB_SHA=sha,HISTORY_OBSERVATION_SYNC_PROFILE=json.dumps(value),
        GITHUB_JOB='history_observation_sync_'+('plan'if operation=='preview'else'apply'),
        RUNNER_TEMP=str(root),TF_WORKSPACE='default',STATE_KEY=controller.private.STATE_KEY,
        STATE_AWS_REGION='ap-northeast-1',AWS_MAX_ATTEMPTS='1',AWS_RETRY_MODE='standard',
        STATE_ACCOUNT_ID='111122223333',STATE_BUCKET='dummy-source-only-state',
        BACKEND_ROLE_ARN='arn:aws:iam::111122223333:role/dummy-'+('plan'if operation=='preview'else'apply'),
        BACKEND_ROLE_ID='DUMMY',TF_VAR_project_id=gate.PROJECT,GITHUB_OUTPUT=str(root/'outputs'),
        GCP_SMOKE_ACCESS_TOKEN='DUMMY_TOKEN',GCP_SMOKE_SERVICE_ACCOUNT='terraform-dev-'+('plan'if operation=='preview'else'apply')+'@'+gate.PROJECT+'.iam.gserviceaccount.com',
        GH_TOKEN='DUMMY_GITHUB_TOKEN')
    env.update({'TF_VAR_manage_'+key:'true'for key in controller.profiles.INPUT_FLAGS})
    for key,value in (('TF_VAR_primary_email_address',fixtures.fixture.legacy_fixture.fixture.EMAIL),
                      ('TF_VAR_primary_email_channel_name',next(item['change']['after']['name']for item in fixtures.fixture.fixture()[0]['resource_changes']if item['type']=='google_monitoring_notification_channel'))):
        env[key]=value
    (root/'outputs').touch(mode=0o600)
    return env


class DummyIO:
    def __init__(self):
        self.parts=fixtures.data();self.normal,self.metadata,self.before,self.after,self.refresh,_,_,_=self.parts
        for value in (self.before,self.after):
            for group in value['check_results']:
                if group['config_addr']=='var.dummy':
                    group['config_addr']='var.region';group['objects'][0]['object_addr']='var.region'
        self.shown=[]
        for group in self.before['check_results']:
            name=group['config_addr'].split('.')[-1]
            self.shown.append(dict(address=dict(kind='var',name=name,to_display=group['config_addr']),status='pass',
                instances=[dict(address=dict(to_display=group['objects'][0]['object_addr']),status='pass')]))
        self.normal['checks']=copy.deepcopy(self.shown);self.refresh['checks']=copy.deepcopy(self.shown)
        email=gate.PROJECT+'@appspot.gserviceaccount.com'
        for item in self.normal['resource_changes']:
            if item['type']=='google_cloudfunctions_function':
                for side in ('before','after'):item['change'][side]['service_account_email']=email
        for state in (self.before,self.after):
            for item in state['resources']:
                if item['type']=='google_cloudfunctions_function':item['instances'][0]['attributes']['service_account_email']=email
        for child in self.refresh['prior_state']['values']['root_module']['child_modules']:
            for item in child['resources']:
                if item['type']=='google_cloudfunctions_function':item['values']['service_account_email']=email
        self.normal['variables']={'quota_policy_settings':{'value':{'threshold':0.8,'second':0.6}}}
        self.refresh['variables']=copy.deepcopy(self.normal['variables'])
        self.metadata.update(etag='DUMMY_NEW',lastModifiedTime='200')
        self.applied=False;self.calls=[];self.fail_apply=False;self.bump_version=True
        self.snapshot_mutation=None;self.bad_head=False

    def bind(self,env):
        value=dict(nonce='ab'*32,backend=dict(account_id=env['STATE_ACCOUNT_ID'],bucket=env['STATE_BUCKET'],
            key=env['STATE_KEY'],region='ap-northeast-1',workspace='default'),history_metadata=self.metadata)
        env['RUNTIME_OWNERSHIP_PACKET_JSON']=json.dumps(dict(schema_version=1,receipt_binding=value))

    def google(self,path,token,body=None,*,host='cloudresourcemanager.googleapis.com',**kwargs):
        self.calls.append(('google',host,path));self.assert_read(body is None)
        if host=='bigquery.googleapis.com':return 200,copy.deepcopy(self.metadata)
        if host=='cloudfunctions.googleapis.com':
            return 200,dict(name=path[3:],runtime='nodejs22',status='ACTIVE',versionId='8',
                serviceAccountEmail=gate.PROJECT+'@appspot.gserviceaccount.com')
        return 200,dict(projectId=gate.PROJECT,state='ACTIVE')

    @staticmethod
    def assert_read(value):
        if not value:raise AssertionError('unexpected dummy mutation')

    def aws(self,*args):
        self.calls.append(('aws',args[1]));self.assert_read(args[0]=='s3api')
        state=self.after if self.applied else self.before
        raw=gate.canonical(state).encode()
        head=dict(ContentLength=len(raw),VersionId='DUMMY_AFTER'if self.applied and self.bump_version else'DUMMY_BEFORE',
            ETag='DUMMY_OPAQUE_STATE_TAG',ServerSideEncryption='AES256')
        if args[1]=='head-object':value=head
        elif args[1]=='get-object':
            self.assert_read(args[args.index('--range')+1]=='bytes=0-4194304')
            Path(args[-1]).write_bytes(raw);value=head
        elif args[1]=='list-objects-v2':
            prefix=args[args.index('--prefix')+1]
            self.assert_read(prefix in ('youtube-study-space/dev/terraform.tfstate.tflock','youtube-study-space/dev/workspaces/'))
            value=dict(Name='dummy-source-only-state',Prefix=prefix,MaxKeys=1,KeyCount=0,IsTruncated=False)
        else:raise AssertionError('unexpected dummy operation')
        return subprocess.CompletedProcess(args,0,json.dumps(value),'')

    def terraform(self,root,tfroot,env,name,args,**kwargs):
        if kwargs.get('before_send'):kwargs['before_send']()
        if kwargs.get('on_call'):kwargs['on_call']()
        self.calls.append(('terraform',args[0],name))
        if args[0]=='init':
            directory=tfroot/'.terraform/modules';directory.mkdir(parents=True,mode=0o700)
            manifest=[dict(Key='',Dir='')]
            manifest[0]['Dir']='.'
            manifest += [dict(Key=key,Source='../../modules/'+value,Dir='../../modules/'+value)for key,value in controller.MODULES.items()]
            controller.write_owned(directory/'modules.json',gate.canonical(dict(Modules=manifest)).encode())
            controller.write_owned(tfroot/'.terraform/terraform.tfstate',b'{"backend":"DUMMY"}')
        elif name in ('normal-plan','post-plan'):
            controller.write_owned(root/('normal.tfplan'if name=='normal-plan'else'post.tfplan'),b'DUMMY')
        elif name=='refresh-plan':
            previous=copy.deepcopy(self.before);previous.update(lineage='',serial=0)
            candidate=copy.deepcopy(self.after);candidate.update(serial=self.before['serial'],check_results=None)
            manifest=json.loads(controller.read_owned(tfroot/'.terraform/modules/modules.json'))['Modules']
            native=[{key:value for key,value in item.items()if key!='Source'or value}for item in manifest]
            members={'tfconfig/modules.json':gate.canonical(native).encode(),'.terraform.lock.hcl':controller.read_owned(tfroot/'.terraform.lock.hcl'),
                'tfplan':fixtures.proto(self.before['check_results']),'tfstate':gate.canonical(candidate).encode(),
                'tfstate-prev':gate.canonical(previous).encode()}
            for item in manifest:
                directory=tfroot if not item['Key']else(tfroot/item['Dir']).resolve()
                for path in directory.glob('*.tf'):members['tfconfig/m-'+item['Key']+'/'+path.name]=controller.read_owned(path)
            controller.write_owned(root/'refresh.tfplan',fixtures.packed(members))
        elif name=='normal-show':return copy.deepcopy(self.normal)
        elif name=='refresh-show':return copy.deepcopy(self.refresh)
        elif name=='post-show':
            value=copy.deepcopy(self.normal);value['resource_drift']=[];return value
        elif name=='apply':
            self.applied=True
            if self.fail_apply:raise TimeoutError(CANARY)
        else:raise AssertionError('unexpected dummy Terraform command')
        return 2 if name=='refresh-plan'else 0

    def git(self,args,**kwargs):
        if self.bad_head and args[:2]==['git','rev-parse']:return subprocess.CompletedProcess(args,0,b'b'*40+b'\n',b'')
        if args[:2]==['git','diff-index']:return subprocess.CompletedProcess(args,0,b'',b'')
        self.assert_read(args[0]=='git')
        return REAL_RUN(args,**kwargs)

    @contextlib.contextmanager
    def patched(self):
        with patch.object(controller.identity,'google',side_effect=self.google),\
             patch.object(controller.private,'aws',side_effect=self.aws),\
             patch.object(controller,'terraform',side_effect=self.terraform),\
             patch.object(controller.subprocess,'run',side_effect=self.git):
            yield


def preview(env,io_):
    io_.bind(env)
    controller.stage(env)
    session=controller.Session(env);session.preview()
    path=Path(env['RUNNER_TEMP'])/controller.provenance.PUBLIC_DIRECTORY/controller.provenance.PUBLIC_FILE
    return session,json.loads(controller.read_owned(path))


def apply_environment(root,proof):
    env=environment(root,'synchronize');value=json.loads(env['HISTORY_OBSERVATION_SYNC_PROFILE'])
    value['preview_reference'].update(run_id=proof['run_id'],source_sha=proof['source_sha'])
    env['HISTORY_OBSERVATION_SYNC_PROFILE']=json.dumps(value);env['GITHUB_RUN_ID']='103'
    return env


def approved(env,io_,proof):
    io_.bind(env)
    with patch.object(controller.provenance,'authenticate',return_value=(proof,{})):
        controller.stage(env)
        session=controller.Session(env)
    return session



class HistoryObservationControllerTest(unittest.TestCase):
    def test_default_closed_invalid_scope_and_arguments_stop_before_any_auth_or_process(self):
        for env in ({},{'HISTORY_OBSERVATION_SYNC_PROFILE':CANARY}):
            with patch.object(controller.subprocess,'run',side_effect=AssertionError('process')),\
                 patch.object(controller.urllib.request,'urlopen',side_effect=AssertionError('network')),\
                 patch('sys.stdout',io.StringIO())as out:
                self.assertEqual(controller.main(['--phase','preview'],env),3)
                self.assertNotIn(CANARY,out.getvalue())
        with patch('sys.stdout',io.StringIO())as out:
            self.assertEqual(controller.main(['--unknown',CANARY],{}),3)
            self.assertNotIn(CANARY,out.getvalue())

    def test_source_coupled_full12_preview_has_float_intent_no_mutation_and_private_cleanup(self):
        with tempfile.TemporaryDirectory()as temp:
            root=Path(temp);env=environment(root);io_=DummyIO()
            with io_.patched():session,proof=preview(env,io_)
            self.assertEqual(proof['counts']['drift'],1);self.assertEqual(proof['counts']['no-op'],12)
            self.assertTrue(proof['raw_candidate_state_verified']);self.assertFalse(proof['execution_permitted'])
            self.assertFalse(any(call[:2]==('terraform','apply')for call in io_.calls))
            text=json.dumps(proof);self.assertNotIn(CANARY,text);self.assertNotIn('DUMMY_OLD',text)
            self.assertEqual(stat.S_IMODE(session.root.stat().st_mode),0o700)
            controller.cleanup(env);self.assertFalse(session.root.exists());controller.publish_check(env)

    def test_preflight_output_operation_is_validated_and_cross_modes_stay_closed(self):
        with tempfile.TemporaryDirectory()as temp:
            env=environment(Path(temp));controller.preflight(env)
            self.assertEqual(Path(env['GITHUB_OUTPUT']).read_text(),'sync_operation=preview\n')
            for key,value in (('TARGET','prod'),('GITHUB_RUN_ATTEMPT','2'),('OWNERSHIP_WAVE','pool'),
                              ('MODE','plan'),('DEV_HISTORY_OBSERVATION_SYNC_ENABLED','false')):
                bad=env|{key:value}
                with self.assertRaises(ValueError):controller.preflight(bad)

    def test_unsafe_input_private_file_paths_and_cleaning_never_follow_symlink(self):
        with tempfile.TemporaryDirectory()as temp:
            root=Path(temp);env=environment(root);io_=DummyIO();io_.bind(env)
            target=root/controller.PRIVATE_DIRECTORY;other=root/'unrelated';other.mkdir();(other/'keep').write_text('keep')
            target.symlink_to(other,target_is_directory=True)
            with self.assertRaises(ValueError):controller.cleanup(env)
            self.assertEqual((other/'keep').read_text(),'keep')
            target.unlink();controller.stage(env)
            path=target/'inputs.json';path.chmod(0o644)
            with self.assertRaises(ValueError):controller.staged(env)

    def test_same_source_preview_then_separate_apply_has_one_cli_and_strict_post_baseline(self):
        with tempfile.TemporaryDirectory()as first,tempfile.TemporaryDirectory()as second:
            io_=DummyIO();env=environment(Path(first))
            with io_.patched():_,proof=preview(env,io_)
            apply=apply_environment(Path(second),proof);io_=DummyIO()
            with patch.object(controller,'APPLY_IMPLEMENTED',True),io_.patched(),\
                 patch.object(controller.provenance,'authenticate',return_value=(proof,{})):
                session=approved(apply,io_,proof);session.apply()
                self.assertEqual(session.write_attempts,1);self.assertTrue(session.state_verified);self.assertTrue(session.post_verified)
                self.assertEqual(sum(call[:2]==('terraform','apply')for call in io_.calls),1)
                self.assertTrue((session.root/'apply-attempt.json').is_file())
                self.assertIn('SYNC_POST_BASELINE=IMPORT0_NOOP12_DRIFT0',controller.render_outcome(session))
                other=controller.Session(apply)
                with self.assertRaises(ValueError):other.apply()
                self.assertEqual(sum(call[:2]==('terraform','apply')for call in io_.calls),1)

    def test_unknown_cli_result_reserves_once_and_never_retries_or_claims_post_state(self):
        with tempfile.TemporaryDirectory()as first,tempfile.TemporaryDirectory()as second:
            io_=DummyIO()
            with io_.patched():_,proof=preview(environment(Path(first)),io_)
            env=apply_environment(Path(second),proof);io_=DummyIO();io_.fail_apply=True
            with patch.object(controller,'APPLY_IMPLEMENTED',True),io_.patched(),\
                 patch.object(controller.provenance,'authenticate',return_value=(proof,{})):
                session=approved(env,io_,proof)
                with self.assertRaises(TimeoutError):session.apply()
                self.assertFalse(session.state_verified);self.assertFalse(session.post_verified)
                with self.assertRaises(ValueError):controller.Session(env).apply()
            self.assertEqual(sum(call[:2]==('terraform','apply')for call in io_.calls),1)
            self.assertNotIn(CANARY,'\n'.join(controller.render_outcome(session)))

    def test_preview_intent_changed_B_or_state_version_stops_before_apply(self):
        for mutation in ('intent','metadata'):
            with tempfile.TemporaryDirectory()as first,tempfile.TemporaryDirectory()as second:
                io_=DummyIO()
                with io_.patched():_,proof=preview(environment(Path(first)),io_)
                env=apply_environment(Path(second),proof);io_=DummyIO()
                if mutation=='intent':proof['commitments']['intent']='f'*64
                else:io_.metadata['etag']='DUMMY_C'
                with patch.object(controller,'APPLY_IMPLEMENTED',True),io_.patched(),\
                     patch.object(controller.provenance,'authenticate',return_value=(proof,{})):
                    session=approved(env,io_,proof)
                    with self.assertRaises(ValueError):session.apply()
                self.assertFalse(any(call[:2]==('terraform','apply')for call in io_.calls))

    def test_expiry_or_backend_change_after_durable_reservation_sends_zero(self):
        for mutation in ('expire','backend','source','auxiliary','binary'):
            with tempfile.TemporaryDirectory()as first,tempfile.TemporaryDirectory()as second:
                io_=DummyIO()
                with io_.patched():_,proof=preview(environment(Path(first)),io_)
                env=apply_environment(Path(second),proof);io_=DummyIO();original=controller.journal
                with patch.object(controller,'APPLY_IMPLEMENTED',True),io_.patched(),\
                     patch.object(controller.provenance,'authenticate',return_value=(proof,{})):
                    session=approved(env,io_,proof)
                    def delayed(root,stage,**kwargs):
                        original(root,stage,**kwargs)
                        if kwargs.get('attempted'):
                            if mutation=='expire':session.env['DEV_HISTORY_OBSERVATION_SYNC_EXPIRES_UTC']=DUMMY_ISSUED_UTC
                            elif mutation=='backend':(root/'backend.hcl').write_bytes(b'DUMMY_CHANGED')
                            elif mutation=='source':io_.bad_head=True
                            elif mutation=='auxiliary':
                                path=next(path for path in session.closure if path.name=='schema.json')
                                (root/'checkout'/path).write_bytes(b'DUMMY_CHANGED')
                            else:(root/'refresh.tfplan').write_bytes(b'DUMMY_CHANGED')
                    with patch.object(controller,'journal',side_effect=delayed):
                        with self.assertRaises(ValueError):session.apply()
                self.assertTrue((session.root/'apply-attempt.json').exists());self.assertEqual(session.write_attempts,1)
                self.assertFalse(any(call[:2]==('terraform','apply')for call in io_.calls))

    def test_native_stdio_emergency_dump_is_private_and_cleanup_removes_all_failure_files(self):
        with tempfile.TemporaryDirectory()as temp:
            root=Path(temp);env=environment(root);io_=DummyIO();io_.bind(env);controller.stage(env)
            private=controller.location(env)
            def fail(args,**kwargs):
                kwargs['stdout'].write(CANARY.encode());kwargs['stderr'].write(CANARY.encode())
                raise TimeoutError(CANARY)
            with patch.object(controller.subprocess,'run',side_effect=fail),patch('sys.stdout',io.StringIO())as public:
                with self.assertRaises(TimeoutError):controller.terraform(private,private,env,'failure',['apply'])
                self.assertNotIn(CANARY,public.getvalue())
            self.assertEqual(stat.S_IMODE((private/'failure.out').stat().st_mode),0o600)
            for name in ('errored.tfstate','errored.tfstate.backup','provider.log'):
                (private/name).write_text(CANARY)
            controller.cleanup(env);self.assertFalse(private.exists())

    def test_workflow_isolated_jobs_bounded_window_owner_environments_and_typed_upload(self):
        text=(controller.ROOT/'.github/workflows/gcp-terraform-authenticated.yml').read_text()
        old=text.split('\n  history_observation_sync_plan:',1)[0]
        self.assertIn("inputs.mode != 'dev-history-observation-sync'",old)
        source=dict(re.findall(r'(?m)^  ([A-Z0-9_]+): "([^"]*)"$',text))
        for gate_ in ('DEV_HISTORY_OBSERVATION_SYNC_ENABLED','DEV_HISTORY_OBSERVATION_SYNC_APPLY_ENABLED'):
            self.assertEqual(source[gate_],'true')
        issued=source['DEV_HISTORY_OBSERVATION_SYNC_ISSUED_UTC']
        expires=source['DEV_HISTORY_OBSERVATION_SYNC_EXPIRES_UTC']
        self.assertEqual(issued,'2026-10-16T04:29:31Z')
        self.assertEqual(expires,'2026-10-17T04:29:31Z')
        start,end=(controller.profiles.timestamp(value)for value in (issued,expires))
        self.assertEqual(end-start,timedelta(hours=24))
        # Evaluate immutable source-time semantics with an explicit test clock;
        # a later CI date must neither extend nor invalidate this source test.
        for operation in ('preview','synchronize'):
            env=requests.dummy_env(operation);env.update(source)
            profile=json.loads(env['HISTORY_OBSERVATION_SYNC_PROFILE'])
            profile.update(issued_utc=issued,expires_utc=expires)
            env['HISTORY_OBSERVATION_SYNC_PROFILE']=json.dumps(profile)
            self.assertEqual(controller.profiles.context(env,phase=operation,now=start)['operation'],operation)
            for instant in (start-timedelta(seconds=1),end):
                with self.assertRaises(ValueError):controller.profiles.context(env,phase=operation,now=instant)
            with self.assertRaises(ValueError):
                controller.profiles.context(env|{'DEV_HISTORY_OBSERVATION_SYNC_ENABLED':'false'},phase=operation,now=start)
            if operation=='synchronize':
                with self.assertRaises(ValueError):
                    controller.profiles.context(env|{'DEV_HISTORY_OBSERVATION_SYNC_APPLY_ENABLED':'false'},phase=operation,now=start)

        for role in ('plan','apply'):
            block=text.split('\n  history_observation_sync_'+role+':',1)[1].split('\n  history_observation_sync_',1)[0]
            self.assertIn('environment: terraform-dev-'+role,block)
            self.assertIn('needs.preflight.outputs.sync_operation',block)
            self.assertIn('persist-credentials: false',block);self.assertIn('if: ${{ always() }}',block)
            self.assertNotIn('continue-on-error',block);self.assertNotIn('-auto-approve',block)
        preview_block=text.split('\n  history_observation_sync_plan:',1)[1].split('\n  history_observation_sync_apply:',1)[0]
        positions=[preview_block.index('name: '+name)for name in controller.provenance.REQUIRED_STEPS if not name.startswith('Post ')]
        self.assertEqual(positions,sorted(positions))
        self.assertEqual(preview_block.count('uses: actions/upload-artifact@'),1)
        self.assertIn('retention-days: 1',preview_block)
        self.assertIn('history-observation-sync-public/history-observation-sync-preview.json',preview_block)
        self.assertNotIn('upload-artifact',text.split('\n  history_observation_sync_apply:',1)[1])
        import test_terraform_authenticated_workflow as baseline
        baseline.TerraformAuthenticatedWorkflowTest.setUpClass()
        fixture_=baseline.TerraformAuthenticatedWorkflowTest()
        for mode in ('plan','apply','current-state-read','runtime-metadata-read','security-probe'):
            result=fixture_.run_preflight(MODE=mode,HISTORY_OBSERVATION_SYNC_PROFILE=CANARY)
            self.assertNotEqual(result.returncode,0);self.assertNotIn(CANARY,result.stdout+result.stderr)

    def test_known_typed_binding_reuses_nonce_and_unknown_packet_never_stages(self):
        from terraform_current_state_attestation import TYPE
        for typed in (False,True):
            with tempfile.TemporaryDirectory()as temp:
                env=environment(Path(temp));io_=DummyIO();io_.bind(env)
                value=json.loads(env['RUNTIME_OWNERSHIP_PACKET_JSON'])
                if typed:value['attestation_type']=TYPE
                env['RUNTIME_OWNERSHIP_PACKET_JSON']=json.dumps(value);controller.stage(env)
                self.assertEqual(controller.staged(env)[2],value['receipt_binding'])
        with tempfile.TemporaryDirectory()as temp:
            env=environment(Path(temp));io_=DummyIO();io_.bind(env)
            value=json.loads(env['RUNTIME_OWNERSHIP_PACKET_JSON']);value['extra']=CANARY
            env['RUNTIME_OWNERSHIP_PACKET_JSON']=json.dumps(value)
            with self.assertRaises(ValueError):controller.stage(env)
            self.assertFalse(controller.location(env).exists())

    def test_identical_version_after_cli_and_post_extra_actions_cannot_complete(self):
        with tempfile.TemporaryDirectory()as first,tempfile.TemporaryDirectory()as second:
            io_=DummyIO()
            with io_.patched():_,proof=preview(environment(Path(first)),io_)
            env=apply_environment(Path(second),proof);io_=DummyIO();io_.bump_version=False
            with io_.patched(),patch.object(controller.provenance,'authenticate',return_value=(proof,{})):
                session=approved(env,io_,proof)
                with self.assertRaises(ValueError):session.apply()
            self.assertEqual(session.cli_calls,1);self.assertFalse(session.state_verified);self.assertFalse(session.post_verified)
        parts=fixtures.data();post=copy.deepcopy(parts[0]);post['resource_drift']=[]
        for mutate in (lambda p:p.update(complete=1),lambda p:p.update(format_version='OTHER'),
            lambda p:p.update(extra=CANARY),lambda p:p.update(action_invocations=[{}]),
            lambda p:p['output_changes']['environment'].update(after=CANARY)):
            bad=copy.deepcopy(post);mutate(bad)
            with self.assertRaises(ValueError):controller.validate_post(bad,parts[3],parts[1],fixtures.fixture.legacy_fixture.fixture.EMAIL,parts[-1])

    def test_unhandled_tracked_tf_json_is_rejected_instead_of_silently_omitted(self):
        response=subprocess.CompletedProcess([],0,b'infra/gcp/environments/dev/unhandled.tf.json\n',b'')
        with patch.object(controller.subprocess,'run',return_value=response):
            with self.assertRaises(ValueError):controller.trusted_closure()

    def test_final_source_io_crossing_expiry_or_clockback_reserves_but_sends_zero(self):
        real_datetime=controller.datetime
        for boundary in ('expiry','before-issued'):
            class Clock(real_datetime):
                current=real_datetime.now(controller.timezone.utc)
                @classmethod
                def now(cls,tz=None):return cls.current
            with tempfile.TemporaryDirectory()as first,tempfile.TemporaryDirectory()as second:
                io_=DummyIO()
                with io_.patched():_,proof=preview(environment(Path(first)),io_)
                env=apply_environment(Path(second),proof);io_=DummyIO()
                with io_.patched(),patch.object(controller.provenance,'authenticate',return_value=(proof,{})),\
                     patch.object(controller.profiles,'datetime',Clock):
                    session=approved(env,io_,proof);git=io_.git;count=[0]
                    def delayed(args,**kwargs):
                        result=git(args,**kwargs)
                        if args[:2]==['git','diff-index']and(session.root/'apply-attempt.json').exists():
                            count[0]+=1
                            # The second post-reservation seal is the final
                            # before_send callback, after private stdout opens.
                            if count[0]==2:
                                Clock.current=controller.profiles.timestamp(session.profile['expires_utc'])if boundary=='expiry'else\
                                    controller.profiles.timestamp(session.profile['issued_utc'])-timedelta(seconds=1)
                        return result
                    with patch.object(controller.subprocess,'run',side_effect=delayed),patch('sys.stdout',io.StringIO())as out:
                        with self.assertRaises(ValueError):session.apply()
                        for line in controller.render_outcome(session):print(line)
                    self.assertEqual(count[0],2);self.assertEqual(session.write_attempts,1);self.assertEqual(session.cli_calls,0)
                    self.assertNotIn(CANARY,out.getvalue());self.assertIn('SYNC_CLI_CALL_ATTEMPTS=0',out.getvalue())
                    self.assertFalse(any(call[:2]==('terraform','apply')for call in io_.calls))


if __name__=='__main__':unittest.main()
