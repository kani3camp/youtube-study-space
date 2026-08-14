import {
	formatClockTime,
	formatSeatId,
	formatSeatState,
} from '../../../lib/format'
import type { CurrentSeat } from '../types'

type CurrentSeatCardProps = {
	currentSeat: CurrentSeat | null
}

export function CurrentSeatCard({ currentSeat }: CurrentSeatCardProps) {
	if (!currentSeat) {
		return (
			<section className="card currentSeatCard currentSeatCard--idle">
				<div className="currentSeatTopline">
					<p className="cardLabel">現在の作業</p>
				</div>
				<div className="currentSeatState">
					<span className="statusDot statusDot--idle" aria-hidden="true" />
					<h2>未入室</h2>
				</div>
				<p className="currentSeatDescription">
					現在、オンライン作業部屋には入室していません。
				</p>
			</section>
		)
	}

	const currentWorkName =
		currentSeat.state === 'break' && currentSeat.breakWorkName !== ''
			? currentSeat.breakWorkName
			: currentSeat.workName
	const stateClassName = `currentSeatCard--${currentSeat.state}`
	const dotClassName =
		currentSeat.state === 'break' ? 'statusDot statusDot--break' : 'statusDot'

	return (
		<section className={`card currentSeatCard ${stateClassName}`}>
			<div className="currentSeatTopline">
				<p className="cardLabel">現在の作業</p>
				<span className="seatBadge">
					席 {formatSeatId(currentSeat.seatId, currentSeat.isMemberSeat)}
				</span>
			</div>

			<div className="currentSeatState">
				<span className={dotClassName} aria-hidden="true" />
				<h2>{formatSeatState(currentSeat.state)}</h2>
			</div>

			<p className="currentSeatWork">{currentWorkName || '作業内容未設定'}</p>

			<dl className="currentSeatMeta">
				<div>
					<dt>開始</dt>
					<dd>{formatClockTime(currentSeat.startedAt)}</dd>
				</div>
				<div>
					<dt>終了予定</dt>
					<dd>{formatClockTime(currentSeat.until)}</dd>
				</div>
			</dl>
		</section>
	)
}
