import fs from 'node:fs'
import path from 'node:path'
import type { NextApiRequest, NextApiResponse } from 'next'

let mp3Files: string[] = []

export default function handler(req: NextApiRequest, res: NextApiResponse) {
	if (mp3Files.length === 0) {
		const directoryPath = path.join(process.cwd(), 'public', 'audio')
		mp3Files = findMp3Files(directoryPath, fs, path)
		console.log(mp3Files)
	}

	if (mp3Files.length === 0) {
		res.status(500).json({ error: 'No mp3 files found' })
		return
	}

	const randomMp3File = mp3Files[Math.floor(Math.random() * mp3Files.length)]
	res.status(200).json({ file: randomMp3File })
}

const findMp3Files = (
	directory: string,
	fs: typeof import('fs'),
	path: typeof import('path'),
): string[] => {
	const files = fs.readdirSync(directory, { withFileTypes: true })
	let fileList: string[] = []

	files.forEach((file) => {
		const fullPath = path.join(directory, file.name)

		if (file.isDirectory()) {
			fileList = fileList.concat(findMp3Files(fullPath, fs, path))
		} else if (path.extname(fullPath).toLowerCase() === '.mp3') {
			const relativePath = `/${path
				.relative(path.join(process.cwd(), 'public'), fullPath)
				.replace(/\\/g, '/')}`
			fileList.push(relativePath)
		}
	})

	return fileList
}
