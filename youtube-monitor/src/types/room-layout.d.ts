export type RoomLayout = {
  version: number;
  font_size_ratio: number;
  room_shape: {
    height: number;
    width: number;
  };
  seat_shape: {
    height: number;
    width: number;
  };
  partition_shapes: {
    name: string;
    width: number;
    height: number;
  }[];
  seats: {
    id: number;
    x: number;
    y: number;
  }[];
  partitions: {
    id: number;
    x: number;
    y: number;
    shape_type: string;
  }[];
};
