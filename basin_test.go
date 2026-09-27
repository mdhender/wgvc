package wgvc

import (
	"math"
	"reflect"
	"slices"
	"testing"
)

// basinTestWorld has an ocean {0, 1} reaching the world boundary, a two-province
// basin {4, 5}, and a one-province basin {7}. Both basins border land province
// 6 but are not connected by water.
func basinTestWorld() World {
	return World{
		Basins: []Basin{{ID: 0, ProvinceIDs: []ProvinceID{2}}}, // stale
		Provinces: []Province{
			{ID: 0, IslandID: NoIslandID, Elevation: -0.7, BasinID: 3},
			{ID: 1, IslandID: NoIslandID, Elevation: -0.1},
			{ID: 2, IslandID: 0, Elevation: 0.3},
			{ID: 3, IslandID: 0, Elevation: 0.5},
			{ID: 4, IslandID: NoIslandID, Elevation: -0.2},
			{ID: 5, IslandID: NoIslandID, Elevation: -0.6},
			{ID: 6, IslandID: 0, Elevation: 0.2},
			{ID: 7, IslandID: NoIslandID, Elevation: 0},
			{ID: 8, IslandID: 0, Elevation: 0.4},
		},
		Edges: []Edge{
			{ID: 0, ProvinceIDs: []ProvinceID{0}}, // world boundary
			{ID: 1, ProvinceIDs: []ProvinceID{0, 1}},
			{ID: 2, ProvinceIDs: []ProvinceID{1, 2}},
			{ID: 3, ProvinceIDs: []ProvinceID{2, 3}},
			{ID: 4, ProvinceIDs: []ProvinceID{2, 4}},
			{ID: 5, ProvinceIDs: []ProvinceID{3, 4}},
			{ID: 6, ProvinceIDs: []ProvinceID{4, 5}},
			{ID: 7, ProvinceIDs: []ProvinceID{5, 6}},
			{ID: 8, ProvinceIDs: []ProvinceID{3, 6}},
			{ID: 9, ProvinceIDs: []ProvinceID{6, 7}},
			{ID: 10, ProvinceIDs: []ProvinceID{7, 8}},
			{ID: 11, ProvinceIDs: []ProvinceID{6, 8}},
		},
	}
}

func TestAssignBasinsFindsEnclosedWaterComponents(t *testing.T) {
	world := basinTestWorld()

	assignBasins(&world)

	want := []Basin{
		{ID: 0, ProvinceIDs: []ProvinceID{4, 5}, SurfaceElevation: 0.2, Depth: 0.6}, // rim 2, 3, 6
		{ID: 1, ProvinceIDs: []ProvinceID{7}, SurfaceElevation: 0.2, Depth: 0},      // rim 6, 8
	}
	if !reflect.DeepEqual(world.Basins, want) {
		t.Fatalf("basins = %+v, want %+v", world.Basins, want)
	}
	wantBasinIDs := []BasinID{NoBasinID, NoBasinID, NoBasinID, NoBasinID, 0, 0, NoBasinID, 1, NoBasinID}
	for provinceID, province := range world.Provinces {
		if province.BasinID != wantBasinIDs[provinceID] {
			t.Errorf("province %d basin = %d, want %d", provinceID, province.BasinID, wantBasinIDs[provinceID])
		}
	}
}

func TestAssignBasinsWithoutEnclosedWater(t *testing.T) {
	world := World{
		Basins: []Basin{{ID: 0}},
		Provinces: []Province{
			{ID: 0, IslandID: NoIslandID, BasinID: 0},
			{ID: 1, IslandID: 0, BasinID: 0},
		},
		Edges: []Edge{
			{ID: 0, ProvinceIDs: []ProvinceID{0}},
			{ID: 1, ProvinceIDs: []ProvinceID{0, 1}},
		},
	}

	assignBasins(&world)

	if len(world.Basins) != 0 {
		t.Fatalf("basins = %+v, want none", world.Basins)
	}
	for provinceID, province := range world.Provinces {
		if province.BasinID != NoBasinID {
			t.Errorf("province %d basin = %d, want NoBasinID", provinceID, province.BasinID)
		}
	}
}

func TestBasinTerrainSizeThreshold(t *testing.T) {
	for _, test := range []struct {
		members int
		want    Terrain
	}{
		{members: 1, want: TerrainLake},
		{members: basinInlandSeaMinProvinces - 1, want: TerrainLake},
		{members: basinInlandSeaMinProvinces, want: TerrainInlandSea},
	} {
		basin := Basin{ProvinceIDs: make([]ProvinceID, test.members)}
		if got := basinTerrain(basin); got != test.want {
			t.Errorf("basinTerrain(%d members) = %q, want %q", test.members, got, test.want)
		}
	}
}

func TestAssignTerrainGivesBasinMembersBasinTerrain(t *testing.T) {
	world := basinTestWorld()
	assignBasins(&world)

	assignTerrain(&world)

	for provinceID, want := range map[int]Terrain{1: TerrainCoastalWater, 4: TerrainLake, 5: TerrainLake, 7: TerrainLake} {
		if got := world.Provinces[provinceID].Terrain; got != want {
			t.Errorf("province %d terrain = %q, want %q", provinceID, got, want)
		}
	}
}

func TestGenerateAssignsBasinsWithoutChangingOtherFields(t *testing.T) {
	config := Config{WorldSeed: 12, ProvinceCount: 1000, IslandCount: 8}
	world, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(world.Basins) == 0 {
		t.Fatal("generated world has no basins")
	}

	neighbors := make([][]ProvinceID, len(world.Provinces))
	var ocean []ProvinceID
	reached := make([]bool, len(world.Provinces))
	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) == 1 {
			if provinceID := edge.ProvinceIDs[0]; world.Provinces[provinceID].IslandID == NoIslandID && !reached[provinceID] {
				reached[provinceID] = true
				ocean = append(ocean, provinceID)
			}
			continue
		}
		first, second := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
		neighbors[first] = append(neighbors[first], second)
		neighbors[second] = append(neighbors[second], first)
	}
	// Water reachable from the world boundary is ocean; all other water must
	// belong to a basin.
	for next := 0; next < len(ocean); next++ {
		for _, neighborID := range neighbors[ocean[next]] {
			if !reached[neighborID] && world.Provinces[neighborID].IslandID == NoIslandID {
				reached[neighborID] = true
				ocean = append(ocean, neighborID)
			}
		}
	}
	for provinceID, province := range world.Provinces {
		inBasin := province.IslandID == NoIslandID && !reached[provinceID]
		if (province.BasinID != NoBasinID) != inBasin {
			t.Errorf("province %d basin = %d, want membership %t", provinceID, province.BasinID, inBasin)
		}
	}

	inlandSeas := 0
	for index, basin := range world.Basins {
		if basin.ID != BasinID(index) {
			t.Errorf("basin %d ID = %d", index, basin.ID)
		}
		if index > 0 && world.Basins[index-1].ProvinceIDs[0] >= basin.ProvinceIDs[0] {
			t.Errorf("basin %d is not ordered by lowest province ID", index)
		}
		if !slices.IsSorted(basin.ProvinceIDs) {
			t.Errorf("basin %d province IDs are not ascending: %v", index, basin.ProvinceIDs)
		}
		wantTerrain := basinTerrain(basin)
		if wantTerrain == TerrainInlandSea {
			inlandSeas++
		}
		surface, depth := math.Inf(1), 0.0
		for _, provinceID := range basin.ProvinceIDs {
			province := world.Provinces[provinceID]
			if province.BasinID != basin.ID {
				t.Errorf("basin %d member %d has basin %d", index, provinceID, province.BasinID)
			}
			if province.Terrain != wantTerrain {
				t.Errorf("basin %d member %d terrain = %q, want %q", index, provinceID, province.Terrain, wantTerrain)
			}
			depth = max(depth, -province.Elevation)
			for _, neighborID := range neighbors[provinceID] {
				neighbor := world.Provinces[neighborID]
				if neighbor.IslandID != NoIslandID {
					surface = min(surface, neighbor.Elevation)
				} else if neighbor.BasinID != basin.ID {
					t.Errorf("basin %d member %d borders water %d outside the basin", index, provinceID, neighborID)
				}
			}
		}
		if basin.SurfaceElevation != surface || basin.SurfaceElevation < 0 || basin.SurfaceElevation >= 1 {
			t.Errorf("basin %d surface = %g, want lowest rim elevation %g in [0, 1)", index, basin.SurfaceElevation, surface)
		}
		if basin.Depth != depth || basin.Depth < 0 || basin.Depth >= 1 {
			t.Errorf("basin %d depth = %g, want deepest member %g in [0, 1)", index, basin.Depth, depth)
		}
	}
	if inlandSeas == 0 {
		t.Error("generated world has no inland sea")
	}

	rerun, err := Generate(config)
	if err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}
	rerun.Basins = nil
	for provinceID := range rerun.Provinces {
		rerun.Provinces[provinceID].BasinID = 0
	}
	assignBasins(&rerun)
	if !reflect.DeepEqual(world, rerun) {
		t.Fatal("assignBasins changed fields other than basins, or is not deterministic")
	}
}
