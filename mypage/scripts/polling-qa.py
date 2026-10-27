"""Exercise the real router and polling gate with synthetic auth/data only."""
import argparse
import json
import re
from pathlib import Path
from urllib.parse import urlencode

from playwright.sync_api import sync_playwright

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--chromium', default='/usr/bin/chromium')
parser.add_argument('--output', default='/tmp/mypage-polling-qa')
args = parser.parse_args()
results = []
errors = []
with sync_playwright() as p:
    browser = p.chromium.launch(executable_path=args.chromium, headless=True)
    for path, expected in [('/mypage', 1), ('/mypage/', 1),
                           ('/mypage/account', 0), ('/privacy', 0),
                           ('/mypage/unknown', 0), ('/unknown', 0)]:
        page = browser.new_page()
        page.on('pageerror', lambda error: errors.append(str(error)))
        page.route('**/*', lambda route: route.continue_()
                   if route.request.url.startswith('http://127.0.0.1:18081/')
                   else route.abort())
        page.clock.install(time='2026-10-24T02:00:00Z')
        page.goto('http://127.0.0.1:18081/runtime-visual.html?' +
                  urlencode({'path': path, 'mode': 'polling-authenticated'}))
        # This dedicated visual-fixture mode restores synthetic auth without a
        # timer so the installed clock controls only the polling behavior under
        # test. Other runtime QA continues to cover delayed auth restoration.
        page.clock.run_for(1)
        if path.startswith('/mypage'):
            page.locator(
                '#synthetic-requests', has_text='/api/auth/session/complete'
            ).wait_for(state='attached')
        if expected:
            page.get_by_text('読書', exact=True).wait_for()
        else:
            page.get_by_role('heading', level=1).wait_for()
        page.clock.pause_at(page.evaluate('new Date(Date.now() + 1000).toISOString()'))

        def reads():
            return json.loads(page.locator('#synthetic-requests').inner_text()).count('/api/mypage')

        assert reads() == expected, (path, 'initial', reads())
        page.clock.run_for(300_000)
        assert reads() == expected * 2, (path, 'poll', reads())
        if expected:
            page.locator('#synthetic-account-route').click()
            page.get_by_role('heading', name='アカウント管理', exact=True).wait_for()
            page.clock.run_for(600_000)
            assert reads() == 2, (path, 'away', reads())
            page.get_by_role('link', name='マイページへ戻る', exact=True).click()
            page.get_by_text('読書', exact=True).wait_for()
            page.locator(
                '#synthetic-requests',
                has_text=re.compile(r'(?:/api/mypage.*){3}'),
            ).wait_for(state='attached')
            assert reads() == 3, (path, 'return', reads())
        results.append({'route': path, 'initialReads': expected, 'passed': True})
        page.close()
    browser.close()
assert not errors, errors
out = Path(args.output)
out.mkdir(parents=True, exist_ok=True)
(out / 'report.json').write_text(json.dumps({'syntheticOnly': True, 'cases': results}, indent=2))
print('PASS: overview-only polling, trailing slash, account/legal/404 and route return')
