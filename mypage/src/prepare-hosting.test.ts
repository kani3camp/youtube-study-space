import { spawnSync } from 'node:child_process'
import {
	existsSync,
	mkdirSync,
	mkdtempSync,
	readFileSync,
	rmSync,
	writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, it } from 'vitest'

it('refuses an empty build and prepares exact public assets only after an index exists', () => {
	const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
	const temp = mkdtempSync(join(tmpdir(), 'mypage-hosting-test-'))
	if (
		dirname(temp) !== resolve(tmpdir()) ||
		!temp.includes('mypage-hosting-test-')
	)
		throw new Error('Unexpected test directory')
	try {
		const manifest = JSON.parse(
			readFileSync(
				join(root, 'docs/mypage/release/public-config.example.json'),
				'utf8',
			),
		)
		manifest.hosting = {
			environment: 'development',
			projectID: manifest.deployment.projectID,
			publicOrigin: manifest.deployment.publicOrigin,
			serviceID: 'synthetic-api',
			region: 'asia-southeast2',
			target: 'synthetic-web',
			inventory: {
				reviewed: true,
				script: ['https://script.example.invalid'],
				connect: ['https://api.example.invalid'],
				frame: ['https://frame.example.invalid'],
				image: ['https://yt3.ggpht.com', 'https://yt3.googleusercontent.com'],
				analyticsApproved: false,
			},
		}
		const selected = join(temp, 'manifest.json')
		const output = join(temp, 'firebase.json')
		const dist = join(temp, 'dist')
		mkdirSync(dist)
		writeFileSync(selected, JSON.stringify(manifest))
		const run = () =>
			spawnSync(
				process.execPath,
				[
					join(root, 'mypage/scripts/prepare-hosting.mjs'),
					'--manifest',
					selected,
					'--dist',
					dist,
					'--output',
					output,
				],
				{ encoding: 'utf8' },
			)
		expect(run().status).toBe(1)
		writeFileSync(join(dist, 'index.html'), '<!doctype html>')
		const sibling = join(temp, 'other-release')
		mkdirSync(sibling)
		const outside = spawnSync(process.execPath, [
			join(root, 'mypage/scripts/prepare-hosting.mjs'),
			'--manifest',
			selected,
			'--dist',
			dist,
			'--output',
			join(sibling, 'firebase.json'),
		])
		expect(outside.status).toBe(1)
		expect(existsSync(join(sibling, 'firebase.json'))).toBe(false)
		expect(run().status).toBe(0)
		expect(
			JSON.parse(readFileSync(output, 'utf8')).hosting.headers[0].headers,
		).toContainEqual({ key: 'Cache-Control', value: 'no-store' })
		// A repeated preparation cannot silently overwrite a reviewed release.
		expect(run().status).toBe(1)
	} finally {
		rmSync(temp, { recursive: true, force: true })
	}
})
