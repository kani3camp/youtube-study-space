import { RoomLayout } from '../../types/room-layout'

export const MemberSimpleRoom: RoomLayout = {
    version: 1,
    floor_image: '/images/vip-test1-room.png',
    font_size_ratio: 0.015,
    room_shape: {
        width: 1920,
        height: 1080,
    },
    seat_shape: {
        width: 287.24409449,
        height: 166.06299213,
    },
    partition_shapes: [
        //     {
        //         name: 'v25',
        //         width: 5,
        //         height: 25,
        //     },
        //     {
        //         name: 'v30',
        //         width: 5,
        //         height: 30,
        //     },
        //     {
        //         name: 'v65',
        //         width: 5,
        //         height: 65,
        //     },
        //     {
        //         name: 'v210',
        //         width: 5,
        //         height: 210,
        //     },
        //     {
        //         name: 'h35',
        //         width: 35,
        //         height: 5,
        //     },
        //     {
        //         name: 'h40',
        //         width: 40,
        //         height: 5,
        //     },
        //     {
        //         name: 'h85',
        //         width: 85,
        //         height: 5,
        //     },
    ],
    seats: [
        {
            id: 1,
            x: 136.06299213,
            y: 408.18897638,
            rotate: 0,
        },
        {
            id: 2,
            x: 627.77952756,
            y: 408.18897638,
            rotate: 0,
        },
        {
            id: 3,
            x: 1058.2677165,
            y: 415.7480315,
            rotate: 0,
        },
        {
            id: 4,
            x: 1511.8110236,
            y: 415.7480315,
            rotate: 0,
        },
        {
            id: 5,
            x: 151.18110236,
            y: 925.98425197,
            rotate: 0,
        },
        {
            id: 6,
            x: 642.51968504,
            y: 925.98425197,
            rotate: 0,
        },
        {
            id: 7,
            x: 1096.0629921,
            y: 925.98425197,
            rotate: 0,
        },
        {
            id: 8,
            x: 1511.8110236,
            y: 925.98425197,
            rotate: 0,
        },
    ],
    partitions: [],
}
