// Pure validation/translation. This script never authenticates or deploys.
import { readFile, writeFile } from 'node:fs/promises'
import { hostingCandidate } from '../src/hosting.ts'
import { checkReleaseConfigJSON } from '../src/release-config.ts'

try {
	if (process.argv.length !== 5) throw new Error()
	const [, , manifestPath, pathsPath, outputPath] = process.argv
	const source = await readFile(manifestPath, 'utf8')
	const report = checkReleaseConfigJSON(source)
	if (
		!report.configurationValid ||
		report.checks.hostingCandidate !== 'consistent'
	)
		throw new Error()
	const config = JSON.parse(source)
	if (
		config.deployment.environment !== 'development' ||
		config.deployment.projectID !== 'test-youtube-study-space' ||
		config.deployment.publicOrigin !== 'https://test--study-space.web.app' ||
		config.frontend.VITE_MYPAGE_ANALYTICS_READY !== 'false' ||
		config.frontend.VITE_PRIVACY_INTAKE_ENABLED !== 'false' ||
		config.hosting.inventory.analyticsApproved !== false
	)
		throw new Error()
	const paths = JSON.parse(await readFile(pathsPath, 'utf8'))
	const candidate = hostingCandidate(config.hosting, paths).firebase.hosting
	const rest = {
		rewrites: candidate.rewrites.map(({ source, destination, run }) => ({
			glob: source,
			...(destination ? { path: destination } : { run }),
		})),
		headers: candidate.headers.map(({ source, headers }) => ({
			glob: source,
			headers: Object.fromEntries(
				headers.map(({ key, value }) => [key, value]),
			),
		})),
	}
	await writeFile(outputPath, JSON.stringify(rest))
} catch {
	console.error('Hosting release configuration rejected; no deployment.')
	process.exitCode = 1
}
