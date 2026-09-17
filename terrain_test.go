package wgvc

import (
	"math"
	"reflect"
	"testing"
)

func TestValueNoiseSamplesAreDeterministicFiniteBoundedAndSmooth(t *testing.T) {
	first := newTerrainNoise(42)
	second := newTerrainNoise(42)
	otherSeed := newTerrainNoise(43)
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
	left := first.sample(Point{X: terrainWavelength - epsilon, Y: 1.25})
	right := first.sample(Point{X: terrainWavelength + epsilon, Y: 1.25})
	if math.Abs(left-right) > epsilon {
		t.Fatalf("samples across a lattice boundary differ by %g", math.Abs(left-right))
	}
}

func TestTerrainFromKnownCornerValues(t *testing.T) {
	cornerValues := []float64{0.1, 0.3, 0.5, 0.7, 0.9}
	for _, test := range []struct {
		name      string
		cornerIDs []CornerID
		want      Terrain
	}{
		{name: "plains", cornerIDs: []CornerID{0, 1}, want: TerrainPlains},
		{name: "plains threshold belongs to hills", cornerIDs: []CornerID{1, 2}, want: TerrainHills},
		{name: "hills", cornerIDs: []CornerID{1, 3}, want: TerrainHills},
		{name: "hills threshold belongs to mountains", cornerIDs: []CornerID{2, 3}, want: TerrainMountains},
		{name: "mountains", cornerIDs: []CornerID{3, 4}, want: TerrainMountains},
	} {
		t.Run(test.name, func(t *testing.T) {
			province := Province{CornerIDs: test.cornerIDs}
			if got := terrainFromCorners(province, cornerValues); got != test.want {
				t.Fatalf("terrainFromCorners() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAssignTerrainChangesOnlyTerrain(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 97, IslandCount: 7}
	got, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	want, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	noise := newTerrainNoise(8675309)
	cornerValues := make([]float64, len(got.Corners))
	for cornerID, corner := range got.Corners {
		cornerValues[cornerID] = noise.sample(corner.Point)
	}
	assignTerrain(&got, cornerValues)
	for provinceID := range got.Provinces {
		got.Provinces[provinceID].Terrain = ""
		want.Provinces[provinceID].Terrain = ""
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("assignTerrain changed world geometry or topology")
	}
}

func TestAssignElevationsFromEdges(t *testing.T) {
	world := World{
		Provinces: []Province{
			{IslandID: 0, Terrain: TerrainWater},
			{IslandID: 0, Terrain: TerrainWater},
			{IslandID: NoIslandID, Terrain: TerrainPlains},
			{IslandID: NoIslandID, Terrain: TerrainPlains},
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
	if world.Provinces[0].Terrain != TerrainWater || world.Provinces[2].Terrain != TerrainPlains {
		t.Fatal("elevation assignment changed legacy terrain")
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
		if province.Terrain != TerrainWater {
			terrains[province.Terrain] = true
		}
	}
	if len(terrains) < 2 {
		t.Fatalf("larger world has only one terrain class: %v", terrains)
	}
}
