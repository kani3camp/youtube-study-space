"""Source-only checks contract; synthetic schema cases are not native proof."""
import contextlib
import copy
import io
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
import history_observation_sync_checks as checks

RESOURCE = 'module.alerts.google_monitoring_alert_policy.quota'
INSTANCE = RESOURCE + '["daily"]'
CONFIGS = frozenset({('var', 'var.environment'), ('var', 'module.child.var.input'),
                     ('var', 'module.off.var.input'), ('resource', RESOURCE),
                     ('output', 'output.label'), ('check', 'check.valid')})
CATALOG = checks.CheckCatalog(CONFIGS, frozenset({('var', 'module.off.var.input')}),
    frozenset({'', 'module.alerts', 'module.child[0]', 'module.off[0]'}), frozenset({INSTANCE}))
RAW = [
    {'object_kind': 'var', 'config_addr': 'var.environment', 'status': 'pass',
     'objects': [{'object_addr': 'var.environment', 'status': 'pass'}]},
    {'object_kind': 'var', 'config_addr': 'module.child.var.input', 'status': 'pass',
     'objects': [{'object_addr': 'module.child[0].var.input', 'status': 'pass'}]},
    {'object_kind': 'var', 'config_addr': 'module.off.var.input', 'status': 'pass', 'objects': None},
    {'object_kind': 'resource', 'config_addr': RESOURCE, 'status': 'pass',
     'objects': [{'object_addr': INSTANCE, 'status': 'pass'}]},
    {'object_kind': 'output', 'config_addr': 'output.label', 'status': 'pass',
     'objects': [{'object_addr': 'output.label', 'status': 'pass'}]},
    {'object_kind': 'check', 'config_addr': 'check.valid', 'status': 'pass',
     'objects': [{'object_addr': 'check.valid', 'status': 'pass'}]},
]
SHOW = [
    {'address': {'kind': 'var', 'name': 'environment', 'to_display': 'var.environment'},
     'status': 'pass', 'instances': [{'address': {'to_display': 'var.environment'}, 'status': 'pass'}]},
    {'address': {'kind': 'var', 'name': 'input', 'module': 'module.child', 'to_display': 'module.child.var.input'},
     'status': 'pass', 'instances': [{'address': {'module': 'module.child[0]', 'to_display': 'module.child[0].var.input'}, 'status': 'pass'}]},
    {'address': {'kind': 'var', 'name': 'input', 'module': 'module.off', 'to_display': 'module.off.var.input'}, 'status': 'pass'},
    {'address': {'kind': 'resource', 'mode': 'managed', 'type': 'google_monitoring_alert_policy',
                 'name': 'quota', 'module': 'module.alerts', 'to_display': RESOURCE},
     'status': 'pass', 'instances': [{'address': {'module': 'module.alerts', 'instance_key': 'daily', 'to_display': INSTANCE}, 'status': 'pass'}]},
    {'address': {'kind': 'output_value', 'name': 'label', 'to_display': 'output.label'},
     'status': 'pass', 'instances': [{'address': {'to_display': 'output.label'}, 'status': 'pass'}]},
    {'address': {'kind': 'check', 'name': 'valid', 'to_display': 'check.valid'},
     'status': 'pass', 'instances': [{'address': {'to_display': 'check.valid'}, 'status': 'pass'}]},
]
KINDS = {'resource': 1, 'output': 2, 'check': 3, 'var': 4}


def varint(value):
    result = bytearray()
    while value > 127:
        result.append((value & 127) | 128)
        value >>= 7
    result.append(value)
    return bytes(result)


def field(number, value):
    if type(value) is int:
        return varint(number << 3) + varint(value)
    if type(value) is str:
        value = value.encode()
    return varint((number << 3) | 2) + varint(len(value)) + value


def group(value):
    result = field(1, KINDS[value['object_kind']]) + field(2, value['config_addr']) + field(3, 1)
    for obj in value['objects'] or []:
        encoded = field(1, obj['object_addr']) + field(2, 1)
        for message in obj.get('failure_messages', []):
            encoded += field(3, message)
        result += field(4, encoded)
    return result


def plan(groups=None, mode=2):
    if groups is None:
        groups = [group(x) for x in RAW]
    return (field(1, 3) + field(14, '1.16.4') + field(17, mode)
            + field(25, 1 if mode == 2 else 0) + field(26, 1)
            + b''.join(field(19, x) for x in groups))


class ChecksContractTests(unittest.TestCase):
    def reject(self, function):
        output = io.StringIO()
        with contextlib.redirect_stdout(output), contextlib.redirect_stderr(output):
            with self.assertRaises(checks.ChecksRejected) as caught:
                function()
        self.assertEqual(str(caught.exception), 'History observation checks STOP')
        self.assertEqual(output.getvalue(), '')

    def raw(self, value=RAW, catalog=CATALOG):
        return checks.raw_checks(value, catalog=catalog)

    def test_complete_four_kind_and_zero_instance_join(self):
        observed = checks.verify_checks(plan(), SHOW, RAW, catalog=CATALOG)
        self.assertEqual(observed, self.raw())
        self.assertEqual(observed[('var', 'module.off.var.input')], ('pass', {}))
        self.assertEqual(len(observed), 6)

    def test_captured_native_variable_wire_message(self):
        # Public-dummy TF1.16.4 field19 capture; resource/zero cases above are
        # official-schema synthetic cases and do not claim native coverage.
        native = bytes.fromhex('0804120f7661722e656e7669726f6e6d656e74180122130a0f7661722e656e7669726f6e6d656e741001')
        observed, info = checks.decode_plan(plan([native]), catalog=CATALOG)
        self.assertEqual(observed, self.raw(RAW[:1]))
        self.assertEqual(info['mode'], 2)

    def test_normal_mode_requires_explicit_expected_mode(self):
        observed, info = checks.decode_plan(plan(mode=0), expected_mode=0, catalog=CATALOG)
        self.assertEqual(observed, self.raw())
        self.assertEqual(info['mode'], 0)
        self.reject(lambda: checks.decode_plan(plan(mode=0), catalog=CATALOG))

    def test_only_check_container_order_is_canonical(self):
        reordered = list(reversed(copy.deepcopy(RAW)))
        self.assertEqual(checks.canonical_checks(RAW, catalog=CATALOG), checks.canonical_checks(reordered, catalog=CATALOG))
        self.assertEqual(RAW[0]['config_addr'], 'var.environment')

    def test_raw_null_empty_and_optional_field_presence_retained(self):
        changed = copy.deepcopy(RAW)
        changed[2]['objects'] = []
        changed[0]['objects'][0]['failure_messages'] = []
        self.assertEqual(self.raw(), self.raw(changed))
        self.assertNotEqual(checks.canonical_checks(RAW, catalog=CATALOG), checks.canonical_checks(changed, catalog=CATALOG))
        self.assertEqual(checks.planned_checks(changed, self.raw(changed), catalog=CATALOG), checks.canonical_checks(RAW, catalog=CATALOG))

    def test_empty_whole_check_set_representation_not_erased(self):
        self.assertIsNone(checks.canonical_checks(None, catalog=CATALOG))
        self.assertEqual(checks.canonical_checks([], catalog=CATALOG), [])
        self.assertIsNone(checks.planned_checks([], {}, catalog=CATALOG))

    def test_catalog_is_mandatory_and_source_addresses_bound(self):
        self.reject(lambda: self.raw(catalog=None))
        bad = checks.CheckCatalog(CONFIGS, CATALOG.zero_instances, frozenset({''}), CATALOG.resource_instances)
        self.reject(lambda: self.raw(catalog=bad))

    def test_catalog_unknown_resource_or_unapproved_zero_rejected(self):
        for bad in (checks.CheckCatalog(CONFIGS, frozenset({('var', 'var.unknown')}), CATALOG.module_instances, CATALOG.resource_instances),
                    checks.CheckCatalog(CONFIGS, CATALOG.zero_instances, CATALOG.module_instances, frozenset({'google_unknown.fake'}))):
            self.reject(lambda bad=bad: self.raw(catalog=bad))

    def test_catalog_mutable_sets_not_accepted(self):
        bad = checks.CheckCatalog(set(CONFIGS), CATALOG.zero_instances, CATALOG.module_instances, CATALOG.resource_instances)
        self.reject(lambda: self.raw(catalog=bad))

    def test_duplicate_raw_config_and_object_addresses_rejected(self):
        self.reject(lambda: self.raw([*RAW, RAW[0]]))
        value = copy.deepcopy(RAW)
        value[0]['objects'].append(value[0]['objects'][0])
        self.reject(lambda: self.raw(value))

    def test_wrong_module_index_and_cross_group_objects_rejected(self):
        for address in ('module.child[1].var.input', 'var.environment'):
            value = copy.deepcopy(RAW)
            value[1]['objects'][0]['object_addr'] = address
            self.reject(lambda value=value: self.raw(value))

    def test_nonzero_empty_group_and_unknown_config_rejected(self):
        value = copy.deepcopy(RAW)
        value[0]['objects'] = None
        self.reject(lambda: self.raw(value))
        value = copy.deepcopy(RAW)
        value[0]['config_addr'] = 'var.PUBLIC_CANARY_NOT_SECRET'
        self.reject(lambda: self.raw(value))

    def test_all_unknown_fail_error_statuses_rejected(self):
        for status in ('unknown', 'fail', 'error', True, 1):
            for obj in (False, True):
                value = copy.deepcopy(RAW)
                target = value[0]['objects'][0] if obj else value[0]
                target['status'] = status
                self.reject(lambda value=value: self.raw(value))

    def test_failure_messages_and_unknown_raw_keys_not_discarded(self):
        value = copy.deepcopy(RAW)
        value[0]['objects'][0]['failure_messages'] = ['PUBLIC_CANARY_NOT_SECRET']
        self.reject(lambda: self.raw(value))
        value = copy.deepcopy(RAW)
        value[0]['extra'] = 'PUBLIC_CANARY_NOT_SECRET'
        self.reject(lambda: self.raw(value))

    def test_show_source_static_fields_exact_no_provider_extra(self):
        for field_name, value in (('provider', 'PUBLIC_CANARY_NOT_SECRET'), ('mode', 'data'), ('type', 'google_other')):
            shown = copy.deepcopy(SHOW)
            shown[3]['address'][field_name] = value
            self.reject(lambda shown=shown: checks.shown_checks(shown, self.raw(), catalog=CATALOG))

    def test_show_output_value_kind_is_not_raw_output_kind(self):
        shown = copy.deepcopy(SHOW)
        shown[4]['address']['kind'] = 'output'
        self.reject(lambda: checks.shown_checks(shown, self.raw(), catalog=CATALOG))

    def test_show_zero_instances_must_be_omitted(self):
        for empty in (None, []):
            shown = copy.deepcopy(SHOW)
            shown[2]['instances'] = empty
            self.reject(lambda shown=shown: checks.shown_checks(shown, self.raw(), catalog=CATALOG))

    def test_show_duplicate_missing_new_and_failed_instances_rejected(self):
        changed = []
        value = copy.deepcopy(SHOW); value.append(value[0]); changed.append(value)
        changed.append(copy.deepcopy(SHOW[:-1]))
        value = copy.deepcopy(SHOW); value[0]['instances'].append(value[0]['instances'][0]); changed.append(value)
        value = copy.deepcopy(SHOW); value[0]['instances'][0]['status'] = 'unknown'; changed.append(value)
        value = copy.deepcopy(SHOW); value[0]['instances'][0]['problems'] = [{'message': 'PUBLIC_CANARY_NOT_SECRET'}]; changed.append(value)
        for shown in changed:
            self.reject(lambda shown=shown: checks.shown_checks(shown, self.raw(), catalog=CATALOG))

    def test_show_bool_instance_key_not_equal_integer_zero(self):
        resource = 'google_test.item'
        catalog = checks.CheckCatalog(frozenset({('resource', resource)}), frozenset(), frozenset({''}), frozenset({resource + '[0]'}))
        raw = [{'object_kind': 'resource', 'config_addr': resource, 'status': 'pass', 'objects': [{'object_addr': resource + '[0]', 'status': 'pass'}]}]
        shown = [{'address': {'kind': 'resource', 'mode': 'managed', 'type': 'google_test', 'name': 'item', 'to_display': resource}, 'status': 'pass',
                  'instances': [{'address': {'to_display': resource + '[0]', 'instance_key': False}, 'status': 'pass'}]}]
        self.reject(lambda: checks.shown_checks(shown, self.raw(raw, catalog), catalog=catalog))

    def test_proto_unknown_nested_fields_and_wrong_wire_rejected(self):
        for encoded in (group(RAW[0]) + field(5, 1), field(1, 'WRONG_WIRE') + field(2, 'var.environment'),
                        field(1, 4) + field(2, 'var.environment') + field(3, 1) + field(4, field(1, 'var.environment') + field(2, 1) + field(4, 1))):
            self.reject(lambda encoded=encoded: checks.decode_plan(plan([encoded]), catalog=CATALOG))

    def test_proto_duplicate_scalar_config_and_object_rejected(self):
        value = group(RAW[0])
        obj = field(1, 'var.environment') + field(2, 1)
        for groups in ([value + field(3, 1)], [value, value], [value + field(4, obj)]):
            self.reject(lambda groups=groups: checks.decode_plan(plan(groups), catalog=CATALOG))

    def test_proto_unknown_enums_zero_kind_and_failure_message_rejected(self):
        for encoded in (field(1, 9) + field(2, 'var.environment'), field(2, 'var.environment'),
                        field(1, 4) + field(2, 'var.environment') + field(3, 9),
                        field(1, 4) + field(2, 'var.environment') + field(3, 1) + field(4, field(1, 'var.environment') + field(2, 1) + field(3, 'PUBLIC_CANARY_NOT_SECRET'))):
            self.reject(lambda encoded=encoded: checks.decode_plan(plan([encoded]), catalog=CATALOG))

    def test_proto_missing_new_checks_cannot_join_before(self):
        self.reject(lambda: checks.verify_checks(plan([group(x) for x in RAW[:-1]]), SHOW, RAW, catalog=CATALOG))
        self.reject(lambda: checks.verify_checks(plan(), SHOW, RAW[:-1], catalog=CATALOG))

    def test_proto_source_versions_unknown_root_fields_and_bool_mode_rejected(self):
        for raw in (plan() + field(99, 1), plan() + field(1, 3), field(1, 4) + field(14, '1.16.4'),
                    b'\x80' * 11, plan() + field(20, 2), plan() + field(19, b'\xff'),
                    field(1, 3) + field(14, 'other') + field(17, 2)):
            self.reject(lambda raw=raw: checks.decode_plan(raw, catalog=CATALOG))
        self.reject(lambda: checks.decode_plan(plan(), expected_mode=True, catalog=CATALOG))

    def test_byte_depth_and_node_budgets(self):
        self.reject(lambda: checks.decode_plan(b'0' * (checks.MAX_BYTES + 1), catalog=CATALOG))
        value = None
        for _ in range(34): value = [value]
        self.reject(lambda: self.raw(value))
        self.reject(lambda: self.raw([None] * 65537))

    def test_planned_checks_are_copy_only_and_reject_changed_map(self):
        original = copy.deepcopy(RAW)
        expected = self.raw()
        planned = checks.planned_checks(RAW, expected, catalog=CATALOG)
        planned[0]['status'] = 'unknown'
        self.assertEqual(RAW, original)
        bad = dict(expected); bad.pop(next(iter(bad)))
        self.reject(lambda: checks.planned_checks(RAW, bad, catalog=CATALOG))


if __name__ == '__main__':
    unittest.main()
