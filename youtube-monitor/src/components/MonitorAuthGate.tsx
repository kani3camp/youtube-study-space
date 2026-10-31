import type { FC, ReactNode } from 'react'
import { useEffect, useState } from 'react'
import {
	type MonitorAuthorizationState,
	observeMonitorAuthorization,
} from '../lib/monitor-auth'
import MonitorAccessStatus from './MonitorAccessStatus'

type MonitorAuthGateProps = {
	children: ReactNode
}

const MonitorAuthGate: FC<MonitorAuthGateProps> = ({ children }) => {
	const [authorization, setAuthorization] = useState<MonitorAuthorizationState>(
		{ status: 'checking' },
	)

	useEffect(() => observeMonitorAuthorization(setAuthorization), [])

	switch (authorization.status) {
		case 'authorized':
			return children
		case 'signed-out':
			return (
				<MonitorAccessStatus
					title="モニター認証が必要です"
					detail="配信画面から隔離した状態で /operator-login を開き、専用アカウントでログインしてください。Firestore は読み込まれていません。"
				/>
			)
		case 'unauthorized':
			return (
				<MonitorAccessStatus
					title="モニター権限がありません"
					detail="専用アカウントの custom claim と UID allowlist を確認してください。Firestore は読み込まれていません。"
				/>
			)
		case 'error':
			return (
				<MonitorAccessStatus
					title="モニター認証を確認できません"
					detail="認証状態またはブラウザの永続化設定を確認してください。安全のためモニターを停止しています。"
				/>
			)
		case 'checking':
			return (
				<MonitorAccessStatus
					title="モニター認証を確認しています"
					detail="認証が確認できるまで Firestore は読み込みません。"
				/>
			)
	}
}

export default MonitorAuthGate
