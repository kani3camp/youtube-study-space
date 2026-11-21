export function formatWorkSeconds(workSec: number | null): string {
	if (workSec === null) return '取得できません'
	const minutes = Math.floor(workSec / 60)
	return `${Math.floor(minutes / 60)}時間 ${minutes % 60}分`
}
