import { RoomLayout } from '../../types/room-layout'

export const LayoutName: RoomLayout = {
    floor_image: '',
    version: 1,
    font_size_ratio: 0.015,
    room_shape: {
        width: 1520,
        height: 1000,
    },
    seat_shape: {
        width: 140,
        height: 100,
    },
    partition_shapes: [
        {
            name: 'partition1',
            width: 999999,
            height: 999999,
        },
    ],
    seats: [
        {
            id: 999999,
            x: 999999,
            y: 999999,
            rotate: 0,
        },
    ],
    partitions: [
        {
            id: 999999,
            x: 999999,
            y: 999999,
            shape_type: 'partition1',
            rotate: 0,
        },
    ],
}
