export type RankAppearance = {
	base: string
	background: string
	outline: string
}

export const rankAppearances = {
	1: { base: '#6B7280', background: '#F0F2F4', outline: '#4B5563' },
	2: { base: '#2E9B5B', background: '#EAF7EF', outline: '#237747' },
	3: { base: '#159B95', background: '#E8F7F5', outline: '#0F756F' },
	4: { base: '#3B73D9', background: '#EAF0FC', outline: '#2D57A8' },
	5: { base: '#6557C7', background: '#EEECFB', outline: '#4B3FA0' },
	6: { base: '#8D4BC2', background: '#F3ECFA', outline: '#6B3795' },
	7: { base: '#D83B93', background: '#FCEAF5', outline: '#A7286F' },
	8: { base: '#E3485D', background: '#FDECEF', outline: '#B92F43' },
	9: { base: '#E56B24', background: '#FFF0E4', outline: '#B84F14' },
	10: { base: '#B98232', background: '#FAF2E4', outline: '#80591F' },
} as const satisfies Record<number, RankAppearance>

export function getRankAppearance(rank: number): RankAppearance | undefined {
	return rankAppearances[rank as keyof typeof rankAppearances]
}
