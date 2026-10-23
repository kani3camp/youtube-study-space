"""Opt-in browser -> native HTTPS -> Go handler acceptance; no API fulfillment."""

import argparse
import json
import re
import sys
import traceback
from datetime import date, datetime, timedelta, timezone
from pathlib import Path
from urllib.parse import parse_qs, urlencode, urlsplit

from playwright.sync_api import sync_playwright


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--foreign", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--chromium", default="/usr/bin/chromium")
    parser.add_argument("--trusted-spki", required=True)
    args = parser.parse_args()
    assert re.fullmatch(r"[A-Za-z0-9+/]{43}=", args.trusted_spki)
    for address in (args.base, args.foreign):
        parsed = urlsplit(address)
        assert parsed.scheme == "https" and parsed.hostname == "127.0.0.1"
        assert parsed.port and not parsed.path and not parsed.query
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    results, responses, errors, blocked = [], [], [], []
    provider_interceptions = []

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(
            executable_path=args.chromium, headless=True,
            # Pin only httptest's public certificate. A cross-origin POST must
            # reach Go rather than stop at Chromium's certificate interstitial.
            args=["--no-sandbox", "--ignore-certificate-errors-spki-list=" + args.trusted_spki],
        )

        def new_page(scenario):
            context = browser.new_context(
                ignore_https_errors=True, viewport={"width": 390, "height": 960},
                service_workers="block",
            )
            response = context.request.get(args.base + "/_qa/scenario?" + urlencode({"name": scenario}))
            assert response.status == 204
            page = context.new_page()
            page.on("pageerror", lambda error: errors.append(str(error)))

            def route(request):
                target = urlsplit(request.request.url)
                if target.scheme == "https" and target.netloc in {
                    urlsplit(args.base).netloc, urlsplit(args.foreign).netloc
                }:
                    request.continue_()
                elif target.scheme == "https" and target.netloc == "accounts.google.com" and target.path == "/o/oauth2/v2/auth":
                    # Fake provider navigation only; never contact Google and
                    # never fulfill /api. Keep state in memory, out of evidence.
                    state = parse_qs(target.query)["state"][0]
                    provider_interceptions.append(True)
                    request.fulfill(
                        status=302, body="",
                        headers={"Location": args.base + "/api/auth/youtube/callback?" + urlencode({
                            "state": state, "code": "synthetic-browser-code"
                        })},
                    )
                else:
                    blocked.append(target.scheme + "://" + target.netloc + target.path)
                    request.abort()

            context.route("**/*", route)

            def record(response):
                path = urlsplit(response.url).path
                if not path.startswith("/api/"):
                    return
                event = {"method": response.request.method, "path": path, "status": response.status}
                if path == "/api/mypage" and response.status == 200:
                    value = response.json()
                    event["generatedAt"] = value["generatedAt"]
                    event["summary"] = value["summary"]
                    event["recent7Days"] = value["recent7Days"]
                responses.append(event)
                (output / "progress.json").write_text(json.dumps({
                    "completedCases": results,
                    "responses": responses,
                }, ensure_ascii=False, indent=2))

            page.on("response", record)
            return context, page

        def login(page, context):
            page.goto(args.base + "/login")
            page.get_by_role("heading", name="マイページへログイン", exact=True).wait_for()
            page.get_by_role("checkbox").nth(0).check()
            page.get_by_role("checkbox").nth(1).check()
            page.get_by_role("button", name="YouTubeでログイン", exact=True).click()
            try:
                page.get_by_role("button", name="このチャンネルでログイン", exact=True).wait_for()
            except Exception:
                # Fixed booleans only: never persist a URL, body or exception.
                (output / "channel-diagnostic.json").write_text(json.dumps({
                    "confirmationRoute": urlsplit(page.url).path == "/login/channel-confirm",
                    "confirmationHeading": page.get_by_role("heading", name="YouTubeチャンネルを確認", exact=True).count() > 0,
                    "alertPresent": page.get_by_role("alert").count() > 0,
                    "statusPresent": page.get_by_role("status").count() > 0,
                }))
                raise
            cookies = [c for c in context.cookies() if c["name"] == "__session"]
            assert len(cookies) == 1 and cookies[0]["secure"] and cookies[0]["httpOnly"]
            assert cookies[0]["sameSite"] == "Lax"
            with page.expect_response(lambda r: urlsplit(r.url).path == "/api/mypage" and r.status == 200) as loaded:
                page.get_by_role("button", name="このチャンネルでログイン", exact=True).click()
            loaded.value.finished()
            page.get_by_role("heading", name="作業サマリー", exact=True).wait_for()
            page.wait_for_function("document.querySelector('.summary-card')?.getAttribute('aria-busy')==='false' && document.querySelector('.metric-lifetime .sr-only')?.textContent==='0時間 10分'")
            assert not [c for c in context.cookies() if c["name"] == "__session"]

        for scenario in ("unknown", "complete", "gap"):
            context, page = new_page(scenario)
            offset = len(responses)
            page.goto(args.base + "/mypage")
            page.get_by_role("heading", name="マイページへログイン", exact=True).wait_for()
            assert not [r for r in responses[offset:] if r["path"] == "/api/mypage"]
            login(page, context)
            value = [r for r in responses[offset:] if r["path"] == "/api/mypage" and r["status"] == 200][-1]
            today = value["summary"]["data"]["today"]
            recent = value["recent7Days"]["data"]
            if scenario == "unknown":
                assert today["workSec"] is None and today["reasonCode"] == "HISTORY_INCOMPLETE"
                assert all(day["workSec"] is None for day in recent)
                recorded = [day for day in recent if "recordedWorkSec" in day]
                assert sum(day["recordedWorkSec"] for day in recorded) == 600
                assert page.locator(".recorded-bar").count() == len(recorded)
                assert page.locator(".unknown-bar").count() == 7 - len(recorded)
                assert page.locator(".recorded-legend").inner_text() == "斜線は記録済み分（集計未確定）"
                # The real Firestore readTime can cross JST midnight; inspect
                # the positive day actually returned rather than assuming today.
                first = recorded[0]
                minutes = first["recordedWorkSec"] // 60
                duration = "1分未満" if minutes == 0 else f"{minutes // 60}時間 {minutes % 60}分"
                calendar_day = date.fromisoformat(first["date"])
                today_jst = datetime.fromisoformat(value["generatedAt"].replace("Z", "+00:00")).astimezone(timezone(timedelta(hours=9))).date()
                weekday = "月火水木金土日"[calendar_day.weekday()]
                human_date = f"{calendar_day.month}月{calendar_day.day}日（{weekday}{'・今日' if calendar_day == today_jst else ''}）"
                label = human_date + ": 記録済み " + duration + "（集計未確定）"
                page.get_by_role("button", name=label, exact=True).click()
                selected = page.locator(".selected-day").inner_text()
                assert human_date in selected and "記録済み " + duration in selected
                assert page.locator(".recent-card").get_by_text("記録がそろっているか未確認です。", exact=True).is_visible()
                page.locator(".day-button").filter(has=page.locator(".unknown-bar")).first.click()
                assert page.locator(".selected-day").inner_text().endswith("取得できません")
                assert "recordedWorkSec" not in recent[0]
            elif scenario == "complete":
                assert today["availability"] == "available"
                assert all(day["availability"] == "available" for day in recent)
                assert sum(day["workSec"] for day in recent) == 600
                assert page.locator(".metric-today .duration-value").count() == 1
                assert page.locator(".unknown-bar").count() == 0
            else:
                gaps = [day for day in recent if day["workSec"] is None]
                assert len(gaps) == 1 and gaps[0]["reasonCode"] == "HISTORY_INCOMPLETE"
                assert today["availability"] == "available"
                assert page.locator(".unknown-bar").count() == 1
                page.locator(".day-button").filter(has=page.locator(".unknown-bar")).click()
                assert page.locator(".selected-day").inner_text().endswith("取得できません")
            assert value["summary"]["data"]["lifetime"]["workSec"] == 600
            expected = {
                ("/api/auth/youtube/start", 200), ("/api/auth/youtube/callback", 302),
                ("/api/auth/youtube/channel", 200), ("/api/auth/youtube/confirm", 200),
                ("/api/auth/session/complete", 204), ("/api/mypage", 200),
            }
            assert expected <= {(r["path"], r["status"]) for r in responses[offset:]}
            page.screenshot(path=str(output / (scenario + ".png")), full_page=True)
            page.get_by_role("button", name="アカウントを開く").click()
            page.get_by_role("button", name="ログアウト", exact=True).click()
            page.get_by_role("heading", name="マイページへログイン", exact=True).wait_for()
            assert page.locator(".summary-card").count() == 0
            assert page.evaluate("Object.keys(localStorage).length") == 0
            results.append("anonymous guard -> browser OAuth/cookie -> confirm -> session -> " + scenario + " history -> logout clears UI")
            context.close()

        for kind, heading in (
            ("moderation", "サービスの利用を制限しています"),
            ("deletion", "保存データの削除手続き中です"),
        ):
            context, page = new_page("complete")
            login(page, context)
            assert context.request.get(args.base + "/_qa/restrict?" + urlencode({"kind": kind})).status == 204
            with page.expect_response(lambda r: urlsplit(r.url).path == "/api/mypage" and r.status == 403):
                page.get_by_role("button", name="最新の情報に更新").click()
            page.get_by_role("heading", name=heading, exact=True).wait_for()
            assert page.locator(".summary-card").count() == 0
            page.screenshot(path=str(output / (kind + ".png")), full_page=True)
            results.append(kind + " restriction after cached success clears private UI")
            context.close()

        context, page = new_page("complete")
        login(page, context)
        denied = page.evaluate("() => window.httpQA.probe({Authorization:''})")
        assert denied["status"] == 401 and denied["body"]["error"]["code"] == "AUTH_REQUIRED"
        denied = page.evaluate("() => window.httpQA.probe({'X-Firebase-AppCheck':'invalid-public-dummy'})")
        assert denied["status"] == 403 and denied["body"]["error"]["code"] == "APP_CHECK_REQUIRED"
        denied = page.evaluate("() => window.httpQA.probe({Authorization:'Bearer invalid-public-dummy'})")
        assert denied["status"] == 401 and denied["body"]["error"]["code"] == "AUTH_REQUIRED"
        results.append("native browser fetch: missing/bad ID and bad AppCheck denied by real handler")
        context.close()

        context, page = new_page("complete")
        login(page, context)
        for _ in range(9):
            assert page.evaluate("() => window.httpQA.probe()")["status"] == 200
        with page.expect_response(lambda r: urlsplit(r.url).path == "/api/mypage" and r.status == 429) as limited:
            page.get_by_role("button", name="最新の情報に更新").click()
        assert int(limited.value.headers["retry-after"]) > 0
        page.get_by_text("最新の情報に更新できませんでした。", exact=False).wait_for()
        page.screenshot(path=str(output / "rate-limit.png"), full_page=True)
        results.append("handler 429/Retry-After reaches existing UI refresh error state without false success")
        context.close()

        context, page = new_page("complete")
        page.goto(args.base + "/login")
        page.get_by_role("heading", name="マイページへログイン", exact=True).wait_for()
        page.goto(args.foreign)
        with page.expect_response(lambda r: urlsplit(r.url).path == "/api/auth/session/complete") as refused:
            page.get_by_role("button", name="foreign origin").click()
        assert refused.value.status == 400
        assert refused.value.request.headers["origin"] == args.foreign
        assert refused.value.json()["error"]["code"] == "INVALID_REQUEST"
        results.append("real cross-origin browser form POST rejected by Go Origin boundary")
        context.close()

        context, page = new_page("complete")
        page.goto(args.base + "/api/auth/youtube/callback?" + urlencode({
            "code": "synthetic-browser-code", "state": "synthetic-invalid-state"
        }))
        page.get_by_role("heading", name="マイページへログイン", exact=True).wait_for()
        assert "code=" not in page.url and "state=" not in page.url
        assert "synthetic-browser-code" not in page.locator("body").inner_text()
        results.append("invalid callback redirects to fixed error without code/state in UI or final URL")
        context.close()
        browser.close()

    assert not errors, "Browser page errors"
    assert not blocked, "Unexpected nonlocal requests"
    report = {
        "syntheticOnly": True, "realGoogleFirebaseHosting": False,
        "nativeAPIHTTP": True, "apiResponsesFulfilledByPlaywright": False,
        "fakeProviderNavigations": len(provider_interceptions), "cases": results,
        "responses": responses, "pageErrors": errors, "unexpectedRequests": blocked,
    }
    (output / "browser-report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2))
    (output / "progress.json").write_text(json.dumps({
        "status": "passed", "completedCases": results, "responses": responses,
    }, ensure_ascii=False, indent=2))
    print("PASS: browser -> native HTTPS -> Go handler synthetic acceptance")


if __name__ == "__main__":
    try:
        main()
    except Exception as failure:
        # Do not emit Playwright call logs, request URLs, headers or raw errors.
        frame = [frame for frame in traceback.extract_tb(failure.__traceback__)
                 if frame.filename == __file__][-1]
        output_arg = sys.argv[sys.argv.index("--output") + 1]
        Path(output_arg).mkdir(parents=True, exist_ok=True)
        kind = type(failure).__name__
        kind = kind if kind in {"AssertionError", "TimeoutError", "Error"} else "Failure"
        Path(output_arg, "failure.json").write_text(json.dumps({
            "status": "failed", "kind": kind, "sourceLine": frame.lineno
        }))
        print("FAIL: synthetic browser HTTP acceptance (fixed diagnostic only)")
        raise SystemExit(1) from None
