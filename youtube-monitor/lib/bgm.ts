import { partType } from "./time_table"

export type Bgm = {
    title: string
    artist: string
    file: string
    forPart: string[]
}

export function getCurrentRandomBgm(currentPartName: string): Bgm {
    const bgm_list: Bgm[] = []
    for (const bgm of (BgmTable.concat(LofiGirlBgmTable))) {
        if (bgm.forPart.includes(currentPartName)) {
            bgm_list.push(bgm)
        }
    }
    if (bgm_list.length > 0) {
        return bgm_list[Math.floor(Math.random() * bgm_list.length)]
    }
    console.error('failed to get current random bgm.')
    return bgm_list[0]
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

export const LofiGirlBgmTable: Bgm[] = [
    {
        title: 'channel 12',
        artist: 'Flovry x tender spring',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'recess',
        artist: 'Flovry x tender spring',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'first heartbreak',
        artist: 'Flovry x tender spring',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'backpack City',
        artist: 'Flovry x tender spring',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'becoming',
        artist: 'Flovry x tender spring',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'c u in class !',
        artist: 'Flovry x tender spring',
        file: '',
        forPart: AllPartType,
    },
    // 1 A.M Study Session
    {
        title: 'Snowman',
        artist: 'WYS',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Cotton Cloud',
        artist: 'Fatb',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'the places we used to walk',
        artist: 'rook1e x tender spring',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'wool gloves',
        artist: 'imagiro',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'I\'m sorry',
        artist: 'Glimlip x Yasper',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Nova',
        artist: 'mell-ø',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'carried away',
        artist: 'goosetaf x the fields tape x francis',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'snow & sand',
        artist: 'j\'san x epektase',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Single Phial',
        artist: 'HM Surf',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Drops',
        artist: 'Cocabona x Glimlip',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'espresso',
        artist: 'Aso',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Luminescence',
        artist: 'Ambulo x mell-ø',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Explorers',
        artist: 'DLJ x BIDØ',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Wish You Were Mine',
        artist: 'Sarcastic Sounds',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Reflections',
        artist: 'BluntOne',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Alone Time',
        artist: 'Purrple Cat',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Owls of the Night',
        artist: 'Kupla',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'amber',
        artist: 'ENRA',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'fever',
        artist: 'Psalm Trees',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Circle',
        artist: 'H.1v',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Cuddlin',
        artist: 'Pandrezz',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Late Night Call',
        artist: 'Jordy Chandra',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Gyoza',
        artist: 'less.people',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Keyframe',
        artist: 'G Mills',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'breeze',
        artist: 'mvdb',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Lunar Drive',
        artist: 'Mondo Loops',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Steps',
        artist: 'dryhope',
        file: '',
        forPart: AllPartType,
    },
    // North Pole
    {
        title: 'Ice Field',
        artist: 'WYS',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Comforting You',
        artist: 'WYS',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Satellite',
        artist: 'WYS',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Take Me Back',
        artist: 'WYS',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Shield',
        artist: 'WYS',
        file: '',
        forPart: AllPartType,
    },
    // L'aventure
    {
        title: 'Hello',
        artist: 'C4C x kokoro',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Say Yes',
        artist: 'C4C x kokoro',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'L\'aventure',
        artist: 'C4C x kokoro',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Chérie',
        artist: 'C4C x kokoro',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Adieu',
        artist: 'C4C x kokoro',
        file: '',
        forPart: AllPartType,
    },
    {
        title: 'Drifter',
        artist: 'C4C x kokoro',
        file: '',
        forPart: AllPartType,
    },
    // 
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