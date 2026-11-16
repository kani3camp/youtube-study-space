"""hosting-smoke.py の判定部分が、通常時・メンテナンス中・状態の取得失敗を正しく見分けるかをテストする。

目的: 状態が分からないのにメンテナンス扱い・通常扱いにしてしまう、ログイン画面とメンテナンス画面を取り違える、といった判定ミスを検出する。
使われる場面: CI Frontend ワークフロー(.github/workflows/ci-frontend.yml)の mypage-check ジョブの「Check Hosting smoke maintenance routing contracts」ステップ。
失敗したとき: CI Frontend が失敗し、PRをマージできない。
なくした場合: hosting-smoke.py の判定ミスが、公開後のDEV確認まで気づけなくなる。
"""
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
