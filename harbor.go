package wgvc

import "slices"

// assignHarbors fills the siting inputs of every land province: counts of
// edges onto ocean and onto basins, the shelter of its adjacent water, and
// the rivers along it, ending at it, and joining at it. Shelter is the mean
// over adjacent water provinces of each one's land-neighbor fraction, where
// a neighbor is any province across an interior edge. Water provinces and
// inland land keep zero values and nil lists. The pass reads membership,
// basins, edges, corners, and rivers, and consumes no randomness.
func assignHarbors(world *World, neighbors [][]ProvinceID) {
	isLand := func(provinceID ProvinceID) bool { return world.Provinces[provinceID].IslandID != NoIslandID }
	landFraction := make([]float64, len(world.Provinces))
	for provinceID := range world.Provinces {
		if isLand(ProvinceID(provinceID)) || len(neighbors[provinceID]) == 0 {
			continue
		}
		land := 0
		for _, neighborID := range neighbors[provinceID] {
			if isLand(neighborID) {
				land++
			}
		}
		landFraction[provinceID] = float64(land) / float64(len(neighbors[provinceID]))
	}

	cornerOwners := make([][]ProvinceID, len(world.Corners))
	for _, province := range world.Provinces {
		for _, cornerID := range province.CornerIDs {
			cornerOwners[cornerID] = append(cornerOwners[cornerID], province.ID)
		}
	}

	for provinceID := range world.Provinces {
		province := &world.Provinces[provinceID]
		province.OceanEdges, province.BasinEdges, province.Shelter = 0, 0, 0
		province.RiverIDs, province.RiverMouthIDs, province.ConfluenceIDs = nil, nil, nil
		if !isLand(province.ID) {
			continue
		}
		shelter, waterEdges := 0.0, 0
		for _, edgeID := range province.EdgeIDs {
			edge := world.Edges[edgeID]
			if edge.RiverID != NoRiverID && !slices.Contains(province.RiverIDs, edge.RiverID) {
				province.RiverIDs = append(province.RiverIDs, edge.RiverID)
			}
			for _, otherID := range edge.ProvinceIDs {
				if otherID == province.ID || isLand(otherID) {
					continue
				}
				waterEdges++
				shelter += landFraction[otherID]
				if world.Provinces[otherID].BasinID == NoBasinID {
					province.OceanEdges++
				} else {
					province.BasinEdges++
				}
			}
		}
		if waterEdges > 0 {
			province.Shelter = shelter / float64(waterEdges)
		}
		slices.Sort(province.RiverIDs)
	}

	for _, river := range world.Rivers {
		mouth := river.CornerIDs[len(river.CornerIDs)-1]
		lastEdge := world.Edges[river.EdgeIDs[len(river.EdgeIDs)-1]]
		for _, provinceID := range cornerOwners[mouth] {
			if !isLand(provinceID) || !slices.Contains(lastEdge.ProvinceIDs, provinceID) {
				continue
			}
			province := &world.Provinces[provinceID]
			if river.Mouth.Kind == RiverEndRiver {
				province.ConfluenceIDs = append(province.ConfluenceIDs, river.ID)
			} else {
				province.RiverMouthIDs = append(province.RiverMouthIDs, river.ID)
			}
		}
	}
}
