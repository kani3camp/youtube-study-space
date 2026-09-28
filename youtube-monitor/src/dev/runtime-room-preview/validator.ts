import type {
	OverlayZone,
	Point,
	RuntimeProfile,
	RuntimeRoomOverlayMap,
} from './overlay-map'
import { roomLayoutForProfile } from './overlay-map'

const EPSILON = 1e-9

export type Bounds = {
	left: number
	top: number
	right: number
	bottom: number
}

export type SeatBounds = Bounds & {
	seatId: number
}

export type SeatCollision = {
	seatIds: [number, number]
}

export type ZoneIntrusion = {
	seatId: number
	zoneId: string
}

export type RuntimeRoomValidation = {
	profile: RuntimeProfile
	seatBounds: SeatBounds[]
	seatCollisions: SeatCollision[]
	overflows: number[]
	protectedVisualZoneIntrusions: ZoneIntrusion[]
	mainCirculationIntrusions: ZoneIntrusion[]
	seatZoneMisses: ZoneIntrusion[]
	isValid: boolean
}

export function rotatedRectBounds({
	x,
	y,
	width,
	height,
	rotate,
}: {
	x: number
	y: number
	width: number
	height: number
	rotate: number
}): Bounds {
	const radians = (rotate * Math.PI) / 180
	const cosine = Math.cos(radians)
	const sine = Math.sin(radians)
	const corners = [
		{ x: 0, y: 0 },
		{ x: width, y: 0 },
		{ x: 0, y: height },
		{ x: width, y: height },
	].map((corner) => ({
		x: x + corner.x * cosine - corner.y * sine,
		y: y + corner.x * sine + corner.y * cosine,
	}))

	return {
		left: Math.min(...corners.map((corner) => corner.x)),
		top: Math.min(...corners.map((corner) => corner.y)),
		right: Math.max(...corners.map((corner) => corner.x)),
		bottom: Math.max(...corners.map((corner) => corner.y)),
	}
}

/** Border contact is allowed; positive-area overlap is not. */
export function boundsOverlap(first: Bounds, second: Bounds): boolean {
	return (
		first.left < second.right - EPSILON &&
		first.right > second.left + EPSILON &&
		first.top < second.bottom - EPSILON &&
		first.bottom > second.top + EPSILON
	)
}

function zonePoints(zone: OverlayZone): Point[] {
	if (zone.shape.type === 'polygon') {
		return zone.shape.points
	}
	const { x, y, width, height } = zone.shape
	return [
		{ x, y },
		{ x: x + width, y },
		{ x: x + width, y: y + height },
		{ x, y: y + height },
	]
}

function boundsPoints(bounds: Bounds): Point[] {
	return [
		{ x: bounds.left, y: bounds.top },
		{ x: bounds.right, y: bounds.top },
		{ x: bounds.right, y: bounds.bottom },
		{ x: bounds.left, y: bounds.bottom },
	]
}

function polygonSamples(polygon: Point[]): Point[] {
	const edgeMidpoints = polygon.map((point, index) => {
		const nextPoint = polygon[(index + 1) % polygon.length]
		return {
			x: (point.x + nextPoint.x) / 2,
			y: (point.y + nextPoint.y) / 2,
		}
	})
	const center = polygon.reduce(
		(accumulator, point) => ({
			x: accumulator.x + point.x / polygon.length,
			y: accumulator.y + point.y / polygon.length,
		}),
		{ x: 0, y: 0 },
	)
	return [...polygon, ...edgeMidpoints, center]
}

function crossProduct(first: Point, second: Point, third: Point): number {
	return (
		(second.x - first.x) * (third.y - first.y) -
		(second.y - first.y) * (third.x - first.x)
	)
}

function segmentsProperlyIntersect(
	firstStart: Point,
	firstEnd: Point,
	secondStart: Point,
	secondEnd: Point,
): boolean {
	const firstSideStart = crossProduct(firstStart, firstEnd, secondStart)
	const firstSideEnd = crossProduct(firstStart, firstEnd, secondEnd)
	const secondSideStart = crossProduct(secondStart, secondEnd, firstStart)
	const secondSideEnd = crossProduct(secondStart, secondEnd, firstEnd)
	return (
		firstSideStart * firstSideEnd < -EPSILON &&
		secondSideStart * secondSideEnd < -EPSILON
	)
}

function polygonsProperlyIntersect(first: Point[], second: Point[]): boolean {
	for (let firstIndex = 0; firstIndex < first.length; firstIndex++) {
		const firstStart = first[firstIndex]
		const firstEnd = first[(firstIndex + 1) % first.length]
		for (let secondIndex = 0; secondIndex < second.length; secondIndex++) {
			const secondStart = second[secondIndex]
			const secondEnd = second[(secondIndex + 1) % second.length]
			if (
				segmentsProperlyIntersect(firstStart, firstEnd, secondStart, secondEnd)
			) {
				return true
			}
		}
	}
	return false
}

function pointInsidePolygonStrict(point: Point, polygon: Point[]): boolean {
	let inside = false
	for (
		let current = 0, previous = polygon.length - 1;
		current < polygon.length;
		previous = current++
	) {
		const currentPoint = polygon[current]
		const previousPoint = polygon[previous]
		if (Math.abs(crossProduct(previousPoint, currentPoint, point)) <= EPSILON) {
			const withinX =
				point.x >= Math.min(previousPoint.x, currentPoint.x) - EPSILON &&
				point.x <= Math.max(previousPoint.x, currentPoint.x) + EPSILON
			const withinY =
				point.y >= Math.min(previousPoint.y, currentPoint.y) - EPSILON &&
				point.y <= Math.max(previousPoint.y, currentPoint.y) + EPSILON
			if (withinX && withinY) {
				return false
			}
		}
		const crossesRay =
			currentPoint.y > point.y !== previousPoint.y > point.y &&
			point.x <
				((previousPoint.x - currentPoint.x) * (point.y - currentPoint.y)) /
					(previousPoint.y - currentPoint.y) +
					currentPoint.x
		if (crossesRay) {
			inside = !inside
		}
	}
	return inside
}

export function boundsOverlapZone(bounds: Bounds, zone: OverlayZone): boolean {
	if (zone.shape.type === 'rect') {
		return boundsOverlap(bounds, {
			left: zone.shape.x,
			top: zone.shape.y,
			right: zone.shape.x + zone.shape.width,
			bottom: zone.shape.y + zone.shape.height,
		})
	}
	const polygon = zonePoints(zone)
	const rectangle = boundsPoints(bounds)
	if (
		polygonSamples(rectangle).some((point) =>
			pointInsidePolygonStrict(point, polygon),
		)
	) {
		return true
	}
	if (
		polygonSamples(polygon).some((point) =>
			pointInsidePolygonStrict(point, rectangle),
		)
	) {
		return true
	}
	return polygonsProperlyIntersect(rectangle, polygon)
}

function pointInsidePolygonInclusive(point: Point, polygon: Point[]): boolean {
	if (pointInsidePolygonStrict(point, polygon)) {
		return true
	}
	return polygon.some((currentPoint, index) => {
		const nextPoint = polygon[(index + 1) % polygon.length]
		if (Math.abs(crossProduct(currentPoint, nextPoint, point)) > EPSILON) {
			return false
		}
		return (
			point.x >= Math.min(currentPoint.x, nextPoint.x) - EPSILON &&
			point.x <= Math.max(currentPoint.x, nextPoint.x) + EPSILON &&
			point.y >= Math.min(currentPoint.y, nextPoint.y) - EPSILON &&
			point.y <= Math.max(currentPoint.y, nextPoint.y) + EPSILON
		)
	})
}

function boundsInsideZone(bounds: Bounds, zone: OverlayZone): boolean {
	if (zone.shape.type === 'rect') {
		return (
			bounds.left >= zone.shape.x - EPSILON &&
			bounds.top >= zone.shape.y - EPSILON &&
			bounds.right <= zone.shape.x + zone.shape.width + EPSILON &&
			bounds.bottom <= zone.shape.y + zone.shape.height + EPSILON
		)
	}
	const polygon = zonePoints(zone)
	const rectangle = boundsPoints(bounds)
	if (
		!rectangle.every((point) => pointInsidePolygonInclusive(point, polygon))
	) {
		return false
	}

	// Four contained corners are insufficient for a concave polygon: an inward
	// notch can cross a rectangle edge or place polygon boundary inside it.
	return (
		!polygon.some((point) => pointInsidePolygonStrict(point, rectangle)) &&
		!polygonsProperlyIntersect(rectangle, polygon)
	)
}

export function validateRuntimeRoom(
	overlayMap: RuntimeRoomOverlayMap,
	profile: RuntimeProfile,
): RuntimeRoomValidation {
	const layout = roomLayoutForProfile(overlayMap, profile)
	const seatBounds = layout.seats.map((seat) => ({
		seatId: seat.id,
		...rotatedRectBounds({
			x: seat.x,
			y: seat.y,
			width: layout.seat_shape.width,
			height: layout.seat_shape.height,
			rotate: seat.rotate,
		}),
	}))
	const seatCollisions: SeatCollision[] = []
	for (let first = 0; first < seatBounds.length; first++) {
		for (let second = first + 1; second < seatBounds.length; second++) {
			if (boundsOverlap(seatBounds[first], seatBounds[second])) {
				seatCollisions.push({
					seatIds: [seatBounds[first].seatId, seatBounds[second].seatId],
				})
			}
		}
	}

	const overflows = seatBounds
		.filter(
			(bounds) =>
				bounds.left < -EPSILON ||
				bounds.top < -EPSILON ||
				bounds.right > layout.room_shape.width + EPSILON ||
				bounds.bottom > layout.room_shape.height + EPSILON,
		)
		.map((bounds) => bounds.seatId)
	const intrusions = (zones: OverlayZone[]): ZoneIntrusion[] =>
		seatBounds.flatMap((bounds) =>
			zones
				.filter((zone) => boundsOverlapZone(bounds, zone))
				.map((zone) => ({ seatId: bounds.seatId, zoneId: zone.id })),
		)
	const protectedVisualZoneIntrusions = intrusions(
		overlayMap.runtime_preview.protected_visual_zones,
	)
	const mainCirculationIntrusions = intrusions(
		overlayMap.runtime_preview.main_circulation,
	)
	const seatZoneMisses = Object.entries(
		overlayMap.runtime_preview.seat_zone_by_seat_id ?? {},
	).flatMap(([seatIdText, zoneId]) => {
		const seatId = Number(seatIdText)
		const bounds = seatBounds.find((candidate) => candidate.seatId === seatId)
		const zone = overlayMap.runtime_preview.seat_zones.find(
			(candidate) => candidate.id === zoneId,
		)
		return bounds && zone && !boundsInsideZone(bounds, zone)
			? [{ seatId, zoneId }]
			: []
	})
	const isValid =
		seatCollisions.length === 0 &&
		overflows.length === 0 &&
		protectedVisualZoneIntrusions.length === 0 &&
		mainCirculationIntrusions.length === 0 &&
		seatZoneMisses.length === 0

	return {
		profile,
		seatBounds,
		seatCollisions,
		overflows,
		protectedVisualZoneIntrusions,
		mainCirculationIntrusions,
		seatZoneMisses,
		isValid,
	}
}
