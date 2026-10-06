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
}
const CookieSettings = createContext<() => void>(() => {})

function message(error: unknown) {
	const code = error instanceof RequestError ? error.code : ''
	if (
		code === 'PRIVACY_RECONSENT_REQUIRED' ||
		code === 'POLICY_VERSION_OUTDATED'
	)
		return '最新のプライバシーポリシーと利用規約を確認し、もう一度ログインしてください。'
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
		const cookieDialog = useRef<HTMLDialogElement>(null)
		const pathname = useRouterState({
			select: (state) => state.location.pathname,
		})
		useEffect(() => runtime.mount(), [])
		useEffect(() => {
			if (cookieOpen) cookieDialog.current?.showModal()
		}, [cookieOpen])
		return (
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
		if (
			state.phase === 'authenticated' &&
			!search.logout &&
			!search.supportChallenge
		)
			return <Navigate to="/mypage" />
		const start = async () => {
			if (controller.current || !privacy || !terms || search.supportChallenge)
				return
			controller.current = new AbortController()
			setBusy(true)
			setError(null)
			try {
				const url = await runtime.start(controller.current.signal)
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
						{search.supportChallenge
							? '問い合わせの本人確認'
							: 'マイページへログイン'}
					</h1>
					<p>
						YouTubeチャンネルを確認して、自分の作業記録を表示します。Googleアカウントのメールアドレスではなく、確認したYouTubeチャンネルが作業部屋のアカウントになります。
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
							{search.supportChallenge && (
								<p role="status">
									本人確認の受付は準備中です。通常ログインはこの依頼の本人確認にはなりません。
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
									!!search.supportChallenge
								}
							>
								{busy ? 'ログインを開始しています…' : 'YouTubeでログイン'}
							</button>
							{error && <p role="alert">{error}</p>}
						</>
					)}
				</section>
			</main>
		)
	}
	function ChannelConfirm() {
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
					if (!c.signal.aborted) setChannel(data)
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
					} else setError(message(e))
				})
			return () => c.abort()
		}, [])
		const confirm = async () => {
			if (!channel || confirming.current || !controller.current) return
			confirming.current = true
			setBusy(true)
			setError(null)
			try {
				await runtime.confirm(
					channel.confirmationRef,
					controller.current.signal,
				)
				if (!controller.current.signal.aborted)
					await router.navigate({ to: '/mypage' })
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
							<p>このチャンネルでマイページを利用します。</p>
							<p className="account-name">{channel.displayName}</p>
							{channel.handle && <p className="muted">{channel.handle}</p>}
							<button
								className="button primary full-width"
								type="button"
								disabled={busy}
								onClick={() => void confirm()}
							>
								{busy ? 'ログインを完了しています…' : 'このチャンネルで続ける'}
							</button>
						</>
					) : (
						<p role={error ? 'alert' : 'status'}>
							{error ?? 'チャンネルを確認しています…'}
						</p>
					)}
					<Link className="manage-link" to="/login" search={{ logout: true }}>
						ログインからやり直す
					</Link>
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
		return (
			<main className="main-content public-page">
				<section className="card policy-page">
					<h1>お問い合わせ</h1>
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
