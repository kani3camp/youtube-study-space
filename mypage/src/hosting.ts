// 外部処理を伴わない候補生成だけを行う。Firebaseのinit/deployは実行せず、
// リポジトリの既存Firestore設定も変更しない。
export type HostingInputs = {
	environment: 'development' | 'production'
	projectID: string
	publicOrigin: string
	serviceID: string
	region: string
	target: string
	inventory: {
		reviewed: boolean
		script: string[]
		connect: string[]
		frame: string[]
		image: string[]
		analyticsApproved: boolean
	}
}

function httpsSource(source: string): string {
	try {
		const url = new URL(source)
		if (
			url.protocol !== 'https:' ||
			url.username ||
			url.password ||
			url.search ||
			url.hash ||
			!/^[a-z0-9.-]+$/.test(url.hostname) ||
			url.hostname.includes('..') ||
			url.port ||
			url.pathname.includes('%') ||
			/[\s;*'"\\]/.test(source) ||
			(url.pathname !== '/' && !/^[a-zA-Z0-9/._-]+$/.test(url.pathname))
		)
			throw new Error()
		if (source !== url.origin && source !== url.origin + url.pathname)
			throw new Error()
		return source
	} catch {
		throw new Error('Hosting inventory unavailable')
	}
}
function sources(values: string[]) {
	if (
		!Array.isArray(values) ||
		values.length > 20 ||
		values.some((x) => typeof x !== 'string')
	)
		throw new Error('Hosting inventory unavailable')
	return [...new Set(values.map(httpsSource))].sort()
}

export function hostingCandidate(
	input: HostingInputs,
	assetPaths: string[] = [],
) {
	if (assetPaths.length > 0) validateHostingAssets(assetPaths)
	const immutableAssets = assetPaths.filter((path) =>
		/^assets\/[A-Za-z0-9][A-Za-z0-9._-]*-[A-Za-z0-9_-]{8,}\.(?:js|css|woff2)$/.test(
			path,
		),
	)
	const expectedRegion =
		input.environment === 'development'
			? 'asia-southeast2'
			: input.environment === 'production'
				? 'asia-northeast2'
				: ''
	if (
		!expectedRegion ||
		input.region !== expectedRegion ||
		!/^[a-z][a-z0-9-]{4,28}[a-z0-9]$/.test(input.projectID) ||
		!/^[a-z][a-z0-9-]{0,61}[a-z0-9]$/.test(input.serviceID) ||
		!/^[a-z][a-z0-9-]{0,61}$/.test(input.target) ||
		!input.inventory.reviewed ||
		httpsSource(input.publicOrigin) !== new URL(input.publicOrigin).origin
	)
		throw new Error('Hosting configuration unavailable')
	const script = sources(input.inventory.script)
	const connect = sources(input.inventory.connect)
	const frame = sources(input.inventory.frame)
	const image = sources(input.inventory.image)
	// 接続先一覧の明示を必須とし、SDK/reCAPTCHAの許可リストを推測しない。
	if (
		script.length === 0 ||
		connect.length === 0 ||
		frame.length === 0 ||
		image.length === 0
	)
		throw new Error('Hosting inventory unavailable')
	// レビュー済みの公開YouTubeチャンネル画像オリジン。
	// reCAPTCHAだけの一覧でAccountAvatarを気付かず遮断しないよう、項目の明示を必須とする。
	for (const origin of [
		'https://yt3.ggpht.com',
		'https://yt3.googleusercontent.com',
	]) {
		if (!image.some((source) => source === origin || source === `${origin}/`))
			throw new Error('Hosting account image inventory unavailable')
	}
	const analytics = (value: string) =>
		/(^|\.)(googletagmanager\.com|google-analytics\.com|analytics\.google\.com)$/.test(
			new URL(value).hostname,
		)
	if (
		!input.inventory.analyticsApproved &&
		[...script, ...connect, ...frame, ...image].some(analytics)
	)
		throw new Error('Hosting inventory unavailable')
	const csp = [
		"default-src 'none'",
		"base-uri 'none'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		`script-src 'self' ${script.join(' ')}`,
		`connect-src 'self' ${connect.join(' ')}`,
		`frame-src ${frame.join(' ')}`,
		`img-src 'self' ${image.join(' ')}`,
		"style-src 'self'",
		"font-src 'self'",
		"manifest-src 'self'",
		'upgrade-insecure-requests',
	].join('; ')
	const header = (key: string, value: string) => ({ key, value })
	return {
		projectID: input.projectID,
		firebase: {
			hosting: {
				target: input.target,
				public: 'dist',
				ignore: ['firebase.json', '**/.*', '**/node_modules/**'],
				rewrites: [
					{
						source: '/api',
						run: { serviceId: input.serviceID, region: input.region },
					},
					{
						source: '/api/**',
						run: { serviceId: input.serviceID, region: input.region },
					},
					{ source: '**', destination: '/index.html' },
				],
				headers: [
					{
						source: '**',
						headers: [
							header('Content-Security-Policy', csp),
							header('Referrer-Policy', 'no-referrer'),
							header('Cache-Control', 'no-store'),
							header('X-Content-Type-Options', 'nosniff'),
							header('X-Frame-Options', 'DENY'),
							header('Strict-Transport-Security', 'max-age=31536000'),
							header(
								'Permissions-Policy',
								'camera=(), microphone=(), geolocation=()',
							),
						],
					},
					...immutableAssets.map((path) => ({
						source: `/${path}`,
						headers: [
							header('Cache-Control', 'public, max-age=31536000, immutable'),
						],
					})),
				],
			},
		},
	}
}

// Hostingは書き換え前に既存の静的ファイルを配信する。
// 公開の静的成果物がAPIを隠さないよう、競合する出力とソースマップを拒否する。
export function validateHostingAssets(paths: string[]) {
	if (
		!paths.includes('index.html') ||
		paths.some(
			(path) =>
				path.startsWith('/') ||
				path.split('/').includes('..') ||
				path === 'api' ||
				path.startsWith('api/') ||
				path.startsWith('.') ||
				path.endsWith('.map') ||
				path.includes('\\'),
		)
	)
		throw new Error('Hosting assets unavailable')
}
