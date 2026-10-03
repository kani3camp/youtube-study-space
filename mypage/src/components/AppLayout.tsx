import type { ReactNode } from 'react'

type AppLayoutProps = {
	children: ReactNode
}

export function AppLayout({ children }: AppLayoutProps) {
	return (
		<div className="appShell">
			<header className="appHeader">
				<p className="appEyebrow">オンライン作業部屋</p>
				<h1 className="appTitle">マイページ</h1>
			</header>
			<main className="appMain">{children}</main>
		</div>
	)
}
