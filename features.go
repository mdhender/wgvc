package wgvc

import (
	"slices"
	"sort"
)

const (
	// featureMinimumProvinces is the smallest terrain region reported as a
	// feature.
	featureMinimumProvinces = 5
	// archipelagoMaxGap is the longest water path, in provinces, between two
	// islands of one archipelago.
	archipelagoMaxGap = 6
)

// terrainFamilies maps each terrain that can form a named feature to its
// feature kind. Terrains absent here (open land, coast, tundra, water) form
// no feature.
var terrainFamilies = map[Terrain]FeatureKind{
	TerrainMountain:         FeatureMountainRange,
	TerrainAlpine:           FeatureMountainRange,
	TerrainVolcano:          FeatureMountainRange,
	TerrainVolcanicHighland: FeatureMountainRange,
	TerrainHills:            FeatureHillCountry,
	TerrainBadlands:         FeatureHillCountry,
	TerrainPlateau:          FeaturePlateau,
	TerrainBorealForest:     FeatureForest,
	TerrainTemperateForest:  FeatureForest,
	TerrainRainforest:       FeatureForest,
	TerrainDesert:           FeatureDesert,
	TerrainScrubland:        FeatureDesert,
	TerrainMarsh:            FeatureWetland,
	TerrainSwamp:            FeatureWetland,
	TerrainBog:              FeatureWetland,
	TerrainGlacialIce:       FeatureIceField,
}

// assignFeatures identifies the geographic units a consumer can name:
// connected same-family terrain regions of at least featureMinimumProvinces
// land provinces, and archipelagos of islands within archipelagoMaxGap water
// provinces of one another. It runs after terrain, reads terrain, membership,
// and adjacency, and consumes no randomness.
func assignFeatures(world *World) {
	neighbors := provinceNeighbors(world)
	world.Features = nil
	kindOf := func(provinceID ProvinceID) (FeatureKind, bool) {
		province := world.Provinces[provinceID]
		if province.IslandID == NoIslandID {
			return "", false
		}
		kind, ok := terrainFamilies[province.Terrain]
		return kind, ok
	}
	visited := make([]bool, len(world.Provinces))
	var features []Feature
	for start := range world.Provinces {
		kind, ok := kindOf(ProvinceID(start))
		if !ok || visited[start] {
			continue
		}
		visited[start] = true
		members := []ProvinceID{ProvinceID(start)}
		for next := 0; next < len(members); next++ {
			for _, neighborID := range neighbors[members[next]] {
				if neighborKind, ok := kindOf(neighborID); ok && neighborKind == kind && !visited[neighborID] {
					visited[neighborID] = true
					members = append(members, neighborID)
				}
			}
		}
		if len(members) < featureMinimumProvinces {
			continue
		}
		slices.Sort(members)
		features = append(features, Feature{
			Kind:        kind,
			IslandIDs:   []IslandID{world.Provinces[start].IslandID},
			ProvinceIDs: members,
		})
	}

	for _, group := range archipelagos(world, neighbors) {
		feature := Feature{Kind: FeatureArchipelago, IslandIDs: group}
		for _, islandID := range group {
			feature.ProvinceIDs = append(feature.ProvinceIDs, world.Islands[islandID].ProvinceIDs...)
		}
		slices.Sort(feature.ProvinceIDs)
		features = append(features, feature)
	}

	rank := map[FeatureKind]int{}
	for index, kind := range FeatureKinds() {
		rank[kind] = index
	}
	sort.SliceStable(features, func(i, j int) bool {
		if rank[features[i].Kind] != rank[features[j].Kind] {
			return rank[features[i].Kind] < rank[features[j].Kind]
		}
		return features[i].ProvinceIDs[0] < features[j].ProvinceIDs[0]
	})
	for index := range features {
		features[index].ID = FeatureID(index)
	}
	world.Features = features
}

// archipelagos groups islands joined by water paths of at most
// archipelagoMaxGap water provinces, returning each group of two or more
// islands ascending, ordered by lowest island.
func archipelagos(world *World, neighbors [][]ProvinceID) [][]IslandID {
	parent := make([]int, len(world.Islands))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	isWater := func(provinceID ProvinceID) bool { return world.Provinces[provinceID].IslandID == NoIslandID }
	for _, island := range world.Islands {
		distance := map[ProvinceID]int{}
		var queue []ProvinceID
		for _, landID := range island.ProvinceIDs {
			for _, neighborID := range neighbors[landID] {
				if _, seen := distance[neighborID]; isWater(neighborID) && !seen {
					distance[neighborID] = 1
					queue = append(queue, neighborID)
				}
			}
		}
		for next := 0; next < len(queue); next++ {
			provinceID := queue[next]
			for _, neighborID := range neighbors[provinceID] {
				if !isWater(neighborID) {
					if other := world.Provinces[neighborID].IslandID; other != island.ID {
						parent[find(int(island.ID))] = find(int(other))
					}
					continue
				}
				if _, seen := distance[neighborID]; !seen && distance[provinceID] < archipelagoMaxGap {
					distance[neighborID] = distance[provinceID] + 1
					queue = append(queue, neighborID)
				}
			}
		}
	}
	groups := map[int][]IslandID{}
	for _, island := range world.Islands {
		root := find(int(island.ID))
		groups[root] = append(groups[root], island.ID)
	}
	var result [][]IslandID
	for _, group := range groups {
		if len(group) >= 2 {
			slices.Sort(group)
			result = append(result, group)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i][0] < result[j][0] })
	return result
}
