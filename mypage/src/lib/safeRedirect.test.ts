import { describe, expect, it } from 'vitest'

import { sanitizeRedirectPath } from './safeRedirect'

describe('sanitizeRedirectPath', () => {
	it.each([
		{ input: undefined, expected: '/' },
		{ input: '', expected: '/' },
		{ input: '   ', expected: '/' },
		{ input: '/', expected: '/' },
		{ input: '/login', expected: '/login' },
		{ input: '/path?query=1', expected: '/path?query=1' },
		{ input: '/path#section', expected: '/path#section' },
	])('allows safe relative paths: $input', ({ input, expected }) => {
		expect(sanitizeRedirectPath(input)).toBe(expected)
	})

	it.each([
		'https://evil.com',
		'https://evil.com/path',
		'//evil.com',
		'//evil.com/path',
		'javascript:alert(1)',
		'data:text/html,hello',
	])('rejects unsafe values: %s', (input) => {
		expect(sanitizeRedirectPath(input)).toBe('/')
	})

	it('normalizes same-origin absolute URLs to pathname', () => {
		const trustedOrigins = ['http://localhost:18081']

		expect(
			sanitizeRedirectPath('http://localhost:18081/foo', '/', {
				trustedOrigins,
			}),
		).toBe('/foo')
		expect(
			sanitizeRedirectPath(
				'http://localhost:18081/login?reason=link_required',
				'/',
				{
					trustedOrigins,
				},
			),
		).toBe('/login?reason=link_required')
	})

	it('uses custom fallback when redirect is unsafe', () => {
		expect(sanitizeRedirectPath('https://evil.com', '/login')).toBe('/login')
	})

	it('falls back when custom fallback is unsafe', () => {
		expect(
			sanitizeRedirectPath('https://evil.com', 'https://evil.com/login'),
		).toBe('/')
	})
})
