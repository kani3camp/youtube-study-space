"""Synthetic, localhost-only acceptance for independent on-demand history."""
import argparse
import json
from pathlib import Path
from urllib.parse import urlencode, urlparse
from playwright.sync_api import sync_playwright

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--chromium', default='/usr/bin/chromium')
parser.add_argument('--base-url', default='http://127.0.0.1:18081')
parser.add_argument('--output', default='/tmp/mypage-history-qa')
args = parser.parse_args()
assert urlparse(args.base_url).hostname in ['127.0.0.1','localhost']
errors = []
with sync_playwright() as p:
    browser = p.chromium.launch(executable_path=args.chromium, headless=True)
    page = browser.new_page(viewport={'width':390, 'height':960})
    page.on('pageerror', lambda error: errors.append(str(error)))
    page.route('**/*', lambda route: route.continue_() if route.request.url.startswith(args.base_url+'/') else route.abort())
    page.clock.install()
    page.goto(args.base_url+'/runtime-visual.html?' + urlencode({'path':'/mypage','mode':'history-authenticated'}))
    page.get_by_text('読書', exact=True).wait_for()
    def count():
        return sum('/api/mypage/history?' in item for item in json.loads(page.locator('#synthetic-requests').inner_text()))
    assert count() == 0
    page.get_by_text('作業の記録を確認', exact=True).click()
    assert count() == 0
    page.get_by_role('button', name='この期間の記録を取得', exact=True).click()
    page.get_by_text('最初の合成記録', exact=True).wait_for()
    assert count() == 1
    page.clock.pause_at(page.evaluate('new Date(Date.now()+1000).toISOString()'))
    page.clock.run_for(300_000)
    assert count() == 1, 'Overview polling must not reload history'
    page.get_by_role('button', name='次の50件', exact=True).click()
    page.get_by_text('次の合成記録', exact=True).wait_for()
    assert count() == 2
    page.locator('#synthetic-switch').click()
    page.get_by_text('新しい合成作業', exact=True).wait_for()
    assert page.get_by_text('次の合成記録', exact=True).count() == 0
    assert count() == 2, 'UID change must not auto-load private history'
    page.clock.resume()
    page.goto(args.base_url+'/runtime-visual.html?' + urlencode({'path':'/mypage','mode':'history-quota-authenticated'}))
    page.get_by_text('読書', exact=True).wait_for()
    page.get_by_text('作業の記録を確認', exact=True).click()
    page.get_by_role('button', name='この期間の記録を取得', exact=True).click()
    page.get_by_text('履歴の取得上限に達しました。表示済みの記録は引き続き確認できます。', exact=True).wait_for()
    assert page.get_by_role('button', name='この期間の記録を取得', exact=True).is_disabled()
    assert page.get_by_text('読書', exact=True).count() == 1
    assert not errors, errors
    browser.close()
out = Path(args.output)
out.mkdir(parents=True, exist_ok=True)
(out/'report.json').write_text(json.dumps({'syntheticOnly':True,'onDemand':True,'noHistoryPolling':True,'uidClear':True,'quotaKeepsOverview':True},indent=2))
print('PASS: on-demand history, independent polling, uid clear and quota isolation')
