"""Execute the same fail-closed result policy used by CI Gate."""
import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest

import importlib.util
SCRIPT = Path(__file__).with_name('check-ci-results.py')
spec = importlib.util.spec_from_file_location('ci_results', SCRIPT)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


class GateResultsTest(unittest.TestCase):
    def fixture(self, **outputs):
        needs = {name: {'result': 'skipped'} for name in gate.DEPENDENCIES}
        needs['changes'] = {'result': 'success', 'outputs': dict.fromkeys(gate.GROUPS, 'false') | outputs}
        needs['repository']['result'] = 'success'
        return needs

    def test_unselected_domains_and_operations_can_skip(self):
        gate.validate(self.fixture(), 'false')

    def test_all_selected_domains_must_succeed(self):
        needs = self.fixture(all='true')
        for name in gate.DOMAIN_GROUPS:
            needs[name]['result'] = 'success'
        gate.validate(needs, 'false')
        for name in gate.DOMAIN_GROUPS:
            changed = copy.deepcopy(needs)
            changed[name]['result'] = 'skipped'
            with self.subTest(name=name), self.assertRaises(ValueError):
                gate.validate(changed, 'false')

    def test_failure_cancellation_and_unknown_results_reach_gate(self):
        for name, result in (('system', 'failure'), ('frontend', 'cancelled'),
                             ('tools', 'unknown')):
            needs = self.fixture()
            needs[name]['result'] = result
            with self.subTest(name=name), self.assertRaises(ValueError):
                gate.validate(needs, 'false')

    def test_detection_or_repository_skip_cannot_pass(self):
        for name in ('changes', 'repository'):
            needs = self.fixture()
            needs[name]['result'] = 'skipped'
            with self.subTest(name=name), self.assertRaises(ValueError):
                gate.validate(needs, 'false')

    def test_missing_dependencies_or_detection_outputs_fail_closed(self):
        needs = self.fixture()
        del needs['system']
        with self.assertRaises(ValueError):
            gate.validate(needs, 'false')
        for bad in (None, '', 'TRUE', True):
            needs = self.fixture(system=bad)
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                gate.validate(needs, 'false')

    def test_requested_operation_cannot_skip_or_fail(self):
        needs = self.fixture()
        for status in ('skipped', 'failure', 'cancelled'):
            needs['operations']['result'] = status
            with self.subTest(status=status), self.assertRaises(ValueError):
                gate.validate(needs, 'true')
        needs['operations']['result'] = 'success'
        gate.validate(needs, 'true')
        for bad in ('', 'TRUE', '1', True):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                gate.validate(needs, bad)

    def test_real_gate_entrypoint_returns_nonzero_on_bad_or_missing_json(self):
        for payload, expected in ((json.dumps(self.fixture()), 0), ('{}', 1), ('not-json', 1), ('[]', 1)):
            env = dict(os.environ, CI_NEEDS=payload, CI_OPERATION_REQUESTED='false')
            result = subprocess.run([sys.executable, str(SCRIPT)], env=env, capture_output=True)
            self.assertEqual(result.returncode, expected, result.stderr)
