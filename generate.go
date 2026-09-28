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
	climateConfig, _ := normalizeClimateConfig(ClimateConfig{PolarIce: config.PolarIce, PeakChill: config.PeakChill})
	world, _, err := generateForRender(growthConfig, climateConfig)
	return world, err
}

// GenerateForRender constructs a terrain-assigned world with the complete
// internal growth configuration and default climate calibration.
func GenerateForRender(growthConfig x24.Config) (World, x24.Result, error) {
	return GenerateForRenderWithClimate(growthConfig, DefaultClimateConfig())
}

// GenerateForRenderWithClimate constructs a world using the renderer's full
// growth configuration and explicit climate calibration targets.
func GenerateForRenderWithClimate(growthConfig x24.Config, climateConfig ClimateConfig) (World, x24.Result, error) {
	climateConfig, err := normalizeClimateConfig(climateConfig)
	if err != nil {
		return World{}, x24.Result{}, err
	}
	return generateForRender(growthConfig, climateConfig)
}

func generateForRender(growthConfig x24.Config, climateConfig ClimateConfig) (World, x24.Result, error) {
	result, err := x24.Generate(growthConfig)
	if err != nil {
		return World{}, x24.Result{}, fmt.Errorf("grow islands: %w", err)
	}

	width, height, _ := aspectratio.Dimensions(growthConfig.AspectRatio)
	mesh, err := tessellateGrowthMesh(result, width, height)
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
		if result.Cells[cellID].IslandID == x24.Water {
			islandID = NoIslandID
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
	assignGeometry(&world)
	neighbors := provinceNeighbors(&world)
	assignExits(&world)
	assignElevations(&world, growthConfig.WorldSeed)
	assignRelief(&world)
	assignClimate(&world, growthConfig.WorldSeed, climateConfig)
	assignBasins(&world, neighbors)
	assignRivers(&world)
	assignCoastDistances(&world, neighbors)
	assignSeaZones(&world, neighbors)
	assignStraits(&world, neighbors)
	assignNecks(&world, neighbors)
	assignHarbors(&world, neighbors)
	assignTerrain(&world)
	assignFeatures(&world, neighbors)
	return world, result, nil
}

// tessellateGrowthMesh builds the canonical world mesh from the growth
// result's own Voronoi diagram: the cell rings and the edge list that
// singlemesh kept from its final diagram. Computing the diagram again from
// the same sites would give the same rings and edges, so the canonical
// corner and edge IDs are those tessellateRectangle would assign
// (TestGrowthMeshMatchesTessellateRectangle). Endpoints are snapped to the
// bounds exactly as the backend path snaps them.
func tessellateGrowthMesh(result x24.Result, width, height float64) (islandMesh, error) {
	if len(result.Cells) == 0 {
		return islandMesh{}, fmt.Errorf("growth result has no cells")
	}
	sites := make([]Point, len(result.Cells))
	geometry := backendGeometry{
		cells: make([]backendCell, len(result.Cells)),
		edges: make([]backendEdge, len(result.Edges)),
	}
	for cellID, cell := range result.Cells {
		sites[cellID] = Point{X: cell.Site.X, Y: cell.Site.Y}
		ring := make([]Point, len(cell.Corners))
		for ringIndex, corner := range cell.Corners {
			ring[ringIndex] = snapPointToBounds(Point{X: corner.X, Y: corner.Y}, width, height)
		}
		if signedArea(ring) < 0 {
			reversePoints(ring)
		}
		geometry.cells[cellID] = backendCell{siteIndex: cellID, ring: ring}
	}
	for edgeIndex, edge := range result.Edges {
		geometry.edges[edgeIndex] = backendEdge{
			ends: [2]Point{
				snapPointToBounds(Point{X: edge.Ends[0].X, Y: edge.Ends[0].Y}, width, height),
				snapPointToBounds(Point{X: edge.Ends[1].X, Y: edge.Ends[1].Y}, width, height),
			},
			siteIndexes: append([]int(nil), edge.CellIDs...),
		}
	}
	return canonicalizeInBounds(NoIslandID, sites, geometry, width, height)
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
