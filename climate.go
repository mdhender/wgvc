package wgvc

import (
	"math"
	"sort"
)

const (
	climateCalibrationRounds = 12
	// minimumWarmthScale and maximumWarmthScale bound the blend weight of the
	// latitude gradient against the heat noise; at 1 heat is pure latitude,
	// at 0 pure noise. The floor keeps a visible gradient on worlds whose
	// peak-chill target is unreachable, where a smaller weight would only
	// reduce the error by flattening the world.
	minimumWarmthScale = 0.25
	maximumWarmthScale = 1.0
	// maximumCoolingStrength is the fraction of heat a land province at the
	// maximum elevation loses.
	maximumCoolingStrength = 1.0
	// climateTargetTolerance is how far a band fraction may sit from its
	// target, as an absolute fraction, and still count as reached; a half
	// province is used instead when that is larger.
	climateTargetTolerance = 0.005
)

var climateBandShares = [5]int{5, 15, 45, 25, 10}

type heatCalibration struct {
	values          []float64
	bands           []int
	warmthScale     float64
	coolingStrength float64
	error           float64
	polarReached    bool
	peakReached     bool
}

func assignClimate(world *World, worldSeed uint64, config ClimateConfig) {
	heatNoise := valueNoise{seed: heatRandom(worldSeed).Uint64()}
	moistureNoise := valueNoise{seed: moistureRandom(worldSeed).Uint64()}
	heatCorners := make([]float64, len(world.Corners))
	moistureCorners := make([]float64, len(world.Corners))
	for cornerID, corner := range world.Corners {
		heatCorners[cornerID] = heatNoise.sample(corner.Point)
		moistureCorners[cornerID] = moistureNoise.sample(corner.Point)
	}

	heatSource := averageProvinceCorners(world.Provinces, heatCorners)
	moisture := averageProvinceCorners(world.Provinces, moistureCorners)
	latitudes := provinceLatitudes(*world)
	calibration := calibrateHeat(world.Provinces, heatSource, latitudes, config)

	moistureBands, moistureOK := classifyClimatePopulation(world.Provinces, moisture)
	for provinceID := range world.Provinces {
		province := &world.Provinces[provinceID]
		province.Heat = calibration.values[provinceID]
		province.HeatBand = HeatBand(calibration.bands[provinceID])
		province.Moisture = moisture[provinceID]
		if moistureOK {
			province.MoistureBand = MoistureBand(moistureBands[provinceID])
		} else {
			province.MoistureBand = MoistureBandModerate
		}
	}
}

func averageProvinceCorners(provinces []Province, cornerValues []float64) []float64 {
	averages := make([]float64, len(provinces))
	for provinceID, province := range provinces {
		total := 0.0
		for _, cornerID := range province.CornerIDs {
			total += cornerValues[cornerID]
		}
		averages[provinceID] = total / float64(len(province.CornerIDs))
	}
	return averages
}

func provinceLatitudes(world World) []float64 {
	latitudes := make([]float64, len(world.Provinces))
	if len(world.Corners) == 0 {
		return latitudes
	}
	minimumY, maximumY := world.Corners[0].Point.Y, world.Corners[0].Point.Y
	for _, corner := range world.Corners[1:] {
		minimumY = min(minimumY, corner.Point.Y)
		maximumY = max(maximumY, corner.Point.Y)
	}
	span := maximumY - minimumY
	if span == 0 {
		for provinceID := range latitudes {
			latitudes[provinceID] = 0.5
		}
		return latitudes
	}
	for provinceID, province := range world.Provinces {
		latitudes[provinceID] = clamp01((province.Center.Y - minimumY) / span)
	}
	return latitudes
}

func calibrateHeat(provinces []Province, heatSource, latitudes []float64, config ClimateConfig) heatCalibration {
	peakSlice := peakCalibrationSlice(provinces, latitudes)
	oceanCount := 0
	for _, province := range provinces {
		if province.IslandID == NoIslandID {
			oceanCount++
		}
	}

	counts, classifiable := climatePopulationCounts(len(provinces))
	if !classifiable || oceanCount == 0 || len(peakSlice) == 0 {
		return middleHeatCalibration(provinces, heatSource, latitudes, (minimumWarmthScale+maximumWarmthScale)/2, maximumCoolingStrength/2)
	}

	polarTolerance := max(climateTargetTolerance, 0.5/float64(oceanCount))
	peakTolerance := max(climateTargetTolerance, 0.5/float64(len(peakSlice)))
	warmthLow, warmthHigh := minimumWarmthScale, maximumWarmthScale
	coolingLow, coolingHigh := 0.0, maximumCoolingStrength
	var best heatCalibration
	haveBest := false

	for round := range climateCalibrationRounds {
		gridSize := 5
		if round == 0 {
			gridSize = 17
		}
		warmthStep := (warmthHigh - warmthLow) / float64(gridSize-1)
		coolingStep := (coolingHigh - coolingLow) / float64(gridSize-1)
		var roundBest heatCalibration
		haveRoundBest := false
		for warmthIndex := range gridSize {
			warmth := warmthLow + float64(warmthIndex)*warmthStep
			for coolingIndex := range gridSize {
				cooling := coolingLow + float64(coolingIndex)*coolingStep
				candidate := evaluateHeatCandidate(provinces, heatSource, latitudes, peakSlice, counts, config, warmth, cooling, polarTolerance, peakTolerance)
				if !haveBest || betterHeatCalibration(candidate, best) {
					best = candidate
					haveBest = true
				}
				if !haveRoundBest || betterHeatCalibration(candidate, roundBest) {
					roundBest = candidate
					haveRoundBest = true
				}
			}
		}

		warmthLow = max(minimumWarmthScale, roundBest.warmthScale-warmthStep)
		warmthHigh = min(maximumWarmthScale, roundBest.warmthScale+warmthStep)
		coolingLow = max(0, roundBest.coolingStrength-coolingStep)
		coolingHigh = min(maximumCoolingStrength, roundBest.coolingStrength+coolingStep)
	}

	// Calibration is best-effort: when no candidate met both targets, the
	// lowest-error candidate keeps its bands rather than collapsing the world
	// to one band. The all-temperate fallback is reserved for worlds that
	// cannot be banded at all.
	return best
}

func evaluateHeatCandidate(provinces []Province, heatSource, latitudes []float64, peakSlice []int, counts [5]int, config ClimateConfig, warmth, cooling, polarTolerance, peakTolerance float64) heatCalibration {
	values := adjustedHeatValues(provinces, heatSource, latitudes, warmth, cooling)
	bands := classifyClimatePopulationWithCounts(provinces, values, counts)
	polarFraction := heatBandFraction(provinces, bands, nil, int(HeatBandPolar))
	peakFraction := heatBandFraction(provinces, bands, peakSlice, int(HeatBandCold))
	return heatCalibration{
		values:          values,
		bands:           bands,
		warmthScale:     warmth,
		coolingStrength: cooling,
		error:           math.Abs(polarFraction-config.PolarIce) + math.Abs(peakFraction-config.PeakChill),
		polarReached:    math.Abs(polarFraction-config.PolarIce) <= polarTolerance,
		peakReached:     math.Abs(peakFraction-config.PeakChill) <= peakTolerance,
	}
}

func middleHeatCalibration(provinces []Province, heatSource, latitudes []float64, warmth, cooling float64) heatCalibration {
	bands := make([]int, len(provinces))
	for provinceID := range bands {
		bands[provinceID] = int(HeatBandTemperate)
	}
	return heatCalibration{
		values:          adjustedHeatValues(provinces, heatSource, latitudes, warmth, cooling),
		bands:           bands,
		warmthScale:     warmth,
		coolingStrength: cooling,
	}
}

// betterHeatCalibration orders candidates. Any candidate within tolerance of
// both targets beats any that is not; among reached candidates the strongest
// latitude gradient wins, because a weak gradient meets the targets by
// accident (noise alone puts about 5% of the ocean in the coldest 5%), and
// the targets are meant to bound how far the gradient can go. Among the rest
// the lowest error wins. Remaining ties fall to the lower cooling.
func betterHeatCalibration(candidate, current heatCalibration) bool {
	candidateReached := candidate.polarReached && candidate.peakReached
	currentReached := current.polarReached && current.peakReached
	if candidateReached != currentReached {
		return candidateReached
	}
	if candidateReached {
		if candidate.warmthScale != current.warmthScale {
			return candidate.warmthScale > current.warmthScale
		}
	} else if candidate.error != current.error {
		return candidate.error < current.error
	}
	if candidate.error != current.error {
		return candidate.error < current.error
	}
	if candidate.warmthScale != current.warmthScale {
		return candidate.warmthScale > current.warmthScale
	}
	return candidate.coolingStrength < current.coolingStrength
}

// adjustedHeatValues blends the heat noise with a north-cold latitude
// gradient, then cools land in proportion to its positive elevation:
//
//	heat = ((1 - warmth) * noise + warmth * (1 - latitude)) * (1 - cooling * max(0, elevation))
//
// Both factors lie in [0, 1], so the result does too without clamping and no
// value can pile up at either end. Latitude 1 is the top of the map (+Y,
// north), which is the cold pole; water receives no elevation cooling.
func adjustedHeatValues(provinces []Province, heatSource, latitudes []float64, warmth, cooling float64) []float64 {
	values := make([]float64, len(provinces))
	for provinceID, province := range provinces {
		heat := (1-warmth)*heatSource[provinceID] + warmth*(1-latitudes[provinceID])
		if province.IslandID != NoIslandID {
			heat *= 1 - cooling*max(0, province.Elevation)
		}
		values[provinceID] = heat
	}
	return values
}

func peakCalibrationSlice(provinces []Province, latitudes []float64) []int {
	land := make([]int, 0, len(provinces))
	for provinceID, province := range provinces {
		if province.IslandID != NoIslandID {
			land = append(land, provinceID)
		}
	}
	if len(land) == 0 {
		return nil
	}
	decileCount := (len(land) + 9) / 10
	byElevation := append([]int(nil), land...)
	sort.Slice(byElevation, func(i, j int) bool {
		first, second := byElevation[i], byElevation[j]
		if provinces[first].Elevation != provinces[second].Elevation {
			return provinces[first].Elevation < provinces[second].Elevation
		}
		return climateTieLess(provinces[first], provinces[second])
	})
	byLatitude := append([]int(nil), land...)
	sort.Slice(byLatitude, func(i, j int) bool {
		first, second := byLatitude[i], byLatitude[j]
		if latitudes[first] != latitudes[second] {
			return latitudes[first] < latitudes[second]
		}
		return climateTieLess(provinces[first], provinces[second])
	})

	highElevation := make(map[int]bool, decileCount)
	for _, provinceID := range byElevation[len(byElevation)-decileCount:] {
		highElevation[provinceID] = true
	}
	result := make([]int, 0, decileCount)
	for _, provinceID := range byLatitude[len(byLatitude)-decileCount:] {
		if highElevation[provinceID] {
			result = append(result, provinceID)
		}
	}
	return result
}

func classifyClimatePopulation(provinces []Province, values []float64) ([]int, bool) {
	counts, ok := climatePopulationCounts(len(provinces))
	if !ok {
		return nil, false
	}
	return classifyClimatePopulationWithCounts(provinces, values, counts), true
}

func classifyClimatePopulationWithCounts(provinces []Province, values []float64, counts [5]int) []int {
	order := make([]int, len(provinces))
	for provinceID := range provinces {
		order[provinceID] = provinceID
	}
	sort.Slice(order, func(i, j int) bool {
		first, second := order[i], order[j]
		if values[first] != values[second] {
			return values[first] < values[second]
		}
		return climateTieLess(provinces[first], provinces[second])
	})
	bands := make([]int, len(provinces))
	offset := 0
	for band, count := range counts {
		for _, provinceID := range order[offset : offset+count] {
			bands[provinceID] = band
		}
		offset += count
	}
	return bands
}

func climatePopulationCounts(population int) ([5]int, bool) {
	var counts, remainders [5]int
	assigned := 0
	for band, share := range climateBandShares {
		product := population * share
		counts[band] = product / 100
		remainders[band] = product % 100
		assigned += counts[band]
	}
	for ; assigned < population; assigned++ {
		best := 0
		for band := 1; band < len(remainders); band++ {
			if remainders[band] > remainders[best] {
				best = band
			}
		}
		counts[best]++
		remainders[best] = -1
	}
	for _, count := range counts {
		if count == 0 {
			return counts, false
		}
	}
	return counts, true
}

func climateTieLess(first, second Province) bool {
	if first.IslandID != second.IslandID {
		return first.IslandID < second.IslandID
	}
	return first.ID < second.ID
}

func heatBandFraction(provinces []Province, bands []int, subset []int, maximumBand int) float64 {
	matched, total := 0, 0
	if subset != nil {
		for _, provinceID := range subset {
			total++
			if bands[provinceID] <= maximumBand {
				matched++
			}
		}
		return float64(matched) / float64(total)
	}
	for provinceID, province := range provinces {
		if province.IslandID != NoIslandID {
			continue
		}
		total++
		if bands[provinceID] <= maximumBand {
			matched++
		}
	}
	return float64(matched) / float64(total)
}

func clamp01(value float64) float64 {
	return min(1, max(0, value))
}
