package wgvc

import "math"

// assignRelief sets each province's relief to the mean absolute elevation
// difference across its shared edges with same-medium neighbors: land with
// land, water with water. Land elevations lie in [0, 1) and water elevations
// in (-1, 0], so every counted difference, and therefore the mean, lies in
// [0, 1). World-boundary and coastline edges are ignored, so the sea-level
// step at the coast cannot dominate. A province with no same-medium neighbor
// has relief 0. The pass reads only island membership and elevation, and
// consumes no randomness.
func assignRelief(world *World) {
	totals := make([]float64, len(world.Provinces))
	counts := make([]int, len(world.Provinces))
	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) != 2 {
			continue
		}
		first, second := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
		firstLand := world.Provinces[first].IslandID != NoIslandID
		secondLand := world.Provinces[second].IslandID != NoIslandID
		if firstLand != secondLand {
			continue
		}
		difference := math.Abs(world.Provinces[first].Elevation - world.Provinces[second].Elevation)
		totals[first] += difference
		totals[second] += difference
		counts[first]++
		counts[second]++
	}
	for provinceID := range world.Provinces {
		province := &world.Provinces[provinceID]
		province.Relief = 0
		if counts[provinceID] > 0 {
			province.Relief = totals[provinceID] / float64(counts[provinceID])
		}
	}
}
