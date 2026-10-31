import { css } from '@emotion/react'
import Head from 'next/head'
import { useRouter } from 'next/router'
import { type FormEvent, useEffect, useState } from 'react'
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

export default function OperatorLogin(): JSX.Element {
	const router = useRouter()
	const [authorization, setAuthorization] = useState<MonitorAuthorizationState>(
		{ status: 'checking' },
	)
	const [email, setEmail] = useState('')
	const [password, setPassword] = useState('')
	const [submitting, setSubmitting] = useState(false)
	const [loginFailed, setLoginFailed] = useState(false)

	useEffect(() => observeMonitorAuthorization(setAuthorization), [])

	const submit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault()
		setSubmitting(true)
		setLoginFailed(false)
		try {
			await signInMonitor(email, password)
			await router.replace('/')
		} catch {
			setLoginFailed(true)
		} finally {
			setSubmitting(false)
		}
	}

	return (
		<>
			<Head>
				<title>YouTube Monitor Operator Login</title>
				<meta name="robots" content="noindex,nofollow" />
			</Head>
			<main css={page}>
				<section css={card}>
					<h1>モニター専用ログイン</h1>
					<p css={warning}>
						この画面を配信映像へ表示しないでください。OBSのブラウザソースを非表示にして「対話」から操作します。
					</p>
					{authorization.status === 'authorized' ? (
						<>
							<p>認証と monitor_reader claim を確認しました。</p>
							<button type="button" onClick={() => void router.replace('/')}>
								モニターへ戻る
							</button>
							<button type="button" onClick={() => void signOutMonitor()}>
								ログアウト
							</button>
						</>
					) : (
						<form onSubmit={(event) => void submit(event)}>
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
									ログインできませんでした。専用アカウントと設定を確認してください。
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
