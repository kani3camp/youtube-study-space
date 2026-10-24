from pathlib import Path
from urllib.parse import urlencode, urlparse
import argparse
import json
from playwright.sync_api import sync_playwright

parser = argparse.ArgumentParser(description='Synthetic service-access QA against the isolated static runtime build')
parser.add_argument('--base-url', default='http://127.0.0.1:18087')
parser.add_argument('--output', default='/tmp/mypage-service-access-qa')
parser.add_argument('--chromium', default='/usr/bin/chromium')
args = parser.parse_args()
assert urlparse(args.base_url).hostname in ['127.0.0.1', 'localhost']
out = Path(args.output)
out.mkdir(parents=True, exist_ok=True)
results, errors, external = [], [], []

with sync_playwright() as p:
    # A dead loopback proxy blocks outbound traffic while Chromium bypasses it
    # for localhost. Request interception is avoided to preserve real bfcache.
    browser = p.chromium.launch(
        executable_path=args.chromium, headless=True,
        args=['--no-sandbox', '--proxy-server=http://127.0.0.1:9', '--proxy-bypass-list=127.0.0.1;localhost'],
        ignore_default_args=['--disable-back-forward-cache'],
    )
    context = browser.new_context(viewport={'width': 390, 'height': 960})
    page = context.new_page()
    page.set_default_timeout(10000)
    context.on('page', lambda opened: opened.on('pageerror', lambda error: errors.append(str(error))))
    page.on('pageerror', lambda error: errors.append(str(error)))
    context.on('request', lambda request: external.append(True) if not request.url.startswith(args.base_url + '/') else None)
    page.add_init_script("window.__bfRestored=false;window.addEventListener('pageshow',event=>{window.__bfRestored=event.persisted})")

    def load(path, mode=''):
        page.goto(args.base_url + '/runtime-visual.html?' + urlencode({'path': path, 'mode': mode}))

    def requests():
        return json.loads(page.locator('#synthetic-requests').inner_text())

    def private_absent():
        assert page.get_by_text('読書', exact=True).count() == 0
        assert page.get_by_text('Sample Channel', exact=True).count() == 0
        assert page.locator('.channel-identity, .task-name, .summary-grid, .recent-card, .account-dialog').count() == 0

    def storage_safe():
        stored = page.evaluate('({local:Object.fromEntries(Object.entries(localStorage)),session:Object.fromEntries(Object.entries(sessionStorage))})')
        assert all(key == 'oss.analytics-consent.v1' and value in ['granted', 'denied'] for key, value in stored['local'].items())
        assert all(key == 'tsr-scroll-restoration-v1_3' and value == '{}' for key, value in stored['session'].items())

    for reason, heading in [
        ('moderation', 'サービスの利用を制限しています'),
        ('deletion', '保存データの削除手続き中です'),
    ]:
        for stage in ['confirm', 'completion', 'old-session']:
            mode = f'restricted-{reason}-{stage}'
            if stage == 'old-session':
                mode = f'authenticated-restricted-{reason}'
            load('/mypage' if stage == 'old-session' else '/login/channel-confirm', mode)
            if stage != 'old-session':
                page.get_by_role('button', name='このチャンネルでログイン').click()
            page.get_by_role('heading', name=heading, exact=True).wait_for()
            private_absent()
            paths = requests()
            assert paths.count('/api/auth/youtube/confirm') == (0 if stage == 'old-session' else 1)
            assert paths.count('/api/auth/session/complete') == (0 if stage == 'confirm' else 1)
            assert '/api/mypage' not in paths
            page.wait_for_timeout(200)
            assert requests() == paths
            storage_safe()
            results.append({'case': stage, 'reason': reason, 'dedicatedRestriction': True, 'privateDataDisplayed': False, 'denialRetries': 0})

        for width in [320, 390, 1440]:
            page.set_viewport_size({'width': width, 'height': 960})
            load('/mypage', 'authenticated')
            page.get_by_text('読書', exact=True).wait_for()
            page.locator('#synthetic-restrict-' + reason).click()
            page.get_by_role('heading', name=heading, exact=True).wait_for()
            private_absent()
            page.evaluate('document.fonts.ready')
            assert page.evaluate('document.documentElement.scrollWidth<=innerWidth+1')
            page.screenshot(path=str(out / f'restricted-{reason}-{width}.png'), full_page=True, style='.synthetic-controls { visibility: hidden; }')
            paths = requests()
            if width == 390:
                # Actual cross-document navigation/back, requiring persisted=true.
                page.goto(args.base_url + '/')
                page.go_back(wait_until='commit')
                page.get_by_role('heading', name=heading, exact=True).wait_for()
                assert page.evaluate('window.__bfRestored===true'), 'Actual bfcache return was not observed'
                private_absent()
                assert requests() == paths
                # Consent UI remains independent, including a real same-origin tab.
                page.get_by_role('button', name='Cookie設定', exact=True).click()
                page.get_by_role('switch').check()
                other = context.new_page()
                other.goto(args.base_url + '/runtime-visual.html?' + urlencode({'path': '/'}))
                other.get_by_role('button', name='Cookie設定', exact=True).click()
                other.get_by_role('switch').uncheck()
                page.wait_for_function("localStorage.getItem('oss.analytics-consent.v1')==='denied'")
                assert not page.get_by_role('switch').is_checked()
                page.get_by_role('button', name='閉じる', exact=True).click()
                other.close()
                assert requests() == paths
                page.get_by_role('heading', name=heading, exact=True).wait_for()
                private_absent()
                for link, target in [('プライバシー', 'プライバシーポリシー'), ('利用規約', '利用規約'), ('お問い合わせ', 'お問い合わせ'), ('トップへ戻る', '自分のペースで、少しずつ。')]:
                    if link == 'トップへ戻る':
                        load('/mypage', f'authenticated-restricted-{reason}')
                        page.get_by_role('heading', name=heading, exact=True).wait_for()
                    page.get_by_role('link', name=link, exact=True).first.click()
                    page.get_by_role('heading', name=target, exact=True).wait_for()
                    private_absent()
                load('/mypage', f'authenticated-restricted-{reason}')
                page.get_by_role('heading', name=heading, exact=True).wait_for()
                page.get_by_role('link', name='新しくログインする', exact=True).click()
                page.get_by_role('heading', name='マイページへログイン', exact=True).wait_for()
                inputs = page.locator('.login-card input[type=checkbox]')
                inputs.nth(0).check()
                inputs.nth(1).check()
                assert page.get_by_role('button', name='YouTubeでログイン', exact=True).is_enabled()
                load('/mypage', f'authenticated-restricted-{reason}')
                page.get_by_role('button', name='ログアウト', exact=True).click()
                page.get_by_role('heading', name='マイページへログイン', exact=True).wait_for()
                private_absent()
                results.append({'case': 'real-bfcache-consent-cross-tab-legal-contact-home-fresh-login-logout', 'reason': reason, 'persisted': True, 'privateDataDisplayed': False})
            storage_safe()
            results.append({'case': 'existing-data-clear', 'reason': reason, 'width': width, 'privateDataDisplayed': False, 'horizontalOverflow': False})

    page.set_viewport_size({'width': 390, 'height': 960})
    load('/login/channel-confirm', 'support-confirm-delete-authenticated-restricted-deletion')
    page.get_by_role('button', name='この依頼の本人確認を完了', exact=True).click()
    page.get_by_role('heading', name='依頼の本人確認が完了しました', exact=True).wait_for()
    assert page.locator('.proof-reference').inner_text() == 'b' * 64
    assert requests().count('/api/auth/session/complete') == 1
    assert '/api/mypage' not in requests()
    storage_safe()
    results.append({'case': 'support-proof-under-restriction', 'freshSupportCompleted': True, 'normalSignIn': False, 'privateDataDisplayed': False})

    assert not external, 'Unexpected outbound request (URLs are omitted)'
    assert not errors, errors
    browser.close()

report = {'syntheticOnly': True, 'realFirebaseOAuth': False, 'outboundProxyBlocked': True, 'externalRequests': len(external), 'actualBfcache': True, 'cases': results, 'pageErrors': errors}
(out / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
print(f'PASS: {len(results)} synthetic service-access browser cases; actual bfcache, no private storage, no outbound requests or page errors.')
