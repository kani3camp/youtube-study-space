// Synthetic QA only. The normal production build never uses this entry/config.
import { defineConfig } from 'vite'
import base from '../vite.config'

export default defineConfig({
	...base,
	define: { 'import.meta.env.DEV': 'true' },
	build: {
		outDir: '/tmp/mypage-bfcache-qa',
		emptyOutDir: false,
		rollupOptions: { input: 'runtime-visual.html' },
	},
})
