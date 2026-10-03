import type { ReactNode } from 'react'

import { env } from '../lib/env'

type AppLayoutProps = {
	children: ReactNode
}

export function AppLayout({ children }: AppLayoutProps) {
	const shellClassName = env.useMock
		? 'appShell appShell--designMock'
		: 'appShell'

	return (
		<div className={shellClassName}>
			<header className="appHeader">
				<p className="appEyebrow">オンライン作業部屋</p>
				<h1 className="appTitle">マイページ</h1>
			</header>
			<main className="appMain">{children}</main>
		</div>
	)
}
