import type { ReactNode } from 'react'

type IconName =
	| 'refresh'
	| 'info'
	| 'calendar'
	| 'history'
	| 'clock'
	| 'logout'
	| 'settings'

export function Icon({ name }: { name: IconName }) {
	const paths: Record<IconName, ReactNode> = {
		refresh: (
			<>
				<path d="M20 7v5h-5M4 17v-5h5" />
				<path d="M6 7a7 7 0 0 1 12-1l2 3M4 15l2 3a7 7 0 0 0 12-1" />
			</>
		),
		info: (
			<>
				<circle cx="12" cy="12" r="9" />
				<path d="M12 11v6M12 7v1" />
			</>
		),
		calendar: (
			<>
				<rect x="3" y="5" width="18" height="16" rx="3" />
				<path d="M7 3v4M17 3v4M3 11h18" />
			</>
		),
		history: (
			<>
				<path d="M3 4v5h5M3 9a9 9 0 1 1 0 7" />
				<path d="M12 7v5l3 2" />
			</>
		),
		clock: (
			<>
				<circle cx="12" cy="12" r="9" />
				<path d="M12 6v6l4 2" />
			</>
		),
		logout: (
			<>
				<path d="M9 4H5v16h4M9 12h12M17 8l4 4-4 4" />
			</>
		),
		settings: (
			<>
				<circle cx="12" cy="12" r="3" />
				<path d="m9 3-1 3-3 1-2 3 2 2-1 3 3 2 2-1 2 3h3l1-3 3-1 2-3-2-2 1-3-3-2-2 1-2-3Z" />
			</>
		),
	}
	return (
		<svg
			className="ui-icon"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="1.7"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			{paths[name]}
		</svg>
	)
}

export function SiteHeader({
	main = false,
	children,
}: {
	main?: boolean
	children?: ReactNode
}) {
	return (
		<header className="site-header">
			<div className="brand-lockup">
				<span className="brand-mark" aria-hidden="true">
					作
				</span>
				<div>
					<p className="brand-name">オンライン作業部屋</p>
					{main ? (
						<h1 className="brand-page">マイページ</h1>
					) : (
						<p className="brand-page">マイページ</p>
					)}
				</div>
			</div>
			{children && <div className="header-actions">{children}</div>}
		</header>
	)
}
