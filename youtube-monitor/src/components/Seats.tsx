import { FC, useEffect, useState } from 'react'
import api from '../lib/api_config'
import { useInterval, validateRoomsStateResponse } from '../lib/common'
import fetcher from '../lib/fetcher'
import { allRooms, numSeatsInAllBasicRooms } from '../rooms/rooms-config'
import * as styles from '../styles/Room.styles'
import {
    RoomsStateResponse,
    Seat,
    SetDesiredMaxSeatsResponse,
} from '../types/api'
import { RoomLayout } from '../types/room-layout'
import CenterLoading from './CenterLoading'
import Message from './Message'
import SeatsPage, { LayoutPageProps } from './SeatsPage'

const Seats: FC = () => {
    const DATA_FETCHING_INTERVAL_MSEC = 6 * 1000
    const PAGING_INTERVAL_MSEC = 8 * 1000

    const [latestRoomsState, setLatestRoomsState] =
        useState<RoomsStateResponse>()
    const [currentPageIndex, setCurrentPageIndex] = useState<number>(0)
    const [usedLayouts, setUsedLayouts] = useState<RoomLayout[]>([])
    const [pageProps, setPageProps] = useState<LayoutPageProps[]>([])

    useEffect(() => {
        fetchAndUpdateRoomLayouts()
    }, [])

    useInterval(async () => {
        await fetchAndUpdateRoomLayouts()
    }, DATA_FETCHING_INTERVAL_MSEC)

    useInterval(() => {
        if (usedLayouts.length > 0) {
            const newPageIndex = (currentPageIndex + 1) % usedLayouts.length
            setCurrentPageIndex(newPageIndex)
        }
    }, PAGING_INTERVAL_MSEC)

    useEffect(() => {
        updatePageProps()
    }, [latestRoomsState, usedLayouts])

    useEffect(() => {
        changePage(currentPageIndex)
    }, [currentPageIndex])

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

    const fetchAndUpdateRoomLayouts = async () => {
        let maxSeats = 0

        // seats取得
        await new Promise<void>((resolve, reject) => {
            console.log('fetchする')
            fetcher<RoomsStateResponse>(api.roomsState)
                .then(async (r) => {
                    console.log('fetchした')
                    if (r.result !== 'ok') {
                        console.error(r)
                        reject()
                    }

                    // 値チェック
                    if (!validateRoomsStateResponse(r)) {
                        console.error('validate response failed: ', r)
                        reject()
                    }

                    setLatestRoomsState(r)
                    maxSeats = r.max_seats

                    // まず、現状の入室状況（seatsとmax_seats）と設定された空席率（min_vacancy_rate）を基に、適切なmax_seatsを求める。
                    let finalDesiredMaxSeats: number
                    const minSeatsByVacancyRate = Math.ceil(
                        r.seats.length / (1 - r.min_vacancy_rate)
                    )
                    // もしmax_seatsが基本ルームの席数より多ければ、臨時ルームを増やす
                    if (minSeatsByVacancyRate > numSeatsInAllBasicRooms()) {
                        let current_num_seats: number =
                            numSeatsInAllBasicRooms()
                        let current_adding_temporary_room_index = 0
                        while (current_num_seats < minSeatsByVacancyRate) {
                            current_num_seats +=
                                allRooms.temporaryRooms[
                                    current_adding_temporary_room_index
                                ].seats.length
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
                        `desired: ${finalDesiredMaxSeats}, current: ${r.max_seats}`
                    )

                    // 求めたmax_seatsが現状の値と異なったら、リクエストを送る
                    if (finalDesiredMaxSeats !== r.max_seats) {
                        console.log(
                            'リクエストを送る',
                            finalDesiredMaxSeats,
                            r.max_seats
                        )
                        requestMaxSeatsUpdate(finalDesiredMaxSeats) // awaitはしない
                    }

                    // リクエストが送信されたら、すぐに反映されるわけではないのでとりあえずレイアウトを用意して表示する
                    // 必要分（＝r.seatsにある席は全てカバーする）だけ臨時レイアウトを追加
                    const nextDisplayLayouts: RoomLayout[] = [
                        ...allRooms.basicRooms,
                    ] // まずは基本ルームを設定
                    if (maxSeats > numSeatsInAllBasicRooms()) {
                        let currentAddingLayoutIndex = 0
                        while (
                            numSeatsOfRoomLayouts(nextDisplayLayouts) < maxSeats
                        ) {
                            nextDisplayLayouts.push(
                                allRooms.temporaryRooms[
                                    currentAddingLayoutIndex
                                ]
                            )
                            currentAddingLayoutIndex =
                                (currentAddingLayoutIndex + 1) %
                                allRooms.temporaryRooms.length
                        }
                    }

                    // TODO: レイアウト的にmaxSeatsより大きい番号の席が含まれそうであれば、それらの席は表示しない

                    setUsedLayouts(nextDisplayLayouts)
                    resolve()
                })
                .catch((err) => {
                    console.error(err)
                    reject(err)
                })
        })
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
        if (latestRoomsState === undefined) {
            return
        }
        // 各項目のスナップショットをとる
        const snapshotPageProps = [...pageProps]
        const snapshotUsedLayouts = [...usedLayouts]
        const snapshotLatestRoomsState = JSON.parse(
            JSON.stringify(latestRoomsState)
        ) as RoomsStateResponse

        if (snapshotUsedLayouts.length < currentPageIndex + 1) {
            // index out of rangeにならないように1ページ目に。
            setCurrentPageIndex(0) // 反映はほんの少し遅延するが、ほんの少しなので視覚的にはすぐに回復するはず？
        }

        let sumSeats = 0
        const newPageProps: LayoutPageProps[] = snapshotUsedLayouts.map(
            (layout, index): LayoutPageProps => {
                const numSeats = layout.seats.length
                const firstSeatIdInLayout = sumSeats + 1 // インデックスではない
                sumSeats += numSeats
                const LastSeatIdInLayout = sumSeats // インデックスではない
                const usedSeatsInLayout: Seat[] =
                    snapshotLatestRoomsState.seats.filter(
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
                        seats={
                            latestRoomsState !== undefined
                                ? latestRoomsState.seats
                                : []
                        }
                    ></Message>
                </div>
            </>
        )
    } else {
        return <CenterLoading></CenterLoading>
    }
}

export default Seats
