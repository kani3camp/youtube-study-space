import { formatSeatState } from '../../../lib/format'
import type { CurrentSeat } from '../types'

type CurrentSeatCardProps = {
	currentSeat: CurrentSeat | null
}

export function CurrentSeatCard({ currentSeat }: CurrentSeatCardProps) {
	if (!currentSeat) {
		return (
			<section className="card">
				<p className="cardLabel">現在の状態</p>
				<h2>未入室</h2>
				<p className="mutedText">
					現在、オンライン作業部屋には入室していません。
				</p>
			</section>
		)
	}

	const currentWorkName =
		currentSeat.state === 'break' && currentSeat.breakWorkName !== ''
			? currentSeat.breakWorkName
			: currentSeat.workName

	return (
		<section className="card">
			<p className="cardLabel">現在の状態</p>
			<h2>{formatSeatState(currentSeat.state)}</h2>

			<dl className="detailList">
				<div>
					<dt>席番号</dt>
					<dd>
						{currentSeat.isMemberSeat ? '/' : ''}
						{currentSeat.seatId}
					</dd>
				</div>
				<div>
					<dt>作業内容</dt>
					<dd>{currentWorkName || '未設定'}</dd>
				</div>
			</dl>
		</section>
	)
}
