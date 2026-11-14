"""ローカルの公開ルートを実ブラウザで遷移し、フォーカスとTab順を検証する。"""
import argparse
import json

from playwright.sync_api import expect, sync_playwright

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--chromium', default='/usr/bin/chromium')
args = parser.parse_args()
base = 'http://127.0.0.1:18081'
results = []


def check_heading(page, path, title):
    expect(page).to_have_url(base + path)
    heading = page.locator('main').get_by_role('heading', level=1, name=title)
    expect(heading).to_be_focused()
    assert page.evaluate('scrollY') == 0, (path, 'page must start at the top')


with sync_playwright() as p:
    browser = p.chromium.launch(executable_path=args.chromium, headless=True)
    for width, height in [(1180, 757), (390, 844)]:
        context = browser.new_context(viewport={'width': width, 'height': height}, service_workers='block')
        page = context.new_page()
        errors, external = [], []
        page.on('pageerror', lambda error: errors.append(str(error)))

        def guard(route):
            if route.request.url.startswith(base + '/'):
                route.continue_()
            else:
                external.append(route.request.resource_type)
                route.abort()

        page.route('**/*', guard)
        page.goto(base + '/contact')
        expect(page.locator('main h1')).to_be_visible()
        # 初回表示では利用者のフォーカスを奪わない。
        expect(page.locator('body')).to_be_focused()
        for path, title, label in [
            ('/privacy', 'プライバシーポリシー', 'プライバシー'),
            ('/terms', '利用規約', '利用規約'),
            ('/contact', 'お問い合わせ', 'お問い合わせ'),
        ]:
            link = page.locator('footer').get_by_role('link', name=label, exact=True)
            link.focus()
            page.keyboard.press('Enter')
            check_heading(page, path, title)
            page.keyboard.press('Tab')
            first_link = page.locator('main a[href]').first
            expect(first_link).to_be_focused()
            results.append({'width': width, 'path': path, 'tabScrollY': page.evaluate('scrollY'), 'tabTargetInMain': True})

        page.go_back()
        check_heading(page, '/terms', '利用規約')
        page.go_forward()
        check_heading(page, '/contact', 'お問い合わせ')

        # 同一ルートのsearch変更やダイアログの開閉で本文へ奪い返さない。
        cookie = page.get_by_role('button', name='Cookie設定', exact=True)
        cookie.focus()
        page.evaluate("history.pushState({}, '', '/contact?supportError=oauth_failed')")
        expect(page.locator('main [role="alert"]')).to_be_visible()
        expect(cookie).to_be_focused()
        page.evaluate("history.pushState({}, '', '/contact?supportError=oauth_failed#details')")
        expect(page).to_have_url(base + '/contact?supportError=oauth_failed#details')
        expect(cookie).to_be_focused()
        page.keyboard.press('Enter')
        expect(page.get_by_role('dialog', name='Cookie設定')).to_be_visible()
        page.keyboard.press('Escape')
        expect(cookie).to_be_focused()
        expect(page.get_by_role('dialog')).to_have_count(0)
        assert not errors and not external, (errors, external)
        context.close()
    browser.close()

print(json.dumps(results, ensure_ascii=False, indent=2))
print('PASS: footer keyboard navigation, main-content Tab order, back/forward, initial focus and Cookie dialog return (desktop/mobile; external requests=0).')
