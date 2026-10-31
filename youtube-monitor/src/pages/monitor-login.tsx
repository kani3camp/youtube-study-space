import { css } from '@emotion/react'
import Head from 'next/head'
import { type FormEvent, useEffect, useState } from 'react'
import {
	monitorAccessMessages,
	reportMonitorAccessIssue,
} from '../lib/monitor-access-issues'
import {
	type MonitorAuthorizationState,
	observeMonitorAuthorization,
	signInMonitor,
	signOutMonitor,
} from '../lib/monitor-auth'

const page = css`
	min-height: 100vh;
	display: flex;
	align-items: center;
	justify-content: center;
	background: #10151f;
	color: #f4f7fb;
`

const card = css`
	width: min(560px, calc(100vw - 64px));
	padding: 48px;
	border: 1px solid rgba(255, 255, 255, 0.28);
	border-radius: 20px;
	background: #192231;

	form {
		display: grid;
		gap: 20px;
	}

	label {
		display: grid;
		gap: 8px;
	}

	input,
	button {
		box-sizing: border-box;
		width: 100%;
		min-height: 52px;
		padding: 10px 14px;
		border: 1px solid rgba(255, 255, 255, 0.35);
		border-radius: 8px;
		font: inherit;
	}

	input {
		background: #0d131d;
		color: #f4f7fb;
	}

	button {
		margin-top: 8px;
		cursor: pointer;
		background: #f4f7fb;
		color: #10151f;
		font-weight: 700;
	}

	button:disabled {
		cursor: wait;
		opacity: 0.65;
	}
`

const warning = css`
	padding: 16px;
	border-left: 4px solid #ffcc66;
	background: rgba(255, 204, 102, 0.12);
	line-height: 1.5;
`

const errorStyle = css`
	color: #ffb4b4;
`

export default function MonitorLogin(): JSX.Element {
	const [authorization, setAuthorization] = useState<MonitorAuthorizationState>(
		{ status: 'checking' },
	)
	const [email, setEmail] = useState('')
	const [password, setPassword] = useState('')
	const [submitting, setSubmitting] = useState(false)
	const [loginFailed, setLoginFailed] = useState(false)
	const [switchingAccount, setSwitchingAccount] = useState(false)
	const [switchFailed, setSwitchFailed] = useState(false)

	useEffect(() => observeMonitorAuthorization(setAuthorization), [])
	useEffect(() => {
		if (authorization.status === 'denied') {
			void reportMonitorAccessIssue(authorization.issue)
		} else if (authorization.status === 'configuration-error') {
			void reportMonitorAccessIssue('firebase-config-missing')
		}
	}, [authorization])

	const submit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault()
		setSubmitting(true)
		setLoginFailed(false)
		setSwitchFailed(false)
		try {
			await signInMonitor(email, password)
			setPassword('')
		} catch {
			setLoginFailed(true)
		} finally {
			setSubmitting(false)
		}
	}

	const switchAccount = async () => {
		setSwitchingAccount(true)
		setSwitchFailed(false)
		try {
			await signOutMonitor()
			setEmail('')
			setPassword('')
			setLoginFailed(false)
		} catch {
			setSwitchFailed(true)
		} finally {
			setSwitchingAccount(false)
		}
	}

	return (
		<>
			<Head>
				<title>モニター認証 | YouTube Study Space</title>
				<meta name="robots" content="noindex,nofollow" />
			</Head>
			<main css={page}>
				<section css={card}>
					<h1>モニター認証</h1>
					{switchingAccount ? (
						<p role="status">ログアウトしています…</p>
					) : authorization.status === 'configuration-error' ? (
						<p css={errorStyle} role="alert">
							{monitorAccessMessages['firebase-config-missing']}
						</p>
					) : authorization.status === 'checking' ? (
						<p role="status">
							ログイン状態とモニター閲覧設定を確認しています。
						</p>
					) : authorization.status === 'authorized' ? (
						<p role="status">ログインしました。</p>
					) : authorization.status === 'denied' ? (
						<>
							<p css={errorStyle} role="alert">
								{monitorAccessMessages[authorization.issue]}
							</p>
							{switchFailed ? (
								<p css={errorStyle} role="alert">
									ログアウトできませんでした。もう一度お試しください。
								</p>
							) : null}
							<button type="button" onClick={() => void switchAccount()}>
								別のアカウントでログイン
							</button>
						</>
					) : (
						<form onSubmit={(event) => void submit(event)}>
							<p css={warning}>
								配信画面が表示されない場合は、ライブチャット、または緊急連絡フォームからご連絡ください。
							</p>
							<label>
								メールアドレス
								<input
									type="email"
									autoComplete="username"
									value={email}
									onChange={(event) => setEmail(event.target.value)}
									required
								/>
							</label>
							<label>
								パスワード
								<input
									type="password"
									autoComplete="current-password"
									value={password}
									onChange={(event) => setPassword(event.target.value)}
									required
								/>
							</label>
							{loginFailed ? (
								<p css={errorStyle} role="alert">
									認証に失敗しました。ライブチャット、または緊急連絡フォームからご連絡ください。
								</p>
							) : null}
							<button type="submit" disabled={submitting}>
								{submitting ? 'ログインしています…' : 'ログイン'}
							</button>
						</form>
					)}
				</section>
			</main>
		</>
	)
}
