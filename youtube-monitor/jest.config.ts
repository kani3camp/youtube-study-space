import type { Config } from 'jest'

const config: Config = {
	coverageProvider: 'v8',
	preset: 'ts-jest',
	roots: ['<rootDir>/src'],
	setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],
	testEnvironment: 'jest-environment-jsdom',
	transform: {
		'^.+\\.(ts|tsx)$': 'ts-jest',
	},
}

export default config
