import type { FC, ReactNode } from 'react'
import { useEffect, useRef, useState } from 'react'
import { reportMonitorAccessIssue } from '../lib/monitor-access-issues'
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
	const lastLoginNotice = useRef<string | null>(null)

	useEffect(() => observeMonitorAuthorization(setAuthorization), [])
	useEffect(() => {
		if (authorization.status === 'authorized') {
			lastLoginNotice.current = null
			return
		}
		if (
			(authorization.status === 'signed-out' ||
				authorization.status === 'unauthorized') &&
			lastLoginNotice.current !== authorization.status
		) {
			lastLoginNotice.current = authorization.status
			void reportMonitorAccessIssue(authorization.status)
		}
	}, [authorization.status])

	switch (authorization.status) {
		case 'authorized':
			return children
		case 'signed-out':
		case 'unauthorized':
		case 'error':
			return (
				<MonitorAccessStatus
					title="認証に失敗しました"
					detail="ライブチャット、または緊急連絡フォームからご連絡ください。"
				/>
			)
		case 'checking':
			return (
				<MonitorAccessStatus
					title="配信画面を準備しています"
					detail="しばらくお待ちください。"
				/>
			)
	}
}

export default MonitorAuthGate
