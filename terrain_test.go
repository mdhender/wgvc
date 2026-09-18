package wgvc

import (
	"math"
	"testing"
)

func TestValueNoiseSamplesAreDeterministicFiniteBoundedAndSmooth(t *testing.T) {
	first := newElevationNoise(42)
	second := newElevationNoise(42)
	otherSeed := newElevationNoise(43)
	seedProbe := Point{X: 8, Y: 12}
	if first.sample(seedProbe) == otherSeed.sample(seedProbe) {
		t.Fatalf("different seeds produced the same sample at %+v", seedProbe)
	}
	for x := -20.0; x <= 20; x += 0.25 {
		for y := -20.0; y <= 20; y += 0.25 {
			point := Point{X: x, Y: y}
			got := first.sample(point)
			if got != second.sample(point) {
				t.Fatalf("sample at %+v is not deterministic", point)
			}
			if math.IsNaN(got) || math.IsInf(got, 0) || got < 0 || got > 1 {
				t.Fatalf("sample at %+v = %g, want finite value in [0, 1]", point, got)
			}
			nearby := first.sample(Point{X: x + 0.01, Y: y})
			if math.Abs(got-nearby) > 0.01 {
				t.Fatalf("nearby samples at (%g, %g) differ by %g", x, y, math.Abs(got-nearby))
			}
		}
	}

	const epsilon = 1e-7
	left := first.sample(Point{X: elevationWavelength - epsilon, Y: 1.25})
	right := first.sample(Point{X: elevationWavelength + epsilon, Y: 1.25})
	if math.Abs(left-right) > epsilon {
		t.Fatalf("samples across a lattice boundary differ by %g", math.Abs(left-right))
	}
}

func TestTerrainVocabularyIsStableAndComplete(t *testing.T) {
	want := []Terrain{
		"deep-ocean", "ocean", "shallow-sea", "coastal-water", "inland-sea", "lake",
		"glacial-ice", "tundra", "marsh", "swamp", "bog",
		"desert", "badlands", "scrubland", "plains", "grassland", "steppe", "savanna",
		"boreal-forest", "temperate-forest", "rainforest", "hills", "mountain", "alpine",
		"volcano", "volcanic-highland", "coast",
	}
	got := Terrains()
	if len(got) != len(want) {
		t.Fatalf("Terrains() returned %d values, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] || !got[index].Valid() {
			t.Errorf("terrain %d = %q (valid=%t), want %q", index, got[index], got[index].Valid(), want[index])
		}
	}
	got[0] = "changed"
	if Terrains()[0] != TerrainDeepOcean {
		t.Fatal("Terrains returned mutable package storage")
	}
	if (Terrain("unknown")).Valid() {
		t.Fatal("unknown terrain is valid")
	}
	for _, terrain := range want {
		wantWater := terrain == TerrainDeepOcean || terrain == TerrainOcean || terrain == TerrainShallowSea ||
			terrain == TerrainCoastalWater || terrain == TerrainInlandSea || terrain == TerrainLake
		if terrain.IsWater() != wantWater {
			t.Errorf("terrain %q IsWater() = %t, want %t", terrain, terrain.IsWater(), wantWater)
		}
	}
}

func TestClassifyTerrainPrecedence(t *testing.T) {
	land := Province{IslandID: 0, Elevation: 0.1, ElevationBand: ElevationBandLowland, HeatBand: HeatBandTemperate, MoistureBand: MoistureBandModerate}
	for _, test := range []struct {
		name          string
		province      Province
		adjacentLand  bool
		adjacentWater bool
		want          Terrain
	}{
		{name: "coastal water precedes depth", province: Province{IslandID: NoIslandID, Elevation: -0.8, ElevationBand: ElevationBandDeepWater}, adjacentLand: true, want: TerrainCoastalWater},
		{name: "deep ocean", province: Province{IslandID: NoIslandID, Elevation: -0.8, ElevationBand: ElevationBandDeepWater}, want: TerrainDeepOcean},
		{name: "ocean", province: Province{IslandID: NoIslandID, Elevation: -0.3, ElevationBand: ElevationBandShallowWater}, want: TerrainOcean},
		{name: "shallow sea", province: Province{IslandID: NoIslandID, Elevation: -0.05, ElevationBand: ElevationBandShallowWater}, want: TerrainShallowSea},
		{name: "glacial ice", province: Province{IslandID: 0, ElevationBand: ElevationBandLowland, HeatBand: HeatBandPolar, MoistureBand: MoistureBandDry}, want: TerrainGlacialIce},
		{name: "arid polar land is tundra", province: Province{IslandID: 0, ElevationBand: ElevationBandLowland, HeatBand: HeatBandPolar, MoistureBand: MoistureBandArid}, want: TerrainTundra},
		{name: "cold mountain is alpine", province: Province{IslandID: 0, ElevationBand: ElevationBandMountain, HeatBand: HeatBandCold}, want: TerrainAlpine},
		{name: "warm mountain", province: Province{IslandID: 0, ElevationBand: ElevationBandMountain, HeatBand: HeatBandWarm}, want: TerrainMountain},
		{name: "rough highland is mountain", province: Province{IslandID: 0, ElevationBand: ElevationBandHighland, Relief: terrainMountainReliefMin}, want: TerrainMountain},
		{name: "smooth highland is hills", province: Province{IslandID: 0, ElevationBand: ElevationBandHighland, Relief: terrainMountainReliefMin - 0.01}, want: TerrainHills},
		{name: "cold wetland is bog", province: Province{IslandID: 0, ElevationBand: ElevationBandLowland, HeatBand: HeatBandCold, MoistureBand: MoistureBandHumid}, want: TerrainBog},
		{name: "temperate wetland is marsh", province: Province{IslandID: 0, ElevationBand: ElevationBandLowland, HeatBand: HeatBandTemperate, MoistureBand: MoistureBandHumid}, want: TerrainMarsh},
		{name: "warm wetland is swamp", province: Province{IslandID: 0, ElevationBand: ElevationBandLowland, HeatBand: HeatBandWarm, MoistureBand: MoistureBandSaturated}, want: TerrainSwamp},
		{name: "coast", province: Province{IslandID: 0, Elevation: terrainCoastMax, ElevationBand: ElevationBandLowland, HeatBand: HeatBandTemperate, MoistureBand: MoistureBandModerate}, adjacentWater: true, want: TerrainCoast},
		{name: "rough dry land is badlands", province: Province{IslandID: 0, Elevation: 0.1, ElevationBand: ElevationBandLowland, Relief: terrainBadlandsReliefMin, HeatBand: HeatBandWarm, MoistureBand: MoistureBandDry}, want: TerrainBadlands},
		{name: "rough upland is hills", province: Province{IslandID: 0, ElevationBand: ElevationBandUpland, Relief: terrainHillReliefMin, HeatBand: HeatBandTemperate, MoistureBand: MoistureBandModerate}, want: TerrainHills},
		{name: "ordinary land uses climate cover", province: land, want: TerrainPlains},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyTerrain(test.province, test.adjacentLand, test.adjacentWater); got != test.want {
				t.Fatalf("classifyTerrain() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTerrainCoverTable(t *testing.T) {
	want := [5][5]Terrain{
		{TerrainTundra, TerrainTundra, TerrainTundra, TerrainTundra, TerrainTundra},
		{TerrainTundra, TerrainSteppe, TerrainBorealForest, TerrainBorealForest, TerrainBorealForest},
		{TerrainScrubland, TerrainGrassland, TerrainPlains, TerrainTemperateForest, TerrainTemperateForest},
		{TerrainDesert, TerrainScrubland, TerrainSavanna, TerrainTemperateForest, TerrainTemperateForest},
		{TerrainDesert, TerrainScrubland, TerrainSavanna, TerrainRainforest, TerrainRainforest},
	}
	for heat := range want {
		for moisture := range want[heat] {
			if terrainCover[heat][moisture] != want[heat][moisture] {
				t.Errorf("cover[%d][%d] = %q, want %q", heat, moisture, terrainCover[heat][moisture], want[heat][moisture])
			}
		}
	}
}

func TestAssignTerrainUsesEdgeAdjacency(t *testing.T) {
	world := World{
		Provinces: []Province{
			{IslandID: 0, Elevation: 0.02, ElevationBand: ElevationBandLowland, HeatBand: HeatBandTemperate, MoistureBand: MoistureBandModerate},
			{IslandID: NoIslandID, Elevation: -0.8, ElevationBand: ElevationBandDeepWater},
			{IslandID: NoIslandID, Elevation: -0.8, ElevationBand: ElevationBandDeepWater},
			{IslandID: 0, Elevation: 0.1, ElevationBand: ElevationBandLowland, HeatBand: HeatBandWarm, MoistureBand: MoistureBandSaturated},
		},
		Edges: []Edge{{ProvinceIDs: []ProvinceID{0, 1}}},
	}

	assignTerrain(&world)

	want := []Terrain{TerrainCoast, TerrainCoastalWater, TerrainDeepOcean, TerrainSwamp}
	for provinceID, terrain := range want {
		if world.Provinces[provinceID].Terrain != terrain {
			t.Errorf("province %d terrain = %q, want %q", provinceID, world.Provinces[provinceID].Terrain, terrain)
		}
	}
}

func TestAssignElevationsFromEdges(t *testing.T) {
	world := World{
		Provinces: []Province{
			{IslandID: 0, Terrain: TerrainGrassland},
			{IslandID: 0, Terrain: TerrainGrassland},
			{IslandID: NoIslandID, Terrain: TerrainOcean},
			{IslandID: NoIslandID, Terrain: TerrainOcean},
		},
		Edges: []Edge{
			{CornerIDs: [2]CornerID{0, 1}, ProvinceIDs: []ProvinceID{0, 1}, Elevation: -1},
			{CornerIDs: [2]CornerID{1, 2}, ProvinceIDs: []ProvinceID{0, 2}, Elevation: -1},
			{CornerIDs: [2]CornerID{2, 3}, ProvinceIDs: []ProvinceID{2}, Elevation: -1},
			{CornerIDs: [2]CornerID{2, 3}, ProvinceIDs: []ProvinceID{2, 3}, Elevation: -1},
		},
	}
	cornerValues := []float64{0.2, 0.6, 0.1, 0.3}

	assignEdgeElevations(&world, cornerValues)
	assignProvinceElevations(&world)

	want := []float64{0.46, 0, 0, -0.82}
	for edgeID, edge := range world.Edges {
		if math.Abs(edge.Elevation-want[edgeID]) > 1e-15 {
			t.Errorf("edge %d elevation = %g, want %g", edgeID, edge.Elevation, want[edgeID])
		}
	}
	wantElevations := []float64{0.23, 0.46, -0.82 / 3, -0.82}
	wantBands := []ElevationBand{ElevationBandUpland, ElevationBandHighland, ElevationBandShallowWater, ElevationBandDeepWater}
	for provinceID, province := range world.Provinces {
		if math.Abs(province.Elevation-wantElevations[provinceID]) > 1e-15 {
			t.Errorf("province %d elevation = %g, want %g", provinceID, province.Elevation, wantElevations[provinceID])
		}
		if province.ElevationBand != wantBands[provinceID] {
			t.Errorf("province %d elevation band = %d, want %d", provinceID, province.ElevationBand, wantBands[provinceID])
		}
	}
	if world.Provinces[0].Terrain != TerrainGrassland || world.Provinces[2].Terrain != TerrainOcean {
		t.Fatal("elevation assignment changed terrain")
	}
}

func TestClassifyElevationPreservesLandAndWaterAtBoundaries(t *testing.T) {
	for _, test := range []struct {
		name      string
		land      bool
		elevation float64
		want      ElevationBand
	}{
		{name: "deep water", elevation: -0.5000001, want: ElevationBandDeepWater},
		{name: "deep-water threshold is shallow water", elevation: -0.5, want: ElevationBandShallowWater},
		{name: "water at sea level", elevation: 0, want: ElevationBandShallowWater},
		{name: "land at sea level", land: true, elevation: 0, want: ElevationBandLowland},
		{name: "lowland threshold is upland", land: true, elevation: 0.2, want: ElevationBandUpland},
		{name: "upland threshold is highland", land: true, elevation: 0.4, want: ElevationBandHighland},
		{name: "highland threshold is mountain", land: true, elevation: 0.6, want: ElevationBandMountain},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyElevation(test.land, test.elevation); got != test.want {
				t.Fatalf("classifyElevation(%t, %g) = %d, want %d", test.land, test.elevation, got, test.want)
			}
		})
	}
	if !(ElevationBandDeepWater < ElevationBandShallowWater &&
		ElevationBandShallowWater < ElevationBandLowland &&
		ElevationBandLowland < ElevationBandUpland &&
		ElevationBandUpland < ElevationBandHighland &&
		ElevationBandHighland < ElevationBandMountain) {
		t.Fatal("elevation bands are not ordered from deep water through mountain")
	}
}

func TestGenerateLargerWorldHasMultipleTerrainClasses(t *testing.T) {
	world, err := Generate(Config{WorldSeed: 42, ProvinceCount: 128, IslandCount: 7})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	terrains := make(map[Terrain]bool)
	for _, province := range world.Provinces {
		if province.IslandID != NoIslandID {
			terrains[province.Terrain] = true
		}
	}
	if len(terrains) < 2 {
		t.Fatalf("larger world has only one terrain class: %v", terrains)
	}
}
