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
import { RequestError } from './features/mypage/memory'
import {
	ExternalLink,
	guideURL,
	liveURL,
	MyPageView,
} from './features/mypage/view'
import type { BrowserRuntime, ChannelConfirmation } from './runtime'

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

export function createApp(runtime: BrowserRuntime, history?: RouterHistory) {
	function useMemory() {
		return useSyncExternalStore(
			runtime.memory.subscribe,
			runtime.memory.getSnapshot,
		)
	}
	function Root() {
		const [cookieOpen, setCookieOpen] = useState(false)
		const [receipt, setReceipt] = useState<string | null>(null)
		const cookieDialog = useRef<HTMLDialogElement>(null)
		const pathname = useRouterState({
			select: (state) => state.location.pathname,
		})
		useEffect(() => runtime.mount(), [])
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
				<CookieSettings.Provider value={() => setCookieOpen(true)}>
					<Outlet />
					{pathname !== '/mypage' && (
						<footer className="site-footer public-footer">
							<Link to="/privacy">プライバシー</Link>
							<Link to="/terms">利用規約</Link>
							<button type="button" onClick={() => setCookieOpen(true)}>
								Cookie設定
							</button>
							<Link to="/contact">お問い合わせ</Link>
						</footer>
					)}
					{cookieOpen && (
						<dialog
							className="account-dialog"
							ref={cookieDialog}
							onClose={() => setCookieOpen(false)}
							aria-labelledby="cookie-heading"
						>
							<h2 id="cookie-heading">Cookie設定</h2>
							<p>
								利用状況の計測は初期設定でオフです。作業内容やチャンネル情報は計測へ送りません。
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
		const [supportFlow, setSupportFlow] = useState(false)
		const receipt = useContext(SupportReceipt)
		const [channel, setChannel] = useState<ChannelConfirmation | null>(null)
		const [error, setError] = useState<string | null>(null)
		const [busy, setBusy] = useState(false)
		const confirming = useRef(false)
		const controller = useRef<AbortController | null>(null)
		useEffect(() => {
			const c = new AbortController()
			controller.current = c
			void runtime
				.channel(c.signal)
				.then((data) => {
					if (!c.signal.aborted) {
						setChannel(data)
						setSupportFlow(data.purpose === 'support')
					}
				})
				.catch((e) => {
					if (c.signal.aborted) return
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
			return () => c.abort()
		}, [])
		const confirm = async () => {
			if (!channel || confirming.current || !controller.current) return
			confirming.current = true
			setBusy(true)
			setError(null)
			try {
				const result = await runtime.confirm(
					channel.confirmationRef,
					controller.current.signal,
					channel.purpose,
				)
				if (!controller.current.signal.aborted) {
					if (result.purpose === 'support') {
						receipt.set(result.requestRef)
						await router.navigate({ to: '/contact' })
					} else await router.navigate({ to: '/mypage' })
				}
			} catch (e) {
				if (!controller.current?.signal.aborted) {
					setChannel(null)
					setError(message(e))
					setBusy(false)
					confirming.current = false
				}
			}
		}
		return (
			<main className="main-content public-page">
				<section className="card login-card">
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
								<p>このチャンネルでマイページを利用します。</p>
							)}
							<p className="account-name">{channel.displayName}</p>
							{channel.handle && <p className="muted">{channel.handle}</p>}
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
										: 'このチャンネルで続ける'}
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
						<Link className="manage-link" to="/login" search={{ logout: true }}>
							ログインからやり直す
						</Link>
					)}
				</section>
			</main>
		)
	}
	function PrivatePage() {
		const state = useMemory()
		const openCookieSettings = useContext(CookieSettings)
		if (state.phase === 'anonymous')
			return <Navigate to="/login" search={{ logout: true }} />
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
	function Privacy() {
		return (
			<main className="main-content public-page">
				<section className="card policy-page">
					<h1>プライバシーについて</h1>
					<p>
						マイページは確認したYouTubeチャンネルと作業部屋の記録を結び付けます。YouTubeのOAuth
						tokenはログイン後に保存しません。
					</p>
					<p>
						作業名や作業記録はログインした本人のマイページに表示します。チャンネル情報が古すぎる場合は表示を停止します。
					</p>
					<p>
						利用状況の計測は初期設定でオフです。作業内容、チャンネルID、プロフィール、本人別のAPI応答を計測へ送りません。
					</p>
					<p>保存データに関する請求は、本人確認ができる窓口で受け付けます。</p>
					<Link to="/contact">個人情報・Privacyに関する問い合わせ</Link>
				</section>
			</main>
		)
	}
	function Terms() {
		return (
			<main className="main-content public-page">
				<section className="card policy-page">
					<h1>利用について</h1>
					<p>
						マイページは作業部屋の記録を本人が確認するための機能です。入室・休憩・退室はYouTubeライブチャットから行います。
					</p>
					<p>
						情報を取得できない場合は、記録がない場合と区別して表示します。継続利用や公開条件は、公開前に確定した利用規約を適用します。
					</p>
					<Link to="/contact">お問い合わせ</Link>
				</section>
			</main>
		)
	}
	function Contact() {
		const receipt = useContext(SupportReceipt)
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
					<p role="status">
						本人確認付きの請求窓口は準備中です。公開前に受付方法をご案内します。
					</p>
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
			component: Privacy,
		}),
		createRoute({
			getParentRoute: () => root,
			path: '/terms',
			component: Terms,
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
