import { execFileSync } from 'node:child_process'

test('prints Biome formatter diagnostics for CI repair', () => {
	execFileSync('pnpm', ['exec', 'biome', 'check', '.'], {
		cwd: process.cwd(),
		stdio: 'inherit',
	})
})
