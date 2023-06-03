export const SectionType = {
    Study: 'study',
    Break: 'break',
}

export type TimeSection = {
    starts: {
        h: number
        m: number
    }
    ends: {
        h: number
        m: number
    }
    sectionType: string
    sectionId: number
    partType: string
}

export function getCurrentSection(): TimeSection {
    // startsとendsの差は23時間未満とする
    const now: Date = new Date()
    for (const section of TimeTable) {
        const starts = section.starts
        const ends = section.ends

        // 適当に初期化
        let startsDate = now
        let endsDate = now
        endsDate.setDate(now.getDate() - 1)

        if (starts.h <= ends.h) {
            startsDate = new Date(
                now.getFullYear(),
                now.getMonth(),
                now.getDate(),
                section.starts.h,
                section.starts.m
            )
            endsDate = new Date(
                now.getFullYear(),
                now.getMonth(),
                now.getDate(),
                section.ends.h,
                section.ends.m
            )
        } else {
            // 日付またがる時の場合
            if (
                (starts.h == now.getHours() && starts.m <= now.getMinutes()) ||
                starts.h < now.getHours()
            ) {
                startsDate = new Date(
                    now.getFullYear(),
                    now.getMonth(),
                    now.getDate(),
                    starts.h,
                    starts.m
                )
                endsDate = new Date(
                    now.getFullYear(),
                    now.getMonth(),
                    now.getDate() + 1,
                    ends.h,
                    ends.m
                )
            } else if (
                now.getHours() < ends.h ||
                (now.getHours() == ends.h && now.getMinutes() < ends.m)
            ) {
                startsDate = new Date(
                    now.getFullYear(),
                    now.getMonth(),
                    now.getDate() - 1,
                    starts.h,
                    starts.m
                )
                endsDate = new Date(
                    now.getFullYear(),
                    now.getMonth(),
                    now.getDate(),
                    ends.h,
                    ends.m
                )
            }
        }
        if (startsDate <= now && now < endsDate) {
            return section
        }
    }
    console.error('no current section.')
    return TimeTable[0]
}

export function getNextSection(): TimeSection | null {
    const currentSection = getCurrentSection()
    if (currentSection !== null) {
        for (const section of TimeTable) {
            if (
                currentSection.ends.h === section.starts.h &&
                currentSection.ends.m === section.starts.m
            ) {
                return section
            }
        }
    }
    console.error('no next section.')
    return currentSection
}

export function remainingTime(
    currentHours: number,
    currentMinutes: number,
    destHours: number,
    destMinutes: number
): number {
    if (currentHours === destHours) {
        return destMinutes - currentMinutes
    } else if (currentHours < destHours) {
        const diffHours: number = destHours - currentHours
        return 60 * (diffHours - 1) + (60 - currentMinutes) + destMinutes
    } else {
        // 日付を跨いでいる
        return 60 * (23 - currentHours) + (60 - currentMinutes) + 60 * destHours + destMinutes
    }
}

export const PartType = {
    Morning: 'common:part_type.morning',
    BeforeNoon: 'common:part_type.before_noon',
    Noon: 'common:part_type.noon',
    AfterNoon1: 'common:part_type.after_noon1',
    AfterNoon2: 'common:part_type.after_noon2',
    Evening: 'common:part_type.evening',
    Night1: 'common:part_type.night1',
    Night2: 'common:part_type.night2',
    MidNight1: 'common:part_type.mid_night1',
    MidNight2: 'common:part_type.mid_night2',
    EarlyMorning: 'common:part_type.early_morning',
}

const TimeTable: TimeSection[] = [
    {
        starts: { h: 0, m: 5 },
        ends: { h: 0, m: 25 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Night2,
    },
    {
        starts: { h: 0, m: 25 },
        ends: { h: 0, m: 50 },
        sectionType: SectionType.Study,
        sectionId: 31,
        partType: PartType.MidNight1,
    },
    {
        starts: { h: 0, m: 50 },
        ends: { h: 0, m: 55 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.MidNight1,
    },
    {
        starts: { h: 0, m: 55 },
        ends: { h: 1, m: 20 },
        sectionType: SectionType.Study,
        sectionId: 32,
        partType: PartType.MidNight1,
    },
    {
        starts: { h: 1, m: 20 },
        ends: { h: 1, m: 25 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.MidNight1,
    },
    {
        starts: { h: 1, m: 25 },
        ends: { h: 1, m: 50 },
        sectionType: SectionType.Study,
        sectionId: 33,
        partType: PartType.MidNight1,
    },
    {
        starts: { h: 1, m: 50 },
        ends: { h: 1, m: 55 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.MidNight1,
    },
    {
        starts: { h: 1, m: 55 },
        ends: { h: 2, m: 20 },
        sectionType: SectionType.Study,
        sectionId: 34,
        partType: PartType.MidNight1,
    },
    {
        starts: { h: 2, m: 20 },
        ends: { h: 2, m: 40 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.MidNight1,
    },
    {
        starts: { h: 2, m: 40 },
        ends: { h: 3, m: 5 },
        sectionType: SectionType.Study,
        sectionId: 35,
        partType: PartType.MidNight2,
    },
    {
        starts: { h: 3, m: 5 },
        ends: { h: 3, m: 10 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.MidNight2,
    },
    {
        starts: { h: 3, m: 10 },
        ends: { h: 3, m: 35 },
        sectionType: SectionType.Study,
        sectionId: 36,
        partType: PartType.MidNight2,
    },
    {
        starts: { h: 3, m: 35 },
        ends: { h: 3, m: 40 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.MidNight2,
    },
    {
        starts: { h: 3, m: 40 },
        ends: { h: 4, m: 5 },
        sectionType: SectionType.Study,
        sectionId: 37,
        partType: PartType.MidNight2,
    },
    {
        starts: { h: 4, m: 5 },
        ends: { h: 4, m: 10 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.MidNight2,
    },
    {
        starts: { h: 4, m: 10 },
        ends: { h: 4, m: 35 },
        sectionType: SectionType.Study,
        sectionId: 38,
        partType: PartType.MidNight2,
    },
    {
        starts: { h: 4, m: 35 },
        ends: { h: 4, m: 55 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.MidNight2,
    },
    {
        starts: { h: 4, m: 55 },
        ends: { h: 5, m: 20 },
        sectionType: SectionType.Study,
        sectionId: 39,
        partType: PartType.EarlyMorning,
    },
    {
        starts: { h: 5, m: 20 },
        ends: { h: 5, m: 25 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.EarlyMorning,
    },
    {
        starts: { h: 5, m: 25 },
        ends: { h: 5, m: 50 },
        sectionType: SectionType.Study,
        sectionId: 40,
        partType: PartType.EarlyMorning,
    },
    {
        starts: { h: 5, m: 50 },
        ends: { h: 5, m: 55 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.EarlyMorning,
    },
    {
        starts: { h: 5, m: 55 },
        ends: { h: 6, m: 20 },
        sectionType: SectionType.Study,
        sectionId: 41,
        partType: PartType.EarlyMorning,
    },
    {
        starts: { h: 6, m: 20 },
        ends: { h: 6, m: 25 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.EarlyMorning,
    },
    {
        starts: { h: 6, m: 25 },
        ends: { h: 6, m: 50 },
        sectionType: SectionType.Study,
        sectionId: 42,
        partType: PartType.EarlyMorning,
    },
    {
        starts: { h: 6, m: 50 },
        ends: { h: 7, m: 0 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.EarlyMorning,
    },
    {
        starts: { h: 7, m: 0 },
        ends: { h: 7, m: 25 },
        sectionType: SectionType.Study,
        sectionId: 1,
        partType: PartType.Morning,
    },
    {
        starts: { h: 7, m: 25 },
        ends: { h: 7, m: 30 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Morning,
    },
    {
        starts: { h: 7, m: 30 },
        ends: { h: 7, m: 55 },
        sectionType: SectionType.Study,
        sectionId: 2,
        partType: PartType.Morning,
    },
    {
        starts: { h: 7, m: 55 },
        ends: { h: 8, m: 0 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Morning,
    },
    {
        starts: { h: 8, m: 0 },
        ends: { h: 8, m: 25 },
        sectionType: SectionType.Study,
        sectionId: 3,
        partType: PartType.Morning,
    },
    {
        starts: { h: 8, m: 25 },
        ends: { h: 8, m: 30 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Morning,
    },
    {
        starts: { h: 8, m: 30 },
        ends: { h: 8, m: 55 },
        sectionType: SectionType.Study,
        sectionId: 4,
        partType: PartType.Morning,
    },
    {
        starts: { h: 8, m: 55 },
        ends: { h: 9, m: 15 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Morning,
    },
    {
        starts: { h: 9, m: 15 },
        ends: { h: 9, m: 40 },
        sectionType: SectionType.Study,
        sectionId: 5,
        partType: PartType.BeforeNoon,
    },
    {
        starts: { h: 9, m: 40 },
        ends: { h: 9, m: 45 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.BeforeNoon,
    },
    {
        starts: { h: 9, m: 45 },
        ends: { h: 10, m: 10 },
        sectionType: SectionType.Study,
        sectionId: 6,
        partType: PartType.BeforeNoon,
    },
    {
        starts: { h: 10, m: 10 },
        ends: { h: 10, m: 15 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.BeforeNoon,
    },
    {
        starts: { h: 10, m: 15 },
        ends: { h: 10, m: 40 },
        sectionType: SectionType.Study,
        sectionId: 7,
        partType: PartType.BeforeNoon,
    },
    {
        starts: { h: 10, m: 40 },
        ends: { h: 10, m: 45 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.BeforeNoon,
    },
    {
        starts: { h: 10, m: 45 },
        ends: { h: 11, m: 10 },
        sectionType: SectionType.Study,
        sectionId: 8,
        partType: PartType.BeforeNoon,
    },
    {
        starts: { h: 11, m: 10 },
        ends: { h: 11, m: 30 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.BeforeNoon,
    },
    {
        starts: { h: 11, m: 30 },
        ends: { h: 11, m: 55 },
        sectionType: SectionType.Study,
        sectionId: 9,
        partType: PartType.Noon,
    },
    {
        starts: { h: 11, m: 55 },
        ends: { h: 12, m: 0 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Noon,
    },
    {
        starts: { h: 12, m: 0 },
        ends: { h: 12, m: 25 },
        sectionType: SectionType.Study,
        sectionId: 10,
        partType: PartType.Noon,
    },
    {
        starts: { h: 12, m: 25 },
        ends: { h: 13, m: 0 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Noon,
    },
    {
        starts: { h: 13, m: 0 },
        ends: { h: 13, m: 25 },
        sectionType: SectionType.Study,
        sectionId: 11,
        partType: PartType.AfterNoon1,
    },
    {
        starts: { h: 13, m: 25 },
        ends: { h: 13, m: 30 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.AfterNoon1,
    },
    {
        starts: { h: 13, m: 30 },
        ends: { h: 13, m: 55 },
        sectionType: SectionType.Study,
        sectionId: 12,
        partType: PartType.AfterNoon1,
    },
    {
        starts: { h: 13, m: 55 },
        ends: { h: 14, m: 0 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.AfterNoon1,
    },
    {
        starts: { h: 14, m: 0 },
        ends: { h: 14, m: 25 },
        sectionType: SectionType.Study,
        sectionId: 13,
        partType: PartType.AfterNoon1,
    },
    {
        starts: { h: 14, m: 25 },
        ends: { h: 14, m: 30 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.AfterNoon1,
    },
    {
        starts: { h: 14, m: 30 },
        ends: { h: 14, m: 55 },
        sectionType: SectionType.Study,
        sectionId: 14,
        partType: PartType.AfterNoon1,
    },
    {
        starts: { h: 14, m: 55 },
        ends: { h: 15, m: 15 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.AfterNoon1,
    },
    {
        starts: { h: 15, m: 15 },
        ends: { h: 15, m: 40 },
        sectionType: SectionType.Study,
        sectionId: 15,
        partType: PartType.AfterNoon2,
    },
    {
        starts: { h: 15, m: 40 },
        ends: { h: 15, m: 45 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.AfterNoon2,
    },
    {
        starts: { h: 15, m: 45 },
        ends: { h: 16, m: 10 },
        sectionType: SectionType.Study,
        sectionId: 16,
        partType: PartType.AfterNoon2,
    },
    {
        starts: { h: 16, m: 10 },
        ends: { h: 16, m: 15 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.AfterNoon2,
    },
    {
        starts: { h: 16, m: 15 },
        ends: { h: 16, m: 40 },
        sectionType: SectionType.Study,
        sectionId: 17,
        partType: PartType.AfterNoon2,
    },
    {
        starts: { h: 16, m: 40 },
        ends: { h: 16, m: 45 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.AfterNoon2,
    },
    {
        starts: { h: 16, m: 45 },
        ends: { h: 17, m: 10 },
        sectionType: SectionType.Study,
        sectionId: 18,
        partType: PartType.AfterNoon2,
    },
    {
        starts: { h: 17, m: 10 },
        ends: { h: 17, m: 40 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.AfterNoon2,
    },
    {
        starts: { h: 17, m: 40 },
        ends: { h: 18, m: 5 },
        sectionType: SectionType.Study,
        sectionId: 19,
        partType: PartType.Evening,
    },
    {
        starts: { h: 18, m: 5 },
        ends: { h: 18, m: 10 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Evening,
    },
    {
        starts: { h: 18, m: 10 },
        ends: { h: 18, m: 35 },
        sectionType: SectionType.Study,
        sectionId: 20,
        partType: PartType.Evening,
    },
    {
        starts: { h: 18, m: 35 },
        ends: { h: 18, m: 40 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Evening,
    },
    {
        starts: { h: 18, m: 40 },
        ends: { h: 19, m: 5 },
        sectionType: SectionType.Study,
        sectionId: 21,
        partType: PartType.Evening,
    },
    {
        starts: { h: 19, m: 5 },
        ends: { h: 19, m: 10 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Evening,
    },
    {
        starts: { h: 19, m: 10 },
        ends: { h: 19, m: 35 },
        sectionType: SectionType.Study,
        sectionId: 22,
        partType: PartType.Evening,
    },
    {
        starts: { h: 19, m: 35 },
        ends: { h: 19, m: 55 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Evening,
    },
    {
        starts: { h: 19, m: 55 },
        ends: { h: 20, m: 20 },
        sectionType: SectionType.Study,
        sectionId: 23,
        partType: PartType.Night1,
    },
    {
        starts: { h: 20, m: 20 },
        ends: { h: 20, m: 25 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Night1,
    },
    {
        starts: { h: 20, m: 25 },
        ends: { h: 20, m: 50 },
        sectionType: SectionType.Study,
        sectionId: 24,
        partType: PartType.Night1,
    },
    {
        starts: { h: 20, m: 50 },
        ends: { h: 20, m: 55 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Night1,
    },
    {
        starts: { h: 20, m: 55 },
        ends: { h: 21, m: 20 },
        sectionType: SectionType.Study,
        sectionId: 25,
        partType: PartType.Night1,
    },
    {
        starts: { h: 21, m: 20 },
        ends: { h: 21, m: 25 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Night1,
    },
    {
        starts: { h: 21, m: 25 },
        ends: { h: 21, m: 50 },
        sectionType: SectionType.Study,
        sectionId: 26,
        partType: PartType.Night1,
    },
    {
        starts: { h: 21, m: 50 },
        ends: { h: 22, m: 10 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Night2,
    },
    {
        starts: { h: 22, m: 10 },
        ends: { h: 22, m: 35 },
        sectionType: SectionType.Study,
        sectionId: 27,
        partType: PartType.Night2,
    },
    {
        starts: { h: 22, m: 35 },
        ends: { h: 22, m: 40 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Night2,
    },
    {
        starts: { h: 22, m: 40 },
        ends: { h: 23, m: 5 },
        sectionType: SectionType.Study,
        sectionId: 28,
        partType: PartType.Night2,
    },
    {
        starts: { h: 23, m: 5 },
        ends: { h: 23, m: 10 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Night2,
    },
    {
        starts: { h: 23, m: 10 },
        ends: { h: 23, m: 35 },
        sectionType: SectionType.Study,
        sectionId: 29,
        partType: PartType.Night2,
    },
    {
        starts: { h: 23, m: 35 },
        ends: { h: 23, m: 40 },
        sectionType: SectionType.Break,
        sectionId: 0,
        partType: PartType.Night2,
    },
    {
        starts: { h: 23, m: 40 },
        ends: { h: 0, m: 5 },
        sectionType: SectionType.Study,
        sectionId: 30,
        partType: PartType.Night2,
    },
]

export default { TimeTable }
