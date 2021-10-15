import { partType } from "./time_table"

export type Bgm = {
    title: string
    artist: string
    file: string
    forPart: string[]
}

export function getCurrentRandomBgm(currentPartName: string): Bgm {
    const bgm_list: Bgm[] = []
    for (const bgm of (LofiGirlBgmTable)) {
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
    // Perspective
    {
        title: 'First Snow',
        artist: 'Chris Mazuera x tender spring',
        file: '/audio/lofigirl/Perspective/1. First Snow.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Abundance',
        artist: 'Chris Mazuera x tender spring',
        file: '/audio/lofigirl/Perspective/2. Abundance.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Giving, not taking',
        artist: 'Chris Mazuera x tender spring',
        file: '/audio/lofigirl/Perspective/3. Giving, not taking.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Winter\'s Kiss',
        artist: 'Chris Mazuera x tender spring',
        file: '/audio/lofigirl/Perspective/4. Winter_s Kiss.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Stay Mindful',
        artist: 'Chris Mazuera x tender spring',
        file: '/audio/lofigirl/Perspective/5. Stay Mindful ft. The Field Tapes.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Perspective',
        artist: 'Chris Mazuera x tender spring',
        file: '/audio/lofigirl/Perspective/6. Perspective.mp3',
        forPart: AllPartType,
    },
    // Jiro Dreams
    {
        title: 'Maki',
        artist: 'Dontcry x Glimlip',
        file: '/audio/lofigirl/Jiro Dreams/1. Dontcry x Glimlip - Maki.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Ebi Tempura',
        artist: 'Dontcry x Glimlip',
        file: '/audio/lofigirl/Jiro Dreams/2. Dontcry x Glimlip - Ebi Tempura.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sashimi',
        artist: 'Dontcry x Glimlip',
        file: '/audio/lofigirl/Jiro Dreams/3. Dontcry x Glimlip - Sashimi.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Edamame',
        artist: 'Dontcry x Glimlip',
        file: '/audio/lofigirl/Jiro Dreams/4. Dontcry x Glimlip - Edamame.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Jiro Dreams',
        artist: 'Dontcry x Glimlip',
        file: '/audio/lofigirl/Jiro Dreams/5. Dontcry x Glimlip x Sleepermane - Jiro Dreams.mp3',
        forPart: AllPartType,
    },
    // Kingdom in Blue
    {
        title: 'Serenity',
        artist: 'Kupla',
        file: '/audio/lofigirl/Kingdom in Blue/01 Kupla - Serenity (master 2.0).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Valentine',
        artist: 'Kupla',
        file: '/audio/lofigirl/Kingdom in Blue/02 Kupla - Valentine (Mastered).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Dew',
        artist: 'Kupla',
        file: '/audio/lofigirl/Kingdom in Blue/03 Kupla - Dew (master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sunray',
        artist: 'Kupla',
        file: '/audio/lofigirl/Kingdom in Blue/04 Kupla - Sunray (master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sleepy Little One',
        artist: 'Kupla',
        file: '/audio/lofigirl/Kingdom in Blue/05 Kupla - Sleepy Little One (Mastered).mp3',
        forPart: AllPartType,
    },
    {
        title: 'In Your Eyes',
        artist: 'Kupla',
        file: '/audio/lofigirl/Kingdom in Blue/06 Kupla - In Your Eyes (master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Roots',
        artist: 'Kupla',
        file: '/audio/lofigirl/Kingdom in Blue/07 Kupla - Roots (Final).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Kingdom in Blue',
        artist: 'Kupla',
        file: '/audio/lofigirl/Kingdom in Blue/08 Kupla - Kingdom in Blue (master).mp3',
        forPart: AllPartType,
    },
    // Cloud Surfing
    {
        title: 'Antarctic Sunrise',
        artist: 'BluntOne',
        file: '/audio/lofigirl/Cloud Surfing/01_AntarcticSunrise.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Gates Of Heaven',
        artist: 'BluntOne',
        file: '/audio/lofigirl/Cloud Surfing/02_GatesOfHeaven.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Monk Serenity',
        artist: 'BluntOne',
        file: '/audio/lofigirl/Cloud Surfing/03_Monk_Serenity.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Under Your Skin',
        artist: 'BluntOne x Baen Mow',
        file: '/audio/lofigirl/Cloud Surfing/04_UnderYourSkin(BluntOne _ Baen Mow).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Render Your Heart',
        artist: 'BluntOne',
        file: '/audio/lofigirl/Cloud Surfing/05_Render_Your_Heart.mp3',
        forPart: AllPartType,
    },
    // Online Mall Music
    {
        title: 'Dimes',
        artist: 'less.people',
        file: '/audio/lofigirl/Online Mall Music/1. Dimes.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Modigliani Nudes',
        artist: 'less.people',
        file: '/audio/lofigirl/Online Mall Music/2. Modigliani nudes.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Laid Up',
        artist: 'less.people',
        file: '/audio/lofigirl/Online Mall Music/3. Laid up .mp3',
        forPart: AllPartType,
    },
    {
        title: 'Blinds',
        artist: 'less.people',
        file: '/audio/lofigirl/Online Mall Music/4. Blinds.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Home Pour',
        artist: 'less.people',
        file: '/audio/lofigirl/Online Mall Music/5. Home pour.mp3',
        forPart: AllPartType,
    },
    {
        title: 'It Will Be Different, I Swear',
        artist: 'less.people',
        file: '/audio/lofigirl/Online Mall Music/6. It will be different, I swear.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Enough to Go Around',
        artist: 'less.people',
        file: '/audio/lofigirl/Online Mall Music/7. Enough to go around.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Everything\'s a Symptom',
        artist: 'less.people',
        file: '/audio/lofigirl/Online Mall Music/8. Everything_s a symptom.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Forget Me',
        artist: 'less.people',
        file: '/audio/lofigirl/Online Mall Music/9. Forget me.mp3',
        forPart: AllPartType,
    },
    // Night Emotions
    {
        title: 'Abandoned',
        artist: 'DLJ x TABAL',
        file: '/audio/lofigirl/Night Emotions/1 - Abandoned (w_ TABAL) MASTER v2.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Blackout',
        artist: 'DLJ',
        file: '/audio/lofigirl/Night Emotions/2 - Blackout MASTER V3.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Further',
        artist: 'DLJ',
        file: '/audio/lofigirl/Night Emotions/3 Further MASTER.mp3',
        forPart: AllPartType,
    },
    {
        title: 'The Docks',
        artist: 'DLJ',
        file: '/audio/lofigirl/Night Emotions/4 - The Docks MASTER V3.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Truth',
        artist: 'DLJ',
        file: '/audio/lofigirl/Night Emotions/5 - Truth MASTER.mp3',
        forPart: AllPartType,
    },
    // Afloat Again
    {
        title: 'Childhood Memories',
        artist: 'mell-ø x Ambulo',
        file: '/audio/lofigirl/Afloat Again/1 Childhood Memories (MASTER).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Solace',
        artist: 'mell-ø x Ambulo',
        file: '/audio/lofigirl/Afloat Again/2 Solace (MASTER2) .mp3',
        forPart: AllPartType,
    },
    {
        title: 'Afloat Again',
        artist: 'mell-ø x Ambulo',
        file: '/audio/lofigirl/Afloat Again/3 Afloat Again (MASTER).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Breathe',
        artist: 'mell-ø x Ambulo',
        file: '/audio/lofigirl/Afloat Again/4 Breathe (MASTER).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Dozing Off',
        artist: 'mell-ø x Ambulo',
        file: '/audio/lofigirl/Afloat Again/5 Dozing Off (MASTER).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Stay the Same',
        artist: 'mell-ø x Ambulo',
        file: '/audio/lofigirl/Afloat Again/6 Stay the Same (MASTER).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Gloom',
        artist: 'mell-ø x Ambulo',
        file: '/audio/lofigirl/Afloat Again/7 Gloom (MASTER).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Epilogue',
        artist: 'mell-ø x Ambulo',
        file: '/audio/lofigirl/Afloat Again/8 Epilogue (MASTER 2).mp3',
        forPart: AllPartType,
    },
    // Perpetual
    {
        title: 'Perpetual',
        artist: 'goosetaf',
        file: '/audio/lofigirl/Perpetual/1 - Perpetual.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Spend Some Time',
        artist: 'goosetaf x fantompower',
        file: '/audio/lofigirl/Perpetual/2 - Spend Some Time w_ fantompower.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Looking Back',
        artist: 'goosetaf',
        file: '/audio/lofigirl/Perpetual/3 - Looking Back.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Full Tide',
        artist: 'goosetaf x HM Surf',
        file: '/audio/lofigirl/Perpetual/4 - Full Tide w_ HM Surf.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sunday Fog',
        artist: 'goosetaf',
        file: '/audio/lofigirl/Perpetual/5 - Sunday Fog.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Transcend',
        artist: 'goosetaf',
        file: '/audio/lofigirl/Perpetual/6 - Transcend.mp3',
        forPart: AllPartType,
    },
    // Contrasts
    {
        title: 'Amber',
        artist: 'dryhope',
        file: '/audio/lofigirl/Contrasts/1. Amber.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Down River',
        artist: 'dryhope',
        file: '/audio/lofigirl/Contrasts/2. Down River.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Someday',
        artist: 'dryhope',
        file: '/audio/lofigirl/Contrasts/3. Someday.mp3',
        forPart: AllPartType,
    },
    {
        title: 'First Light',
        artist: 'dryhope',
        file: '/audio/lofigirl/Contrasts/4. First Light.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Contrasts',
        artist: 'dryhope',
        file: '/audio/lofigirl/Contrasts/5. Contrasts.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Shade',
        artist: 'dryhope',
        file: '/audio/lofigirl/Contrasts/6. Shade.mp3',
        forPart: AllPartType,
    },
    // TODO: このアルバムはnpm run startのときに再生できなくなる原因となってしまう。原因不明。npm run devだと問題なし。
    // Frozen Roses
    {
        title: 'A While',
        artist: 'a[way]',
        file: '/audio/lofigirl/Frozen Roses/1 a[way] - A While.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Frosted Wood',
        artist: 'a[way]',
        file: '/audio/lofigirl/Frozen Roses/2 a[way] - Frosted Wood.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Frozen Snow',
        artist: 'a[way]',
        file: '/audio/lofigirl/Frozen Roses/3 a[way] - Frozen Snow.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Cozy Dreams',
        artist: 'a[way]',
        file: '/audio/lofigirl/Frozen Roses/4 a[way] - Cozy Dreams.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Warm Nights',
        artist: 'a[way]',
        file: '/audio/lofigirl/Frozen Roses/5 a[way] - Warm Nights.mp3',
        forPart: AllPartType,
    },
    // 2 A.M Study Session
    {
        title: 'Missing Earth',
        artist: 'hoogway',
        file: '/audio/lofigirl/2 AM Study Session/01 hoogway - Missing Earth (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'You',
        artist: 'hoogway',
        file: '/audio/lofigirl/2 AM Study Session/02 Cocabona - You (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Ruby',
        artist: 'Cocabona',
        file: '/audio/lofigirl/2 AM Study Session/03 Sleepermane x Sebastian Kamae - Ruby (Kupla Master) (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Ships',
        artist: 'Sleepermane x Sebastian Kamae',
        file: '/audio/lofigirl/2 AM Study Session/04 Elior x eaup - Ships (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'VHS',
        artist: 'Elior x eaup',
        file: '/audio/lofigirl/2 AM Study Session/05 Spencer Hunt x WYS - VHS (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Pale Moon',
        artist: 'Spencer Hunt x WYS',
        file: '/audio/lofigirl/2 AM Study Session/06 Dr. Dundiff x Allem Iversom - Pale Moon (Kupla Master2) (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Puddles',
        artist: 'E I S U',
        file: '/audio/lofigirl/2 AM Study Session/07 E I S U - Puddles (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Honey & Lemon',
        artist: 'Lilac',
        file: '/audio/lofigirl/2 AM Study Session/08 lilac - Honey _ Lemon (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Nautilus',
        artist: 'WYS',
        file: '/audio/lofigirl/2 AM Study Session/09 WYS - Nautilus (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Compassion',
        artist: 'Steezy Prime x Spencer Hunt',
        file: '/audio/lofigirl/2 AM Study Session/10 Steezy Prime x Spencer Hunt - Compassion (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'It\'s Going to Be a Good Day',
        artist: 'ocha',
        file: '/audio/lofigirl/2 AM Study Session/11 ocha - It_s Going to Be a Good Day (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Midnight Snack',
        artist: 'Purrple Cat',
        file: '/audio/lofigirl/2 AM Study Session/12 purrple cat - Midnight Snack (Kupl Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Infused',
        artist: 'Yasper x Glimlip',
        file: '/audio/lofigirl/2 AM Study Session/13 Yasper x Glimlip - Infused (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Torii',
        artist: 'Fatb',
        file: '/audio/lofigirl/2 AM Study Session/14 Fatb - Torii (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'February',
        artist: 'Jay-Lounge',
        file: '/audio/lofigirl/2 AM Study Session/15 Jay-Lounge - February (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'See u Soon',
        artist: 'Tzelun',
        file: '/audio/lofigirl/2 AM Study Session/16 Tzelun - See u Soon (Song for Dad) (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Night Owls',
        artist: 'Casiio x Sleepermane',
        file: '/audio/lofigirl/2 AM Study Session/17 Casioo x Sleepermane - Night Owls (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'waking up slowly',
        artist: 'No Spirit x SAINT WKND',
        file: '/audio/lofigirl/2 AM Study Session/18 No Spirit x SAINT WKND - waking up slowly (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Stars and Chimneys',
        artist: 'Kalaido',
        file: '/audio/lofigirl/2 AM Study Session/19 Kalaido - Stars and Chimneys (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'in passing',
        artist: 'stream error',
        file: '/audio/lofigirl/2 AM Study Session/20 stream error - in passing (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Breath',
        artist: 'H.1',
        file: '/audio/lofigirl/2 AM Study Session/21 H.1 - Breath (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Inspect',
        artist: 'Nothingtosay',
        file: '/audio/lofigirl/2 AM Study Session/22 Nothingtosay - Inspect (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sweet Look',
        artist: 'jhove x bert',
        file: '/audio/lofigirl/2 AM Study Session/23 jhove x bert - Sweet Look (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Drowsy',
        artist: 'brillion.',
        file: '/audio/lofigirl/2 AM Study Session/24 brillion. - Drowsy (Kupla Master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Ghost in my Mind',
        artist: 'j\'san x epektase',
        file: '/audio/lofigirl/2 AM Study Session/25 j_san x epektase - Ghost in my Mind (Kupla Master).mp3',
        forPart: AllPartType,
    },
    // Vondelpark
    {
        title: 'Vondelpark',
        artist: 'Sebastian Kamae x Aylior',
        file: '/audio/lofigirl/Vondelpark/1 Vondelpark (MASTERED).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Q&A',
        artist: 'Sebastian Kamae x Aylior',
        file: '/audio/lofigirl/Vondelpark/2 Q_A (MASTERED).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Mr Catchy',
        artist: 'Sebastian Kamae x Aylior',
        file: '/audio/lofigirl/Vondelpark/3 Mr Catchy (MASTERED).mp3',
        forPart: AllPartType,
    },
    {
        title: 'dontyouknow',
        artist: 'Sebastian Kamae x Aylior',
        file: '/audio/lofigirl/Vondelpark/4 dontyouknow (MASTERED).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Outlet',
        artist: 'Sebastian Kamae x Aylior',
        file: '/audio/lofigirl/Vondelpark/5 Outlet (MASTERED).mp3',
        forPart: AllPartType,
    },
    // Sweet Dreams
    {
        title: 'Black Cherry',
        artist: 'Purrple Cat',
        file: '/audio/lofigirl/Sweet Dreams/1 - Black Cherry.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Caramellow',
        artist: 'Purrple Cat',
        file: '/audio/lofigirl/Sweet Dreams/2 - Caramellow.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Late Night Latte',
        artist: 'Purrple Cat',
        file: '/audio/lofigirl/Sweet Dreams/3 - Late Night Latte.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sundae Sunset',
        artist: 'Purrple Cat',
        file: '/audio/lofigirl/Sweet Dreams/4 - Sundae Sunset.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Dark Chocolate',
        artist: 'Purrple Cat',
        file: '/audio/lofigirl/Sweet Dreams/5 - Dark Chocolate.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sugar Coat',
        artist: 'Purrple Cat',
        file: '/audio/lofigirl/Sweet Dreams/6 - Sugar Coat.mp3',
        forPart: AllPartType,
    },
    // Future feelings
    {
        title: 'Pure Bliss',
        artist: 'SPEECHLESS',
        file: '/audio/lofigirl/Future feelings/1 - pure_bliss_FINAL_.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Late Night Adventure',
        artist: 'SPEECHLESS',
        file: '/audio/lofigirl/Future feelings/2 - late_night_adventure_FINAL.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Nightfall',
        artist: 'SPEECHLESS',
        file: '/audio/lofigirl/Future feelings/3 - nightfall_FINAL.mp3',
        forPart: AllPartType,
    },
    {
        title: 'After Thought',
        artist: 'SPEECHLESS',
        file: '/audio/lofigirl/Future feelings/4 - after_thought_FINAL.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sleep Well',
        artist: 'SPEECHLESS',
        file: '/audio/lofigirl/Future feelings/5 - sleep_well (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Alone Forever',
        artist: 'SPEECHLESS',
        file: '/audio/lofigirl/Future feelings/5 - alone_forever_FINAL.mp3',
        forPart: AllPartType,
    },
    // Calm Lands
    {
        title: 'Chrono',
        artist: 'Monma',
        file: '/audio/lofigirl/Calm Lands/1 - Chrono.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Meet You In The Park',
        artist: 'Monma',
        file: '/audio/lofigirl/Calm Lands/2 - Meet You In The Park.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sequences',
        artist: 'Monma',
        file: '/audio/lofigirl/Calm Lands/3 - Sequences.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Snaps',
        artist: 'Monma',
        file: '/audio/lofigirl/Calm Lands/4 - Snaps.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Winter Days',
        artist: 'Monma',
        file: '/audio/lofigirl/Calm Lands/5 - Winter Days.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Waking Up',
        artist: 'Monma',
        file: '/audio/lofigirl/Calm Lands/6 - Waking Up.mp3',
        forPart: AllPartType,
    },
    // Tomorrows that follow
    {
        title: 'Mariana',
        artist: 'ENRA x Sleepermane',
        file: '/audio/lofigirl/Tomorrows That Follow/1 - ENRA _ Sleepermane - Mariana_master.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Now & Then',
        artist: 'ENRA x Sleepermane',
        file: '/audio/lofigirl/Tomorrows That Follow/2 - ENRA _ Sleepermane - Now _ Then_master.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Aislin',
        artist: 'ENRA x Sleepermane',
        file: '/audio/lofigirl/Tomorrows That Follow/3 - ENRA _ Sleepermane - Aislin_master.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Mirror Image',
        artist: 'ENRA x Sleepermane',
        file: '/audio/lofigirl/Tomorrows That Follow/4 - ENRA _ Sleepermane - Mirror Image_master.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Shifting',
        artist: 'ENRA x Sleepermane',
        file: '/audio/lofigirl/Tomorrows That Follow/5 - ENRA _ Sleepermane - Shifting_master.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Soft Spoken',
        artist: 'ENRA x Sleepermane',
        file: '/audio/lofigirl/Tomorrows That Follow/6 - ENRA _ Sleepermane - Soft-Spoken_master.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Tomorrows That Follow',
        artist: 'ENRA x Sleepermane',
        file: '/audio/lofigirl/Tomorrows That Follow/7 - ENRA _ Sleepermane - Tomorrows That Follow_master.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Reminders',
        artist: 'ENRA x Sleepermane',
        file: '/audio/lofigirl/Tomorrows That Follow/8 - ENRA _ Sleepermane - Reminders_master.mp3',
        forPart: AllPartType,
    },
    // Relief
    {
        title: 'SnowFlakes',
        artist: 'Pandrezz',
        file: '/audio/lofigirl/Relief/01 - SnowFlakes.mp3',
        forPart: AllPartType,
    },
    {
        title: 'When She Cries',
        artist: 'Pandrezz',
        file: '/audio/lofigirl/Relief/02 - When She Cries (2.0).mp3',
        forPart: AllPartType,
    },
    {
        title: 'When She Sleeps',
        artist: 'Pandrezz',
        file: '/audio/lofigirl/Relief/03 - When She Sleeps.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Deep Down',
        artist: 'Pandrezz',
        file: '/audio/lofigirl/Relief/04 - Deep Down.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Crystal Lake',
        artist: 'Pandrezz x Epektase',
        file: '/audio/lofigirl/Relief/05 - Crystal Lake (feat. Epektase).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Just Hold On',
        artist: 'Pandrezz',
        file: '/audio/lofigirl/Relief/06 - Just Hold On.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Last Minute',
        artist: 'Pandrezz',
        file: '/audio/lofigirl/Relief/07 - Last Minute.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Deserved Rest',
        artist: 'Pandrezz',
        file: '/audio/lofigirl/Relief/08 - Deserved Rest.mp3',
        forPart: AllPartType,
    },
    // Before You Go
    {
        title: 'escape',
        artist: 'jhove x Kokoro',
        file: '/audio/lofigirl/Before You Go/1 - jhove - escape ft kokoro 01.mp3',
        forPart: AllPartType,
    },
    {
        title: 'we\'ll be fine, i promise',
        artist: 'jhove',
        file: '/audio/lofigirl/Before You Go/2 - jhove - we_ll be fine, i promise.mp3',
        forPart: AllPartType,
    },
    {
        title: 'what if it all turned out fine',
        artist: 'jhove',
        file: '/audio/lofigirl/Before You Go/3 - what if it all turned out fine (2).mp3',
        forPart: AllPartType,
    },
    {
        title: 'been a while',
        artist: 'jhove',
        file: '/audio/lofigirl/Before You Go/4 - jhove - been a while (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'reminiscing',
        artist: 'jhove x Flovry',
        file: '/audio/lofigirl/Before You Go/5 - jhove - reminiscing ft flovry (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'back when',
        artist: 'jhove x tender spring',
        file: '/audio/lofigirl/Before You Go/6 - jhove tender spring - back when.mp3',
        forPart: AllPartType,
    },
    {
        title: 'away from home',
        artist: 'jhove x Bert',
        file: '/audio/lofigirl/Before You Go/7 - away from home ft bert.mp3',
        forPart: AllPartType,
    },
    {
        title: 'if only you knew',
        artist: 'jhove',
        file: '/audio/lofigirl/Before You Go/8 - if only you knew.mp3',
        forPart: AllPartType,
    },
    {
        title: 'before you go',
        artist: 'jhove',
        file: '/audio/lofigirl/Before You Go/9 - before you go - jhove.mp3',
        forPart: AllPartType,
    },
    // A way of existing
    {
        title: 'Pendulum',
        artist: 'Kanisan x no one\'s perfect',
        file: '/audio/lofigirl/A way of existing/1 - Pendulum.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Lost',
        artist: 'Kanisan x no one\'s perfect',
        file: '/audio/lofigirl/A way of existing/2 - Lost.mp3',
        forPart: AllPartType,
    },
    {
        title: 'A Meditation',
        artist: 'Kanisan x no one\'s perfect',
        file: '/audio/lofigirl/A way of existing/3 - A Meditation.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Gentle Wind',
        artist: 'Kanisan x no one\'s perfect',
        file: '/audio/lofigirl/A way of existing/4 - Gentle Wind.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Nothingness',
        artist: 'Kanisan x no one\'s perfect',
        file: '/audio/lofigirl/A way of existing/5 - Nothingness.mp3',
        forPart: AllPartType,
    },
    // Bedtime Stories Pt. 2
    {
        title: 'When The Sun Goes Down',
        artist: 'brillion.',
        file: '/audio/lofigirl/bedtime stories pt 2/1 - When The Sun Goes Down (3).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Reflection',
        artist: 'brillion.',
        file: '/audio/lofigirl/bedtime stories pt 2/2 - Reflection.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Nightlight',
        artist: 'brillion. x Nolfo',
        file: '/audio/lofigirl/bedtime stories pt 2/3 - brillion. x Nolfo - Nightlight.mp3',
        forPart: AllPartType,
    },
    {
        title: 'REM',
        artist: 'brillion. x Strehlow',
        file: '/audio/lofigirl/bedtime stories pt 2/4 - brillion. x Strehlow - REM.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Cradle',
        artist: 'brillion. x HM Surf',
        file: '/audio/lofigirl/bedtime stories pt 2/5 - brillion. x HM Surf - Cradle (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Eventide',
        artist: 'brillion. x No Spirit x Sitting Duck',
        file: '/audio/lofigirl/bedtime stories pt 2/6 - brillion. x No Spirit x Sitting Duck - Eventide (2).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Just Close Your Eyes',
        artist: 'brillion. x Lucid Green',
        file: '/audio/lofigirl/bedtime stories pt 2/7 - brillion. x Lucid Green - Just Close Your Eyes.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Through The Cloud',
        artist: 'brillion. x Tender Spring',
        file: '/audio/lofigirl/bedtime stories pt 2/8 - brillion. x Tender Spring - Through The Clouds.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Dreamscape',
        artist: 'brillion. x Kupla x Arbour',
        file: '/audio/lofigirl/bedtime stories pt 2/9 - brillion. x Kupla x Arbour - Dreamscape.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Journey',
        artist: 'brillion. x TyLuv',
        file: '/audio/lofigirl/bedtime stories pt 2/10 - brillion. x TyLuv - Journey (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Moon Theme',
        artist: 'brillion. x chief.',
        file: '/audio/lofigirl/bedtime stories pt 2/11 - brillion. x chief. - Moon Theme (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'In Clover',
        artist: 'brillion. x HM Surf',
        file: '/audio/lofigirl/bedtime stories pt 2/12 - brillion. x HM Surf - In Clover (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Memories',
        artist: 'brillion. x Chris Mazuera',
        file: '/audio/lofigirl/bedtime stories pt 2/13 - brilllion. x Chris Mazuera - Memories.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Still Dreaming',
        artist: 'brillion. x Pointy Features',
        file: '/audio/lofigirl/bedtime stories pt 2/14 - brillion. x Pointy Features - Still Dreaming (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Aurora',
        artist: 'brillion. x Monma x Tom Doolie',
        file: '/audio/lofigirl/bedtime stories pt 2/15 - brillion. x Monma x Tom Doolie - Aurora (1).mp3',
        forPart: AllPartType,
    },
    // Odyssey
    {
        title: 'Bunnies',
        artist: 'dontcry x nokiaa',
        file: '/audio/lofigirl/Odyssey/1. Dontcry _ Nokiaa - Bunnies.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Tides',
        artist: 'dontcry x nokiaa',
        file: '/audio/lofigirl/Odyssey/2. Dontcry _ Nokiaa - Tides.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Since',
        artist: 'dontcry x nokiaa',
        file: '/audio/lofigirl/Odyssey/3. Dontcry _ Nokiaa - Since.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Decay',
        artist: 'dontcry x nokiaa',
        file: '/audio/lofigirl/Odyssey/4. Dontcry _ Nokiaa - Decay.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Cosmos',
        artist: 'dontcry x nokiaa',
        file: '/audio/lofigirl/Odyssey/5. Dontcry _ Nokiaa - Cosmos.mp3',
        forPart: AllPartType,
    },
    // Sound Asleep
    {
        title: 'Dreamscape',
        artist: 'Spencer Hunt',
        file: '/audio/lofigirl/Sound asleep/1 dreamscape.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Lonely',
        artist: 'Spencer Hunt',
        file: '/audio/lofigirl/Sound asleep/2 lonely.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Thoughts Of You',
        artist: 'Spencer Hunt',
        file: '/audio/lofigirl/Sound asleep/3 thoughts of you.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Moonlight',
        artist: 'Spencer Hunt',
        file: '/audio/lofigirl/Sound asleep/4 moonlight.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sleepy',
        artist: 'Spencer Hunt',
        file: '/audio/lofigirl/Sound asleep/5 sleepy.mp3',
        forPart: AllPartType,
    },
    {
        title: 'I Love You, Goodnight',
        artist: 'Spencer Hunt',
        file: '/audio/lofigirl/Sound asleep/6 i love you, goodnight.mp3',
        forPart: AllPartType,
    },
    // Terrapin
    {
        title: 'À l\'aube',
        artist: 'Mondo Loops x kanisan',
        file: '/audio/lofigirl/Terrapin/1 - à-l_aube (With Kanisan) 2.0 MASTER.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Drive To Midnight',
        artist: 'Mondo Loops',
        file: '/audio/lofigirl/Terrapin/2 - Drive to midnight (Fade).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Starside Groove',
        artist: 'Mondo Loops',
        file: '/audio/lofigirl/Terrapin/3 - Starside Groove (Master version) (1).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Tea House',
        artist: 'Mondo Loops x kanisan',
        file: '/audio/lofigirl/Terrapin/4 - tea-house-(With Kanisan).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Terraping',
        artist: 'Mondo Loops',
        file: '/audio/lofigirl/Terrapin/5 - Terrapin 72 bpm track flatt.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Waves Calling',
        artist: 'Mondo Loops x kanisan',
        file: '/audio/lofigirl/Terrapin/6 - Waves Calling (With Kanisan).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Winter Shells',
        artist: 'Mondo Loops x kanisan',
        file: '/audio/lofigirl/Terrapin/7 - winter-shells(With Kanisan).mp3',
        forPart: AllPartType,
    },
    // Hush
    {
        title: 'Insomnia',
        artist: 'Team Astro',
        file: '/audio/lofigirl/Hush/1. Team Astro - Insomnia.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Liquid Luck',
        artist: 'Team Astro',
        file: '/audio/lofigirl/Hush/2. Team Astro - Liquid Luck.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Helpless',
        artist: 'Team Astro',
        file: '/audio/lofigirl/Hush/3. Team Astro - Helpless.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Lullaby',
        artist: 'Team Astro',
        file: '/audio/lofigirl/Hush/4. Team Astro - Lullaby.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Empty Shelves',
        artist: 'Team Astro',
        file: '/audio/lofigirl/Hush/5. Team Astro - Empty Shelves.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Love Lockdown',
        artist: 'Team Astro',
        file: '/audio/lofigirl/Hush/6. Team Astro - Love Lockdown.mp3',
        forPart: AllPartType,
    },
    // conscious ego
    {
        title: 'Cianite',
        artist: 'Fatb x Flitz&Suppe',
        file: '/audio/lofigirl/conscious ego/1 Fatb - Cianite feat Flitz_Suppe short intro.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Unravel',
        artist: 'Fatb x dryhope',
        file: '/audio/lofigirl/conscious ego/2 Fatb - Unravel feat dryhope.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Dittany',
        artist: 'Fatb x Flitz&Suppe',
        file: '/audio/lofigirl/conscious ego/3 Fatb - Dittany feat Flitz_Suppe.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Lost Thoughts',
        artist: 'Fatb x ZENDR',
        file: '/audio/lofigirl/conscious ego/4 Fatb - Lost Thoughts feat ZENDR.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Bloom',
        artist: 'Fatb x Tesk',
        file: '/audio/lofigirl/conscious ego/5 Fatb - Bloom feat Tesk.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Settembre',
        artist: 'Fatb',
        file: '/audio/lofigirl/conscious ego/6 Fatb - Settembre.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Aurora Boreale',
        artist: 'Fatb x mell-ø',
        file: '/audio/lofigirl/conscious ego/7 Fatb - Aurora Boreale feat mell-ø.mp3',
        forPart: AllPartType,
    },
    // Tranquility
    {
        title: 'Lush',
        artist: 'G Mills',
        file: '/audio/lofigirl/Tranquility/01_Lush.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Icicles',
        artist: 'G Mills x Chris Mazuera x tender spring',
        file: '/audio/lofigirl/Tranquility/02_Icicles (ft. Chris Mazuera _ tender spring).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Crimson',
        artist: 'G Mills',
        file: '/audio/lofigirl/Tranquility/03_Crimson.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sunshower',
        artist: 'G Mills',
        file: '/audio/lofigirl/Tranquility/04_Sunshower.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sublimation',
        artist: 'G Mills x Arbour',
        file: '/audio/lofigirl/Tranquility/05_Sublimation (ft. Arbour).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Rest Your Head',
        artist: 'G Mills',
        file: '/audio/lofigirl/Tranquility/06_Rest Your Head.mp3',
        forPart: AllPartType,
    },
    // Way of Life
    {
        title: 'Waking Up',
        artist: 'Yasumu',
        file: '/audio/lofigirl/Way of Life/01 Yasumu - Waking up.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Questions',
        artist: 'Yasumu',
        file: '/audio/lofigirl/Way of Life/02 Yasumu - Questions.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Thunderstorm',
        artist: 'Yasumu',
        file: '/audio/lofigirl/Way of Life/03 Yasumu - Thunderstorm.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Leafy Breeze',
        artist: 'Yasumu',
        file: '/audio/lofigirl/Way of Life/04 Yasumu - Leafy Breeze.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Evening Jam',
        artist: 'Yasumu',
        file: '/audio/lofigirl/Way of Life/05 Yasumu - Evening Jam (new master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Midnight Thoughts',
        artist: 'Yasumu',
        file: '/audio/lofigirl/Way of Life/06 Yasumu - Midnight thoughts.mp3',
        forPart: AllPartType,
    },
    // Discovery
    {
        title: 'Introvert',
        artist: 'Blue Wednesday',
        file: '/audio/lofigirl/Discovery/01 Blue Wednesday - Introvert.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Driftwood',
        artist: 'Blue Wednesday x Middle School x Tender Spring',
        file: '/audio/lofigirl/Discovery/02 Blue Wednesday x Middle School x Tender Spring - Driftwood.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Japanese Garden',
        artist: 'Blue Wednesday',
        file: '/audio/lofigirl/Discovery/03 Blue Wednesday - Japanese Garden.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Cascadia',
        artist: 'Blue Wednesday x Dillan Witherow',
        file: '/audio/lofigirl/Discovery/04 Blue Wednesday x Dillan Witherow - Cascadia.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Youth',
        artist: 'Blue Wednesday',
        file: '/audio/lofigirl/Discovery/05 Blue Wednesday - Youth.mp3',
        forPart: AllPartType,
    },
    // Dozing
    {
        title: 'Obscurity',
        artist: 'Chris Mazuera',
        file: '/audio/lofigirl/Dozing/1. Obscurity.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Distant',
        artist: 'Chris Mazuera',
        file: '/audio/lofigirl/Dozing/2. Distant.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Counting',
        artist: 'Chris Mazuera x G Mills',
        file: '/audio/lofigirl/Dozing/3. Counting w_ G Mills.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Day 12',
        artist: 'Chris Mazuera',
        file: '/audio/lofigirl/Dozing/4. Day 12.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Dozing',
        artist: 'Chris Mazuera',
        file: '/audio/lofigirl/Dozing/5. Dozing.mp3',
        forPart: AllPartType,
    },
    // Naoko
    {
        title: 'Darjeeling',
        artist: 'Tom Doolie',
        file: '/audio/lofigirl/Naoko/01 Darjeeling.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Rain on Sunday',
        artist: 'Tom Doolie',
        file: '/audio/lofigirl/Naoko/02 Rain on Sunday.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Rackets',
        artist: 'Tom Doolie x Rudy Raw',
        file: '/audio/lofigirl/Naoko/03 Rackets w Rudy Raw.mp3',
        forPart: AllPartType,
    },
    {
        title: 'New Fields',
        artist: 'Tom Doolie x Saib',
        file: '/audio/lofigirl/Naoko/04 New fields w Saib.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Circuit',
        artist: 'Tom Doolie',
        file: '/audio/lofigirl/Naoko/05 Circuit.mp3',
        forPart: AllPartType,
    },
    {
        title: 'To The Sea And Back',
        artist: 'Tom Doolie',
        file: '/audio/lofigirl/Naoko/06 To The Sea And Back.mp3',
        forPart: AllPartType,
    },
    // Sometimes I Wait For You
    {
        title: 'Autumn Morning',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/1 - autumn morning.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Before The Rain Comes',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/2 - before the rain comes.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Breathing In October',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/3 - Breathing in October.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Ease Out',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/4 - Ease out wav.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Saturday',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/5 - saturday.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Snug',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/6 - snug.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Warm Inside',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/7 - warm inside.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Small Things',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/8 - Small Things.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Stray',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/9 - Stray.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Felt The Same',
        artist: 'Softy',
        file: '/audio/lofigirl/Sometimes I Wait For You/10 - felt the same.mp3',
        forPart: AllPartType,
    },
    // Forever Ago
    {
        title: 'Same Ocean',
        artist: 'Hoogway',
        file: '/audio/lofigirl/Forever Ago/1 Hoogway - Same Ocean.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Carousel',
        artist: 'Hoogway',
        file: '/audio/lofigirl/Forever Ago/2 Hoogway - Carousel.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Forever Ago',
        artist: 'Hoogway',
        file: '/audio/lofigirl/Forever Ago/3 Hoogway - Forever Ago.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Different Waves',
        artist: 'Hoogway x Nohone x Tohaj',
        file: '/audio/lofigirl/Forever Ago/4 Hoogway - Different Waves x Nohone, Tohaj.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Sail Away',
        artist: 'Hoogway',
        file: '/audio/lofigirl/Forever Ago/5 Hoogway - Sail Away.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Skyline',
        artist: 'Hoogway x DLJ',
        file: '/audio/lofigirl/Forever Ago/6 Hoogway - Skyline x DLJ.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Love Letter',
        artist: 'Hoogway',
        file: '/audio/lofigirl/Forever Ago/7 - hoogway - Love letter (master).mp3',
        forPart: AllPartType,
    },
    {
        title: 'Everything (You Are)',
        artist: 'Hoogway',
        file: '/audio/lofigirl/Forever Ago/8 Hoogway - Everything (You Are).mp3',
        forPart: AllPartType,
    },
    // Dove
    {
        title: 'Innocent',
        artist: 'Oatmello x Dayn x Epektase',
        file: '/audio/lofigirl/Dove/01 Innocent w_ Dayn and Epektase.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Tranquility',
        artist: 'Oatmello x fantom power',
        file: '/audio/lofigirl/Dove/02 Tranquility w_ Fantom Power.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Hushed Tones',
        artist: 'Oatmello',
        file: '/audio/lofigirl/Dove/03 Hushed Tones.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Essenced',
        artist: 'Oatmello',
        file: '/audio/lofigirl/Dove/04 Essenced.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Minimal',
        artist: 'Oatmello',
        file: '/audio/lofigirl/Dove/05 Minimal.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Good Night',
        artist: 'Oatmello x Late Era',
        file: '/audio/lofigirl/Dove/06 Good Night w_ Late Era.mp3',
        forPart: AllPartType,
    },
    // "Yōkai",
    {
        title: "Tanuki",
        artist: "Flitz&Suppe x Mr. Käfer",
        file: "public/audio/lofigirl/Yokai/01_Tanuki_Master_v2.mp3",
        forPart: AllPartType
    },
    {
        title: "Kodamas Path",
        artist: "Flitz&Suppe x Mr. Käfer",
        file: "public/audio/lofigirl/Yokai/2_Kodamas Path_Master_v1.mp3",
        forPart: AllPartType
    },
    {
        title: "Hidden Onsen",
        artist: "Flitz&Suppe x Mr. Käfer",
        file: "public/audio/lofigirl/Yokai/03_Hidden Onsen_Master_v2.mp3",
        forPart: AllPartType
    },
    {
        title: "Team Steam",
        artist: "Flitz&Suppe x Mr. Käfer ft. Scayos",
        file: "public/audio/lofigirl/Yokai/4_Tea Steam(ft Scayos)_Master_v1.mp3",
        forPart: AllPartType
    },
    {
        title: "Mujinas Ramen Shop",
        artist: "Flitz&Suppe x Mr. Käfer",
        file: "public/audio/lofigirl/Yokai/05_Mujinas Ramen_Shop_Master_v2.mp3",
        forPart: AllPartType
    },
    {
        title: "Sea of Trees",
        artist: "Flitz&Suppe x Mr. Käfer ft. Kupla",
        file: "public/audio/lofigirl/Yokai/6_Sea of Trees(ft. Kupla)_Master_v1.mp3",
        forPart: AllPartType
    },
    {
        title: "Ato",
        artist: "Flitz&Suppe x Mr. Käfer ft. Kupla",
        file: "public/audio/lofigirl/Yokai/7_Ato(ft. Kupla)_Master_v1.mp3",
        forPart: AllPartType
    },
    // "Summer Nights",
    {
        title: "A Walk In The Park",
        artist: "Laffey",
        file: "public/audio/lofigirl/Summer Nights/1 Laffey - A Walk In The Park (Master V2).mp3",
        forPart: AllPartType
    },
    {
        title: "Nighttime",
        artist: "Laffey",
        file: "public/audio/lofigirl/Summer Nights/2 Laffey - Nighttime (Master V2).mp3",
        forPart: AllPartType
    },
    {
        title: "Crush",
        artist: "Laffey",
        file: "public/audio/lofigirl/Summer Nights/3 Laffey - Crush (Master V2).mp3",
        forPart: AllPartType
    },
    {
        title: "Umbrella",
        artist: "Laffey",
        file: "public/audio/lofigirl/Summer Nights/4 Laffey - Umbrella (Master V2).mp3",
        forPart: AllPartType
    },
    {
        title: "Gloomy",
        artist: "Laffey",
        file: "public/audio/lofigirl/Summer Nights/5 Laffey - Gloomy (Master V2).mp3",
        forPart: AllPartType
    },
    {
        title: "Moonlight",
        artist: "Laffey",
        file: "public/audio/lofigirl/Summer Nights/6 Laffey - Moonlight (Master V2).mp3",
        forPart: AllPartType
    },
    {
        title: "Streetlights",
        artist: "Laffey",
        file: "public/audio/lofigirl/Summer Nights/7 Laffey - Streetlights (Master V2).mp3",
        forPart: AllPartType
    },
    {
        title: "Campfire",
        artist: "Laffey",
        file: "public/audio/lofigirl/Summer Nights/8 Laffey - Campfire (Master V2).mp3",
        forPart: AllPartType
    },
    // "Introspective",
    {
        title: "Cleared Sky",
        artist: "Nothingtosay",
        file: "public/audio/lofigirl/Introspective/1 - cleared sky.mp3",
        forPart: AllPartType
    },
    {
        title: "Blue Afternoon",
        artist: "Nothingtosay x midnight alpha.",
        file: "public/audio/lofigirl/Introspective/2 - Blue Afternoon (w midnight alpha).mp3",
        forPart: AllPartType
    },
    {
        title: "I'll Follow You",
        artist: "Nothingtosay",
        file: "public/audio/lofigirl/Introspective/3 - i_ll follow you.mp3",
        forPart: AllPartType
    },
    {
        title: "I Can't Sleep",
        artist: "Nothingtosay",
        file: "public/audio/lofigirl/Introspective/4 - i can_t sleep.mp3",
        forPart: AllPartType
    },
    {
        title: "Far Away",
        artist: "Nothingtosay x Zeyn",
        file: "public/audio/lofigirl/Introspective/5 - far awat ft zeyn.mp3",
        forPart: AllPartType
    },
    {
        title: "I am not lost",
        artist: "Nothingtosay",
        file: "public/audio/lofigirl/Introspective/6 - i am not lost.mp3",
        forPart: AllPartType
    },
    {
        title: "Lazyness",
        artist: "Nothingtosay",
        file: "public/audio/lofigirl/Introspective/7 - lazyness.mp3",
        forPart: AllPartType
    },
    {
        title: "For You",
        artist: "Nothingtosay x D0d",
        file: "public/audio/lofigirl/Introspective/8 - for you ft D0d.mp3",
        forPart: AllPartType
    },
    // "Lazy Sunday",
    {
        title: "It All",
        artist: "jhove",
        file: "public/audio/lofigirl/Lazy Sunday/1 jhove - it all (Kupla Master) (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Pillows",
        artist: "towerz x Spencer Hunt",
        file: "public/audio/lofigirl/Lazy Sunday/2 towerz x spencer hunt - pillows (Kupla Master) (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Eternal Life",
        artist: "Hoogway",
        file: "public/audio/lofigirl/Lazy Sunday/3 hoogway - Eternal life (Kupla Master) (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Habits",
        artist: "Allem Iversom",
        file: "public/audio/lofigirl/Lazy Sunday/4 Allem Iversom - Habits (Kupla Master (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Teakwood",
        artist: "Aso",
        file: "public/audio/lofigirl/Lazy Sunday/5 Aso - Teakwood (Kupla Master) (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Over The Valley",
        artist: "jhove x trxxshed",
        file: "public/audio/lofigirl/Lazy Sunday/6 jhove x trxxshed - over the valley (Kupla Master) (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Cruisin",
        artist: "Elior x eaup",
        file: "public/audio/lofigirl/Lazy Sunday/7 Elior x eaup - Cruisin_ (Kupla Master) (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Kiptime",
        artist: "brillion. x HM surf",
        file: "public/audio/lofigirl/Lazy Sunday/8 brillion - Kiptime w HM surf (Kupla Master) (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Garnet",
        artist: "Monma x Cocabona",
        file: "public/audio/lofigirl/Lazy Sunday/9 Monma x Cocabona - Garnet (Kupla Master) (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Builind A New Life",
        artist: "Celestial Alignment",
        file: "public/audio/lofigirl/Lazy Sunday/10 Celestial Alignment - building a new life (Kupla Master) (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Diet Cola",
        artist: "tender spring x Middle School",
        file: "public/audio/lofigirl/Lazy Sunday/11 tender spring - diet cola w middle school (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Stray",
        artist: "Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dont Let Go",
        artist: "Blue Wednesday x tender spring",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Remorse",
        artist: "Thaehan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pink Night Sky",
        artist: "Dr Dundiff",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mmmm",
        artist: "G Mills x HM surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nautilus",
        artist: "mell-ø x Phlocalyst",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "3 am",
        artist: "DLJ x TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Soft to Touch",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Vivid Memories",
        artist: "Otaam x Sitting Duck",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Floating Away",
        artist: "Glimlip x Yasper",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nostalgia",
        artist: "Glimlip x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Untold Stories",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Desire",
        artist: "Nospirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Wandering Another world",
        artist: "Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Snowflakes",
        artist: "Eisu x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Quilted Dreams",
        artist: "Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Psilo",
        artist: "lofty x pointy features",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "30. Before",
        artist: "Chiccote's Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "1. Ponds",
        artist: "Elior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Staring Through",
    {
        title: "sun",
        artist: "Kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "off",
        artist: "Kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "cold",
        artist: "Kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "station",
        artist: "Kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "bodies",
        artist: "Kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "sing",
        artist: "Kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "slow",
        artist: "Kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "put",
        artist: "Kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "painting",
        artist: "Kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Silent River",
    {
        title: "Raining",
        artist: "Mila Coolness",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Silent River",
        artist: "Mila Coolness",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Heron",
        artist: "Mila Coolness",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ibis",
        artist: "Mila Coolness",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Balance",
        artist: "Mila Coolness",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Surf",
        artist: "Mila Coolness",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Far Away",
        artist: "Mila Coolness",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Life Forms",
    {
        title: "Eons",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Heroes",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "A Waltz for My Best Friend",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Magic",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Soft to Touch",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Those Were the Days",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mycelium",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Weightless",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Microscopic",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Distant Lands",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Natural Ways",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Purple Vision",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "4. Sylvan",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "5. Last Walk",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "6. Safe Haven",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Florist",
    {
        title: "Violet",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Orchid",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hyacinth",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Catmint",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hydraengea",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Inside Space",
    {
        title: "An Unknown Journey",
        artist: "TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Crystal Land",
        artist: "TABAL x Blumen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Inside Space",
        artist: "TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Menthol Breathing",
        artist: "TABAL x tah.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "No Return",
        artist: "TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Window Seat",
    {
        title: "Counting Sheep",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Blue Moon",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Staying In",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pink Skies",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Rainy Day",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dusk",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dawn",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cloudburst",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moonwake",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Breeze",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Midnight",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Letting Go",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Illusion",
    {
        title: "away",
        artist: "Chiccote's Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "finding",
        artist: "Chiccote's Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "timeless",
        artist: "Chiccote's Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "back",
        artist: "Chiccote's Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "illusion",
        artist: "Chiccote's Beats x Pueblo Vista",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Cocobolo",
    {
        title: "Sugar Haze",
        artist: "HM Surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Icy Waves",
        artist: "HM Surf x Iceboi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mount And Blade",
        artist: "HM Surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Vet",
        artist: "HM Surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mud",
        artist: "HM Surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cloyster",
        artist: "HM Surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "It's Chronic",
        artist: "HM Surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hey Jerry",
        artist: "HM Surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Solitude",
    {
        title: "The Day You Left",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Broken",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Not The Same",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Isolated",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Things Will Work Out",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "It's Okay",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Beginnings",
    {
        title: "Feels Like Home",
        artist: "Iamalex x Dillan Witherow ft. tender spring",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nightwalk",
        artist: "Iamalex x Dillan Witherow ft. Azula",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Above Water",
        artist: "Iamalex x Dillan Witherow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Over The Clouds",
        artist: "Iamalex x Dillan Witherow ft. Azula",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Coral",
        artist: "Iamalex x Dillan Witherow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Falling Forward",
        artist: "Iamalex x Dillan Witherow ft. Chris Mazuera",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lazy Morning",
        artist: "Iamalex x Dillan Witherow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "La Hague",
    {
        title: "Going South",
        artist: "Blumen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "La Hague",
        artist: "Blumen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "On My Way",
        artist: "Blumen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Stay",
        artist: "Blumen x Tah.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "L'Aube",
        artist: "Blumen x mell-ø",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Control the Sky",
        artist: "Blumen x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Memories We Made",
    {
        title: "Memories We Made",
        artist: "No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Trees In The Wind",
        artist: "No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Rain In My Head",
        artist: "No Spirit x Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hope",
        artist: "No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "In The Grass",
        artist: "No Spirit x Flaneur",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Some Alone Time",
        artist: "No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Train Of Thought",
        artist: "No Spirit x Sitting Duck",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "La Rochelle",
        artist: "No Spirit x mell-ø x Sitting Duck",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Havanna",
        artist: "No Spirit x Sitting Duck",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Purple Sky",
        artist: "No Spirit x Swink",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Washed Ashore",
        artist: "No Spirit x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Waiting For The Sun",
        artist: "No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Feel free to imagine",
    {
        title: "Get lost in the mind's ocean",
        artist: "Eugenio Izzi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "I'll wait for you at home, even if it's raining",
        artist: "Eugenio Izzi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Distant, but near realities",
        artist: "Eugenio Izzi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Climb the roof breathe better",
        artist: "Eugenio Izzi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The fairie's city",
        artist: "Eugenio Izzi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "3 A.M Study Session",
    {
        title: "Glowing lights",
        artist: "No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "somewhere else",
        artist: "dryhope",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "sheets",
        artist: "Eisu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Phantasm",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "after sunset",
        artist: "Project AER x WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Luna",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lucid",
        artist: "Sebastian Kamae x Intoku",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "contemplation",
        artist: "epektase x j'san",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Signals",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Feelings",
        artist: "Phlocalyst x Living Room",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Blind Forest",
        artist: "Pandrezz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Show Me How",
        artist: "SwuM x chief.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Numb",
        artist: "Comodo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Weightless",
        artist: "TABAL x mell-ø",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Night Bus",
        artist: "Ambulo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fireflies",
        artist: "Kanisan x Frad",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Timeless",
        artist: "H.1",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Recharge",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "fade away",
        artist: "cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Escape Route",
        artist: "Mondo Loops x Kanisan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "fateful slumber",
        artist: "towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Crescent",
        artist: "brillion.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "persist",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Drifting Far Away",
        artist: "Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Cabin Fever",
    {
        title: "Love Cabin",
        artist: "xander.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Driving Alone",
        artist: "xander.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Don't let her Go",
        artist: "xander.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Glitter",
        artist: "xander. x Carrick",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dreams Come True",
        artist: "xander.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Morning Time",
        artist: "xander.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Missing You",
        artist: "xander.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Kenopsia",
    {
        title: "Gravity",
        artist: "dryhope",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Childhood Home",
        artist: "dryhope",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Quetzal",
        artist: "dryhope",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Higher State",
        artist: "dryhope",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Evoke",
        artist: "dryhope x dontcry",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "No Secrets",
        artist: "dryhope",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Moonglow",
    {
        title: "Warm Meadows",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Balcony Nights",
        artist: "S N U G x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Blankets",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dreams of You",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Snooze",
        artist: "S N U G x Jordy Chandra",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Stargazing",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Missing You",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Night Coffee",
        artist: "S N U G x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "At Ease",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "In Motion",
    {
        title: "Seasons",
        artist: "ENRA x Dr Niar",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "In Motion",
        artist: "ENRA",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Consequences",
        artist: "ENRA",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Silver Lining",
        artist: "ENRA x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Where We Left Off",
        artist: "ENRA",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Virginia",
        artist: "ENRA",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Heading Home",
    {
        title: "Reflections",
        artist: "Ajmw x chief.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Way Back When",
        artist: "Ajmw",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hometown",
        artist: "Ajmw",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Patterns",
        artist: "Ajmw x less.people x C4C x Dwyer",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Clouds",
        artist: "Ajmw",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Wonderland Chapter 1",
    {
        title: "Wonderland",
        artist: "Sitting Duck x Squeeda",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Unknown",
        artist: "Sitting Duck x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ancient Tales",
        artist: "Sitting Duck x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Setting Sails",
        artist: "Sitting Duck x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Be Kind",
        artist: "Sitting Duck x Dillan Witherow  x Azula",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dreamstate",
        artist: "Sitting Duck x Dillan Witherow  x Azula",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Slow Mornings",
        artist: "Sitting Duck x Hoffy Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Forest Whispers",
        artist: "Sitting Duck x Ambulo x Louk",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "The Ridge",
    {
        title: "The Ridge",
        artist: "Allem Iversom",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Whole Again",
        artist: "Allem Iversom",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moon",
        artist: "Allem Iversom",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Wasting",
        artist: "Allem Iversom",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Howufeel",
        artist: "Allem Iversom",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The End",
        artist: "Allem Iversom",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Drifting Away",
    {
        title: "Wishing, Waiting",
        artist: "Hevi x Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "If You Knew",
        artist: "Hevi x Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Runaway",
        artist: "Hevi x Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Uncertainty",
        artist: "Hevi x Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Empathise",
        artist: "Hevi x Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Azure Blue",
    {
        title: "Downt The Port",
        artist: "Miramare x Clément Matra",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Foam",
        artist: "Miramare x Clément Matra",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Marseille",
        artist: "Miramare x Clément Matra",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ocean Drift",
        artist: "Miramare x Clément Matra",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "That Old Beach House",
        artist: "Miramare x Clément Matra",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Autumn in Budapest",
    {
        title: "Aether",
        artist: "BluntOne x Ky akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Zen Fusion",
        artist: "BluntOne x Baen Mow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Natures Cures",
        artist: "BluntOne",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Thrivin'",
        artist: "BluntOne x Fatb",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Misty Dawn",
        artist: "BluntOne",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pocket Full of Hope",
        artist: "BluntOne x Baen Mow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Air Never been So Fresh",
        artist: "BluntOne",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Simplicity",
        artist: "BluntOne",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Danube Blues",
        artist: "BluntOne Baen Mow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Time Remembered",
    {
        title: "neff",
        artist: "chief.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "message in a bottle",
        artist: "chief.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "ashes",
        artist: "chief.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "distance",
        artist: "chief. x Kurt Stewart",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "you used to",
        artist: "chief.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "liquid",
        artist: "chief. x Odyssee",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "downtown",
        artist: "chief. x Joe Nora",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "it happens",
        artist: "chief.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Chance Encounter",
    {
        title: "Rewinding Memories",
        artist: "Refeeld x Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Tell Me Your Name",
        artist: "Refeeld x Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Blue Terrain",
        artist: "Refeeld x Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Your Best Option",
        artist: "Refeeld x Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Maybe We're Still Sleeping",
        artist: "Refeeld x Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Until Forever",
    {
        title: "We Met",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "First Dates",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Day One",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fallen in Love",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Discovering",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Feeling home",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "At The Sea",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Island Walks",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Soulmates",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Thermal Baths",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Different Cities",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pancakes",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Balcony Breakfast",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Chocolate Puddings",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Late Night Talks",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Until Forever",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Underneath",
    {
        title: "Surfaced",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "underneath",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lightning bugs",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Atlantis",
        artist: "Casiio x Sleepermane ft. Sling Dilly",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cascades",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Prisms",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Wishing Well",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Luna",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lagoon",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Wetlands",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Depths",
        artist: "Casiio x Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Riverside",
    {
        title: "Riverside",
        artist: "Slo Loris",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Aftercastle",
        artist: "Slo Loris x Strehlow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Kings of the Indoors",
        artist: "Slo Loris x Tender Spring",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lily Field",
        artist: "Slo Loris",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pier",
        artist: "Slo Loris",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Until Tomorrow",
    {
        title: "Until Tomorrow",
        artist: "Towerz x Fourwalls",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Distant Thoughts",
        artist: "Towerz x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Drifting",
        artist: "Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hearts",
        artist: "Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Chao",
        artist: "Towerz x Chris Mazuera",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Catbug",
        artist: "Towerz x Tender Spring",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Aimless Wander",
        artist: "Towerz x Fourwalls x Farewell",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Skinny Atlas",
        artist: "Towerz x Skinny Atlas",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Zero",
    {
        title: "Zero",
        artist: "Sebastian Kamae x Aylior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Poolside",
        artist: "Sebastian Kamae x Aylior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sunrise",
        artist: "Sebastian Kamae x Aylior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dunes",
        artist: "Sebastian Kamae x Aylior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Snooze",
        artist: "Sebastian Kamae x Aylior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Wake Up",
        artist: "Sebastian Kamae x Aylior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Bedtime Stories Pt. 3",
    {
        title: "Interlude",
        artist: "brillion.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Summit",
        artist: "brillion. x Kurt Stewart",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Forever",
        artist: "brillion. x Khutko",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Drift",
        artist: "brillion. x  chief.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Searching",
        artist: "brillion. x Fatb",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moonglow",
        artist: "brillion. x Hm Surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Melatonin",
        artist: "brillion. x Sleepdealer",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Orbit",
        artist: "brillion. x Lucid Green",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "My Spaceship",
        artist: "brillion.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Floating",
        artist: "brillion. x Jazzinuf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Down To Earth",
        artist: "brillion. x Odyssee",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Discovery",
        artist: "brillion. x NOlfo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Arrival",
        artist: "brillion. x Imagiro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Eternal",
        artist: "brillion. x No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Transcendence",
        artist: "brillion. x Sitting Duck x Hoffy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "A Friendly Warmth",
    {
        title: "It’s alright to feel afraid",
        artist: "Tender Spring x Blurred Figures",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Keep friends close",
        artist: "Tender Spring x Blurred Figures ft. Chris Mazuera",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "A friendly warmth",
        artist: "Tender Spring x Blurred Figures",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Birds of a feather",
        artist: "Tender Spring x Blurred Figures ft. Middle School",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Don’t worry, I’ll always be here",
        artist: "Tender Spring x Blurred Figures",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hugs",
        artist: "Tender Spring x Blurred Figures ft. INKY!",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "The Beauty Around Us",
    {
        title: "Reverie",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moonflower",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hiding Place",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cloud Cover",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lakeview",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Skylark",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Until The Morning Comes",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Overlook",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Constellation",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Daisies",
        artist: "softy x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "The Prophecy",
    {
        title: "Kaya Village",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Daydreaming",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Vision",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Seeker",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sacred Tree",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Prophecy Unfolds",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Lanterns",
    {
        title: "Under A Wishing Sky",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Celestial",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Your Light",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fireworks",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Singing to the Moon",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Forever Changing",
    {
        title: "Forever Changing",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Home",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Constellations",
        artist: "Laffey x Oatmello",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Stillness",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Astral",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Under The Stars",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Looking Back",
        artist: "Laffey x Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "By The Pond",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Midnight",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sunsets",
        artist: "Laffey x Rook1e",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Been Thinking",
    {
        title: "Blue eyes",
        artist: "Jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Far Away",
        artist: "Jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Above Couds",
        artist: "Jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "About Us",
        artist: "Jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "You",
        artist: "Jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "After Hours",
        artist: "Jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Escapade",
    {
        title: "Moon Waltz",
        artist: "Elijah Lee x aimless",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Explorers",
        artist: "Elijah Lee x aimless",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Escape With Me",
        artist: "Elijah Lee x aimless",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Our Hideaway",
        artist: "Elijah Lee x aimless",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Infinite",
        artist: "Elijah Lee x aimless",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Midnight Sky",
        artist: "Elijah Lee x aimless",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Rüya",
    {
        title: "Rüya",
        artist: "Kanisan x WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Left Alone",
        artist: "Kanisan x Nymano",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Kokoro",
        artist: "Kanisan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Couldn't Help",
        artist: "Kanisan x Pandrezz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Withheld Call",
        artist: "Kanisan x Mau",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lights Out",
        artist: "Kanisan x  Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hour Away",
        artist: "Kanisan x Sadtoi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sorrow",
        artist: "Kanisan x pointy features",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Distant Worlds",
    {
        title: "Alien Sky",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Missing You",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Falling Star",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lunar Eclipse",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Too Tired",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Reflection",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cold Pizza",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Finding Myself",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "After the Rain",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Morning Light",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Tabula Rasa",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Angelic",
    {
        title: "Lilac",
        artist: "Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moonwalker",
        artist: "Kainbeats x Kanisan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Steadfast",
        artist: "Kainbeats x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Purity",
        artist: "Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Outcast",
        artist: "Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ephemerality",
        artist: "Kainbeats x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Angelic",
        artist: "Kainbeats x Hevi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pure Warmth",
        artist: "Kainbeats x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Walls",
    {
        title: "Discover",
        artist: "Elior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Walls",
        artist: "Elior x Aylior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lucid Dreams",
        artist: "Elior x DJ Garlik",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Liza",
        artist: "Elior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moving On",
        artist: "Elior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Savour",
        artist: "Elior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "One day it’s over",
    {
        title: "As the world burns",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hoboken",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Healthy distraction",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dowsy",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "You all sound the same",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Starting late",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "28 days",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Capturing the light",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Firefly",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "I wish you love",
        artist: "less.people",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Nostalgia",
    {
        title: "Infinity ...",
        artist: "Mujo x Sweet Medicine ft. Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "SchoolYard",
        artist: "Mujo x Sweet Medicine ft. WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lagoon",
        artist: "Mujo x Sweet Medicine",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Crystals",
        artist: "Mujo x Sweet Medicine ft. Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Escaped",
        artist: "Mujo x Sweet Medicine ft. Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nostalgia",
        artist: "Mujo x Sweet Medicine",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Memory Within A Dream",
    {
        title: "Yerba Mate",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "In The End",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Yuma",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Desireless",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Overthinking",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Memory Within A Dream",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Solar Reset",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Join Me",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "You've Forgotten How",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Constant Motion",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "38 Hz",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Canella",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fragments Of Our Youth",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "It Was Always There",
        artist: "Ky Akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Latibule",
    {
        title: "Windmill City",
        artist: "goosetaf x Dillan Witherow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Silk",
        artist: "goosetaf x Blue Wednesday",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Full of Heart",
        artist: "goosetaf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Tree Sap",
        artist: "goosetaf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Tucked Inside",
        artist: "goosetaf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Afternoon Commute",
        artist: "goosetaf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Siren",
        artist: "goosetaf x brillion.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Somewhere Away",
        artist: "goosetaf x INKY!",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "outer space",
    {
        title: "takeoff",
        artist: "j'san x epektase",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "outer space",
        artist: "j'san x epektase",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "alone in the void",
        artist: "j'san x epektase",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "outer peace | inner demons",
        artist: "j'san x epektase",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "deep dive",
        artist: "j'san x epektase",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "a new world",
        artist: "j'san x epektase",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "on the edge",
        artist: "j'san x epektase",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Cozy Winter ☕",
    {
        title: "Over The Moon",
        artist: "Team  Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "After You",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moonlit Walk",
        artist: "Purpple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fjallstoppur",
        artist: "Enluv x E I S U",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Vulnerable",
        artist: "Squeeda",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "sparkler",
        artist: "Towerz x farewell",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "night lamp",
        artist: "jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "overthinking",
        artist: "cxlt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Soaring",
        artist: "Elior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Rain Come Again",
        artist: "Xander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Drifting",
        artist: "G Mills x aimless",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "San Francisco",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Love's Dissonance",
        artist: "lofty x pointy features x quist",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Tetra",
        artist: "Monma x Cocabona",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "every second",
        artist: "aimless x Soho",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ebs and Flows",
        artist: "Glimlip",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Days Will Pass",
        artist: "TABAL x eaup",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Serene",
        artist: "Ambulo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Inside Out",
        artist: "Sleepermane x Sling Dilly",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dreaming of Snow",
        artist: "Otaam x Squeeda",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Floating",
        artist: "eaup x Elior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Campfire",
        artist: "bert x Nerok",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hammock",
        artist: "Azula x IamAlex x Dillan Witherow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sixth Station",
        artist: "Anbuu x Blue Wednesday",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Heated Blanket",
        artist: "Tysu x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Formless",
        artist: "Kainbeats x S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Counting Stars",
        artist: "Chiccote's Beat x Pueblo Vista",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "moonfall",
        artist: "Towerz x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Waves",
        artist: "Fourwalls",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "a roomful of memories and longing",
        artist: "Celestial Alignment",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Always Drifting",
        artist: "Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "As The Sun Sets",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Staring Contest",
    {
        title: "Nobody There",
        artist: "fourwalls",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Staring Contest",
        artist: "fourwalls",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Firefly",
        artist: "fourwalls x Skinny Atlas",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Skin",
        artist: "fourwalls x nighlight",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Above The Clouds",
        artist: "fourwalls",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Out West",
        artist: "fourwalls x Chris Mazuera",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Waiting At The Lights",
        artist: "fourwalls x tender spring",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Remembering",
        artist: "fourwalls x Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Smile From A Friend",
        artist: "fourwalls x farewell",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "The Pursuit of Simplicity",
    {
        title: "Go Time",
        artist: "C4C",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Skkip Town",
        artist: "C4C",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Vagabond Life",
        artist: "C4C",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "New Homeland",
        artist: "C4C",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Build with Love",
        artist: "C4C x Blue Wednesday",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Light a Fire",
        artist: "C4C",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Enjoy",
        artist: "C4C",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Melodic Nostalgic",
    {
        title: "introspection",
        artist: "tomcbumpz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "saudade",
        artist: "tomcbumpz x Yutaka hirasaka",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "eyes shut, mind open",
        artist: "tomcbumpz x tender spring",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "within & without",
        artist: "tomcbumpz x Paniyolo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "be",
        artist: "tomcbumpz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Temple Garden",
    {
        title: "Lotus",
        artist: "BVG",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Gentle Wind",
        artist: "BVG",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Spring Rain",
        artist: "BVG",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Youth",
        artist: "BVG x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Spirited Away",
        artist: "BVG x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "River Glow",
    {
        title: "Dusk",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Down by the Lake",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fire Flies",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mercury Retrograde",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Astral Hour",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Diamonds",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pillow Beat with Strehlow",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Marmalade Sky",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Crystal Lake",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "River Glow",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Traces of Light",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sliver Of Morning",
        artist: "Tyluv.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Time In Motion",
    {
        title: "Unwritten",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mind Pool",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Garden Flower",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Distant Memory",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "EthereaL",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "No Words",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Light Years Apart",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Grey",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Feels Like Home",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Bluebird",
        artist: "Dontcry x Nokiaa ft. Sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Before It's Late",
    {
        title: "Late",
        artist: "Hevi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "These Are The Nights",
        artist: "Hevi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Imaginary",
        artist: "Hevi x Kurt Stewart x S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "We Had Better Days",
        artist: "Hevi x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lucid",
        artist: "Hevi x Naga",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Beyond the Dreams",
        artist: "Hevi x Stuffed Tomato",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Finally Breathing",
        artist: "Hevi x Redmatic",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mind At Ease",
        artist: "Hevi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Alone",
        artist: "Hevi x INKY!",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Bedtime",
        artist: "Hevi x probablyasleep",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lonely Nights",
        artist: "Hevi x Trxxshed",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Yesterday",
        artist: "Hevi x Trxxshed",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Leave This Town",
        artist: "Hevi x tender spring",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Traveler",
    {
        title: "Travelers",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lucid",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "See You When I See You",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Planet Buddies",
        artist: "Team Astro x cocabona",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Snowflakes",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Beehive",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Countdown to Zero",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nothingness",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Searching...",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pluto",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Stop Calling Me Cute",
        artist: "Team Astro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Blue Woods",
    {
        title: "Finally Breathing",
        artist: "GlobulDub",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Gone Home",
        artist: "GlobulDub",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Night Cat",
        artist: "GlobulDub",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sail",
        artist: "GlobulDub",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sweet Memories",
        artist: "GlobulDub",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Precious Moments",
    {
        title: "Heading Home",
        artist: "Celestial Alignment ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hammock",
        artist: "Celestial Alignment ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Feeling Is Still There",
        artist: "Celestial Alignment x Mecklin",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Let Me Get You A Glass",
        artist: "Celestial Alignment ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Eaves",
        artist: "Celestial Alignment x Payubeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Precious Moments",
        artist: "Celestial Alignment x Glacier Kid",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "It's All Good",
        artist: "Celestial Alignment ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Winter Love",
    {
        title: "Winter Love",
        artist: "Dr. Dundiff",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Snowday",
        artist: "Dr. Dundiff",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Through the Woods",
        artist: "Dr. Dundiff x Ian Ewing",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Whistiling Winds",
        artist: "Dr. Dundiff",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "From My Window",
        artist: "Dr. Dundiff x Cocabona",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Orange Leaves",
        artist: "Dr. Dundiff",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "A Bridge Between",
    {
        title: "A Bridge Between",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Something To Cherish",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Heartsease",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Willows",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Familiar Feeling",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lighted Path",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ripples",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Abundance",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Empty Spaces",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Choices",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Seeking Peace",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Arrowhead",
        artist: "Towerz x hi jude",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Land of Calm",
    {
        title: "Rent a Van",
        artist: "Tom Doolie",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Awake",
        artist: "Tom Doolie",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mavericks",
        artist: "Tom Doolie ft. Hya x Rich Jacques",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Into you",
        artist: "Tom Doolie",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Rehab",
        artist: "Tom Doolie",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sincere",
        artist: "Tom Doolie x DAO",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Relatives",
    {
        title: "Playgrounds",
        artist: "Phlocalyst  ft. mell-ø x Akīn",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Homage",
        artist: "Phlocalyst ft. Sátyr x Akīn",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Bamboo",
        artist: "Phlocalyst ft. Elior x Living Room",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Beautiful Morning",
        artist: "Phlocalyst ft. Living Room x Akīn",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Old Friend",
        artist: "Phlocalyst ft. Living Room x Akīn",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Midnight Gazing",
    {
        title: "Hidden In Dusk",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Elusive",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Star Sailing",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Blue Note",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Evening Porch",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Gilding",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Forrest Lullaby",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dragons Dreams",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Goyo",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Homebound",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Late Night Magic",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "On The Way Home",
        artist: "Mondo Loops x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Nightfall",
    {
        title: "NightFall",
        artist: "S N U G x Nuver",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Frost",
        artist: "S N U G x Nuver",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Alaska",
        artist: "S N U G x Nuver",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Afloat",
        artist: "S N U G x Nuver ft. Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lighthouse",
        artist: "S N U G x Nuver ft. Sitting Duck",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "After Dark",
        artist: "S N U G x Nuver",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moonscapes",
        artist: "S N U G x Nuver",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Horizons",
        artist: "S N U G x Nuver ft. Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Paths",
        artist: "S N U G x Nuver",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dazed",
        artist: "S N U G x Nuver",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nova",
        artist: "S N U G x Nuver ft. Jordy Chandra",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "It's Getting Late",
        artist: "S N U G x Nuver",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Hourglass",
    {
        title: "Hourglass",
        artist: "Thaehan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "No Regrets",
        artist: "Thaehan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "One Last Time",
        artist: "Thaehan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Grains of Sand",
        artist: "Thaehan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ephemeral",
        artist: "Thaehan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Falling dreams",
    {
        title: "a lonely star",
        artist: "jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "before i met you",
        artist: "jhove x elijah lee",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "beyond the stars",
        artist: "jhove x tysu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "closes eyes",
        artist: "jhove x hm surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "times flies",
        artist: "jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "in the morning",
        artist: "jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "i know, goodbye",
        artist: "jhove x amess",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Evermore",
    {
        title: "True Love",
        artist: "WYS x Sweet Medicine",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mourning Dove",
        artist: "WYS x Sweet Medicine",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Practiced Compassion",
        artist: "WYS x Sweet Medicine",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Aftermath",
        artist: "WYS x Sweet Medicine",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Arizona Zero",
        artist: "WYS x Sweet Medicine",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Down the Line",
        artist: "WYS x Sweet Medicine",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Washed Out",
        artist: "WYS x Sweet Medicine",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Feelings",
    {
        title: "Inner Peace",
        artist: "Bcalm x Banks",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Firelight",
        artist: "Bcalm x Banks ft. Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Comfort",
        artist: "Bcalm x Banks ft. Fletcher Reed",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sleep Patterns",
        artist: "Bcalm x Banks ft. Sleep Patterns",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Crystalize",
        artist: "Bcalm x Banks",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Winter Sun",
        artist: "Bcalm x Banks",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Because",
        artist: "Bcalm x Banks",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Toughts",
        artist: "Bcalm x Banks",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "I'll Remember u",
        artist: "Bcalm x Banks",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mary",
        artist: "Bcalm x Banks",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Growth Patterns",
    {
        title: "Growth",
        artist: "Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "A Better Place",
        artist: "Project AER x cxlt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Brighter  Days",
        artist: "Project AER x Refeeld",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Likelife",
        artist: "Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mind Over Matter",
        artist: "Project AER x Fletcher Reed",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Open Eyes",
        artist: "Project AER x WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Downtime",
        artist: "Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Finality of it All",
        artist: "Project AER x Colours in Context",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Polar",
    {
        title: "Noctilucent",
        artist: "Ambulo x Squeeda",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Polar",
        artist: "Ambulo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sun dog",
        artist: "Ambulo x Squeeda",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Resilience",
        artist: "Ambulo x mell-o",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Child",
        artist: "Ambulo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pleasant",
        artist: "Ambulo x Kasper",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Intentions",
        artist: "Ambulo x Kasper",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Tender Memories",
    {
        title: "Reassuring Skies",
        artist: "Lenny Loops x Hoffy Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Tender Memories",
        artist: "Lenny Loops x Hoffy Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Daydreaming",
        artist: "Lenny Loops x Hoffy Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lost in Thought",
        artist: "Lenny Loops x Hoffy Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Almost Asleep",
        artist: "Lenny Loops x Hoffy Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Caring",
        artist: "Lenny Loops x Hoffy Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Last Light",
    {
        title: "Hidden Clouds",
        artist: "TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "A Childish Day",
        artist: "TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Last Light",
        artist: "TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Other Way",
        artist: "TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fireflies",
        artist: "TABAL x  DLJ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Finally Home",
        artist: "TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Motions",
    {
        title: "Wander",
        artist: "Tibeauthetraveler",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ember",
        artist: "Tibeauthetraveler x Eleven",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Northern Lights",
        artist: "Tibeauthetraveler",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Motions",
        artist: "Tibeauthetraveler",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Water Circles",
        artist: "Tibeauthetraveler x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Starry Night",
        artist: "Tibeauthetraveler  x Just Steezy Things",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Out of Breath",
        artist: "Tibeauthetraveler",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Before Sunrise",
    {
        title: "Fig Trees",
        artist: "Dillan Witherow x Wednesday",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Rose Bay",
        artist: "Dillan Witherow x Santpoort",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Before Sunrise",
        artist: "Dillan Witherow x tender spring x azula",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "twopointeight",
        artist: "Dillan Witherow x Blurred Figures",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Inside Out",
        artist: "Dillan Witherow Blue Wednesday",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Field of Stars",
        artist: "Dillan Witherow x Sitting Duck x No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Superlunary",
        artist: "Dillan Witherow x G Mills",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "To You, From Me",
        artist: "Dillan Witherow x tender spring",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hiding In a Flower",
        artist: "Dillan Witherow x No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Opposite Ends",
        artist: "Dillan Witherow x WYS x azula",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "First Snow",
        artist: "Dillan Witherow x Sitting Duck",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "After Hours",
    {
        title: "Long Walk Short Dock",
        artist: "Blue Wednesday x Dillan Witherow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dots",
        artist: "Blue Wednesday",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Wildflower",
        artist: "Blue Wednesday",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Attic",
        artist: "Blue Wednesday x INKY!",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "I See You In Slow Motion",
        artist: "Blue Wednesday",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Homeland",
    {
        title: "Farewell",
        artist: "L'Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Speculations",
        artist: "L'Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Past Things",
        artist: "L'Outlander x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Looking For Answers",
        artist: "L'Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Higher Calling",
        artist: "L'Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Homeland",
        artist: "L'Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "A Day At A Time",
    {
        title: "A Day At A Time",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Infinite",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "In This Moment",
        artist: "Laffey x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "New Beginnings",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Acceptance",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Together",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Exhale",
        artist: "Laffey x Dilan Witherow",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Auburn",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Compassion",
        artist: "Laffey x  Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Comfort",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Journey",
        artist: "Laffey x Sunlight Jr.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "A New Path",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Healing",
        artist: "Laffey",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Distant Images",
    {
        title: "Slow Ride",
        artist: "Softy x Kaspa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hideaway",
        artist: "Softy x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Evergreen",
        artist: "Softy x Lucid Green",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Blue Lining",
        artist: "Softy x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Glimpses",
        artist: "Softy x Lucid Green",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Solstice",
        artist: "Softy x Pointy Features",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Inside All Day",
        artist: "Softy x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Homesick",
        artist: "Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "After All",
        artist: "Softy x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Contrasts",
        artist: "Softy x Kaspa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sundown",
        artist: "Softy x Celestial Alignment",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Close To",
        artist: "Softy x Refeeld",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Retro Colors",
    {
        title: "Ivory",
        artist: "Trxxshed x Jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Synesthesia",
        artist: "Trxxshed x Clangon",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lost In Between",
        artist: "Trxxshed",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Magnitude",
        artist: "Trxxshed x j'san",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Passing Time",
        artist: "Trxxshed x fourwalls",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Reminiscence",
        artist: "Trxxshed",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Altitude",
        artist: "Trxxshed x Lomtre",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Saturated",
        artist: "Trxxshed",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Early Days",
        artist: "Trxxshed x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dysomnia",
        artist: "Trxxshed x Creative Self",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Obscure Sorrows",
        artist: "Trxxshed",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Rituals",
    {
        title: "Rituals",
        artist: "Living Room x M e a d o w",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Buddha",
        artist: "Living Room x Phlocalyst",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Kyoto Sunrise",
        artist: "Living Room x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Circle Of Truste",
        artist: "Living Room x Akīn",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Consciousness",
        artist: "Living Room x ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sublime",
        artist: "Living Room x Otaam",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Blue Hour",
        artist: "Living Room x Phlocalyst",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sleepmodee",
        artist: "Living Room x Rudy Raw",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Wonderland Chapter II",
    {
        title: "Chasing Dreams",
        artist: "Sitting Duck x Khukto",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Save Tonight",
        artist: "Sitting Duck x Khukto",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sweet Honey",
        artist: "Sitting Duck x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hope",
        artist: "Sitting Duck x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lessons We Learned",
        artist: "Sitting Duck x No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Reflection",
        artist: "Sitting Duck x Cloud Break",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Changes",
        artist: "Sitting Duck x Nuver",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Faded",
        artist: "Sitting Duck x Sinnr",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Wilderness",
    {
        title: "dim the lights",
        artist: "Nvmb x Lona Moor",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "wilderness",
        artist: "Nvmb",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "intimate",
        artist: "Nvmb",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "your colors",
        artist: "Nvmb",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "lakeside",
        artist: "Nvmb",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "funny place",
        artist: "Nvmb",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "campfire",
        artist: "Nvmb",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "inaudible",
        artist: "Nvmb x Tom Doolie",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Shine On",
    {
        title: "Coral Caves",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nightcall",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dozy",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Kindred Spirits",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Low Tide",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moon Base",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Five Years",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Seven Seas",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Afloat",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Red Alley",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lights Low",
        artist: "Pointy features x Kanisan x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Oblivion",
    {
        title: "Destination Unknown",
        artist: "amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Levitate",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Leave The World Behind",
        artist: "amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Follow Me",
        artist: "amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lost In Time",
        artist: "amies x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Arriving",
        artist: "amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Memory Lane",
        artist: "amies x midnight alpha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dimensions",
        artist: "amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Silenced",
        artist: "amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "New Beginning",
        artist: "amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Can We Talk",
    {
        title: "Call You Soon",
        artist: "Glimlip x Louk",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "How Have You Been",
        artist: "Glimlip x Louk",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Your Words Not Mine",
        artist: "Glimlip x Louk",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "I'll Meet You At The Station",
        artist: "Glimlip x Louk",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Seeing You Is Like",
        artist: "Glimlip x Louk",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Sons of the Dew",
    {
        title: "We Met in the Forest",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Wisteria Arbour",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Samurai's Dream",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nagano, 5 Am",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Kosame",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Love Under The Roof",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Overgrown",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Kami's Gift",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Botanicals",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Despite the Pain",
        artist: "Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Melody Mountain",
    {
        title: "Lavender",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Orion",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Memory of You",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Tender Souls",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Under the Bridge",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Valley of Hope",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Melody Mountain",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Castles in the Snow",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fairies",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Orchid",
        artist: "Kupla",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Cloud Studies",
    {
        title: "Almost Dreaming",
        artist: "Enluv",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cloud Studies",
        artist: "Enluv x E I S U x tapei",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Woodland Hills",
        artist: "Enluv x Squeeda x No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Balance",
        artist: "Enluv x Sitting Duck x Squeeda",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Contemplate",
        artist: "Enluv",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Within",
        artist: "Enluv",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Winterhome",
        artist: "Enluv x tapei",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Frozen Over",
        artist: "Enluv x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Altitude",
        artist: "Enluv x tysu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Flying Away",
        artist: "Enluv",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Days of Tomorrow",
    {
        title: "Driving North",
        artist: "M e a d o w x Living Room x Phlocalyst",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Empty Streets",
        artist: "M e a d o w x Rudy Raw x Phlocalyst",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Stargazing",
        artist: "M e a d o w x Hoffy Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Time",
        artist: "M e a d o w x Drxnk x Sátyr",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fluid",
        artist: "M e a d o w x Sátyr x Drxnk",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Leaves",
        artist: "M e a d o w x Drxnk x Sátyr",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Evening",
        artist: "M e a d o w x Otaam",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "4 A.M Study Session",
    {
        title: "Snooze Button",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Parallel",
        artist: "Tom Doolie x lōland",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Counting Sheep",
        artist: "jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Bliss",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Above the Clouds",
        artist: "amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sayonara",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Im Fine",
        artist: "Kayou",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Secret Garden",
        artist: "Thaehan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Carry Me",
        artist: "No Spirit",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Alaska",
        artist: "lōland x Nokiaa x Tom Doolie",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Night Walk",
        artist: "l'Outlander x Kanisan",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Drowsy Town",
        artist: "Miramare x Clément Matrat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nemui",
        artist: "lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sunsets",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Voyager",
        artist: "dryhope",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Shimmer",
        artist: "sleepermane",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Night Drive",
        artist: "Ky akasha",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Drowning",
        artist: "Kanisan x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Shimmering Nights",
        artist: "Mondo Loops x Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Everything Went Quiet",
        artist: "cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lullaby",
        artist: "Chiccotes Beats x Pueblo Vista",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Imaginary",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Patience",
        artist: "Arbour",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Midsummer",
        artist: "Sebastian Kamae",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Rainbowsend",
        artist: "Living Room",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Pegan Hill",
    {
        title: "Old Cars",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Brook",
        artist: "Kurt Stewart x Lomme x Yutaka Hirasaka",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cope",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Long Way",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Window Seat",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Daydreams",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Adrift",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nightfall",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moonlight",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Years Ago",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moments",
        artist: "Kurt Stewart x Lomme",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Ghosts of the Floating World",
    {
        title: "Teahouse Spirits",
        artist: "Kalaido",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Streelight Reverie",
        artist: "Kalaido",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Kami",
        artist: "Kalaido",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "One Summer Afternoon",
        artist: "Kalaido ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Power Lines and Pastel Skies",
        artist: "Kalaido x biosphere",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Floating Ghosts",
        artist: "Kalaido",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Maboroshi",
        artist: "Kalaido",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Phantoms and Dreams",
        artist: "Kalaido ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sleepy Town",
        artist: "Kalaido",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Houses on Hills",
        artist: "Kalaido x Kennebec",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Yume",
        artist: "Kalaido",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ending Theme",
        artist: "Kalaido x aqualina",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Night Yokai",
        artist: "Kalaido",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Neon Memories",
        artist: "Kalaido",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Departure",
    {
        title: "Departure",
        artist: "Peak Twilight x S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ascent",
        artist: "Peak Twilight x Aizyc",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lunar Shores",
        artist: "Peak Twilight x no one’s perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Magical Connection",
        artist: "Peak Twilight x Prithvi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Desolation",
        artist: "Peak Twilight x mell-ø",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Picturesque",
        artist: "Peak Twilight x Tibeauthetraveler",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Until We Meet Again",
        artist: "Peak Twilight x amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "love you two",
    {
        title: "My Person",
        artist: "kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Builder Home",
        artist: "kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cool Winds",
        artist: "kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Their Chair",
        artist: "kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Can’t See",
        artist: "kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Turn Ways",
        artist: "kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Mirror of Time",
    {
        title: "Repressed Emotions",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Left Behind",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Searching for Answers",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Brighter Days",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "A New Beginning",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Flowstate",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Petrichor",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "How We Feel",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dreamlands",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mirror of Time",
        artist: "Yasumu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Golden Hour",
    {
        title: "Dreams We Shared",
        artist: "Fourwalls x jhove x Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Still Shining",
        artist: "Fourwalls x jhove x Skinny Atlas",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "La Lune",
        artist: "Fourwalls x jhove x allove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pillow Fight",
        artist: "Fourwalls x jhove x nightlight",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Golden Hour",
        artist: "Fourwalls x jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Somewhere In Time",
    {
        title: "Somewhere In Time",
        artist: "cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Slow Hours",
        artist: "cxlt. x herman.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Above The Quiet City",
        artist: "cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "After The Rain",
        artist: "cxlt. x Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Canvas",
        artist: "cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Reliving",
        artist: "cxlt. x amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "In Between",
        artist: "cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Glowing Light",
        artist: "cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Blurred",
        artist: "cxlt. x lednem",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Home, Now",
        artist: "cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Finding Beauty",
    {
        title: "Midnight Journey",
        artist: "Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fading Stars",
        artist: "Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Wanderer ft. kudo",
        artist: "Kainbeats x kudo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lonely Path",
        artist: "Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Respite",
        artist: "Kainbeats x Kurt Stewart",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cirrus Bridge",
        artist: "Kainbeats x no one’s perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cloudy Springs",
        artist: "Kainbeats x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hillside Tree",
        artist: "Kainbeats x S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Glass Spire",
        artist: "Kainbeats x Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Palace in The Sky",
        artist: "Kainbeats x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Satellite Nights",
    {
        title: "Meteor Shower",
        artist: "drkmnd x Ambulo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Satellite Nights",
        artist: "drkmnd",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pluto",
        artist: "drkmnd x Allem Iverson",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Jupiter Jam",
        artist: "drkmnd",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Signal",
        artist: "drkmnd",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Last Alive",
        artist: "drkmnd",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Violet",
    {
        title: "Walk Out",
        artist: "Khutko x Blue Wednesday",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Underbelly",
        artist: "Khutko",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Chimes",
        artist: "Khutko",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Violet",
        artist: "Khutko",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pillow",
        artist: "Khutko",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Vanishing Journey",
    {
        title: "Night Drive",
        artist: "Elijah Lee",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Birdcage",
        artist: "Elijah Lee x Epona",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Trapped in My Mind",
        artist: "Elijah Lee",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Calm",
        artist: "Elijah Lee",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Introspection",
        artist: "Elijah Lee",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Coming Home",
        artist: "Elijah Lee x mell-ø",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sigh of Relief",
        artist: "Elijah Lee x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "High Flying",
    {
        title: "Warm Shimmers",
        artist: "Loafy Building x Project AER",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Found Movement",
        artist: "Loafy Building x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Timeless",
        artist: "Loafy Building x Ayzic",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "High Flying",
        artist: "Loafy Building x Yestalgia",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moonglow",
        artist: "Loafy Building x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Max’s Garden",
        artist: "Loafy Building x w00ds",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sleepless Wonder",
        artist: "Loafy Building x Hoffy Beats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Particles",
    {
        title: "Afterglow",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Bamboo",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cycles",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Suntai",
        artist: "Sleepermane x Casiio x Odyssee",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Maya",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Particles",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Magenta",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Returnal",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Distant Blue",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mockingbird",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Atoms",
        artist: "Sleepermane x Casiio",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Argo",
    {
        title: "Flood",
        artist: "Sátyr x Drxnk",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Circle",
        artist: "Sátyr x Phlocalyst x mell-ø",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Emerald",
        artist: "Sátyr x Phlocalyst x Rudy Raw",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cicada",
        artist: "Sátyr x Phlocalyst x LESKY",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Marble",
        artist: "Sátyr x Drxnk x Elior",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Hyacinth",
        artist: "Sátyr x Drxnk x Akīn",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lucid Dreamin'",
        artist: "Sátyr x Drxnk x Living Room",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Floating Dreams",
    {
        title: "Sunday Morning",
        artist: "BVG x møndberg x Spencer Hunt",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "set sail",
        artist: "BVG x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dreams Can Come True",
        artist: "BVG x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "almost home",
        artist: "BVG x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fireplace",
        artist: "BVG x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "insomnia",
        artist: "BVG x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "after rain",
        artist: "BVG x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The World at Night",
        artist: "BVG x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "When I Dreamt of You",
    {
        title: "I Want To See You Smile",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Perfume",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "When I Dreamt of You",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dearest",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dry Your Eyes",
        artist: "Lilac",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "The Story",
    {
        title: "Wishing Well",
        artist: "Kaspa. x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Holding On",
        artist: "Kaspa. x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Star Trails",
        artist: "Kaspa. x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "English Rain",
        artist: "Kaspa. x Pointy Features",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mull Over",
        artist: "Kaspa. x eaup",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Beauty In All Forms",
    {
        title: "Beauty In All Forms",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Stay Here",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Forever More",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lovely",
        artist: "Hoogway x High On Stars",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Earl Grey",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Soft Garden",
        artist: "Hoogway x Softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Days Like This",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Through Your Eyes",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Rivage",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Heading North",
        artist: "Hoogway x DLJ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Etoiles",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Healing",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "All In The Stars",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Miles Away",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "For The Roses",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Outside For A While",
        artist: "Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Daydreaming",
    {
        title: "Cabin In The Forest",
        artist: "Xander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Leaves",
        artist: "Xander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Love Stories",
        artist: "Xander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lodge",
        artist: "Xander x Chris Mazuera",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Somber",
        artist: "Xander x Philip Somber",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Love Is Still Here",
        artist: "Xander x Carrick",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Summer Love",
        artist: "Xander x Carrick",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dreaming",
        artist: "Xander x Philip Somber",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Something Between Us",
        artist: "Xander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Warm Summer",
        artist: "Xander x Philip Somber",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Miss What We Had",
        artist: "Xander x Goosetaf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "beyond the pines",
    {
        title: "letting go",
        artist: "steezy prime",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "haven",
        artist: "steezy prime",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "sanctuary",
        artist: "steezy prime",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "campfire",
        artist: "steezy prime x Devon Rea",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "youth",
        artist: "steezy prime x Ayzic",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "beluga",
        artist: "steezy prime x tender spring",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "canopy",
        artist: "steezy prime x no one's perfect",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "promises",
        artist: "steezy prime x Tibeauthetraveler",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Reflections in the moonlight",
    {
        title: "Starry night",
        artist: "eugenio izzi x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "A fresh breath",
        artist: "eugenio izzi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lethargy",
        artist: "eugenio izzi x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Like a serendipity",
        artist: "eugenio izzi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The autumn sea",
        artist: "eugenio izzi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Purple Skies",
    {
        title: "Fading Mist",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Evenings",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Purple Skies",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dusk",
        artist: "S N U G x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Findings",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "New Beginnings",
        artist: "S N U G x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Haze",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Another Era",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Illusion",
        artist: "S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Late Return",
        artist: "S N U G x Jordy Chandra",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Sleep Cycles EP",
    {
        title: "time for bed",
        artist: "goodnyght",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "slept in",
        artist: "goodnyght",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "working late",
        artist: "goodnyght",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "early morning",
        artist: "goodnyght",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "dreams",
        artist: "goodnyght",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Massa",
    {
        title: "Massa",
        artist: "l’Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Goshen",
        artist: "l’Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Desert Night",
        artist: "l’Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "To The Euphrates",
        artist: "l’Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Silk Road",
        artist: "l’Outlander",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Perspectives",
    {
        title: "Seashore",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Can't Stay",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cold Water",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "On My Own",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Another Life",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mist",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Circles",
        artist: "Dontcry x Nokiaa",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Adventure Island",
    {
        title: "Where We Take Us",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Time Stands Still",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Breathtaking",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Ferris Wheel",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Shipwreck Cove",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lost Treasure",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Rainbow Falls",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Storm Clouds",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Puddle Jumping",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Mysterious Lights",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Around the Campfire",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "S’mores",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Wanted",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Journey’s End",
        artist: "Purrple Cat",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "offline",
    {
        title: "breezehome",
        artist: "Bert",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "getting better",
        artist: "Bert",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "oregon",
        artist: "Bert",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "cheat codes",
        artist: "Bert",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "sincerely",
        artist: "Bert",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "apart",
        artist: "Bert x møndberg",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "soul gem",
        artist: "Bert",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "two moons",
        artist: "Bert x Trxxshed",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "fuji",
        artist: "Bert x Jhove",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "The Shallows",
    {
        title: "the shallows",
        artist: "hi jude x Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "companion",
        artist: "hi jude x Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "extend",
        artist: "hi jude x Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "undertones",
        artist: "hi jude x Towerz x edelwize",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "distant places",
        artist: "hi jude x Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "oath",
        artist: "hi jude x Towerz x Xandra",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "close to home",
        artist: "hi jude x Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "lakeside",
        artist: "hi jude x Towerz",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "The Bad Party",
    {
        title: "The Bad Party",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Trampoline",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Numb",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "By The Pool",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Her",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Rehash",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sleeping On A Chair",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Unspoken",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Constant Fear",
        artist: "WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Time Capsule",
    {
        title: "Endless Seas",
        artist: "DLJ x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "To the Moon",
        artist: "DLJ x BIDØ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Thousands of Dots",
        artist: "DLJ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Souvenir",
        artist: "DLJ x Nymano",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sand City",
        artist: "DLJ",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Moving Parts",
        artist: "DLJ x Dosi",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Far Away",
        artist: "DLJ x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Same Thoughts",
        artist: "DLJ x Tah.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lost Stars",
        artist: "DLJ x TABAL",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Nocturne",
    {
        title: "Nocturne",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Nighttide",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Lost In Thoughts",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Blue Moon",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Into The Void",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Fireflies",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Echoes In Time",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Eden",
        artist: "amies x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Sea Beams",
    {
        title: "Escapade",
        artist: "Kinissue x Hoffy Beats x Ambulo",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Shore",
        artist: "Kinissue x cxlt. x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Sea Beams",
        artist: "Kinissue x Banks",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Amorous",
        artist: "Kinissue x steezy prime",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cascade",
        artist: "Kinissue x Ayzic",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Shipwreck",
        artist: "Kinissue x Pointy Features",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Imagenero",
    {
        title: "Warm Atmos",
        artist: "Rudy Raw x HM Surf",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Golden Clouds",
        artist: "Rudy Raw x mell-ø",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Liquid Spots",
        artist: "Rudy Raw x Sátyr x Phlocalyst",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Cosmic Nights",
        artist: "Rudy Raw x Phlocalyst",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Peaceful Fusion",
        artist: "Rudy Raw",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Space Magnet",
        artist: "Rudy Raw x Tom Doolie",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Acoustic Dreams",
        artist: "Rudy Raw",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Shelter",
    {
        title: "Leicester Square",
        artist: "Loafy Building x Socrab x ticofaces",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Reflecting",
        artist: "Loafy Building x Hoogway",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Busyland",
        artist: "Loafy Building x Tibeauthetraveler x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Floods of Calm",
        artist: "Loafy Building x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Navy Skies",
        artist: "Loafy Building x Ayzic",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Shelter",
        artist: "Loafy Building x w00ds",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Camden to Chinatown",
        artist: "Loafy Building x Raimu",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Somewhere",
        artist: "Loafy Building x Mondo Loops",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "True Colours",
        artist: " Loafy Building x ticofaces x Socrab",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "My Happy Place",
        artist: "Loafy Building x Kainbeats",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "A place above heaven",
    {
        title: "Safest place on earth",
        artist: "aMess x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Final moments",
        artist: "aMess x kokoro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Field of flowers",
        artist: "aMess x S N U G",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "I’ll be here in the morning",
        artist: "aMess",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Dependency",
        artist: "aMess",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Friday nights",
        artist: "aMess x amies",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Midnight walk",
        artist: "aMess x cxlt.",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Your cozy home",
        artist: "aMess",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Endless",
        artist: "aMess x Bcalm x Banks",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "The Inner Light",
    {
        title: "Pure Soul",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Between Worlds",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Imaginary",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Komorebi",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Overgrown",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Healing River",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Bittersweet Sorrow",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Butterfly Lullaby",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Warm Sleep",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Light",
        artist: "Tenno",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Enchantments",
    {
        title: "Aisuru",
        artist: "DaniSogen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Evening Rain",
        artist: "DaniSogen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Heart of Sakura",
        artist: "DaniSogen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "The Secret Road",
        artist: "DaniSogen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Pure Dream",
        artist: "DaniSogen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Allure",
        artist: "DaniSogen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Quiet Whisper",
        artist: "DaniSogen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "A While Ago",
        artist: "DaniSogen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Place I’ve Never Been Before",
        artist: "DaniSogen",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Shifting Past",
    {
        title: "Come Around",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Come Around .mp3",
        forPart: AllPartType
    },
    {
        title: "Gentle Soul",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Gentle Soul.mp3",
        forPart: AllPartType
    },
    {
        title: "Open Gates",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Open Gates .mp3",
        forPart: AllPartType
    },
    {
        title: "Softened",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Softened .mp3",
        forPart: AllPartType
    },
    {
        title: "Takeoff",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Takeoff .mp3",
        forPart: AllPartType
    },
    {
        title: "Wavelength",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Wavelength .mp3",
        forPart: AllPartType
    },
    {
        title: "Awakened Mind",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Awakened Mind .mp3",
        forPart: AllPartType
    },
    {
        title: "Bloom",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Bloom.mp3",
        forPart: AllPartType
    },
    {
        title: "Shifting",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Shifting .mp3",
        forPart: AllPartType
    },
    {
        title: "Sunrise",
        artist: "Kaspa. x Softy",
        file: "public/audio/lofigirl/MP3 - Kaspa. x Softy - Shifting Past/Kaspa. x softy - Sunrise .mp3",
        forPart: AllPartType
    },
    // "dream tapes",
    {
        title: "been waiting for you",
        artist: "Jhove",
        file: "public/audio/lofigirl/MP3 - Jhove - dream tapes/jhove - been waiting for you.mp3",
        forPart: AllPartType
    },
    {
        title: "i don’t wanna grow old",
        artist: "Jhove",
        file: "public/audio/lofigirl/MP3 - Jhove - dream tapes/jhove - i dont wanna grow old.mp3",
        forPart: AllPartType
    },
    {
        title: "white leaf",
        artist: "Jhove",
        file: "public/audio/lofigirl/MP3 - Jhove - dream tapes/jhove - white leaf.mp3",
        forPart: AllPartType
    },
    {
        title: "just around the corner",
        artist: "Jhove",
        file: "public/audio/lofigirl/MP3 - Jhove - dream tapes/jhove - just around the corner.mp3",
        forPart: AllPartType
    },
    {
        title: "gaze",
        artist: "Jhove",
        file: "public/audio/lofigirl/MP3 - Jhove - dream tapes/jhove - gaze.mp3",
        forPart: AllPartType
    },
    {
        title: "i’m your fallen soldier",
        artist: "Jhove x kokoro",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "when i close my eyes",
        artist: "Jhove x Dillan Witherow",
        file: "public/audio/lofigirl/MP3 - Jhove - dream tapes/jhove - when i close my eyes (ft. dillan witherow).mp3",
        forPart: AllPartType
    },
    {
        title: "please, never let go",
        artist: "Jhove",
        file: "public/audio/lofigirl/MP3 - Jhove - dream tapes/jhove - please, never let go.mp3",
        forPart: AllPartType
    },
    {
        title: "i can’t find my mask",
        artist: "Jhove",
        file: "public/audio/lofigirl/MP3 - Jhove - dream tapes/jhove - i cant find my mask.mp3",
        forPart: AllPartType
    },
    // "Notes from Yesterday",
    {
        title: "just you",
        artist: "Swink",
        file: "public/audio/lofigirl/MP3 - Swink - Notes form Yesterday/1 - Just you.mp3",
        forPart: AllPartType
    },
    {
        title: "leaves fly away",
        artist: "Swink",
        file: "public/audio/lofigirl/MP3 - Swink - Notes form Yesterday/2 - leaves fly away.mp3",
        forPart: AllPartType
    },
    {
        title: "afternoon in the park",
        artist: "Swink",
        file: "public/audio/lofigirl/MP3 - Swink - Notes form Yesterday/3 - afternoon in the park.mp3",
        forPart: AllPartType
    },
    {
        title: "haku",
        artist: "Swink",
        file: "public/audio/lofigirl/MP3 - Swink - Notes form Yesterday/4 - haku.mp3",
        forPart: AllPartType
    },
    {
        title: "sleepin in the park",
        artist: "Swink",
        file: "public/audio/lofigirl/MP3 - Swink - Notes form Yesterday/5 - sleepin in the park.mp3",
        forPart: AllPartType
    },
    {
        title: "the end of fall",
        artist: "Swink",
        file: "public/audio/lofigirl/MP3 - Swink - Notes form Yesterday/6 - the end of fall.mp3",
        forPart: AllPartType
    },
    // "Creating Memories",
    {
        title: "Creating Memories",
        artist: "Yasumu",
        file: "public/audio/lofigirl/MP3 - Yasumu - Creating Memories/Yasumu - Creating Memories.mp3",
        forPart: AllPartType
    },
    {
        title: "Drift Away",
        artist: "Yasumu",
        file: "public/audio/lofigirl/MP3 - Yasumu - Creating Memories/Yasumu - Drift Away.mp3",
        forPart: AllPartType
    },
    {
        title: "Dreaming About It",
        artist: "Yasumu",
        file: "public/audio/lofigirl/MP3 - Yasumu - Creating Memories/Yasumu - Dreaming About It.mp3",
        forPart: AllPartType
    },
    {
        title: "Perspectives",
        artist: "Yasumu",
        file: "public/audio/lofigirl/MP3 - Yasumu - Creating Memories/Yasumu - Perspectives.mp3",
        forPart: AllPartType
    },
    {
        title: "Leaves",
        artist: "Yasumu",
        file: "public/audio/lofigirl/MP3 - Yasumu - Creating Memories/Yasumu - Leaves.mp3",
        forPart: AllPartType
    },
    {
        title: "At Night",
        artist: "Yasumu",
        file: "public/audio/lofigirl/MP3 - Yasumu - Creating Memories/Yasumu - At Night.mp3",
        forPart: AllPartType
    },
    {
        title: "I Thought We Were Friends",
        artist: "Yasumu",
        file: "public/audio/lofigirl/MP3 - Yasumu - Creating Memories/Yasumu - I Thought We Were Friends.mp3",
        forPart: AllPartType
    },
    // "Dreamscapes",
    {
        title: "The Last Time",
        artist: "Kayou.",
        file: "public/audio/lofigirl/MP3 - Kayou - Dreamscapes/Kayou. - The Last Time.mp3",
        forPart: AllPartType
    },
    {
        title: "Stargazing",
        artist: "Kayou.",
        file: "public/audio/lofigirl/MP3 - Kayou - Dreamscapes/Kayou. - Stargazing.mp3",
        forPart: AllPartType
    },
    {
        title: "Growing Up",
        artist: "Kayou.",
        file: "public/audio/lofigirl/MP3 - Kayou - Dreamscapes/Kayou. - Growing Up.mp3",
        forPart: AllPartType
    },
    {
        title: "Fireflies",
        artist: "Kayou.",
        file: "public/audio/lofigirl/MP3 - Kayou - Dreamscapes/Kayou. - Fireflies.mp3",
        forPart: AllPartType
    },
    {
        title: "Echoes Of The Past",
        artist: "Kayou.",
        file: "public/audio/lofigirl/MP3 - Kayou - Dreamscapes/Kayou. - Echoes Of The Past.mp3",
        forPart: AllPartType
    },
    {
        title: "Sternbilder",
        artist: "Kayou.",
        file: "public/audio/lofigirl/MP3 - Kayou - Dreamscapes/Kayou. - Sternbilder.mp3",
        forPart: AllPartType
    },
    {
        title: "Passing Lights",
        artist: "Kayou.",
        file: "public/audio/lofigirl/MP3 - Kayou - Dreamscapes/Kayou. - Passing Lights.mp3",
        forPart: AllPartType
    },
    {
        title: "Memory Lane",
        artist: "Kayou.",
        file: "public/audio/lofigirl/MP3 - Kayou - Dreamscapes/Kayou. - Memory Lane.mp3",
        forPart: AllPartType
    },
    // "Ethereal Nights",
    {
        title: "Ocean Planet",
        artist: "SCayos x Barnes Blvd.",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/01 Ocean Planet w_ Barnes Blvd MASTER (online-audio-converter.com).mp3",
        forPart: AllPartType
    },
    {
        title: "Horizon",
        artist: "SCayos",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/02 Horizon MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Andromeda",
        artist: "SCayos",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/03 Andromeda MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Astro",
        artist: "SCayos x frumhere x Hixon Foster",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/04 Astro w_ Frumhere & Hixon Foster MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Satellite",
        artist: "SCayos",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/05 Satellite MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Galaxy",
        artist: "SCayos x Phlocalyst",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/06 Galaxy w_ Phlocalyst MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Nebula",
        artist: "SCayos",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/07 Nebula MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Interstellar",
        artist: "SCayos x Strehlow",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/08 Interstellar w_ Strehlow MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Lunar Souls",
        artist: "SCayos x Interlude",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/09 Lunar Souls (Interlude) MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Spacecraft",
        artist: "SCayos x frumhere",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/10 Spacecraft w_ Frumhere MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Ethereal Nights",
        artist: "SCayos",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/11 Ethereal Nights MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Moonshine",
        artist: "SCayos x Hixon Foster",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/12 Moonshine w_ Hixon Foster MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Orion",
        artist: "SCayos x Azayaka",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/13 Orion w_ Azayaka MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Moondrops",
        artist: "SCayos x Phlocalyst",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/14 Moondrops w_ Phlocalyst MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Constellations",
        artist: "SCayos",
        file: "public/audio/lofigirl/MP3 - SCayos - Ethereal Nights/15 Constellations MASTER.mp3",
        forPart: AllPartType
    },
    // "Forest Tales",
    {
        title: "Dusty Records",
        artist: "Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/1) Dusty Records.mp3",
        forPart: AllPartType
    },
    {
        title: "Essence of the Forest",
        artist: "Mondo Loops x Purrple Cat",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/2) Essence of the Forest - w Purple Cat.mp3",
        forPart: AllPartType
    },
    {
        title: "Kyoshi",
        artist: "Mondo Loops x L.Dre",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/3) Kyoshi (Ft L.Dre).mp3",
        forPart: AllPartType
    },
    {
        title: "Secret Forest",
        artist: "Mondo Loops x softy",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/4) Secret Forest (Ft Softy_.mp3",
        forPart: AllPartType
    },
    {
        title: "Have Hope",
        artist: "Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/5) Have Hope.mp3",
        forPart: AllPartType
    },
    {   // ファイル名とタイトルが一致しない。
        title: "Suntory Time",
        artist: "Mondo Loops x WYS",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Old Dee",
        artist: "Mondo Loops x Kanisan",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/7) Old Dee Ft Kanisan.mp3",
        forPart: AllPartType
    },
    {
        title: "A Journey In The Dark",
        artist: "Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/8) A Journey In The Dark.mp3",
        forPart: AllPartType
    },
    {
        title: "Treasures In The Cave",
        artist: "Mondo Loops x softy",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/9) Treasures In The Cave Ft Softy.mp3",
        forPart: AllPartType
    },
    {
        title: "Visions In The Trees",
        artist: "Mondo Loops x Kanisan",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/10) Visions In The Trees Ft Kanisan.mp3",
        forPart: AllPartType
    },
    {
        title: "End Of The Water",
        artist: "Mondo Loops x L.Dre",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/11) End Of The Water (Ft L.Dre).mp3",
        forPart: AllPartType
    },
    {
        title: "Videotapes",
        artist: "Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/12) Videotapes.mp3",
        forPart: AllPartType
    },
    {
        title: "Forgotten Riddles",
        artist: "Mondo Loops x Kanisan",
        file: "public/audio/lofigirl/MP3 - Mondo Loops - Forest Tales/13) Forgotten Riddles Ft Kanisan.mp3",
        forPart: AllPartType
    },
    // "Simple Things",
    {
        title: "Thinking of You",
        artist: "Oatmello",
        file: "public/audio/lofigirl/MP3 - Oatmello - Simple Things/01_Thinking of You.mp3",
        forPart: AllPartType
    },
    {
        title: "Dark Chocolate",
        artist: "Oatmello x Slo Loris",
        file: "public/audio/lofigirl/MP3 - Oatmello - Simple Things/02_Dark Chocolate with Slo Loris.mp3",
        forPart: AllPartType
    },
    {
        title: "Gentle Breeze",
        artist: "Oatmello x TyLuv. x Dillion Witherow",
        file: "public/audio/lofigirl/MP3 - Oatmello - Simple Things/03_Gentle Breeze with Tyluv and Dillon Witherow.mp3",
        forPart: AllPartType
    },
    {
        title: "Fresh Snow",
        artist: "Oatmello",
        file: "public/audio/lofigirl/MP3 - Oatmello - Simple Things/04_Fresh Snow.mp3",
        forPart: AllPartType
    },
    {
        title: "Hot Coffee",
        artist: "Oatmello x SCayos",
        file: "public/audio/lofigirl/MP3 - Oatmello - Simple Things/05_Hot Coffee with Scayos.mp3",
        forPart: AllPartType
    },
    {
        title: "Watching the Stars",
        artist: "Oatmello",
        file: "public/audio/lofigirl/MP3 - Oatmello - Simple Things/06_Watching the Stars.mp3",
        forPart: AllPartType
    },
    {
        title: "Ocean Sunset",
        artist: "Oatmello",
        file: "public/audio/lofigirl/MP3 - Oatmello - Simple Things/07_Ocean Sunset.mp3",
        forPart: AllPartType
    },
    {
        title: "Afternoon Nap",
        artist: "Oatmello",
        file: "public/audio/lofigirl/MP3 - Oatmello - Simple Things/08_Afternoon Nap.mp3",
        forPart: AllPartType
    },
    // "After Sunset",
    {
        title: "After Sunset",
        artist: "Living Room x Viktor Minsky",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Viktor Minsky - After Sunset MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Friendship",
        artist: "Living Room x Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Mondo Loops - Friendship MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Big Dreams",
        artist: "Living Room x Otaam",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Otaam - Big Dreams MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "The Night Is Full Of Wonders",
        artist: "Living Room x mell-ø",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Mell-o - The Night Is Full Of Wonders MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Prayer",
        artist: "Living Room x Viktor Minsky x Rosoul",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Rosoul x Viktor Minsky - Prayer MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Bright Future",
        artist: "Living Room x Phlocalyst",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Phlocalyst - Bright Future MASTER (1).mp3",
        forPart: AllPartType
    },
    {
        title: "Seven",
        artist: "Living Room x Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Mondo Loops - Seven MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Northern Tales",
        artist: "Living Room x Epona",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Elior - Northern Tales MASTER2.mp3",
        forPart: AllPartType
    },
    {
        title: "Purple Sky",
        artist: "Living Room x Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Mondo Loops - Purple Sky MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Moonflowers",
        artist: "Living Room x Akīn",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Akin - Moonflowers MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Movie",
        artist: "Living Room x Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Living Room - After Sunset/Living Room x Mondo Loops - Movie MASTER.mp3",
        forPart: AllPartType
    },
    // "Springtime, with friends",
    {
        title: "i got u",
        artist: "tender spring x another silent weekend x Blue Wednesday",
        file: "public/audio/lofigirl/MP3 - Tender Spring - Springtime, with friends/i got u w. asw & blue wednesday.mp3",
        forPart: AllPartType
    },
    {
        title: "springtime, with friends",
        artist: "tender spring x biniou",
        file: "public/audio/lofigirl/MP3 - Tender Spring - Springtime, with friends/springtime, with friends w. biniou.mp3",
        forPart: AllPartType
    },
    {
        title: "last sunset",
        artist: "tender spring x Towerz x edelwize",
        file: "public/audio/lofigirl/MP3 - Tender Spring - Springtime, with friends/last sunset w towerz & edelwize.mp3",
        forPart: AllPartType
    },
    {
        title: "slow melt",
        artist: "tender spring x another silent weekend x INKY!",
        file: "public/audio/lofigirl/MP3 - Tender Spring - Springtime, with friends/slow melt w asw & INKY!.mp3",
        forPart: AllPartType
    },
    {
        title: "holding",
        artist: "tender spring x Tatami Construct",
        file: "public/audio/lofigirl/MP3 - Tender Spring - Springtime, with friends/holding hams w tatami construct (1).mp3",
        forPart: AllPartType
    },
    {
        title: "plucky",
        artist: "tender spring x Blurred Figures",
        file: "public/audio/lofigirl/MP3 - Tender Spring - Springtime, with friends/plucky w blurred figures.mp3",
        forPart: AllPartType
    },
    {
        title: "snowstorm in april",
        artist: "tender spring x biniou x chief.",
        file: "public/audio/lofigirl/MP3 - Tender Spring - Springtime, with friends/snowstorm in april w. chief & biniou.mp3",
        forPart: AllPartType
    },
    // "Quietude",
    {
        title: "Circadian",
        artist: "S N U G x Enluv",
        file: "public/audio/lofigirl/MP3 - S N U G - Quietude/1. circadian ft. Enluv.mp3",
        forPart: AllPartType
    },
    {
        title: "Equinox",
        artist: "S N U G",
        file: "public/audio/lofigirl/MP3 - S N U G - Quietude/3. equinox.mp3",
        forPart: AllPartType
    },
    {
        title: "4. fireflies",
        artist: "S N U G x Dimension 32",
        file: "public/audio/lofigirl/MP3 - S N U G - Quietude/7. fireflies ft. Dimension 32.mp3",
        forPart: AllPartType
    },
    {
        title: "5. mahogany",
        artist: "S N U G x Mondo Loops",
        file: "public/audio/lofigirl/MP3 - S N U G - Quietude/8. mahogany ft. Mondo Loops.mp3",
        forPart: AllPartType
    },
    {
        title: "7. Lustre",
        artist: "S N U G",
        file: "public/audio/lofigirl/MP3 - S N U G - Quietude/5. lustre.mp3",
        forPart: AllPartType
    },
    // "Silk",
    {
        title: "Summer Rain",
        artist: "iamalex x Felty",
        file: "public/audio/lofigirl/MP3 - iamalex x Felty - Silk/1. iamalex & Felty - Summer Rain.mp3",
        forPart: AllPartType
    },
    {
        title: "Kiss Me",
        artist: "iamalex x Felty x Blossum",
        file: "public/audio/lofigirl/MP3 - iamalex x Felty - Silk/2. iamalex & Felty - Kiss Me (Feat. Blossum) .mp3",
        forPart: AllPartType
    },
    {
        title: "Desert",
        artist: "iamalex x Felty",
        file: "public/audio/lofigirl/MP3 - iamalex x Felty - Silk/3. iamalex & Felty - Desert.mp3",
        forPart: AllPartType
    },
    {
        title: "Fields",
        artist: "iamalex x Felty x Jhove",
        file: "public/audio/lofigirl/MP3 - iamalex x Felty - Silk/4. iamalex & Felty - Fields (Feat. Jhove).mp3",
        forPart: AllPartType
    },
    {
        title: "On A Cloud",
        artist: "iamalex x Felty",
        file: "public/audio/lofigirl/MP3 - iamalex x Felty - Silk/5. iamalex & Felty - On A Cloud.mp3",
        forPart: AllPartType
    },
    {
        title: "Sunday Sleeping",
        artist: "iamalex x Felty",
        file: "public/audio/lofigirl/MP3 - iamalex x Felty - Silk/6. iamalex & Felty - Sunday Sleeping.mp3",
        forPart: AllPartType
    },
    {
        title: "Far From Home",
        artist: "iamalex x Felty",
        file: "public/audio/lofigirl/MP3 - iamalex x Felty - Silk/7. iamalex & Felty - Far From Home.mp3",
        forPart: AllPartType
    },
    // "Inference",
    {
        title: "Divine",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Divine.mp3",
        forPart: AllPartType
    },
    {
        title: "Slow Down",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Slow Down.mp3",
        forPart: AllPartType
    },
    {
        title: "Beacon",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Beacon.mp3",
        forPart: AllPartType
    },
    {
        title: "Traffic Lights",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Traffic Lights.mp3",
        forPart: AllPartType
    },
    {
        title: "Bird Watcher",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Bird Watcher.mp3",
        forPart: AllPartType
    },
    {
        title: "94",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/94 new.mp3",
        forPart: AllPartType
    },
    {
        title: "Graze",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Graze.mp3",
        forPart: AllPartType
    },
    {
        title: "Breakaway",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Breakaway.mp3",
        forPart: AllPartType
    },
    {
        title: "Flash",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Flash.mp3",
        forPart: AllPartType
    },
    {
        title: "Butterflies",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Butterflies.mp3",
        forPart: AllPartType
    },
    {
        title: "Lockdown",
        artist: "Mavine",
        file: "public/audio/lofigirl/MP3 - Mavine - Inference/Lockdown.mp3",
        forPart: AllPartType
    },
    // "Before It’s Late, Pt. 2",
    {
        title: "Morning Brew",
        artist: "Hevi x Paper Ocean",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/Morning Brew (feat. Paper Ocean).mp3",
        forPart: AllPartType
    },
    {
        title: "Summer Evenings",
        artist: "Hevi x Dimension32",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/Summer Evenings (feat. Dimension32).mp3",
        forPart: AllPartType
    },
    {
        title: "Warm Waves",
        artist: "Hevi x Hoogway",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/Warm Waves (feat. Hoogway).mp3",
        forPart: AllPartType
    },
    {
        title: "Looking Back",
        artist: "Hevi x Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/Looking Back (feat. Mondo Loops).mp3",
        forPart: AllPartType
    },
    {
        title: "And Then I Woke Up",
        artist: "Hevi x no one’s perfect",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/and then i woke up (feat. no one's perfect).mp3",
        forPart: AllPartType
    },
    {
        title: "Dusk",
        artist: "Hevi x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/Dusk (feat. Lawrence Walther).mp3",
        forPart: AllPartType
    },
    {
        title: "Circles",
        artist: "Hevi x Kurt Stewart",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/Circles (feat. Kurt Stewart).mp3",
        forPart: AllPartType
    },
    {
        title: "Ghosts",
        artist: "Hevi x no one’s perfect",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/ghosts (feat. no one's perfect).mp3",
        forPart: AllPartType
    },
    {
        title: "Drift",
        artist: "Hevi x no one’s perfect",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/Drift (feat. no one's perfect).mp3",
        forPart: AllPartType
    },
    {
        title: "Homecoming",
        artist: "Hevi x Bert",
        file: "public/audio/lofigirl/MP3 - Hevi - Before It's Late, Pt. 2/homecoming (feat. Bert).mp3",
        forPart: AllPartType
    },
    // "Distance Love",
    {
        title: "miss you",
        artist: "Tibeauthetraveler",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/1. miss you (master).mp3",
        forPart: AllPartType
    },
    {
        title: "good morning, love",
        artist: "Tibeauthetraveler x Hoogway",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/2. good morning, love (ft. Hoogway) (master).mp3",
        forPart: AllPartType
    },
    {
        title: "soul searching",
        artist: "Tibeauthetraveler",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/3. soul searching (master).mp3",
        forPart: AllPartType
    },
    {
        title: "bloom",
        artist: "Tibeauthetraveler x Krynoze",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/4. bloom (ft. Krynoze) (master).mp3",
        forPart: AllPartType
    },
    {
        title: "looking at the moon",
        artist: "Tibeauthetraveler",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/5. looking at the moon (master).mp3",
        forPart: AllPartType
    },
    {
        title: "faraway",
        artist: "Tibeauthetraveler x Antonius B",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/6. faraway (ft. Antonius B) (master).mp3",
        forPart: AllPartType
    },
    {
        title: "come closer",
        artist: "Tibeauthetraveler ",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/7. come closer (master).mp3",
        forPart: AllPartType
    },
    {
        title: "your eyes",
        artist: "Tibeauthetraveler x JinSei x Sam Cross",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/8. your eyes (ft. JinSei & Sam Cross) (master).mp3",
        forPart: AllPartType
    },
    {
        title: "station to station",
        artist: "Tibeauthetraveler x Banks x Bcalm",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/9. station to station (ft. Banks & Bcalm) (master).mp3",
        forPart: AllPartType
    },
    {
        title: "it’s ok",
        artist: "Tibeauthetraveler",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraverler - Distance Love/10. It's ok (master).mp3",
        forPart: AllPartType
    },
    // "Alley Of Trees",
    {
        title: "Alley of Trees",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Alley Of Trees x Softy.mp3",
        forPart: AllPartType
    },
    {
        title: "Words on Water",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Words On Water x Softy.mp3",
        forPart: AllPartType
    },
    {
        title: "Only You",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Only You x Softy.mp3",
        forPart: AllPartType
    },
    {
        title: "Saudade",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Saudade x Softy.mp3",
        forPart: AllPartType
    },
    {
        title: "Lune",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Lune x Softy.mp3",
        forPart: AllPartType
    },
    {
        title: "Seaside Farewell",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Seaside Farewell x Softy.mp3",
        forPart: AllPartType
    },
    {
        title: "Les Nuages",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Nuages x Softy.mp3",
        forPart: AllPartType
    },
    {
        title: "Open Skies",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Open Skies.mp3",
        forPart: AllPartType
    },
    {
        title: "Frozen Waters",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Frozen Waters x Softy .mp3",
        forPart: AllPartType
    },
    {
        title: "Faded Hills",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Faded Hills x Softy.mp3",
        forPart: AllPartType
    },
    {
        title: "Golden Lakes",
        artist: "Hoogway x Softy",
        file: "public/audio/lofigirl/MP3 - HoogWay x Softy - Alley of Trees/Golden Lakes x Softy.mp3",
        forPart: AllPartType
    },
    // "A Spirit’s Tale",
    {
        title: "Distant Voices",
        artist: "BVG x møndberg",
        file: "public/audio/lofigirl/MP3 - BVG - A Spirit's Tale/BVG x møndberg - Distant Voices.mp3",
        forPart: AllPartType
    },
    {
        title: "Memories",
        artist: "BVG x møndberg",
        file: "public/audio/lofigirl/MP3 - BVG - A Spirit's Tale/BVG x møndberg - Memories.mp3",
        forPart: AllPartType
    },
    {
        title: "Wisdom",
        artist: "BVG x møndberg",
        file: "public/audio/lofigirl/MP3 - BVG - A Spirit's Tale/BVG x møndberg - Wisdom.mp3",
        forPart: AllPartType
    },
    {
        title: "Letting Go",
        artist: "BVG x møndberg x Trix.",
        file: "public/audio/lofigirl/MP3 - BVG - A Spirit's Tale/BVG x Trix. - Letting Go (ft.møndberg).mp3",
        forPart: AllPartType
    },
    {
        title: "Moon Lake",
        artist: "BVG x Tibeauthetraveler",
        file: "public/audio/lofigirl/MP3 - BVG - A Spirit's Tale/BVG x Tibeauthetraveler - Moon Lake.mp3",
        forPart: AllPartType
    },
    {
        title: "Solemn Winds",
        artist: "BVG x møndberg",
        file: "public/audio/lofigirl/MP3 - BVG - A Spirit's Tale/BVG x møndberg - Solemn Winds.mp3",
        forPart: AllPartType
    },
    {
        title: "Sands Of Time",
        artist: "BVG x møndberg",
        file: "public/audio/lofigirl/MP3 - BVG - A Spirit's Tale/BVG x møndberg - Sands Of Time.mp3",
        forPart: AllPartType
    },
    {
        title: "The Other Side",
        artist: "BVG",
        file: "public/audio/lofigirl/MP3 - BVG - A Spirit's Tale/BVG - The Other Side.mp3",
        forPart: AllPartType
    },
    {
        title: "Serenity",
        artist: "BVG x møndberg",
        file: "public/audio/lofigirl/MP3 - BVG - A Spirit's Tale/BVG x møndberg - Serenity.mp3",
        forPart: AllPartType
    },
    // "Time To Go", ダウンロードリンクなし
    // {
    //   title: "Drift Away",
    //   artist: "mell-ø x Ambulo",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Clocks",
    //   artist: "mell-ø",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Alder",
    //   artist: "mell-ø",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Forest Trails",
    //   artist: "mell-ø x no one's perfect",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Currents",
    //   artist: "mell-ø x mtch.",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Away From Home",
    //   artist: "mell-ø",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Northern Lights",
    //   artist: "mell-ø x Osaki",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Passage",
    //   artist: "mell-ø x Ambulo",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // "Lost World",
    {
        title: "Lost World",
        artist: "squeeda",
        file: "public/audio/lofigirl/MP3 - Squeeda - Lost World/1 Lost World.mp3",
        forPart: AllPartType
    },
    {
        title: "Summer Eyes",
        artist: "squeeda x No Spirit",
        file: "public/audio/lofigirl/MP3 - Squeeda - Lost World/2 Summer Eyes (w No Spirit).mp3",
        forPart: AllPartType
    },
    {
        title: "Timepaint",
        artist: "squeeda",
        file: "public/audio/lofigirl/MP3 - Squeeda - Lost World/3 Timepaint.mp3",
        forPart: AllPartType
    },
    {
        title: "Field Trip",
        artist: "squeeda",
        file: "public/audio/lofigirl/MP3 - Squeeda - Lost World/4 Field Trip.mp3",
        forPart: AllPartType
    },
    {
        title: "Invisible Medicine",
        artist: "squeeda x Enluv",
        file: "public/audio/lofigirl/MP3 - Squeeda - Lost World/5 Invisible Medicine (w Enluv).mp3",
        forPart: AllPartType
    },
    {
        title: "Dreamsand",
        artist: "squeeda x Ambulo",
        file: "public/audio/lofigirl/MP3 - Squeeda - Lost World/6 Dreamsand (w Ambulo).mp3",
        forPart: AllPartType
    },
    // "The Way Back", ダウンロードリンクがない
    // {
    //   title: "The Way Back",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Silver Silver",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Spanish Castle",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Blazing Sun",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "After The Storm",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Western Point",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Wild Horses",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Everything We Left",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Heartland",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Diamond Dust",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Harbour",
    //   artist: "WYS x Sweet Medicine",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // "Soothing Breeze",
    {
        title: "Cherry Tree",
        artist: "Tibeauthetraveler",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/01 Tibeauthetraveler - Cherry Tree (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "The Guiding Wind",
        artist: "Tenno",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/02 Tenno - The Guiding Wind (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Kaigan",
        artist: "Raimu x Tophat Panda",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/03 Raimu _ Tophat Panda - Kaigan (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Windmills",
        artist: "Ambulo",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/04 Ambulo - Windmills  (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Shibui",
        artist: "Jhove",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/05 jhove - shibui (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Mystic Mountain",
        artist: "Purrple Cat",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/06 Purrple Cat - Mystic Mountain (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "In Love With The Sky",
        artist: "Raimu x DaniSogen",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/07 Raimu _ DaniSogen - In Love With The Sky (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "The View From The Monastery",
        artist: "Celestial Alignment",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/08 Celestial Alignment - The View From The Monastery (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Winter Gardens",
        artist: " Midnight Alpha x Nothingtosay",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/09 Midnight Alpha - Winter Gardens (w Nothingtosay) (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Lushan Sun",
        artist: "Sweet Medicine",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/10 Sweet Medicine - Lushan Sun (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Wander",
        artist: "Dryhope",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/11 Dryhope - Wander (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Until Dawn",
        artist: "Kanisan",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/12 Kanisan - Until Dawn (Kupla Master 2.0).mp3",
        forPart: AllPartType
    },
    {
        title: "West Of Zhuhai",
        artist: "Yestalgia x Loafy Building",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/13 Yestalgia X Loafy Building - West Of Zhuhai (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "The Path You Choose",
        artist: "BVG",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/14 BVG - The Path You Choose (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Neon TIger",
        artist: "Purrple Cat",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/15 Purrple Cat - Neon Tiger (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Tsuyu",
        artist: "Otaam x C4C",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/16 Otaam x C4C - Tsuyu (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Waterfall",
        artist: "BVG x møndberg",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/17 BVG x møndberg - Waterfall (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Koi",
        artist: "Phlocalyst x Living Room x Myríad",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/18 Phlocalyst _ Living Room _ Myríad - Koi (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Fuji",
        artist: "Living Room x Otaam ",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/19 Living Room x Otaam - Fuji (Kupla Master).mp3",
        forPart: AllPartType
    },
    {
        title: "Danso Lullaby",
        artist: "Mondo Loops x softy",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/20 Mondo Loops - Danso Lullaby (w Softy) (Kupla MAster).mp3",
        forPart: AllPartType
    },
    {
        title: "High Sun",
        artist: "Jhove",
        file: "public/audio/lofigirl/MP3 - Compilation 7 - Soothing Breeze/21 Jhove- High Sun (Kupla Master).mp3",
        forPart: AllPartType
    },
    // "Cloud Shapes", ダウンロードリンクがない
    // {
    //   title: "Parhelia",
    //   artist: "Leavv",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Cloud Shapes",
    //   artist: "Leavv",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Backyard Shower",
    //   artist: "Leavv",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Fields",
    //   artist: "Leavv",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Travelogue",
    //   artist: "Leavv x C4C",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // "Better Days",
    {
        title: "Panacea",
        artist: "Mujo x Sweet Medicine x juniorodeo",
        file: "public/audio/lofigirl/MP3 - Mujo x Sweet Medicine - Better Days/01 - Mujo x Sweet Medicine x juniorodeo - Panacea MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Everything Gone",
        artist: "Mujo x Sweet Medicine x Jhove",
        file: "public/audio/lofigirl/MP3 - Mujo x Sweet Medicine - Better Days/02 - Mujo x Sweet Medicine x Jhove - Everything Gone MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Flooded With Light",
        artist: "Mujo x Sweet Medicine",
        file: "public/audio/lofigirl/MP3 - Mujo x Sweet Medicine - Better Days/03 - Mujo x Sweet Medicine - Flooded With Light MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Backwards",
        artist: "Mujo x Sweet Medicine x Purrple Cat",
        file: "public/audio/lofigirl/MP3 - Mujo x Sweet Medicine - Better Days/04 - Mujo x Sweet Medicine x Purrple Cat - Backwards MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Evergreen",
        artist: "Mujo x Sweet Medicine x G Mills",
        file: "public/audio/lofigirl/MP3 - Mujo x Sweet Medicine - Better Days/05 - Mujo x Sweet Medicine x G Mills - Evergreen MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "All The Good Times",
        artist: "Mujo x Sweet Medicine x Hoogway",
        file: "public/audio/lofigirl/MP3 - Mujo x Sweet Medicine - Better Days/06 - Mujo x Sweet Medicine x Hoogway - All The Good Times MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Peace Of Mind",
        artist: "Mujo x Sweet Medicine",
        file: "public/audio/lofigirl/MP3 - Mujo x Sweet Medicine - Better Days/07 - Mujo x Sweet Medicine - Peace Of Mind MASTER.mp3",
        forPart: AllPartType
    },
    {
        title: "Healing Winds",
        artist: "Mujo x Sweet Medicine",
        file: "public/audio/lofigirl/MP3 - Mujo x Sweet Medicine - Better Days/08 - Mujo x Sweet Medicine - Healing Winds MASTER.mp3",
        forPart: AllPartType
    },
    // "blindsighted",
    {
        title: "clear eyes, blind sight",
        artist: "Kainbeats",
        file: "public/audio/lofigirl/MP3 - kainbeats - blindsighted/1. clear eyes, blind sight.mp3",
        forPart: AllPartType
    },
    {
        title: "where light can’t reach",
        artist: "Kainbeats x Sleepermane",
        file: "public/audio/lofigirl/MP3 - kainbeats - blindsighted/2. where light can_t reach (ft. Sleepermane).mp3",
        forPart: AllPartType
    },
    {
        title: "timeless gift",
        artist: "Kainbeats x Hoogway",
        file: "public/audio/lofigirl/MP3 - kainbeats - blindsighted/3. timeless gift ft. Hoogway mastered.mp3",
        forPart: AllPartType
    },
    {
        title: "something dear",
        artist: "Kainbeats x Towerz",
        file: "public/audio/lofigirl/MP3 - kainbeats - blindsighted/4. something dear (ft. Towerz).mp3",
        forPart: AllPartType
    },
    {
        title: "beauty in everything",
        artist: "Kainbeats",
        file: "public/audio/lofigirl/MP3 - kainbeats - blindsighted/5. beauty in everything.mp3",
        forPart: AllPartType
    },
    {
        title: "lovely glow",
        artist: "Kainbeats",
        file: "public/audio/lofigirl/MP3 - kainbeats - blindsighted/6. lovely glow.mp3",
        forPart: AllPartType
    },
    // "One Way Ticket",
    {
        title: "One Way Ticke",
        artist: "l’Outlander",
        file: "public/audio/lofigirl/MP3 - l'outlander - One Way Ticket/1.One Way Ticket.mp3",
        forPart: AllPartType
    },
    {
        title: "Warm Country",
        artist: "l’Outlander",
        file: "public/audio/lofigirl/MP3 - l'outlander - One Way Ticket/2.Warm Country.mp3",
        forPart: AllPartType
    },
    {
        title: "Summer Nights",
        artist: "l’Outlander x Pandrezz",
        file: "public/audio/lofigirl/MP3 - l'outlander - One Way Ticket/3.Summer Nights ft.Pandrezz.mp3",
        forPart: AllPartType
    },
    {
        title: "Hamsin",
        artist: "l’Outlander",
        file: "public/audio/lofigirl/MP3 - l'outlander - One Way Ticket/4.Hamsin.mp3",
        forPart: AllPartType
    },
    {
        title: "City On A Hill",
        artist: "l’Outlander",
        file: "public/audio/lofigirl/MP3 - l'outlander - One Way Ticket/5.City On A Hill.mp3",
        forPart: AllPartType
    },
    {
        title: "Soul Searching",
        artist: "l’Outlander",
        file: "public/audio/lofigirl/MP3 - l'outlander - One Way Ticket/6.Soul Searching.mp3",
        forPart: AllPartType
    },
    {
        title: "Forever Young",
        artist: "l’Outlander x hoogway",
        file: "public/audio/lofigirl/MP3 - l'outlander - One Way Ticket/7.Forever Young ft.hoogway.mp3",
        forPart: AllPartType
    },
    // "Feelin Better",
    {
        title: "I Hope U Feel Better Now",
        artist: "Pandrezz",
        file: "public/audio/lofigirl/MP3 - Pandrezz - Feelin Better/1. i hope u feel better now.mp3",
        forPart: AllPartType
    },
    {
        title: "Friday Night With You",
        artist: "Pandrezz",
        file: "public/audio/lofigirl/MP3 - Pandrezz - Feelin Better/2. Friday Night With You.mp3",
        forPart: AllPartType
    },
    {
        title: "Shivers",
        artist: "Pandrezz",
        file: "public/audio/lofigirl/MP3 - Pandrezz - Feelin Better/3. Shivers.mp3",
        forPart: AllPartType
    },
    {
        title: "Feelin Warm",
        artist: "Pandrezz",
        file: "public/audio/lofigirl/MP3 - Pandrezz - Feelin Better/4. Feelin Warm-1.mp3",
        forPart: AllPartType
    },
    {
        title: "Misty Village",
        artist: "Pandrezz",
        file: "public/audio/lofigirl/MP3 - Pandrezz - Feelin Better/5. Pandrezz - Misty Village.mp3",
        forPart: AllPartType
    },
    {
        title: "Could Have Done More",
        artist: "Pandrezz",
        file: "public/audio/lofigirl/MP3 - Pandrezz - Feelin Better/6. Could Have Done More.mp3",
        forPart: AllPartType
    },
    {
        title: "Beginner",
        artist: "Pandrezz x Kronomuzik",
        file: "public/audio/lofigirl/MP3 - Pandrezz - Feelin Better/7. Beginner (ft Kronomuzik).mp3",
        forPart: AllPartType
    },
    {
        title: "Single Star",
        artist: "Pandrezz",
        file: "public/audio/lofigirl/MP3 - Pandrezz - Feelin Better/8. Single Star.mp3",
        forPart: AllPartType
    },
    {
        title: "Lonely Friday Night",
        artist: "Pandrezz",
        file: "public/audio/lofigirl/MP3 - Pandrezz - Feelin Better/9. Lonely Friday Night.mp3",
        forPart: AllPartType
    },
    // "Elements",
    {
        title: "times with you",
        artist: "Bcalm x softy",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/1. Bcalm _ softy - times with you.mp3",
        forPart: AllPartType
    },
    {
        title: "skyblue",
        artist: "Bcalm x Purrple Cat",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/2. Bcalm _ Purrple Cat - skyblue.mp3",
        forPart: AllPartType
    },
    {
        title: "cutie",
        artist: "Bcalm x Banks",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/3. Bcalm _ Banks - cutie.mp3",
        forPart: AllPartType
    },
    {
        title: "traveller",
        artist: "Bcalm x Purrple Cat",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/4. Bcalm _ Purrple Cat - traveller.mp3",
        forPart: AllPartType
    },
    {
        title: "rose fields",
        artist: "Bcalm x Banks",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/5. Bcalm _ Banks - rose fields.mp3",
        forPart: AllPartType
    },
    {
        title: "pebbles",
        artist: "Bcalm x Banks x Mondo Loops",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/6. Bcalm _ Banks _ Mondo Loops - pebbles.mp3",
        forPart: AllPartType
    },
    {
        title: "within",
        artist: "Bcalm x Hoogway",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/7. Bcalm _ Hoogway - within.mp3",
        forPart: AllPartType
    },
    {
        title: "signals",
        artist: "Bcalm x Kainbeats",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/8. Bcalm _ Kainbeats - signals.mp3",
        forPart: AllPartType
    },
    {
        title: "daisy",
        artist: "Bcalm x Banks x Purrple Cat",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/9. Bcalm _ Banks _ Purrple Cat - daisy.mp3",
        forPart: AllPartType
    },
    {
        title: "hope",
        artist: "Bcalm x Purrple Cat",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/10. Bcalm _ Purrple Cat - hope.mp3",
        forPart: AllPartType
    },
    {
        title: "your eyes",
        artist: "Bcalm x Banks x No Spirit",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/11. Bcalm _ Banks _ No Spirit - your eyes.mp3",
        forPart: AllPartType
    },
    {
        title: "time",
        artist: "Bcalm x Hendy",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/12. Bcalm _ Hendy - time.mp3",
        forPart: AllPartType
    },
    {
        title: "traces",
        artist: "Bcalm x cxl",
        file: "public/audio/lofigirl/MP3 - Bcalm - Elements/13. Bcalm _ cxlt - traces.mp3",
        forPart: AllPartType
    },
    // "At Long Last",
    {
        title: "from me to you",
        artist: "towerz x edelwize x kokoro",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/1. towerz _ edelwize ft. kokoro - from me to you.mp3",
        forPart: AllPartType
    },
    {
        title: "day by day",
        artist: "towerz x edelwize x spencer hunt",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/2. towerz _ edelwize ft. spencer hunt - day by day.mp3",
        forPart: AllPartType
    },
    {
        title: "reckless",
        artist: "towerz x edelwize",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/3. towerz _ edelwize - reckless.mp3",
        forPart: AllPartType
    },
    {
        title: "tomorrow never came",
        artist: "towerz x edelwize",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/4. towerz _ edelwize - tomorrow never came.mp3",
        forPart: AllPartType
    },
    {
        title: "in the cold",
        artist: "towerz x edelwize x spencer hunt",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/5. towerz _ edelwize ft. spencer hunt - in the cold.mp3",
        forPart: AllPartType
    },
    {
        title: "trusting hands",
        artist: "towerz x edelwize",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/6. towerz _ edelwize - trusting hands.mp3",
        forPart: AllPartType
    },
    {
        title: "at long last",
        artist: "towerz x edelwize x umbriel ",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/7. towerz _ edelwize ft. umbriel - at long last.mp3",
        forPart: AllPartType
    },
    {
        title: "mayflower",
        artist: "towerz x edelwize",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/8. towerz _ edelwize - mayflower.mp3",
        forPart: AllPartType
    },
    {
        title: "sandscape",
        artist: "towerz x edelwize",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/9. towerz _ edelwize - sandscape.mp3",
        forPart: AllPartType
    },
    {
        title: "soft hands",
        artist: "towerz x edelwize x  jhove",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/10. towerz _ edelwize ft. jhove - soft hands.mp3",
        forPart: AllPartType
    },
    {
        title: "follow me",
        artist: "towerz x edelwize",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/11. towerz _ edelwize - follow me.mp3",
        forPart: AllPartType
    },
    {
        title: "folding house",
        artist: "towerz x edelwize x hi jude",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/12. towerz _ edelwize ft. hi jude - folding house.mp3",
        forPart: AllPartType
    },
    {
        title: "to fall",
        artist: "towerz x edelwize",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/13. towerz _ edelwize - to fall.mp3",
        forPart: AllPartType
    },
    {
        title: "channel 67",
        artist: "towerz x edelwize",
        file: "public/audio/lofigirl/MP3 - Towerz - At Long Last/14. towerz _ edelwize - channel 67.mp3",
        forPart: AllPartType
    },
    // "Blue Hour",
    {
        title: "Places",
        artist: "ENRA x dr. niar",
        file: "public/audio/lofigirl/MP3 - ENRA x dr. niar - Blue Hour/01 - Places.mp3",
        forPart: AllPartType
    },
    {
        title: "Azure",
        artist: "ENRA x dr. niar",
        file: "public/audio/lofigirl/MP3 - ENRA x dr. niar - Blue Hour/02 - Azure.mp3",
        forPart: AllPartType
    },
    {
        title: "Ballads",
        artist: "ENRA x dr. niar",
        file: "public/audio/lofigirl/MP3 - ENRA x dr. niar - Blue Hour/03 - Ballads.mp3",
        forPart: AllPartType
    },
    {
        title: "Velvet",
        artist: "ENRA x dr. niar",
        file: "public/audio/lofigirl/MP3 - ENRA x dr. niar - Blue Hour/04 - Velvet.mp3",
        forPart: AllPartType
    },
    {
        title: "Autumn",
        artist: "ENRA x dr. niar",
        file: "public/audio/lofigirl/MP3 - ENRA x dr. niar - Blue Hour/05 - Autumn.mp3",
        forPart: AllPartType
    },
    {
        title: "Old Feelings",
        artist: "ENRA x dr. niar",
        file: "public/audio/lofigirl/MP3 - ENRA x dr. niar - Blue Hour/06 - Old Feelings.mp3",
        forPart: AllPartType
    },
    {
        title: "Anywhere But Here",
        artist: "ENRA x dr. niar",
        file: "public/audio/lofigirl/MP3 - ENRA x dr. niar - Blue Hour/07 - Anywhere But Here.mp3",
        forPart: AllPartType
    },
    {
        title: "Let Go",
        artist: "ENRA x dr. niar",
        file: "public/audio/lofigirl/MP3 - ENRA x dr. niar - Blue Hour/08 - Let Go.mp3",
        forPart: AllPartType
    },
    // "Scenery",
    {
        title: "Scenery",
        artist: "Tibeauthetraveler x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraveler x lawrence - Scenery/1. Scenery (master).mp3",
        forPart: AllPartType
    },
    {
        title: "Childlike Wonder",
        artist: "Tibeauthetraveler x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraveler x lawrence - Scenery/2. Childlike Wonder (master).mp3",
        forPart: AllPartType
    },
    {
        title: "Longing",
        artist: "Tibeauthetraveler x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraveler x lawrence - Scenery/3. Longing (master).mp3",
        forPart: AllPartType
    },
    {
        title: "Holiday",
        artist: "Tibeauthetraveler x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraveler x lawrence - Scenery/4. Holiday (master).mp3",
        forPart: AllPartType
    },
    {
        title: "Memory Lane",
        artist: "Tibeauthetraveler x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraveler x lawrence - Scenery/5. Memory Lane (master).mp3",
        forPart: AllPartType
    },
    {
        title: "Neighbordhood",
        artist: "Tibeauthetraveler x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraveler x lawrence - Scenery/6. Neighborhood (master).mp3",
        forPart: AllPartType
    },
    {
        title: "Whispers",
        artist: "Tibeauthetraveler x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraveler x lawrence - Scenery/7. Whispers (test master).mp3",
        forPart: AllPartType
    },
    {
        title: "Soft Breeze",
        artist: "Tibeauthetraveler x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraveler x lawrence - Scenery/8. Soft Breeze (master).mp3",
        forPart: AllPartType
    },
    {
        title: "Liberate",
        artist: "Tibeauthetraveler x Lawrence Walther",
        file: "public/audio/lofigirl/MP3 - Tibeauthetraveler x lawrence - Scenery/9. Liberate (master).mp3",
        forPart: AllPartType
    },
    // "Belonging",
    {
        title: "Horizon",
        artist: "Laffey x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Forest Friends",
        artist: "Laffey x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Willow",
        artist: "Laffey x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Belonging",
        artist: "Laffey x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Patience",
        artist: "Laffey x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Teardrops",
        artist: "Laffey x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Beyond",
        artist: "Laffey x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    {
        title: "Gentle",
        artist: "Laffey x softy",
        file: "xxxxxxxxxxxx",
        forPart: AllPartType
    },
    // "Ballerina",
    {
        title: "Ballerina",
        artist: "Epona",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/1. Ballerina.mp3",
        forPart: AllPartType
    },
    {
        title: "Waterfall",
        artist: "Epona x Elijah Lee",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/2. Waterfall (with Elijah Lee).mp3",
        forPart: AllPartType
    },
    {
        title: "Just Another Day",
        artist: "Epona",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/3. Just Another Day.mp3",
        forPart: AllPartType
    },
    {
        title: "Meditations",
        artist: "Epona",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/4. Meditations.mp3",
        forPart: AllPartType
    },
    {
        title: "Daisies",
        artist: "Epona x Phlocalyst",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/5. Daisies (with Phlocalyst).mp3",
        forPart: AllPartType
    },
    {
        title: "Wandering",
        artist: "Epona x Epifania",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/6. Wandering (with Epifania).mp3",
        forPart: AllPartType
    },
    {
        title: "Misty",
        artist: "Epona",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/7. Misty.mp3",
        forPart: AllPartType
    },
    {
        title: "Monday",
        artist: "Epona x Sebastian Kamae",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/8. Monday (with Sebastian Kamae).mp3",
        forPart: AllPartType
    },
    {
        title: "Rainfall",
        artist: "Epona",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/9. Rainfall.mp3",
        forPart: AllPartType
    },
    {
        title: "Strangers",
        artist: "Epona",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/10. Strangers.mp3",
        forPart: AllPartType
    },
    {
        title: "Moonlight",
        artist: "Epona",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/11. Moonlight.mp3",
        forPart: AllPartType
    },
    {
        title: "My Ocean",
        artist: "Epona",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/12. My Ocean.mp3",
        forPart: AllPartType
    },
    {
        title: "Matcha",
        artist: "Epona x Ruby",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/13. Matcha (with Ruby).mp3",
        forPart: AllPartType
    },
    {
        title: "Grounding",
        artist: "Epona x Epifania",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/14. Grounding (with Epifania).mp3",
        forPart: AllPartType
    },
    {
        title: "Drifter",
        artist: "Epona",
        file: "public/audio/lofigirl/MP3 - Epona - Ballerina/15. Drifter.mp3",
        forPart: AllPartType
    },
    // "finding comfort",
    {
        title: "snowfall",
        artist: "Blurred Figures x another silent weekend",
        file: "public/audio/lofigirl/MP3 - Blurred figured x another silent weekend - finding comfort/snowfall.mp3",
        forPart: AllPartType
    },
    {
        title: "take it easy",
        artist: "Blurred Figures x another silent weekend",
        file: "public/audio/lofigirl/MP3 - Blurred figured x another silent weekend - finding comfort/take it easy (mastered).mp3",
        forPart: AllPartType
    },
    {
        title: "ready when you are",
        artist: "Blurred Figures x another silent weekend x fourwalls",
        file: "public/audio/lofigirl/MP3 - Blurred figured x another silent weekend - finding comfort/ready when you are (ft. fourwalls).mp3",
        forPart: AllPartType
    },
    {
        title: "beige palette",
        artist: "Blurred Figures x another silent weekend",
        file: "public/audio/lofigirl/MP3 - Blurred figured x another silent weekend - finding comfort/beige palette (mastered).mp3",
        forPart: AllPartType
    },
    {
        title: "i’m with you",
        artist: "Blurred Figures x another silent weekend",
        file: "public/audio/lofigirl/MP3 - Blurred figured x another silent weekend - finding comfort/im with you.mp3",
        forPart: AllPartType
    },
    {
        title: "no worries",
        artist: "Blurred Figures x another silent weekend",
        file: "public/audio/lofigirl/MP3 - Blurred figured x another silent weekend - finding comfort/no worries (mastered).mp3",
        forPart: AllPartType
    },
    {
        title: "everything goes past",
        artist: "Blurred Figures x another silent weekend",
        file: "public/audio/lofigirl/MP3 - Blurred figured x another silent weekend - finding comfort/everything goes past.mp3",
        forPart: AllPartType
    },
    {
        title: "kermode",
        artist: "Blurred Figures x another silent weekend",
        file: "public/audio/lofigirl/MP3 - Blurred figured x another silent weekend - finding comfort/kermode.mp3",
        forPart: AllPartType
    },
    {
        title: "goodnight",
        artist: "Blurred Figures x another silent weekend",
        file: "public/audio/lofigirl/MP3 - Blurred figured x another silent weekend - finding comfort/goodnight.mp3",
        forPart: AllPartType
    },
    // "Sleepovers",
    {
        title: "Cozy Cuddles",
        artist: "LESKY x Sitting Duck x Waywell",
        file: "public/audio/lofigirl/MP3 - LESKY - Sleepovers/01 Cozy Cuddles (ft. Sitting Duck _ Waywell).mp3",
        forPart: AllPartType
    },
    {
        title: "White Sheets",
        artist: "LESKY x Phlocalyst x Waywell",
        file: "public/audio/lofigirl/MP3 - LESKY - Sleepovers/02 White Sheets (ft. Phlocalyst _ Waywell).mp3",
        forPart: AllPartType
    },
    {
        title: "Bathroom Marble",
        artist: "LESKY x  Akin x Cuebe",
        file: "public/audio/lofigirl/MP3 - LESKY - Sleepovers/03 Bathroom Marble (ft. Akin _ Cuebe).mp3",
        forPart: AllPartType
    },
    {
        title: "Midas",
        artist: "LESKY x Waywell",
        file: "public/audio/lofigirl/MP3 - LESKY - Sleepovers/04 Midas (ft. Waywell).mp3",
        forPart: AllPartType
    },
    {
        title: "Nightingale",
        artist: "LESKY x M E A D O W x Mowlvoorph",
        file: "public/audio/lofigirl/MP3 - LESKY - Sleepovers/05 Nightingale (ft. M e a d o w _ Mowlvoorph).mp3",
        forPart: AllPartType
    },
    {
        title: "Dawnstar",
        artist: "LESKY x Waywell x Phlocalyst x Sátyr",
        file: "public/audio/lofigirl/MP3 - LESKY - Sleepovers/06 Dawnstar (ft. Waywell _ Phlocalyst _ Sátyr).mp3",
        forPart: AllPartType
    },
    // "Moments To Keep",
    {
        title: "Blossom",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Blossom x Nowun.mp3",
        forPart: AllPartType
    },
    {
        title: "Inhale",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Inhale x Nowun.mp3",
        forPart: AllPartType
    },
    {
        title: "Keep You Safe",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Keep You Safe x Nowun.mp3",
        forPart: AllPartType
    },
    {
        title: "Haze",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Haze x Nowun.mp3",
        forPart: AllPartType
    },
    {
        title: "Say When",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Say When x Nowun.mp3",
        forPart: AllPartType
    },
    {
        title: "Morning Sun",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Morning Sun x Nowun.mp3",
        forPart: AllPartType
    },
    {
        title: "Late Signs",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Late Signs x Nowun.mp3",
        forPart: AllPartType
    },
    {
        title: "Silhouette",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Silhouettes x Nowun.mp3",
        forPart: AllPartType
    },
    {
        title: "Just Wait",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Just Wait x Nowun.mp3",
        forPart: AllPartType
    },
    {
        title: "Everything We Need",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Everything We Need.mp3",
        forPart: AllPartType
    },
    {
        title: "Moments To Keep",
        artist: "Hoogway x Nowun",
        file: "public/audio/lofigirl/MP3 - Hoogway x Nowun - Moments To Keep/Moments To Keep.mp3",
        forPart: AllPartType
    },
    // "Silent Emotions", 音源のダウンロードリンクなし
    // {
    //   title: "Nostalgic",
    //   artist: "Dimension 32 x L’Outlander",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Mindfulness",
    //   artist: "Dimension 32 x Hoogway x Bhxa",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Point Me Home",
    //   artist: "Dimension 32 x  cxlt.",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Raining In My Head",
    //   artist: "Dimension 32 x Banks",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Nuit Blanche",
    //   artist: "Dimension 32 x L’Outlander",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Silent Emotions",
    //   artist: "Dimension 32 x Softy",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Amnesia",
    //   artist: "Dimension 32 x Hevi",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Lunar Rotations",
    //   artist: "Dimension 32 x S N U G",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // {
    //   title: "Night Vision",
    //   artist: "Dimension 32 x Hevi x Bhxa",
    //   file: "xxxxxxxxxxxx",
    //   forPart: AllPartType
    // },
    // "When I’m Gone",
    {
        title: "Ethereal",
        artist: "Hevi x H.1",
        file: "public/audio/lofigirl/MP3 - Hevi - When i'm Gone/1) Ethereal (feat H.1).mp3",
        forPart: AllPartType
    },
    {
        title: "Above Skies",
        artist: "Hevi x H.1",
        file: "public/audio/lofigirl/MP3 - Hevi - When i'm Gone/2) Above Skies (feat. H.1).mp3",
        forPart: AllPartType
    },
    {
        title: "Don’t Hurt Yourself",
        artist: "Hevi",
        file: "public/audio/lofigirl/MP3 - Hevi - When i'm Gone/3) Don_t Hurt Yourself.mp3",
        forPart: AllPartType
    },
    {
        title: "When I’m Gone",
        artist: "Hevi",
        file: "public/audio/lofigirl/MP3 - Hevi - When i'm Gone/4) When I_m Gone.mp3",
        forPart: AllPartType
    },
    {
        title: "Dream On",
        artist: "Hevi x Casiio",
        file: "public/audio/lofigirl/MP3 - Hevi - When i'm Gone/5) Dream On (feat. Casiio).mp3",
        forPart: AllPartType
    },
    {
        title: "I’m Sorry",
        artist: "Hevi",
        file: "public/audio/lofigirl/MP3 - Hevi - When i'm Gone/6) I_m Sorry.mp3",
        forPart: AllPartType
    },
    {
        title: "Closer",
        artist: "Hevi",
        file: "public/audio/lofigirl/MP3 - Hevi - When i'm Gone/7) Closer.mp3",
        forPart: AllPartType
    },
    {
        title: "Frames",
        artist: "Hevi",
        file: "public/audio/lofigirl/MP3 - Hevi - When i'm Gone/8) Frames.mp3",
        forPart: AllPartType
    },
    {
        title: "Blurry",
        artist: "Hevi",
        file: "public/audio/lofigirl/MP3 - Hevi - When i'm Gone/9) Blurry.mp3",
        forPart: AllPartType
    },
    // "Seeing Beauty in Everything",
    {
        title: "Exchange",
        artist: "Ky akasha",
        file: "public/audio/lofigirl/MP3 - Ky akasha - Seeing Beauty in Everything/1 Exchange.mp3",
        forPart: AllPartType
    },
    {
        title: "Stellar Wind",
        artist: "Ky akasha",
        file: "public/audio/lofigirl/MP3 - Ky akasha - Seeing Beauty in Everything/2 Stellar Wind.mp3",
        forPart: AllPartType
    },
    {
        title: "Seeing Beauty in Everything",
        artist: "Ky akasha",
        file: "public/audio/lofigirl/MP3 - Ky akasha - Seeing Beauty in Everything/3 Seeing Beauty in Everything.mp3",
        forPart: AllPartType
    },
    {
        title: "Vide",
        artist: "Ky akasha",
        file: "public/audio/lofigirl/MP3 - Ky akasha - Seeing Beauty in Everything/4 Vide.mp3",
        forPart: AllPartType
    },
    {
        title: "Last Resort",
        artist: "Ky akasha",
        file: "public/audio/lofigirl/MP3 - Ky akasha - Seeing Beauty in Everything/5 Last Resort.mp3",
        forPart: AllPartType
    },
    {
        title: "Epok",
        artist: "Ky akasha",
        file: "public/audio/lofigirl/MP3 - Ky akasha - Seeing Beauty in Everything/6 Epok.mp3",
        forPart: AllPartType
    },
    // "Layover",
    {
        title: "we’ll be waiting a while",
        artist: "S N U G x tender spring x Rook1e",
        file: "/audio/lofigirl/MP3 - S N U G - Layover/we_ll be waiting a while ft. Rook1e _ tender spring.mp3",
        forPart: AllPartType
    },
    {
        title: "notion",
        artist: "S N U G",
        file: "/audio/lofigirl/MP3 - S N U G - Layover/bloom.mp3",
        forPart: AllPartType
    },
    {
        title: "autumn warmth",
        artist: "S N U G x Rook1e",
        file: "/audio/lofigirl/MP3 - S N U G - Layover/autumn warmth ft. Rook1e.mp3",
        forPart: AllPartType
    },
    {
        title: "leap of faith",
        artist: "S N U G",
        file: "/audio/lofigirl/MP3 - S N U G - Layover/leap of faith.mp3",
        forPart: AllPartType
    },
    {
        title: "bloom",
        artist: "S N U G",
        file: "/audio/lofigirl/MP3 - S N U G - Layover/bloom.mp3",
        forPart: AllPartType
    },
    {
        title: "drifting into the sunset",
        artist: "S N U G x Mondo Loops x Rook1e",
        file: "/audio/lofigirl/MP3 - S N U G - Layover/drifting into the sunset ft. Rook1e _ Mondo Loops.mp3",
        forPart: AllPartType
    },
    {
        title: "hazelnut",
        artist: "S N U G",
        file: "/audio/lofigirl/MP3 - S N U G - Layover/hazelnut.mp3",
        forPart: AllPartType
    },
    {
        title: "convo",
        artist: "S N U G",
        file: "/audio/lofigirl/MP3 - S N U G - Layover/convo.mp3",
        forPart: AllPartType
    },
    // "A World After",
    {
        title: "Germination",
        artist: "Krynoze x aMess",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/01 - Germination (ft. Amess).mp3",
        forPart: AllPartType
    },
    {
        title: "Submarine Embrace",
        artist: "Krynoze x Hoogway",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/02 - Submarine Embrace (ft. Hoogway).mp3",
        forPart: AllPartType
    },
    {
        title: "Sediments",
        artist: "Krynoze x Dimension 32",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/03 - Sediments (ft. Dimension 32).mp3",
        forPart: AllPartType
    },
    {
        title: "Unfamiliar Beds",
        artist: "Krynoze x Hoogway",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/04 - Unfamiliar Beds (ft. Hoogway).mp3",
        forPart: AllPartType
    },
    {
        title: "Crackling Woods",
        artist: "Krynoze x Goson",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/05 - Crackling Woods (ft. Goson).mp3",
        forPart: AllPartType
    },
    {
        title: "Reins",
        artist: "Krynoze x Sweet Medicine",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/06 - Reins (ft. Sweet Medicine).mp3",
        forPart: AllPartType
    },
    {
        title: "Breaking Dawn",
        artist: "Krynoze x Tibeauthetraveler",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/07 - Breaking Dawn (ft. Tibeauthetraveler).mp3",
        forPart: AllPartType
    },
    {
        title: "Turnings",
        artist: "Krynoze x WYS",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/08 - Turnings (ft. WYS).mp3",
        forPart: AllPartType
    },
    {
        title: "Drippin' Love",
        artist: "Krynoze x Slowheal",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/09 - Drippin_ Love (ft. Slowheal).mp3",
        forPart: AllPartType
    },
    {
        title: "Reverie",
        artist: "Krynoze x Tibeauthetraveler",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/10 - Reverie (ft. Tibeauthetraveler).mp3",
        forPart: AllPartType
    },
    {
        title: "Ripples",
        artist: "Krynoze x Devon Rea",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/11 - Ripples (ft. Devon Rea).mp3",
        forPart: AllPartType
    },
    {
        title: "Blooming Dales",
        artist: "Krynoze x Diiolme",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/12 - Blooming Dales (ft. Diiolme).mp3",
        forPart: AllPartType
    },
    {
        title: "Homecoming",
        artist: "Krynoze x Matchbox Youth",
        file: "/audio/lofigirl/MP3 - Krynoze - A World After/13 - Homecoming (ft. Matchbox Youth).mp3",
        forPart: AllPartType
    }

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

