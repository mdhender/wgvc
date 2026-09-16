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

func TestGeneratePreservesPlannedGeometry(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 20, IslandCount: 4}
	allocations, err := allocateProvinces(config.ProvinceCount, config.IslandCount)
	if err != nil {
		t.Fatalf("allocateProvinces() error = %v", err)
	}
	layout, err := planIslands(config, allocations)
	if err != nil {
		t.Fatalf("planIslands() error = %v", err)
	}
	meshes, err := tessellateIslands(layout.islands)
	if err != nil {
		t.Fatalf("tessellateIslands() error = %v", err)
	}
	world, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	provinceOffset, cornerOffset, edgeOffset := 0, 0, 0
	for islandIndex, plan := range layout.islands {
		mesh := meshes.islands[islandIndex]
		for localCornerID, point := range mesh.corners {
			if got, want := world.Corners[cornerOffset+localCornerID].Point, plan.footprint.pointAt(point); got != want {
				t.Errorf("island %d corner %d = %+v, want %+v", islandIndex, localCornerID, got, want)
			}
		}
		for localProvinceID, cell := range mesh.cells {
			province := world.Provinces[provinceOffset+localProvinceID]
			if got, want := province.Center, plan.footprint.pointAt(cell.center); got != want {
				t.Errorf("island %d province %d center = %+v, want %+v", islandIndex, localProvinceID, got, want)
			}
			for ringIndex, localCornerID := range cell.cornerIDs {
				if got, want := province.CornerIDs[ringIndex], CornerID(cornerOffset+localCornerID); got != want {
					t.Errorf("island %d province %d corner %d = %d, want %d", islandIndex, localProvinceID, ringIndex, got, want)
				}
			}
		}
		for localEdgeID, meshEdge := range mesh.edges {
			edge := world.Edges[edgeOffset+localEdgeID]
			wantCorners := [2]CornerID{CornerID(cornerOffset + meshEdge.cornerIDs[0]), CornerID(cornerOffset + meshEdge.cornerIDs[1])}
			if edge.CornerIDs != wantCorners {
				t.Errorf("island %d edge %d corners = %v, want %v", islandIndex, localEdgeID, edge.CornerIDs, wantCorners)
			}
			for incidenceIndex, localProvinceID := range meshEdge.siteIndexes {
				if got, want := edge.ProvinceIDs[incidenceIndex], ProvinceID(provinceOffset+localProvinceID); got != want {
					t.Errorf("island %d edge %d province %d = %d, want %d", islandIndex, localEdgeID, incidenceIndex, got, want)
				}
			}
		}
		provinceOffset += len(mesh.cells)
		cornerOffset += len(mesh.corners)
		edgeOffset += len(mesh.edges)
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

func assertValidWorld(t *testing.T, world World, config Config) {
	t.Helper()
	if got := len(world.Islands); got != config.IslandCount {
		t.Fatalf("island count = %d, want %d", got, config.IslandCount)
	}
	if got := len(world.Provinces); got != config.ProvinceCount {
		t.Fatalf("province count = %d, want %d", got, config.ProvinceCount)
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
		if count != 1 {
			t.Errorf("province %d appears in %d island memberships", provinceID, count)
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
				t.Errorf("interior edge %d joins islands %d and %d", edge.ID, world.Provinces[first].IslandID, world.Provinces[second].IslandID)
			}
			neighbors[first] = append(neighbors[first], second)
			neighbors[second] = append(neighbors[second], first)
		}
	}

	islandAreas := make([]float64, len(world.Islands))
	for provinceIndex, province := range world.Provinces {
		if province.ID != ProvinceID(provinceIndex) {
			t.Errorf("province at index %d has ID %d", provinceIndex, province.ID)
		}
		if int(province.IslandID) < 0 || int(province.IslandID) >= len(world.Islands) {
			t.Fatalf("province %d has invalid island ID %d", province.ID, province.IslandID)
		}
		if province.Terrain != TerrainPlains {
			t.Errorf("province %d terrain = %q, want %q", province.ID, province.Terrain, TerrainPlains)
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
		}
		area := signedArea(ring)
		if area <= 0 {
			t.Errorf("province %d signed area = %g, want positive", province.ID, area)
		}
		islandAreas[province.IslandID] += area
		for ringIndex, start := range ring {
			if cross(start, ring[(ringIndex+1)%len(ring)], province.Center) < -1e-8 {
				t.Errorf("province %d does not contain its center %+v", province.ID, province.Center)
				break
			}
		}
	}

	for islandID, island := range world.Islands {
		if math.Abs(islandAreas[islandID]-float64(len(island.ProvinceIDs))) > 1e-7*float64(len(island.ProvinceIDs)) {
			t.Errorf("island %d polygon area = %.17g, want %d", islandID, islandAreas[islandID], len(island.ProvinceIDs))
		}
		assertIslandConnected(t, island, neighbors)
	}
	assertIslandsSeparated(t, world)
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
