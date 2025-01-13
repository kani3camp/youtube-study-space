import { RoomLayout } from '../../types/room-layout'

// FIXME: 表示バグあり
export const oneSeatRoom: RoomLayout = {
    floor_image: '',
    font_size_ratio: 0.018,
    room_shape: {
        width: 330,
        height: 230,
    },
    seat_shape: {
        width: 35,
        height: 25,
    },
    partition_shapes: [],
    seats: [
        {
            id: 1,
            x: 100,
            y: 100,
            rotate: 0,
        },
    ],
    partitions: [],
}
