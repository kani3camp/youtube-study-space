import { type ReactNode, useEffect, useRef, useState } from 'react'
import { Icon, SiteHeader } from '../../components/site-header'
import { formatWorkSeconds } from '../../lib/format'
import type { Account, Current, ReasonCode } from './contract'
import type { MemoryState, Retained, ViewData } from './memory'

export const guideURL =
	'https://kani3camp.github.io/youtube-study-space/docs/essential'

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
const weekday = new Intl.DateTimeFormat('ja-JP', {
	timeZone: 'Asia/Tokyo',
	weekday: 'short',
})
function weekdayLabel(date: string) {
	return weekday.format(new Date(`${date}T00:00:00+09:00`))
}
function dateLabel(date: string, today?: string | null) {
	return `${Number(date.slice(5, 7))}月${Number(date.slice(8))}日（${weekdayLabel(date)}${date === today ? '・今日' : ''}）`
}
function shortDate(date: string) {
	return `${Number(date.slice(5, 7))}/${Number(date.slice(8))}`
}

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

export function AccountAvatar({ url }: { url: string | null }) {
	const [imageFailed, setImageFailed] = useState(false)
	useEffect(() => setImageFailed(!url), [url])
	if (url?.startsWith('https://') && !imageFailed)
		return (
			<img
				src={url}
				alt=""
				referrerPolicy="no-referrer"
				onError={() => setImageFailed(true)}
			/>
		)
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="1.6"
			aria-hidden="true"
		>
			<circle cx="12" cy="8" r="3.5" />
			<path d="M5 21v-2a7 7 0 0 1 14 0v2" />
		</svg>
	)
}

function DurationValue({ seconds }: { seconds: number | null }) {
	const text = formatWorkSeconds(seconds)
	const parts = /^(\d+)時間 (\d+)分$/.exec(text)
	if (!parts) return <span className="duration-unavailable">{text}</span>
	return (
		<span className="duration-value">
			<span className="sr-only">{text}</span>
			<span className="duration-part" aria-hidden="true">
				<span className="duration-number">{parts[1]}</span>
				<span className="duration-unit">時間</span>
			</span>
			<span className="duration-part" aria-hidden="true">
				<span className="duration-number">{parts[2]}</span>
				<span className="duration-unit">分</span>
			</span>
		</span>
	)
}

function Skeleton({ area }: { area: 'current' | 'summary' | 'recent' }) {
	return (
		<div className={`skeleton skeleton-${area}`} aria-hidden="true">
			<span />
			<span />
			<span />
		</div>
	)
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

function StaleNote({
	value,
	sharedAsOf,
}: {
	value: Retained<unknown>
	sharedAsOf?: string | null
}) {
	if (!value.stale) return null
	if (value.reason === 'METADATA_TOO_OLD')
		return (
			<p className="stale-note" role="status">
				チャンネル情報の有効期限が切れました。再取得まで表示できません。
			</p>
		)
	if (value.data === null) {
		const explanations: Partial<Record<ReasonCode, string>> = {
			HISTORY_LIMIT_EXCEEDED: '記録が多いため集計できません。',
			DATA_INCONSISTENT: '記録の整合性を確認できません。',
		}
		const explanation = value.reason ? explanations[value.reason] : undefined
		if (explanation) return <p className="stale-note">{explanation}</p>
	}
	return (
		<p className="stale-note">
			{value.data === null
				? '情報を取得できませんでした'
				: '更新できませんでした'}
			{value.data !== null && value.asOf && value.asOf !== sharedAsOf && (
				<> · {timestamp(value.asOf)}時点</>
			)}
		</p>
	)
}

function CurrentCard({
	value,
	busy,
	retry,
	liveURL,
}: {
	value?: Retained<Current>
	busy: boolean
	retry: () => void
	liveURL: string | null
}) {
	const current = value?.data
	if (!current)
		return (
			<section
				className="card current-card"
				aria-labelledby="current-heading"
				aria-busy={busy}
			>
				<h2 id="current-heading">
					{busy ? '現在の作業' : '情報を取得できませんでした'}
				</h2>
				{busy ? (
					<>
						<p className="sr-only" role="status">
							読み込んでいます…
						</p>
						<Skeleton area="current" />
					</>
				) : (
					<>
						<p className="muted" role="alert">
							記録が消えたわけではありません。少し待ってからお試しください。
						</p>
						<button className="button primary" type="button" onClick={retry}>
							再読み込み
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
	const timelinePosition = `calc(6px + (100% - 12px) * ${progress / 100})`
	const currentAsOf = value?.asOf && (
		<p className="jst-note">入室状況 · {timestamp(value.asOf)}時点</p>
	)
	return (
		<section
			className={`card current-card ${active ? (resting ? 'resting' : 'working') : 'empty'}`}
			aria-labelledby="current-heading"
		>
			<h2 id="current-heading" className="sr-only">
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
							<span>
								{current.roomType === 'member'
									? 'メンバールーム'
									: '通常ルーム'}
							</span>
							<span className="seat-divider" aria-hidden="true">
								{' '}
								·{' '}
							</span>
							<span>
								席 <strong>{current.seatNumber ?? '—'}</strong>
							</span>
						</span>
					</div>
					<p className="task-label">いまの作業</p>
					<p className="task-name">
						{current.workName?.trim() ? current.workName : '作業名は未設定'}
					</p>
					<div className="time-panel">
						<div className="time-row">
							<div>
								<p>{resting ? '休憩開始' : '開始時刻'}</p>
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
										style={{ left: timelinePosition }}
									/>
								</div>
								{asOf > end && (
									<p className="overdue-note">終了予定を過ぎています</p>
								)}
							</>
						)}
						{currentAsOf}
					</div>
				</>
			) : (
				<>
					<p className="state-badge empty-badge">
						<span className="state-dot" aria-hidden="true" />
						{current.state === 'unregistered' ? '未登録' : '未入室'}
					</p>
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
					{currentAsOf}
				</>
			)}
			{value && <StaleNote value={value} sharedAsOf={value.asOf} />}
			<div className="hero-links current-links">
				{active && (
					<ExternalLink href={guideURL}>使い方・コマンド</ExternalLink>
				)}
				{liveURL && (
					<ExternalLink href={liveURL} primary={!active}>
						YouTubeライブを開く
					</ExternalLink>
				)}
				{!active && (
					<ExternalLink href={guideURL}>使い方・コマンド</ExternalLink>
				)}
			</div>
		</section>
	)
}

function workStatsAsOf(data: ViewData | null) {
	let newest: string | null = null
	for (const value of [
		...Object.values(data?.summary ?? {}),
		...(data?.recent ?? []),
	]) {
		const { asOf } = value
		if (asOf && (!newest || Date.parse(asOf) > Date.parse(newest)))
			newest = asOf
	}
	return newest
}

function SummaryCard({
	data,
	busy,
	asOf,
}: {
	data: ViewData | null
	busy: boolean
	asOf: string | null
}) {
	const labels = { today: '今日', week: '今週', lifetime: '累計' }
	return (
		<section
			className="card summary-card"
			aria-labelledby="summary-heading"
			aria-busy={busy}
		>
			<h2 id="summary-heading">作業サマリー</h2>
			{busy && !data ? (
				<Skeleton area="summary" />
			) : (
				<dl>
					{(['today', 'week', 'lifetime'] as const).map((key) => {
						const value = data?.summary[key]
						const originalDate = value?.asOf
							? day.format(new Date(value.asOf))
							: null
						const currentDate = data?.current.asOf
							? day.format(new Date(data.current.asOf))
							: null
						return (
							<div className={`metric metric-${key}`} key={key}>
								<dt>
									<Icon name={key === 'lifetime' ? 'history' : 'calendar'} />
									{key === 'today' &&
									originalDate &&
									originalDate !== currentDate
										? `${originalDate.slice(5).replace('-', '/')}の作業時間`
										: key === 'week' &&
												originalDate &&
												originalDate !== currentDate
											? '週の作業時間'
											: labels[key]}
									{key === 'week' && <span>（月曜から）</span>}
								</dt>
								<dd>
									<DurationValue seconds={value?.data ?? null} />
								</dd>
								{value && <StaleNote value={value} sharedAsOf={asOf} />}
							</div>
						)
					})}
				</dl>
			)}
			<p className="jst-note summary-timezone">
				<Icon name="clock" />
				日本時間（JST） · 休憩を除く
			</p>
			{asOf && (
				<p className="jst-note stats-as-of">
					作業集計（直近7日を含む） · {timestamp(asOf)}時点
				</p>
			)}
		</section>
	)
}

function RecentCard({
	data,
	busy,
	asOf,
}: {
	data: ViewData | null
	busy: boolean
	asOf: string | null
}) {
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
			<div className="chart-heading">
				<h2 id="recent-heading" aria-label="直近7日の作業時間">
					直近7日
				</h2>
				{days.length > 0 && (
					<p className="chart-range">
						{shortDate(days[0].date)}〜{shortDate(days[days.length - 1].date)}
					</p>
				)}
			</div>
			{days.length > 0 ? (
				<>
					<p className="selected-day" aria-live="polite">
						{selected && (
							<>
								<span className="selected-date">
									{dateLabel(selected.date, today)}
								</span>
								<strong>{formatWorkSeconds(selected.data)}</strong>
							</>
						)}
					</p>
					<fieldset className="bar-chart" aria-label="日別の作業時間">
						{days.map((item) => (
							<button
								className={`day-button ${item.date === today ? 'is-today' : ''}`}
								key={item.date}
								type="button"
								aria-pressed={item.date === selected?.date}
								aria-label={`${dateLabel(item.date, today)}: ${formatWorkSeconds(item.data)}`}
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
													: item.data === 0
														? '4px'
														: `${(item.data / max) * 100}%`,
										}}
									/>
								</span>
								<span className="day-label">
									<span
										className={item.date === today ? 'today-label' : undefined}
									>
										{item.date === today ? '今日' : weekdayLabel(item.date)}
									</span>
									<span>{Number(item.date.slice(8))}日</span>
								</span>
							</button>
						))}
					</fieldset>
					{selected && <StaleNote value={selected} sharedAsOf={asOf} />}
				</>
			) : busy ? (
				<>
					<p className="sr-only" role="status">
						読み込んでいます…
					</p>
					<Skeleton area="recent" />
				</>
			) : (
				<p role="status">情報を取得できませんでした</p>
			)}
		</section>
	)
}

function AccountPanel({
	value,
	close,
	logout,
}: {
	value: Retained<Account> | undefined
	close: () => void
	logout: () => void
}) {
	const account = value?.data ?? null
	const dialog = useRef<HTMLDialogElement>(null)
	useEffect(() => {
		dialog.current?.showModal()
	}, [])
	return (
		<dialog
			className="account-dialog account-panel"
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
			<h2 id="account-heading" className="sr-only">
				アカウント
			</h2>
			<div className="account-identity">
				<div className="account-avatar">
					<AccountAvatar url={account?.avatarUrl ?? null} />
				</div>
				<div className="account-identity-text">
					<p className="account-name">
						{account?.displayName ?? 'チャンネル情報を取得できませんでした'}
					</p>
					{account?.handle && <p className="muted">{account.handle}</p>}
				</div>
				<button
					className="icon-button"
					type="button"
					onClick={() => dialog.current?.close()}
					aria-label="アカウントを閉じる"
				>
					×
				</button>
			</div>
			{value && <StaleNote value={value} />}
			<button
				className="button full-width account-logout"
				type="button"
				onClick={() => {
					dialog.current?.close()
					logout()
				}}
			>
				<Icon name="logout" />
				ログアウト
			</button>
			<a className="manage-link account-manage-link" href="/mypage/account">
				<Icon name="settings" />
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
	liveURL,
}: {
	state: MemoryState
	refresh: () => void
	logout: () => void
	cookieSettings: () => void
	liveURL: string | null
}) {
	const [panelOpen, setPanelOpen] = useState(false)
	const avatar = useRef<HTMLButtonElement>(null)
	const privateVisible = state.phase === 'authenticated'
	const data = privateVisible ? state.data : null
	const statsAsOf = workStatsAsOf(data)
	const account = data?.account.data ?? null
	const initialFailure = !data && !state.busy && !!state.error
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
			<SiteHeader main>
				{privateVisible && (
					<button
						className="refresh-button"
						type="button"
						onClick={refresh}
						disabled={state.busy}
						aria-label="最新の情報に更新"
					>
						<Icon name="refresh" />
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
					<AccountAvatar url={account?.avatarUrl ?? null} />
				</button>
			</SiteHeader>
			<main id="main-content" className="main-content" tabIndex={-1}>
				{privateVisible ? (
					<>
						<p className="greeting">おかえりなさい。</p>
						{state.error && data && (
							<div className="notice" role="status">
								<p>最新の情報に更新できませんでした。</p>
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
						<div
							className={`mypage-grid${initialFailure ? ' initial-failure' : ''}`}
						>
							<CurrentCard
								value={data?.current}
								busy={state.busy}
								retry={refresh}
								liveURL={liveURL}
							/>
							{!initialFailure && (
								<>
									<SummaryCard data={data} busy={state.busy} asOf={statsAsOf} />
									<RecentCard data={data} busy={state.busy} asOf={statsAsOf} />
								</>
							)}
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
				<AccountPanel
					value={data?.account}
					close={closePanel}
					logout={logout}
				/>
			)}
		</div>
	)
}
