package wgvc

// Generate validates config and constructs the deterministic allocation stage
// of a world. Islands and provinces are in canonical ID order. Geometry is
// empty until the deferred planning and tessellation stages are implemented;
// consequently province generating centers and polygon rings are not yet set.
func Generate(config Config) (World, error) {
	if err := config.validate(); err != nil {
		return World{}, err
	}

	allocations, err := allocateProvinces(config.ProvinceCount, config.IslandCount)
	if err != nil {
		return World{}, err
	}

	world := World{
		Islands:   make([]Island, config.IslandCount),
		Provinces: make([]Province, 0, config.ProvinceCount),
	}
	for islandIndex, count := range allocations {
		island := Island{
			ID:          IslandID(islandIndex),
			ProvinceIDs: make([]ProvinceID, 0, count),
		}
		for range count {
			provinceID := ProvinceID(len(world.Provinces))
			island.ProvinceIDs = append(island.ProvinceIDs, provinceID)
			world.Provinces = append(world.Provinces, Province{
				ID:       provinceID,
				IslandID: island.ID,
				Terrain:  TerrainPlains,
			})
		}
		world.Islands[islandIndex] = island
	}
	return world, nil
}
