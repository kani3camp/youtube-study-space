#!/usr/bin/env python3
"""Separate default-off dev observation protocol; public output is finite only.

Raw Terraform output (including EmergencyDumpState), State, plan archives,
provider logs and config snapshots stay in one owned private working tree.
This protocol never issues a historical/current-state root receipt.
"""
from __future__ import annotations
import base64
import copy
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import time
import urllib.request

import terraform_history_observation_sync_profile as profiles
import terraform_history_observation_sync_provenance as provenance
import terraform_history_plan_receipt as private
import terraform_history_post_noop_receipt as post_noop
import terraform_identity_smoke as identity
import terraform_ownership_receipt as ownership
from terraform_history_table_metadata import stable_table_metadata

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'infra/gcp/scripts'))
import validate_history_observation_sync as gate
import history_observation_sync_archive as archive
import history_observation_sync_checks as checks
from prepare_user_activity_history_adoption import prepare
from prepare_user_activity_history_workflow import PrivateArgumentParser, TABLE_PATH

PRIVATE_DIRECTORY = 'history-observation-sync-private'
APPLY_IMPLEMENTED = True  # Compiled capability only; source gates remain off.
FRESH_SECONDS = 60
MAX_LOG_BYTES = 16 * 1024 * 1024
MODULES = {
    'backup_bucket':'retained-backup-bucket','firestore_export_dataset':'retained-bigquery-dataset',
    'order_history':'retained-order-history','notification_channels':'monitoring-notification-channels',
    'youtube_quota_alerts':'youtube-quota-alerts','export_topic':'firestore-export-topic',
    'export_scheduler':'firestore-export-scheduler','export_function':'firestore-export-function',
    'runtime_wif':'runtime-aws-wif','owned_apis':'owned-project-apis','user_activity_history':'retained-user-activity-history',
}
COUNTED_MODULES = frozenset(('export_topic','export_scheduler','export_function','user_activity_history'))
STAGES = frozenset(('PROFILE','BINDING','SOURCE','IDENTITY','METADATA','STATE_BEFORE','INIT','OBSERVATION',
    'CANDIDATE','ARCHIVE','FRESH_RECHECK','PREVIEW','APPROVAL','SEAL','APPLY','STATE_AFTER','POST_BASELINE','CLEANUP'))


def need(value):
    if not value: raise ValueError('sync protocol STOP')


def encoded(value): return gate.canonical(value).encode()


def now(): return datetime.now(timezone.utc)


def context(env,operation=None):
    operation = operation or env.get('HISTORY_OBSERVATION_SYNC_OPERATION')
    profile = profiles.context(env,phase=operation)
    role = 'plan' if operation == 'preview' else 'apply'
    need(env.get('GITHUB_JOB') == 'history_observation_sync_'+role)
    need(env.get('TF_WORKSPACE') == 'default' and env.get('STATE_KEY') == private.STATE_KEY
        and env.get('STATE_AWS_REGION') == 'ap-northeast-1' and env.get('AWS_MAX_ATTEMPTS') == '1'
        and env.get('AWS_RETRY_MODE') == 'standard')
    need(env.get('TF_VAR_project_id') == gate.PROJECT)
    allowed={'TF_VAR_project_id','TF_VAR_primary_email_address','TF_VAR_primary_email_channel_name',
        'TF_VAR_export_function_execution_service_account_email'}|{'TF_VAR_manage_'+key for key in profiles.INPUT_FLAGS}
    need(all(key in allowed for key in env if key.startswith('TF_VAR_')))
    if operation == 'synchronize': need(APPLY_IMPLEMENTED)
    return profile


def owned_directory(path):
    need(path.is_absolute() and path.resolve() == path and not path.is_symlink())
    info=path.stat()
    need(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid() and stat.S_IMODE(info.st_mode)==0o700)
    return path


def location(env):
    parent=Path(env['RUNNER_TEMP'])
    need(parent.is_absolute() and parent.resolve()==parent and parent.is_dir() and not parent.is_symlink()
        and parent.stat().st_uid==os.geteuid() and not parent.is_relative_to(ROOT))
    return parent/PRIVATE_DIRECTORY


def read_owned(path,maximum=gate.MAX_BYTES):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW)
    with os.fdopen(fd,'rb') as stream:
        info=os.fstat(stream.fileno())
        need(path.resolve()==path and stat.S_ISREG(info.st_mode) and info.st_uid==os.geteuid() and info.st_nlink==1
            and stat.S_IMODE(info.st_mode)==0o600 and 0 < info.st_size <= maximum)
        raw=stream.read(maximum+1)
        need(len(raw)==info.st_size)
    return raw


def write_owned(path,raw):
    need(type(raw) is bytes and len(raw)<=MAX_LOG_BYTES)
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    with os.fdopen(fd,'wb') as stream:
        stream.write(raw);stream.flush();os.fsync(stream.fileno())


def journal(root,stage,*,attempted=False):
    need(stage in STAGES and type(attempted) is bool)
    path=root/'journal.jsonl'
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_APPEND|os.O_NOFOLLOW,0o600)
    with os.fdopen(fd,'ab') as stream:
        info=os.fstat(stream.fileno())
        need(info.st_uid==os.geteuid() and info.st_nlink==1 and stat.S_ISREG(info.st_mode)
            and stat.S_IMODE(info.st_mode)==0o600 and info.st_size<8192)
        stream.write(encoded(dict(stage=stage,utc=now().strftime('%Y-%m-%dT%H:%M:%SZ'),write_attempt=attempted))+b'\n')
        stream.flush();os.fsync(stream.fileno())


def staged(env):
    profile=context(env);root=owned_directory(location(env))
    value=private.decode(read_owned(root/'inputs.json'))
    need(set(value)=={'profile','binding'} and value['profile']==profile)
    ownership.check_current_binding(value['binding'],env)
    return root,profile,value['binding']


def preflight(env):
    value=profiles.validate_profile(env.get('HISTORY_OBSERVATION_SYNC_PROFILE',''),env.get('GITHUB_SHA'))
    env=dict(env);env['HISTORY_OBSERVATION_SYNC_OPERATION']=value['operation']
    profiles.context(env,phase=value['operation'])
    if value['operation']=='synchronize':need(APPLY_IMPLEMENTED)
    fd=os.open(env['GITHUB_OUTPUT'],os.O_WRONLY|os.O_APPEND|os.O_NOFOLLOW)
    with os.fdopen(fd,'w') as stream:
        info=os.fstat(stream.fileno());need(stat.S_ISREG(info.st_mode)and info.st_uid==os.geteuid())
        stream.write('sync_operation='+value['operation']+'\n')


def stage(env):
    profile=context(env)
    raw=env.get('RUNTIME_OWNERSHIP_PACKET_JSON','')
    need(type(raw)is str and 0<len(raw.encode())<=gate.MAX_BYTES)
    value=ownership.decode(raw)
    need(type(value)is dict and type(value.get('schema_version'))is int and value['schema_version']==1)
    if set(value)=={'schema_version','attestation_type','receipt_binding'}:
        from terraform_current_state_attestation import validate_binding_envelope
        validate_binding_envelope(value)
    else:need(set(value)=={'schema_version','receipt_binding'})
    ownership.check_current_binding(value['receipt_binding'],env)
    proof=None
    if profile['operation']=='synchronize':
        proof,_=provenance.authenticate(profile['preview_reference'],profile,request=provenance.GitHubRead(env['GH_TOKEN']))
    root=location(env);root.mkdir(mode=0o700)
    owned_directory(root)
    write_owned(root/'inputs.json',encoded(dict(profile=profile,binding=value['receipt_binding'])))
    if proof is not None:write_owned(root/'approved-preview.json',encoded(proof))
    journal(root,'BINDING')


def verify_oidc(env):
    staged(env)
    role='plan' if env['HISTORY_OBSERVATION_SYNC_OPERATION']=='preview' else 'apply'
    need(env['BACKEND_ROLE_ARN'].endswith('-'+role))
    request=urllib.request.Request(env['ACTIONS_ID_TOKEN_REQUEST_URL']+'&audience=sts.amazonaws.com',
        headers={'Authorization':'Bearer '+env['ACTIONS_ID_TOKEN_REQUEST_TOKEN']})
    with urllib.request.urlopen(request,timeout=15) as response:
        raw=response.read(32769);need(len(raw)<=32768)
    token=private.decode(raw)['value']
    need(type(token) is str and len(token)<=16384)
    payload=token.split('.')[1]
    claims=private.decode(base64.urlsafe_b64decode(payload+'='*(-len(payload)%4)))
    identity.verify_oidc_claims(claims,env,expected_identity=role)


def verify_aws(env):
    staged(env);identity.verify_aws_identity(env,require_role_id=True)


def trusted_closure():
    result=subprocess.run(['git','ls-tree','-r','--name-only','HEAD','--',
        'infra/gcp/environments/dev','infra/gcp/modules'],cwd=ROOT,capture_output=True,timeout=10)
    need(result.returncode==0 and len(result.stdout)<=131072)
    closure={}
    allowed={Path('infra/gcp/environments/dev')}|{Path('infra/gcp/modules')/name for name in MODULES.values()}
    for name in result.stdout.decode().splitlines():
        path=Path(name)
        direct=path.parent in allowed and (path.suffix=='.tf' or path.name in ('schema.json','.terraform.lock.hcl'))
        query=path.parent.name=='queries' and path.parent.parent in allowed and path.suffix=='.tftpl'
        if path.parent in allowed and path.name.endswith('.tf.json'):raise ValueError('unsupported source closure')
        if not (direct or query):continue
        source=ROOT/path;need(source.resolve()==source and source.is_file())
        info=source.stat();need(stat.S_ISREG(info.st_mode)and info.st_uid==os.geteuid()and info.st_nlink==1 and not info.st_mode&0o022)
        for parent in (source.parent,*source.parents):
            if not parent.is_relative_to(ROOT):continue
            info=parent.stat();need(stat.S_ISDIR(info.st_mode)and info.st_uid==os.geteuid()and not info.st_mode&0o022)
        blob=subprocess.run(['git','show','HEAD:'+name],cwd=ROOT,capture_output=True,timeout=10)
        need(blob.returncode==0 and len(blob.stdout)<=gate.MAX_BYTES and source.read_bytes()==blob.stdout)
        closure[path]=blob.stdout
    need(bool(closure) and Path('infra/gcp/environments/dev/.terraform.lock.hcl')in closure)
    return closure


def source_catalog(closure=None):
    closure=trusted_closure()if closure is None else closure
    configs={('resource',re.sub(r'\[(?:\d+|"[^\"]*")\]','',address)) for _,address,_ in gate.ALIASES}
    directories=[('',Path('infra/gcp/environments/dev'))]+[
        ('module.'+name+'.',Path('infra/gcp/modules')/directory) for name,directory in MODULES.items()]
    for prefix,directory in directories:
        files=[raw for path,raw in closure.items()if path.parent==directory and path.suffix=='.tf'];need(bool(files))
        for raw in files:
            for keyword,name in re.findall(r'(?m)^\s*(variable|output|check)\s+"([a-zA-Z0-9_]+)"\s*\{',raw.decode()):
                kind='var'if keyword=='variable'else keyword
                configs.add((kind,prefix+kind+'.'+name))
    modules={''}|{'module.'+name+('[0]'if name in COUNTED_MODULES else '')for name in MODULES}
    return checks.CheckCatalog(configs=frozenset(configs),zero_instances=private.ZERO_INSTANCE_CHECKS,
        module_instances=frozenset(modules),resource_instances=frozenset(address for _,address,_ in gate.ALIASES))


def copy_sources(root,closure):
    target=root/'checkout';target.mkdir(mode=0o700)
    for path,raw in closure.items():
        destination=target/path
        destination.parent.mkdir(mode=0o700,parents=True,exist_ok=True)
        write_owned(destination,raw)
    return target/'infra/gcp/environments/dev'


def source_snapshot(tfroot,members):
    # All sources are local and source-owned. The initialized module manifest
    # is checked against the fixed module set and exact copied source paths.
    manifest=archive.json_value(read_owned(tfroot/'.terraform/modules/modules.json'))
    need(type(manifest) is dict and set(manifest)=={'Modules'} and type(manifest['Modules']) is list)
    actual=archive.json_value(members['tfconfig/modules.json'])
    need(type(actual) is list)
    initialized={};snapshots={}
    for item in manifest['Modules']:
        need(type(item) is dict and {'Key','Dir'}<=set(item)<={'Key','Dir','Source'}
            and item['Key'] not in initialized and type(item['Dir']) is str)
        key=item['Key'];need(key=='' or key in MODULES)
        expected=tfroot if key=='' else tfroot.parents[1]/'modules'/MODULES[key]
        need((tfroot/item['Dir']).resolve()==expected.resolve())
        if key:need(item.get('Source')=='../../modules/'+MODULES[key])
        else:need('Source'not in item or item['Source']=='')
        initialized[key]={k:v for k,v in item.items() if k!='Source' or v!=''}
        for path in expected.glob('*.tf'):
            need(not path.is_symlink())
            if path.is_file():
                snapshots['tfconfig/m-'+key+'/'+str(path.relative_to(expected))]=read_owned(path)
    need(set(initialized)=={''}|set(MODULES))
    seen={}
    for item in actual:
        need(type(item)is dict and {'Key','Dir'}<=set(item)<={'Key','Dir','Source'} and item['Key'] not in seen)
        seen[item['Key']]={k:v for k,v in item.items() if k!='Source' or v!=''}
    need(seen==initialized)
    snapshots['tfconfig/modules.json']=members['tfconfig/modules.json']
    snapshots['.terraform.lock.hcl']=read_owned(tfroot/'.terraform.lock.hcl')
    return snapshots


def terraform(root,tfroot,env,name,args,*,codes=(0,),show=False,before_send=None,on_call=None):
    need(re.fullmatch('[a-z-]+',name) is not None)
    command=['terraform','-chdir='+str(tfroot),*args]
    child=dict(env)
    for key in tuple(child):
        if key.startswith(('TF_CLI_ARGS','TF_LOG')):child.pop(key)
    child.update(TF_IN_AUTOMATION='true',TF_INPUT='false',TF_WORKSPACE='default')
    out=root/(name+('.json' if show else '.out'));err=root/(name+'.err')
    outfd=os.open(out,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    try:errfd=os.open(err,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    except Exception:os.close(outfd);raise
    with os.fdopen(outfd,'wb') as stdout,os.fdopen(errfd,'wb') as stderr:
        if before_send is not None:before_send()
        if on_call is not None:on_call()
        result=subprocess.run(command,env=child,stdin=subprocess.DEVNULL,stdout=stdout,stderr=stderr,timeout=240)
    need(out.stat().st_size<=MAX_LOG_BYTES and err.stat().st_size<=MAX_LOG_BYTES and result.returncode in codes)
    return archive.json_value(read_owned(out,gate.MAX_PLAN_BYTES)) if show else result.returncode


class Session:
    def __init__(self,env):
        self.env=dict(env);self.root,self.profile,self.binding=staged(self.env)
        self.closure=trusted_closure();self.catalog=source_catalog(self.closure);self.stage='PROFILE';self.write_attempts=0;self.cli_calls=0;self.candidate_verified=False;self.state_verified=False;self.post_verified=False

    def check(self):
        root,profile,binding=staged(self.env)
        need(root==self.root and profile==self.profile and encoded(binding)==encoded(self.binding))

    def mark(self,stage):
        self.stage=stage;journal(self.root,stage);self.check()

    def metadata(self):
        self.check();token=self.env['GCP_SMOKE_ACCESS_TOKEN']
        status,value=identity.google(TABLE_PATH,token,host='bigquery.googleapis.com');need(status==200)
        stable=stable_table_metadata(value)
        need(type(value.get('etag')) is str and bool(value['etag'])
            and type(value.get('lastModifiedTime')) is str and re.fullmatch('[0-9]+',value['lastModifiedTime']))
        ownership.check_current_binding(self.binding,self.env,value)
        need(encoded(stable_table_metadata(self.binding['history_metadata']))==encoded(stable))
        return dict(stable=stable,etag=value['etag'],last_modified_time=int(value['lastModifiedTime'])),value

    def snapshot(self,name,metadata):
        self.check()
        head,raw=post_noop.snapshot(self.env,self.root,name+'.json',metadata,gate.MAX_BYTES)
        gate._state(raw);private.checks_pass(private.decode(raw).get('check_results'),imported=True)
        return head,raw

    def function(self):
        path='v1/projects/'+gate.PROJECT+'/locations/asia-southeast2/functions/firestoreCollectionsExport'
        status,value=identity.google(path,self.env['GCP_SMOKE_ACCESS_TOKEN'],host='cloudfunctions.googleapis.com')
        need(status==200 and value.get('name')==path[3:] and value.get('runtime')=='nodejs22'
            and value.get('status')=='ACTIVE' and value.get('versionId')=='8'
            and value.get('serviceAccountEmail')==gate.PROJECT+'@appspot.gserviceaccount.com')
        return value

    def project(self):
        role='plan'if self.profile['operation']=='preview'else'apply'
        need(self.env['GCP_SMOKE_SERVICE_ACCOUNT']=='terraform-dev-'+role+'@'+gate.PROJECT+'.iam.gserviceaccount.com')
        status,value=identity.google('v3/projects/'+gate.PROJECT,self.env['GCP_SMOKE_ACCESS_TOKEN'])
        need(status==200 and value.get('projectId')==gate.PROJECT and value.get('state')=='ACTIVE')

    def build(self):
        self.mark('SOURCE')
        result=subprocess.run(['git','rev-parse','HEAD'],cwd=ROOT,capture_output=True,timeout=10)
        need(result.returncode==0 and result.stdout== (self.env['GITHUB_SHA']+'\n').encode())
        self.mark('IDENTITY');self.project()
        self.mark('METADATA');self.meta,self.fullmeta=self.metadata();self.function_before=self.function()
        self.email=self.function_before['serviceAccountEmail']
        self.env['TF_VAR_export_function_execution_service_account_email']=self.email
        self.mark('STATE_BEFORE');self.head,self.raw=self.snapshot('state-before',self.fullmeta)
        self.tfroot=copy_sources(self.root,self.closure)
        inputs={k:v for k,v in prepare(self.fullmeta).items()if k!='manage_user_activity_history'}
        self.history_inputs=encoded(inputs)
        write_owned(self.root/'history.tfvars.json',self.history_inputs)
        backend=self.binding['backend']
        text='\n'.join([f'bucket = {json.dumps(backend["bucket"])}',f'key = {json.dumps(backend["key"])}',
            'region = "ap-northeast-1"','encrypt = true','use_lockfile = true','max_retries = 1',
            'workspace_key_prefix = "youtube-study-space/dev/workspaces"',
            'allowed_account_ids = ['+json.dumps(backend['account_id'])+']'])+'\n'
        write_owned(self.root/'backend.hcl',text.encode())
        self.backend_bytes=read_owned(self.root/'backend.hcl')
        self.mark('INIT')
        terraform(self.root,self.tfroot,self.env,'init',['init','-reconfigure','-input=false','-lockfile=readonly','-backend-config='+str(self.root/'backend.hcl')])
        self.backend_cache=read_owned(self.tfroot/'.terraform/terraform.tfstate')
        self.mark('OBSERVATION')
        common=['-input=false','-lock-timeout=0s','-detailed-exitcode','-var-file='+str(self.root/'history.tfvars.json')]
        terraform(self.root,self.tfroot,self.env,'normal-plan',['plan',*common,'-out='+str(self.root/'normal.tfplan')],codes=(0,))
        self.normal=terraform(self.root,self.tfroot,self.env,'normal-show',['show','-json',str(self.root/'normal.tfplan')],show=True)
        self.observation=gate.validate_observation(self.normal,metadata=self.fullmeta,execution_email=self.email,source_sha=self.env['GITHUB_SHA'])
        self.mark('CANDIDATE')
        terraform(self.root,self.tfroot,self.env,'refresh-plan',['plan','-refresh-only',*common,'-out='+str(self.root/'refresh.tfplan')],codes=(2,))
        self.refresh=terraform(self.root,self.tfroot,self.env,'refresh-show',['show','-json',str(self.root/'refresh.tfplan')],show=True)
        gate.validate_refresh_candidate(self.refresh,self.normal,metadata=self.fullmeta,execution_email=self.email,source_sha=self.env['GITHUB_SHA'])
        self.mark('ARCHIVE')
        self.binary=read_owned(self.root/'refresh.tfplan',archive.MAX_ARCHIVE_BYTES)
        members=archive.read_archive(self.binary)
        self.expected=archive.inspect_candidate(self.binary,self.raw,self.normal,self.refresh,
            expected_snapshot=source_snapshot(self.tfroot,members),catalog=self.catalog)
        need(self.meta['etag']==gate._observation(self.normal)[1]['after']['etag']
            and self.meta['last_modified_time']==gate._observation(self.normal)[1]['after']['last_modified_time'])
        self.candidate_verified=True
        self.mark('FRESH_RECHECK');self.recheck('after-plan')
        scope=dict(profile=profiles.shared_scope(self.profile),backend=self.binding['backend'])
        values=dict(scope=scope,state_before=dict(head=self.head,raw_sha256=hashlib.sha256(self.raw).hexdigest()),
            state_expected=archive.normalized_state(self.expected,catalog=self.catalog),metadata=self.meta,
            intent=dict(source_sha=self.env['GITHUB_SHA'],terraform_version=gate.TERRAFORM_VERSION,
                provider_version=gate.PROVIDER_VERSION,variables=self.normal.get('variables'),
                configuration=self.normal.get('configuration'),function=self.function_before))
        self.commitments={key:provenance.commitment(self.binding,key,value)for key,value in values.items()}
        self.seal=hashlib.sha256(self.binary).digest()
        self.environment_seal=encoded(self.env)
        return self.commitments

    def recheck(self,name):
        started=time.monotonic();self.check()
        head,raw=self.snapshot(name,self.fullmeta);meta,_=self.metadata()
        need(encoded(head)==encoded(self.head) and raw==self.raw and encoded(meta)==encoded(self.meta)
            and encoded(self.function())==encoded(self.function_before))
        self.check();need(time.monotonic()-started<=FRESH_SECONDS)
        self.last_read=time.monotonic()

    def preview(self):
        self.build();self.mark('PREVIEW')
        proof=dict(schema_version=1,kind=provenance.TYPE,repository_id=340900071,source_sha=self.env['GITHUB_SHA'],
            run_id=int(self.env['GITHUB_RUN_ID']),run_attempt=1,job_role='plan',environment='terraform-dev-plan',
            target='dev',root_contract='gcp-dev-default',request_id=self.profile['request_id'],
            issued_utc=self.profile['issued_utc'],expires_utc=self.profile['expires_utc'],observed_utc=now().strftime('%Y-%m-%dT%H:%M:%SZ'),
            counts=dict(gate.COUNTS),drift_alias='USER_ACTIVITY_HISTORY',field_codes=['ETAG','LAST_MODIFIED_TIME'],
            commitments=self.commitments,raw_candidate_state_verified=True,execution_permitted=False,migration_ready=False,
            checks=dict.fromkeys(provenance.CHECKS,True),accounting=dict(provider_requests='unknown',actual_cost='unknown'))
        provenance.validate_proof(proof,self.profile)
        directory=Path(self.env['RUNNER_TEMP'])/provenance.PUBLIC_DIRECTORY
        directory.mkdir(mode=0o700);owned_directory(directory)
        write_owned(directory/provenance.PUBLIC_FILE,encoded(proof)+b'\n')

    def check_seal(self):
        self.check()
        need(hashlib.sha256(read_owned(self.root/'refresh.tfplan',archive.MAX_ARCHIVE_BYTES)).digest()==self.seal)
        for path,value in self.closure.items():
            need(read_owned(self.root/'checkout'/path)==value)
        checkout=self.root/'checkout'
        for directory,names,files in os.walk(checkout,followlinks=False):
            directory=Path(directory)
            for name in tuple(names):
                child=directory/name
                need(not child.is_symlink())
                if child==self.tfroot/'.terraform':names.remove(name)
            for name in files:
                path=directory/name;relative=path.relative_to(checkout)
                need(relative in self.closure and read_owned(path)==self.closure[relative])
        need(read_owned(self.root/'history.tfvars.json')==self.history_inputs
            and read_owned(self.root/'backend.hcl')==self.backend_bytes
            and read_owned(self.tfroot/'.terraform/terraform.tfstate')==self.backend_cache)
        need(encoded(self.env)==self.environment_seal)
        head=subprocess.run(['git','rev-parse','HEAD'],cwd=ROOT,capture_output=True,timeout=10)
        need(head.returncode==0 and head.stdout==(self.env['GITHUB_SHA']+'\n').encode()
            and trusted_closure()==self.closure)
        source=subprocess.run(['git','diff-index','--quiet','HEAD','--'],cwd=ROOT,capture_output=True,timeout=10)
        need(source.returncode==0 and time.monotonic()-self.last_read<=FRESH_SECONDS)
        # Source/git/file IO above can cross an immutable UTC boundary. The
        # final admission is pure and introduces no new IO before CLI send.
        need(encoded(context(self.env))==encoded(self.profile))

    def call_reserved(self):
        need(self.write_attempts==1 and self.cli_calls==0)
        self.cli_calls+=1

    def apply(self):
        need(APPLY_IMPLEMENTED and not (self.root/'apply-attempt.json').exists())
        self.mark('APPROVAL')
        approved=private.decode(read_owned(self.root/'approved-preview.json'))
        current,_=provenance.authenticate(self.profile['preview_reference'],self.profile,
            request=provenance.GitHubRead(self.env['GH_TOKEN']))
        need(encoded(approved)==encoded(current))
        self.build()
        need(encoded(approved['commitments'])==encoded(self.commitments))
        self.mark('SEAL');self.recheck('before-apply');self.check_seal()
        # Reserve the sole CLI invocation durably before send. The final check
        # follows fsync; timeout/unknown result never retries or rolls back.
        need(self.write_attempts==0)
        write_owned(self.root/'apply-attempt.json',encoded(dict(cli_attempt=1,source_sha=self.env['GITHUB_SHA'])))
        directory_fd=os.open(self.root,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
        try:os.fsync(directory_fd)
        finally:os.close(directory_fd)
        self.write_attempts+=1
        journal(self.root,'APPLY',attempted=True)
        self.stage='APPLY';self.check_seal()
        terraform(self.root,self.tfroot,self.env,'apply',
            ['apply','-input=false','-lock-timeout=0s',str(self.root/'refresh.tfplan')],before_send=self.check_seal,on_call=self.call_reserved)
        self.mark('STATE_AFTER')
        final_head,final_raw=self.snapshot('state-after',self.fullmeta)
        archive.verify_after(final_raw,self.expected,catalog=self.catalog)
        need(final_head['VersionId']!=self.head['VersionId'])
        self.state_verified=True
        meta,_=self.metadata();need(encoded(meta)==encoded(self.meta))
        self.mark('POST_BASELINE')
        common=['-input=false','-lock-timeout=0s','-detailed-exitcode','-var-file='+str(self.root/'history.tfvars.json')]
        terraform(self.root,self.tfroot,self.env,'post-plan',['plan',*common,'-out='+str(self.root/'post.tfplan')],codes=(0,))
        post=terraform(self.root,self.tfroot,self.env,'post-show',['show','-json',str(self.root/'post.tfplan')],show=True)
        validate_post(post,self.expected,self.fullmeta,self.email,self.catalog)
        last_head,last_raw=self.snapshot('state-after-post',self.fullmeta)
        need(encoded(last_head)==encoded(final_head)and last_raw==final_raw)
        meta,_=self.metadata();need(encoded(meta)==encoded(self.meta))
        self.check();self.post_verified=True

    def failure_safety(self):
        # An observation failure never becomes success. Best-effort safety
        # reads may only establish bounded facts; raw diagnostics remain private.
        if not hasattr(self,'raw')or not hasattr(self,'fullmeta'):return
        try:
            self.snapshot('failure-safety',self.fullmeta)
            self.metadata()
        except Exception:pass


def validate_post(plan,expected,metadata,email,catalog):
    gate._bounded(plan)
    need(type(plan)is dict and set(plan)<=gate._PLAN_KEYS and plan.get('format_version')=='1.2'
        and plan.get('terraform_version')==gate.TERRAFORM_VERSION
        and plan.get('complete')is True and plan.get('errored')is False
        and plan.get('resource_drift',[])==[])
    need('applyable'not in plan or type(plan['applyable'])is bool)
    for key in ('deferred_changes','action_invocations','deferred_action_invocations'):
        need(type(plan.get(key,[]))is list and plan.get(key,[])==[])
    outputs=plan.get('output_changes');need(type(outputs)is dict and set(outputs)=={'environment','project_id'})
    for key,value in (('environment','development'),('project_id',gate.PROJECT)):
        change=outputs[key]
        need(type(change)is dict and {'actions','before','after'}<=set(change)<=
            {'actions','before','after','after_unknown','before_sensitive','after_sensitive'}
            and change['actions']==['no-op']and change['before']==change['after']==value
            and not gate.diagnostic._mask_true(change.get('after_unknown',False))
            and change.get('before_sensitive',False)is False and change.get('after_sensitive',False)is False)
    expected_counts=dict(gate.COUNTS);expected_counts['drift']=0
    from terraform_plan_diagnostic import counts
    need(counts(plan)==expected_counts)
    gate.validate_history(plan,metadata=metadata,phase='post',execution_email=email)
    changes=plan.get('resource_changes');need(type(changes)is list and len(changes)==12)
    _,attributes=gate._state(encoded(expected));seen=set()
    for item in changes:
        index,fields=gate.diagnostic._identity(item);need(index not in seen);seen.add(index)
        gate._change(item['change'],fields,'no-op')
        need(encoded(item['change']['before'])==encoded(item['change']['after'])==encoded(attributes[item['address']]))
    need(seen==set(range(12)))
    checks.shown_checks(plan.get('checks'),checks.raw_checks(expected.get('check_results'),catalog=catalog),catalog=catalog)



def render_outcome(session):
    # No response strings, paths, exception text, hashes or values enter this
    # renderer. Unobserved checks stay unobserved even after a failed CLI.
    need(type(session.write_attempts)is int and session.write_attempts in (0,1)
        and type(session.cli_calls)is int and session.cli_calls in (0,1)
        and all(type(value)is bool for value in (session.candidate_verified,session.state_verified,session.post_verified)))
    return [
        'SYNC_CLI_RESERVATIONS='+str(session.write_attempts),
        'SYNC_CLI_CALL_ATTEMPTS='+str(session.cli_calls),
        'SYNC_NATIVE_PUT_ATTEMPTS=UNKNOWN',
        'SYNC_EXPECTED_DRIFT='+('1'if session.candidate_verified else 'NOT_OBSERVED'),
        'SYNC_STATE_FIELDS=ETAG,LAST_MODIFIED_TIME',
        'SYNC_STATE_DELTA='+('TWO_LEAVES_SERIAL_PLUS_ONE_CHECK_CONTENTS_UNCHANGED'if session.state_verified else 'NOT_OBSERVED'),
        'SYNC_POST_BASELINE='+('IMPORT0_NOOP12_DRIFT0'if session.post_verified else 'NOT_OBSERVED'),
        'SYNC_ISSUER=false; migration_ready=false',
    ]


def cleanup(env):
    root=location(env)
    if root.exists():owned_directory(root);shutil.rmtree(root)
    need(not root.exists())


def publish_check(env):
    profile=context(env,'preview')
    need(not location(env).exists())
    path=owned_directory(Path(env['RUNNER_TEMP'])/provenance.PUBLIC_DIRECTORY)/provenance.PUBLIC_FILE
    provenance.validate_proof(private.decode(read_owned(path,8192)),profile)


def main(argv=None,env=None):
    env=dict(os.environ if env is None else env);phase='PROFILE';session=None
    previous_umask=os.umask(0o077)
    try:
        parser=PrivateArgumentParser();parser.add_argument('--phase',required=True,
            choices=('preflight','stage','oidc','aws','preview','apply','cleanup','cleanup-public','publish-check'))
        args=parser.parse_args(argv)
        if args.phase=='preflight':preflight(env)
        elif args.phase=='stage':stage(env)
        elif args.phase=='oidc':verify_oidc(env)
        elif args.phase=='aws':verify_aws(env)
        elif args.phase=='cleanup':phase='CLEANUP';cleanup(env)
        elif args.phase=='cleanup-public':
            location(env)
            directory=Path(env['RUNNER_TEMP'])/provenance.PUBLIC_DIRECTORY
            if directory.exists():owned_directory(directory);shutil.rmtree(directory)
        elif args.phase=='publish-check':publish_check(env)
        else:
            session=Session(env)
            try:getattr(session,args.phase)()
            except Exception:
                phase=session.stage;session.failure_safety();raise
    except Exception:
        os.umask(previous_umask)
        if session is not None:
            try:
                for line in render_outcome(session):print(line)
            except Exception:print('SYNC_OUTCOME=NOT_OBSERVED')
        print('HISTORY_OBSERVATION_SYNC: STOP; stage='+ (phase if phase in STAGES else 'PROFILE'))
        return 3
    os.umask(previous_umask)
    if session is not None:
        for line in render_outcome(session):print(line)
    print('HISTORY_OBSERVATION_SYNC: PASS; execution_permitted=false; migration_ready=false')
    return 0


if __name__=='__main__':raise SystemExit(main())
