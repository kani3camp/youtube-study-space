import { partType } from "./time_table"

export type Bgm = {
    title: string
    artist: string
    file: string
    forPart: string[]
}

export function getCurrentRandomBgm(currentPartName: string): Bgm | null {
    const bgm_list: Bgm[] = []
    for (const bgm of BgmTable) {
        if (bgm.forPart.includes(currentPartName)) {
            bgm_list.push(bgm)
        }
    }
    if (bgm_list.length > 0) {
        return bgm_list[Math.floor(Math.random() * bgm_list.length)]
    }
    return null
}

const AllPartType = [
    partType.Morning,
    partType.BeforeNoon,
    partType.Noon,
    partType.AfterNoon1,
    partType.AfterNoon2,
    partType.Evening,
    partType.Night1,
    partType.Night2,
    partType.MidNight1,
    partType.MidNight2,
    partType.EarlyMorning,
]

export const BgmTable: Bgm[] = [
    {
        title: 'Lo-Fi Sunset',
        artist: 'だんご工房 さん',
        file: '/audio/Lo-Fi_Sunset.mp3',
        forPart: AllPartType,
    },
    {
        title: 'ノスタルジア',
        artist: 'こばっと さん',
        file: '/audio/ノスタルジア_3.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Someday (Prod. Khaim)',
        artist: 'Khaim さん',
        file: '/audio/Someday_(Prod._Khaim).mp3',
        forPart: AllPartType,
    },
    {
        title: 'You and Me',
        artist: 'しゃろう さん',
        file: '/audio/You_and_Me_2.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Somebody (Prod. Khaim)',
        artist: 'Khaim さん',
        file: '/audio/Somebody_(Prod._Khaim).mp3',
        forPart: AllPartType,
    },
    {
        title: '2:23 AM',
        artist: 'しゃろう さん',
        file: '/audio/2_23_AM_2.mp3',
        forPart: [partType.MidNight1, partType.MidNight2],
    },
    {
        title: '10℃',
        artist: 'しゃろう さん',
        file: '/audio/10℃_2.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Chilly',
        artist: 'Kyaai さん',
        file: '/audio/Chilly_2.mp3',
        forPart: AllPartType,
    },
    {
        title: 'カフェの雨音。',
        artist: '夕焼けモンスター さん',
        file: '/audio/カフェの雨音。_2.mp3',
        forPart: AllPartType,
    },
    {
        title: '黒ねこのボッサ',
        artist: 'れいな さん',
        file: '/audio/黒ねこのボッサ.mp3',
        forPart: AllPartType,
    },
    {
        title: '午後のカフェ',
        artist: '高橋　志郎 さん',
        file: '/audio/午後のカフェ.mp3',
        forPart: AllPartType,
    },
    {
        title: 'カフェオレオレオ',
        artist: 'もっぴーさうんど さん',
        file: '/audio/カフェオレオレオ.mp3',
        forPart: AllPartType,
    },
    {
        title: 'あの子のあだ名はピアノさん',
        artist: 'ネコト さん',
        file: '/audio/あの子のあだ名はピアノさん.mp3',
        forPart: AllPartType,
    },
    {
        title: '東京は朝の七時',
        artist: 'ネコト さん',
        file: '/audio/東京は朝の七時.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Somehow (Prod. Khaim)',
        artist: 'Khaim さん',
        file: '/audio/Somehow_(Prod._Khaim).mp3',
        forPart: AllPartType,
    },
    {
        title: 'ローファイ図書委員',
        artist: 'ネコト さん',
        file: '/audio/ローファイ図書委員.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Stay On Your Mind',
        artist: 'Khaim さん',
        file: '/audio/Stay_On_Your_Mind.mp3',
        forPart: AllPartType,
    },
    {
        title: 'RAINY GARDEN',
        artist: 'SAKURA BEATZ.JP さん',
        file: '/audio/RAINY_GARDEN_2.mp3',
        forPart: AllPartType,
    },
    {
        title: '朝日溢れる回廊',
        artist: '畑中ゆう さん',
        file: '/audio/朝日溢れる回廊_2.mp3',
        forPart: AllPartType,
    },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },

    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },
    // {
    //     title: '',
    //     artist: '',
    //     file: '',
    //     forPart: AllPartType,
    // },

]