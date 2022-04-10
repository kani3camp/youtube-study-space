import { RoomLayout } from "../../types/room-layout";

export const LayoutName: RoomLayout = {
    version: 1,
    font_size_ratio: 0.018,
    room_shape: {
        width: 400,
        height: 280
    },
    seat_shape: {
        width: 40,
        height: 30
    },
    partition_shapes: [
        {
            name: "partition1",
            width: 40,
            height: 30
        },
    ],
    seats: [
        {
            id: 1,
            x: 50,
            y: 10
        },
        {
            id: 2,
            x: 90,
            y: 40
        },
        {
            id: 3,
            x: 50,
            y: 70
        },
        {
            id: 4,
            x: 10,
            y: 40
        },
        {
            id: 5,
            x: 180,
            y: 20
        },
        {
            id: 6,
            x: 220,
            y: 50
        },
        {
            id: 7,
            x: 180,
            y: 80
        },
        {
            id: 8,
            x: 140,
            y: 50
        },
        {
            id: 9,
            x: 315,
            y: 10
        },
        {
            id: 10,
            x: 355,
            y: 40
        },
        {
            id: 11,
            x: 315,
            y: 70
        },
        {
            id: 12,
            x: 275,
            y: 40
        },
        {
            id: 13,
            x: 110,
            y: 100
        },
        {
            id: 14,
            x: 150,
            y: 130
        },
        {
            id: 15,
            x: 110,
            y: 160
        },
        {
            id: 16,
            x: 70,
            y: 130
        },
        {
            id: 17,
            x: 260,
            y: 100
        },
        {
            id: 18,
            x: 300,
            y: 130
        },
        {
            id: 19,
            x: 260,
            y: 160
        },
        {
            id: 20,
            x: 220,
            y: 130
        },
        {
            id: 21,
            x: 50,
            y: 185
        },
        {
            id: 22,
            x: 90,
            y: 215
        },
        {
            id: 23,
            x: 50,
            y: 250
        },
        {
            id: 24,
            x: 10,
            y: 215
        },
        {
            id: 25,
            x: 180,
            y: 175
        },
        {
            id: 26,
            x: 220,
            y: 205
        },
        {
            id: 27,
            x: 180,
            y: 235
        },
        {
            id: 28,
            x: 140,
            y: 205
        },
        {
            id: 29,
            x: 315,
            y: 185
        },
        {
            id: 30,
            x: 355,
            y: 215
        },
        {
            id: 31,
            x: 315,
            y: 245
        },
        {
            id: 32,
            x: 275,
            y: 215
        },
    ],
    partitions: [
        {
            id: 1,
            x: 50,
            y: 40,
            shape_type: "partition1"
        },
        {
            id: 2,
            x: 180,
            y: 50,
            shape_type: "partition1"
        },
        {
            id: 3,
            x: 315,
            y: 40,
            shape_type: "partition1"
        },
        {
            id: 4,
            x: 110,
            y: 130,
            shape_type: "partition1"
        },
        {
            id: 5,
            x: 280,
            y: 130,
            shape_type: "partition1"
        },
        {
            id: 6,
            x: 50,
            y: 215,
            shape_type: "partition1"
        },
        {
            id: 7,
            x: 180,
            y: 205,
            shape_type: "partition1"
        },
        {
            id: 8,
            x: 315,
            y: 215,
            shape_type: "partition1"
        },
    ]
}
