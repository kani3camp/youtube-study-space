export type Rank = {
	FromHours: number
	ToHours: number
	ColorCode: string
	Image: string
}

export const ranks: Rank[] = [
	{
		FromHours: 0,
		ToHours: 5,
		ColorCode: '#ffffff',
		Image: '/images/forest-5375005_1920.jpg',
	},
	{
		FromHours: 5,
		ToHours: 10,
		ColorCode: '#FFD4CC',
		Image: '/images/wolf-7331712_1920.jpg',
	},
	{
		FromHours: 10,
		ToHours: 20,
		ColorCode: '#FF9580',
		Image: '/images/hd-wallpaper-6203544_1920.jpg',
	},
	{
		FromHours: 20,
		ToHours: 30,
		ColorCode: '#FFC880',
		Image: '/images/flower-7355080.png',
	},
	{
		FromHours: 30,
		ToHours: 50,
		ColorCode: '#FFFB7F',
		Image: '/images/ice-cream-4985161.png',
	},
	{
		FromHours: 50,
		ToHours: 70,
		ColorCode: '#D0FF80',
		Image: '/images/secret-3120483_1920.jpg',
	},
	{
		FromHours: 70,
		ToHours: 100,
		ColorCode: '#9DFF7F',
		Image: '/images/jungle-1807476_1920.jpg',
	},
	{
		FromHours: 100,
		ToHours: 150,
		ColorCode: '#80FF95',
		Image: '/images/mountains-4580418_1920.jpg',
	},
	{
		FromHours: 150,
		ToHours: 200,
		ColorCode: '#80FFC8',
		Image: '/images/zen-2804942_1920.jpg',
	},
	{
		FromHours: 200,
		ToHours: 300,
		ColorCode: '#80FFFB',
		Image: '/images/beach-1836335_1920.jpg',
	},
	{
		FromHours: 300,
		ToHours: 400,
		ColorCode: '#80D0FF',
		Image: '/images/island-2084365_1920.jpg',
	},
	{
		FromHours: 400,
		ToHours: 500,
		ColorCode: '#809EFF',
		Image: '/images/sky-7388593_1920.jpg',
	},
	{
		FromHours: 500,
		ToHours: 700,
		ColorCode: '#947FFF',
		Image: '/images/hd-wallpaper-3625405_1920.jpg',
	},
	{
		FromHours: 700,
		ToHours: 1000,
		ColorCode: '#C880FF',
		Image: '/images/city-5848267_1920.jpg',
	},
	{
		FromHours: 1000,
		ToHours: Number.POSITIVE_INFINITY,
		ColorCode: '#FF7FFF',
		Image: '/images/spring-2545809_1920.jpg',
	},
]

export const hoursToRank = (hours: number): Rank => {
	let rank: Rank | undefined = undefined
	ranks.forEach((r) => {
		if (r.FromHours <= hours && hours < r.ToHours) {
			rank = r
		}
	})
	if (rank !== undefined) {
		return rank
	}
	console.error('invalid hours: ', hours)
	return ranks[0]
}

export const hoursToColorCode = (hours: number): string => {
	return hoursToRank(hours).ColorCode
}
