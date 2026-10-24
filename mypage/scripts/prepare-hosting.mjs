// Offline preparation only: no environment discovery, auth or deploy.
import { lstat, readdir, readFile, writeFile } from 'node:fs/promises'
import { dirname, join, relative, resolve } from 'node:path'
import { hostingCandidate, validateHostingAssets } from '../src/hosting.ts'
import {
	checkReleaseConfigJSON,
	releaseConfigMaxBytes,
} from '../src/release-config.ts'

try {
	const args = process.argv.slice(2)
	if (
		args.length !== 6 ||
		args[0] !== '--manifest' ||
		args[2] !== '--dist' ||
		args[4] !== '--output'
	)
		throw new Error()
	const manifest = resolve(args[1])
	const dist = resolve(args[3])
	const output = resolve(args[5])
	const stat = await lstat(manifest)
	if (!stat.isFile() || stat.size > releaseConfigMaxBytes) throw new Error()
	const source = await readFile(manifest, 'utf8')
	const report = checkReleaseConfigJSON(source)
	if (
		!report.configurationValid ||
		report.checks.hostingCandidate !== 'consistent'
	)
		throw new Error()
	const paths = []
	async function walk(directory, prefix = '') {
		const entries = await readdir(directory, { withFileTypes: true })
		for (const entry of entries) {
			if (entry.isSymbolicLink()) throw new Error()
			const path = prefix + entry.name
			if (entry.isDirectory())
				await walk(join(directory, entry.name), `${path}/`)
			else if (entry.isFile()) paths.push(path)
			else throw new Error()
			if (paths.length > 10000) throw new Error()
		}
	}
	if (!(await lstat(dist)).isDirectory()) throw new Error()
	await walk(dist)
	validateHostingAssets(paths)
	const candidate = hostingCandidate(JSON.parse(source).hosting, paths.sort())
	candidate.firebase.hosting.public = relative(dirname(output), dist)
		.split('\\')
		.join('/')
	// Never replace an existing release config accidentally.
	await writeFile(output, `${JSON.stringify(candidate.firebase, null, 2)}\n`, {
		flag: 'wx',
	})
	console.log(
		'Hosting candidate prepared from actual build paths; no deployment.',
	)
} catch {
	console.error(
		'Hosting preparation unavailable; inspect manifest, build and output selection.',
	)
	process.exitCode = 1
}
