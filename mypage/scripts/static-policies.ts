import { readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { createServer, type Plugin, type ResolvedConfig } from 'vite'

export function staticPolicies(): Plugin {
	const emptyRoot = '<div id="root"></div>'
	const templateTitle = '<title>オンライン作業部屋 マイページ</title>'

	let config: ResolvedConfig

	return {
		name: 'mypage-static-policies',
		apply: 'build',
		configResolved(resolved) {
			config = resolved
		},
		async writeBundle() {
			const { root, mode, envDir, build } = config
			const outDir = resolve(root, build.outDir)
			const template = await readFile(resolve(outDir, 'index.html'), 'utf8')
			if (!template.includes(emptyRoot) || !template.includes(templateTitle))
				throw new Error(
					'Static policy HTML template is missing its root or title',
				)

			const server = await createServer({
				configFile: false,
				root,
				mode,
				envDir,
				appType: 'custom',
				server: { middlewareMode: true, watch: null, ws: false },
				optimizeDeps: { noDiscovery: true, include: [] },
			})
			try {
				const { renderStaticPolicies } = (await server.ssrLoadModule(
					'/src/static-policies.tsx',
				)) as typeof import('../src/static-policies')
				for (const { filename, title, body } of renderStaticPolicies()) {
					const html = template
						.replace(emptyRoot, () => `<div id="root">${body}</div>`)
						.replace(
							templateTitle,
							() => `<title>${title} | オンライン作業部屋</title>`,
						)
					await writeFile(resolve(outDir, filename), html)
				}
			} finally {
				await server.close()
			}
		},
	}
}
