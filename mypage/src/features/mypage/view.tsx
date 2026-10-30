import { type ReactNode, useEffect, useRef, useState } from 'react'
import { Icon, SiteHeader } from '../../components/site-header'
import { formatWorkSeconds } from '../../lib/format'
import type { Account, Current, ReasonCode } from './contract'
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

function historySeconds(value?: Retained<number>) {
	return value?.data ?? value?.recorded?.data ?? null
}

function historyText(value?: Retained<number>, tentative = true) {
	if (value?.data === null && value.recorded) {
		const seconds = value.recorded.data
		const text = seconds < 60 ? '1分未満' : formatWorkSeconds(seconds)
		return `記録済み ${text}${tentative ? '（集計未確定）' : ''}`
	}
	return formatWorkSeconds(value?.data ?? null)
}

function MetricValue({ value }: { value?: Retained<number> }) {
	if (value?.data === null && value.recorded)
		return (
			<span className="recorded-value">
				<span className="recorded-caption">記録済み</span>
				{value.recorded.data < 60 ? (
					'1分未満'
				) : (
					<DurationValue seconds={value.recorded.data} />
				)}
			</span>
		)
	return <DurationValue seconds={value?.data ?? null} />
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

function StaleNote({ value }: { value: Retained<unknown> }) {
	if (!value.stale) return null
	if (value.reason === 'METADATA_TOO_OLD')
		return (
			<p className="stale-note" role="status">
				チャンネル情報の有効期限が切れました。再取得まで表示できません。
			</p>
		)
	if (value.data === null) {
		if (value.recorded)
			return (
				<p className="stale-note history-note">
					<span className="history-status">
						<Icon name="info" />
						<span>集計未確定</span>
						<span className="note-time">
							{timestamp(value.recorded.asOf)}時点
						</span>
					</span>
					<span>記録がそろっているか未確認です。</span>
				</p>
			)
		const explanations: Partial<Record<ReasonCode, string>> = {
			HISTORY_INCOMPLETE: 'この期間の記録がそろっていません。',
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
									<span
										className="timeline-caption"
										style={{
											left: timelinePosition,
											transform:
												progress < 14
													? 'translateX(-6px)'
													: progress > 86
														? 'translateX(calc(-100% + 6px))'
														: 'translateX(-50%)',
										}}
									>
										{time(value?.asOf ?? null)}時点
									</span>
								</div>
								{asOf > end && (
									<p className="overdue-note">終了予定を過ぎています</p>
								)}
							</>
						)}
					</div>
					<p className="jst-note">
						日本時間（JST） · {time(value?.asOf ?? null)}時点
					</p>
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
				</>
			)}
			{value && <StaleNote value={value} />}
			<div className="hero-links current-links">
				{active && (
					<ExternalLink href={guideURL}>使い方・コマンド</ExternalLink>
				)}
				<ExternalLink href={liveURL} primary={!active}>
					YouTubeライブを開く
				</ExternalLink>
				{!active && (
					<ExternalLink href={guideURL}>使い方・コマンド</ExternalLink>
				)}
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
			{busy && !data ? (
				<Skeleton area="summary" />
			) : (
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
									<Icon name={key === 'lifetime' ? 'history' : 'calendar'} />
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
									<MetricValue value={value} />
								</dd>
								{value && <StaleNote value={value} />}
							</div>
						)
					})}
				</dl>
			)}
			<p className="jst-note summary-timezone">
				<Icon name="clock" />
				日本時間（JST） · 休憩を除く
			</p>
		</section>
	)
}

function RecentCard({ data, busy }: { data: ViewData | null; busy: boolean }) {
	const [selectedDate, setSelectedDate] = useState<string | null>(null)
	const days = data?.recent ?? []
	const selected =
		days.find((item) => item.date === selectedDate) ?? days.at(-1)
	const max = Math.max(1, ...days.map((item) => historySeconds(item) ?? 0))
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
								<strong>{historyText(selected, false)}</strong>
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
								aria-label={`${dateLabel(item.date, today)}: ${historyText(item)}`}
								onClick={() => setSelectedDate(item.date)}
								onFocus={() => setSelectedDate(item.date)}
							>
								<span className="bar-track" aria-hidden="true">
									<span
										className={`bar ${historySeconds(item) === null ? 'unknown-bar' : item.data === null ? 'recorded-bar' : ''}`}
										style={{
											height:
												historySeconds(item) === null
													? '2px'
													: historySeconds(item) === 0
														? '4px'
														: `${((historySeconds(item) ?? 0) / max) * 100}%`,
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
					{days.some((item) => item.data === null && item.recorded) && (
						<p className="muted recorded-legend">
							斜線は記録済み分（集計未確定）
						</p>
					)}
					{selected && <StaleNote value={selected} />}
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
						<div
							className={`mypage-grid${initialFailure ? ' initial-failure' : ''}`}
						>
							<CurrentCard
								value={data?.current}
								busy={state.busy}
								retry={refresh}
							/>
							{!initialFailure && (
								<>
									<SummaryCard data={data} busy={state.busy} />
									<RecentCard data={data} busy={state.busy} />
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
