import { describe, expect, it } from 'vitest'
import {
	type HostingInputs,
	hostingCandidate,
	validateHostingAssets,
} from './hosting'

function fixture(): HostingInputs {
	return {
		environment: 'development',
		projectID: 'demo-synthetic-mypage',
		publicOrigin: 'https://example.invalid',
		serviceID: 'synthetic-mypage',
		region: 'asia-southeast2',
		target: 'synthetic-web',
		inventory: {
			reviewed: true,
			script: ['https://scripts.example.invalid/enterprise.js'],
			connect: ['https://api.example.invalid'],
			frame: ['https://frames.example.invalid'],
			image: ['https://images.example.invalid'],
			analyticsApproved: false,
		},
	}
}
describe('Hosting candidate without deployment', () => {
	it('routes both API root and subtree before SPA and preserves strict static security headers', () => {
		const input = fixture()
		const result = hostingCandidate(input)
		expect(result.firebase.hosting.rewrites).toEqual([
			{
				source: '/api',
				run: { serviceId: input.serviceID, region: input.region },
			},
			{
				source: '/api/**',
				run: { serviceId: input.serviceID, region: input.region },
			},
			{ source: '**', destination: '/index.html' },
		])
		const headers = result.firebase.hosting.headers[0]?.headers
		expect(headers).toContainEqual({ key: 'Cache-Control', value: 'no-store' })
		expect(headers).toContainEqual({
			key: 'Referrer-Policy',
			value: 'no-referrer',
		})
		const csp = headers?.find((x) => x.key === 'Content-Security-Policy')?.value
		expect(csp).toContain("default-src 'none'")
		expect(csp).not.toMatch(/unsafe-inline|unsafe-eval|\*/)
		input.environment = 'production'
		input.region = 'asia-northeast2'
		expect(
			hostingCandidate(input).firebase.hosting.rewrites[0]?.run?.region,
		).toBe('asia-northeast2')
	})
	it('rejects incomplete inventory, wrong region/origin and source injection', () => {
		for (const source of [
			'https://*.example.invalid',
			'https://a.invalid; script-src *',
			'http://example.invalid',
			'https://name:secret@example.invalid',
			'https://example.invalid/?code=private',
			'https://example.invalid/#private',
			"'unsafe-inline'",
			'https://example.invalid/%2f',
		]) {
			const input = fixture()
			input.inventory.script = [source]
			expect(() => hostingCandidate(input)).toThrow()
		}
		const input = fixture()
		input.region = 'us-central1'
		expect(() => hostingCandidate(input)).toThrow()
		input.region = 'asia-southeast2'
		input.inventory.reviewed = false
		expect(() => hostingCandidate(input)).toThrow()
		input.inventory.reviewed = true
		input.inventory.connect = []
		expect(() => hostingCandidate(input)).toThrow()
	})
	it('requires separate analytics approval and rejects static API collisions/maps', () => {
		const input = fixture()
		input.inventory.script.push('https://www.googletagmanager.com/gtag/js')
		expect(() => hostingCandidate(input)).toThrow()
		input.inventory.analyticsApproved = true
		expect(() => hostingCandidate(input)).not.toThrow()
		validateHostingAssets([
			'index.html',
			'assets/synthetic.js',
			'assets/synthetic.css',
		])
		for (const path of [
			'api',
			'api/unknown',
			'assets/private.map',
			'../private',
			'/absolute',
			'.env',
		])
			expect(() => validateHostingAssets(['index.html', path])).toThrow()
	})
})
