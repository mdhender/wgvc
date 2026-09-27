package wgvc

import (
	"math"
	"reflect"
	"testing"
)

func TestAssignReliefUsesSameMediumEdgeNeighbors(t *testing.T) {
	// Provinces 0-2 form a land triangle, province 4 is land touching only
	// water and the world boundary, and provinces 3 and 5 are water.
	world := World{
		Provinces: []Province{
			{ID: 0, IslandID: 0, Elevation: 0.5},
			{ID: 1, IslandID: 0, Elevation: 0.2},
			{ID: 2, IslandID: 0, Elevation: 0.9},
			{ID: 3, IslandID: NoIslandID, Elevation: -0.4},
			{ID: 4, IslandID: 1, Elevation: 0.3, Relief: 0.8},
			{ID: 5, IslandID: NoIslandID, Elevation: -0.9},
		},
		Edges: []Edge{
			{ID: 0, ProvinceIDs: []ProvinceID{0, 1}},
			{ID: 1, ProvinceIDs: []ProvinceID{0, 2}},
			{ID: 2, ProvinceIDs: []ProvinceID{0, 3}}, // coastline
			{ID: 3, ProvinceIDs: []ProvinceID{1, 2}},
			{ID: 4, ProvinceIDs: []ProvinceID{0}}, // world boundary
			{ID: 5, ProvinceIDs: []ProvinceID{3, 5}},
			{ID: 6, ProvinceIDs: []ProvinceID{3, 4}}, // coastline
			{ID: 7, ProvinceIDs: []ProvinceID{4}},    // world boundary
		},
	}

	assignRelief(&world)

	want := []float64{
		(0.3 + 0.4) / 2, // neighbors 1 and 2; coast and boundary ignored
		(0.3 + 0.7) / 2, // neighbors 0 and 2
		(0.4 + 0.7) / 2, // neighbors 0 and 1
		0.5,             // water neighbor 5; coastlines to 0 and 4 ignored
		0,               // no same-medium neighbor; stale value cleared
		0.5,             // water neighbor 3
	}
	for provinceID, province := range world.Provinces {
		if math.Abs(province.Relief-want[provinceID]) > 1e-12 {
			t.Errorf("province %d relief = %.17g, want %g", provinceID, province.Relief, want[provinceID])
		}
		if math.IsNaN(province.Relief) || province.Relief < 0 || province.Relief > 1 {
			t.Errorf("province %d relief = %g, want finite value in [0, 1]", provinceID, province.Relief)
		}
	}
}

func TestAssignReliefExtremesStayInUnitRange(t *testing.T) {
	world := World{
		Provinces: []Province{
			{ID: 0, IslandID: 0, Elevation: 0},
			{ID: 1, IslandID: 0, Elevation: math.Nextafter(1, 0)},
			{ID: 2, IslandID: NoIslandID, Elevation: 0},
			{ID: 3, IslandID: NoIslandID, Elevation: math.Nextafter(-1, 0)},
		},
		Edges: []Edge{
			{ID: 0, ProvinceIDs: []ProvinceID{0, 1}},
			{ID: 1, ProvinceIDs: []ProvinceID{2, 3}},
			{ID: 2, ProvinceIDs: []ProvinceID{1, 3}}, // coastline spanning nearly 2
		},
	}

	assignRelief(&world)

	for provinceID, province := range world.Provinces {
		if math.IsNaN(province.Relief) || province.Relief < 0 || province.Relief >= 1 {
			t.Errorf("province %d relief = %.17g, want finite value in [0, 1)", provinceID, province.Relief)
		}
	}
}

func TestGenerateAssignsReliefWithoutChangingOtherFields(t *testing.T) {
	config := Config{WorldSeed: 0x0123456789abcdef, ProvinceCount: 1000, IslandCount: 8}
	world, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	type neighborSum struct {
		total float64
		count int
	}
	sums := make([]neighborSum, len(world.Provinces))
	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) != 2 {
			continue
		}
		first, second := world.Provinces[edge.ProvinceIDs[0]], world.Provinces[edge.ProvinceIDs[1]]
		if (first.IslandID == NoIslandID) != (second.IslandID == NoIslandID) {
			continue
		}
		difference := math.Abs(first.Elevation - second.Elevation)
		for _, provinceID := range edge.ProvinceIDs {
			sums[provinceID].total += difference
			sums[provinceID].count++
		}
	}
	nonzero := 0
	for provinceID, province := range world.Provinces {
		want := 0.0
		if sums[provinceID].count > 0 {
			want = sums[provinceID].total / float64(sums[provinceID].count)
		}
		if math.Abs(province.Relief-want) > 1e-12 {
			t.Errorf("province %d relief = %g, want same-medium neighbor mean %g", provinceID, province.Relief, want)
		}
		if province.Relief > 0 {
			nonzero++
		}
	}
	if nonzero == 0 {
		t.Fatal("every generated province has zero relief")
	}

	// Rerunning the pass from zeroed relief must reproduce the generated values
	// and leave geometry, adjacency, membership, elevation, and climate alone.
	rerun, err := Generate(config)
	if err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}
	for provinceID := range rerun.Provinces {
		rerun.Provinces[provinceID].Relief = 0
	}
	assignRelief(&rerun)
	if !reflect.DeepEqual(world, rerun) {
		t.Fatal("assignRelief changed fields other than relief, or is not deterministic")
	}
}
