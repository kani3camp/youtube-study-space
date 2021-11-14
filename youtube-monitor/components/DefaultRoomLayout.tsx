import React, { useState, useEffect, useReducer } from "react";
import styles from "./DefaultRoomLayout.module.sass";
import { RoomLayout } from "../types/room-layout";
import { DefaultRoomState, seat } from "../types/room-state";
import api from "../lib/api_config";
import fetcher from "../lib/fetcher";
import { RoomsStateResponse } from "../types/room-state";
import { bindActionCreators } from "redux";


const DefaultRoomLayout = ({ }: any) => {
  const MAX_NUM_ITEMS_PER_PAGE = 42
  const PAGING_INTERVAL_MSEC = 5000
  const PROGRESS_BAR_REFRESH_INTERVAL_MSEC = 30
  const PROGRESS_BAR_GROW_RATE = PROGRESS_BAR_REFRESH_INTERVAL_MSEC / PAGING_INTERVAL_MSEC * 100

  const [roomState, setRoomState] = useState<DefaultRoomState>()
  const [displaySeats, setDisplaySeats] = useState<seat[]>([])
  const [initialized, setInitialized] = useState<boolean>(false)
  const [progressBarWidth, setProgressBarWidth] = useState<number>(0)

  useEffect(() => {
    console.log('useEffect')
    if (!initialized) {
      init()
    } else {
      const now = new Date()
      // if (canRefreshPage) {
      updateDisplaySeats(roomState)
      // }
    }
  }, [initialized, roomState]);

  const init = () => {
    const fetchIntervalId = setInterval(() => {
      console.log('fetching')
      fetcher<RoomsStateResponse>(api.roomsState)
        .then((r) => {
          setRoomState(r.default_room)
        })
        .catch((err) => console.error(err));
    }, PAGING_INTERVAL_MSEC);

    setInitialized(true)
  }

  const maxSeatIndex = (seats: seat[]): number => {
    let maxSeatIndex = 0
    seats.forEach((each_seat: seat) => {
      if (each_seat.seat_id > maxSeatIndex) {
        maxSeatIndex = each_seat.seat_id
      }
    })
    return maxSeatIndex
  }

  const updateDisplaySeats = (roomState: DefaultRoomState | undefined) => {
    if (roomState && roomState?.seats.length > 0) {
      // console.log('previous display setas: ', displaySeats)

      // 前回のページの最後尾の席番号を求める
      let previousLastSeatId = 0
      if (displaySeats.length > 0) {
        previousLastSeatId = displaySeats[displaySeats.length - 1].seat_id
      }
      // console.log('previousLastSeatId: ', previousLastSeatId)

      // 次のページの先頭の席番号を求める
      let nextInitialSeatId = maxSeatIndex(roomState.seats) // 初期化。
      roomState.seats.forEach((each_seat: seat) => {
        if (each_seat.seat_id > previousLastSeatId && each_seat.seat_id < nextInitialSeatId) {
          nextInitialSeatId = each_seat.seat_id
          // console.log('暫定：', nextInitialSeatId)
        }
      })
      // console.log('nextInitialSeatId: ', nextInitialSeatId)

      let allSeatIds: number[] = []
      roomState.seats.forEach((each_seat: seat) => {
        allSeatIds.push(each_seat.seat_id)
      })
      allSeatIds.sort((a, b) => {
        return a - b
      })

      let nextDisplaySeatIds: number[] = []
      const nextInitialSeatIndex = allSeatIds.findIndex(item => item === nextInitialSeatId)
      let allocatingIndex = nextInitialSeatIndex
      while (true) {
        // props.roomState.seatsの全ての席を割り当てるか、MAX_NUM_ITEMS_PER_PAGE個の席を割り当てるまで
        if (nextDisplaySeatIds.length === roomState.seats.length || nextDisplaySeatIds.length === MAX_NUM_ITEMS_PER_PAGE) {
          break
        }
        nextDisplaySeatIds.push(allSeatIds[allocatingIndex])
        allocatingIndex = (allocatingIndex + 1) % roomState.seats.length
      }
      // console.log('nextDisplaySeatIds: ', nextDisplaySeatIds)

      let nextDisplaySeats: seat[] = []
      nextDisplaySeatIds.forEach((each_id: number) => {
        roomState.seats.forEach((each_seat: seat) => {
          if (each_seat.seat_id === each_id) {
            nextDisplaySeats.push(each_seat)
          }
        })
      })
      // console.log('nextDisplaySeats: ', nextDisplaySeats)

      setDisplaySeats(nextDisplaySeats)
      // resetProgressBar()
      console.log('完了')
    } else {
      setDisplaySeats([])
    }
  }

  // const resetProgressBar = () => {
  //   const progressBarDiv = document.getElementById('progress-bar')
  //   if (progressBarDiv) {
  //     progressBarDiv.style.width = "0%"
  //   } else {
  //     console.error('failed to get progress bar div')
  //   }
  //   setTimeout(growProgressBar, 100)
  // }

  // const growProgressBar = () => {
  //   // console.log(growProgressBar.name)
  //   const progressBarDiv = document.getElementById('progress-bar')
  //   if (progressBarDiv) {
  //     const currentWidth = Number(progressBarDiv.style.width.replace('%', ''))
  //     // console.log(progressBarDiv.style.width)
  //     // console.log('current width: ', currentWidth)
  //     if (currentWidth < 100) {
  //       progressBarDiv.style.width = (currentWidth + PROGRESS_BAR_GROW_RATE).toString() + "%"
  //     } else {
  //       return  // 100%に達してる場合はsetTimeoutループは終了
  //     }
  //   } else {
  //     console.error('failed to get progress bar div')
  //   }
  //   setTimeout(growProgressBar, 100)
  // }

  if (roomState) {
    return (
      <>
        <div
          id={styles.roomLayout}
        >
          {/* {seatList} */}
          {
            displaySeats.map((eachSeat: seat) => {
              const workName = eachSeat.work_name
              const displayName = eachSeat.user_display_name
              const seatColor = roomState.seats.find(s => s.seat_id === eachSeat.seat_id)?.color_code;

              return (
                <div
                  key={eachSeat.seat_id}
                  className={styles.seat}
                  style={{
                    backgroundColor: seatColor,
                  }}
                >
                  {<div className={styles.seatId} style={{ fontWeight: "bold" }}>
                    {eachSeat.seat_id}
                  </div>}
                  {workName !== '' && (<div className={styles.workName}>{workName}</div>)}
                  <div
                    className={styles.userDisplayName}
                  >
                    {displayName}
                  </div>

                </div>

              );
            })
          }

        </div>
        {/* <div id="progress-bar" className={styles.progressBar} style={{
        }}></div> */}
      </>
    );
  } else {
    return <div>Loading</div>;
  }
}

export default DefaultRoomLayout;
