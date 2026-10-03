import { useState } from 'react'

import { formatDuration } from '../../../lib/format'

type MockViewState =
	| 'work'
	| 'break'
	| 'idle'
	| 'not_registered'
	| 'link_required'
	| 'error'
	| 'loading'

type MockDialog =
	| 'live'
	| 'relink'
	| 'logout'
	| 'cookie'
	| 'privacy'
	| 'terms'
	| 'contact'
	| null

type WorkState = 'work' | 'break' | 'idle'

const mockStateOptions: Array<{ value: MockViewState; label: string }> = [
	{ value: 'work', label: '作業中' },
	{ value: 'break', label: '休憩中' },
	{ value: 'idle', label: '未入室' },
	{ value: 'not_registered', label: '未登録' },
	{ value: 'link_required', label: '連携切れ' },
	{ value: 'error', label: 'エラー' },
	{ value: 'loading', label: 'Loading' },
]

const chartDays = [
	{ label: '9/27', minutes: 45 },
	{ label: '9/28', minutes: 120 },
	{ label: '9/29', minutes: 0 },
	{ label: '9/30', minutes: 95 },
	{ label: '10/1', minutes: 60 },
	{ label: '10/2', minutes: 150 },
	{ label: '今日', dateLabel: '10/3', minutes: 115, isToday: true },
]

export function MyPageInteractiveMock() {
	const [viewState, setViewState] = useState<MockViewState>('work')
	const [selectedDayIndex, setSelectedDayIndex] = useState(chartDays.length - 1)
	const [dialog, setDialog] = useState<MockDialog>(null)
	const [analyticsAllowed, setAnalyticsAllowed] = useState(true)
	const selectedDay = chartDays[selectedDayIndex]

	return (
		<div className="mypageMockRoot">
			<MockReviewControls state={viewState} onChange={setViewState} />

			{viewState === 'loading' ? (
				<MockLoading />
			) : viewState === 'error' ? (
				<MockError onRetry={() => setViewState('work')} />
			) : viewState === 'link_required' ? (
				<MockLinkRequired onRelink={() => setDialog('relink')} />
			) : viewState === 'not_registered' ? (
				<div className="publicStandardMock">
					<div className="mockSingleColumn">
						<MockNotRegistered onLive={() => setDialog('live')} />
						<AccountPanel
							onRelink={() => setDialog('relink')}
							onLogout={() => setDialog('logout')}
						/>
					</div>
					<MockFooter onOpen={setDialog} />
				</div>
			) : (
				<div className="publicStandardMock">
					<div className="mockContentGrid">
						<CurrentWorkPanel
							state={viewState}
							onLive={() => setDialog('live')}
						/>
						<SummaryPanel />
						<SevenDayPanel
							selectedDayIndex={selectedDayIndex}
							onSelectDay={setSelectedDayIndex}
						/>
						<AccountPanel
							onRelink={() => setDialog('relink')}
							onLogout={() => setDialog('logout')}
						/>
					</div>
					<p className="mockChartSelection" aria-live="polite">
						{selectedDay.dateLabel ?? selectedDay.label} の作業時間:{' '}
						<strong>{formatDuration(selectedDay.minutes * 60)}</strong>
					</p>
					<MockFooter onOpen={setDialog} />
				</div>
			)}

			{dialog ? (
				<MockDialogOverlay
					type={dialog}
					analyticsAllowed={analyticsAllowed}
					onAnalyticsChange={setAnalyticsAllowed}
					onClose={() => setDialog(null)}
				/>
			) : null}
		</div>
	)
}

type MockReviewControlsProps = {
	state: MockViewState
	onChange: (state: MockViewState) => void
}

function MockReviewControls({ state, onChange }: MockReviewControlsProps) {
	return (
		<section className="mockReviewControls" aria-label="モック表示状態">
			<div>
				<p className="mockReviewLabel">レビュー操作</p>
				<p className="mockReviewHint">
					画面状態を切り替えて、レスポンシブ表示と操作感を確認できます。
				</p>
			</div>
			<div className="mockStateButtons">
				{mockStateOptions.map((option) => (
					<button
						className="mockStateButton"
						type="button"
						key={option.value}
						aria-pressed={state === option.value}
						onClick={() => onChange(option.value)}
					>
						{option.label}
					</button>
				))}
			</div>
		</section>
	)
}

type CurrentWorkPanelProps = {
	state: WorkState
	onLive: () => void
}

function CurrentWorkPanel({ state, onLive }: CurrentWorkPanelProps) {
	if (state === 'idle') {
		return (
			<section className="mockPanel mockCurrentWork mockCurrentWork--idle">
				<p className="mockSectionLabel">現在の作業</p>
				<div className="mockStatusLine">
					<span className="mockStatusDot mockStatusDot--idle" aria-hidden="true" />
					<span>未入室</span>
				</div>
				<h2 className="mockCurrentTitle">現在は入室していません</h2>
				<p className="mockCurrentDescription">
					作業を始めると、ここに現在の席と作業内容が表示されます。
				</p>
				<button className="mockButton mockButton--primary" type="button" onClick={onLive}>
					YouTubeライブを開く
				</button>
			</section>
		)
	}

	const isBreak = state === 'break'

	return (
		<section
			className={`mockPanel mockCurrentWork ${
				isBreak ? 'mockCurrentWork--break' : 'mockCurrentWork--work'
			}`}
		>
			<div className="mockCurrentTopline">
				<p className="mockSectionLabel">現在の作業</p>
				<span className="mockSeatBadge">席 12</span>
			</div>
			<div className="mockStatusLine">
				<span
					className={`mockStatusDot ${
						isBreak ? 'mockStatusDot--break' : 'mockStatusDot--work'
					}`}
					aria-hidden="true"
				/>
				<span>{isBreak ? '休憩中' : '作業中'}</span>
			</div>
			<h2 className="mockCurrentTitle">
				{isBreak ? 'コーヒーブレイク' : '設計レビューとAPI実装'}
			</h2>
			<dl className="mockCurrentMeta">
				<div>
					<dt>{isBreak ? '休憩開始' : '開始'}</dt>
					<dd>{isBreak ? '10:05' : '09:15'}</dd>
				</div>
				<div>
					<dt>{isBreak ? '休憩終了予定' : '終了予定'}</dt>
					<dd>{isBreak ? '10:20' : '11:00'}</dd>
				</div>
			</dl>
			<div className="mockCurrentFooter">
				<span>時刻はJST</span>
				<button
					className="mockButton mockButton--secondary"
					type="button"
					onClick={onLive}
				>
					YouTubeライブを開く
				</button>
			</div>
		</section>
	)
}

function SummaryPanel() {
	const summary = [
		{ label: '今日', value: 2 * 60 * 60 + 35 * 60 },
		{ label: '今週', value: 8 * 60 * 60 + 50 * 60 },
		{ label: '累計', value: 1234 * 60 * 60 + 20 * 60 },
	]

	return (
		<section className="mockPanel mockSummary">
			<p className="mockSectionLabel">作業サマリー</p>
			<div className="mockSummaryGrid">
				{summary.map((item) => (
					<div className="mockSummaryMetric" key={item.label}>
						<span>{item.label}</span>
						<strong>{formatDuration(item.value)}</strong>
					</div>
				))}
			</div>
			<p className="mockTimezoneText">集計は日本時間（JST）</p>
		</section>
	)
}

type SevenDayPanelProps = {
	selectedDayIndex: number
	onSelectDay: (index: number) => void
}

function SevenDayPanel({
	selectedDayIndex,
	onSelectDay,
}: SevenDayPanelProps) {
	const maxMinutes = Math.max(...chartDays.map((day) => day.minutes))

	return (
		<section className="mockPanel mockSevenDay">
			<div className="mockSectionHeading">
				<div>
					<p className="mockSectionLabel">直近7日</p>
					<h2>最近の積み重ね</h2>
				</div>
				<span className="mockChartUnit">作業時間</span>
			</div>
			<div className="mockBarChart" aria-label="直近7日間の日別作業時間">
				{chartDays.map((day, index) => {
					const height =
						day.minutes === 0
							? '2px'
							: `${Math.max(8, (day.minutes / maxMinutes) * 100)}%`

					return (
						<button
							className={`mockBarItem ${
								day.isToday ? 'mockBarItem--today' : ''
							}`}
							type="button"
							key={day.dateLabel ?? day.label}
							aria-pressed={selectedDayIndex === index}
							aria-label={`${day.dateLabel ?? day.label} ${
								day.minutes
							}分`}
							onClick={() => onSelectDay(index)}
							onFocus={() => onSelectDay(index)}
						>
							<span className="mockBarTrack" aria-hidden="true">
								<span
									className={`mockBar ${
										day.minutes === 0 ? 'mockBar--zero' : ''
									}`}
									style={{ height }}
								/>
							</span>
							<span className="mockBarLabel">{day.label}</span>
						</button>
					)
				})}
			</div>
		</section>
	)
}

type AccountPanelProps = {
	onRelink: () => void
	onLogout: () => void
}

function AccountPanel({ onRelink, onLogout }: AccountPanelProps) {
	return (
		<section className="mockPanel mockAccount">
			<p className="mockSectionLabel">YouTubeアカウント</p>
			<div className="mockAccountProfile">
				<div className="mockAvatar" aria-hidden="true">
					J
				</div>
				<div className="mockAccountIdentity">
					<h2>Jun Study</h2>
					<span className="mockLinkedBadge">連携済み</span>
				</div>
			</div>
			<p className="mockAccountDescription">
				現在このYouTubeチャンネルと作業記録を紐付けています。
			</p>
			<div className="mockAccountActions">
				<button className="mockTextButton" type="button" onClick={onRelink}>
					再連携
				</button>
				<button className="mockTextButton" type="button" onClick={onLogout}>
					ログアウト
				</button>
			</div>
		</section>
	)
}

type MockFooterProps = {
	onOpen: (dialog: Exclude<MockDialog, null>) => void
}

function MockFooter({ onOpen }: MockFooterProps) {
	return (
		<footer className="mockFooter">
			<button type="button" onClick={() => onOpen('privacy')}>
				Privacy Policy
			</button>
			<button type="button" onClick={() => onOpen('terms')}>
				利用規約
			</button>
			<button type="button" onClick={() => onOpen('cookie')}>
				Cookie設定
			</button>
			<button type="button" onClick={() => onOpen('contact')}>
				問い合わせ
			</button>
		</footer>
	)
}

type MockDialogOverlayProps = {
	type: Exclude<MockDialog, null>
	analyticsAllowed: boolean
	onAnalyticsChange: (allowed: boolean) => void
	onClose: () => void
}

function MockDialogOverlay({
	type,
	analyticsAllowed,
	onAnalyticsChange,
	onClose,
}: MockDialogOverlayProps) {
	const content = getDialogContent(type)

	return (
		<div className="mockDialogBackdrop" role="presentation" onMouseDown={onClose}>
			<section
				className="mockDialog"
				role="dialog"
				aria-modal="true"
				aria-labelledby="mock-dialog-title"
				onMouseDown={(event) => event.stopPropagation()}
			>
				<p className="mockSectionLabel">操作モック</p>
				<h2 id="mock-dialog-title">{content.title}</h2>
				<p>{content.description}</p>

				{type === 'cookie' ? (
					<label className="mockCookieToggle">
						<input
							type="checkbox"
							checked={analyticsAllowed}
							onChange={(event) => onAnalyticsChange(event.target.checked)}
						/>
						<span>
							<strong>Analytics</strong>
							<small>
								GA4による利用状況の計測。拒否しても主要機能は利用できます。
							</small>
						</span>
					</label>
				) : null}

				{type === 'relink' ? (
					<div className="mockRelinkPreview">
						<div className="mockAvatar" aria-hidden="true">
							J
						</div>
						<div>
							<strong>Jun Study</strong>
							<span>このチャンネルを連携します</span>
						</div>
					</div>
				) : null}

				<div className="mockDialogActions">
					{type === 'relink' ? (
						<button
							className="mockButton mockButton--primary"
							type="button"
							onClick={onClose}
						>
							このチャンネルを連携
						</button>
					) : null}
					<button
						className="mockButton mockButton--secondary"
						type="button"
						onClick={onClose}
					>
						閉じる
					</button>
				</div>
			</section>
		</div>
	)
}

function getDialogContent(type: Exclude<MockDialog, null>) {
	switch (type) {
		case 'live':
			return {
				title: 'YouTubeライブを開く',
				description:
					'本番ではオンライン作業部屋のYouTubeライブを別タブで開く導線です。このモックでは遷移しません。',
			}
		case 'relink':
			return {
				title: 'YouTubeチャンネルを再連携',
				description:
					'取得したチャンネルを無言で確定せず、アバターと表示名を確認してから連携する流れを想定しています。',
			}
		case 'logout':
			return {
				title: 'ログアウト',
				description:
					'本番ではFirebase Authからサインアウトし、ログイン画面へ戻ります。',
			}
		case 'cookie':
			return {
				title: 'Cookie設定',
				description:
					'必須Cookieは常時利用し、Analyticsだけを後から変更できる想定です。',
			}
		case 'privacy':
			return {
				title: 'Privacy Policy',
				description:
					'本番では未ログインでも閲覧できるPrivacy Policyページへ遷移します。',
			}
		case 'terms':
			return {
				title: '利用規約',
				description:
					'本番では未ログインでも閲覧できる利用規約ページへ遷移します。',
			}
		case 'contact':
			return {
				title: '問い合わせ',
				description:
					'保存データ削除など、ログアウトや再連携とは意味が異なる手続きもここから案内する想定です。',
			}
	}
}

type MockNotRegisteredProps = {
	onLive: () => void
}

function MockNotRegistered({ onLive }: MockNotRegisteredProps) {
	return (
		<section className="mockPanel mockEmptyState">
			<p className="mockSectionLabel">作業記録</p>
			<h2>まだ作業記録がありません</h2>
			<p>
				オンライン作業部屋のライブチャットから入室すると、ここに作業時間が記録されます。
			</p>
			<button className="mockButton mockButton--primary" type="button" onClick={onLive}>
				YouTubeライブを開く
			</button>
		</section>
	)
}

type MockLinkRequiredProps = {
	onRelink: () => void
}

function MockLinkRequired({ onRelink }: MockLinkRequiredProps) {
	return (
		<section className="mockPanel mockStandalonePanel">
			<p className="mockSectionLabel">YouTube連携</p>
			<h2>YouTubeとの連携が必要です</h2>
			<p>
				本人のYouTubeチャンネルを確認するため、読み取り権限を使って再連携してください。
			</p>
			<button
				className="mockButton mockButton--primary"
				type="button"
				onClick={onRelink}
			>
				再連携する
			</button>
		</section>
	)
}

type MockErrorProps = {
	onRetry: () => void
}

function MockError({ onRetry }: MockErrorProps) {
	return (
		<section className="mockPanel mockStandalonePanel mockErrorPanel">
			<p className="mockSectionLabel">読み込みエラー</p>
			<h2>情報を取得できませんでした</h2>
			<p>
				0時間として代替表示せず、前回の成功データがない場合は明示的にエラーを表示します。
			</p>
			<button
				className="mockButton mockButton--primary"
				type="button"
				onClick={onRetry}
			>
				再読み込み
			</button>
		</section>
	)
}

function MockLoading() {
	return (
		<div className="publicStandardMock" aria-label="読み込み中">
			<div className="mockContentGrid mockSkeletonGrid">
				<section className="mockPanel mockSkeletonCurrent">
					<div className="mockSkeletonLine mockSkeletonLine--short" />
					<div className="mockSkeletonLine mockSkeletonLine--medium" />
					<div className="mockSkeletonLine mockSkeletonLine--title" />
					<div className="mockSkeletonMeta">
						<div />
						<div />
					</div>
				</section>
				<section className="mockPanel mockSkeletonSummary">
					<div className="mockSkeletonLine mockSkeletonLine--short" />
					<div className="mockSkeletonSummaryMetrics">
						<div />
						<div />
						<div />
					</div>
				</section>
				<section className="mockPanel mockSkeletonChart">
					<div className="mockSkeletonLine mockSkeletonLine--medium" />
					<div className="mockSkeletonBars">
						{chartDays.map((day) => (
							<span key={day.dateLabel ?? day.label} />
						))}
					</div>
				</section>
				<section className="mockPanel mockSkeletonAccount">
					<div className="mockSkeletonLine mockSkeletonLine--short" />
					<div className="mockSkeletonAccountRow">
						<span />
						<div>
							<i />
							<i />
						</div>
					</div>
				</section>
			</div>
		</div>
	)
}
