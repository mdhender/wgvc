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

	assignTerrain(&got, 8675309)
	for provinceID := range got.Provinces {
		got.Provinces[provinceID].Terrain = ""
		want.Provinces[provinceID].Terrain = ""
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("assignTerrain changed world geometry or topology")
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
