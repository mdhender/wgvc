package wgvc

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// centeredIslandTile is the experiment's independent land-only tile. Its
// tight land bounds, rather than the former candidate square, are centered on
// the local origin.
type centeredIslandTile struct {
	id     IslandID
	mesh   islandMesh
	bounds rectangle
}

type placedIslandTile struct {
	center Point
	mesh   islandMesh
	bounds rectangle
}

type oceanTileExperiment struct {
	bounds  rectangle
	islands []placedIslandTile
}

func TestCenteredLandOnlyTileExperiment(t *testing.T) {
	config := Config{WorldSeed: 0x0123456789abcdef, ProvinceCount: 137, IslandCount: 11}
	tiles := buildCenteredIslandTiles(t, config)
	if got := len(tiles); got != config.IslandCount {
		t.Fatalf("tile count = %d, want %d", got, config.IslandCount)
	}

	land := 0
	for tileIndex, tile := range tiles {
		if tile.id != IslandID(tileIndex) {
			t.Errorf("tile %d ID = %d", tileIndex, tile.id)
		}
		land += len(tile.mesh.cells)
		center := rectangleCenter(tile.bounds)
		if math.Abs(center.X) > 1e-12 || math.Abs(center.Y) > 1e-12 {
			t.Errorf("tile %d bounds center = %+v, want (0,0)", tile.id, center)
		}
		if got, want := meshArea(tile.mesh), float64(len(tile.mesh.cells)); math.Abs(got-want) > 1e-8*want {
			t.Errorf("tile %d land area = %.17g, want %g", tile.id, got, want)
		}
	}
	if land != config.ProvinceCount {
		t.Errorf("land cells = %d, want %d", land, config.ProvinceCount)
	}

	first := placeCenteredIslandTiles(t, config.WorldSeed, tiles)
	second := placeCenteredIslandTiles(t, config.WorldSeed, tiles)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("identical seed and tiles produced different ocean layouts")
	}
	assertValidOceanTileLayout(t, first)
}

func TestCenteredLandOnlyTilePlacementIsSeedSensitive(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 53, IslandCount: 7}
	tiles := buildCenteredIslandTiles(t, config)
	first := placeCenteredIslandTiles(t, config.WorldSeed, tiles)
	second := placeCenteredIslandTiles(t, config.WorldSeed+1, tiles)
	if reflect.DeepEqual(first, second) {
		t.Fatal("different placement seeds produced identical ocean layouts")
	}
	assertValidOceanTileLayout(t, first)
	assertValidOceanTileLayout(t, second)
}

func buildCenteredIslandTiles(t *testing.T, config Config) []centeredIslandTile {
	t.Helper()
	allocations, err := allocateProvinces(config.ProvinceCount, config.IslandCount)
	if err != nil {
		t.Fatalf("allocateProvinces() error = %v", err)
	}
	plans, err := planIslands(config, allocations)
	if err != nil {
		t.Fatalf("planIslands() error = %v", err)
	}
	meshes, err := tessellateIslands(plans)
	if err != nil {
		t.Fatalf("tessellateIslands() error = %v", err)
	}

	tiles := make([]centeredIslandTile, len(plans))
	for islandIndex, plan := range plans {
		mesh := meshes.islands[islandIndex]
		area, err := retainedLandArea(mesh)
		if err != nil {
			t.Fatalf("island %d retainedLandArea() error = %v", plan.id, err)
		}
		scale := math.Sqrt(float64(plan.landProvinceCount) / area)
		bounds := meshBounds(mesh)
		center := rectangleCenter(bounds)
		transform := uniformTransform{
			scale: scale,
			translation: Point{
				X: -scale * center.X,
				Y: -scale * center.Y,
			},
		}
		centered := transformMesh(mesh, transform)
		tiles[islandIndex] = centeredIslandTile{
			id:     plan.id,
			mesh:   centered,
			bounds: meshBounds(centered),
		}
	}
	return tiles
}

// placeCenteredIslandTiles treats the ocean as one square container and uses
// seeded rejection packing for the land-only tiles. It deliberately stops at
// the overlay boundary: a single convex public Province cannot represent the
// resulting ocean-with-holes topology.
func placeCenteredIslandTiles(t *testing.T, worldSeed uint64, tiles []centeredIslandTile) oceanTileExperiment {
	t.Helper()
	maxSpan, paddedArea := 0.0, 0.0
	for _, tile := range tiles {
		width := tile.bounds.max.X - tile.bounds.min.X
		height := tile.bounds.max.Y - tile.bounds.min.Y
		maxSpan = math.Max(maxSpan, math.Max(width, height)+2*islandWorldMargin)
		paddedArea += (width + islandWaterGap) * (height + islandWaterGap)
	}

	side := math.Max(maxSpan, 1.5*math.Sqrt(paddedArea)+2*islandWorldMargin)
	for expansion := 0; expansion < 20; expansion++ {
		random := placementRandom(worldSeed)
		half := side / 2
		placed := make([]placedIslandTile, 0, len(tiles))
		for _, tile := range tiles {
			width := tile.bounds.max.X - tile.bounds.min.X
			height := tile.bounds.max.Y - tile.bounds.min.Y
			xLimit := half - islandWorldMargin - width/2
			yLimit := half - islandWorldMargin - height/2
			accepted := false
			for attempt := 0; attempt < 10_000; attempt++ {
				center := Point{
					X: (2*random.Float64() - 1) * xLimit,
					Y: (2*random.Float64() - 1) * yLimit,
				}
				bounds := translateRectangle(tile.bounds, center)
				if overlapsPlacedTiles(bounds, placed) {
					continue
				}
				placed = append(placed, placedIslandTile{
					center: center,
					mesh: transformMesh(tile.mesh, uniformTransform{
						scale:       1,
						translation: center,
					}),
					bounds: bounds,
				})
				accepted = true
				break
			}
			if !accepted {
				break
			}
		}
		if len(placed) == len(tiles) {
			return oceanTileExperiment{
				bounds:  rectangle{min: Point{X: -half, Y: -half}, max: Point{X: half, Y: half}},
				islands: placed,
			}
		}
		side *= 1.2
	}
	t.Fatal("could not pack centered island tiles after expanding the ocean tile")
	return oceanTileExperiment{}
}

func assertValidOceanTileLayout(t *testing.T, layout oceanTileExperiment) {
	t.Helper()
	for islandIndex, island := range layout.islands {
		if island.bounds.min.X-layout.bounds.min.X < islandWorldMargin-1e-9 ||
			island.bounds.min.Y-layout.bounds.min.Y < islandWorldMargin-1e-9 ||
			layout.bounds.max.X-island.bounds.max.X < islandWorldMargin-1e-9 ||
			layout.bounds.max.Y-island.bounds.max.Y < islandWorldMargin-1e-9 {
			t.Errorf("island tile %d lies outside the ocean margin", islandIndex)
		}
		for other := islandIndex + 1; other < len(layout.islands); other++ {
			if gap := rectangleGap(island.bounds, layout.islands[other].bounds); gap < islandWaterGap-1e-9 {
				t.Errorf("island tiles %d and %d gap = %g, want at least %g", islandIndex, other, gap, islandWaterGap)
			}
		}
	}
}

func overlapsPlacedTiles(bounds rectangle, placed []placedIslandTile) bool {
	for _, island := range placed {
		if rectangleGap(bounds, island.bounds) < islandWaterGap {
			return true
		}
	}
	return false
}

func meshBounds(mesh islandMesh) rectangle {
	bounds := rectangle{min: mesh.corners[0], max: mesh.corners[0]}
	for _, point := range mesh.corners[1:] {
		bounds.min.X = math.Min(bounds.min.X, point.X)
		bounds.min.Y = math.Min(bounds.min.Y, point.Y)
		bounds.max.X = math.Max(bounds.max.X, point.X)
		bounds.max.Y = math.Max(bounds.max.Y, point.Y)
	}
	return bounds
}

func rectangleCenter(bounds rectangle) Point {
	return Point{X: (bounds.min.X + bounds.max.X) / 2, Y: (bounds.min.Y + bounds.max.Y) / 2}
}

func translateRectangle(bounds rectangle, translation Point) rectangle {
	return rectangle{
		min: Point{X: bounds.min.X + translation.X, Y: bounds.min.Y + translation.Y},
		max: Point{X: bounds.max.X + translation.X, Y: bounds.max.Y + translation.Y},
	}
}

func TestCenteredIslandTileGallery(t *testing.T) {
	got := renderCenteredIslandTileGallery(t)
	path := filepath.Join("docs", "tile-experiment.svg")
	if os.Getenv("WGVC_UPDATE_TILE_EXPERIMENT") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read retained experiment gallery %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("experiment gallery is stale; reproduce with WGVC_UPDATE_TILE_EXPERIMENT=1 go test -run TestCenteredIslandTileGallery")
	}
}

func renderCenteredIslandTileGallery(t *testing.T) []byte {
	t.Helper()
	config := Config{WorldSeed: 0x0123456789abcdef, ProvinceCount: 137, IslandCount: 11}
	world, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	layout := placeCenteredIslandTiles(t, config.WorldSeed, buildCenteredIslandTiles(t, config))

	var svg strings.Builder
	svg.WriteString("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"1200\" height=\"620\" viewBox=\"0 0 1200 620\">\n")
	svg.WriteString("<rect width=\"1200\" height=\"620\" fill=\"#f7f5ef\"/>\n")
	svg.WriteString("<style>text{font-family:ui-monospace,monospace;fill:#17212b}.title{font-size:22px;font-weight:700}.label{font-size:14px}.panel{fill:#c6e3ec;stroke:#9babb2}.province{fill:#eadfbe;stroke:#7b7567;stroke-width:.7;stroke-linejoin:round}.water{fill:#c6e3ec;stroke:#83aab8}.coast{fill:none;stroke:#24343d;stroke-width:2.4;stroke-linejoin:round}</style>\n")
	svg.WriteString("<text class=\"title\" x=\"24\" y=\"32\">Centered land-only island tile experiment — issue #18</text>\n")
	renderCurrentWorldPanel(&svg, world, 24, 56, 564, 500)
	renderOceanTilePanel(&svg, layout, 612, 56, 564, 500)
	svg.WriteString("<text class=\"label\" x=\"24\" y=\"590\">Right panel is a layout overlay, not valid public World topology: one convex ocean province cannot contain holes.</text>\n")
	svg.WriteString("</svg>\n")
	return []byte(svg.String())
}

func renderCurrentWorldPanel(svg *strings.Builder, world World, x, y, width, height float64) {
	fmt.Fprintf(svg, "<g transform=\"translate(%.0f %.0f)\"><text class=\"label\" x=\"0\" y=\"15\">current: one global Voronoi mesh</text><rect class=\"panel\" x=\"0\" y=\"28\" width=\"%.0f\" height=\"%.0f\"/>\n", x, y, width, height-28)
	transform := galleryTransform(world, width, height-28, 18)
	for _, province := range world.Provinces {
		class := "province"
		if province.Terrain == TerrainWater {
			class = "water"
		}
		fmt.Fprintf(svg, "<polygon class=\"%s\" points=\"", class)
		for _, cornerID := range province.CornerIDs {
			point := transform(world.Corners[cornerID].Point)
			fmt.Fprintf(svg, "%.2f,%.2f ", point.X, point.Y+28)
		}
		svg.WriteString("\"/>\n")
	}
	svg.WriteString("</g>\n")
}

func renderOceanTilePanel(svg *strings.Builder, layout oceanTileExperiment, x, y, width, height float64) {
	fmt.Fprintf(svg, "<g transform=\"translate(%.0f %.0f)\"><text class=\"label\" x=\"0\" y=\"15\">experiment: centered land tiles on one ocean tile</text><rect class=\"panel\" x=\"0\" y=\"28\" width=\"%.0f\" height=\"%.0f\"/>\n", x, y, width, height-28)
	transform := rectangleTransform(layout.bounds, width, height-28, 18)
	for _, island := range layout.islands {
		for _, cell := range island.mesh.cells {
			svg.WriteString("<polygon class=\"province\" points=\"")
			for _, cornerID := range cell.cornerIDs {
				point := transform(island.mesh.corners[cornerID])
				fmt.Fprintf(svg, "%.2f,%.2f ", point.X, point.Y+28)
			}
			svg.WriteString("\"/>\n")
		}
	}
	svg.WriteString("</g>\n")
}

func rectangleTransform(bounds rectangle, width, height, margin float64) func(Point) Point {
	worldWidth := bounds.max.X - bounds.min.X
	worldHeight := bounds.max.Y - bounds.min.Y
	scale := math.Min((width-2*margin)/worldWidth, (height-2*margin)/worldHeight)
	xOffset := margin + (width-2*margin-worldWidth*scale)/2
	yOffset := margin + (height-2*margin-worldHeight*scale)/2
	return func(point Point) Point {
		return Point{
			X: xOffset + (point.X-bounds.min.X)*scale,
			Y: yOffset + (bounds.max.Y-point.Y)*scale,
		}
	}
}
