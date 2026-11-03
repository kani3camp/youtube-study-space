"""Hosting smokeが通常稼働・メンテナンスと、検証不能を区別する契約。"""
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('hosting_smoke', Path(__file__).with_name('hosting-smoke.py'))
smoke = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)


class MaintenanceRoutingTest(unittest.TestCase):
    def test_anonymous_user_reaches_login_when_normal(self):
        self.assertEqual(smoke.protected_destination('/login', 200, {'maintenanceEnabled': False}),
                         ('/login', 'マイページへログイン'))

    def test_anonymous_mypage_visit_reaches_login_with_logout_flag(self):
        self.assertEqual(smoke.protected_destination('/mypage', 200, {'maintenanceEnabled': False}),
                         ('/login?logout=true', 'マイページへログイン'))

    def test_anonymous_user_reaches_maintenance_while_active(self):
        for route in ('/login', '/mypage'):
            with self.subTest(route=route):
                self.assertEqual(smoke.protected_destination(route, 200, {'maintenanceEnabled': True}),
                                 ('/maintenance', 'ただいまメンテナンス中です'))

    def test_unavailable_status_is_not_accepted_as_maintenance(self):
        for status in (401, 403, 404, 500, 503):
            with self.subTest(status=status), self.assertRaises(AssertionError):
                smoke.protected_destination('/login', status, {'maintenanceEnabled': True})

    def test_malformed_status_is_not_accepted_as_normal_or_maintenance(self):
        for payload in (None, [], {}, {'maintenanceEnabled': 'false'},
                        {'maintenanceEnabled': 0}, {'maintenanceEnabled': 1},
                        {'maintenanceEnabled': None},
                        {'maintenanceEnabled': True, 'unexpected': 'synthetic-private'}):
            with self.subTest(payload=payload), self.assertRaises(AssertionError) as error:
                smoke.protected_destination('/login', 200, payload)
            self.assertNotIn('synthetic-private', str(error.exception))


if __name__ == '__main__':
    unittest.main()
