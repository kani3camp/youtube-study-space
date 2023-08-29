import { RoomLayout } from '../../types/room-layout'

export const GLOnlineLayout: RoomLayout = {
    floor_image: '/images/GL_inONLINE.png',
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
    partition_shapes: [],
    seats: [
        { id: 1, x: 428, y: 83, rotate: 0 },
        { id: 2, x: 614, y: 63, rotate: 0 },
        { id: 3, x: 358, y: 366, rotate: 0 },
        { id: 4, x: 640, y: 410, rotate: 0 },
        { id: 5, x: 839, y: 84, rotate: 0 },
        { id: 6, x: 1068, y: 541, rotate: 0 },
        { id: 7, x: 1043, y: 282, rotate: 0 },
        { id: 8, x: 1232, y: 389, rotate: 0 },
        { id: 9, x: 34, y: 588, rotate: 0 },
        { id: 10, x: 1185, y: 853, rotate: 0 },
    ],
    partitions: [],
}
