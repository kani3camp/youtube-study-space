import React, { useState, useEffect, useReducer } from "react";
import styles from "./DefaultRoomLayout.module.sass";
import { RoomLayout } from "../types/room-layout";
import { DefaultRoomState, seat } from "../types/room-state";
import api from "../lib/api_config";
import fetcher from "../lib/fetcher";
import { RoomsStateResponse } from "../types/room-state";
import { bindActionCreators } from "redux";


const DefaultRoomLayout = ({ }: any) => {
  const MAX_NUM_ITEMS_PER_PAGE = 48
  const PAGING_INTERVAL_SEC = 5

  const [roomState, setRoomState] = useState<DefaultRoomState>()
  const [displaySeats, setDisplaySeats] = useState<seat[]>([])
  const [initialized, setInitialized] = useState<boolean>(false)
  // const [canRefreshPage, setCanRefreshPage] = useState<boolean>(true)

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
    }, PAGING_INTERVAL_SEC * 1000);

    // const interval = setInterval(() => {
    //   console.log('interval')
    //   let time = new Date()
    //   console.log(time)
    //   time.setSeconds(time.getSeconds() + PAGING_INTERVAL_SEC)
    //   console.log(time)
    //   // setCanRefreshPage(true)
    // }, PAGING_INTERVAL_SEC * 1000);
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

      // setCanRefreshPage(false)
      setDisplaySeats(nextDisplaySeats)
      console.log('完了')
    } else {
      setDisplaySeats([])
    }
  }

  if (roomState) {
    const roomSeats = roomState.seats

    const sortSeats = (seats: seat[]): seat[] => {
      let seatIds: number[] = []
      seats.forEach((each_seat: seat) => {
        seatIds.push(each_seat.seat_id)
      })
      seatIds.sort((a, b) => {
        return a - b
      })
      let sortedSeats: seat[] = []
      seatIds.forEach((seatId) => {
        seats.forEach((each_seat) => {
          if (each_seat.seat_id === seatId) {
            sortedSeats.push(each_seat)
          }
        })
      })
      return sortedSeats
    }


    const seatList = sortSeats(roomState.seats).slice(0, MAX_NUM_ITEMS_PER_PAGE).map((seat, index) => {
      const workName = seat.work_name
      const displayName = seat.user_display_name
      const seatColor = roomSeats.find(s => s.seat_id === seat.seat_id)?.color_code;
      return (
        <div
          key={seat.seat_id}
          className={styles.seat}
          style={{
            backgroundColor: seatColor,
            width: "14%",
            height: "10%",
            fontSize: "1rem",
          }}
        >
          {<div className={styles.seatId} style={{ fontWeight: "bold" }}>
            {seat.seat_id}
          </div>}
          {workName !== '' && (<div className={styles.workName}>{workName}</div>)}
          <div
            className={styles.userDisplayName}
          >
            {displayName}
          </div>
        </div>
      )
    })

    return (
      <div
        id={styles.roomLayout}
      >
        {/* {seatList} */}
        {
          displaySeats.map((eachSeat: seat) => {
            const workName = eachSeat.work_name
            const displayName = eachSeat.user_display_name
            const seatColor = roomSeats.find(s => s.seat_id === eachSeat.seat_id)?.color_code;

            return (
              <div
                key={eachSeat.seat_id}
                className={styles.seat}
                style={{
                  backgroundColor: seatColor,
                  width: "14%",
                  height: "10%",
                  fontSize: "1rem",
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
    );
  } else {
    return <div>Loading</div>;
  }
}

export default DefaultRoomLayout;
