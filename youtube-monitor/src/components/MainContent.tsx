import { FC, useEffect, useMemo, useState } from 'react'
import api from '../lib/api_config'
import fetcher from '../lib/fetcher'
import {
    allRooms,
    numSeatsInGeneralAllBasicRooms,
    numSeatsInMemberAllBasicRooms,
} from '../rooms/rooms-config'
import { mainContent } from '../styles/MainContent.styles'
import { Seat, SetDesiredMaxSeatsResponse } from '../types/api'
import { RoomLayout } from '../types/room-layout'
import CenterLoading from './CenterLoading'
import Message from './Message'
import SeatsPage, { LayoutPageProps } from './SeatsPage'

import { FirebaseOptions, initializeApp } from 'firebase/app'
import {
    collection,
    doc,
    DocumentData,
    FirestoreDataConverter,
    getFirestore,
    onSnapshot,
    query,
    QueryDocumentSnapshot,
    SnapshotOptions,
} from 'firebase/firestore'
import { useInterval } from '../lib/common'
import { Constants } from '../lib/constants'

const Seats: FC = () => {
    const PAGING_INTERVAL_MSEC = Constants.pagingIntervalSeconds * 1000

    const [latestGeneralSeats, setLatestGeneralSeats] = useState<Seat[]>([])
    const [latestMemberSeats, setLatestMemberSeats] = useState<Seat[]>([])
    const [latestGeneralMaxSeats, setLatestGeneralMaxSeats] = useState<number>()
    const [latestMemberMaxSeats, setLatestMemberMaxSeats] = useState<number>()
    const [latestMinVacancyRate, setLatestMinVacancyRate] = useState<number>()
    const [currentPageIndex, setCurrentPageIndex] = useState<number>(0)
    const [activeGeneralLayouts, setActiveGeneralLayouts] = useState<RoomLayout[]>([])
    const [activeMemberLayouts, setActiveMemberLayouts] = useState<RoomLayout[]>([])
    const [pageProps, setPageProps] = useState<LayoutPageProps[]>([])

    useEffect(() => {
        if (process.env.NEXT_PUBLIC_API_KEY === undefined) {
            alert('NEXT_PUBLIC_API_KEY is not defined')
        }

        initFirestore()
    }, [])

    useInterval(() => {
        console.log('interval', new Date())
        if (pageProps.length > 0) {
            const newPageIndex = (currentPageIndex + 1) % pageProps.length
            setCurrentPageIndex(newPageIndex)

            reviewMaxSeats()
        }
    }, PAGING_INTERVAL_MSEC)

    useEffect(() => {
        updatePageProps()
    }, [latestGeneralSeats, latestMemberSeats, activeGeneralLayouts, activeMemberLayouts])

    useEffect(() => {
        reviewMaxSeats()
    }, [latestGeneralMaxSeats, latestMemberMaxSeats, latestMinVacancyRate])

    useEffect(() => {
        changePage(currentPageIndex)
    }, [currentPageIndex])

    const initFirestore = () => {
        // check env variables
        if (process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID === undefined) {
            alert('NEXT_PUBLIC_FIREBASE_PROJECT_ID undefined.')
            return
        }
        if (process.env.NEXT_PUBLIC_FIREBASE_API_KEY === undefined) {
            alert('NEXT_PUBLIC_FIREBASE_API_KEY undefined.')
            return
        }

        const firebaseConfig: FirebaseOptions = {
            apiKey: process.env.NEXT_PUBLIC_FIREBASE_API_KEY,
            // authDomain: 'test--study-space.firebaseapp.com',
            projectId: process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID,
        }
        const app = initializeApp(firebaseConfig)
        const db = getFirestore(app)

        const seatConverter: FirestoreDataConverter<Seat> = {
            toFirestore(seat: Seat): DocumentData {
                return {
                    'seat-id': seat.seat_id,
                    'user-id': seat.user_id,
                    'user-display-name': seat.user_display_name,
                    'work-name': seat.work_name,
                    'break-work-name': seat.break_work_name,
                    'entered-at': seat.entered_at,
                    until: seat.until,
                    appearance: {
                        'color-code': seat.appearance.color_code1,
                        'num-stars': seat.appearance.num_stars,
                        'glow-animation': seat.appearance.color_gradient_enabled,
                    },
                    state: seat.state,
                    'current-state-started-at': seat.current_state_started_at,
                    'current-state-until': seat.current_state_until,
                    'cumulative-work-sec': seat.cumulative_work_sec,
                    'daily-cumulative-work-sec': seat.daily_cumulative_work_sec,
                }
            },
            fromFirestore(snapshot: QueryDocumentSnapshot, options: SnapshotOptions): Seat {
                const data = snapshot.data(options)
                return {
                    seat_id: data['seat-id'],
                    user_id: data['user-id'],
                    user_display_name: data['user-display-name'],
                    work_name: data['work-name'],
                    break_work_name: data['break-work-name'],
                    entered_at: data['entered-at'],
                    until: data.until,
                    appearance: {
                        color_code1: data.appearance['color-code1'],
                        color_code2: data.appearance['color-code2'],
                        num_stars: data.appearance['num-stars'],
                        color_gradient_enabled: data.appearance['color-gradient-enabled'],
                    },
                    state: data.state,
                    current_state_started_at: data['current-state-started-at'],
                    current_state_until: data['current-state-until'],
                    cumulative_work_sec: data['cumulative-work-sec'],
                    daily_cumulative_work_sec: data['daily-cumulative-work-sec'],
                    user_profile_image_url: data['user-profile-image-url'],
                }
            },
        }

        type Constants = {
            max_seats: number
            member_max_seats: number
            min_vacancy_rate: number
        }
        const constantsConverter: FirestoreDataConverter<Constants> = {
            toFirestore(constants: Constants): DocumentData {
                return {
                    'max-seats': constants.max_seats,
                    'member-max-seats': constants.member_max_seats,
                    'min-vacancy-rate': constants.min_vacancy_rate,
                }
            },
            fromFirestore(snapshot: QueryDocumentSnapshot, options: SnapshotOptions): Constants {
                const data = snapshot.data(options)
                return {
                    max_seats: data['max-seats'],
                    member_max_seats: data['member-max-seats'],
                    min_vacancy_rate: data['min-vacancy-rate'],
                }
            },
        }

        const generalSeatsQuery = query(collection(db, 'seats')).withConverter(seatConverter)
        onSnapshot(generalSeatsQuery, (querySnapshot) => {
            const seats: Seat[] = []
            querySnapshot.forEach((doc) => {
                seats.push(doc.data())
            })
            setLatestGeneralSeats(seats)
        })
        const memberSeatsQuery = query(collection(db, 'member-seats')).withConverter(seatConverter)
        onSnapshot(memberSeatsQuery, (querySnapshot) => {
            const seats: Seat[] = []
            querySnapshot.forEach((doc) => {
                seats.push(doc.data())
            })
            setLatestMemberSeats(seats)
        })

        onSnapshot(doc(db, 'config', 'constants').withConverter(constantsConverter), (doc) => {
            const generalMaxSeats = (doc.data() as Constants).max_seats
            const memberMaxSeats = (doc.data() as Constants).member_max_seats
            const minVacancyRate = (doc.data() as Constants).min_vacancy_rate
            console.log('max seats: ', generalMaxSeats)
            console.log('min vacancy rate: ', minVacancyRate)
            setLatestGeneralMaxSeats(generalMaxSeats)
            setLatestMemberMaxSeats(memberMaxSeats)
            setLatestMinVacancyRate(minVacancyRate)
        })
    }

    /**
     * 表示するページを変更する。
     * @param pageIndex 次に表示したいページのインデックス番号（0始まり）
     */
    const changePage = (pageIndex: number) => {
        console.log('change index: ', pageIndex)
        const snapshotPageProps = [...pageProps]
        if (pageIndex + 1 > snapshotPageProps.length) {
            pageIndex = 0 // index out of range にならないように１ページ目に。
        }
        const newPageProps: LayoutPageProps[] = snapshotPageProps.map((page, index) => {
            if (index === pageIndex) {
                page.display = true
            } else {
                page.display = false
            }
            return page
        })
        setPageProps(newPageProps)
    }

    const layoutPagesMemo = useMemo(
        () =>
            pageProps.map((pageProp, index) => (
                <SeatsPage
                    key={index}
                    firstSeatId={pageProp.firstSeatId}
                    roomLayout={pageProp.roomLayout}
                    usedSeats={pageProp.usedSeats}
                    display={pageProp.display}
                    memberOnly={pageProp.memberOnly}
                ></SeatsPage>
            )),
        [pageProps]
    )

    const reviewMaxSeats = async () => {
        const snapshotGeneralMaxSeats = latestGeneralMaxSeats
        const snapshotMemberMaxSeats = latestMemberMaxSeats
        const snapshotMinVacancyRate = latestMinVacancyRate
        const snapshotGeneralSeats = [...latestGeneralSeats]
        const snapshotMemberSeats = [...latestMemberSeats]
        if (
            snapshotGeneralMaxSeats === undefined ||
            snapshotMemberMaxSeats === undefined ||
            snapshotMinVacancyRate === undefined
        ) {
            return
        }
        console.log('reviewing max seats.')

        // まず、現状の入室状況（seatsとmax_seats）と設定された空席率（min_vacancy_rate）を基に、適切なmax_seatsを求める。
        let finalDesiredGeneralMaxSeats: number
        const generalMinSeatsByVacancyRate = Math.ceil(
            snapshotGeneralSeats.length / (1 - snapshotMinVacancyRate)
        )
        // もしmax_seatsが基本ルームの席数より多ければ、臨時ルームを増やす
        if (generalMinSeatsByVacancyRate > numSeatsInGeneralAllBasicRooms()) {
            let current_num_seats: number = numSeatsInGeneralAllBasicRooms()
            let current_adding_temporary_room_index = 0
            while (current_num_seats < generalMinSeatsByVacancyRate) {
                current_num_seats +=
                    allRooms.generalTemporaryRooms[current_adding_temporary_room_index].seats.length
                current_adding_temporary_room_index =
                    (current_adding_temporary_room_index + 1) %
                    allRooms.generalTemporaryRooms.length
            }
            finalDesiredGeneralMaxSeats = current_num_seats
        } else {
            // そうでなければ、基本ルームの席数とするべき
            finalDesiredGeneralMaxSeats = numSeatsInGeneralAllBasicRooms()
        }
        console.log(`desired: ${finalDesiredGeneralMaxSeats}, current: ${snapshotMemberMaxSeats}`)

        let finalDesiredMemberMaxSeats: number
        const memberMinSeatsByVacancyRate = Math.ceil(
            snapshotMemberSeats.length / (1 - snapshotMinVacancyRate)
        )
        // もしmax_seatsが基本ルームの席数より多ければ、臨時ルームを増やす
        if (memberMinSeatsByVacancyRate > numSeatsInMemberAllBasicRooms()) {
            let current_num_seats: number = numSeatsInMemberAllBasicRooms()
            let current_adding_temporary_room_index = 0
            while (current_num_seats < memberMinSeatsByVacancyRate) {
                current_num_seats +=
                    allRooms.memberTemporaryRooms[current_adding_temporary_room_index].seats.length
                current_adding_temporary_room_index =
                    (current_adding_temporary_room_index + 1) % allRooms.memberTemporaryRooms.length
            }
            finalDesiredMemberMaxSeats = current_num_seats
        } else {
            // そうでなければ、基本ルームの席数とするべき
            finalDesiredMemberMaxSeats = numSeatsInMemberAllBasicRooms()
        }
        console.log(
            `[member] desired: ${finalDesiredMemberMaxSeats}, current: ${snapshotMemberMaxSeats}`
        )

        // 求めたmax_seatsが現状の値と異なったら、リクエストを送る
        if (
            finalDesiredGeneralMaxSeats !== snapshotGeneralMaxSeats ||
            finalDesiredMemberMaxSeats !== snapshotMemberMaxSeats
        ) {
            console.log(
                'リクエストを送る',
                snapshotGeneralMaxSeats,
                ' => ',
                finalDesiredGeneralMaxSeats,
                snapshotMemberMaxSeats,
                ' => ',
                finalDesiredMemberMaxSeats
            )
            requestMaxSeatsUpdate(finalDesiredGeneralMaxSeats, finalDesiredMemberMaxSeats) // awaitはしない
        }

        // リクエストが送信されたら、すぐに反映されるわけではないのでとりあえずレイアウトを用意して表示する
        // 必要分（＝r.seatsにある席は全てカバーする）だけ臨時レイアウトを追加
        const nextGeneralLayouts: RoomLayout[] = [...allRooms.generalBasicRooms] // まずは基本ルームを設定
        if (snapshotGeneralMaxSeats > numSeatsInGeneralAllBasicRooms()) {
            let currentAddingLayoutIndex = 0
            while (numSeatsOfRoomLayouts(nextGeneralLayouts) < snapshotGeneralMaxSeats) {
                nextGeneralLayouts.push(allRooms.generalTemporaryRooms[currentAddingLayoutIndex])
                currentAddingLayoutIndex =
                    (currentAddingLayoutIndex + 1) % allRooms.generalTemporaryRooms.length
            }
        }
        const nextMemberLayouts: RoomLayout[] = [...allRooms.memberBasicRooms] // まずは基本ルームを設定
        if (snapshotMemberMaxSeats > numSeatsInMemberAllBasicRooms()) {
            let currentAddingLayoutIndex = 0
            while (numSeatsOfRoomLayouts(nextMemberLayouts) < snapshotMemberMaxSeats) {
                nextMemberLayouts.push(allRooms.memberTemporaryRooms[currentAddingLayoutIndex])
                currentAddingLayoutIndex =
                    (currentAddingLayoutIndex + 1) % allRooms.memberTemporaryRooms.length
            }
        }

        // TODO: レイアウト的にmaxSeatsより大きい番号の席が含まれそうであれば、それらの席は表示しない

        setActiveGeneralLayouts(nextGeneralLayouts)
        setActiveMemberLayouts(nextMemberLayouts)
    }

    const requestMaxSeatsUpdate = async (
        desiredGeneralMaxSeats: number,
        desiredMemberMaxSeats: number
    ) => {
        await fetcher<SetDesiredMaxSeatsResponse>(api.setDesiredMaxSeats, {
            method: 'POST',
            body: JSON.stringify({
                desired_max_seats: desiredGeneralMaxSeats,
                desired_member_max_seats: desiredMemberMaxSeats,
            }),
        }).then(async () => {
            console.log('リクエストした')
        })
    }

    const updatePageProps = () => {
        // 各項目のスナップショットをとる
        const snapshotPageProps = [...pageProps]
        const snapshotActiveGeneralLayouts = [...activeGeneralLayouts]
        const snapshotActiveMemberLayouts = [...activeMemberLayouts]
        const snapshotLatestGeneralSeats = [...latestGeneralSeats]
        const snapshotLatestMemberSeats = [...latestMemberSeats]

        if (snapshotPageProps.length < currentPageIndex + 1) {
            // index out of rangeにならないように1ページ目に。
            setCurrentPageIndex(0) // 反映はほんの少し遅延するが、ほんの少しなので視覚的にはすぐに回復するはず？
        }

        let sumSeatsGeneral = 0
        let sumSeatsMember = 0
        const mapFunc =
            (member_only: boolean) =>
            (layout: RoomLayout): LayoutPageProps => {
                const numSeats = layout.seats.length
                const firstSeatIdInLayout = member_only ? sumSeatsMember + 1 : sumSeatsGeneral + 1 // not index
                if (member_only) {
                    sumSeatsMember += numSeats
                } else {
                    sumSeatsGeneral += numSeats
                }
                const LastSeatIdInLayout = member_only ? sumSeatsMember : sumSeatsGeneral // not index
                const usedSeatsInLayout: Seat[] = (
                    member_only ? snapshotLatestMemberSeats : snapshotLatestGeneralSeats
                ).filter(
                    (seat) =>
                        firstSeatIdInLayout <= seat.seat_id && seat.seat_id <= LastSeatIdInLayout
                )

                return {
                    roomLayout: layout,
                    firstSeatId: firstSeatIdInLayout,
                    usedSeats: usedSeatsInLayout,
                    display: false, // set later
                    memberOnly: member_only,
                }
            }

        const newGeneralPageProps: LayoutPageProps[] = snapshotActiveGeneralLayouts.map(
            mapFunc(false)
        )
        const newMemberPageProps: LayoutPageProps[] = snapshotActiveMemberLayouts.map(mapFunc(true))
        const newPageProps: LayoutPageProps[] = newGeneralPageProps.concat(newMemberPageProps)
        // set if display
        for (let i = 0; i < newPageProps.length; i++) {
            if (snapshotPageProps.length === 0 && i === 0) {
                // 初回構築のときは1ページ目を表示
                newPageProps[i].display = true
            } else if (i >= snapshotPageProps.length) {
                // 増えたページの場合は、表示はfalse
                newPageProps[i].display = false
            } else {
                newPageProps[i].display = snapshotPageProps[i].display
            }
        }
        setPageProps(newPageProps)
    }

    /**
     * Number of seats of the given layouts.
     * @param layouts
     * @returns
     */
    const numSeatsOfRoomLayouts = (layouts: RoomLayout[]) => {
        let count = 0
        for (const layout of layouts) {
            count += layout.seats.length
        }
        return count
    }

    const messageMemo = useMemo(
        () => (
            <Message
                currentPageIndex={currentPageIndex}
                currentPagesLength={pageProps.length}
                currentPageIsMember={
                    pageProps.length > 0 ? pageProps[currentPageIndex].memberOnly : false
                }
                seats={latestGeneralSeats.concat(latestMemberSeats)}
            ></Message>
        ),
        [currentPageIndex, activeGeneralLayouts, latestGeneralSeats, latestMemberSeats, pageProps]
    )

    if (pageProps.length > 0) {
        return (
            <>
                <div css={mainContent}>
                    {layoutPagesMemo}
                    {messageMemo}
                </div>
            </>
        )
    } else {
        return <CenterLoading></CenterLoading>
    }
}

export default Seats
