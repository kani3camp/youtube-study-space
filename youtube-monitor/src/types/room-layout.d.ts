export type RoomLayout = {
	floor_image: string
	font_size_ratio: number
	room_shape: {
		height: number
		width: number
	}
	seat_shape: {
		height: number
		width: number
	}
	partition_shapes: {
		name: string
		width: number
		height: number
	}[]
	seats: {
		id: number
		x: number
		y: number
		rotate: number // CSSのtransformの仕様に合わせた回転方向、単位。
	}[]
	partitions: {
		id: number
		x: number
		y: number
		shape_type: string
		rotate: number
	}[]
}
