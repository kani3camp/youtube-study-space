import { RoomLayout } from '../../types/room-layout'

export const LayoutName: RoomLayout = {
  floor_image: '',
  version: 1,
  font_size_ratio: 0.018,
  room_shape: {
    width: 330,
    height: 230,
  },
  seat_shape: {
    width: 35,
    height: 25,
  },
  partition_shapes: [
    {
      name: 'partition1',
      width: 999,
      height: 999,
    },
  ],
  seats: [
    {
      id: 999,
      x: 999,
      y: 999,
      rotate: 0
    },
  ],
  partitions: [
    {
      id: 999,
      x: 999,
      y: 999,
      shape_type: 'partition1',
      rotate: 0,
    },  
  ],
}
