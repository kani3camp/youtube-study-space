"""DEV専用の匿名UI疎通確認。ログイン・GA4への送信・HARは使わない。"""
from urllib.parse import urlsplit

ORIGIN = 'https://test--study-space.web.app'
STATUS_PATH = '/api/public/mypage-status'


def protected_destination(route, status, payload):
    """公開APIで確認できた状態だけを受け入れ、匿名利用者の到達先を返す。"""
    if (status != 200 or not isinstance(payload, dict)
            or set(payload) != {'maintenanceEnabled'}
            or type(payload['maintenanceEnabled']) is not bool):
        raise AssertionError('Invalid public maintenance status; response intentionally omitted')
    if payload['maintenanceEnabled']:
        return '/maintenance', 'ただいまメンテナンス中です'
    destination = '/login?logout=true' if route == '/mypage' else '/login'
    return destination, 'マイページへログイン'


def check_protected_page(page, route, origin=ORIGIN):
    from playwright.sync_api import expect

    # 実際の画面が読んだ状態を使い、別リクエストとの状態の食い違いを避ける。
    # networkidleは常時通信のあるページで成立しないことがあるため使わない。
    # 準備完了はステータス応答と見出しの表示で判定する。
    with page.expect_response(origin + STATUS_PATH) as pending:
        page.goto(origin + route, wait_until='domcontentloaded')
    response = pending.value
    try:
        payload = response.json()
    except ValueError:
        raise AssertionError('Invalid public maintenance JSON; response intentionally omitted') from None
    destination, heading = protected_destination(route, response.status, payload)
    expect(page).to_have_url(origin + destination)
    expect(page.get_by_role('heading', name=heading, exact=True)).to_be_visible()


def smoke(page, origin=ORIGIN):
    from playwright.sync_api import expect

    analytics = []

    def observe(request):
        host = urlsplit(request.url).hostname or ''
        if any(host == domain or host.endswith('.' + domain) for domain in (
            'googletagmanager.com', 'google-analytics.com', 'analytics.google.com',
        )):
            analytics.append(True)  # Do not retain URLs, queries, or identifiers.

    page.on('request', observe)
    check_protected_page(page, '/login', origin)
    banner = page.get_by_role('heading', name='サイトをもっと使いやすくするために')
    expect(banner).to_be_visible()
    assert not analytics, 'Unexpected GA4 request before consent'
    page.get_by_role('button', name='許可しない', exact=True).click()
    expect(banner).to_have_count(0)
    page.get_by_role('button', name='Cookie設定', exact=True).click()
    dialog = page.get_by_role('dialog', name='Cookie設定')
    expect(dialog).to_be_visible()
    expect(dialog.get_by_role('switch', name='アクセス解析を許可')).not_to_be_checked()
    page.keyboard.press('Escape')
    expect(dialog).to_be_hidden()
    for route, heading in (('/privacy', 'プライバシーポリシー'), ('/terms', '利用規約')):
        # networkidleは常時通信のあるページで成立しないことがあるため使わない。
        # 準備完了はURLと見出しの表示で判定する。
        page.goto(origin + route, wait_until='domcontentloaded')
        expect(page).to_have_url(origin + route)
        expect(page.get_by_role('heading', name=heading, exact=True)).to_be_visible()
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1')
    check_protected_page(page, '/mypage', origin)
    assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1')
    expect(banner).to_have_count(0)
    assert not analytics, 'Unexpected GA4 request; URL intentionally omitted'


def main():
    from playwright.sync_api import sync_playwright

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        context = browser.new_context(viewport={'width': 390, 'height': 960})
        smoke(context.new_page())
        browser.close()
    print('PASS: live anonymous DEV UI, maintenance routing, Cookie settings, GA4 requests=0. Real GA4 sending remains unverified.')


if __name__ == '__main__':
    main()
