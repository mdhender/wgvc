package wgvc

// assignGeometry derives the measurements that follow from the final mesh:
// each edge's length, each province's boundary edges in ring order, and each
// province's polygon area. Edge lengths and areas are in world units, where
// the mesh has been scaled so the land provinces have a mean area of 1. The
// pass reads only corners, rings, and edge endpoints, and consumes no
// randomness.
func assignGeometry(world *World) {
	edgesByCorners := make(map[[2]CornerID]EdgeID, len(world.Edges))
	for edgeID := range world.Edges {
		edge := &world.Edges[edgeID]
		edge.Length = pointDistance(world.Corners[edge.CornerIDs[0]].Point, world.Corners[edge.CornerIDs[1]].Point)
		edgesByCorners[edge.CornerIDs] = edge.ID
	}
	for provinceID := range world.Provinces {
		province := &world.Provinces[provinceID]
		ring := make([]Point, len(province.CornerIDs))
		province.EdgeIDs = make([]EdgeID, len(province.CornerIDs))
		for ringIndex, cornerID := range province.CornerIDs {
			ring[ringIndex] = world.Corners[cornerID].Point
			next := province.CornerIDs[(ringIndex+1)%len(province.CornerIDs)]
			province.EdgeIDs[ringIndex] = edgesByCorners[orderedCornerPair(cornerID, next)]
		}
		province.Area = signedArea(ring)
	}
}

func orderedCornerPair(first, second CornerID) [2]CornerID {
	if first > second {
		first, second = second, first
	}
	return [2]CornerID{first, second}
}
