import { RoomLayout } from "../../types/room-layout";

export const ver2RoomLayout: RoomLayout = {
    version: 12,
    font_size_ratio: 0.02,
    room_shape: {
        width: 330,
        height: 230
    },
    seat_shape: {
        width: 40,
        height: 30
    },
    partition_shapes: [
        {
            name: "s-v",
            width: 5,
            height: 30
        },
        {
            name: "l-v",
            width: 5,
            height: 100
        },
        {
            name: "n-h",
            width: 40,
            height: 5
        },
        {
            name: "n-v",
            width: 5,
            height: 75
        }
    ],
    seats: [
        {
            id: 1,
            x: 0,
            y: 20
        },
        {
            id: 2,
            x: 45,
            y: 20
        },
        {
            id: 3,
            x: 90,
            y: 20
        },
        {
            id: 4,
            x: 0,
            y: 70
        },
        {
            id: 5,
            x: 45,
            y: 70
        },
        {
            id: 6,
            x: 90,
            y: 70
        },
        {
            id: 7,
            x: 0,
            y: 120
        },
        {
            id: 8,
            x: 45,
            y: 120
        },
        {
            id: 9,
            x: 90,
            y: 120
        },
        {
            id: 10,
            x: 0,
            y: 170
        },
        {
            id: 11,
            x: 45,
            y: 170
        },
        {
            id: 12,
            x: 90,
            y: 170
        },
        {
            id: 13,
            x: 155,
            y: 0
        },
        {
            id: 14,
            x: 155,
            y: 50
        },
        {
            id: 15,
            x: 220,
            y: 0
        },
        {
            id: 16,
            x: 220,
            y: 50
        },
        {
            id: 17,
            x: 285,
            y: 0
        },
        {
            id: 18,
            x: 285,
            y: 50
        },
        {
            id: 19,
            x: 155,
            y: 135
        },
        {
            id: 20,
            x: 200,
            y: 135
        },
        {
            id: 21,
            x: 245,
            y: 135
        },
        {
            id: 22,
            x: 290,
            y: 135
        },
        {
            id: 23,
            x: 155,
            y: 170
        },
        {
            id: 24,
            x: 200,
            y: 170
        },
        {
            id: 25,
            x: 245,
            y: 170
        },
        {
            id: 26,
            x: 290,
            y: 170
        }
    ],
    partitions: [
        {
            id: 1,
            x: 40,
            y: 20,
            shape_type: "s-v"
        },
        {
            id: 2,
            x: 85,
            y: 20,
            shape_type: "s-v"
        },
        {
            id: 3,
            x: 40,
            y: 70,
            shape_type: "s-v"
        },
        {
            id: 4,
            x: 85,
            y: 70,
            shape_type: "s-v"
        },
        {
            id: 5,
            x: 40,
            y: 120,
            shape_type: "s-v"
        },
        {
            id: 6,
            x: 85,
            y: 120,
            shape_type: "s-v"
        },
        {
            id: 7,
            x: 40,
            y: 170,
            shape_type: "s-v"
        },
        {
            id: 8,
            x: 85,
            y: 170,
            shape_type: "s-v"
        },
        {
            id: 9,
            x: 195,
            y: 0,
            shape_type: "l-v"
        },
        {
            id: 10,
            x: 155,
            y: 45,
            shape_type: "n-h"
        },
        {
            id: 11,
            x: 155,
            y: 95,
            shape_type: "n-h"
        },
        {
            id: 12,
            x: 260,
            y: 0,
            shape_type: "l-v"
        },
        {
            id: 13,
            x: 220,
            y: 45,
            shape_type: "n-h"
        },
        {
            id: 14,
            x: 220,
            y: 95,
            shape_type: "n-h"
        },
        {
            id: 15,
            x: 325,
            y: 0,
            shape_type: "l-v"
        },
        {
            id: 16,
            x: 285,
            y: 45,
            shape_type: "n-h"
        },
        {
            id: 17,
            x: 285,
            y: 95,
            shape_type: "n-h"
        },
        {
            id: 18,
            x: 150,
            y: 130,
            shape_type: "n-v"
        },
        {
            id: 19,
            x: 195,
            y: 130,
            shape_type: "n-v"
        },
        {
            id: 20,
            x: 240,
            y: 130,
            shape_type: "n-v"
        },
        {
            id: 21,
            x: 285,
            y: 130,
            shape_type: "n-v"
        },
        {
            id: 22,
            x: 155,
            y: 165,
            shape_type: "n-h"
        },
        {
            id: 23,
            x: 200,
            y: 165,
            shape_type: "n-h"
        },
        {
            id: 24,
            x: 245,
            y: 165,
            shape_type: "n-h"
        },
        {
            id: 25,
            x: 290,
            y: 165,
            shape_type: "n-h"
        }
    ]
}
