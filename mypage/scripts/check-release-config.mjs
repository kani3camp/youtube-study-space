import { constants } from 'node:fs'
import { open } from 'node:fs/promises'
import {
	checkReleaseConfigJSON,
	releaseConfigMaxBytes,
	releaseInputFailure,
} from '../src/release-config.ts'

// An explicitly selected bounded manifest is the only configuration source.
// Never read process.env, .env, ADC/key files, SDK state or external services.
async function boundedInput(args) {
	if (args.length === 1 && args[0] === '--stdin') {
		const chunks = []
		let bytes = 0
		for await (const chunk of process.stdin) {
			bytes += chunk.length
			if (bytes > releaseConfigMaxBytes) {
				process.stdin.destroy()
				return releaseInputFailure('INPUT_TOO_LARGE')
			}
			chunks.push(chunk)
		}
		return decode(Buffer.concat(chunks))
	}
	if (args.length !== 2 || args[0] !== '--file' || !args[1])
		return releaseInputFailure('INPUT_UNAVAILABLE')
	// O_NONBLOCK avoids hanging on accidentally selected FIFOs; only regular
	// files are read, and a bounded read handles growth after the size check.
	const file = await open(args[1], constants.O_RDONLY | constants.O_NONBLOCK)
	try {
		const stat = await file.stat()
		if (!stat.isFile()) return releaseInputFailure('INPUT_UNAVAILABLE')
		if (stat.size > releaseConfigMaxBytes)
			return releaseInputFailure('INPUT_TOO_LARGE')
		const buffer = Buffer.alloc(releaseConfigMaxBytes + 1)
		let bytes = 0
		while (bytes < buffer.length) {
			const read = await file.read(buffer, bytes, buffer.length - bytes, null)
			if (read.bytesRead === 0) break
			bytes += read.bytesRead
		}
		if (bytes > releaseConfigMaxBytes)
			return releaseInputFailure('INPUT_TOO_LARGE')
		return decode(buffer.subarray(0, bytes))
	} finally {
		await file.close()
	}
}
function decode(buffer) {
	try {
		return checkReleaseConfigJSON(
			new TextDecoder('utf-8', { fatal: true }).decode(buffer),
		)
	} catch {
		return releaseInputFailure('INVALID_JSON')
	}
}

let result
try {
	result = await boundedInput(process.argv.slice(2))
} catch {
	// Filesystem/JSON errors can contain paths, keys or supplied values.
	result = releaseInputFailure('INPUT_UNAVAILABLE')
}
process.stdout.write(`${JSON.stringify(result)}\n`)
process.exitCode = result.configurationValid ? 0 : 1
