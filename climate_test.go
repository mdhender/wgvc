package wgvc

import (
	"math"
	"slices"
	"testing"
)

func TestClimatePopulationCountsUseLargestRemainders(t *testing.T) {
	for _, test := range []struct {
		population int
		want       [5]int
		ok         bool
	}{
		{population: 4, want: [5]int{0, 1, 2, 1, 0}},
		{population: 19, want: [5]int{1, 3, 8, 5, 2}, ok: true},
		{population: 20, want: [5]int{1, 3, 9, 5, 2}, ok: true},
	} {
		got, ok := climatePopulationCounts(test.population)
		if got != test.want || ok != test.ok {
			t.Errorf("climatePopulationCounts(%d) = %v, %t, want %v, %t", test.population, got, ok, test.want, test.ok)
		}
	}
}

func TestClimateClassificationBreaksValueTiesByIslandThenProvince(t *testing.T) {
	provinces := make([]Province, 20)
	values := make([]float64, len(provinces))
	for provinceID := range provinces {
		provinces[provinceID] = Province{ID: ProvinceID(provinceID), IslandID: IslandID(provinceID % 3)}
		values[provinceID] = 0.5
	}
	provinces[17].IslandID = NoIslandID

	bands, ok := classifyClimatePopulation(provinces, values)
	if !ok {
		t.Fatal("20 provinces should support all five climate bands")
	}
	if got := bands[17]; got != int(HeatBandPolar) {
		t.Errorf("water province won value tie with band %d, want polar", got)
	}
	if got := bands[0]; got != int(HeatBandCold) {
		t.Errorf("lowest-ID province on lowest land island has band %d, want cold", got)
	}
	counts := [5]int{}
	for _, band := range bands {
		counts[band]++
	}
	if want := [5]int{1, 3, 9, 5, 2}; counts != want {
		t.Errorf("band counts = %v, want %v", counts, want)
	}
}

func TestAdjustedHeatUsesLatitudeAndCoolsOnlyPositiveLandElevation(t *testing.T) {
	provinces := []Province{
		{IslandID: NoIslandID, Elevation: -0.8},
		{IslandID: 0, Elevation: 0.2},
		{IslandID: 0, Elevation: 0.8},
		{IslandID: 0, Elevation: -0.2},
	}
	got := adjustedHeatValues(provinces, []float64{0.4, 0.4, 0.4, 0.4}, []float64{0, 0.5, 0.5, 1}, 0.4, 0.5)
	want := []float64{0.4, 0.5, 0.2, 0.8}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-15 {
			t.Errorf("province %d heat = %g, want %g", i, got[i], want[i])
		}
	}
}

func TestClimateFallbackAssignsMiddleBands(t *testing.T) {
	provinces := []Province{
		{ID: 0, IslandID: NoIslandID},
		{ID: 1, IslandID: 0, Elevation: 0.2},
		{ID: 2, IslandID: 0, Elevation: 0.5},
		{ID: 3, IslandID: 0, Elevation: 0.8},
	}
	calibration := calibrateHeat(provinces, []float64{0.1, 0.3, 0.6, 0.9}, []float64{0, 0.3, 0.6, 1}, DefaultClimateConfig())
	for provinceID, band := range calibration.bands {
		if band != int(HeatBandTemperate) {
			t.Errorf("tiny-map province %d heat band = %d, want temperate", provinceID, band)
		}
	}
	if _, ok := classifyClimatePopulation(provinces, []float64{0.1, 0.3, 0.6, 0.9}); ok {
		t.Fatal("tiny moisture population unexpectedly supported all five bands")
	}
}

func TestGenerateAssignsIndependentBoundedClimateFromCorners(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 128, IslandCount: 7}
	world, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	moistureNoise := valueNoise{seed: moistureRandom(config.WorldSeed).Uint64()}
	moistureCorners := make([]float64, len(world.Corners))
	for cornerID, corner := range world.Corners {
		moistureCorners[cornerID] = moistureNoise.sample(corner.Point)
	}
	wantMoisture := averageProvinceCorners(world.Provinces, moistureCorners)
	seenHeat, seenMoisture := map[float64]bool{}, map[float64]bool{}
	for provinceID, province := range world.Provinces {
		if math.IsNaN(province.Heat) || math.IsInf(province.Heat, 0) || province.Heat < 0 || province.Heat > 1 {
			t.Errorf("province %d heat = %g, want finite value in [0, 1]", provinceID, province.Heat)
		}
		if province.HeatBand < HeatBandPolar || province.HeatBand > HeatBandHot {
			t.Errorf("province %d heat band = %d, want valid band", provinceID, province.HeatBand)
		}
		if province.Moisture != wantMoisture[provinceID] {
			t.Errorf("province %d moisture = %g, want corner mean %g", provinceID, province.Moisture, wantMoisture[provinceID])
		}
		if province.MoistureBand < MoistureBandArid || province.MoistureBand > MoistureBandSaturated {
			t.Errorf("province %d moisture band = %d, want valid band", provinceID, province.MoistureBand)
		}
		seenHeat[province.Heat] = true
		seenMoisture[province.Moisture] = true
	}
	if len(seenHeat) < 2 || len(seenMoisture) < 2 {
		t.Fatalf("climate fields lack variation: heat=%d moisture=%d", len(seenHeat), len(seenMoisture))
	}

	heatNoise := valueNoise{seed: heatRandom(config.WorldSeed).Uint64()}
	heatCorners := make([]float64, len(world.Corners))
	for cornerID, corner := range world.Corners {
		heatCorners[cornerID] = heatNoise.sample(corner.Point)
	}
	if slices.Equal(heatCorners, moistureCorners) {
		t.Fatal("heat and moisture corner fields are identical")
	}
}

func TestGenerateLargeWorldCalibratesAllHeatBands(t *testing.T) {
	world, err := Generate(Config{WorldSeed: 42, ProvinceCount: 1_500, IslandCount: 15})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	want, ok := climatePopulationCounts(len(world.Provinces))
	if !ok {
		t.Fatalf("population %d unexpectedly cannot represent all bands", len(world.Provinces))
	}
	got := [5]int{}
	for _, province := range world.Provinces {
		got[province.HeatBand]++
	}
	if got != want {
		t.Errorf("heat band counts = %v, want calibrated population shares %v", got, want)
	}
}

func TestClimateConfigValidationAndDefaults(t *testing.T) {
	got, err := normalizeClimateConfig(ClimateConfig{})
	if err != nil {
		t.Fatalf("normalizeClimateConfig() error = %v", err)
	}
	if got != DefaultClimateConfig() {
		t.Errorf("zero climate config = %+v, want defaults %+v", got, DefaultClimateConfig())
	}
	for _, config := range []ClimateConfig{
		{PolarIce: -0.1},
		{PolarIce: 1.1},
		{PolarIce: math.NaN()},
		{PeakChill: -0.1},
		{PeakChill: 1.1},
		{PeakChill: math.Inf(1)},
	} {
		if _, err := normalizeClimateConfig(config); err == nil {
			t.Errorf("normalizeClimateConfig(%+v) returned no error", config)
		}
	}
}
