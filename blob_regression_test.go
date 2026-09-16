package wgvc

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

type blobRegressionFixture struct {
	name               string
	config             Config
	wantCorners        int
	wantEdges          int
	wantProvinceCounts []int
	wantCoastlineEdges []int
	wantConcaveTurns   []int
	wantSilhouettes    []string
}

func TestBlobShapeRegressionFixtures(t *testing.T) {
	fixtures := []blobRegressionFixture{
		{
			name:               "medium",
			config:             Config{WorldSeed: 42, ProvinceCount: 53, IslandCount: 1},
			wantCorners:        140,
			wantEdges:          192,
			wantProvinceCounts: []int{53},
			wantCoastlineEdges: []int{65},
			wantConcaveTurns:   []int{29},
			wantSilhouettes:    []string{"5b4dbb9b1d3e6575"},
		},
		{
			name:               "asymmetric_multi_island",
			config:             Config{WorldSeed: 8675309, ProvinceCount: 128, IslandCount: 4},
			wantCorners:        344,
			wantEdges:          468,
			wantProvinceCounts: []int{54, 36, 19, 19},
			wantCoastlineEdges: []int{65, 43, 31, 35},
			wantConcaveTurns:   []int{31, 20, 13, 14},
			wantSilhouettes: []string{
				"46741b49952c97ac",
				"1a9511595963dbea",
				"b72385e4d3c4a0fa",
				"32a3bd423a656088",
			},
		},
		{
			name:               "large",
			config:             Config{WorldSeed: 0xdeadbeef, ProvinceCount: 256, IslandCount: 1},
			wantCorners:        580,
			wantEdges:          835,
			wantProvinceCounts: []int{256},
			wantCoastlineEdges: []int{137},
			wantConcaveTurns:   []int{67},
			wantSilhouettes:    []string{"b755c2075650a7c4"},
		},
	}

	seenSilhouettes := make(map[string]string)
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			assertCandidateOceanInvariants(t, fixture.config)

			world, err := Generate(fixture.config)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			assertValidWorld(t, world, fixture.config)

			coastlineEdges := make([]int, len(world.Islands))
			concaveTurns := make([]int, len(world.Islands))
			silhouettes := make([]string, len(world.Islands))
			provinceCounts := make([]int, len(world.Islands))
			for islandIndex, island := range world.Islands {
				provinceCounts[islandIndex] = len(island.ProvinceIDs)
				loop := canonicalCoastlineLoop(t, world, island)
				coastlineEdges[islandIndex] = len(loop)
				concaveTurns[islandIndex] = coastlineConcaveTurns(world, loop)
				silhouettes[islandIndex] = coastlineSignature(world, loop)
				if len(island.ProvinceIDs) >= 20 {
					if len(loop) <= 4 {
						t.Errorf("island %d coastline has only %d edges; want a non-square silhouette", island.ID, len(loop))
					}
					if concaveTurns[islandIndex] == 0 {
						t.Errorf("island %d coastline is convex; want visible bays or indentations", island.ID)
					}
				}
				key := silhouettes[islandIndex]
				label := fmt.Sprintf("%s/island%d", fixture.name, islandIndex)
				if previous, exists := seenSilhouettes[key]; exists {
					t.Errorf("silhouette duplicates %s", previous)
				}
				seenSilhouettes[key] = label
			}

			if len(world.Corners) != fixture.wantCorners || len(world.Edges) != fixture.wantEdges ||
				!slices.Equal(provinceCounts, fixture.wantProvinceCounts) ||
				!slices.Equal(coastlineEdges, fixture.wantCoastlineEdges) ||
				!slices.Equal(concaveTurns, fixture.wantConcaveTurns) ||
				!slices.Equal(silhouettes, fixture.wantSilhouettes) {
				t.Fatalf("fixture changed:\n  provinces: %v\n  corners: %d\n  edges: %d\n  coastline edges: %v\n  concave turns: %v\n  silhouettes: %q",
					provinceCounts, len(world.Corners), len(world.Edges), coastlineEdges, concaveTurns, silhouettes)
			}
		})
	}
}

func assertCandidateOceanInvariants(t *testing.T, config Config) {
	t.Helper()
	allocations, err := allocateProvinces(config.ProvinceCount, config.IslandCount)
	if err != nil {
		t.Fatalf("allocateProvinces() error = %v", err)
	}
	plans, err := planIslands(config, allocations)
	if err != nil {
		t.Fatalf("planIslands() error = %v", err)
	}
	for _, plan := range plans {
		candidate, err := tessellateIsland(plan.id, plan.candidates.sites)
		if err != nil {
			t.Fatalf("island %d tessellate candidates: %v", plan.id, err)
		}
		selectedIDs, err := selectCandidateCells(plan.landProvinceCount, plan.candidates, candidate, plan.shape)
		if err != nil {
			t.Fatalf("island %d select candidates: %v", plan.id, err)
		}
		neighbors, frame, err := candidateMeshTopology(candidate)
		if err != nil {
			t.Fatalf("island %d candidate topology: %v", plan.id, err)
		}
		land := make([]bool, len(candidate.cells))
		for _, candidateID := range selectedIDs {
			land[candidateID] = true
			if frame[candidateID] {
				t.Errorf("island %d selected clipping-frame candidate %d", plan.id, candidateID)
			}
		}
		assertSelectionConnected(t, land, neighbors, plan.landProvinceCount)
		assertWaterReachesFrame(t, land, frame, neighbors)
	}
}

func canonicalCoastlineLoop(t *testing.T, world World, island Island) []CornerID {
	t.Helper()
	coastline := make([]Edge, 0)
	neighbors := make(map[CornerID][]CornerID)
	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) != 1 || world.Provinces[edge.ProvinceIDs[0]].IslandID != island.ID {
			continue
		}
		coastline = append(coastline, edge)
		first, second := edge.CornerIDs[0], edge.CornerIDs[1]
		neighbors[first] = append(neighbors[first], second)
		neighbors[second] = append(neighbors[second], first)
	}
	if len(coastline) < 3 {
		t.Fatalf("island %d coastline has only %d edges", island.ID, len(coastline))
	}
	start := coastline[0].CornerIDs[0]
	for cornerID, adjacent := range neighbors {
		if len(adjacent) != 2 {
			t.Fatalf("island %d coastline corner %d has degree %d, want 2", island.ID, cornerID, len(adjacent))
		}
		if cornerID < start {
			start = cornerID
		}
	}
	sort.Slice(neighbors[start], func(i, j int) bool { return neighbors[start][i] < neighbors[start][j] })
	loop := []CornerID{start}
	previous, current := start, neighbors[start][0]
	for current != start {
		loop = append(loop, current)
		adjacent := neighbors[current]
		if len(adjacent) != 2 {
			t.Fatalf("island %d coastline corner %d has degree %d, want 2", island.ID, current, len(adjacent))
		}
		next := adjacent[0]
		if next == previous {
			next = adjacent[1]
		}
		previous, current = current, next
		if len(loop) > len(coastline) {
			t.Fatalf("island %d coastline does not close", island.ID)
		}
	}
	if len(loop) != len(coastline) {
		t.Fatalf("island %d coastline loop uses %d of %d edges", island.ID, len(loop), len(coastline))
	}
	if loop[0] != minimumCornerID(loop) || loop[1] >= loop[len(loop)-1] {
		t.Fatalf("island %d coastline loop is not canonical: %v", island.ID, loop)
	}
	return loop
}

func coastlineConcaveTurns(world World, loop []CornerID) int {
	points := make([]Point, len(loop))
	for index, cornerID := range loop {
		points[index] = world.Corners[cornerID].Point
	}
	orientation := math.Copysign(1, signedArea(points))
	concave := 0
	for index, point := range points {
		next := points[(index+1)%len(points)]
		after := points[(index+2)%len(points)]
		if orientation*pointCross(point, next, after) < -1e-8 {
			concave++
		}
	}
	return concave
}

func coastlineSignature(world World, loop []CornerID) string {
	minPoint, maxPoint := world.Corners[loop[0]].Point, world.Corners[loop[0]].Point
	for _, cornerID := range loop[1:] {
		point := world.Corners[cornerID].Point
		minPoint.X = math.Min(minPoint.X, point.X)
		minPoint.Y = math.Min(minPoint.Y, point.Y)
		maxPoint.X = math.Max(maxPoint.X, point.X)
		maxPoint.Y = math.Max(maxPoint.Y, point.Y)
	}
	scale := math.Max(maxPoint.X-minPoint.X, maxPoint.Y-minPoint.Y)
	hash := sha256.New()
	for _, cornerID := range loop {
		point := world.Corners[cornerID].Point
		fmt.Fprintf(hash, "%.8f,%.8f;", (point.X-minPoint.X)/scale, (point.Y-minPoint.Y)/scale)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))[:16]
}

func minimumCornerID(values []CornerID) CornerID {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

func TestBlobIslandGallery(t *testing.T) {
	got := renderBlobIslandGallery(t)
	path := filepath.Join("docs", "blob-islands.svg")
	if os.Getenv("WGVC_UPDATE_BLOB_GALLERY") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read retained gallery %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("retained gallery is stale; reproduce with WGVC_UPDATE_BLOB_GALLERY=1 go test -run TestBlobIslandGallery")
	}
}

func renderBlobIslandGallery(t *testing.T) []byte {
	t.Helper()
	fixtures := []Config{
		{WorldSeed: 7, ProvinceCount: 4, IslandCount: 4},
		{WorldSeed: 42, ProvinceCount: 53, IslandCount: 1},
		{WorldSeed: 0xdeadbeef, ProvinceCount: 256, IslandCount: 1},
		{WorldSeed: 8675309, ProvinceCount: 128, IslandCount: 4},
	}

	var svg strings.Builder
	svg.WriteString("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"1200\" height=\"940\" viewBox=\"0 0 1200 940\">\n")
	svg.WriteString("<rect width=\"1200\" height=\"940\" fill=\"#f7f5ef\"/>\n")
	svg.WriteString("<style>text{font-family:ui-monospace,monospace;fill:#17212b}.title{font-size:22px;font-weight:700}.label{font-size:14px}.panel{fill:#dcebf0;stroke:#9babb2;stroke-width:1}.province{fill:#eadfbe;stroke:#7b7567;stroke-width:.7;stroke-linejoin:round}.coast{stroke:#24343d;stroke-width:2.4;stroke-linecap:round}</style>\n")
	svg.WriteString("<text class=\"title\" x=\"24\" y=\"32\">Deterministic blob-island gallery — issue #15</text>\n")
	for fixtureIndex, config := range fixtures {
		world, err := Generate(config)
		if err != nil {
			t.Fatalf("gallery Generate(%+v): %v", config, err)
		}
		panelX := 24.0 + float64(fixtureIndex%2)*588
		panelY := 52.0 + float64(fixtureIndex/2)*422
		const panelWidth, panelHeight = 564.0, 386.0
		fmt.Fprintf(&svg, "<g transform=\"translate(%.0f %.0f)\">\n", panelX, panelY)
		fmt.Fprintf(&svg, "<text class=\"label\" x=\"0\" y=\"15\">seed=%d provinces=%d islands=%d</text>\n", config.WorldSeed, config.ProvinceCount, config.IslandCount)
		fmt.Fprintf(&svg, "<rect class=\"panel\" x=\"0\" y=\"28\" width=\"%.0f\" height=\"%.0f\"/>\n", panelWidth, panelHeight-28)
		transform := galleryTransform(world, panelWidth, panelHeight-28, 18)
		for _, province := range world.Provinces {
			svg.WriteString("<polygon class=\"province\" points=\"")
			for _, cornerID := range province.CornerIDs {
				point := transform(world.Corners[cornerID].Point)
				fmt.Fprintf(&svg, "%.2f,%.2f ", point.X, point.Y+28)
			}
			svg.WriteString("\"/>\n")
		}
		for _, edge := range world.Edges {
			if len(edge.ProvinceIDs) != 1 {
				continue
			}
			first := transform(world.Corners[edge.CornerIDs[0]].Point)
			second := transform(world.Corners[edge.CornerIDs[1]].Point)
			fmt.Fprintf(&svg, "<line class=\"coast\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\"/>\n", first.X, first.Y+28, second.X, second.Y+28)
		}
		svg.WriteString("</g>\n")
	}
	svg.WriteString("<text class=\"label\" x=\"24\" y=\"900\">uniform land fill · thin province boundaries · heavy coastline · no terrain coloring</text>\n")
	svg.WriteString("<text class=\"label\" x=\"24\" y=\"924\">reproduce: WGVC_UPDATE_BLOB_GALLERY=1 go test -run TestBlobIslandGallery</text>\n")
	svg.WriteString("</svg>\n")
	return []byte(svg.String())
}

func galleryTransform(world World, width, height, margin float64) func(Point) Point {
	minimum, maximum := world.Corners[0].Point, world.Corners[0].Point
	for _, corner := range world.Corners[1:] {
		minimum.X = math.Min(minimum.X, corner.Point.X)
		minimum.Y = math.Min(minimum.Y, corner.Point.Y)
		maximum.X = math.Max(maximum.X, corner.Point.X)
		maximum.Y = math.Max(maximum.Y, corner.Point.Y)
	}
	worldWidth, worldHeight := maximum.X-minimum.X, maximum.Y-minimum.Y
	scale := math.Min((width-2*margin)/worldWidth, (height-2*margin)/worldHeight)
	xOffset := margin + (width-2*margin-worldWidth*scale)/2
	yOffset := margin + (height-2*margin-worldHeight*scale)/2
	return func(point Point) Point {
		return Point{
			X: xOffset + (point.X-minimum.X)*scale,
			Y: yOffset + (maximum.Y-point.Y)*scale,
		}
	}
}
