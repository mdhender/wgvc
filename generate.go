package wgvc

import "fmt"

// Generate constructs a deterministic world with separated islands, clipped
// Voronoi provinces, and spatially correlated terrain. All IDs equal their
// indexes in the corresponding world collections.
func Generate(config Config) (World, error) {
	if err := config.validate(); err != nil {
		return World{}, err
	}

	allocations, err := allocateProvinces(config.ProvinceCount, config.IslandCount)
	if err != nil {
		return World{}, fmt.Errorf("allocate provinces: %w", err)
	}
	plans, err := planIslands(config, allocations)
	if err != nil {
		return World{}, fmt.Errorf("plan islands: %w", err)
	}
	meshes, err := tessellateIslands(plans)
	if err != nil {
		return World{}, fmt.Errorf("tessellate islands: %w", err)
	}
	layout, err := placeIslands(config, plans, meshes)
	if err != nil {
		return World{}, fmt.Errorf("place islands: %w", err)
	}

	world := World{
		Islands:   make([]Island, config.IslandCount),
		Provinces: make([]Province, 0, config.ProvinceCount),
	}
	for islandIndex, placed := range layout.islands {
		mesh := placed.mesh
		provinceOffset := len(world.Provinces)
		cornerOffset := len(world.Corners)

		island := Island{
			ID:          IslandID(islandIndex),
			ProvinceIDs: make([]ProvinceID, len(mesh.cells)),
		}
		for localCornerID, point := range mesh.corners {
			cornerID := CornerID(cornerOffset + localCornerID)
			world.Corners = append(world.Corners, Corner{
				ID:    cornerID,
				Point: point,
			})
		}
		for localProvinceID, cell := range mesh.cells {
			provinceID := ProvinceID(provinceOffset + localProvinceID)
			island.ProvinceIDs[localProvinceID] = provinceID
			cornerIDs := make([]CornerID, len(cell.cornerIDs))
			for ringIndex, localCornerID := range cell.cornerIDs {
				cornerIDs[ringIndex] = CornerID(cornerOffset + localCornerID)
			}
			world.Provinces = append(world.Provinces, Province{
				ID:        provinceID,
				IslandID:  island.ID,
				Center:    cell.center,
				CornerIDs: cornerIDs,
				Terrain:   TerrainPlains,
			})
		}
		for _, edge := range mesh.edges {
			provinceIDs := make([]ProvinceID, len(edge.siteIndexes))
			for incidenceIndex, localProvinceID := range edge.siteIndexes {
				provinceIDs[incidenceIndex] = ProvinceID(provinceOffset + localProvinceID)
			}
			edgeID := EdgeID(len(world.Edges))
			world.Edges = append(world.Edges, Edge{
				ID: edgeID,
				CornerIDs: [2]CornerID{
					CornerID(cornerOffset + edge.cornerIDs[0]),
					CornerID(cornerOffset + edge.cornerIDs[1]),
				},
				ProvinceIDs: provinceIDs,
			})
		}
		world.Islands[islandIndex] = island
	}
	assignTerrain(&world, config.WorldSeed)
	return world, nil
}
