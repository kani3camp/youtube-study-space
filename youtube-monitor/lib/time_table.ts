// const SectionType = {
//     Study: 'study',
//     Break: 'break',
// } as const

// export type SectionType = typeof SectionType[keyof typeof SectionType]

export const SectionType = {
    Study: 'study',
    Break: 'break'
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
    partName: string
}

export function getCurrentSection(): TimeSection | null {
    const now: Date = new Date()
    for (const section of TimeTable) {
        // TODO: 日付またがる時の場合
        const startsDate: Date = new Date(
            now.getFullYear(), 
            now.getMonth(), 
            now.getDate(), 
            section.starts.h,
            section.starts.m
        )
        const endsDate: Date = new Date(
            now.getFullYear(),
            now.getMonth(),
            now.getDate(),
            section.ends.h,
            section.ends.m
        )
        if (startsDate <= now && now < endsDate) {
            return section
        }
    }
    console.error('no current section.')
    return null
}

export function remainingTime(currentHours: number, currentMinutes: number, destHours: number, destMinutes: number): number {
    if (currentHours === destHours) {
        return destMinutes - currentMinutes
    } else if (currentHours < destHours) {
        const diffHours: number = destHours - currentHours
        return 60*(diffHours - 1) + (60 - currentMinutes) + destMinutes
    } else {    // 日付を跨いでいる
        return 60*(23 - currentHours) + (60 - currentMinutes) + 60*destHours + destMinutes
    }
}
  
  
  
const TimeTable: TimeSection[] = [
    {
        starts: {h: 0, m: 5},
        ends: {h: 0, m: 25},
        sectionType: SectionType.Break,
        sectionId: ,
        partName: '深夜パートⅠ'
    },
    {
        starts: {h: 0, m: 25},
        ends: {h: 0, m: 50},
        sectionType: SectionType.Study,
        sectionId: 
    },
    {
        starts: {h: 0, m: 50},
        ends: {h: 0, m: 55},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 0, m: 55},
        ends: {h: 1, m: 20},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 1, m: 20},
        ends: {h: 1, m: 25},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 1, m: 25},
        ends: {h: 1, m: 50},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 1, m: 50},
        ends: {h: 1, m: 55},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 1, m: 55},
        ends: {h: 2, m: 20},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 2, m: 20},
        ends: {h: 2, m: 40},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 2, m: 40},
        ends: {h: 3, m: 5},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 3, m: 5},
        ends: {h: 3, m: 10},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 3, m: 10},
        ends: {h: 3, m: 35},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 3, m: 35},
        ends: {h: 3, m: 40},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 3, m: 40},
        ends: {h: 4, m: 5},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 4, m: 5},
        ends: {h: 4, m: 10},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 4, m: 10},
        ends: {h: 4, m: 35},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 4, m: 35},
        ends: {h: 4, m: 55},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 4, m: 55},
        ends: {h: 5, m: 20},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 5, m: 20},
        ends: {h: 5, m: 25},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 5, m: 25},
        ends: {h: 5, m: 50},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 5, m: 50},
        ends: {h: 5, m: 55},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 5, m: 55},
        ends: {h: 6, m: 20},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 6, m: 20},
        ends: {h: 6, m: 25},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 6, m: 25},
        ends: {h: 6, m: 50},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 6, m: 50},
        ends: {h: 7, m: 0},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 7, m: 0},
        ends: {h: 7, m: 25},
        sectionType: SectionType.Study,
        sectionId: 1,
        partName: '朝パート'
    },
    {
        starts: {h: 7, m: 25},
        ends: {h: 7, m: 30},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 7, m: 30},
        ends: {h: 7, m: 55},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 7, m: 55},
        ends: {h: 8, m: 0},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 8, m: 0},
        ends: {h: 8, m: 25},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 8, m: 25},
        ends: {h: 8, m: 30},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 8, m: 30},
        ends: {h: 8, m: 55},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 8, m: 55},
        ends: {h: 9, m: 15},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 9, m: 15},
        ends: {h: 9, m: 40},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 9, m: 40},
        ends: {h: 9, m: 45},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 9, m: 45},
        ends: {h: 10, m: 10},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 10, m: 10},
        ends: {h: 10, m: 15},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 10, m: 15},
        ends: {h: 10, m: 40},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 10, m: 40},
        ends: {h: 10, m: 45},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 10, m: 45},
        ends: {h: 11, m: 10},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 11, m: 10},
        ends: {h: 11, m: 30},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 11, m: 30},
        ends: {h: 11, m: 55},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 11, m: 55},
        ends: {h: 12, m: 0},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 12, m: 0},
        ends: {h: 12, m: 25},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 12, m: 25},
        ends: {h: 13, m: 0},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 13, m: 0},
        ends: {h: 13, m: 25},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 13, m: 25},
        ends: {h: 13, m: 30},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 13, m: 30},
        ends: {h: 13, m: 55},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 13, m: 55},
        ends: {h: 14, m: 0},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 14, m: 0},
        ends: {h: 14, m: 25},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 14, m: 25},
        ends: {h: 14, m: 30},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 14, m: 30},
        ends: {h: 14, m: 55},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 14, m: 55},
        ends: {h: 15, m: 15},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 15, m: 15},
        ends: {h: 15, m: 40},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 15, m: 40},
        ends: {h: 15, m: 45},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 15, m: 45},
        ends: {h: 16, m: 10},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 16, m: 10},
        ends: {h: 16, m: 15},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 16, m: 15},
        ends: {h: 16, m: 40},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 16, m: 40},
        ends: {h: 16, m: 45},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 16, m: 45},
        ends: {h: 17, m: 10},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 17, m: 10},
        ends: {h: 17, m: 40},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 17, m: 40},
        ends: {h: 18, m: 5},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 18, m: 5},
        ends: {h: 18, m: 10},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 18, m: 10},
        ends: {h: 18, m: 35},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 18, m: 35},
        ends: {h: 18, m: 40},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 18, m: 40},
        ends: {h: 19, m: 5},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 19, m: 5},
        ends: {h: 19, m: 10},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 19, m: 10},
        ends: {h: 19, m: 35},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 19, m: 35},
        ends: {h: 19, m: 55},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 19, m: 55},
        ends: {h: 20, m: 20},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 20, m: 20},
        ends: {h: 20, m: 25},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 20, m: 25},
        ends: {h: 20, m: 50},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 20, m: 50},
        ends: {h: 20, m: 55},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 20, m: 55},
        ends: {h: 21, m: 20},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 21, m: 20},
        ends: {h: 21, m: 25},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 21, m: 25},
        ends: {h: 21, m: 50},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 21, m: 50},
        ends: {h: 22, m: 10},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 22, m: 10},
        ends: {h: 22, m: 35},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 22, m: 35},
        ends: {h: 22, m: 40},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 22, m: 40},
        ends: {h: 23, m: 5},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 23, m: 10},
        ends: {h: 23, m: 35},
        sectionType: SectionType.Break
    },
    {
        starts: {h: 23, m: 35},
        ends: {h: 23, m: 40},
        sectionType: SectionType.Study
    },
    {
        starts: {h: 23, m: 40},
        ends: {h: 0, m: 5},
        sectionType: SectionType.Break
    },
]


export default {TimeTable}