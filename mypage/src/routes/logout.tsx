import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { signOutFirebase } from '../features/auth/auth'

export const Route = createFileRoute('/logout')({
	component: LogoutPage,
})

function LogoutPage() {
	const navigate = useNavigate()
	const [failed, setFailed] = useState(false)

	useEffect(() => {
		async function logout() {
			try {
				await signOutFirebase()

				await navigate({
					to: '/login',
					search: { redirect: '/' },
					replace: true,
				})
			} catch {
				setFailed(true)
			}
		}

		void logout()
	}, [navigate])

	if (failed) {
		return (
			<section className="cardStack">
				<div className="card">
					<h2>ログアウトに失敗しました</h2>
					<p className="mutedText">時間を置いてもう一度お試しください。</p>
				</div>
			</section>
		)
	}

	return (
		<section className="cardStack">
			<div className="card">
				<h2>ログアウトしています</h2>
				<p className="mutedText">ログイン画面へ移動します。</p>
			</div>
		</section>
	)
}
