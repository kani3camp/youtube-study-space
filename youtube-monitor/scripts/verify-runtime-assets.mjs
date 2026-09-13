import { readdir, readFile, stat } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const sourceExtensions = new Set([
	'.js',
	'.jsx',
	'.ts',
	'.tsx',
	'.css',
	'.scss',
	'.sass',
])
const ignoredSourceSuffixes = [
	'.test.js',
	'.test.jsx',
	'.test.ts',
	'.test.tsx',
	'.spec.js',
	'.spec.jsx',
	'.spec.ts',
	'.spec.tsx',
	'.stories.js',
	'.stories.jsx',
	'.stories.ts',
	'.stories.tsx',
]
const ignoredSourceDirectories = new Set(['dev', 'stories'])
const runtimeAssetPattern = /(['"`])(\/(?:images|chime)\/[^'"`\s?#]+)\1/g

const isSourceFile = (filePath) => {
	if (!sourceExtensions.has(path.extname(filePath))) {
		return false
	}
	return !ignoredSourceSuffixes.some((suffix) => filePath.endsWith(suffix))
}

const walkFiles = async (directory, shouldIgnoreDirectory = () => false) => {
	let entries
	try {
		entries = await readdir(directory, { withFileTypes: true })
	} catch (error) {
		if (error?.code === 'ENOENT') {
			return []
		}
		throw error
	}

	const files = []
	for (const entry of entries) {
		const entryPath = path.join(directory, entry.name)
		if (entry.isDirectory()) {
			if (shouldIgnoreDirectory(entryPath)) {
				continue
			}
			files.push(...(await walkFiles(entryPath, shouldIgnoreDirectory)))
		} else if (entry.isFile()) {
			files.push(entryPath)
		}
	}
	return files
}

export const collectReferencedRuntimeAssets = async (srcDir) => {
	const assets = new Set()
	const shouldIgnoreSourceDirectory = (directory) =>
		ignoredSourceDirectories.has(path.relative(srcDir, directory))

	for (const filePath of await walkFiles(srcDir, shouldIgnoreSourceDirectory)) {
		if (!isSourceFile(filePath)) {
			continue
		}
		const source = await readFile(filePath, 'utf8')
		for (const match of source.matchAll(runtimeAssetPattern)) {
			assets.add(match[2])
		}
	}
	return [...assets].sort()
}

export const findBgmFiles = async (audioDir) =>
	(await walkFiles(audioDir))
		.filter((filePath) => path.extname(filePath).toLowerCase() === '.mp3')
		.sort()

const inspectFile = async (filePath) => {
	try {
		const fileStat = await stat(filePath)
		if (!fileStat.isFile()) {
			return 'not-file'
		}
		if (fileStat.size === 0) {
			return 'empty'
		}
		return 'ok'
	} catch (error) {
		if (error?.code === 'ENOENT') {
			return 'missing'
		}
		throw error
	}
}

export const inspectRuntimeAssets = async (rootDir = process.cwd()) => {
	const srcDir = path.join(rootDir, 'src')
	const publicDir = path.join(rootDir, 'public')
	const referencedAssets = await collectReferencedRuntimeAssets(srcDir)
	const issues = []

	for (const assetPath of referencedAssets) {
		const status = await inspectFile(path.join(publicDir, assetPath.slice(1)))
		if (status === 'missing' || status === 'not-file') {
			issues.push(`missing file: public${assetPath}`)
		} else if (status === 'empty') {
			issues.push(`empty file: public${assetPath}`)
		}
	}

	const bgmFiles = await findBgmFiles(path.join(publicDir, 'audio'))
	if (bgmFiles.length === 0) {
		issues.push(
			'no BGM files: public/audio must contain at least one .mp3 file',
		)
	} else {
		for (const bgmFile of bgmFiles) {
			if ((await inspectFile(bgmFile)) === 'empty') {
				issues.push(`empty BGM file: ${path.relative(rootDir, bgmFile)}`)
			}
		}
	}

	return {
		bgmFiles,
		issues,
		referencedAssets,
	}
}

const main = async () => {
	const result = await inspectRuntimeAssets()
	if (result.issues.length > 0) {
		console.error('❌ Runtime asset validation failed.\n')
		for (const issue of result.issues) {
			console.error(`- ${issue}`)
		}
		console.error(
			'\nPlace the required runtime assets under youtube-monitor/public before building or starting the monitor.',
		)
		process.exitCode = 1
		return
	}

	console.log(
		`✅ Runtime assets ready (${result.referencedAssets.length} referenced image/chime files, ${result.bgmFiles.length} BGM files).`,
	)
}

const invokedPath = process.argv[1] ? path.resolve(process.argv[1]) : ''
if (invokedPath === fileURLToPath(import.meta.url)) {
	await main()
}
