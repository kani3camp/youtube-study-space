const DEFAULT_FALLBACK = '/'

type SanitizeRedirectOptions = {
	trustedOrigins?: string[]
}

function isSafeRelativePath(value: string): boolean {
	return value.startsWith('/') && !value.startsWith('//')
}

function pathnameFromTrustedURL(
	value: string,
	trustedOrigins: string[],
): string | null {
	let url: URL
	try {
		url = new URL(value)
	} catch {
		return null
	}

	if (!trustedOrigins.includes(url.origin)) {
		return null
	}

	return `${url.pathname}${url.search}${url.hash}`
}

function resolveTrustedOrigins(options?: SanitizeRedirectOptions): string[] {
	if (options?.trustedOrigins !== undefined) {
		return options.trustedOrigins
	}

	if (typeof window !== 'undefined') {
		return [window.location.origin]
	}

	return []
}

export function sanitizeRedirectPath(
	value: string | undefined,
	fallback = DEFAULT_FALLBACK,
	options?: SanitizeRedirectOptions,
): string {
	const normalizedFallback =
		fallback.trim() === '' || !isSafeRelativePath(fallback)
			? DEFAULT_FALLBACK
			: fallback

	if (value === undefined) {
		return normalizedFallback
	}

	const trimmed = value.trim()
	if (trimmed === '') {
		return normalizedFallback
	}

	if (isSafeRelativePath(trimmed)) {
		return trimmed
	}

	const pathname = pathnameFromTrustedURL(trimmed, resolveTrustedOrigins(options))
	if (pathname !== null && isSafeRelativePath(pathname)) {
		return pathname
	}

	return normalizedFallback
}
