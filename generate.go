package wgvc

import (
	"errors"
	"fmt"
	"math"

	"github.com/mdhender/wgvc/internal/x23"
)

const (
	productionOceanPercentage  = 0.68
	preferredEdgeDistance      = 3
	preferredIslandDistance    = 3
	fallbackEdgeDistance       = 1
	fallbackIslandDistance     = 2
	productionMaximumRounds    = 20
	productionLloydRelaxations = 2
)

// Generate constructs a deterministic world by growing separated islands on
// one world-level Voronoi mesh. All IDs equal their indexes in the
// corresponding world collections.
func Generate(config Config) (World, error) {
	if err := config.validate(); err != nil {
		return World{}, err
	}

	growthConfig := x23.Config{
		WorldSeed:         config.WorldSeed,
		ProvinceCount:     config.ProvinceCount,
		IslandCount:       config.IslandCount,
		OceanPercentage:   productionOceanPercentage,
		MinEdgeDistance:   preferredEdgeDistance,
		MinIslandDistance: preferredIslandDistance,
		MaxRounds:         productionMaximumRounds,
		Relaxations:       productionLloydRelaxations,
	}
	result, err := x23.Generate(growthConfig)
	if errors.Is(err, x23.ErrUnsatisfiable) {
		growthConfig.MinEdgeDistance = fallbackEdgeDistance
		growthConfig.MinIslandDistance = fallbackIslandDistance
		result, err = x23.Generate(growthConfig)
	}
	if err != nil {
		return World{}, fmt.Errorf("grow islands: %w", err)
	}

	sites := make([]Point, len(result.Cells))
	for cellID, cell := range result.Cells {
		sites[cellID] = Point{X: cell.Site.X, Y: cell.Site.Y}
	}
	mesh, err := tessellateIsland(NoIslandID, sites)
	if err != nil {
		return World{}, fmt.Errorf("tessellate world: %w", err)
	}
	landArea, err := grownLandArea(mesh, result)
	if err != nil {
		return World{}, fmt.Errorf("measure land: %w", err)
	}
	scale := math.Sqrt(float64(config.ProvinceCount) / landArea)
	mesh = transformMesh(mesh, uniformTransform{scale: scale})

	world := World{Islands: make([]Island, config.IslandCount)}
	for islandID := range world.Islands {
		world.Islands[islandID].ID = IslandID(islandID)
	}
	for cornerID, point := range mesh.corners {
		world.Corners = append(world.Corners, Corner{ID: CornerID(cornerID), Point: point})
	}
	for cellID, cell := range mesh.cells {
		provinceID := ProvinceID(cellID)
		islandID := IslandID(result.Cells[cellID].IslandID)
		terrain := TerrainPlains
		if result.Cells[cellID].IslandID == x23.Water {
			islandID = NoIslandID
			terrain = TerrainWater
		} else {
			world.Islands[islandID].ProvinceIDs = append(world.Islands[islandID].ProvinceIDs, provinceID)
		}
		cornerIDs := make([]CornerID, len(cell.cornerIDs))
		for ringIndex, cornerID := range cell.cornerIDs {
			cornerIDs[ringIndex] = CornerID(cornerID)
		}
		world.Provinces = append(world.Provinces, Province{
			ID:        provinceID,
			IslandID:  islandID,
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

func grownLandArea(mesh islandMesh, result x23.Result) (float64, error) {
	if len(mesh.cells) != len(result.Cells) {
		return 0, fmt.Errorf("mesh has %d cells for %d growth cells", len(mesh.cells), len(result.Cells))
	}
	area := 0.0
	for cellID, cell := range mesh.cells {
		if result.Cells[cellID].IslandID == x23.Water {
			continue
		}
		ring := make([]Point, len(cell.cornerIDs))
		for ringIndex, cornerID := range cell.cornerIDs {
			ring[ringIndex] = mesh.corners[cornerID]
		}
		area += signedArea(ring)
	}
	if math.IsNaN(area) || math.IsInf(area, 0) || area <= 0 {
		return 0, fmt.Errorf("land area must be finite and positive: %g", area)
	}
	return area, nil
}
