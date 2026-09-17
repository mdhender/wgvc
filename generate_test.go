package wgvc

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestGenerateRejectsInvalidConfigWithoutPartialWorld(t *testing.T) {
	for _, config := range []Config{
		{},
		{ProvinceCount: 1, IslandCount: 0},
		{ProvinceCount: 0, IslandCount: 1},
		{ProvinceCount: -1, IslandCount: 1},
		{ProvinceCount: 2, IslandCount: 3},
		{ProvinceCount: 2, IslandCount: 1, AspectRatio: "16/9"},
		{ProvinceCount: 2, IslandCount: 1, AspectRatio: "0:1"},
	} {
		world, err := Generate(config)
		if err == nil {
			t.Errorf("Generate(%+v) returned no error", config)
		}
		if !reflect.DeepEqual(world, World{}) {
			t.Errorf("Generate(%+v) returned partial world %+v", config, world)
		}
	}
}

func TestGenerateValidWorlds(t *testing.T) {
	seeds := []uint64{0, 1, 0xdeadbeef}
	for provinceCount := 1; provinceCount <= 12; provinceCount++ {
		for islandCount := 1; islandCount <= provinceCount; islandCount++ {
			for _, seed := range seeds {
				config := Config{WorldSeed: seed, ProvinceCount: provinceCount, IslandCount: islandCount}
				t.Run(fmt.Sprintf("p%d/i%d/s%d", provinceCount, islandCount, seed), func(t *testing.T) {
					world, err := Generate(config)
					if err != nil {
						t.Fatalf("Generate() error = %v", err)
					}
					assertValidWorld(t, world, config)
				})
			}
		}
	}

	for _, config := range []Config{
		{WorldSeed: 42, ProvinceCount: 97, IslandCount: 7},
		{WorldSeed: 8675309, ProvinceCount: 128, IslandCount: 13},
	} {
		t.Run(fmt.Sprintf("p%d/i%d", config.ProvinceCount, config.IslandCount), func(t *testing.T) {
			world, err := Generate(config)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			assertValidWorld(t, world, config)
		})
	}
}

func TestGenerateUsesOneMeshForLandAndWater(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 20, IslandCount: 4}
	world, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	land, water := 0, 0
	for _, province := range world.Provinces {
		if province.Terrain == TerrainWater {
			water++
			if province.IslandID != NoIslandID {
				t.Errorf("water province %d island ID = %d, want NoIslandID", province.ID, province.IslandID)
			}
			continue
		}
		land++
		if province.IslandID == NoIslandID {
			t.Errorf("land province %d has NoIslandID", province.ID)
		}
	}
	if land != config.ProvinceCount || water == 0 {
		t.Fatalf("world has %d land and %d water provinces, want %d land and nonzero water", land, water, config.ProvinceCount)
	}
}

func TestGenerateMergesDirectlyConnectedIslands(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 20, IslandCount: 4}
	world, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got := len(world.Islands); got != 1 {
		t.Fatalf("surviving island count = %d, want 1 after mergers", got)
	}
	if got := len(world.Islands[0].ProvinceIDs); got != config.ProvinceCount {
		t.Fatalf("merged island has %d land provinces, want %d", got, config.ProvinceCount)
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	config := Config{WorldSeed: 1234, ProvinceCount: 40, IslandCount: 6}
	first, err := Generate(config)
	if err != nil {
		t.Fatalf("first Generate() error = %v", err)
	}
	second, err := Generate(config)
	if err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("Generate() results differ for identical inputs")
	}
}

func TestGenerateUsesFixedAreaAspectRatioBounds(t *testing.T) {
	for _, aspect := range []AspectRatio{AspectRatioStandard, AspectRatioWidescreen, AspectRatioCinema, AspectRatioPortrait} {
		t.Run(string(aspect), func(t *testing.T) {
			config := Config{WorldSeed: 42, ProvinceCount: 60, IslandCount: 5, AspectRatio: aspect}
			world, err := Generate(config)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			assertValidWorld(t, world, config)

			minimum, maximum := world.Corners[0].Point, world.Corners[0].Point
			for _, corner := range world.Corners[1:] {
				minimum.X, minimum.Y = math.Min(minimum.X, corner.Point.X), math.Min(minimum.Y, corner.Point.Y)
				maximum.X, maximum.Y = math.Max(maximum.X, corner.Point.X), math.Max(maximum.Y, corner.Point.Y)
			}
			widthRatio, heightRatio, _ := config.aspectDimensions()
			wantRatio := widthRatio / heightRatio
			gotRatio := (maximum.X - minimum.X) / (maximum.Y - minimum.Y)
			if math.Abs(gotRatio-wantRatio) > geometryTolerance*wantRatio {
				t.Errorf("world bounds ratio = %.17g, want %.17g", gotRatio, wantRatio)
			}
		})
	}
}

func TestGenerateZeroAspectRatioMatchesSquare(t *testing.T) {
	config := Config{WorldSeed: 1234, ProvinceCount: 40, IslandCount: 6}
	zero, err := Generate(config)
	if err != nil {
		t.Fatalf("zero-value aspect Generate() error = %v", err)
	}
	config.AspectRatio = AspectRatioSquare
	square, err := Generate(config)
	if err != nil {
		t.Fatalf("square Generate() error = %v", err)
	}
	if !reflect.DeepEqual(zero, square) {
		t.Fatal("zero-value aspect ratio does not produce the square default")
	}
}

func TestGenerateConcurrentCallsAreIndependent(t *testing.T) {
	config := Config{WorldSeed: 1234, ProvinceCount: 40, IslandCount: 6}
	want, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	const calls = 24
	errors := make(chan error, calls)
	for range calls {
		go func() {
			got, err := Generate(config)
			if err != nil {
				errors <- err
				return
			}
			if !reflect.DeepEqual(got, want) {
				errors <- fmt.Errorf("concurrent Generate result differs")
				return
			}
			errors <- nil
		}()
	}
	for range calls {
		if err := <-errors; err != nil {
			t.Error(err)
		}
	}
}

func TestGenerateKeepsWaterProvinces(t *testing.T) {
	config := Config{WorldSeed: 0x0123456789abcdef, ProvinceCount: 137, IslandCount: 11}
	world, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	assertValidWorld(t, world, config)

	land, water := 0, 0
	for _, province := range world.Provinces {
		if province.Terrain == TerrainWater {
			water++
		} else {
			land++
		}
	}
	if land != config.ProvinceCount {
		t.Errorf("land province count = %d, want %d", land, config.ProvinceCount)
	}
	if water == 0 {
		t.Error("world has no water provinces")
	}
}

func assertValidWorld(t *testing.T, world World, config Config) {
	t.Helper()
	if got := len(world.Islands); got < 1 || got > config.IslandCount {
		t.Fatalf("surviving island count = %d, want between 1 and %d", got, config.IslandCount)
	}
	if got := len(world.Provinces); got <= config.ProvinceCount {
		t.Fatalf("total province count = %d, want more than %d land provinces", got, config.ProvinceCount)
	}

	memberships := make([]int, len(world.Provinces))
	for islandIndex, island := range world.Islands {
		if island.ID != IslandID(islandIndex) {
			t.Errorf("island at index %d has ID %d", islandIndex, island.ID)
		}
		if len(island.ProvinceIDs) == 0 {
			t.Errorf("island %d has no provinces", island.ID)
		}
		for membershipIndex, provinceID := range island.ProvinceIDs {
			if int(provinceID) < 0 || int(provinceID) >= len(world.Provinces) {
				t.Fatalf("island %d has invalid province ID %d", island.ID, provinceID)
			}
			if membershipIndex > 0 && island.ProvinceIDs[membershipIndex-1] >= provinceID {
				t.Errorf("island %d memberships are not canonical: %v", island.ID, island.ProvinceIDs)
			}
			memberships[provinceID]++
			if world.Provinces[provinceID].IslandID != island.ID {
				t.Errorf("island %d contains province %d assigned to island %d", island.ID, provinceID, world.Provinces[provinceID].IslandID)
			}
		}
	}
	for provinceID, count := range memberships {
		want := 1
		if world.Provinces[provinceID].Terrain == TerrainWater {
			want = 0
		}
		if count != want {
			t.Errorf("province %d appears in %d island memberships, want %d", provinceID, count, want)
		}
	}

	for cornerID, corner := range world.Corners {
		if corner.ID != CornerID(cornerID) {
			t.Errorf("corner at index %d has ID %d", cornerID, corner.ID)
		}
		if !finitePoint(corner.Point) {
			t.Errorf("corner %d is not finite: %+v", corner.ID, corner.Point)
		}
	}

	edgesByCorners := make(map[[2]CornerID]EdgeID, len(world.Edges))
	neighbors := make([][]ProvinceID, len(world.Provinces))
	edgeTraversals := make([][][2]CornerID, len(world.Edges))
	cornerInProvince := make([]bool, len(world.Corners))
	elevationTotals := make([]float64, len(world.Provinces))
	elevationEdgeCounts := make([]int, len(world.Provinces))
	noise := newTerrainNoise(config.WorldSeed)
	cornerValues := make([]float64, len(world.Corners))
	for cornerID, corner := range world.Corners {
		cornerValues[cornerID] = noise.sample(corner.Point)
	}
	for edgeIndex, edge := range world.Edges {
		if edge.ID != EdgeID(edgeIndex) {
			t.Errorf("edge at index %d has ID %d", edgeIndex, edge.ID)
		}
		if edge.CornerIDs[0] < 0 || int(edge.CornerIDs[1]) >= len(world.Corners) || edge.CornerIDs[0] >= edge.CornerIDs[1] {
			t.Fatalf("edge %d has invalid corners %v", edge.ID, edge.CornerIDs)
		}
		if previous, exists := edgesByCorners[edge.CornerIDs]; exists {
			t.Errorf("edges %d and %d have identical corners %v", previous, edge.ID, edge.CornerIDs)
		}
		edgesByCorners[edge.CornerIDs] = edge.ID
		if len(edge.ProvinceIDs) != 1 && len(edge.ProvinceIDs) != 2 {
			t.Fatalf("edge %d has invalid incidence %v", edge.ID, edge.ProvinceIDs)
		}
		for incidenceIndex, provinceID := range edge.ProvinceIDs {
			if int(provinceID) < 0 || int(provinceID) >= len(world.Provinces) {
				t.Fatalf("edge %d has invalid province ID %d", edge.ID, provinceID)
			}
			if incidenceIndex > 0 && edge.ProvinceIDs[incidenceIndex-1] >= provinceID {
				t.Errorf("edge %d incidence is not canonical: %v", edge.ID, edge.ProvinceIDs)
			}
		}
		if len(edge.ProvinceIDs) == 1 && world.Provinces[edge.ProvinceIDs[0]].Terrain != TerrainWater {
			t.Errorf("world-boundary edge %d is incident to land province %d", edge.ID, edge.ProvinceIDs[0])
		}
		if len(edge.ProvinceIDs) == 2 {
			first, second := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
			firstWater := world.Provinces[first].Terrain == TerrainWater
			secondWater := world.Provinces[second].Terrain == TerrainWater
			if !firstWater && !secondWater && world.Provinces[first].IslandID != world.Provinces[second].IslandID {
				t.Errorf("interior edge %d joins islands %d and %d", edge.ID, world.Provinces[first].IslandID, world.Provinces[second].IslandID)
			}
			if !firstWater && !secondWater && world.Provinces[first].IslandID == world.Provinces[second].IslandID {
				neighbors[first] = append(neighbors[first], second)
				neighbors[second] = append(neighbors[second], first)
			}
		}
		wantElevation := 0.0
		if len(edge.ProvinceIDs) == 2 {
			firstLand := world.Provinces[edge.ProvinceIDs[0]].IslandID != NoIslandID
			secondLand := world.Provinces[edge.ProvinceIDs[1]].IslandID != NoIslandID
			if firstLand == secondLand {
				edgeNoise := (cornerValues[edge.CornerIDs[0]] + cornerValues[edge.CornerIDs[1]]) / 2
				if firstLand {
					wantElevation = elevationLandMargin + (1-elevationLandMargin)*edgeNoise
				} else {
					wantElevation = -1 + (1-elevationWaterMargin)*edgeNoise
				}
			}
		}
		if edge.Elevation != wantElevation {
			t.Errorf("edge %d elevation = %g, want %g", edge.ID, edge.Elevation, wantElevation)
		}
		if math.IsNaN(edge.Elevation) || math.IsInf(edge.Elevation, 0) || edge.Elevation < -1 || edge.Elevation > 1 {
			t.Errorf("edge %d elevation = %g, want finite value in [-1, 1]", edge.ID, edge.Elevation)
		}
		for _, provinceID := range edge.ProvinceIDs {
			elevationTotals[provinceID] += edge.Elevation
			elevationEdgeCounts[provinceID]++
		}
	}

	landArea := 0.0
	totalArea := 0.0
	minimum, maximum := world.Corners[0].Point, world.Corners[0].Point
	for _, corner := range world.Corners[1:] {
		minimum.X = math.Min(minimum.X, corner.Point.X)
		minimum.Y = math.Min(minimum.Y, corner.Point.Y)
		maximum.X = math.Max(maximum.X, corner.Point.X)
		maximum.Y = math.Max(maximum.Y, corner.Point.Y)
	}
	for provinceIndex, province := range world.Provinces {
		if province.ID != ProvinceID(provinceIndex) {
			t.Errorf("province at index %d has ID %d", provinceIndex, province.ID)
		}
		if province.Terrain == TerrainWater {
			if province.IslandID != NoIslandID {
				t.Errorf("water province %d has island ID %d, want NoIslandID", province.ID, province.IslandID)
			}
		} else if int(province.IslandID) < 0 || int(province.IslandID) >= len(world.Islands) {
			t.Fatalf("land province %d has invalid island ID %d", province.ID, province.IslandID)
		}
		switch province.Terrain {
		case TerrainWater, TerrainPlains, TerrainHills, TerrainMountains:
		default:
			t.Errorf("province %d has unsupported terrain %q", province.ID, province.Terrain)
		}
		wantElevation := elevationTotals[provinceIndex] / float64(elevationEdgeCounts[provinceIndex])
		if province.Elevation != wantElevation {
			t.Errorf("province %d elevation = %g, want boundary-edge mean %g", province.ID, province.Elevation, wantElevation)
		}
		if math.IsNaN(province.Elevation) || math.IsInf(province.Elevation, 0) || province.Elevation < -1 || province.Elevation > 1 {
			t.Errorf("province %d elevation = %g, want finite value in [-1, 1]", province.ID, province.Elevation)
		}
		wantBand := classifyElevation(province.IslandID != NoIslandID, province.Elevation)
		if province.ElevationBand != wantBand {
			t.Errorf("province %d elevation band = %d, want %d", province.ID, province.ElevationBand, wantBand)
		}
		if !finitePoint(province.Center) {
			t.Errorf("province %d center is not finite: %+v", province.ID, province.Center)
		}
		if len(province.CornerIDs) < 3 {
			t.Fatalf("province %d has only %d corners", province.ID, len(province.CornerIDs))
		}

		ring := make([]Point, len(province.CornerIDs))
		for ringIndex, cornerID := range province.CornerIDs {
			if int(cornerID) < 0 || int(cornerID) >= len(world.Corners) {
				t.Fatalf("province %d has invalid corner ID %d", province.ID, cornerID)
			}
			ring[ringIndex] = world.Corners[cornerID].Point
			cornerInProvince[cornerID] = true
			next := province.CornerIDs[(ringIndex+1)%len(province.CornerIDs)]
			key := orderedCornerIDs(cornerID, next)
			edgeID, ok := edgesByCorners[key]
			if !ok {
				t.Errorf("province %d segment %v has no edge", province.ID, key)
				continue
			}
			if !containsProvinceID(world.Edges[edgeID].ProvinceIDs, province.ID) {
				t.Errorf("province %d segment edge %d lacks incidence", province.ID, edgeID)
			}
			edgeTraversals[edgeID] = append(edgeTraversals[edgeID], [2]CornerID{cornerID, next})
		}
		area := signedArea(ring)
		if area <= 0 {
			t.Errorf("province %d signed area = %g, want positive", province.ID, area)
		}
		totalArea += area
		if province.Terrain != TerrainWater {
			landArea += area
		}
		for ringIndex, start := range ring {
			if cross(start, ring[(ringIndex+1)%len(ring)], province.Center) < -1e-8 {
				t.Errorf("province %d does not contain its center %+v", province.ID, province.Center)
				break
			}
		}
	}
	worldArea := (maximum.X - minimum.X) * (maximum.Y - minimum.Y)
	if math.Abs(totalArea-worldArea) > 1e-8*worldArea {
		t.Errorf("province area sum = %.17g, world bounds area = %.17g", totalArea, worldArea)
	}
	for cornerID, used := range cornerInProvince {
		if !used {
			t.Errorf("corner %d is orphaned", cornerID)
		}
	}
	for edgeID, traversals := range edgeTraversals {
		incidenceCount := len(world.Edges[edgeID].ProvinceIDs)
		if len(traversals) != incidenceCount {
			t.Errorf("edge %d has %d polygon traversals for %d incident provinces", edgeID, len(traversals), incidenceCount)
			continue
		}
		if len(traversals) == 2 && (traversals[0][0] != traversals[1][1] || traversals[0][1] != traversals[1][0]) {
			t.Errorf("interior edge %d is not traversed in opposite directions: %v", edgeID, traversals)
		}
	}

	if math.Abs(landArea-float64(config.ProvinceCount)) > 1e-7*float64(config.ProvinceCount) {
		t.Errorf("land polygon area = %.17g, want %d", landArea, config.ProvinceCount)
	}
	for _, island := range world.Islands {
		assertIslandConnected(t, island, neighbors)
	}
}

func assertIslandConnected(t *testing.T, island Island, neighbors [][]ProvinceID) {
	t.Helper()
	seen := map[ProvinceID]bool{island.ProvinceIDs[0]: true}
	queue := []ProvinceID{island.ProvinceIDs[0]}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbor := range neighbors[current] {
			if !seen[neighbor] {
				seen[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}
	if len(seen) != len(island.ProvinceIDs) {
		t.Errorf("island %d adjacency component contains %d provinces, want %d", island.ID, len(seen), len(island.ProvinceIDs))
	}
}

func orderedCornerIDs(first, second CornerID) [2]CornerID {
	if first > second {
		first, second = second, first
	}
	return [2]CornerID{first, second}
}

func containsProvinceID(values []ProvinceID, target ProvinceID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
