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

func TestGenerateUsesPlacedCandidateSitesInOneWorldMesh(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 20, IslandCount: 4}
	allocations, err := allocateProvinces(config.ProvinceCount, config.IslandCount)
	if err != nil {
		t.Fatalf("allocateProvinces() error = %v", err)
	}
	plans, meshes := retainedPlacementFixture(t, config, allocations)
	layout, err := placeIslands(config, plans, meshes)
	if err != nil {
		t.Fatalf("placeIslands() error = %v", err)
	}
	world, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	provinceOffset := 0
	for islandIndex, placed := range layout.islands {
		selected := make([]bool, len(placed.candidateMesh.cells))
		for _, candidateID := range placed.landCandidateIDs {
			selected[candidateID] = true
		}
		for candidateID, cell := range placed.candidateMesh.cells {
			province := world.Provinces[provinceOffset+candidateID]
			if !pointsNear(province.Center, cell.center) {
				t.Errorf("island %d candidate %d center = %+v, want %+v", islandIndex, candidateID, province.Center, cell.center)
			}
			if province.IslandID != placed.id {
				t.Errorf("island %d candidate %d owner = %d", islandIndex, candidateID, province.IslandID)
			}
			if gotLand := province.Terrain != TerrainWater; gotLand != selected[candidateID] {
				t.Errorf("island %d candidate %d land = %t, want %t", islandIndex, candidateID, gotLand, selected[candidateID])
			}
		}
		provinceOffset += len(placed.candidateMesh.cells)
	}

	sharedOcean := false
	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) != 2 {
			continue
		}
		first, second := world.Provinces[edge.ProvinceIDs[0]], world.Provinces[edge.ProvinceIDs[1]]
		if first.IslandID != second.IslandID && first.Terrain == TerrainWater && second.Terrain == TerrainWater {
			sharedOcean = true
			break
		}
	}
	if !sharedOcean {
		t.Fatal("world mesh has no water edge joining sites from different island candidate maps")
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
	if got := len(world.Islands); got != config.IslandCount {
		t.Fatalf("island count = %d, want %d", got, config.IslandCount)
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
	waterNeighbors := make([][]ProvinceID, len(world.Provinces))
	edgeTraversals := make([][][2]CornerID, len(world.Edges))
	cornerInProvince := make([]bool, len(world.Corners))
	crossIslandWaterEdge := false
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
		if len(edge.ProvinceIDs) == 2 {
			first, second := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
			if world.Provinces[first].IslandID != world.Provinces[second].IslandID {
				if world.Provinces[first].Terrain != TerrainWater || world.Provinces[second].Terrain != TerrainWater {
					t.Errorf("interior edge %d joins land across island site groups %d and %d", edge.ID, world.Provinces[first].IslandID, world.Provinces[second].IslandID)
				}
				crossIslandWaterEdge = true
			}
			if world.Provinces[first].Terrain != TerrainWater && world.Provinces[second].Terrain != TerrainWater {
				neighbors[first] = append(neighbors[first], second)
				neighbors[second] = append(neighbors[second], first)
			}
			if world.Provinces[first].Terrain == TerrainWater && world.Provinces[second].Terrain == TerrainWater {
				waterNeighbors[first] = append(waterNeighbors[first], second)
				waterNeighbors[second] = append(waterNeighbors[second], first)
			}
		}
	}
	if config.IslandCount > 1 && !crossIslandWaterEdge {
		t.Error("world mesh has no cross-island water adjacency")
	}

	islandAreas := make([]float64, len(world.Islands))
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
		if int(province.IslandID) < 0 || int(province.IslandID) >= len(world.Islands) {
			t.Fatalf("province %d has invalid island ID %d", province.ID, province.IslandID)
		}
		switch province.Terrain {
		case TerrainWater, TerrainPlains, TerrainHills, TerrainMountains:
		default:
			t.Errorf("province %d has unsupported terrain %q", province.ID, province.Terrain)
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
			islandAreas[province.IslandID] += area
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

	for islandID, island := range world.Islands {
		if math.Abs(islandAreas[islandID]-float64(len(island.ProvinceIDs))) > 1e-7*float64(len(island.ProvinceIDs)) {
			t.Errorf("island %d polygon area = %.17g, want %d", islandID, islandAreas[islandID], len(island.ProvinceIDs))
		}
		assertIslandConnected(t, island, neighbors)
	}
	assertWaterConnected(t, world, waterNeighbors)
	assertIslandsSeparated(t, world)
}

func assertWaterConnected(t *testing.T, world World, neighbors [][]ProvinceID) {
	t.Helper()
	first := ProvinceID(-1)
	want := 0
	for provinceID, province := range world.Provinces {
		if province.Terrain != TerrainWater {
			continue
		}
		want++
		if first == -1 {
			first = ProvinceID(provinceID)
		}
	}
	seen := map[ProvinceID]bool{first: true}
	queue := []ProvinceID{first}
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
	if len(seen) != want {
		t.Errorf("water adjacency component contains %d provinces, want %d", len(seen), want)
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

func assertIslandsSeparated(t *testing.T, world World) {
	t.Helper()
	bounds := make([]rectangle, len(world.Islands))
	for islandIndex, island := range world.Islands {
		first := world.Provinces[island.ProvinceIDs[0]].CornerIDs[0]
		bounds[islandIndex] = rectangle{min: world.Corners[first].Point, max: world.Corners[first].Point}
		for _, provinceID := range island.ProvinceIDs {
			for _, cornerID := range world.Provinces[provinceID].CornerIDs {
				point := world.Corners[cornerID].Point
				bounds[islandIndex].min.X = math.Min(bounds[islandIndex].min.X, point.X)
				bounds[islandIndex].min.Y = math.Min(bounds[islandIndex].min.Y, point.Y)
				bounds[islandIndex].max.X = math.Max(bounds[islandIndex].max.X, point.X)
				bounds[islandIndex].max.Y = math.Max(bounds[islandIndex].max.Y, point.Y)
			}
		}
	}
	for first := range bounds {
		for second := first + 1; second < len(bounds); second++ {
			if gap := rectangleGap(bounds[first], bounds[second]); gap < islandWaterGap-1e-8 {
				t.Errorf("islands %d and %d polygon gap = %g, want at least %g", first, second, gap, islandWaterGap)
			}
		}
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
