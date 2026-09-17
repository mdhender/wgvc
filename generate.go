package wgvc

import (
	"fmt"
	"math"

	"github.com/mdhender/wgvc/internal/aspectratio"
	"github.com/mdhender/wgvc/internal/x24"
)

// Generate constructs a deterministic world by growing and merging islands on
// one world-level Voronoi mesh. All IDs equal their indexes in the corresponding
// world collections.
func Generate(config Config) (World, error) {
	if err := config.validate(); err != nil {
		return World{}, err
	}

	growthConfig := x24.DefaultConfig()
	growthConfig.WorldSeed = config.WorldSeed
	growthConfig.ProvinceCount = config.ProvinceCount
	growthConfig.IslandCount = config.IslandCount
	growthConfig.AspectRatio = string(config.AspectRatio)
	world, _, err := GenerateForRender(growthConfig)
	return world, err
}

// GenerateForRender constructs a terrain-assigned world with the complete
// internal growth configuration. Its internal parameter and result types keep
// the calibration controls out of the public generator contract.
func GenerateForRender(growthConfig x24.Config) (World, x24.Result, error) {
	result, err := x24.Generate(growthConfig)
	if err != nil {
		return World{}, x24.Result{}, fmt.Errorf("grow islands: %w", err)
	}

	sites := make([]Point, len(result.Cells))
	for cellID, cell := range result.Cells {
		sites[cellID] = Point{X: cell.Site.X, Y: cell.Site.Y}
	}
	width, height, _ := aspectratio.Dimensions(growthConfig.AspectRatio)
	mesh, err := tessellateRectangle(NoIslandID, sites, width, height)
	if err != nil {
		return World{}, x24.Result{}, fmt.Errorf("tessellate world: %w", err)
	}
	landArea, err := grownLandArea(mesh, result)
	if err != nil {
		return World{}, x24.Result{}, fmt.Errorf("measure land: %w", err)
	}
	scale := math.Sqrt(float64(growthConfig.ProvinceCount) / landArea)
	mesh = transformMesh(mesh, uniformTransform{scale: scale})

	world := World{Islands: make([]Island, len(result.Islands))}
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
		if result.Cells[cellID].IslandID == x24.Water {
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
			Elevation:   0,
		})
	}
	assignTerrain(&world, growthConfig.WorldSeed)
	assignEdgeElevations(&world)
	return world, result, nil
}

func grownLandArea(mesh islandMesh, result x24.Result) (float64, error) {
	if len(mesh.cells) != len(result.Cells) {
		return 0, fmt.Errorf("mesh has %d cells for %d growth cells", len(mesh.cells), len(result.Cells))
	}
	area := 0.0
	for cellID, cell := range mesh.cells {
		if result.Cells[cellID].IslandID == x24.Water {
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
