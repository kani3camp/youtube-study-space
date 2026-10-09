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
	useRef,
	useState,
	useSyncExternalStore,
} from 'react'
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
	ref: string | null
	set: (ref: string | null) => void
}>({ ref: null, set: () => {} })
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
		const [receipt, setReceipt] = useState<string | null>(null)
		const cookieDialog = useRef<HTMLDialogElement>(null)
		const cookieOpener = useRef<HTMLElement | null>(null)
		const consent = useSyncExternalStore(
			privacyServices.consent.subscribe,
			privacyServices.consent.getSnapshot,
		)
		const auth = useMemory()
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
		useEffect(() => runtime.mount(), [])
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
			const clear = () => setReceipt(null)
			let previousUID = runtime.session?.currentUID()
			const unsubscribe = runtime.session?.subscribe((uid) => {
				if (uid !== previousUID) clear()
				previousUID = uid
			})
			window.addEventListener('pagehide', clear)
			return () => {
				unsubscribe?.()
				window.removeEventListener('pagehide', clear)
			}
		}, [])
		useEffect(() => {
			if (cookieOpen) cookieDialog.current?.showModal()
		}, [cookieOpen])
		return (
			<SupportReceipt.Provider value={{ ref: receipt, set: setReceipt }}>
				<CookieSettings.Provider value={openCookie}>
					<Outlet />
					{consent.value === 'unset' && (
						<aside
							className="consent-banner"
							aria-labelledby="analytics-consent-heading"
						>
							<h2 id="analytics-consent-heading">利用状況の計測について</h2>
							<p>
								計測は初期設定でオフです。許可しなくても、ログインやマイページを利用できます。
							</p>
							<p>
								Google
								Analyticsによる任意の計測は公開準備中です。設定だけをこのブラウザに保存します。
							</p>
							<div className="hero-links">
								<button
									className="button"
									type="button"
									onClick={() => privacyServices.consent.set('granted')}
								>
									計測を許可
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
					{(pathname !== '/mypage' || auth.phase === 'restricted') && (
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
											.querySelector<HTMLButtonElement>('.site-footer button')
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
								} else if (!event.shiftKey && document.activeElement === last) {
									event.preventDefault()
									first?.focus()
								}
							}}
							aria-labelledby="cookie-heading"
						>
							<h2 id="cookie-heading">Cookie設定</h2>
							<p>
								Google
								Analyticsの連携は公開準備中です。チャンネル情報・作業内容・本人別のAPI応答は計測へ送りません。
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
								<span>利用状況の計測を許可</span>
							</label>
							<p role="status">
								{consent.value === 'granted'
									? '任意の計測：許可'
									: '任意の計測：オフ'}
							</p>
							{!consent.saved && (
								<p role="alert">
									設定を保存できませんでした。この画面では選択を適用しています。次回はもう一度ご確認ください。
								</p>
							)}
							<p>
								必須の保存と通信は、認証・セキュリティのため常に有効です。任意の計測は初期設定でオフです。
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
				</CookieSettings.Provider>
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
		const search = loginRoute.useSearch()
		const receipt = useContext(SupportReceipt)
		const supportMode = !!search.supportChallenge || !!search.supportInvalid
		const [privacy, setPrivacy] = useState(false)
		const [terms, setTerms] = useState(false)
		const [busy, setBusy] = useState(false)
		const [error, setError] = useState<string | null>(null)
		const controller = useRef<AbortController | null>(null)
		useEffect(() => () => controller.current?.abort(), [])
		if (state.phase === 'bootstrapping' || state.phase === 'signing-out')
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
			const c = new AbortController()
			controller.current = c
			let previousUID = runtime.session?.currentUID()
			const invalidate = () => {
				c.abort()
				setChannel(null)
				setBusy(false)
				setError(
					'確認が中断されました。下のリンクから手続きをやり直してください。',
				)
			}
			const unsubscribe = runtime.session?.subscribe((uid) => {
				if (uid !== previousUID && supportConfirmation.current) invalidate()
				previousUID = uid
			})
			window.addEventListener('pagehide', invalidate)
			void runtime
				.channel(c.signal)
				.then((data) => {
					if (!c.signal.aborted) {
						channelGeneration.current = runtime.memory.generation
						setChannel(data)
						supportConfirmation.current = data.purpose === 'support'
						setSupportFlow(data.purpose === 'support')
						if (data.purpose === 'login')
							privacyServices.analytics.event('channel_confirmation_viewed')
					}
				})
				.catch((e) => {
					if (c.signal.aborted) return
					const restriction = restrictionFrom(e)
					if (restriction) {
						setChannel(null)
						setRestriction(restriction)
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
			return () => {
				c.abort()
				unsubscribe?.()
				window.removeEventListener('pagehide', invalidate)
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
						receipt.set(result.requestRef)
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
					const restriction = restrictionFrom(e)
					if (restriction) setRestriction(restriction)
					else setError(message(e))
					setBusy(false)
					confirming.current = false
				}
			}
		}
		return (
			<main className="main-content public-page channel-confirm-page">
				{activeRestriction ? (
					<RestrictionNotice code={activeRestriction} />
				) : (
					<section className="channel-confirm">
						<h1>YouTubeチャンネルを確認</h1>
						{channel ? (
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
		const openCookieSettings = useContext(CookieSettings)
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
			/>
		)
	}
	function AccountManage() {
		const state = useMemory()
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
		const [purpose, setPurpose] = useState<SupportPurpose>('delete')
		const [body, setBody] = useState('')
		const [created, setCreated] = useState<PrivacyIntakeReceipt | null>(null)
		const [ref, setRef] = useState('')
		const [status, setStatus] = useState<PrivacyRequestStatus | null>(null)
		const [busy, setBusy] = useState(false)
		const [error, setError] = useState('')
		const [cooldown, setCooldown] = useState(0)
		const key = useRef<string | null>(null)
		const inFlight = useRef(false)
		const controller = useRef<AbortController | null>(null)
		const available =
			runtime.intakeEnabled &&
			!!readPublicPolicyConfig().config.privacyContactURL
		const signedIn = !!runtime.session?.currentUID()
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
				controller.current?.abort()
				setCreated(null)
				setStatus(null)
				key.current = null
			}
			const unsubscribe = runtime.session?.subscribe((uid) => {
				if (uid !== previousUID) clear()
				previousUID = uid
			})
			window.addEventListener('pagehide', clear)
			return () => {
				clear()
				unsubscribe?.()
				window.removeEventListener('pagehide', clear)
			}
		}, [])
		const change = (nextPurpose: SupportPurpose, nextBody: string) => {
			setPurpose(nextPurpose)
			setBody(nextBody)
			key.current = null
			setCreated(null)
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
			setBusy(true)
			setError('')
			if (!key.current) key.current = crypto.randomUUID().replaceAll('-', '')
			const pending = new AbortController()
			controller.current = pending
			try {
				const result = await runtime.submitPrivacyRequest(
					purpose,
					body.trim(),
					key.current,
					pending.signal,
				)
				if (!pending.signal.aborted) {
					setCreated(result)
					setRef(result.requestRef)
					setBody('')
				}
			} catch (cause) {
				if (!pending.signal.aborted) showError(cause)
			} finally {
				inFlight.current = false
				setBusy(false)
			}
		}
		const loadStatus = async () => {
			if (inFlight.current || !available || !signedIn || cooling) return
			inFlight.current = true
			setBusy(true)
			setError('')
			const pending = new AbortController()
			controller.current = pending
			try {
				const result = await runtime.privacyRequestStatus(
					(receipt.ref ?? ref).trim(),
					pending.signal,
				)
				if (!pending.signal.aborted) setStatus(result)
			} catch (cause) {
				if (!pending.signal.aborted) {
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
				inFlight.current = false
				setBusy(false)
			}
		}
		return (
			<main className="main-content public-page">
				<section className="card policy-page">
					<h1>お問い合わせ</h1>
					{receipt.ref && (
						<section aria-labelledby="support-complete">
							<h2 id="support-complete">依頼の本人確認が完了しました</h2>
							<p>
								受付窓口で依頼内容と照合して対応します。データの削除やログインの解除は、この画面では実行していません。
							</p>
							<p>確認結果の参照番号</p>
							<p className="proof-reference">{receipt.ref}</p>
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
								value={receipt.ref ?? ref}
								onChange={(event) => {
									receipt.set(null)
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
	]
	const router = createRouter({
		routeTree: root.addChildren(routes),
		history,
		scrollRestoration: false,
		getScrollRestorationKey: (location) => location.state.__TSR_key ?? 'public',
	})
	return router
}
