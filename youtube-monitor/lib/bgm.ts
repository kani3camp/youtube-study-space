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
    // Ages Ago
    {
        title: 'channel 12',
        artist: 'Flovry x tender spring',
        file: '/audio/lofigirl/Ages Ago/1. channel 12.mp3',
        forPart: AllPartType,
    },
    {
        title: 'recess',
        artist: 'Flovry x tender spring',
        file: '/audio/lofigirl/Ages Ago/2. recess.mp3',
        forPart: AllPartType,
    },
    {
        title: 'first heartbreak',
        artist: 'Flovry x tender spring',
        file: '/audio/lofigirl/Ages Ago/3. first heartbreak.mp3',
        forPart: AllPartType,
    },
    {
        title: 'backpack City',
        artist: 'Flovry x tender spring',
        file: '/audio/lofigirl/Ages Ago/4. backpack city.mp3',
        forPart: AllPartType,
    },
    {
        title: 'becoming',
        artist: 'Flovry x tender spring',
        file: '/audio/lofigirl/Ages Ago/5. becoming.mp3',
        forPart: AllPartType,
    },
    {
        title: 'c u in class !',
        artist: 'Flovry x tender spring',
        file: '/audio/lofigirl/Ages Ago/6. c u in class!.mp3',
        forPart: AllPartType,
    },
    // 1 A.M Study Session
    {
        title: 'Snowman',
        artist: 'WYS',
        file: '/audio/lofigirl/1 A.M Study Session/01 WYS - Snowman (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Cotton Cloud',
        artist: 'Fatb',
        file: '/audio/lofigirl/1 A.M Study Session/03 Fatb - Cotton Cloud (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'the places we used to walk',
        artist: 'rook1e x tender spring',
        file: '/audio/lofigirl/1 A.M Study Session/04 rook1e x tender spring - the places we used to walk (Kupla master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'wool gloves',
        artist: 'imagiro',
        file: '/audio/lofigirl/1 A.M Study Session/05 imagiro - wool gloves (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'I\'m sorry',
        artist: 'Glimlip x Yasper',
        file: '/audio/lofigirl/1 A.M Study Session/06 Glimlip x Yasper - I_m sorry (Mastered).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Nova',
        artist: 'mell-ø',
        file: '/audio/lofigirl/1 A.M Study Session/07 mell-ø - Nova (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'carried away',
        artist: 'goosetaf x the fields tape x francis',
        file: '/audio/lofigirl/1 A.M Study Session/08 goosetaf x the fields tape x francis - carried away (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'snow & sand',
        artist: 'j\'san x epektase',
        file: '/audio/lofigirl/1 A.M Study Session/09 j_san x epektase - snow _ sand (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Single Phial',
        artist: 'HM Surf',
        file: '/audio/lofigirl/1 A.M Study Session/10 HM Surf - Single Phial (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Drops',
        artist: 'Cocabona x Glimlip',
        file: '/audio/lofigirl/1 A.M Study Session/11 cocabona x Glimlip - Drops (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'espresso',
        artist: 'Aso',
        file: '/audio/lofigirl/1 A.M Study Session/12 Aso - espresso (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Luminescence',
        artist: 'Ambulo x mell-ø',
        file: '/audio/lofigirl/1 A.M Study Session/13 Ambulo x mell-ø - Luminescence (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Explorers',
        artist: 'DLJ x BIDØ',
        file: '/audio/lofigirl/1 A.M Study Session/14 DLJ x BIDØ - Explorers (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Wish You Were Mine',
        artist: 'Sarcastic Sounds',
        file: '/audio/lofigirl/1 A.M Study Session/15 Sarcastic Sounds - Wish You Were Mine (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Reflections',
        artist: 'BluntOne',
        file: '/audio/lofigirl/1 A.M Study Session/16 BluntOne - Reflections (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Alone Time',
        artist: 'Purrple Cat',
        file: '/audio/lofigirl/1 A.M Study Session/17 Purrple Cat - Alone Time (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Owls of the Night',
        artist: 'Kupla',
        file: '/audio/lofigirl/1 A.M Study Session/18 Kupla - Owls of the Night (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'amber',
        artist: 'ENRA',
        file: '/audio/lofigirl/1 A.M Study Session/19 ENRA - amber (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'fever',
        artist: 'Psalm Trees',
        file: '/audio/lofigirl/1 A.M Study Session/17 Purrple Cat - Alone Time (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Circle',
        artist: 'H.1v',
        file: '/audio/lofigirl/1 A.M Study Session/21 H.1 - Circle (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Cuddlin',
        artist: 'Pandrezz',
        file: '/audio/lofigirl/1 A.M Study Session/22 Pandrezz - Cuddlin (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Late Night Call',
        artist: 'Jordy Chandra',
        file: '/audio/lofigirl/1 A.M Study Session/23 Jordy Chandra - Late Night Call (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Gyoza',
        artist: 'less.people',
        file: '/audio/lofigirl/1 A.M Study Session/24 less.people - Gyoza (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Keyframe',
        artist: 'G Mills',
        file: '/audio/lofigirl/1 A.M Study Session/25 G Mills - Keyframe (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'breeze',
        artist: 'mvdb',
        file: '/audio/lofigirl/1 A.M Study Session/26 mvdb - breeze (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Lunar Drive',
        artist: 'Mondo Loops',
        file: '/audio/lofigirl/1 A.M Study Session/27 Mondo Loops - Lunar Drive (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Steps',
        artist: 'dryhope',
        file: '/audio/lofigirl/1 A.M Study Session/28 dryhope - Steps (Kupla Master).mp3',
        forPart: AllPartType,
    },
    // North Pole
    {
        title: 'Ice Field',
        artist: 'WYS',
        file: '/audio/lofigirl/North Pole/1 Ice Field.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Comforting You',
        artist: 'WYS',
        file: '/audio/lofigirl/North Pole/2 Comforting You.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Satellite',
        artist: 'WYS',
        file: '/audio/lofigirl/North Pole/3 Satellite.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Take Me Back',
        artist: 'WYS',
        file: '/audio/lofigirl/North Pole/4 Take Me Back .mp3',
        forPart: AllPartType,
    },
    {
        title: 'Shield',
        artist: 'WYS',
        file: '/audio/lofigirl/North Pole/5 Shield .mp3',
        forPart: AllPartType,
    },
    // L'aventure
    {
        title: 'Hello',
        artist: 'C4C x kokoro',
        file: '/audio/lofigirl/L\'Aventure/1. C4C x kokoro - Hello.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Say Yes',
        artist: 'C4C x kokoro',
        file: '/audio/lofigirl/L\'Aventure/2. C4C x kokoro - Say Yes.mp3',
        forPart: AllPartType,
    },
    {
        title: 'L\'aventure',
        artist: 'C4C x kokoro',
        file: '/audio/lofigirl/L\'Aventure/3. C4C x kokoro - L_aventure.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Chérie',
        artist: 'C4C x kokoro',
        file: '/audio/lofigirl/L\'Aventure/4. C4C x kokoro - Chérie.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Adieu',
        artist: 'C4C x kokoro',
        file: '/audio/lofigirl/L\'Aventure/5. C4C x kokoro - Adieu.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Drifter',
        artist: 'C4C x kokoro',
        file: '/audio/lofigirl/L\'Aventure/6. C4C x kokoro - Drifter.mp3',
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