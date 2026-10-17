#!/usr/bin/env python3
"""Invented full12 data for typed archive joins; no native/full12 proof claim."""
import copy
import contextlib
import io
import json
from pathlib import Path
import random
import struct
import sys
import unittest
import warnings
import zipfile

ROOT=Path(__file__).resolve().parents[3]
sys.path[:0]=[str(ROOT/'.github/scripts'),str(ROOT/'infra/gcp/scripts'),str(Path(__file__).parent)]
import history_observation_sync_archive as archive
import history_observation_sync_checks as checks
import test_history_observation_sync_validation as fixture

gate=archive.gate


def varint(value):
    result=bytearray()
    while value>127:result.append((value&127)|128);value>>=7
    return bytes(result)+bytes([value])


def field(number,value):
    if type(value)is int:return varint(number<<3)+varint(value)
    if type(value)is str:value=value.encode()
    return varint(number<<3|2)+varint(len(value))+value


def proto(raw_checks,mode=2):
    value=field(1,3)+field(14,'1.16.4')+field(17,mode)+field(25,1 if mode==2 else 0)+field(26,1)+field(18,b'')
    for group in raw_checks:
        obj=b''.join(field(4,field(1,item['object_addr'])+field(2,1)) for item in group['objects'])
        value+=field(19,field(1,4)+field(2,group['config_addr'])+field(3,1)+obj)
    return value


def data():
    normal,metadata=fixture.fixture();_,before,after,_=fixture.state_fixture.fixture()
    changes,drift=gate._observation(normal)
    dependencies={}
    for state,is_after in ((before,False),(after,True)):
        for resource in state['resources']:
            base=(resource['module']+'.'if'module'in resource else '')+resource['type']+'.'+resource['name']
            for instance in resource['instances']:
                address=base+('['+gate.canonical(instance['index_key'])+']'if'index_key'in instance else '')
                instance['attributes']=copy.deepcopy(drift['before']if address==gate.HISTORY and not is_after else changes[address])
                if is_after:dependencies[address]=copy.deepcopy(instance.get('dependencies',[]))
    groups=[dict(object_kind='var',config_addr='var.'+name,status='pass',
        objects=[dict(object_addr='var.'+name,status='pass')])for name in ('project_id','dummy')]
    shown=[dict(address=dict(kind='var',name=item['config_addr'][4:],to_display=item['config_addr']),
        status='pass',instances=[dict(address=dict(to_display=item['objects'][0]['object_addr']),status='pass')])for item in groups]
    before['check_results']=copy.deepcopy(groups);after['check_results']=list(reversed(copy.deepcopy(groups)))
    normal['checks']=copy.deepcopy(shown)
    refresh=fixture.candidate(normal);refresh['checks']=shown
    def dependency_fields(module):
        for item in module.get('resources',[]):item['depends_on']=dependencies[item['address']]
        for child in module.get('child_modules',[]):dependency_fields(child)
    dependency_fields(refresh['prior_state']['values']['root_module'])
    previous=copy.deepcopy(before);previous.update(lineage='',serial=0)
    candidate=copy.deepcopy(after);candidate.update(serial=before['serial'],check_results=None)
    snapshot={'tfconfig/modules.json':b'[{"Key":"","Dir":"."}]','tfconfig/m-/main.tf':b'# invented\n','.terraform.lock.hcl':b'# invented lock\n'}
    members=dict(snapshot,tfplan=proto(groups),tfstate=gate.canonical(candidate).encode())
    members['tfstate-prev']=gate.canonical(previous).encode()
    catalog=checks.CheckCatalog(configs=frozenset({('var','var.project_id'),('var','var.dummy')}),
        zero_instances=frozenset(),module_instances=frozenset({''}),resource_instances=frozenset())
    return normal,metadata,before,after,refresh,members,snapshot,catalog


def packed(members,method=zipfile.ZIP_DEFLATED):
    buffer=io.BytesIO()
    with zipfile.ZipFile(buffer,'w',compression=method)as zipfile_:
        for name,value in members.items():zipfile_.writestr(name,value)
    return buffer.getvalue()


def inspect(parts,raw=None):
    normal,_,before,_,refresh,members,snapshot,catalog=parts
    return archive.inspect_candidate(packed(members)if raw is None else raw,gate.canonical(before).encode(),normal,refresh,
        expected_snapshot=snapshot,catalog=catalog)


class HistoryObservationArchiveTest(unittest.TestCase):
    def reject(self,callback):
        out,err=io.StringIO(),io.StringIO()
        with contextlib.redirect_stdout(out),contextlib.redirect_stderr(err):
            with self.assertRaises((gate.SyncRejected,checks.ChecksRejected))as result:callback()
        self.assertNotIn(fixture.CANARY,str(result.exception)+out.getvalue()+err.getvalue())
        self.assertEqual(out.getvalue()+err.getvalue(),'')

    def test_all12_private_raw_proto_show_source_join_and_semantic_checks_order_only(self):
        parts=data()
        admission=gate.validate_refresh_candidate(parts[4],parts[0],**fixture.args(parts[1]))
        self.assertFalse(admission['execution_permitted'])
        original=copy.deepcopy(parts)
        expected=inspect(parts)
        self.assertEqual(expected['serial'],parts[2]['serial']+1)
        expected_original=copy.deepcopy(expected)
        archive.verify_after(gate.canonical(parts[3]).encode(),expected,catalog=parts[-1])
        self.assertEqual(parts,original)
        self.assertEqual(expected,expected_original)
        self.assertIsNone(parts[5]and json.loads(parts[5]['tfstate'])['check_results'])

    def test_non_target_sensitivity_and_identity_change_cannot_hide_in_matching_show(self):
        for field_name,value in (('identity',{'id':'DUMMY_CHANGED'}),('identity_schema_version',1),
                                 ('sensitive_attributes',[[{'type':'get_attr','value':'project'}]])):
            parts=data();candidate=json.loads(parts[5]['tfstate'])
            candidate['resources'][0]['instances'][0][field_name]=value
            shown=parts[4]['prior_state']['values']['root_module']['resources'][0]
            if field_name=='sensitive_attributes':shown['sensitive_values']={'project':True}
            else:shown[field_name]=copy.deepcopy(value)
            parts[5]['tfstate']=gate.canonical(candidate).encode()
            self.reject(lambda:inspect(parts))

    def test_other_raw_array_order_and_field_presence_are_not_canonicalized(self):
        for mutation in (lambda state:state['resources'][0]['instances'][0]['dependencies'].reverse(),
                         lambda state:next(r for r in state['resources']if len(r['instances'])>1)['instances'].reverse(),
                         lambda state:state['resources'][0]['instances'][0].update(identity=None),
                         lambda state:state['resources'][0]['instances'][0].update(identity_schema_version=0)):
            parts=data();expected=inspect(parts);after=copy.deepcopy(parts[3]);mutation(after)
            self.reject(lambda:archive.verify_after(gate.canonical(after).encode(),expected,catalog=parts[-1]))

    def test_candidate_and_post_lineage_type_unknown_fields_and_exact_serial_are_stop(self):
        mutations=(lambda state:state.update(lineage='00000000-0000-0000-0000-000000000002'),
                   lambda state:state.update(serial=True),lambda state:state.update(serial=state['serial']+2),
                   lambda state:state.update(unknown=fixture.CANARY),
                   lambda state:state['resources'][0]['instances'][0].update(unknown=fixture.CANARY))
        for mutation in mutations:
            parts=data();candidate=json.loads(parts[5]['tfstate']);mutation(candidate)
            parts[5]['tfstate']=gate.canonical(candidate).encode();self.reject(lambda:inspect(parts))
            parts=data();expected=inspect(parts);after=copy.deepcopy(parts[3]);mutation(after)
            self.reject(lambda:archive.verify_after(gate.canonical(after).encode(),expected,catalog=parts[-1]))

    def test_mode17_and_complete_field19_set_are_required_not_plan_filename(self):
        parts=data();groups=parts[2]['check_results']
        poisons=[proto(groups)+field(3,b''),proto(groups).replace(field(18,b''),b'',1),
            proto(groups)+field(18,b''),proto(groups,mode=0),proto(groups[:1]),proto(groups+groups[:1]),
            proto(groups)+field(99,fixture.CANARY),
            proto([])+field(19,field(1,4)+field(2,'var.project_id')+field(3,9)),
            proto([])+field(19,field(1,4)+field(2,'var.project_id')+field(3,1)+field(5,fixture.CANARY)),
            proto([])+field(19,field(1,4)+field(2,'var.project_id')+field(3,1)
                +field(4,field(1,'var.project_id')+field(2,1)+field(3,fixture.CANARY)))]
        for poison in poisons:
            parts=data();parts[5]['tfplan']=poison;self.reject(lambda:inspect(parts))

    def test_checks_cannot_be_new_failed_unknown_or_poisoned_in_any_join(self):
        for status in ('unknown','fail','error'):
            parts=data();parts[4]['checks'][0]['status']=status;self.reject(lambda:inspect(parts))
            parts=data();previous=json.loads(parts[5]['tfstate-prev']);previous['check_results'][0]['status']=status
            parts[5]['tfstate-prev']=gate.canonical(previous).encode();self.reject(lambda:inspect(parts))
            parts=data();expected=inspect(parts);after=copy.deepcopy(parts[3]);after['check_results'][0]['status']=status
            self.reject(lambda:archive.verify_after(gate.canonical(after).encode(),expected,catalog=parts[-1]))
        parts=data();candidate=json.loads(parts[5]['tfstate']);candidate['check_results']=copy.deepcopy(parts[2]['check_results'])
        candidate['check_results'].append(copy.deepcopy(candidate['check_results'][0]))
        parts[5]['tfstate']=gate.canonical(candidate).encode();self.reject(lambda:inspect(parts))

    def test_every_source_snapshot_byte_and_member_is_pinned(self):
        for name in ('tfconfig/modules.json','.terraform.lock.hcl','tfconfig/m-/main.tf'):
            parts=data();parts[5][name]+=b' ';self.reject(lambda:inspect(parts))
            parts=data();parts[5].pop(name);self.reject(lambda:inspect(parts))
        for name in ('tfconfig/m-/untracked.tf','tfconfig/m-/.hidden.tf','tfconfig/extra/modules.json'):
            parts=data();parts[5][name]=b'# not in reviewed snapshot';self.reject(lambda:inspect(parts))

    def test_prior_show_dependency_set_and_order_must_match_raw_state(self):
        for mutate in (lambda item:item.pop('depends_on'),lambda item:item['depends_on'].reverse(),
                       lambda item:item.update(depends_on=[fixture.CANARY])):
            parts=data();item=parts[4]['prior_state']['values']['root_module']['resources'][0]
            mutate(item);self.reject(lambda:inspect(parts))

    def test_duplicate_zip_member_and_bad_crc_stop_before_json_or_public_output(self):
        parts=data();buffer=io.BytesIO()
        with warnings.catch_warnings():
            warnings.simplefilter('ignore',UserWarning)
            with zipfile.ZipFile(buffer,'w')as writer:
                for name,value in parts[5].items():writer.writestr(name,value)
                writer.writestr('tfstate',parts[5]['tfstate'])
        self.reject(lambda:inspect(parts,buffer.getvalue()))
        raw=bytearray(packed(parts[5],method=zipfile.ZIP_STORED))
        with zipfile.ZipFile(io.BytesIO(raw))as reader:
            offset=reader.getinfo('tfstate').header_offset
        name_length,extra_length=struct.unpack_from('<HH',raw,offset+26)
        raw[offset+30+name_length+extra_length]^=1
        with self.assertRaises(zipfile.BadZipFile):
            with zipfile.ZipFile(io.BytesIO(raw))as reader:reader.read('tfstate')
        self.reject(lambda:inspect(parts,bytes(raw)))

    def test_zip_truncation_unsupported_method_count_size_and_expansion_are_bounded(self):
        parts=data();raw=packed(parts[5]);self.reject(lambda:inspect(parts,raw[:-23]))
        self.reject(lambda:inspect(parts,packed(parts[5],method=zipfile.ZIP_BZIP2)))
        self.reject(lambda:inspect(parts,b'PK\x03\x04'+b'0'*archive.MAX_ARCHIVE_BYTES))
        parts=data();parts[5].update({'tfconfig/extra'+str(i):b'x' for i in range(archive.MAX_MEMBERS)})
        self.reject(lambda:inspect(parts))
        parts=data();parts[5]['tfconfig/m-/main.tf']=b'X'*100000
        self.reject(lambda:inspect(parts))
        # Deterministic public bytes repeated within DEFLATE's 32KiB window:
        # each member/ratio and the archive fit, but total expansion exceeds
        # 16MiB. This isolates the whole-archive expansion bound.
        block=random.Random(0).randbytes(16*1024)*256
        parts=data();parts[5].update({'tfconfig/large'+str(i):block for i in range(5)})
        raw=packed(parts[5]);self.assertLess(len(raw),archive.MAX_ARCHIVE_BYTES)
        with zipfile.ZipFile(io.BytesIO(raw))as reader:
            self.assertGreater(sum(x.file_size for x in reader.infolist()),archive.MAX_EXPANDED_BYTES)
            self.assertTrue(all(x.file_size<=gate.MAX_BYTES and x.file_size<=max(x.compress_size,1)*256 for x in reader.infolist()))
        self.reject(lambda:inspect(parts,raw))

    def test_duplicate_or_nonfinite_private_json_is_rejected_without_values(self):
        for name in ('tfstate','tfstate-prev'):
            for raw in (b'{"serial":1,"serial":2}',b'{"serial":NaN}',b'{"serial":Infinity}'):
                parts=data();parts[5][name]=raw;self.reject(lambda:inspect(parts))

    def test_unknown_member_zip_paths_symlinks_duplicates_and_budgets_are_stop(self):
        for name in ('../x','tfstate/../x','unknown', '/x','tfconfig/../../x'):
            parts=data();parts[5][name]=b'x';self.reject(lambda:inspect(parts))
        parts=data();parts[5]['tfconfig/m-/main.tf']=b'X'*(gate.MAX_BYTES+1);self.reject(lambda:inspect(parts))
        buffer=io.BytesIO()
        with zipfile.ZipFile(buffer,'w')as z:
            entry=zipfile.ZipInfo('tfstate');entry.external_attr=(0o120777<<16);z.writestr(entry,b'target')
            z.writestr('tfplan',b'x');z.writestr('tfstate-prev',b'{}')
        self.reject(lambda:archive.read_archive(buffer.getvalue()))

    def test_whole_private_state_outputs_dependencies_identity_and_candidate_serial_are_pinned(self):
        mutations=[lambda s:s['outputs']['project_id'].update(value=fixture.CANARY),
            lambda s:s['resources'][0]['instances'][0].update(private=fixture.CANARY+'_changed'),
            lambda s:s['resources'][0]['instances'][0].update(dependencies=[fixture.CANARY]),
            lambda s:s['resources'][0]['instances'][0].update(identity={'id':fixture.CANARY}),
            lambda s:s.update(serial=7),lambda s:s['resources'].reverse()]
        for mutate in mutations:
            parts=data();value=json.loads(parts[5]['tfstate']);mutate(value)
            parts[5]['tfstate']=gate.canonical(value).encode();self.reject(lambda:inspect(parts))

    def test_previous_stub_all_payload_and_headers_are_strict(self):
        for mutate in (lambda s:s.update(serial=True),lambda s:s.update(lineage=fixture.CANARY),
            lambda s:s['resources'][0]['instances'][0]['attributes'].update(project=fixture.CANARY)):
            parts=data();value=json.loads(parts[5]['tfstate-prev']);mutate(value)
            parts[5]['tfstate-prev']=gate.canonical(value).encode();self.reject(lambda:inspect(parts))

    def test_two_leaf_plan_A_to_B_never_accepts_archive_A_to_C(self):
        parts=data();value=json.loads(parts[5]['tfstate'])
        value['resources'][-1]['instances'][0]['attributes']['etag']='DUMMY_C'
        parts[5]['tfstate']=gate.canonical(value).encode();self.reject(lambda:inspect(parts))
        parts=data();parts[5]['tfconfig/m-/main.tf']=b'# changed';self.reject(lambda:inspect(parts))
        parts=data();parts[4]['prior_state']['values']['root_module']['resources'][0]['identity']={'id':'X'}
        self.reject(lambda:inspect(parts))

    def test_changed_check_content_field_presence_and_post_serial_cannot_hide_in_sort(self):
        for mutation in (lambda s:s['check_results'][0].update(status='fail'),
            lambda s:s['check_results'][0]['objects'][0].update(failure_messages=[]),
            lambda s:s.update(serial=s['serial']+1)):
            parts=data();expected=inspect(parts);bad=copy.deepcopy(parts[3]);mutation(bad)
            self.reject(lambda:archive.verify_after(gate.canonical(bad).encode(),expected,catalog=parts[-1]))

    def test_sensitive_path_cty_types_and_show_masks_join_without_unmarking(self):
        attributes={'labels':{'email':'DUMMY'},'list':[{'secret':'DUMMY'}]}
        paths=[[{'type':'get_attr','value':'labels'}],
            [{'type':'get_attr','value':'list'},{'type':'index','value':{'type':'number','value':0}},
             {'type':'get_attr','value':'secret'}]]
        self.assertEqual(archive.sensitive_paths(paths,attributes),
            archive.mask_paths({'labels':True,'list':[{'secret':True}]},attributes))
        for changed in ({'type':'number','value':True},{'type':'number','value':1.0},
                        {'type':'string','value':'missing'},{'type':'unexpected','value':0}):
            value=copy.deepcopy(paths);value[1][1]['value']=changed
            self.reject(lambda:archive.sensitive_paths(value,attributes))


if __name__=='__main__':unittest.main()
