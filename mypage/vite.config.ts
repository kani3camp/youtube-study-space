import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import { staticPolicies } from './scripts/static-policies.ts'

export default defineConfig({
	plugins: [react(), tailwindcss(), staticPolicies()],
	server: { port: 18081 },
})
