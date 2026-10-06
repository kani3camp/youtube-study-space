import { type ReactNode, useEffect, useRef, useState } from 'react'
import { formatWorkSeconds } from '../../lib/format'
import type { Account, Current } from './contract'
import type { MemoryState, Retained, ViewData } from './memory'

export const guideURL =
	'https://kani3camp.github.io/youtube-study-space/docs/essential'
export const liveURL =
	'https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg/live'

const dateTime = new Intl.DateTimeFormat('ja-JP', {
	timeZone: 'Asia/Tokyo',
	month: 'numeric',
	day: 'numeric',
	hour: '2-digit',
	minute: '2-digit',
})
const clock = new Intl.DateTimeFormat('ja-JP', {
	timeZone: 'Asia/Tokyo',
	hour: '2-digit',
	minute: '2-digit',
	hour12: false,
})
const day = new Intl.DateTimeFormat('sv-SE', {
	timeZone: 'Asia/Tokyo',
	year: 'numeric',
	month: '2-digit',
	day: '2-digit',
})

function timestamp(value: string | null) {
	return value && Number.isFinite(Date.parse(value))
		? dateTime.format(new Date(value))
		: '時刻不明'
}
function time(value: string | null) {
	return value && Number.isFinite(Date.parse(value))
		? clock.format(new Date(value))
		: '—'
}

export function ExternalLink({
	href,
	children,
	primary = false,
}: {
	href: string
	children: ReactNode
	primary?: boolean
}) {
	return (
		<a
			className={primary ? 'button primary' : 'external-link'}
			href={href}
			target="_blank"
			rel="noopener noreferrer"
		>
			{children}
			<span aria-hidden="true"> ↗</span>
			<span className="sr-only">（別タブで開く）</span>
		</a>
	)
}

function StaleNote({ value }: { value: Retained<unknown> }) {
	if (!value.stale) return null
	return (
		<p className="stale-note">
			{value.data === null
				? '情報を取得できませんでした'
				: `更新できませんでした · ${timestamp(value.asOf)}時点`}
		</p>
	)
}

function CurrentCard({
	value,
	busy,
	retry,
}: {
	value?: Retained<Current>
	busy: boolean
	retry: () => void
}) {
	const current = value?.data
	if (!current)
		return (
			<section
				className="card current-card"
				aria-labelledby="current-heading"
				aria-busy={busy}
			>
				<h2 id="current-heading">現在の作業</h2>
				{busy ? (
					<p role="status">読み込んでいます…</p>
				) : (
					<>
						<p>情報を取得できませんでした</p>
						<p className="muted">
							記録が消えたわけではありません。少し待ってからお試しください。
						</p>
						<button className="button" type="button" onClick={retry}>
							再試行
						</button>
					</>
				)}
			</section>
		)
	const active = current.state === 'work' || current.state === 'break'
	const resting = current.state === 'break'
	const start = Date.parse(current.stateStartedAt ?? '')
	const end = Date.parse(current.expectedEndAt ?? '')
	const asOf = Date.parse(value?.asOf ?? '')
	const timeline =
		active &&
		Number.isFinite(start) &&
		Number.isFinite(end) &&
		Number.isFinite(asOf) &&
		end > start
	const progress = timeline
		? Math.min(100, Math.max(0, ((asOf - start) / (end - start)) * 100))
		: 0
	return (
		<section
			className={`card current-card ${active ? (resting ? 'resting' : 'working') : 'empty'}`}
			aria-labelledby="current-heading"
		>
			<h2 id="current-heading" className="eyebrow">
				現在の作業
			</h2>
			{active ? (
				<>
					<div className="seat-row">
						<span className="state-badge">
							<span aria-hidden="true" className="state-dot" />
							{resting ? '休憩中' : '作業中'}
						</span>
						<span className="seat-location">
							{current.roomType === 'member' ? 'メンバールーム' : '通常ルーム'}{' '}
							· 席 <strong>{current.seatNumber ?? '—'}</strong>
						</span>
					</div>
					<p className="task-label">{resting ? 'いまの作業' : '作業内容'}</p>
					<p className="task-name">
						{current.workName?.trim() ? current.workName : '作業名は未設定'}
					</p>
					<div className="time-panel">
						<div className="time-row">
							<div>
								<p>{resting ? '休憩開始' : '作業開始'}</p>
								<strong>{time(current.stateStartedAt)}</strong>
							</div>
							<div>
								<p>{resting ? '休憩終了予定' : '終了予定'}</p>
								<strong>{time(current.expectedEndAt)}</strong>
							</div>
						</div>
						{timeline && (
							<>
								<div className="timeline" aria-hidden="true">
									<span className="timeline-track" />
									<span className="timeline-start" />
									<span className="timeline-end" />
									<span
										className="timeline-now"
										style={{ left: `${progress}%` }}
									/>
								</div>
								<p className="timeline-caption">
									{asOf > end
										? '終了予定を過ぎています'
										: `${time(value?.asOf ?? null)}時点`}
								</p>
							</>
						)}
					</div>
				</>
			) : (
				<>
					<p className="empty-title">
						{current.state === 'unregistered'
							? 'まだ作業記録がありません'
							: '現在は入室していません'}
					</p>
					<p className="muted">
						入室・休憩・退室は、YouTubeライブチャットで操作します。
						{current.state === 'unregistered'
							? '参加すると、ここに作業時間が記録されます。'
							: '入室すると、ここに現在の作業が表示されます。'}
					</p>
				</>
			)}
			{value && <StaleNote value={value} />}
			<div className="hero-links">
				<ExternalLink href={liveURL} primary={!active}>
					YouTubeライブを開く
				</ExternalLink>
				<ExternalLink href={guideURL}>使い方・コマンド</ExternalLink>
			</div>
		</section>
	)
}

function SummaryCard({ data, busy }: { data: ViewData | null; busy: boolean }) {
	const labels = { today: '今日', week: '今週', lifetime: '累計' }
	return (
		<section
			className="card summary-card"
			aria-labelledby="summary-heading"
			aria-busy={busy}
		>
			<h2 id="summary-heading">作業サマリー</h2>
			<dl>
				{(['today', 'week', 'lifetime'] as const).map((key) => {
					const value = data?.summary[key]
					const originalDate = value?.asOf
						? day.format(new Date(value.asOf))
						: null
					const currentDate = data
						? day.format(new Date(data.generatedAt))
						: null
					return (
						<div className={`metric metric-${key}`} key={key}>
							<dt>
								{labels[key]}
								{key === 'week' && <span>（月曜から）</span>}
								{key === 'today' &&
									originalDate &&
									originalDate !== currentDate && (
										<span>
											（{originalDate.slice(5).replace('-', '/')}の記録）
										</span>
									)}
							</dt>
							<dd>
								{busy && !data ? '—' : formatWorkSeconds(value?.data ?? null)}
							</dd>
							{value && <StaleNote value={value} />}
						</div>
					)
				})}
			</dl>
			<p className="jst-note">日本時間（JST）で集計 · 作業時間のみ</p>
		</section>
	)
}

function RecentCard({ data, busy }: { data: ViewData | null; busy: boolean }) {
	const [selectedDate, setSelectedDate] = useState<string | null>(null)
	const days = data?.recent ?? []
	const selected =
		days.find((item) => item.date === selectedDate) ?? days.at(-1)
	const max = Math.max(1, ...days.map((item) => item.data ?? 0))
	const today = data ? day.format(new Date(data.generatedAt)) : null
	return (
		<section
			className="card recent-card"
			aria-labelledby="recent-heading"
			aria-busy={busy}
		>
			<p className="eyebrow">直近7日</p>
			<h2 id="recent-heading">最近の積み重ね</h2>
			<p className="muted chart-label">作業時間</p>
			{days.length > 0 ? (
				<>
					<div className="bar-chart">
						{days.map((item) => (
							<button
								className={`day-button ${item.date === today ? 'is-today' : ''}`}
								key={item.date}
								type="button"
								aria-pressed={item.date === selected?.date}
								aria-label={`${item.date}: ${formatWorkSeconds(item.data)}`}
								onClick={() => setSelectedDate(item.date)}
								onFocus={() => setSelectedDate(item.date)}
							>
								<span className="bar-track" aria-hidden="true">
									<span
										className={`bar ${item.data === null ? 'unknown-bar' : ''}`}
										style={{
											height:
												item.data === null
													? '2px'
													: `${(item.data / max) * 100}%`,
										}}
									/>
								</span>
								<span className="day-label">
									{Number(item.date.slice(5, 7))}/{Number(item.date.slice(8))}
								</span>
								{item.date === today && (
									<span className="today-label">今日</span>
								)}
							</button>
						))}
					</div>
					<p className="selected-day" aria-live="polite">
						{selected?.date}: {formatWorkSeconds(selected?.data ?? null)}
					</p>
					{selected && <StaleNote value={selected} />}
				</>
			) : (
				<p role="status">
					{busy ? '読み込んでいます…' : '情報を取得できませんでした'}
				</p>
			)}
		</section>
	)
}

function AccountPanel({
	account,
	close,
	logout,
}: {
	account: Account | null
	close: () => void
	logout: () => void
}) {
	const dialog = useRef<HTMLDialogElement>(null)
	useEffect(() => {
		dialog.current?.showModal()
	}, [])
	return (
		<dialog
			className="account-dialog"
			ref={dialog}
			onClose={close}
			onKeyDown={(event) => {
				if (event.key !== 'Tab') return
				const targets = [
					...event.currentTarget.querySelectorAll<HTMLElement>(
						'button:not([disabled]), a[href], [tabindex="0"]',
					),
				].filter((item) => item.getClientRects().length > 0)
				const first = targets[0]
				const last = targets.at(-1)
				if (!first || !last) {
					event.preventDefault()
					return
				}
				if (event.shiftKey && document.activeElement === first) {
					event.preventDefault()
					last.focus()
				} else if (!event.shiftKey && document.activeElement === last) {
					event.preventDefault()
					first.focus()
				}
			}}
			aria-labelledby="account-heading"
		>
			<div className="dialog-heading">
				<h2 id="account-heading">アカウント</h2>
				<button
					className="icon-button"
					type="button"
					onClick={() => dialog.current?.close()}
					aria-label="アカウントを閉じる"
				>
					×
				</button>
			</div>
			<p className="account-name">
				{account?.displayName ?? 'チャンネル情報を取得できませんでした'}
			</p>
			{account?.handle && <p className="muted">{account.handle}</p>}
			<button
				className="button primary full-width"
				type="button"
				onClick={() => {
					dialog.current?.close()
					logout()
				}}
			>
				ログアウト
			</button>
			<a className="manage-link" href="/mypage/account">
				アカウント管理
			</a>
		</dialog>
	)
}

export function MyPageView({
	state,
	refresh,
	logout,
	cookieSettings,
}: {
	state: MemoryState
	refresh: () => void
	logout: () => void
	cookieSettings: () => void
}) {
	const [panelOpen, setPanelOpen] = useState(false)
	const avatar = useRef<HTMLButtonElement>(null)
	const privateVisible = state.phase === 'authenticated'
	const data = privateVisible ? state.data : null
	const account = data?.account.data ?? null
	const closePanel = () => {
		setPanelOpen(false)
		avatar.current?.focus()
	}
	useEffect(() => {
		if (state.phase !== 'authenticated') setPanelOpen(false)
	}, [state.phase])
	return (
		<div className="app-shell">
			<a className="skip-link" href="#main-content">
				本文へ移動
			</a>
			<header className="site-header">
				<div>
					<p className="eyebrow">オンライン作業部屋</p>
					<h1>マイページ</h1>
				</div>
				<div className="header-actions">
					{privateVisible && (
						<button
							className="refresh-button"
							type="button"
							onClick={refresh}
							disabled={state.busy}
							aria-label="最新の情報に更新"
						>
							{state.busy ? '更新中…' : '更新'}
						</button>
					)}
					<button
						type="button"
						ref={avatar}
						className="avatar-button"
						onClick={() => setPanelOpen(true)}
						disabled={!privateVisible}
						aria-label="アカウントを開く"
						aria-haspopup="dialog"
					>
						{account?.avatarUrl?.startsWith('https://') ? (
							<img
								src={account.avatarUrl}
								alt=""
								referrerPolicy="no-referrer"
							/>
						) : (
							<span aria-hidden="true">◯</span>
						)}
					</button>
				</div>
			</header>
			<main id="main-content" className="main-content" tabIndex={-1}>
				{privateVisible ? (
					<>
						<p className="greeting">
							おかえりなさい。今日も、少し進めましょう。
						</p>
						{state.error && (
							<div className="notice" role="status">
								<p>
									最新の情報に更新できませんでした。
									{state.receivedAt !== null &&
										`最後に取得できた ${dateTime.format(new Date(state.receivedAt))} の情報を表示しています。`}
								</p>
								<button
									className="button"
									type="button"
									onClick={refresh}
									disabled={state.busy}
								>
									再試行
								</button>
							</div>
						)}
						<div className="mypage-grid">
							<CurrentCard
								value={data?.current}
								busy={state.busy}
								retry={refresh}
							/>
							<SummaryCard data={data} busy={state.busy} />
							<RecentCard data={data} busy={state.busy} />
						</div>
					</>
				) : (
					<div className="card">
						<p role="status">
							{state.phase === 'bootstrapping'
								? 'ログイン状態を確認しています…'
								: state.phase === 'signing-out'
									? 'ログアウトしています…'
									: 'ログアウトしました。'}
						</p>
						{state.error === 'LOGOUT_FAILED' && (
							<>
								<p>
									ログアウトを完了できませんでした。もう一度お試しください。
								</p>
								<button className="button" type="button" onClick={logout}>
									ログアウトを再試行
								</button>
							</>
						)}
					</div>
				)}
			</main>
			<footer className="site-footer">
				<ExternalLink href={guideURL}>使い方</ExternalLink>
				<a href="/privacy">プライバシー</a>
				<a href="/terms">利用規約</a>
				<button type="button" onClick={cookieSettings}>
					Cookie設定
				</button>
				<a href="/contact">お問い合わせ</a>
			</footer>
			{panelOpen && privateVisible && (
				<AccountPanel account={account} close={closePanel} logout={logout} />
			)}
		</div>
	)
}
