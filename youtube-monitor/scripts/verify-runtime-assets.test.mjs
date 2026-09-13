import assert from 'node:assert/strict'
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { inspectRuntimeAssets } from './verify-runtime-assets.mjs'

const withFixture = async (run) => {
	const rootDir = await mkdtemp(path.join(os.tmpdir(), 'runtime-assets-'))
	try {
		await mkdir(path.join(rootDir, 'src'), { recursive: true })
		await mkdir(path.join(rootDir, 'public'), { recursive: true })
		await run(rootDir)
	} finally {
		await rm(rootDir, { recursive: true, force: true })
	}
}

const writeFixtureFile = async (rootDir, relativePath, content = 'fixture') => {
	const filePath = path.join(rootDir, relativePath)
	await mkdir(path.dirname(filePath), { recursive: true })
	await writeFile(filePath, content)
}

test('accepts referenced image/chime assets and recursively discovered BGM files', async () => {
	await withFixture(async (rootDir) => {
		await writeFixtureFile(
			rootDir,
			'src/runtime.tsx',
			"const image = '/images/rooms/room.png'\nconst chime = '/chime/chime1.mp3'\n",
		)
		await writeFixtureFile(rootDir, 'public/images/rooms/room.png')
		await writeFixtureFile(rootDir, 'public/chime/chime1.mp3')
		await writeFixtureFile(rootDir, 'public/audio/album/bgm.mp3')

		const result = await inspectRuntimeAssets(rootDir)
		assert.deepEqual(result.issues, [])
		assert.deepEqual(result.referencedAssets, [
			'/chime/chime1.mp3',
			'/images/rooms/room.png',
		])
		assert.equal(result.bgmFiles.length, 1)
	})
})

test('reports missing referenced assets and an empty BGM directory', async () => {
	await withFixture(async (rootDir) => {
		await writeFixtureFile(
			rootDir,
			'src/runtime.ts',
			"const image = '/images/rooms/missing.png'\n",
		)

		const result = await inspectRuntimeAssets(rootDir)
		assert.deepEqual(result.issues, [
			'missing file: public/images/rooms/missing.png',
			'no BGM files: public/audio must contain at least one .mp3 file',
		])
	})
})

test('ignores asset references that only exist in tests or stories', async () => {
	await withFixture(async (rootDir) => {
		await writeFixtureFile(
			rootDir,
			'src/example.test.ts',
			"const image = '/images/test-only.png'\n",
		)
		await writeFixtureFile(
			rootDir,
			'src/example.stories.tsx',
			"const image = '/images/story-only.png'\n",
		)
		await writeFixtureFile(rootDir, 'public/audio/bgm.mp3')

		const result = await inspectRuntimeAssets(rootDir)
		assert.deepEqual(result.issues, [])
		assert.deepEqual(result.referencedAssets, [])
	})
})
