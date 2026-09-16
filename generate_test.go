package wgvc

import (
	"reflect"
	"testing"
)

func TestGenerateRejectsInvalidConfig(t *testing.T) {
	for _, config := range []Config{
		{},
		{ProvinceCount: 1, IslandCount: 0},
		{ProvinceCount: 0, IslandCount: 1},
		{ProvinceCount: -1, IslandCount: 1},
		{ProvinceCount: 2, IslandCount: 3},
	} {
		if _, err := Generate(config); err == nil {
			t.Errorf("Generate(%+v) returned no error", config)
		}
	}
}

func TestGenerateBuildsCanonicalAllocationWorld(t *testing.T) {
	world, err := Generate(Config{WorldSeed: 42, ProvinceCount: 20, IslandCount: 4})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got, want := len(world.Islands), 4; got != want {
		t.Fatalf("len(Islands) = %d, want %d", got, want)
	}
	if got, want := len(world.Provinces), 20; got != want {
		t.Fatalf("len(Provinces) = %d, want %d", got, want)
	}

	wantCounts := []int{8, 6, 3, 3}
	nextProvince := ProvinceID(0)
	for islandIndex, island := range world.Islands {
		if island.ID != IslandID(islandIndex) {
			t.Errorf("island at index %d has ID %d", islandIndex, island.ID)
		}
		if got := len(island.ProvinceIDs); got != wantCounts[islandIndex] {
			t.Errorf("island %d has %d provinces, want %d", island.ID, got, wantCounts[islandIndex])
		}
		for _, provinceID := range island.ProvinceIDs {
			if provinceID != nextProvince {
				t.Fatalf("province membership ID = %d, want %d", provinceID, nextProvince)
			}
			province := world.Provinces[provinceID]
			if province.ID != provinceID || province.IslandID != island.ID {
				t.Errorf("province %d = %+v, want matching canonical and island IDs", provinceID, province)
			}
			nextProvince++
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	config := Config{WorldSeed: 1234, ProvinceCount: 20, IslandCount: 4}
	first, err := Generate(config)
	if err != nil {
		t.Fatalf("first Generate() error = %v", err)
	}
	second, err := Generate(config)
	if err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Generate() results differ:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}
