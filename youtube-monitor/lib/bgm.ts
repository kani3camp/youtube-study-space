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
        file: '/audio/dova/Lo-Fi_Sunset.mp3',
        forPart: AllPartType,
    },
    {
        title: 'ノスタルジア',
        artist: 'こばっと さん',
        file: '/audio/dova/ノスタルジア_3.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Someday (Prod. Khaim)',
        artist: 'Khaim さん',
        file: '/audio/dova/Someday_(Prod._Khaim).mp3',
        forPart: AllPartType,
    },
    {
        title: 'You and Me',
        artist: 'しゃろう さん',
        file: '/audio/dova/You_and_Me_2.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Somebody (Prod. Khaim)',
        artist: 'Khaim さん',
        file: '/audio/dova/Somebody_(Prod._Khaim).mp3',
        forPart: AllPartType,
    },
    {
        title: '2:23 AM',
        artist: 'しゃろう さん',
        file: '/audio/dova/2_23_AM_2.mp3',
        forPart: [partType.MidNight1, partType.MidNight2],
    },
    {
        title: '10℃',
        artist: 'しゃろう さん',
        file: '/audio/dova/10℃_2.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Chilly',
        artist: 'Kyaai さん',
        file: '/audio/dova/Chilly_2.mp3',
        forPart: AllPartType,
    },
    {
        title: 'カフェの雨音。',
        artist: '夕焼けモンスター さん',
        file: '/audio/dova/カフェの雨音。_2.mp3',
        forPart: AllPartType,
    },
    {
        title: '黒ねこのボッサ',
        artist: 'れいな さん',
        file: '/audio/dova/黒ねこのボッサ.mp3',
        forPart: AllPartType,
    },
    {
        title: '午後のカフェ',
        artist: '高橋　志郎 さん',
        file: '/audio/dova/午後のカフェ.mp3',
        forPart: AllPartType,
    },
    {
        title: 'あの子のあだ名はピアノさん',
        artist: 'ネコト さん',
        file: '/audio/dova/あの子のあだ名はピアノさん.mp3',
        forPart: AllPartType,
    },
    {
        title: '東京は朝の七時',
        artist: 'ネコト さん',
        file: '/audio/dova/東京は朝の七時.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Somehow (Prod. Khaim)',
        artist: 'Khaim さん',
        file: '/audio/dova/Somehow_(Prod._Khaim).mp3',
        forPart: AllPartType,
    },
    {
        title: 'ローファイ図書委員',
        artist: 'ネコト さん',
        file: '/audio/dova/ローファイ図書委員.mp3',
        forPart: AllPartType,
    },
    {
        title: 'Stay On Your Mind',
        artist: 'Khaim さん',
        file: '/audio/dova/Stay_On_Your_Mind.mp3',
        forPart: AllPartType,
    },
    {
        title: 'RAINY GARDEN',
        artist: 'SAKURA BEATZ.JP さん',
        file: '/audio/dova/RAINY_GARDEN_2.mp3',
        forPart: AllPartType,
    },
    {
        title: '朝日溢れる回廊',
        artist: '畑中ゆう さん',
        file: '/audio/dova/朝日溢れる回廊_2.mp3',
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