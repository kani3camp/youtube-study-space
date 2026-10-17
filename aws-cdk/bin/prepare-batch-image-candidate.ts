#!/usr/bin/env node
import * as fs from 'node:fs'
import { writeBatchImageCandidate } from '../lib/batch-image-candidate'

try {
	const [inputFlag, inputPath, outFlag, outPath, ...extra] = process.argv.slice(2)
	if (inputFlag !== '--input' || outFlag !== '--out' || !inputPath || !outPath || extra.length) {
		throw new Error('usage')
	}
	const fd = fs.openSync(inputPath, fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW)
	let input: unknown
	try {
		const stat = fs.fstatSync(fd)
		if (!stat.isFile() || (stat.mode & 0o077) !== 0 ||
			(process.getuid && stat.uid !== process.getuid()) || stat.size > 5 * 1024 * 1024) throw new Error('private input required')
		input = JSON.parse(fs.readFileSync(fd, 'utf8'))
	} finally { fs.closeSync(fd) }
	writeBatchImageCandidate(input as Parameters<typeof writeBatchImageCandidate>[0], outPath)
	console.log('PASS: offline candidate and rollback prepared; live provenance and approval required')
} catch {
	// Inputs contain private infrastructure values. Do not echo paths or data.
	console.error('STOP: invalid candidate input or private output')
	process.exitCode = 1
}
