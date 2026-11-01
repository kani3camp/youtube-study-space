import { constants } from 'node:fs'
import { open } from 'node:fs/promises'
import {
	checkReleaseConfigJSON,
	releaseConfigMaxBytes,
	releaseInputFailure,
} from '../src/release-config.ts'

// 明示的に選んだサイズ制限付きマニフェストだけを設定元にする。
// process.env・.env・ADC・鍵ファイル・SDK状態・外部サービスを読まない。
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
	// 誤ってFIFOを選んでもO_NONBLOCKで待機を防ぎ、通常ファイルだけを読む。
	// サイズ検査後の増加にも、上限付き読み取りで対応する。
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
	// ファイルシステムやJSONのエラーには、パス・キー・入力値が含まれ得る。
	result = releaseInputFailure('INPUT_UNAVAILABLE')
}
process.stdout.write(`${JSON.stringify(result)}\n`)
process.exitCode = result.configurationValid ? 0 : 1
