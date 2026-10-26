import {
	createRootRoute,
	createRoute,
	createRouter,
	Link,
	Navigate,
	Outlet,
	type RouterHistory,
	useRouterState,
} from '@tanstack/react-router'
import {
	createContext,
	useContext,
	useEffect,
	useLayoutEffect,
	useRef,
	useState,
	useSyncExternalStore,
} from 'react'
import { SiteHeader } from './components/site-header'
import { HistoryPanel } from './features/mypage/history-view'
import {
	RequestError,
	type RestrictionCode,
	restrictionFrom,
} from './features/mypage/memory'
import {
	AccountAvatar,
	ExternalLink,
	guideURL,
	liveURL,
	MyPageView,
} from './features/mypage/view'
import { maintenanceDestination } from './maintenance'
import { PrivacyPolicy, PublicPolicyContact, TermsOfUse } from './policies'
import { consentKey, createPrivacy } from './privacy'
import { readPublicPolicyConfig } from './public-policy-config'
import type {
	BrowserRuntime,
	ChannelConfirmation,
	PrivacyIntakeReceipt,
	PrivacyRequestStatus,
	SupportPurpose,
} from './runtime'

type LoginSearch = {
	error?: string
	logout?: boolean
	supportChallenge?: string
	supportInvalid?: boolean
}
const SupportReceipt = createContext<{
	proofRef: string | null
	supportRequestRef: string | null
	set: (proofRef: string | null, supportRequestRef?: string | null) => void
}>({ proofRef: null, supportRequestRef: null, set: () => {} })
type IntakeDraft = {
	purpose: SupportPurpose
	body: string
	submissionKey: string | null
	created: PrivacyIntakeReceipt | null
}
const emptyIntakeDraft: IntakeDraft = {
	purpose: 'delete',
	body: '',
	submissionKey: null,
	created: null,
}
const IntakeMemory = createContext<{
	value: IntakeDraft
	set: (next: IntakeDraft) => void
}>({ value: emptyIntakeDraft, set: () => {} })
const supportPurposeLabel = {
	delete: '保存データの削除依頼',
	revoke: 'すべてのログインの解除依頼',
	disclosure: '保存データの開示依頼',
}
function receiptTime(value: string) {
	const time = Date.parse(value)
	return Number.isFinite(time)
		? `${new Intl.DateTimeFormat('ja-JP', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Tokyo' }).format(time)}（日本時間）`
		: value
}
const CookieSettings = createContext<() => void>(() => {})

function message(error: unknown) {
	const code = error instanceof RequestError ? error.code : ''
	if (
		code === 'PRIVACY_RECONSENT_REQUIRED' ||
		code === 'POLICY_VERSION_OUTDATED'
	)
		return '最新のプライバシーポリシーと利用規約を確認し、もう一度ログインしてください。'
	if (code === 'SUPPORT_CHANNEL_MISMATCH')
		return '依頼の対象チャンネルと一致しません。窓口から案内された本人確認リンクを使って、対象のチャンネルを確認してください。'
	if (code === 'SUPPORT_CHALLENGE_INVALID')
		return '本人確認リンクが無効か、有効期限が切れています。受付窓口へ新しいリンクをご依頼ください。'
	if (code === 'RATE_LIMITED')
		return 'しばらく待ってから、もう一度お試しください。'
	if (code.startsWith('OAUTH_TRANSACTION_'))
		return '確認の有効期限が切れたか、別の画面でやり直されています。ログインからもう一度お試しください。'
	return '手続きを完了できませんでした。少し待ってから、もう一度お試しください。'
}

export function createApp(
	runtime: BrowserRuntime,
	history?: RouterHistory,
	privacyServices = createPrivacy(),
) {
	function useMemory() {
		return useSyncExternalStore(
			runtime.memory.subscribe,
			runtime.memory.getSnapshot,
		)
	}
	function useMaintenance() {
		return useSyncExternalStore(
			runtime.maintenance.subscribe,
			runtime.maintenance.getSnapshot,
		)
	}
	function MaintenanceGatePending() {
		const maintenance = useMaintenance()
		const unknown = maintenance.phase === 'unknown'
		return (
			<main className="main-content public-page maintenance-page">
				<section className="card maintenance-card" aria-live="polite">
					<p className="eyebrow">マイページ</p>
					<h1>
						{unknown
							? '現在、マイページの状態を確認できません'
							: 'マイページの利用状況を確認しています'}
					</h1>
					<p>
						{unknown
							? '安全のため、状態を確認できるまでログイン情報やマイページの内容は表示しません。'
							: '確認が終わるまで、この画面でお待ちください。'}
					</p>
					{unknown && (
						<button
							className="button primary"
							type="button"
							onClick={() => window.location.reload()}
						>
							再読み込み
						</button>
					)}
				</section>
			</main>
		)
	}
	function RestrictionNotice({ code }: { code: RestrictionCode }) {
		const state = useMemory()
		return (
			<section
				className="card login-card"
				aria-labelledby="restriction-heading"
			>
				<h1 id="restriction-heading">
					{code === 'DATA_DELETION_IN_PROGRESS'
						? '保存データの削除手続き中です'
						: 'サービスの利用を制限しています'}
				</h1>
				<p role="alert">
					{code === 'DATA_DELETION_IN_PROGRESS'
						? '削除手続きが進行しているため、マイページを利用できません。手続きについては受付窓口へお問い合わせください。'
						: 'このチャンネルではマイページを利用できません。利用制限については受付窓口へお問い合わせください。'}
				</p>
				<div className="hero-links">
					<Link className="button" to="/">
						トップへ戻る
					</Link>
					<Link className="button" to="/contact">
						お問い合わせ
					</Link>
					<Link className="button" to="/login" search={{ logout: true }}>
						新しくログインする
					</Link>
					{runtime.session?.currentUID() && (
						<button
							className="button"
							type="button"
							onClick={() => void runtime.memory.logout()}
						>
							{state.error === 'LOGOUT_FAILED'
								? 'ログアウトを再試行'
								: 'ログアウト'}
						</button>
					)}
				</div>
			</section>
		)
	}
	function Root() {
		const [cookieOpen, setCookieOpen] = useState(false)
		const [receipt, setReceipt] = useState<{
			proofRef: string
			supportRequestRef: string | null
		} | null>(null)
		const [intakeDraft, setIntakeDraft] =
			useState<IntakeDraft>(emptyIntakeDraft)
		const cookieDialog = useRef<HTMLDialogElement>(null)
		const cookieOpener = useRef<HTMLElement | null>(null)
		const runtimeDisposer = useRef<(() => void) | null>(null)
		const consent = useSyncExternalStore(
			privacyServices.consent.subscribe,
			privacyServices.consent.getSnapshot,
		)
		const auth = useMemory()
		const maintenance = useMaintenance()
		const href = useRouterState({ select: (state) => state.location.href })
		const openCookie = () => {
			cookieOpener.current =
				document.activeElement instanceof HTMLElement
					? document.activeElement
					: null
			setCookieOpen(true)
		}
		const pathname = useRouterState({
			select: (state) => state.location.pathname,
		})
		const myPageActive = useRouterState({
			select: (state) =>
				state.matches.some((match) => match.routeId === '/mypage') &&
				!state.matches.some((match) => match.globalNotFound),
		})
		const publicChrome =
			pathname !== '/mypage' ||
			auth.phase === 'restricted' ||
			maintenance.phase !== 'normal'
		const supportLogin =
			pathname === '/login' &&
			new URLSearchParams(href.split('?')[1] ?? '').has('supportChallenge')
		const protectedRoute =
			pathname.startsWith('/mypage') || (pathname === '/login' && !supportLogin)
		const maintenanceRoute = pathname === '/maintenance'
		const channelConfirmRoute = pathname === '/login/channel-confirm'
		// Set the initial route before mount can restore an authenticated UID.
		useEffect(() => {
			runtime.memory.setRouteActive(myPageActive)
		}, [myPageActive])
		useLayoutEffect(() => runtime.mountMaintenance(), [])
		useEffect(() => {
			if (
				!runtimeDisposer.current &&
				protectedRoute &&
				maintenance.phase === 'normal'
			)
				runtimeDisposer.current = runtime.mountSession()
		}, [protectedRoute, maintenance.phase])
		useEffect(
			() => () => {
				runtimeDisposer.current?.()
				runtimeDisposer.current = null
			},
			[],
		)
		useLayoutEffect(() => {
			runtime.maintenance.setRouteActive(
				protectedRoute || maintenanceRoute || channelConfirmRoute,
				maintenanceRoute,
			)
		}, [protectedRoute, maintenanceRoute, channelConfirmRoute])
		useEffect(() => {
			const destination = maintenanceDestination(
				pathname,
				supportLogin,
				maintenance.phase,
			)
			if (destination === '/maintenance') {
				void router.navigate({ to: destination, replace: true })
			}
			if (destination === '/') void router.navigate({ to: '/', replace: true })
		}, [maintenance.phase, pathname, supportLogin])
		useEffect(() => {
			if (maintenance.phase === 'normal') runtime.resumeAfterMaintenance()
		}, [maintenance])
		useEffect(() => privacyServices.analytics.mount(), [])
		useEffect(() => {
			privacyServices.analytics.page(href)
		}, [href])
		useEffect(() => {
			if (pathname === '/mypage' && auth.phase === 'authenticated')
				privacyServices.analytics.event('mypage_viewed')
		}, [pathname, auth.phase])
		useEffect(() => {
			const changed = (event: StorageEvent) => {
				if (event.key === consentKey || event.key === null)
					privacyServices.consent.reload()
			}
			window.addEventListener('storage', changed)
			return () => window.removeEventListener('storage', changed)
		}, [])
		useEffect(() => {
			if (pathname !== '/contact') setReceipt(null)
		}, [pathname])
		useEffect(() => {
			const clear = () => {
				setReceipt(null)
				setIntakeDraft(emptyIntakeDraft)
			}
			const clearRestoredPage = (event: PageTransitionEvent) => {
				if (event.persisted) clear()
			}
			let previousUID = runtime.session?.currentUID()
			const unsubscribe = runtime.session?.subscribe((uid) => {
				if (uid !== previousUID) clear()
				previousUID = uid
			})
			window.addEventListener('pagehide', clear)
			window.addEventListener('pageshow', clearRestoredPage)
			return () => {
				unsubscribe?.()
				window.removeEventListener('pagehide', clear)
				window.removeEventListener('pageshow', clearRestoredPage)
			}
		}, [])
		useEffect(() => {
			if (cookieOpen) cookieDialog.current?.showModal()
		}, [cookieOpen])
		return (
			<SupportReceipt.Provider
				value={{
					proofRef: receipt?.proofRef ?? null,
					supportRequestRef: receipt?.supportRequestRef ?? null,
					set: (proofRef, supportRequestRef = null) =>
						setReceipt(proofRef ? { proofRef, supportRequestRef } : null),
				}}
			>
				<IntakeMemory.Provider
					value={{ value: intakeDraft, set: setIntakeDraft }}
				>
					<CookieSettings.Provider value={openCookie}>
						<div
							className={publicChrome ? 'app-shell public-shell' : undefined}
						>
							{publicChrome && <SiteHeader />}
							<Outlet />
							{privacyServices.analytics.available &&
								consent.value === 'unset' && (
									<aside
										className="consent-banner"
										aria-labelledby="analytics-consent-heading"
									>
										<h2 id="analytics-consent-heading">
											サイトをもっと使いやすくするために
										</h2>
										<p>
											よろしければ、ページの閲覧やログインなどの操作をGoogle
											アナリティクス（GA4）で分析し、サイトの改善に役立ててもよろしいですか？
										</p>
										<p>
											Cookie等を使用しますが、YouTubeチャンネルID・表示名・作業内容は送信しません。許可しなくてもログインやマイページを利用でき、設定は後から変更できます。
										</p>
										<div className="hero-links">
											<button
												className="button"
												type="button"
												onClick={() => privacyServices.consent.set('granted')}
											>
												アクセス解析を許可
											</button>
											<button
												className="button"
												type="button"
												onClick={() => privacyServices.consent.set('denied')}
											>
												許可しない
											</button>
										</div>
									</aside>
								)}
							{publicChrome && (
								<footer className="site-footer public-footer">
									<Link to="/privacy">プライバシー</Link>
									<Link to="/terms">利用規約</Link>
									<button type="button" onClick={openCookie}>
										Cookie設定
									</button>
									<Link to="/contact">お問い合わせ</Link>
								</footer>
							)}
							{cookieOpen && (
								<dialog
									className="account-dialog"
									ref={cookieDialog}
									onClose={() => {
										setCookieOpen(false)
										queueMicrotask(() => {
											if (cookieOpener.current?.isConnected)
												cookieOpener.current.focus()
											else
												document
													.querySelector<HTMLButtonElement>(
														'.site-footer button',
													)
													?.focus()
										})
									}}
									onKeyDown={(event) => {
										if (event.key !== 'Tab') return
										const targets = [
											...event.currentTarget.querySelectorAll<HTMLElement>(
												'button:not([disabled]), input:not([disabled]), a[href]',
											),
										].filter((item) => item.getClientRects().length > 0)
										const first = targets[0]
										const last = targets.at(-1)
										if (event.shiftKey && document.activeElement === first) {
											event.preventDefault()
											last?.focus()
										} else if (
											!event.shiftKey &&
											document.activeElement === last
										) {
											event.preventDefault()
											first?.focus()
										}
									}}
									aria-labelledby="cookie-heading"
								>
									<h2 id="cookie-heading">Cookie設定</h2>
									{privacyServices.analytics.available ? (
										<>
											<p>
												Google
												アナリティクス（GA4）でページの閲覧や一部の操作を分析し、サイト改善に役立てます。Cookie等を使用しますが、チャンネルID・表示名・作業内容は送信しません。
											</p>
											<label className="consent-label">
												<input
													type="checkbox"
													role="switch"
													aria-checked={consent.value === 'granted'}
													checked={consent.value === 'granted'}
													onChange={(event) =>
														privacyServices.consent.set(
															event.target.checked ? 'granted' : 'denied',
														)
													}
												/>
												<span>アクセス解析を許可</span>
											</label>
											<p role="status">
												{consent.value === 'granted'
													? 'アクセス解析：許可'
													: 'アクセス解析：オフ'}
											</p>
											{!consent.saved && (
												<p role="alert">
													設定を保存できませんでした。この画面では選択を適用しています。次回はもう一度ご確認ください。
												</p>
											)}
										</>
									) : (
										<p role="status">
											任意のアクセス解析は現在利用していません。利用開始時にあらためてご案内します。
										</p>
									)}
									<p>
										認証・セキュリティなどに必要な保存と通信は常に有効です。アクセス解析への同意は任意です。
									</p>
									<button
										type="button"
										className="button"
										onClick={() => cookieDialog.current?.close()}
									>
										閉じる
									</button>
								</dialog>
							)}
						</div>
					</CookieSettings.Provider>
				</IntakeMemory.Provider>
			</SupportReceipt.Provider>
		)
	}
	function PublicHome() {
		return (
			<main className="main-content public-page">
				<p className="eyebrow">オンライン作業部屋</p>
				<h1>自分のペースで、少しずつ。</h1>
				<p>
					オンライン作業部屋は、YouTubeライブチャットで入室・休憩・退室しながら、いっしょに作業する場所です。
				</p>
				<p>
					マイページでは、現在の作業と、今日・今週・累計・直近7日の作業時間を確認できます。
				</p>
				<div className="hero-links">
					<Link className="button primary" to="/login">
						マイページへ
					</Link>
					<ExternalLink href={liveURL}>YouTubeライブを開く</ExternalLink>
					<ExternalLink href={guideURL}>使い方・コマンド</ExternalLink>
				</div>
			</main>
		)
	}
	function Login() {
		const state = useMemory()
		const maintenance = useMaintenance()
		const search = loginRoute.useSearch()
		const receipt = useContext(SupportReceipt)
		const supportMode = !!search.supportChallenge || !!search.supportInvalid
		const [privacy, setPrivacy] = useState(false)
		const [terms, setTerms] = useState(false)
		const [busy, setBusy] = useState(false)
		const [error, setError] = useState<string | null>(null)
		const controller = useRef<AbortController | null>(null)
		useEffect(() => () => controller.current?.abort(), [])
		if (!supportMode && maintenance.phase === 'active')
			return <Navigate to="/maintenance" replace />
		if (!supportMode && maintenance.phase !== 'normal')
			return <MaintenanceGatePending />
		if (
			!supportMode &&
			(state.phase === 'bootstrapping' || state.phase === 'signing-out')
		)
			return (
				<main className="main-content public-page">
					<p role="status">ログイン状態を確認しています…</p>
				</main>
			)
		if (state.phase === 'authenticated' && !search.logout && !supportMode)
			return <Navigate to="/mypage" />
		const start = async () => {
			if (controller.current || !privacy || !terms || search.supportInvalid)
				return
			controller.current = new AbortController()
			setBusy(true)
			setError(null)
			receipt.set(null)
			try {
				if (!supportMode) privacyServices.analytics.event('login_started')
				const url = await runtime.start(
					controller.current.signal,
					search.supportChallenge,
				)
				if (!controller.current.signal.aborted) location.assign(url)
			} catch (e) {
				setError(message(e))
				controller.current = null
				setBusy(false)
			}
		}
		return (
			<main className="main-content public-page">
				<Link to="/">オンライン作業部屋</Link>
				<section className="card login-card">
					<h1>
						{supportMode ? '問い合わせの本人確認' : 'マイページへログイン'}
					</h1>
					<p>
						{supportMode
							? '受付窓口から届いた依頼について、新しくYouTubeの許可を得て対象チャンネルを確認します。現在のログイン状態だけでは本人確認を完了しません。'
							: 'YouTubeチャンネルを確認して、自分の作業記録を表示します。Googleアカウントのメールアドレスではなく、確認したYouTubeチャンネルが作業部屋のアカウントになります。'}
					</p>
					{state.phase === 'restricted' && !supportMode && (
						<p className="notice" role="status">
							現在のチャンネルではマイページを利用できません。新しくYouTubeチャンネルを確認してログインできます。
						</p>
					)}
					{state.error === 'LOGOUT_FAILED' ? (
						<>
							<p role="alert">
								ログアウトを完了できませんでした。もう一度お試しください。
							</p>
							<button
								className="button"
								type="button"
								onClick={() => void runtime.memory.logout()}
							>
								ログアウトを再試行
							</button>
						</>
					) : (
						<>
							{search.error && (
								<p className="notice" role="alert">
									ログインを完了できませんでした。もう一度お試しください。
								</p>
							)}
							{(state.error === 'PRIVACY_RECONSENT_REQUIRED' ||
								state.error === 'POLICY_VERSION_OUTDATED') && (
								<p role="alert">最新の同意内容をご確認ください。</p>
							)}
							{runtime.unavailable && (
								<p role="alert">
									{runtime.unavailable === 'SUPPORTED_BROWSER_REQUIRED'
										? 'このブラウザではログインを開始できません。最新版のChrome、Edge、Firefox、Safariでお試しください。'
										: '現在ログインをご利用いただけません。しばらくしてからお試しください。'}
								</p>
							)}
							{supportMode && (
								<p role="status">
									この操作は依頼の本人確認です。保存データの削除やログインの解除は、この画面では実行しません。
								</p>
							)}
							{search.supportInvalid && (
								<p role="alert">
									本人確認リンクが無効です。受付窓口へ新しいリンクをご依頼ください。
								</p>
							)}
							<label className="consent-label">
								<input
									type="checkbox"
									checked={privacy}
									onChange={(e) => setPrivacy(e.target.checked)}
								/>
								<span>
									<Link to="/privacy">プライバシーポリシー</Link>
									を確認し、同意します
								</span>
							</label>
							<label className="consent-label">
								<input
									type="checkbox"
									checked={terms}
									onChange={(e) => setTerms(e.target.checked)}
								/>
								<span>
									<Link to="/terms">利用規約</Link>を確認し、同意します
								</span>
							</label>
							<button
								className="button primary full-width"
								type="button"
								onClick={() => void start()}
								disabled={
									!privacy ||
									!terms ||
									busy ||
									!!runtime.unavailable ||
									!!search.supportInvalid
								}
							>
								{busy
									? '手続きを開始しています…'
									: supportMode
										? 'YouTubeで本人確認を始める'
										: 'YouTubeでログイン'}
							</button>
							{error && <p role="alert">{error}</p>}
						</>
					)}
				</section>
			</main>
		)
	}
	function ChannelConfirm() {
		const state = useMemory()
		const maintenance = useMaintenance()
		const [supportFlow, setSupportFlow] = useState(false)
		const receipt = useContext(SupportReceipt)
		const [channel, setChannel] = useState<ChannelConfirmation | null>(null)
		const [error, setError] = useState<string | null>(null)
		const [restriction, setRestriction] = useState<RestrictionCode | null>(null)
		const [busy, setBusy] = useState(false)
		const confirming = useRef(false)
		const supportConfirmation = useRef(false)
		const controller = useRef<AbortController | null>(null)
		const channelGeneration = useRef(runtime.memory.generation)
		const activeRestriction =
			restriction ??
			(state.phase === 'restricted' &&
			channel?.purpose === 'login' &&
			channelGeneration.current !== runtime.memory.generation
				? (state.error as RestrictionCode)
				: null)
		useEffect(() => {
			if (!activeRestriction) return
			setChannel(null)
			setBusy(false)
			setRestriction(activeRestriction)
		}, [activeRestriction])
		useEffect(() => {
			let disposed = false
			let interrupted = false
			let kind: 'unknown' | 'login' | 'support' = 'unknown'
			let hasChannel = false
			let maintenanceUpdatedAt =
				runtime.maintenance.getSnapshot().status?.updatedAt ?? null
			let previousUID = runtime.session?.currentUID()
			const close = (markInterrupted: boolean) => {
				if (markInterrupted) interrupted = true
				controller.current?.abort()
				controller.current = null
				hasChannel = false
				setChannel(null)
				setBusy(false)
				confirming.current = false
				if (markInterrupted)
					setError(
						'確認が中断されました。下のリンクから手続きをやり直してください。',
					)
			}
			const load = () => {
				if (
					disposed ||
					interrupted ||
					controller.current ||
					hasChannel ||
					kind === 'support'
				)
					return
				const c = new AbortController()
				controller.current = c
				setError(null)
				void runtime
					.channel(c.signal)
					.then((data) => {
						if (disposed || c.signal.aborted || controller.current !== c) return
						channelGeneration.current = runtime.memory.generation
						kind = data.purpose
						supportConfirmation.current = data.purpose === 'support'
						setSupportFlow(data.purpose === 'support')
						if (data.purpose === 'support') {
							hasChannel = true
							setChannel(data)
							runtime.maintenance.setRouteActive(false)
							return
						}
						privacyServices.analytics.event('channel_confirmation_viewed')
						const current = runtime.maintenance.getSnapshot()
						if (current.phase === 'normal') {
							hasChannel = true
							setChannel(data)
						} else {
							controller.current = null
							if (current.phase === 'active')
								void router.navigate({ to: '/maintenance', replace: true })
						}
					})
					.catch((e) => {
						if (disposed || c.signal.aborted || controller.current !== c) return
						controller.current = null
						if (
							runtime.maintenance.getSnapshot().phase === 'active' ||
							(e instanceof RequestError && e.code === 'MAINTENANCE_ACTIVE')
						) {
							void router.navigate({ to: '/maintenance', replace: true })
							return
						}
						const foundRestriction = restrictionFrom(e)
						if (foundRestriction) {
							setChannel(null)
							setRestriction(foundRestriction)
							return
						}
						if (
							e instanceof RequestError &&
							e.code.startsWith('OAUTH_TRANSACTION_')
						) {
							void router.navigate({
								to: '/login',
								search: { error: 'oauth_state_invalid', logout: true },
							})
						} else {
							if (e instanceof RequestError && e.code.startsWith('SUPPORT_'))
								setSupportFlow(true)
							setError(message(e))
						}
					})
			}
			const maintenanceChanged = () => {
				const current = runtime.maintenance.getSnapshot()
				const changedNormal =
					current.phase === 'normal' &&
					maintenanceUpdatedAt !== null &&
					current.status.updatedAt !== maintenanceUpdatedAt
				maintenanceUpdatedAt = current.status?.updatedAt ?? null
				if (interrupted) return
				if (kind !== 'login') return
				if (current.phase !== 'normal' || changedNormal) close(false)
				if (current.phase === 'active')
					void router.navigate({ to: '/maintenance', replace: true })
				else if (current.phase === 'normal') load()
			}
			const unsubscribe = runtime.session?.subscribe((uid) => {
				if (uid !== previousUID && supportConfirmation.current) close(true)
				previousUID = uid
			})
			const unsubscribeMaintenance =
				runtime.maintenance.subscribe(maintenanceChanged)
			const pagehide = () => close(true)
			const pageshow = (event: PageTransitionEvent) => {
				if (event.persisted) close(true)
			}
			window.addEventListener('pagehide', pagehide)
			window.addEventListener('pageshow', pageshow)
			load()
			return () => {
				disposed = true
				controller.current?.abort()
				unsubscribe?.()
				unsubscribeMaintenance()
				window.removeEventListener('pagehide', pagehide)
				window.removeEventListener('pageshow', pageshow)
			}
		}, [])
		const confirm = async () => {
			if (!channel || confirming.current || !controller.current) return
			confirming.current = true
			const currentController = controller.current
			setBusy(true)
			setError(null)
			try {
				const result = await runtime.confirm(
					channel.confirmationRef,
					currentController.signal,
					channel.purpose,
				)
				if (
					controller.current === currentController &&
					!currentController.signal.aborted
				) {
					if (result.purpose === 'support') {
						receipt.set(result.requestRef, result.supportRequestRef)
						await router.navigate({ to: '/contact' })
					} else {
						privacyServices.analytics.event('login')
						await router.navigate({ to: '/mypage' })
					}
				}
			} catch (e) {
				if (
					controller.current === currentController &&
					!currentController.signal.aborted
				) {
					setChannel(null)
					if (
						channel.purpose === 'login' &&
						e instanceof RequestError &&
						e.code === 'MAINTENANCE_ACTIVE'
					) {
						void router.navigate({ to: '/maintenance', replace: true })
						return
					}
					const restriction = restrictionFrom(e)
					if (restriction) setRestriction(restriction)
					else setError(message(e))
					setBusy(false)
					confirming.current = false
				}
			}
		}
		if (maintenance.phase === 'active' && channel?.purpose !== 'support')
			return <MaintenancePage />
		if (maintenance.phase !== 'normal' && channel?.purpose !== 'support')
			return <MaintenanceGatePending />
		return (
			<main className="main-content public-page channel-confirm-page">
				{activeRestriction ? (
					<RestrictionNotice code={activeRestriction} />
				) : (
					<section className="channel-confirm">
						<h1>YouTubeチャンネルを確認</h1>
						{channel?.purpose === 'support' ||
						(maintenance.phase === 'normal' && channel) ? (
							<>
								{channel.purpose === 'support' ? (
									<>
										<p>
											依頼の目的：{supportPurposeLabel[channel.supportPurpose]}
										</p>
										<p>
											このチャンネルの依頼について、本人確認だけを完了します。依頼の実行状況は受付窓口からご案内します。
										</p>
									</>
								) : (
									<p>作業部屋で使っているチャンネルか確認してください。</p>
								)}
								<div className="card channel-identity">
									<div className="channel-avatar">
										<AccountAvatar url={channel.avatarUrl} />
									</div>
									<p className="account-name">{channel.displayName}</p>
									{channel.handle && <p className="muted">{channel.handle}</p>}
								</div>
								<button
									className="button primary full-width"
									type="button"
									disabled={busy}
									onClick={() => void confirm()}
								>
									{busy
										? '手続きを完了しています…'
										: channel.purpose === 'support'
											? 'この依頼の本人確認を完了'
											: 'このチャンネルでログイン'}
								</button>
							</>
						) : (
							<p role={error ? 'alert' : 'status'}>
								{error ?? 'チャンネルを確認しています…'}
							</p>
						)}
						{supportFlow ? (
							<Link className="manage-link" to="/contact">
								受付窓口へ戻る
							</Link>
						) : (
							<Link
								className="manage-link"
								to="/login"
								search={{ logout: true }}
							>
								ログインからやり直す
							</Link>
						)}
					</section>
				)}
			</main>
		)
	}
	function PrivatePage() {
		const state = useMemory()
		const maintenance = useMaintenance()
		const openCookieSettings = useContext(CookieSettings)
		if (maintenance.phase === 'active')
			return <Navigate to="/maintenance" replace />
		if (maintenance.phase !== 'normal') return <MaintenanceGatePending />
		if (state.phase === 'anonymous')
			return <Navigate to="/login" search={{ logout: true }} />
		if (state.phase === 'restricted')
			return (
				<main className="main-content public-page">
					<RestrictionNotice code={state.error as RestrictionCode} />
				</main>
			)
		return (
			<MyPageView
				state={state}
				refresh={() => void runtime.memory.refresh(true)}
				logout={() => void runtime.memory.logout()}
				cookieSettings={openCookieSettings}
				history={
					runtime.historyEnabled && state.phase === 'authenticated' ? (
						<HistoryPanel key={runtime.memory.generation} runtime={runtime} />
					) : undefined
				}
			/>
		)
	}
	function AccountManage() {
		const state = useMemory()
		const maintenance = useMaintenance()
		if (maintenance.phase === 'active')
			return <Navigate to="/maintenance" replace />
		if (maintenance.phase !== 'normal') return <MaintenanceGatePending />
		if (state.phase === 'anonymous') return <Navigate to="/login" />
		if (state.phase === 'restricted')
			return (
				<main className="main-content public-page">
					<RestrictionNotice code={state.error as RestrictionCode} />
				</main>
			)
		if (state.phase !== 'authenticated')
			return (
				<main className="main-content public-page">
					<p role="status">ログイン状態を確認しています…</p>
				</main>
			)
		return (
			<main className="main-content public-page">
				<Link to="/mypage">マイページへ戻る</Link>
				<section className="card">
					<h1>アカウント管理</h1>
					<p>
						ログアウト、Google側のアクセス許可の取消、保存データの削除は、それぞれ別の操作です。
					</p>
					<button
						className="button"
						type="button"
						onClick={() => void runtime.memory.logout()}
					>
						ログアウト
					</button>
					<details>
						<summary>Google側のアクセス許可</summary>
						<p>
							Googleの許可を取り消しても、このサービスのログインは自動で終了しません。次回ログイン時に再度チャンネルを確認します。
						</p>
						<ExternalLink href="https://myaccount.google.com/connections">
							Googleアカウントの接続設定
						</ExternalLink>
					</details>
					<details>
						<summary>保存データについて</summary>
						<p>
							ログアウトやGoogle側の許可取消だけでは、保存済みの作業記録は削除されません。削除依頼には、対象チャンネルの本人確認が必要です。
						</p>
						<Link to="/contact">保存データの削除について問い合わせる</Link>
					</details>
				</section>
			</main>
		)
	}
	function Contact() {
		const receipt = useContext(SupportReceipt)
		const draft = useContext(IntakeMemory)
		const { purpose, body, created } = draft.value
		const [ref, setRef] = useState('')
		const [status, setStatus] = useState<PrivacyRequestStatus | null>(null)
		const [busy, setBusy] = useState(false)
		const [error, setError] = useState('')
		const [cooldown, setCooldown] = useState(0)
		const [sessionUID, setSessionUID] = useState(
			runtime.session?.currentUID() ?? null,
		)
		const inFlight = useRef(false)
		const controller = useRef<AbortController | null>(null)
		const operation = useRef(0)
		const available =
			runtime.intakeEnabled &&
			!!readPublicPolicyConfig().config.privacyContactURL
		const signedIn = !!sessionUID
		const statusRef =
			receipt.supportRequestRef ?? (ref || created?.requestRef || '')
		const currentStatusRef = useRef(statusRef)
		currentStatusRef.current = statusRef
		const bodyBytes = new TextEncoder().encode(body.trim()).length
		const cooling = cooldown > Date.now()
		useEffect(() => {
			if (!cooling) return
			const timer = window.setTimeout(
				() => setCooldown(0),
				cooldown - Date.now(),
			)
			return () => window.clearTimeout(timer)
		}, [cooldown, cooling])
		const showError = (cause: unknown) => {
			setError(message(cause))
			if (cause instanceof RequestError && cause.code === 'RATE_LIMITED')
				setCooldown(Date.now() + Math.max(1, cause.retryAfter) * 1000)
		}
		useEffect(() => {
			let previousUID = runtime.session?.currentUID()
			const clear = () => {
				operation.current += 1
				controller.current?.abort()
				controller.current = null
				inFlight.current = false
				currentStatusRef.current = ''
				setBusy(false)
				setStatus(null)
				setRef('')
				setError('')
				setCooldown(0)
			}
			const clearRestoredPage = (event: PageTransitionEvent) => {
				if (event.persisted) clear()
			}
			const unsubscribe = runtime.session?.subscribe((uid) => {
				if (uid !== previousUID) clear()
				previousUID = uid
				setSessionUID(uid)
			})
			window.addEventListener('pagehide', clear)
			window.addEventListener('pageshow', clearRestoredPage)
			return () => {
				operation.current += 1
				controller.current?.abort()
				unsubscribe?.()
				window.removeEventListener('pagehide', clear)
				window.removeEventListener('pageshow', clearRestoredPage)
			}
		}, [])
		const change = (nextPurpose: SupportPurpose, nextBody: string) => {
			if (inFlight.current) return
			receipt.set(null)
			draft.set({
				purpose: nextPurpose,
				body: nextBody,
				submissionKey: null,
				created: null,
			})
			setStatus(null)
		}
		const submit = async () => {
			if (
				inFlight.current ||
				!available ||
				!signedIn ||
				cooling ||
				bodyBytes > 2000
			)
				return
			inFlight.current = true
			const sequence = ++operation.current
			const uid = runtime.session?.currentUID()
			setBusy(true)
			setError('')
			receipt.set(null)
			const submissionKey =
				draft.value.submissionKey ?? crypto.randomUUID().replaceAll('-', '')
			draft.set({ ...draft.value, submissionKey })
			const pending = new AbortController()
			controller.current = pending
			try {
				const result = await runtime.submitPrivacyRequest(
					purpose,
					body.trim(),
					submissionKey,
					pending.signal,
				)
				if (
					operation.current === sequence &&
					!pending.signal.aborted &&
					runtime.session?.currentUID() === uid
				) {
					draft.set({ purpose, body: '', submissionKey, created: result })
					setRef(result.requestRef)
				}
			} catch (cause) {
				if (
					operation.current === sequence &&
					!pending.signal.aborted &&
					runtime.session?.currentUID() === uid
				)
					showError(cause)
			} finally {
				if (operation.current === sequence) {
					controller.current = null
					inFlight.current = false
					setBusy(false)
				}
			}
		}
		const loadStatus = async () => {
			if (inFlight.current || !available || !signedIn || cooling) return
			inFlight.current = true
			const sequence = ++operation.current
			const uid = runtime.session?.currentUID()
			const requestedRef = statusRef.trim()
			setBusy(true)
			setError('')
			const pending = new AbortController()
			controller.current = pending
			try {
				const result = await runtime.privacyRequestStatus(
					requestedRef,
					pending.signal,
				)
				if (
					operation.current === sequence &&
					!pending.signal.aborted &&
					runtime.session?.currentUID() === uid &&
					currentStatusRef.current.trim() === requestedRef
				)
					setStatus(result)
			} catch (cause) {
				if (
					operation.current === sequence &&
					!pending.signal.aborted &&
					runtime.session?.currentUID() === uid &&
					currentStatusRef.current.trim() === requestedRef
				) {
					if (
						cause instanceof RequestError &&
						cause.code === 'SUPPORT_CHALLENGE_INVALID'
					)
						setError(
							'本人確認済みの依頼を確認できません。参照番号とログイン中のチャンネルをご確認ください。',
						)
					else showError(cause)
				}
			} finally {
				if (operation.current === sequence) {
					controller.current = null
					inFlight.current = false
					setBusy(false)
				}
			}
		}
		return (
			<main className="main-content public-page">
				<section className="card policy-page">
					<h1>お問い合わせ</h1>
					{receipt.proofRef && (
						<section aria-labelledby="support-complete">
							<h2 id="support-complete">依頼の本人確認が完了しました</h2>
							<p>
								受付窓口で依頼内容と照合して対応します。データの削除やログインの解除は、この画面では実行していません。
							</p>
							<p>確認結果の参照番号</p>
							<p className="proof-reference">{receipt.proofRef}</p>
							<p>
								この番号は画面を離れたり、再読み込みすると表示されなくなります。確認結果は受付窓口からご案内します。
							</p>
						</section>
					)}
					<p>
						保存データの削除、開示、全ログインの解除は、返信可能な窓口と本人確認を通じて受け付けます。
					</p>
					<PublicPolicyContact />
					{available && signedIn ? (
						<section aria-labelledby="privacy-intake-heading">
							<h2 id="privacy-intake-heading">アプリ内で依頼する</h2>
							<p>
								受付後に同じYouTubeチャンネルで本人確認します。本人確認だけで削除や開示は実行しません。
							</p>
							{!created && (
								<form
									className="privacy-intake-form"
									onSubmit={(event) => {
										event.preventDefault()
										void submit()
									}}
								>
									<label htmlFor="privacy-purpose">依頼内容</label>
									<select
										id="privacy-purpose"
										value={purpose}
										disabled={busy}
										onChange={(event) =>
											change(event.target.value as SupportPurpose, body)
										}
									>
										<option value="delete">保存データの削除</option>
										<option value="disclosure">開示</option>
										<option value="revoke">全ログインの解除</option>
									</select>
									<label htmlFor="privacy-body">
										補足（2000バイト以内。認証情報は記入しないでください）
									</label>
									<textarea
										id="privacy-body"
										value={body}
										disabled={busy}
										maxLength={2000}
										onChange={(event) => change(purpose, event.target.value)}
									/>
									<button
										className="button primary"
										disabled={busy || cooling || bodyBytes > 2000}
										type="submit"
									>
										{busy ? '送信中…' : '依頼を受け付ける'}
									</button>
								</form>
							)}
							{created && (
								<div role="status">
									<p className="proof-reference">
										受付しました。参照番号：{created.requestRef}
									</p>
									<p>受付時刻：{receiptTime(created.acceptedAt)}</p>
									{created.supportChallenge && (
										<Link
											to="/login"
											search={{ supportChallenge: created.supportChallenge }}
										>
											YouTubeで本人確認を続ける
										</Link>
									)}
									{!created.supportChallenge &&
										created.status === 'awaiting_proof' && (
											<p role="alert">
												本人確認リンクの期限が切れています。上記のプライバシー窓口へ再案内をご依頼ください。受付時刻は変わりません。
											</p>
										)}
								</div>
							)}
							<h3>依頼の状態・返信</h3>
							<p>
								本人確認後、同じチャンネルの有効なログインがある場合に表示できます。利用制限中でも確認できます。
							</p>
							<label htmlFor="privacy-ref">参照番号</label>
							<input
								id="privacy-ref"
								value={statusRef}
								disabled={busy}
								onChange={(event) => {
									if (inFlight.current) return
									receipt.set(null)
									currentStatusRef.current = event.target.value
									setRef(event.target.value)
									setStatus(null)
								}}
							/>
							<button
								className="button"
								type="button"
								disabled={busy || cooling}
								onClick={() => void loadStatus()}
							>
								状態を確認
							</button>
							{status && (
								<div role="status">
									<p>状態：本人確認済み</p>
									{status.reply && <p>運営からの返信：{status.reply}</p>}
								</div>
							)}
							{error && <p role="alert">{error}</p>}
						</section>
					) : available ? (
						<p role="status">
							この端末に利用可能なログインがありません。上記の返信可能なプライバシー窓口へご連絡ください。利用制限中の方も請求できます。
						</p>
					) : (
						<p role="status">
							本人確認付きの請求窓口は準備中です。公開前に受付方法をご案内します。
						</p>
					)}
					{!available && (
						<p>
							依頼受付後、同じチャンネルを新しいYouTubeの許可で確認し、確認結果を依頼目的に結び付ける案です。通常のログインだけで依頼を実行しません。ログインや本人確認を利用できない場合も、窓口へお問い合わせいただけるよう準備します。
						</p>
					)}
					<ExternalLink href={guideURL}>使い方・コマンド</ExternalLink>
				</section>
			</main>
		)
	}
	function MaintenancePage() {
		const maintenance = useMaintenance()
		const auth = useMemory()
		const [sessionUID, setSessionUID] = useState(
			() => runtime.session?.currentUID() ?? null,
		)
		const heading = useRef<HTMLHeadingElement>(null)
		useEffect(() => heading.current?.focus(), [])
		useEffect(
			() =>
				runtime.session?.subscribe((uid) => setSessionUID(uid)) ?? (() => {}),
			[],
		)
		const estimate = maintenance.status?.estimatedEndAt
		const estimateText = estimate
			? new Intl.DateTimeFormat('ja-JP', {
					dateStyle: 'medium',
					timeStyle: 'short',
					timeZone: 'Asia/Tokyo',
				}).format(Date.parse(estimate))
			: null
		const unknown = maintenance.phase === 'unknown'
		const checking = maintenance.phase === 'checking'
		return (
			<main className="main-content public-page maintenance-page">
				<section
					className="card maintenance-card"
					aria-labelledby="maintenance-heading"
				>
					<p className="eyebrow">マイページ</p>
					<h1 id="maintenance-heading" ref={heading} tabIndex={-1}>
						{unknown
							? '現在、マイページの状態を確認できません'
							: checking
								? 'マイページの利用状況を確認しています'
								: 'ただいまメンテナンス中です'}
					</h1>
					<p>
						{unknown
							? '安全のため、状態を確認できるまでログイン情報やマイページの内容は表示しません。'
							: checking
								? '確認が終わるまで、この画面でお待ちください。'
								: 'マイページを一時的にご利用いただけません。時間をおいて、もう一度お試しください。'}
					</p>
					<div className="maintenance-status" aria-live="polite" role="status">
						{maintenance.phase === 'checking' && '利用状況を確認しています…'}
						{maintenance.phase === 'unknown' &&
							'現在の利用状況を確認できません。安全のため、この画面でお待ちください。'}
						{maintenance.phase === 'active' &&
							(estimateText
								? `終了見込み：${estimateText}（JST）`
								: '終了時刻は未定です。')}
					</div>
					<div className="hero-links maintenance-actions">
						<button
							className="button primary"
							type="button"
							onClick={() => window.location.reload()}
						>
							再読み込み
						</button>
						{sessionUID && (
							<button
								className="button"
								type="button"
								onClick={() => void runtime.memory.logout()}
							>
								{auth.error === 'LOGOUT_FAILED'
									? 'ログアウトを再試行'
									: 'ログアウト'}
							</button>
						)}
						<Link className="button" to="/">
							トップへ戻る
						</Link>
					</div>
				</section>
			</main>
		)
	}
	const root = createRootRoute({
		component: Root,
		notFoundComponent: () => (
			<main className="main-content public-page">
				<h1>ページが見つかりません</h1>
				<Link to="/">トップへ戻る</Link>
			</main>
		),
	})
	const loginRoute = createRoute({
		getParentRoute: () => root,
		path: '/login',
		component: Login,
		validateSearch: (raw: Record<string, unknown>): LoginSearch => ({
			supportInvalid:
				(raw.supportInvalid === true || raw.supportChallenge !== undefined) &&
				!(
					typeof raw.supportChallenge === 'string' &&
					/^[a-f0-9]{64}$/.test(raw.supportChallenge)
				)
					? true
					: undefined,
			error: typeof raw.error === 'string' ? raw.error.slice(0, 64) : undefined,
			logout: raw.logout === true || raw.logout === 'true' ? true : undefined,
			supportChallenge:
				typeof raw.supportChallenge === 'string' &&
				/^[a-f0-9]{64}$/.test(raw.supportChallenge)
					? raw.supportChallenge
					: undefined,
		}),
	})
	const routes = [
		createRoute({
			getParentRoute: () => root,
			path: '/',
			component: PublicHome,
		}),
		loginRoute,
		createRoute({
			getParentRoute: () => root,
			path: '/login/channel-confirm',
			component: ChannelConfirm,
		}),
		createRoute({
			getParentRoute: () => root,
			path: '/mypage',
			component: PrivatePage,
		}),
		createRoute({
			getParentRoute: () => root,
			path: '/mypage/account',
			component: AccountManage,
		}),
		createRoute({
			getParentRoute: () => root,
			path: '/privacy',
			component: PrivacyPolicy,
		}),
		createRoute({
			getParentRoute: () => root,
			path: '/terms',
			component: TermsOfUse,
		}),
		createRoute({
			getParentRoute: () => root,
			path: '/contact',
			component: Contact,
		}),
		createRoute({
			getParentRoute: () => root,
			path: '/maintenance',
			component: MaintenancePage,
		}),
	]
	const router = createRouter({
		routeTree: root.addChildren(routes),
		history,
		scrollRestoration: false,
		getScrollRestorationKey: (location) => location.state.__TSR_key ?? 'public',
	})
	return router
}
