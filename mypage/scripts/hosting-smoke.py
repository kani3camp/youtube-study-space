"""Anonymous DEV-only UI smoke. No login, GA4 opt-in, HAR, or screenshots."""
from urllib.parse import urlsplit
from playwright.sync_api import sync_playwright

origin = 'https://test--study-space.web.app'
analytics = []


def observe(request):
    host = urlsplit(request.url).hostname or ''
    if any(host == domain or host.endswith('.' + domain) for domain in (
        'googletagmanager.com', 'google-analytics.com', 'analytics.google.com',
    )):
        analytics.append(True)  # Do not retain URLs, queries, or identifiers.


with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True)
    context = browser.new_context(viewport={'width': 390, 'height': 960})
    page = context.new_page()
    page.on('request', observe)
    page.goto(origin + '/login', wait_until='networkidle')
    assert page.url == origin + '/login'
    assert page.get_by_role('heading', name='サイトをもっと使いやすくするために').count() == 0
    page.get_by_role('button', name='Cookie設定', exact=True).click()
    dialog = page.get_by_role('dialog', name='Cookie設定')
    dialog.get_by_text('任意のアクセス解析は現在利用していません。利用開始時にあらためてご案内します。', exact=True).wait_for()
    assert dialog.get_by_role('switch').count() == 0
    page.keyboard.press('Escape')
    for route in ('/privacy', '/terms', '/mypage'):
        page.goto(origin + route, wait_until='networkidle')
        assert page.url.startswith(origin + '/')
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1')
    assert not analytics, 'Unexpected GA4 request; URL intentionally omitted'
    browser.close()
print('PASS: live anonymous DEV UI, Cookie settings, GA4 requests=0. Real GA4 sending remains unverified.')
