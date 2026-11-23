"""既存の主要メール通知先を値を変えずに取り込む通常経路の例外を検証する。"""
from contextlib import redirect_stdout
from copy import deepcopy
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import Mock, patch

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
import primary_email_adoption as adoption
import terraform_normal_ops as ops

PROJECT = 'youtube-study-space'
CHANNEL = f'projects/{PROJECT}/notificationChannels/1234567890'
VALUES = {
    'deletion_policy': 'DELETE', 'description': '', 'display_name': '[production] Study Space primary email',
    'enabled': True, 'force_delete': False, 'id': CHANNEL, 'labels': {'email_address': 'fixture@example.invalid'},
    'name': CHANNEL, 'project': PROJECT, 'sensitive_labels': [], 'timeouts': None, 'type': 'email',
    'user_labels': {'environment': 'production', 'managed_by': 'manual'}, 'verification_status': '',
}


def plan(**change):
    """Shape observed from the pinned provider's native import plan."""
    return {'format_version': '1.2', 'terraform_version': ops.common.VERSION, 'errored': False, 'complete': True,
            'resource_changes': [{
                'address': adoption.ADDRESS, 'module_address': 'module.notification_channels', 'mode': 'managed',
                'type': 'google_monitoring_notification_channel', 'name': 'primary_email', 'index': 0,
                'provider_name': 'registry.terraform.io/hashicorp/google',
                'change': {'actions': ['update'], 'before': deepcopy(VALUES), 'after': deepcopy(VALUES),
                           'after_unknown': {}, 'before_sensitive': {'labels': {}},
                           'after_sensitive': {'labels': {'email_address': True}},
                           'importing': {'id': CHANNEL}} | change}]}


class PrimaryEmailAdoption(unittest.TestCase):
    def test_selected_value_preserving_import_is_the_only_accepted_import(self):
        # A no-op import already carries the recipient mark; an update adds only that mark.
        for change in ({'actions': ['update']},
                       {'actions': ['no-op'], 'before_sensitive': {'labels': {'email_address': True}}}):
            with self.subTest(actions=change['actions']):
                counts = ops.validate(plan(**change), {}, project=PROJECT, primary_email_channel=CHANNEL)
                self.assertEqual(counts['create'] + counts['delete'] + counts['replace'], 0)
        # The production adoption plan also lists every already-owned resource as no-op.
        candidate = plan()
        candidate['resource_changes'].append({
            'address': 'google_pubsub_topic.owned', 'mode': 'managed', 'type': 'google_pubsub_topic',
            'provider_name': 'registry.terraform.io/hashicorp/google',
            'change': {'actions': ['no-op'], 'before': {'project': PROJECT}, 'after': {'project': PROJECT}}})
        with self.subTest(case='alongside owned no-op resources'):
            counts = ops.validate(candidate, {}, project=PROJECT, primary_email_channel=CHANNEL)
            self.assertEqual((counts['update'], counts['no-op']), (1, 1))

    def test_unselected_or_altered_imports_are_rejected(self):
        def changed(key, value):
            def mutate(candidate):
                candidate['resource_changes'][0]['change']['after'][key] = value
            return mutate

        cases = {
            'value change': changed('user_labels', {'environment': 'production', 'managed_by': 'terraform'}),
            'recipient change': changed('labels', {'email_address': 'other@example.invalid'}),
            'disabled': changed('enabled', False),
            'unknown': lambda p: p['resource_changes'][0]['change'].update(after_unknown={'verification_status': True}),
            'replace': lambda p: p['resource_changes'][0]['change'].update(actions=['delete', 'create'], replace_paths=[['type']]),
            'other import id': lambda p: p['resource_changes'][0]['change'].update(
                importing={'id': f'projects/{PROJECT}/notificationChannels/999'}),
            'other address': lambda p: p['resource_changes'][0].update(address='google_monitoring_notification_channel.other'),
            'other type': lambda p: p['resource_changes'][0].update(type='google_monitoring_alert_policy'),
            'beta provider': lambda p: p['resource_changes'][0].update(provider_name='registry.terraform.io/hashicorp/google-beta'),
            'recipient left unmarked': lambda p: p['resource_changes'][0]['change'].update(after_sensitive={'labels': {}}),
            'other attribute marked': lambda p: p['resource_changes'][0]['change'].update(
                after_sensitive={'labels': {'email_address': True}, 'name': True}),
            'update without a mark change': lambda p: p['resource_changes'][0]['change'].update(
                before_sensitive={'labels': {'email_address': True}}),
            'missing masks': lambda p: p['resource_changes'][0]['change'].pop('after_sensitive'),
            'other attribute marked on both sides': lambda p: p['resource_changes'][0]['change'].update(
                before_sensitive={'labels': {}, 'name': True}, after_sensitive={'labels': {'email_address': True}, 'name': True}),
            'other label mark removed': lambda p: p['resource_changes'][0]['change'].update(
                before_sensitive={'labels': {'email_address': True, 'other': True}}),
            'non-boolean sensitive leaf': lambda p: p['resource_changes'][0]['change'].update(
                after_sensitive={'labels': {'email_address': True}, 'name': 'true'}),
            'null sensitive leaf': lambda p: p['resource_changes'][0]['change'].update(
                before_sensitive={'labels': {}, 'name': None}),
            'nested mark in a list': lambda p: p['resource_changes'][0]['change'].update(
                after_sensitive={'labels': {'email_address': True}, 'sensitive_labels': [{'password': True}]}),
            'extra marks on both sides of a no-op': lambda p: p['resource_changes'][0]['change'].update(
                actions=['no-op'], before_sensitive={'labels': {'email_address': True}, 'name': True},
                after_sensitive={'labels': {'email_address': True}, 'name': True}),
            'null unknown mask': lambda p: p['resource_changes'][0]['change'].update(after_unknown=None),
            'non-boolean unknown leaf': lambda p: p['resource_changes'][0]['change'].update(after_unknown={'name': None}),
            'mixed with another import': lambda p: p['resource_changes'].append({
                'address': 'module.monitor_auth[0].google_identity_platform_config.auth', 'mode': 'managed',
                'type': 'google_identity_platform_config', 'provider_name': 'registry.terraform.io/hashicorp/google',
                'change': {'actions': ['no-op'], 'importing': {'id': f'projects/{PROJECT}/config'},
                           'before': {'project': PROJECT}, 'after': {'project': PROJECT}}}),
            'mixed with another change': lambda p: p['resource_changes'].append({
                'address': 'google_pubsub_topic.other', 'mode': 'managed', 'type': 'google_pubsub_topic',
                'provider_name': 'registry.terraform.io/hashicorp/google',
                'change': {'actions': ['update'], 'before': {'project': PROJECT}, 'after': {'project': PROJECT}}}),
        }
        for name, mutate in cases.items():
            with self.subTest(case=name):
                candidate = plan(); mutate(candidate)
                with self.assertRaises(ValueError):
                    # Approve the replace, and enable the Auth import, so only the
                    # primary email import exception can reject the plan.
                    ops.validate(candidate, {adoption.ADDRESS: ['delete', 'create']} if name == 'replace' else {},
                                 project=PROJECT, primary_email_channel=CHANNEL,
                                 monitor_auth_enabled=name == 'mixed with another import')
        for name, channel in (('not selected', None), ('non-numeric', f'projects/{PROJECT}/notificationChannels/x'),
                              ('foreign project', 'projects/test-youtube-study-space/notificationChannels/1234567890')):
            with self.subTest(case=name):
                with self.assertRaises(ValueError):
                    ops.validate(plan(), {}, project=PROJECT, primary_email_channel=channel)
        development = 'projects/test-youtube-study-space/notificationChannels/1234567890'
        candidate = plan()
        for values in (candidate['resource_changes'][0]['change']['before'], candidate['resource_changes'][0]['change']['after']):
            values.update(project='test-youtube-study-space', name=development, id=development)
        candidate['resource_changes'][0]['change']['importing'] = {'id': development}
        with self.subTest(case='development target'), self.assertRaises(ValueError):
            ops.validate(candidate, {}, project='test-youtube-study-space', primary_email_channel=development)
        with self.subTest(case='import left in the post-apply plan'), self.assertRaises(ValueError):
            ops.validate(plan(actions=['no-op'], before_sensitive={'labels': {'email_address': True}}), {},
                         post=True, project=PROJECT, primary_email_channel=CHANNEL)

    def test_public_inputs_cannot_select_the_adoption_or_carry_its_private_values(self):
        transport = {'schema_version': 'gcp-migration-inputs-v1', 'manifest': {}, 'plan_integrity_key': 'a' * 64,
                     'tfvars': {'project_id': PROJECT}}
        environment = {'NORMAL_TARGET': 'prod', 'NORMAL_INPUTS_JSON': json.dumps(transport)}
        with patch.object(ops.common, 'decode_input_transport', return_value=transport), \
                patch.object(ops.selected_source, 'read_file', return_value=b'{}'):
            self.assertEqual(ops.inputs(environment)[0], {'project_id': PROJECT})
        for key in sorted(adoption.PRIVATE_KEYS):
            with self.subTest(key=key), patch.object(ops.common, 'decode_input_transport', return_value=transport), \
                    patch.object(ops.selected_source, 'read_file', return_value=json.dumps({key: True}).encode()), \
                    self.assertRaises(ValueError):
                ops.inputs(environment)

    def test_only_an_explicit_private_selection_enables_the_exception(self):
        self.assertEqual(adoption.channel_name({'manage_primary_email': True, 'primary_email_channel_name': CHANNEL}), CHANNEL)
        for tfvars in ({}, {'primary_email_channel_name': CHANNEL},
                       {'manage_primary_email': 'true', 'primary_email_channel_name': CHANNEL},
                       {'manage_primary_email': True, 'primary_email_channel_name': None}, None):
            with self.subTest(tfvars=tfvars):
                self.assertIsNone(adoption.channel_name(tfvars))

    def test_controller_selects_the_exception_only_from_this_runs_private_inputs(self):
        selected = {'manage_primary_email': True, 'primary_email_channel_name': CHANNEL}
        for tfvars, accepted in ((selected, True), ({}, False),
                                 (selected | {'manage_primary_email': False}, False),
                                 (selected | {'primary_email_channel_name': CHANNEL + '1'}, False)):
            with self.subTest(tfvars=tfvars), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                (root / 'inputs.tfvars.json').write_text(json.dumps(tfvars))

                def terraform(_env, _root, _tfroot, _name, args, codes=(0,)):
                    for arg in args:
                        if arg.startswith('-out='):
                            Path(arg.removeprefix('-out=')).write_bytes(b'synthetic-plan')
                    return Mock(returncode=2, stdout=json.dumps(plan()).encode())

                environment = {'NORMAL_MODE': 'normal-plan', 'NORMAL_TARGET': 'prod'}
                with patch.object(ops.common, 'terraform', side_effect=terraform), \
                        patch.object(ops, 'verify_storage_ownership'), redirect_stdout(io.StringIO()):
                    if accepted:
                        counts = ops.plan(environment, root, root, 'normal', {})[2]
                        self.assertEqual(counts['update'], 1)
                    else:
                        with self.assertRaises(ops.PlanRejected):
                            ops.plan(environment, root, root, 'normal', {})


if __name__ == '__main__':
    unittest.main()
