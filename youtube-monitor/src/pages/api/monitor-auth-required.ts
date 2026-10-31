import type { NextApiRequest, NextApiResponse } from 'next'

const lastLoggedAt = {
	'signed-out': Number.NEGATIVE_INFINITY,
	unauthorized: Number.NEGATIVE_INFINITY,
}

export default function handler(
	req: NextApiRequest,
	res: NextApiResponse,
): void {
	res.setHeader('Cache-Control', 'no-store')
	if (req.method !== 'POST') {
		res.setHeader('Allow', 'POST')
		res.status(405).end()
		return
	}
	// 別サイトのフォーム等からの通知を拒否する。認証情報やリクエスト本文は不要。
	const reason = req.headers['x-monitor-auth-required']
	if (reason !== 'signed-out' && reason !== 'unauthorized') {
		res.status(403).end()
		return
	}

	const now = Date.now()
	// 再読み込みや複数ブラウザからの通知でも、ログを埋めないようにする。
	if (now - lastLoggedAt[reason] >= 60_000) {
		lastLoggedAt[reason] = now
		const message =
			reason === 'signed-out'
				? '未ログインです。'
				: 'ログイン済みですがモニター閲覧権限がありません。monitor_reader: true を確認してください。'
		console.info(
			`[monitor] ${message}ログインURL: http://localhost:${process.env.PORT ?? '3000'}/monitor-login`,
		)
	}
	res.status(204).end()
}
