import { RoomLayout } from "../../types/room-layout";

export const HimajinRoomLayout: RoomLayout = {
    version: 1,
    font_size_ratio: 0.018,
    room_shape: {
        width: 330,
        height: 230
    },
    seat_shape: {
        width: 35,
        height: 25
    },
    partition_shapes: [
        {
            name: "h35",
            width: 35,
            height: 5
        },
        {
            name: "h40",
            width: 40,
            height: 5
        },
        {
            name: "h200",
            width: 200,
            height: 5
        },
        {
            name: "v25",
            width: 5,
            height: 25
        },
        {
            name: "v30",
            width: 5,
            height: 30
        },
        {
            name: "v60",
            width: 5,
            height: 60
        },
        {
            name: "v185",
            width: 5,
            height: 185
        },
        {
            name: "v90",
            width: 5,
            height: 90
        },
    ],
    seats: [
        {
            id: 1,
            x: 0,
            y: 0
        },
        {
            id: 2,
            x: 0,
            y: 30
        },
        {
            id: 3,
            x: 0,
            y: 60
        },
        {
            id: 4,
            x: 0,
            y: 90
        },
        {
            id: 5,
            x: 0,
            y: 120
        },
        {
            id: 6,
            x: 0,
            y: 150
        },
        {
            id: 7,
            x: 0,
            y: 180
        },
        {
            id: 8,
            x: 45,
            y: 30
        },
        {
            id: 9,
            x: 85,
            y: 30
        },
        {
            id: 10,
            x: 85,
            y: 60
        },
        {
            id: 11,
            x: 45,
            y: 90
        },
        {
            id: 12,
            x: 85,
            y: 90
        },
        {
            id: 13,
            x: 45,
            y: 120
        },
        {
            id: 14,
            x: 85,
            y: 120
        },
        {
            id: 15,
            x: 85,
            y: 150
        },
        {
            id: 16,
            x: 45,
            y: 180
        },
        {
            id: 17,
            x: 85,
            y: 180
        },
        {
            id: 18,
            x: 145,
            y: 0
        },
        {
            id: 19,
            x: 125,
            y: 90
        },
        {
            id: 20,
            x: 165,
            y: 90
        },
        {
            id: 21,
            x: 125,
            y: 120
        },
        {
            id: 22,
            x: 165,
            y: 120
        },
        {
            id: 23,
            x: 145,
            y: 205
        },
        {
            id: 24,
            x: 205,
            y: 30
        },
        {
            id: 25,
            x: 245,
            y: 30
        },
        {
            id: 26,
            x: 205,
            y: 60
        },
        {
            id: 27,
            x: 205,
            y: 90
        },
        {
            id: 28,
            x: 245,
            y: 90
        },
        {
            id: 29,
            x: 205,
            y: 120
        },
        {
            id: 30,
            x: 245,
            y: 120
        },
        {
            id: 31,
            x: 205,
            y: 150
        },
        {
            id: 32,
            x: 205,
            y: 180
        },
        {
            id: 33,
            x: 245,
            y: 180
        },
        {
            id: 34,
            x: 295,
            y: 0
        },
        {
            id: 35,
            x: 295,
            y: 30
        },
        {
            id: 36,
            x: 295,
            y: 60
        },
        {
            id: 37,
            x: 295,
            y: 90
        },
        {
            id: 38,
            x: 295,
            y: 120
        },
        {
            id: 39,
            x: 205,
            y: 150
        },
        {
            id: 40,
            x: 205,
            y: 180
        },
    ],
    partitions: [
        {
            id: 1,
            x: 0,
            y: 25,
            shape_type: "h35"
        },
        {
            id: 2,
            x: 0,
            y: 55,
            shape_type: "h35"
        },
        {
            id: 3,
            x: 0,
            y: 85,
            shape_type: "h35"
        },
        {
            id: 4,
            x: 0,
            y: 115,
            shape_type: "h35"
        },
        {
            id: 5,
            x: 0,
            y: 145,
            shape_type: "h35"
        },
        {
            id: 6,
            x: 0,
            y: 175,
            shape_type: "h35"
        },
        {
            id: 7,
            x: 0,
            y: 205,
            shape_type: "h35"
        },
        {
            id: 8,
            x: 80,
            y: 25,
            shape_type: "v185"
        },
        {
            id: 9,
            x: 45,
            y: 55,
            shape_type: "h35"
        },
        {
            id: 10,
            x: 85,
            y: 55,
            shape_type: "h40"
        },
        {
            id: 11,
            x: 85,
            y: 85,
            shape_type: "h40"
        },
        {
            id: 12,
            x: 85,
            y: 145,
            shape_type: "h40"
        },
        {
            id: 13,
            x: 85,
            y: 175,
            shape_type: "h40"
        },
        {
            id: 14,
            x: 45,
            y: 175,
            shape_type: "h35"
        },
        {
            id: 15,
            x: 45,
            y: 115,
            shape_type: "h35"
        },
        {
            id: 16,
            x: 120,
            y: 90,
            shape_type: "v25"
        },
        {
            id: 17,
            x: 120,
            y: 120,
            shape_type: "v25"
        },
        {
            id: 18,
            x: 85,
            y: 115,
            shape_type: "h200"
        },
        {
            id: 19,
            x: 160,
            y: 55,
            shape_type: "v60"
        },
        {
            id: 20,
            x: 160,
            y: 120,
            shape_type: "v60"
        },
        {
            id: 21,
            x: 140,
            y: 0,
            shape_type: "v30"
        },
        {
            id: 22,
            x: 180,
            y: 0,
            shape_type: "v30"
        },
        {
            id: 23,
            x: 140,
            y: 200,
            shape_type: "v30"
        },
        {
            id: 24,
            x: 180,
            y: 200,
            shape_type: "v30"
        },
        {
            id: 25,
            x: 240,
            y: 25,
            shape_type: "v90"
        },
        {
            id: 26,
            x: 200,
            y: 55,
            shape_type: "h40"
        },
        {
            id: 27,
            x: 245,
            y: 55,
            shape_type: "h40"
        },
        {
            id: 28,
            x: 280,
            y: 25,
            shape_type: "v30"
        },
        {
            id: 29,
            x: 205,
            y: 85,
            shape_type: "h35"
        },
        {
            id: 30,
            x: 200,
            y: 85,
            shape_type: "v30"
        },
        {
            id: 31,
            x: 280,
            y: 85,
            shape_type: "v30"
        },
        {
            id: 32,
            x: 200,
            y: 120,
            shape_type: "v30"
        },
        {
            id: 33,
            x: 280,
            y: 120,
            shape_type: "v30"
        },
        {
            id: 34,
            x: 240,
            y: 120,
            shape_type: "v90"
        },
        {
            id: 35,
            x: 205,
            y: 145,
            shape_type: "h35"
        },
        {
            id: 36,
            x: 200,
            y: 175,
            shape_type: "h40"
        },
        {
            id: 37,
            x: 245,
            y: 175,
            shape_type: "h40"
        },
        {
            id: 38,
            x: 280,
            y: 180,
            shape_type: "v30"
        },
        {
            id: 39,
            x: 295,
            y: 25,
            shape_type: "h35"
        },
        {
            id: 40,
            x: 295,
            y: 55,
            shape_type: "h35"
        },
        {
            id: 41,
            x: 295,
            y: 85,
            shape_type: "h35"
        },
        {
            id: 42,
            x: 295,
            y: 115,
            shape_type: "h35"
        },
        {
            id: 43,
            x: 295,
            y: 145,
            shape_type: "h35"
        },
        {
            id: 44,
            x: 295,
            y: 175,
            shape_type: "h35"
        },
        {
            id: 45,
            x: 295,
            y: 205,
            shape_type: "h35"
        },
    ]
}
