package wgvc

import "fmt"

// Generate constructs a deterministic world with separated islands, one
// world-level Voronoi mesh, and spatially correlated terrain. All IDs equal
// their indexes in the corresponding world collections.
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
	mesh, islandIDs, land, err := tessellateWorld(layout)
	if err != nil {
		return World{}, fmt.Errorf("tessellate world: %w", err)
	}

	world := World{
		Islands: make([]Island, config.IslandCount),
	}
	for islandIndex, placed := range layout.islands {
		world.Islands[islandIndex] = Island{
			ID:          placed.id,
			ProvinceIDs: make([]ProvinceID, 0, placed.landProvinceCount),
		}
	}
	for cornerID, point := range mesh.corners {
		world.Corners = append(world.Corners, Corner{ID: CornerID(cornerID), Point: point})
	}
	for cellID, cell := range mesh.cells {
		provinceID := ProvinceID(cellID)
		terrain := TerrainWater
		if land[cellID] {
			world.Islands[islandIDs[cellID]].ProvinceIDs = append(world.Islands[islandIDs[cellID]].ProvinceIDs, provinceID)
			terrain = TerrainPlains
		}
		cornerIDs := make([]CornerID, len(cell.cornerIDs))
		for ringIndex, cornerID := range cell.cornerIDs {
			cornerIDs[ringIndex] = CornerID(cornerID)
		}
		world.Provinces = append(world.Provinces, Province{
			ID:        provinceID,
			IslandID:  islandIDs[cellID],
			Center:    cell.center,
			CornerIDs: cornerIDs,
			Terrain:   terrain,
		})
	}
	for edgeIndex, edge := range mesh.edges {
		provinceIDs := make([]ProvinceID, len(edge.siteIndexes))
		for incidenceIndex, cellID := range edge.siteIndexes {
			provinceIDs[incidenceIndex] = ProvinceID(cellID)
		}
		world.Edges = append(world.Edges, Edge{
			ID:          EdgeID(edgeIndex),
			CornerIDs:   [2]CornerID{CornerID(edge.cornerIDs[0]), CornerID(edge.cornerIDs[1])},
			ProvinceIDs: provinceIDs,
		})
	}
	assignTerrain(&world, config.WorldSeed)
	return world, nil
}

// tessellateWorld uses every placed candidate site as a generator in one
// square enclosing the complete layout. Private frame sites still constrain
// each coastline, while their world-level water cells bridge the former gaps
// between independently clipped candidate envelopes.
func tessellateWorld(layout islandLayout) (islandMesh, []IslandID, []bool, error) {
	bounds := squareBounds(layout.bounds)
	side := bounds.max.X - bounds.min.X
	normalize := func(point Point) Point {
		return Point{X: (point.X - bounds.min.X) / side, Y: (point.Y - bounds.min.Y) / side}
	}

	count := 0
	for _, island := range layout.islands {
		count += len(island.candidateMesh.cells)
	}
	sites := make([]Point, 0, count)
	islandIDs := make([]IslandID, 0, count)
	land := make([]bool, 0, count)
	for _, island := range layout.islands {
		selected := make([]bool, len(island.candidateMesh.cells))
		for _, candidateID := range island.landCandidateIDs {
			selected[candidateID] = true
		}
		for candidateID, cell := range island.candidateMesh.cells {
			sites = append(sites, normalize(cell.center))
			islandIDs = append(islandIDs, island.id)
			land = append(land, selected[candidateID])
		}
	}

	mesh, err := tessellateIsland(0, sites)
	if err != nil {
		return islandMesh{}, nil, nil, err
	}
	return transformMesh(mesh, uniformTransform{scale: side, translation: bounds.min}), islandIDs, land, nil
}

func squareBounds(bounds rectangle) rectangle {
	width := bounds.max.X - bounds.min.X
	height := bounds.max.Y - bounds.min.Y
	if width < height {
		padding := (height - width) / 2
		bounds.min.X -= padding
		bounds.max.X += padding
	} else if height < width {
		padding := (width - height) / 2
		bounds.min.Y -= padding
		bounds.max.Y += padding
	}
	return bounds
}
