import type { NextApiRequest, NextApiResponse } from 'next'
import {
	isMonitorAccessIssue,
	type MonitorAccessIssue,
	monitorAccessMessages,
} from '../../lib/monitor-access-issues'

const lastLoggedAt = new Map<MonitorAccessIssue, number>()

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
	if (!isMonitorAccessIssue(reason)) {
		res.status(403).end()
		return
	}

	const now = Date.now()
	// 再読み込みや複数ブラウザからの通知でも、ログを埋めないようにする。
	if (now - (lastLoggedAt.get(reason) ?? Number.NEGATIVE_INFINITY) >= 60_000) {
		lastLoggedAt.set(reason, now)
		const loginUrl =
			reason === 'signed-out'
				? `ログインURL: http://localhost:${process.env.PORT ?? '3000'}/monitor-login`
				: ''
		console.info(`[monitor] ${monitorAccessMessages[reason]}${loginUrl}`)
	}
	res.status(204).end()
}
