package wgvc

import "math"

// assignExits numbers each province's boundary edges clockwise from north.
// The ring is counterclockwise with +Y north, so clockwise is reverse ring
// order. Exit 1 is the edge with the smallest outward bearing in [0, 360),
// the first exit clockwise from due north; ties, which convexity should
// prevent, fall to the lower edge ID. Every cell is convex, so bearings
// increase strictly with exit number. The pass reads only corners, rings,
// and edge incidence, and consumes no randomness.
func assignExits(world *World) {
	for provinceID := range world.Provinces {
		province := &world.Provinces[provinceID]
		n := len(province.CornerIDs)
		bearings := make([]float64, n)
		start := 0
		for ringIndex := range n {
			first := world.Corners[province.CornerIDs[ringIndex]].Point
			second := world.Corners[province.CornerIDs[(ringIndex+1)%n]].Point
			bearings[ringIndex] = outwardBearing(first, second)
			if bearings[ringIndex] < bearings[start] ||
				(bearings[ringIndex] == bearings[start] && province.EdgeIDs[ringIndex] < province.EdgeIDs[start]) {
				start = ringIndex
			}
		}
		province.Exits = make([]Exit, n)
		for number := 1; number <= n; number++ {
			ringIndex := ((start-(number-1))%n + n) % n
			edge := world.Edges[province.EdgeIDs[ringIndex]]
			neighborID := NoProvinceID
			for _, incident := range edge.ProvinceIDs {
				if incident != province.ID {
					neighborID = incident
				}
			}
			province.Exits[number-1] = Exit{
				Number:     number,
				EdgeID:     edge.ID,
				NeighborID: neighborID,
				Bearing:    bearings[ringIndex],
				Compass:    compassFor(bearings[ringIndex]),
			}
		}
	}
}

// outwardBearing is the bearing of the outward normal of a counterclockwise
// ring segment, in degrees clockwise from north (+Y), in [0, 360).
func outwardBearing(first, second Point) float64 {
	normalX := second.Y - first.Y
	normalY := -(second.X - first.X)
	degrees := math.Atan2(normalX, normalY) * 180 / math.Pi
	if degrees < 0 {
		degrees += 360
	}
	if degrees >= 360 {
		degrees -= 360
	}
	return degrees
}

var compassPoints = [8]Compass{
	CompassNorth, CompassNortheast, CompassEast, CompassSoutheast,
	CompassSouth, CompassSouthwest, CompassWest, CompassNorthwest,
}

// compassFor is the 8-point label whose 45-degree sector contains bearing.
func compassFor(bearing float64) Compass {
	return compassPoints[int(math.Floor((bearing+22.5)/45))%8]
}
