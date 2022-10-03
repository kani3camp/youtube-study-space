import { FC, useEffect, useState } from 'react'
import api from '../lib/api_config'
import { useInterval } from '../lib/common'
import fetcher from '../lib/fetcher'
import { allRooms, numSeatsInAllBasicRooms } from '../rooms/rooms-config'
import * as styles from '../styles/Room.styles'
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

const Seats: FC = () => {
    const PAGING_INTERVAL_MSEC = 8 * 1000

    const [latestSeats, setLatestSeats] = useState<Seat[]>([])
    const [latestMaxSeats, setLatestMaxSeats] = useState<number>()
    const [latestMinVacancyRate, setLatestMinVacancyRate] = useState<number>()
    const [currentPageIndex, setCurrentPageIndex] = useState<number>(0)
    const [usedLayouts, setUsedLayouts] = useState<RoomLayout[]>([])
    const [pageProps, setPageProps] = useState<LayoutPageProps[]>([])

    useEffect(() => {
        if (process.env.NEXT_PUBLIC_API_KEY === undefined) {
            alert('NEXT_PUBLIC_API_KEY is not defined')
        }

        initFirestore()
    }, [])

    useInterval(() => {
        if (usedLayouts.length > 0) {
            const newPageIndex = (currentPageIndex + 1) % usedLayouts.length
            setCurrentPageIndex(newPageIndex)
        }
    }, PAGING_INTERVAL_MSEC)

    useEffect(() => {
        updatePageProps()
    }, [latestSeats, usedLayouts])

    useEffect(() => {
        reviewMaxSeats()
    }, [latestMaxSeats, latestMinVacancyRate])

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
            authDomain: 'test--study-space.firebaseapp.com',
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
                        'color-code': seat.appearance.color_code,
                        'num-stars': seat.appearance.num_stars,
                        'glow-animation': seat.appearance.glow_animation,
                    },
                    state: seat.state,
                    'current-state-started-at': seat.current_state_started_at,
                    'current-state-until': seat.current_state_until,
                    'cumulative-work-sec': seat.cumulative_work_sec,
                    'daily-cumulative-work-sec': seat.daily_cumulative_work_sec,
                }
            },
            fromFirestore(
                snapshot: QueryDocumentSnapshot,
                options: SnapshotOptions
            ): Seat {
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
                        color_code: data.appearance['color-code'],
                        num_stars: data.appearance['num-stars'],
                        glow_animation: data.appearance['glow-animation'],
                    },
                    state: data.state,
                    current_state_started_at: data['current-state-started-at'],
                    current_state_until: data['current-state-until'],
                    cumulative_work_sec: data['cumulative-work-sec'],
                    daily_cumulative_work_sec:
                        data['daily-cumulative-work-sec'],
                }
            },
        }

        type Constants = {
            max_seats: number
            min_vacancy_rate: number
        }
        const constantsConverter: FirestoreDataConverter<Constants> = {
            toFirestore(constants: Constants): DocumentData {
                return {
                    'max-seats': constants.max_seats,
                    'min-vacancy-rate': constants.min_vacancy_rate,
                }
            },
            fromFirestore(
                snapshot: QueryDocumentSnapshot,
                options: SnapshotOptions
            ): Constants {
                const data = snapshot.data(options)
                return {
                    max_seats: data['max-seats'],
                    min_vacancy_rate: data['min-vacancy-rate'],
                }
            },
        }

        const seatsQuery = query(collection(db, 'seats')).withConverter(
            seatConverter
        )
        onSnapshot(seatsQuery, (querySnapshot) => {
            const seats: Seat[] = []
            querySnapshot.forEach((doc) => {
                seats.push(doc.data())
            })
            console.log('Current seats: ', seats)
            setLatestSeats(seats)
        })

        onSnapshot(
            doc(db, 'config', 'constants').withConverter(constantsConverter),
            (doc) => {
                const maxSeats = (doc.data() as Constants).max_seats
                const minVacancyRate = (doc.data() as Constants)
                    .min_vacancy_rate
                setLatestMaxSeats(maxSeats)
                setLatestMinVacancyRate(minVacancyRate)
            }
        )
    }

    /**
     * 表示するページを変更する。
     * @param pageIndex 次に表示したいページのインデックス番号（0始まり）
     */
    const changePage = (pageIndex: number) => {
        const snapshotPageProps = [...pageProps]
        if (pageIndex + 1 > snapshotPageProps.length) {
            pageIndex = 0 // index out of range にならないように１ページ目に。
        }
        const newPageProps: LayoutPageProps[] = snapshotPageProps.map(
            (page, index) => {
                if (index === pageIndex) {
                    page.display = true
                } else {
                    page.display = false
                }
                return page
            }
        )
        setPageProps(newPageProps)
    }

    const layoutPages = pageProps.map((pageProp, index) => (
        <SeatsPage
            key={index}
            firstSeatId={pageProp.firstSeatId}
            roomLayout={pageProp.roomLayout}
            usedSeats={pageProp.usedSeats}
            display={pageProp.display}
        ></SeatsPage>
    ))

    const reviewMaxSeats = async () => {
        const snapshotMaxSeats = latestMaxSeats
        const snapshotMinVacancyRate = latestMinVacancyRate
        const snapshotSeats = [...latestSeats]
        if (
            snapshotMaxSeats === undefined ||
            snapshotMinVacancyRate === undefined
        ) {
            return
        }
        console.log(reviewMaxSeats.name)

        // まず、現状の入室状況（seatsとmax_seats）と設定された空席率（min_vacancy_rate）を基に、適切なmax_seatsを求める。
        let finalDesiredMaxSeats: number
        const minSeatsByVacancyRate = Math.ceil(
            snapshotSeats.length / (1 - snapshotMinVacancyRate)
        )
        // もしmax_seatsが基本ルームの席数より多ければ、臨時ルームを増やす
        if (minSeatsByVacancyRate > numSeatsInAllBasicRooms()) {
            let current_num_seats: number = numSeatsInAllBasicRooms()
            let current_adding_temporary_room_index = 0
            while (current_num_seats < minSeatsByVacancyRate) {
                current_num_seats +=
                    allRooms.temporaryRooms[current_adding_temporary_room_index]
                        .seats.length
                current_adding_temporary_room_index =
                    (current_adding_temporary_room_index + 1) %
                    allRooms.temporaryRooms.length
            }
            finalDesiredMaxSeats = current_num_seats
        } else {
            // そうでなければ、基本ルームの席数とするべき
            finalDesiredMaxSeats = numSeatsInAllBasicRooms()
        }
        console.log(
            `desired: ${finalDesiredMaxSeats}, current: ${snapshotMaxSeats}`
        )

        // 求めたmax_seatsが現状の値と異なったら、リクエストを送る
        if (finalDesiredMaxSeats !== snapshotMaxSeats) {
            console.log(
                'リクエストを送る',
                finalDesiredMaxSeats,
                snapshotMaxSeats
            )
            requestMaxSeatsUpdate(finalDesiredMaxSeats) // awaitはしない
        }

        // リクエストが送信されたら、すぐに反映されるわけではないのでとりあえずレイアウトを用意して表示する
        // 必要分（＝r.seatsにある席は全てカバーする）だけ臨時レイアウトを追加
        const nextDisplayLayouts: RoomLayout[] = [...allRooms.basicRooms] // まずは基本ルームを設定
        if (snapshotMaxSeats > numSeatsInAllBasicRooms()) {
            let currentAddingLayoutIndex = 0
            while (
                numSeatsOfRoomLayouts(nextDisplayLayouts) < snapshotMaxSeats
            ) {
                nextDisplayLayouts.push(
                    allRooms.temporaryRooms[currentAddingLayoutIndex]
                )
                currentAddingLayoutIndex =
                    (currentAddingLayoutIndex + 1) %
                    allRooms.temporaryRooms.length
            }
        }

        // TODO: レイアウト的にmaxSeatsより大きい番号の席が含まれそうであれば、それらの席は表示しない

        setUsedLayouts(nextDisplayLayouts)
    }

    const requestMaxSeatsUpdate = async (desiredMaxSeats: number) => {
        await fetcher<SetDesiredMaxSeatsResponse>(api.setDesiredMaxSeats, {
            method: 'POST',
            body: JSON.stringify({
                desired_max_seats: desiredMaxSeats,
            }),
        }).then(async () => {
            console.log('リクエストした')
        })
    }

    const updatePageProps = () => {
        // 各項目のスナップショットをとる
        const snapshotPageProps = [...pageProps]
        const snapshotUsedLayouts = [...usedLayouts]
        const snapshotLatestSeats = [...latestSeats]

        if (snapshotUsedLayouts.length < currentPageIndex + 1) {
            // index out of rangeにならないように1ページ目に。
            setCurrentPageIndex(0) // 反映はほんの少し遅延するが、ほんの少しなので視覚的にはすぐに回復するはず？
        }

        let sumSeats = 0
        const newPageProps: LayoutPageProps[] = snapshotUsedLayouts.map(
            (layout, index): LayoutPageProps => {
                const numSeats = layout.seats.length
                const firstSeatIdInLayout = sumSeats + 1 // not index
                sumSeats += numSeats
                const LastSeatIdInLayout = sumSeats // not index
                const usedSeatsInLayout: Seat[] = snapshotLatestSeats.filter(
                    (seat) =>
                        firstSeatIdInLayout <= seat.seat_id &&
                        seat.seat_id <= LastSeatIdInLayout
                )
                let displayThisPage = false
                if (pageProps.length == 0 && index === 0) {
                    // 初回構築のときは1ページ目を表示
                    displayThisPage = true
                } else if (index >= snapshotPageProps.length) {
                    // 増えたページの場合は、表示はfalse
                    displayThisPage = false
                } else {
                    displayThisPage = snapshotPageProps[index].display
                }

                return {
                    roomLayout: layout,
                    firstSeatId: firstSeatIdInLayout,
                    usedSeats: usedSeatsInLayout,
                    display: displayThisPage,
                }
            }
        )
        setPageProps(newPageProps)
    }

    /**
     * 実際に必要な席数。
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

    if (pageProps.length > 0) {
        return (
            <>
                <div css={styles.defaultRoom}>
                    {layoutPages}
                    <Message
                        currentPageIndex={currentPageIndex}
                        currentRoomsLength={usedLayouts.length}
                        seats={latestSeats}
                    ></Message>
                </div>
            </>
        )
    } else {
        return <CenterLoading></CenterLoading>
    }
}

export default Seats
