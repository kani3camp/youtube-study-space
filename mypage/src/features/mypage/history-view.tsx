import { useEffect, useRef, useState } from 'react'
import { formatWorkSeconds } from '../../lib/format'
import type { BrowserRuntime } from '../../runtime'
import { type HistoryPage, HistoryRequestError } from './history'
import { RequestError } from './memory'

function jstDate(offset: number) {
	return new Date(Date.now() + 9 * 3600_000 + offset * 86400_000)
		.toISOString()
		.slice(0, 10)
}
function time(value: string) {
	return new Intl.DateTimeFormat('ja-JP', {
		timeZone: 'Asia/Tokyo',
		dateStyle: 'short',
		timeStyle: 'short',
	}).format(new Date(value))
}

export function HistoryPanel({ runtime }: { runtime: BrowserRuntime }) {
	const [from, setFrom] = useState(() => jstDate(-6))
	const [to, setTo] = useState(() => jstDate(1))
	const [page, setPage] = useState<HistoryPage | null>(null)
	const [busy, setBusy] = useState(false)
	const [error, setError] = useState<string | null>(null)
	const [resetAt, setResetAt] = useState<string | null>(null)
	const controller = useRef<AbortController | null>(null)
	useEffect(() => () => controller.current?.abort(), [])
	useEffect(() => {
		if (!resetAt) return
		const timer = setTimeout(
			() => setResetAt(null),
			Math.max(0, Date.parse(resetAt) - Date.now()),
		)
		return () => clearTimeout(timer)
	}, [resetAt])
	const load = async (next = false) => {
		if (controller.current || (resetAt && Date.parse(resetAt) > Date.now()))
			return
		const current = new AbortController()
		controller.current = current
		setBusy(true)
		setError(null)
		try {
			const input =
				next && page
					? {
							from: page.from,
							to: page.to,
							cursor: page.nextCursor ?? undefined,
						}
					: { from, to }
			const result = await runtime.historyPage(
				{ ...input, limit: 50 },
				current.signal,
			)
			if (!current.signal.aborted && controller.current === current)
				setPage(result)
		} catch (failure) {
			if (current.signal.aborted) return
			if (failure instanceof HistoryRequestError && failure.status === 429) {
				setResetAt(
					failure.resetAt ??
						new Date(Date.now() + failure.retryAfter * 1000).toISOString(),
				)
				setError(
					'履歴の取得上限に達しました。表示済みの記録は引き続き確認できます。',
				)
			} else if (
				failure instanceof RequestError &&
				['FEATURE_UNAVAILABLE', 'HISTORY_NOT_READY'].includes(failure.code)
			)
				setError(
					'この期間の履歴はまだ利用できません。基本画面の記録は引き続き確認できます。',
				)
			else if (
				failure instanceof RequestError &&
				failure.code === 'HISTORY_LIMIT_EXCEEDED'
			)
				setError('対象の記録が多いため、期間を短くしてお試しください。')
			else if (
				failure instanceof RequestError &&
				failure.code === 'INVALID_CURSOR'
			)
				setError(
					'取得の有効期限が切れたか、条件が変わりました。期間を指定して取得し直してください。',
				)
			else
				setError(
					'履歴を取得できませんでした。少し待ってから再試行してください。',
				)
		} finally {
			if (controller.current === current) {
				controller.current = null
				setBusy(false)
			}
		}
	}
	return (
		<details className="card history-panel">
			<summary>作業の記録を確認</summary>
			<p className="muted">
				指定期間に完了した作業を50件ずつ表示します。過去の記録が不完全な期間もあります。日付は日本時間です。
			</p>
			<form
				onSubmit={(event) => {
					event.preventDefault()
					void load()
				}}
			>
				<label>
					開始日
					<input
						type="date"
						value={from}
						onChange={(event) => setFrom(event.target.value)}
						required
					/>
				</label>
				<label>
					終了日（この日を含まない）
					<input
						type="date"
						value={to}
						onChange={(event) => setTo(event.target.value)}
						required
					/>
				</label>
				<button
					className="button"
					type="submit"
					disabled={busy || resetAt !== null}
				>
					この期間の記録を取得
				</button>
			</form>
			{error && <p role="alert">{error}</p>}
			{resetAt && <p>再開できる時刻：{time(resetAt)}（日本時間）</p>}
			{page && (
				<>
					<p className="muted">
						{page.from}〜{page.to}未満 ／ {time(page.asOf)}時点の記録
					</p>
					{!page.sourceConsistent && (
						<p role="status">記録に重複などの不整合がある可能性があります。</p>
					)}
					{page.items.length === 0 ? (
						<p>この期間に完了した作業の記録はありません。</p>
					) : (
						<ul>
							{page.items.map((entry) => (
								<li key={entry.key}>
									<p>{entry.workName || '作業名の設定なし'}</p>
									<p className="muted">
										{time(entry.startedAt)}〜{time(entry.endedAt)} ／{' '}
										{formatWorkSeconds(entry.workSec)}
									</p>
								</li>
							))}
						</ul>
					)}
					{page.nextCursor && (
						<button
							className="button"
							type="button"
							disabled={busy || resetAt !== null}
							onClick={() => void load(true)}
						>
							次の50件
						</button>
					)}
				</>
			)}
			{busy && <p role="status">履歴を取得しています…</p>}
		</details>
	)
}
