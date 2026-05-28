export function formatDuration(sec: number): string {
	if (!Number.isFinite(sec) || sec < 0) {
		return '0m'
	}

	const totalMinutes = Math.floor(sec / 60)
	const hours = Math.floor(totalMinutes / 60)
	const minutes = totalMinutes % 60

	if (hours <= 0) {
		return `${minutes}m`
	}

	return `${hours}h ${minutes.toString().padStart(2, '0')}m`
}

export function formatSeatState(state: 'work' | 'break'): string {
	switch (state) {
		case 'work':
			return '作業中'
		case 'break':
			return '休憩中'
	}
}
