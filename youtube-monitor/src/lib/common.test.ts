import { numSeatsOfRoomLayouts } from './common'

jest.mock('next/font/google', () => ({
    M_PLUS_Rounded_1c: jest.fn(() => ({
        style: {
            fontFamily: 'mock-font-name',
        },
    })),
    Source_Code_Pro: jest.fn(() => ({
        style: {
            fontFamily: 'mock-font-name',
        },
    })),
}))

test('numSeatsOfRoomLayouts', () => {
    expect(numSeatsOfRoomLayouts([])).toBe(0)
    expect(
        numSeatsOfRoomLayouts([
            roomLayoutWithSeats([
                { id: 1, x: 0, y: 0, rotate: 0 },
                { id: 2, x: 0, y: 0, rotate: 0 },
            ]),
        ])
    ).toBe(2)
})

const roomLayoutWithSeats = (seats: { id: number; x: number; y: number; rotate: number }[]) => ({
    floor_image: '',
    version: 0,
    font_size_ratio: 1,
    room_shape: {
        height: 0,
        width: 0,
    },
    seat_shape: {
        height: 0,
        width: 0,
    },
    partition_shapes: [],
    seats,
    partitions: [],
})
