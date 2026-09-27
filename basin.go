package wgvc

import "slices"

// basinInlandSeaMinProvinces is the member count at which a basin reads as an
// inland sea rather than a lake. Generated basins are mostly one or two
// provinces; a few reach ten or more.
const basinInlandSeaMinProvinces = 10

// assignBasins partitions water provinces into components connected by
// water-water edges. A component with a world-boundary edge is ocean; every
// other component is a basin enclosed by land. Basins are ordered by their
// lowest province ID. The pass reads island membership, adjacency, and
// elevation, consumes no randomness, and changes only basin fields.
func assignBasins(world *World) {
	isWater := func(provinceID ProvinceID) bool {
		return world.Provinces[provinceID].IslandID == NoIslandID
	}
	neighbors := make([][]ProvinceID, len(world.Provinces))
	onBoundary := make([]bool, len(world.Provinces))
	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) == 1 {
			onBoundary[edge.ProvinceIDs[0]] = true
			continue
		}
		first, second := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
		neighbors[first] = append(neighbors[first], second)
		neighbors[second] = append(neighbors[second], first)
	}

	world.Basins = nil
	for provinceID := range world.Provinces {
		world.Provinces[provinceID].BasinID = NoBasinID
	}
	visited := make([]bool, len(world.Provinces))
	for start := range world.Provinces {
		startID := ProvinceID(start)
		if visited[start] || !isWater(startID) {
			continue
		}
		members, enclosed := waterComponent(startID, neighbors, onBoundary, visited, isWater)
		if !enclosed {
			continue
		}
		basin := Basin{ID: BasinID(len(world.Basins)), ProvinceIDs: members}
		surfaceSet := false
		for _, memberID := range members {
			member := &world.Provinces[memberID]
			member.BasinID = basin.ID
			basin.Depth = max(basin.Depth, -member.Elevation)
			for _, neighborID := range neighbors[memberID] {
				if isWater(neighborID) {
					continue
				}
				if elevation := world.Provinces[neighborID].Elevation; !surfaceSet || elevation < basin.SurfaceElevation {
					basin.SurfaceElevation = elevation
					surfaceSet = true
				}
			}
		}
		world.Basins = append(world.Basins, basin)
	}
}

// waterComponent collects the water component containing start in ascending
// province order and reports whether none of it touches the world boundary.
func waterComponent(start ProvinceID, neighbors [][]ProvinceID, onBoundary, visited []bool, isWater func(ProvinceID) bool) ([]ProvinceID, bool) {
	members := []ProvinceID{start}
	visited[start] = true
	enclosed := true
	for next := 0; next < len(members); next++ {
		provinceID := members[next]
		if onBoundary[provinceID] {
			enclosed = false
		}
		for _, neighborID := range neighbors[provinceID] {
			if !visited[neighborID] && isWater(neighborID) {
				visited[neighborID] = true
				members = append(members, neighborID)
			}
		}
	}
	slices.Sort(members)
	return members, enclosed
}

func basinTerrain(basin Basin) Terrain {
	if len(basin.ProvinceIDs) >= basinInlandSeaMinProvinces {
		return TerrainInlandSea
	}
	return TerrainLake
}
