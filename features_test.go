package wgvc

import (
	"reflect"
	"testing"
)

func TestArchipelagosJoinIslandsWithinTheGap(t *testing.T) {
	const w = NoIslandID
	for _, test := range []struct {
		name string
		row  []IslandID
		want [][]IslandID
	}{
		{"six water provinces join", []IslandID{0, w, w, w, w, w, w, 1}, [][]IslandID{{0, 1}}},
		{"seven water provinces do not", []IslandID{0, w, w, w, w, w, w, w, 1}, nil},
		{"chains are transitive", []IslandID{0, w, 1, w, w, w, w, w, w, 2, w, w, w, w, w, w, w, 3}, [][]IslandID{{0, 1, 2}}},
	} {
		world := chainWorld(test.row)
		if got := archipelagos(&world, provinceNeighbors(&world)); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: archipelagos = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestFeaturesGroupTerrainFamiliesAboveMinimumSize(t *testing.T) {
	world := chainWorld([]IslandID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	terrains := []Terrain{
		TerrainMountain, TerrainAlpine, TerrainMountain, TerrainMountain, TerrainAlpine, // 5: a range
		TerrainPlains,
		TerrainHills, TerrainBadlands, TerrainHills, TerrainHills, // 4: too small
		TerrainPlateau,
	}
	for id, terrain := range terrains {
		world.Provinces[id].Terrain = terrain
	}
	assignFeatures(&world)
	want := []Feature{{ID: 0, Kind: FeatureMountainRange, IslandIDs: []IslandID{0}, ProvinceIDs: []ProvinceID{0, 1, 2, 3, 4}}}
	if !reflect.DeepEqual(world.Features, want) {
		t.Errorf("features = %+v, want %+v", world.Features, want)
	}
}

func TestDefaultWorldHasFeaturesOfSeveralKinds(t *testing.T) {
	world, err := Generate(Config{WorldSeed: 42, ProvinceCount: 1_500, IslandCount: 15})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[FeatureKind]int{}
	for _, feature := range world.Features {
		kinds[feature.Kind]++
	}
	for _, kind := range []FeatureKind{FeatureMountainRange, FeatureHillCountry, FeaturePlateau, FeatureForest} {
		if kinds[kind] == 0 {
			t.Errorf("no %s feature in a 1,500-province world: %v", kind, kinds)
		}
	}
	assertValidFeatures(t, world)
}
