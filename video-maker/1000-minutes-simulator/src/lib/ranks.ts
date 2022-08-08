export type Rank = {
    FromHours: number
    ToHours: number
    ColorCode: string
}

export const ranks: Rank[] = [
    {
        FromHours: 0,
        ToHours: 5,
        ColorCode: '#fff',
    },
    {
        FromHours: 5,
        ToHours: 10,
        ColorCode: '#FFD4CC',
    },
    {
        FromHours: 10,
        ToHours: 20,
        ColorCode: '#FF9580',
    },
    {
        FromHours: 20,
        ToHours: 30,
        ColorCode: '#FFC880',
    },
    {
        FromHours: 30,
        ToHours: 50,
        ColorCode: '#FFFB7F',
    },
    {
        FromHours: 50,
        ToHours: 70,
        ColorCode: '#D0FF80',
    },
    {
        FromHours: 70,
        ToHours: 100,
        ColorCode: '#9DFF7F',
    },
    {
        FromHours: 100,
        ToHours: 150,
        ColorCode: '#80FF95',
    },
    {
        FromHours: 150,
        ToHours: 200,
        ColorCode: '#80FFC8',
    },
    {
        FromHours: 200,
        ToHours: 300,
        ColorCode: '#80FFFB',
    },
    {
        FromHours: 300,
        ToHours: 400,
        ColorCode: '#80D0FF',
    },
    {
        FromHours: 400,
        ToHours: 500,
        ColorCode: '#809EFF',
    },
    {
        FromHours: 500,
        ToHours: 700,
        ColorCode: '#947FFF',
    },
    {
        FromHours: 700,
        ToHours: 1000,
        ColorCode: '#C880FF',
    },
    {
        FromHours: 1000,
        ToHours: Infinity,
        ColorCode: '#FF7FFF',
    },
]
