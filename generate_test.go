package wgvc

import (
	"fmt"
	"math"
	"reflect"
	"slices"
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
		if province.IslandID == NoIslandID {
			water++
			continue
		}
		land++
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
		if province.IslandID == NoIslandID {
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
		if world.Provinces[provinceID].IslandID == NoIslandID {
			want = 0
		}
		if count != want {
			t.Errorf("province %d appears in %d island memberships, want %d", provinceID, count, want)
		}
	}

	cornerLand := make([]bool, len(world.Corners))
	cornerWater := make([]bool, len(world.Corners))
	for _, province := range world.Provinces {
		for _, cornerID := range province.CornerIDs {
			if province.IslandID == NoIslandID {
				cornerWater[cornerID] = true
			} else {
				cornerLand[cornerID] = true
			}
		}
	}
	for cornerID, corner := range world.Corners {
		if corner.ID != CornerID(cornerID) {
			t.Errorf("corner at index %d has ID %d", cornerID, corner.ID)
		}
		if !finitePoint(corner.Point) {
			t.Errorf("corner %d is not finite: %+v", corner.ID, corner.Point)
		}
		switch {
		case cornerLand[cornerID] && cornerWater[cornerID]:
			if corner.Elevation != 0 {
				t.Errorf("coast corner %d elevation = %g, want 0", corner.ID, corner.Elevation)
			}
		case cornerLand[cornerID]:
			if corner.Elevation < elevationLandMargin || corner.Elevation >= 1 {
				t.Errorf("land corner %d elevation = %g, want [%g, 1)", corner.ID, corner.Elevation, elevationLandMargin)
			}
		default:
			if corner.Elevation <= -1 || corner.Elevation > -elevationWaterMargin {
				t.Errorf("water corner %d elevation = %g, want (-1, %g]", corner.ID, corner.Elevation, -elevationWaterMargin)
			}
		}
	}

	edgesByCorners := make(map[[2]CornerID]EdgeID, len(world.Edges))
	neighbors := make([][]ProvinceID, len(world.Provinces))
	edgeTraversals := make([][][2]CornerID, len(world.Edges))
	cornerInProvince := make([]bool, len(world.Corners))
	elevationTotals := make([]float64, len(world.Provinces))
	elevationEdgeCounts := make([]int, len(world.Provinces))
	noise := newElevationNoise(config.WorldSeed)
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
		if len(edge.ProvinceIDs) == 1 && world.Provinces[edge.ProvinceIDs[0]].IslandID != NoIslandID {
			t.Errorf("world-boundary edge %d is incident to land province %d", edge.ID, edge.ProvinceIDs[0])
		}
		if len(edge.ProvinceIDs) == 2 {
			first, second := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
			firstWater := world.Provinces[first].IslandID == NoIslandID
			secondWater := world.Provinces[second].IslandID == NoIslandID
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
		wantLength := pointDistance(world.Corners[edge.CornerIDs[0]].Point, world.Corners[edge.CornerIDs[1]].Point)
		if edge.Length != wantLength || !(edge.Length > 0) || math.IsInf(edge.Length, 0) {
			t.Errorf("edge %d length = %g, want positive corner distance %g", edge.ID, edge.Length, wantLength)
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
		if province.IslandID != NoIslandID && (int(province.IslandID) < 0 || int(province.IslandID) >= len(world.Islands)) {
			t.Fatalf("land province %d has invalid island ID %d", province.ID, province.IslandID)
		}
		if !province.Terrain.Valid() {
			t.Errorf("province %d has unsupported terrain %q", province.ID, province.Terrain)
		}
		if province.Terrain.IsWater() != (province.IslandID == NoIslandID) {
			t.Errorf("province %d terrain %q disagrees with island ID %d", province.ID, province.Terrain, province.IslandID)
		}
		if province.Terrain == TerrainVolcano || province.Terrain == TerrainVolcanicHighland {
			t.Errorf("province %d received reserved terrain %q", province.ID, province.Terrain)
		}
		inlandWater := province.Terrain == TerrainInlandSea || province.Terrain == TerrainLake
		if province.BasinID == NoBasinID {
			if inlandWater {
				t.Errorf("province %d outside any basin has inland-water terrain %q", province.ID, province.Terrain)
			}
		} else {
			if province.IslandID != NoIslandID {
				t.Errorf("land province %d has basin %d", province.ID, province.BasinID)
			}
			if int(province.BasinID) < 0 || int(province.BasinID) >= len(world.Basins) || !slices.Contains(world.Basins[province.BasinID].ProvinceIDs, province.ID) {
				t.Errorf("province %d basin %d does not list it as a member", province.ID, province.BasinID)
			} else if want := basinTerrain(world.Basins[province.BasinID]); province.Terrain != want {
				t.Errorf("basin province %d terrain = %q, want %q", province.ID, province.Terrain, want)
			}
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
		if math.IsNaN(province.Relief) || province.Relief < 0 || province.Relief >= 1 {
			t.Errorf("province %d relief = %g, want finite value in [0, 1)", province.ID, province.Relief)
		}
		if !finitePoint(province.Center) {
			t.Errorf("province %d center is not finite: %+v", province.ID, province.Center)
		}
		if len(province.CornerIDs) < 3 {
			t.Fatalf("province %d has only %d corners", province.ID, len(province.CornerIDs))
		}

		if len(province.EdgeIDs) != len(province.CornerIDs) {
			t.Fatalf("province %d has %d edge IDs for %d corners", province.ID, len(province.EdgeIDs), len(province.CornerIDs))
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
			if province.EdgeIDs[ringIndex] != edgeID {
				t.Errorf("province %d edge IDs[%d] = %d, want segment edge %d", province.ID, ringIndex, province.EdgeIDs[ringIndex], edgeID)
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
		if province.Area != area {
			t.Errorf("province %d area = %g, want polygon area %g", province.ID, province.Area, area)
		}
		assertValidExits(t, world, province)
		totalArea += area
		if province.IslandID != NoIslandID {
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
	for _, edge := range world.Edges {
		for _, provinceID := range edge.ProvinceIDs {
			if !slices.Contains(world.Provinces[provinceID].EdgeIDs, edge.ID) {
				t.Errorf("edge %d is incident to province %d, which does not list it in %v", edge.ID, provinceID, world.Provinces[provinceID].EdgeIDs)
			}
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
	assertValidRivers(t, world)
	assertValidSeas(t, world)
	assertValidHarbors(t, world)
}

// assertValidHarbors recomputes each land province's water edge counts,
// shelter, and river lists from edges, rivers, and corners.
func assertValidHarbors(t *testing.T, world World) {
	t.Helper()
	neighbors := provinceNeighbors(&world)
	isLand := func(id ProvinceID) bool { return world.Provinces[id].IslandID != NoIslandID }
	for _, province := range world.Provinces {
		if !isLand(province.ID) {
			if province.OceanEdges != 0 || province.BasinEdges != 0 || province.Shelter != 0 || province.RiverIDs != nil || province.RiverMouthIDs != nil || province.ConfluenceIDs != nil {
				t.Errorf("water province %d has harbor fields set", province.ID)
			}
			continue
		}
		ocean, basin, shelter := 0, 0, 0.0
		var rivers []RiverID
		for _, edgeID := range province.EdgeIDs {
			edge := world.Edges[edgeID]
			if edge.RiverID != NoRiverID && !slices.Contains(rivers, edge.RiverID) {
				rivers = append(rivers, edge.RiverID)
			}
			for _, otherID := range edge.ProvinceIDs {
				if otherID == province.ID || isLand(otherID) {
					continue
				}
				land := 0
				for _, n := range neighbors[otherID] {
					if isLand(n) {
						land++
					}
				}
				shelter += float64(land) / float64(len(neighbors[otherID]))
				if world.Provinces[otherID].BasinID == NoBasinID {
					ocean++
				} else {
					basin++
				}
			}
		}
		if ocean+basin > 0 {
			shelter /= float64(ocean + basin)
		}
		slices.Sort(rivers)
		if province.OceanEdges != ocean || province.BasinEdges != basin || math.Abs(province.Shelter-shelter) > 1e-12 || province.Shelter < 0 || province.Shelter > 1 {
			t.Errorf("province %d harbor = %d ocean, %d basin, shelter %g; want %d, %d, %g", province.ID, province.OceanEdges, province.BasinEdges, province.Shelter, ocean, basin, shelter)
		}
		if (province.CoastDistance == 0) != (ocean+basin > 0) {
			t.Errorf("province %d coast distance %d disagrees with %d water edges", province.ID, province.CoastDistance, ocean+basin)
		}
		if !slices.Equal(province.RiverIDs, rivers) && !(len(rivers) == 0 && len(province.RiverIDs) == 0) {
			t.Errorf("province %d rivers = %v, want %v", province.ID, province.RiverIDs, rivers)
		}
		for _, list := range [][]RiverID{province.RiverMouthIDs, province.ConfluenceIDs} {
			if !slices.IsSorted(list) {
				t.Errorf("province %d river end list %v is not sorted", province.ID, list)
			}
			for _, riverID := range list {
				river := world.Rivers[riverID]
				mouth := river.CornerIDs[len(river.CornerIDs)-1]
				last := world.Edges[river.EdgeIDs[len(river.EdgeIDs)-1]]
				if !slices.Contains(province.CornerIDs, mouth) || !slices.Contains(last.ProvinceIDs, province.ID) {
					t.Errorf("province %d lists river %d ending elsewhere", province.ID, riverID)
				}
				if slices.Contains(province.ConfluenceIDs, riverID) != (river.Mouth.Kind == RiverEndRiver) {
					t.Errorf("province %d files river %d (mouth %s) in the wrong list", province.ID, riverID, river.Mouth.Kind)
				}
			}
		}
	}
	mouths, confluences := 0, 0
	for _, province := range world.Provinces {
		mouths += len(province.RiverMouthIDs)
		confluences += len(province.ConfluenceIDs)
	}
	riverMouths, riverConfluences := 0, 0
	for _, river := range world.Rivers {
		if river.Mouth.Kind == RiverEndRiver {
			riverConfluences++
		} else {
			riverMouths++
		}
	}
	// Each river ends between its last edge's two land provinces.
	if mouths != 2*riverMouths || confluences != 2*riverConfluences {
		t.Errorf("provinces list %d mouths and %d confluences for %d and %d rivers", mouths, confluences, riverMouths, riverConfluences)
	}
}

// assertValidSeas checks coast distances against a fresh BFS, and the sea
// zone, strait, and neck contracts: zones cover exactly the ocean, are
// connected, capped, and canonically ordered; straits join two distinct
// islands through water with the named shores adjacent; necks are land whose
// removal separates their two ends into regions of the stated sizes.
func assertValidSeas(t *testing.T, world World) {
	t.Helper()
	neighbors := provinceNeighbors(&world)
	isLand := func(id ProvinceID) bool { return world.Provinces[id].IslandID != NoIslandID }
	isOcean := func(id ProvinceID) bool { return !isLand(id) && world.Provinces[id].BasinID == NoBasinID }

	want := make([]int, len(world.Provinces))
	var queue []ProvinceID
	for id := range world.Provinces {
		want[id] = -1
		for _, n := range neighbors[id] {
			if isLand(n) != isLand(ProvinceID(id)) {
				want[id] = 0
				queue = append(queue, ProvinceID(id))
				break
			}
		}
	}
	for next := 0; next < len(queue); next++ {
		id := queue[next]
		for _, n := range neighbors[id] {
			if want[n] < 0 && isLand(n) == isLand(id) {
				want[n] = want[id] + 1
				queue = append(queue, n)
			}
		}
	}
	for id, province := range world.Provinces {
		if province.CoastDistance != want[id] || province.CoastDistance < 0 {
			t.Errorf("province %d coast distance = %d, want %d", id, province.CoastDistance, want[id])
		}
		if isOcean(ProvinceID(id)) != (province.SeaZoneID != NoSeaZoneID) {
			t.Errorf("province %d ocean=%t has sea zone %d", id, isOcean(ProvinceID(id)), province.SeaZoneID)
		}
	}

	for index, zone := range world.SeaZones {
		if zone.ID != SeaZoneID(index) || len(zone.ProvinceIDs) == 0 || len(zone.ProvinceIDs) > 2*seaZoneTargetSize {
			t.Errorf("sea zone at index %d has ID %d and %d provinces, want at most %d", index, zone.ID, len(zone.ProvinceIDs), 2*seaZoneTargetSize)
			continue
		}
		if index > 0 && world.SeaZones[index-1].ProvinceIDs[0] >= zone.ProvinceIDs[0] {
			t.Errorf("sea zone %d is not ordered by lowest member after zone %d", zone.ID, index-1)
		}
		if !slices.IsSorted(zone.ProvinceIDs) || !slices.Contains(zone.ProvinceIDs, zone.CenterProvinceID) {
			t.Errorf("sea zone %d members %v are unsorted or omit center %d", zone.ID, zone.ProvinceIDs, zone.CenterProvinceID)
		}
		for _, id := range zone.ProvinceIDs {
			if world.Provinces[id].SeaZoneID != zone.ID {
				t.Errorf("sea zone %d member %d has zone %d", zone.ID, id, world.Provinces[id].SeaZoneID)
			}
		}
		reached := boundedBFS(zone.ProvinceIDs[0], neighbors, len(world.Provinces), func(id ProvinceID) bool { return world.Provinces[id].SeaZoneID == zone.ID })
		if !slices.Equal(reached, zone.ProvinceIDs) {
			t.Errorf("sea zone %d is not connected: reached %d of %d members", zone.ID, len(reached), len(zone.ProvinceIDs))
		}
	}

	for index, strait := range world.Straits {
		if strait.ID != StraitID(index) || strait.IslandIDs[0] >= strait.IslandIDs[1] || strait.Width < 1 || strait.Width > maxStraitWidth || len(strait.ProvinceIDs) == 0 || !slices.IsSorted(strait.ProvinceIDs) {
			t.Errorf("strait at index %d is malformed: %+v", index, strait)
			continue
		}
		for _, id := range strait.ProvinceIDs {
			if isLand(id) {
				t.Errorf("strait %d contains land province %d", strait.ID, id)
			}
		}
		reached := boundedBFS(strait.ProvinceIDs[0], neighbors, len(world.Provinces), func(id ProvinceID) bool { return slices.Contains(strait.ProvinceIDs, id) })
		if !slices.Equal(reached, strait.ProvinceIDs) {
			t.Errorf("strait %d is not connected by water", strait.ID)
		}
		for side, shore := range strait.Shores {
			if len(shore) == 0 || !slices.IsSorted(shore) {
				t.Errorf("strait %d shore %d = %v, want a sorted non-empty list", strait.ID, side, shore)
			}
			for _, id := range shore {
				adjacent := false
				for _, n := range neighbors[id] {
					adjacent = adjacent || slices.Contains(strait.ProvinceIDs, n)
				}
				if world.Provinces[id].IslandID != strait.IslandIDs[side] || !adjacent {
					t.Errorf("strait %d shore province %d is not island %d land adjacent to the strait", strait.ID, id, strait.IslandIDs[side])
				}
			}
		}
	}

	for index, neck := range world.Necks {
		if neck.ID != NeckID(index) || neck.Width < 1 || neck.Width > maxNeckWidth || len(neck.ProvinceIDs) == 0 || !slices.IsSorted(neck.ProvinceIDs) {
			t.Errorf("neck at index %d is malformed: %+v", index, neck)
			continue
		}
		if index > 0 && (world.Necks[index-1].IslandID > neck.IslandID || world.Necks[index-1].IslandID == neck.IslandID && world.Necks[index-1].ProvinceIDs[0] >= neck.ProvinceIDs[0]) {
			t.Errorf("neck %d is out of order after neck %d", neck.ID, index-1)
		}
		for _, id := range neck.ProvinceIDs {
			if world.Provinces[id].IslandID != neck.IslandID {
				t.Errorf("neck %d province %d is not on island %d", neck.ID, id, neck.IslandID)
			}
		}
		open := func(id ProvinceID) bool {
			return world.Provinces[id].IslandID == neck.IslandID && !slices.Contains(neck.ProvinceIDs, id)
		}
		if neck.EndSizes[0] < neck.EndSizes[1] || neck.EndSizes[1] < 1 {
			t.Errorf("neck %d end sizes %v are not descending and positive", neck.ID, neck.EndSizes)
		}
		var regions [2][]ProvinceID
		for side, end := range neck.Ends {
			if len(end) == 0 || !slices.IsSorted(end) {
				t.Errorf("neck %d end %d = %v, want a sorted non-empty list", neck.ID, side, end)
				continue
			}
			regions[side] = boundedBFS(end[0], neighbors, len(world.Provinces), open)
			if len(regions[side]) != neck.EndSizes[side] {
				t.Errorf("neck %d end %d region has %d provinces, want %d", neck.ID, side, len(regions[side]), neck.EndSizes[side])
			}
			for _, id := range end {
				adjacent := false
				for _, n := range neighbors[id] {
					adjacent = adjacent || slices.Contains(neck.ProvinceIDs, n)
				}
				if _, inRegion := slices.BinarySearch(regions[side], id); !open(id) || !adjacent || !inRegion {
					t.Errorf("neck %d end province %d is not open land adjacent to the neck in its region", neck.ID, id)
				}
			}
		}
		if len(regions[0]) > 0 && len(regions[1]) > 0 {
			if _, joined := slices.BinarySearch(regions[0], regions[1][0]); joined {
				t.Errorf("neck %d does not separate its ends", neck.ID)
			}
		}
	}
}

// assertValidRivers checks the river contracts: chains are contiguous edges
// between land provinces in flow order with non-decreasing discharge, each
// river edge belongs to exactly the river that lists it, mouths and sources
// name the right kind of end, and rivers are ordered by discharge.
func assertValidRivers(t *testing.T, world World) {
	t.Helper()
	edgeRivers := make([]int, len(world.Edges))
	for riverIndex, river := range world.Rivers {
		if river.ID != RiverID(riverIndex) {
			t.Errorf("river at index %d has ID %d", riverIndex, river.ID)
		}
		if len(river.EdgeIDs) == 0 || len(river.CornerIDs) != len(river.EdgeIDs)+1 {
			t.Errorf("river %d has %d corners for %d edges", river.ID, len(river.CornerIDs), len(river.EdgeIDs))
			continue
		}
		if river.Class != ClassifyDischarge(river.Discharge) || river.Class == "" {
			t.Errorf("river %d class %q does not match discharge %g", river.ID, river.Class, river.Discharge)
		}
		if riverIndex > 0 && river.Discharge > world.Rivers[riverIndex-1].Discharge {
			t.Errorf("river %d discharge %g exceeds river %d's %g", river.ID, river.Discharge, riverIndex-1, world.Rivers[riverIndex-1].Discharge)
		}
		previousDischarge := 0.0
		for position, edgeID := range river.EdgeIDs {
			if int(edgeID) < 0 || int(edgeID) >= len(world.Edges) {
				t.Fatalf("river %d references unknown edge %d", river.ID, edgeID)
			}
			edge := world.Edges[edgeID]
			edgeRivers[edgeID]++
			if edge.RiverID != river.ID {
				t.Errorf("river %d edge %d has RiverID %d", river.ID, edgeID, edge.RiverID)
			}
			first, second := river.CornerIDs[position], river.CornerIDs[position+1]
			if edge.CornerIDs != orderedCornerIDs(first, second) {
				t.Errorf("river %d edge %d joins %v, want %d and %d", river.ID, edgeID, edge.CornerIDs, first, second)
			}
			if len(edge.ProvinceIDs) != 2 || world.Provinces[edge.ProvinceIDs[0]].IslandID == NoIslandID || world.Provinces[edge.ProvinceIDs[1]].IslandID == NoIslandID {
				t.Errorf("river %d edge %d is not between two land provinces: %v", river.ID, edgeID, edge.ProvinceIDs)
			}
			if edge.Discharge < riverStreamDischarge || edge.Discharge < previousDischarge {
				t.Errorf("river %d edge %d discharge %g is below the stream threshold or below upstream %g", river.ID, edgeID, edge.Discharge, previousDischarge)
			}
			previousDischarge = edge.Discharge
		}
		if last := world.Edges[river.EdgeIDs[len(river.EdgeIDs)-1]]; last.Discharge != river.Discharge {
			t.Errorf("river %d discharge %g differs from its last edge's %g", river.ID, river.Discharge, last.Discharge)
		}
		mouthCorner := river.CornerIDs[len(river.CornerIDs)-1]
		switch river.Mouth.Kind {
		case RiverEndOcean:
			if !cornerTouchesWater(world, mouthCorner, NoBasinID) || river.Mouth.BasinID != NoBasinID || river.Mouth.RiverID != NoRiverID {
				t.Errorf("river %d ocean mouth %+v at corner %d is not on ocean", river.ID, river.Mouth, mouthCorner)
			}
		case RiverEndBasin:
			if river.Mouth.BasinID == NoBasinID || !cornerTouchesWater(world, mouthCorner, river.Mouth.BasinID) || river.Mouth.RiverID != NoRiverID {
				t.Errorf("river %d basin mouth %+v at corner %d is not on that basin", river.ID, river.Mouth, mouthCorner)
			}
		case RiverEndRiver:
			if river.Mouth.RiverID == NoRiverID || river.Mouth.RiverID == river.ID || int(river.Mouth.RiverID) >= len(world.Rivers) || river.Mouth.BasinID != NoBasinID {
				t.Errorf("river %d confluence mouth %+v is invalid", river.ID, river.Mouth)
			} else if trunk := world.Rivers[river.Mouth.RiverID]; !slices.Contains(trunk.CornerIDs[:len(trunk.CornerIDs)-1], mouthCorner) || trunk.Discharge < river.Discharge {
				t.Errorf("river %d joins river %d away from its course or into a smaller river", river.ID, river.Mouth.RiverID)
			}
		default:
			t.Errorf("river %d has mouth kind %q", river.ID, river.Mouth.Kind)
		}
		sourceCorner := river.CornerIDs[0]
		switch river.Source.Kind {
		case RiverEndSpring:
			if river.Source.BasinID != NoBasinID || river.Source.RiverID != NoRiverID {
				t.Errorf("river %d spring source %+v carries an ID", river.ID, river.Source)
			}
		case RiverEndBasin:
			if river.Source.BasinID == NoBasinID || !cornerTouchesWater(world, sourceCorner, river.Source.BasinID) {
				t.Errorf("river %d basin source %+v at corner %d is not on that basin", river.ID, river.Source, sourceCorner)
			}
		default:
			t.Errorf("river %d has source kind %q", river.ID, river.Source.Kind)
		}
	}
	for edgeID, edge := range world.Edges {
		if (edge.RiverID != NoRiverID) != (edgeRivers[edgeID] == 1) || edgeRivers[edgeID] > 1 {
			t.Errorf("edge %d has RiverID %d but appears in %d rivers", edgeID, edge.RiverID, edgeRivers[edgeID])
		}
		if math.IsNaN(edge.Discharge) || edge.Discharge < 0 {
			t.Errorf("edge %d discharge = %g, want non-negative", edgeID, edge.Discharge)
		}
	}
}

// cornerTouchesWater reports whether a corner belongs to a water province in
// the given basin, or to ocean when basinID is NoBasinID.
func cornerTouchesWater(world World, cornerID CornerID, basinID BasinID) bool {
	for _, province := range world.Provinces {
		if province.IslandID != NoIslandID || province.BasinID != basinID {
			continue
		}
		if slices.Contains(province.CornerIDs, cornerID) {
			return true
		}
	}
	return false
}

// assertValidExits checks that exits are the province's edges numbered
// densely from 1, clockwise from north (reverse ring order), with strictly
// increasing outward bearings that start at the smallest one, correct
// neighbors, and compass labels that match their bearings.
func assertValidExits(t *testing.T, world World, province Province) {
	t.Helper()
	n := len(province.EdgeIDs)
	if len(province.Exits) != n {
		t.Errorf("province %d has %d exits for %d edges", province.ID, len(province.Exits), n)
		return
	}
	start := -1
	for ringIndex, edgeID := range province.EdgeIDs {
		if edgeID == province.Exits[0].EdgeID {
			start = ringIndex
		}
	}
	if start < 0 {
		t.Errorf("province %d exit 1 edge %d is not a boundary edge", province.ID, province.Exits[0].EdgeID)
		return
	}
	for exitIndex, exit := range province.Exits {
		if exit.Number != exitIndex+1 {
			t.Errorf("province %d exit at index %d has number %d", province.ID, exitIndex, exit.Number)
		}
		ringIndex := ((start-exitIndex)%n + n) % n
		if exit.EdgeID != province.EdgeIDs[ringIndex] {
			t.Errorf("province %d exit %d edge = %d, want reverse-ring edge %d", province.ID, exit.Number, exit.EdgeID, province.EdgeIDs[ringIndex])
		}
		first := world.Corners[province.CornerIDs[ringIndex]].Point
		second := world.Corners[province.CornerIDs[(ringIndex+1)%n]].Point
		if want := outwardBearing(first, second); exit.Bearing != want || exit.Bearing < 0 || exit.Bearing >= 360 {
			t.Errorf("province %d exit %d bearing = %g, want outward normal %g in [0, 360)", province.ID, exit.Number, exit.Bearing, want)
		}
		if exitIndex > 0 && exit.Bearing <= province.Exits[exitIndex-1].Bearing {
			t.Errorf("province %d exit %d bearing %g does not increase from %g", province.ID, exit.Number, exit.Bearing, province.Exits[exitIndex-1].Bearing)
		}
		if exit.Compass != compassFor(exit.Bearing) || !slices.Contains(compassPoints[:], exit.Compass) {
			t.Errorf("province %d exit %d compass = %q for bearing %g", province.ID, exit.Number, exit.Compass, exit.Bearing)
		}
		edge := world.Edges[exit.EdgeID]
		wantNeighbor := NoProvinceID
		for _, incident := range edge.ProvinceIDs {
			if incident != province.ID {
				wantNeighbor = incident
			}
		}
		if exit.NeighborID != wantNeighbor {
			t.Errorf("province %d exit %d neighbor = %d, want %d", province.ID, exit.Number, exit.NeighborID, wantNeighbor)
		}
	}
	for _, exit := range province.Exits[1:] {
		if exit.Bearing < province.Exits[0].Bearing {
			t.Errorf("province %d exit 1 bearing %g is not the smallest; exit %d has %g", province.ID, province.Exits[0].Bearing, exit.Number, exit.Bearing)
		}
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
