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

// heatCalibration is one point of the calibration grid. values and bands are
// filled only for the calibration that is returned; candidates carry just the
// parameters and their scores.
type heatCalibration struct {
	values          []float64
	bands           []int
	warmthScale     float64
	coolingStrength float64
	error           float64
	polarReached    bool
	peakReached     bool
}

// heatScratch holds the per-province work arrays one calibration reuses
// across every grid candidate, so evaluating a candidate allocates nothing.
type heatScratch struct {
	values      []float64
	order       []int
	inPeakSlice []bool
}

func newHeatScratch(provinceCount int, peakSlice []int) *heatScratch {
	scratch := &heatScratch{
		values:      make([]float64, provinceCount),
		order:       make([]int, provinceCount),
		inPeakSlice: make([]bool, provinceCount),
	}
	for _, provinceID := range peakSlice {
		scratch.inPeakSlice[provinceID] = true
	}
	return scratch
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
	if !classifiable || oceanCount == 0 {
		return middleHeatCalibration(provinces, heatSource, latitudes, (minimumWarmthScale+maximumWarmthScale)/2, maximumCoolingStrength/2)
	}

	// A world whose northernmost land decile holds no top-decile elevation
	// has no peak slice to calibrate against. Its heat bands are still well
	// defined, so calibration then serves the polar target alone.
	polarTolerance := max(climateTargetTolerance, 0.5/float64(oceanCount))
	peakTolerance := math.Inf(1)
	if len(peakSlice) > 0 {
		peakTolerance = max(climateTargetTolerance, 0.5/float64(len(peakSlice)))
	}
	warmthLow, warmthHigh := minimumWarmthScale, maximumWarmthScale
	coolingLow, coolingHigh := 0.0, maximumCoolingStrength
	scratch := newHeatScratch(len(provinces), peakSlice)
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
			warmth := warmthLow + float64(float64(warmthIndex)*warmthStep)
			for coolingIndex := range gridSize {
				cooling := coolingLow + float64(float64(coolingIndex)*coolingStep)
				candidate := evaluateHeatCandidate(provinces, heatSource, latitudes, oceanCount, len(peakSlice), counts, config, warmth, cooling, polarTolerance, peakTolerance, scratch)
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
	// cannot be banded at all. Only the winner's values and bands are built.
	best.values = adjustedHeatValues(provinces, heatSource, latitudes, best.warmthScale, best.coolingStrength)
	best.bands = classifyClimatePopulationWithCounts(provinces, best.values, counts)
	return best
}

// evaluateHeatCandidate scores one grid point. The score needs only two
// facts about the band classification: how many ocean provinces the sort
// would place in the polar band, and how many peak-slice provinces it would
// place in the polar or cold bands. Both are prefix counts of the sorted
// order, so a selection finds them in linear time without sorting, using the
// same total order the final classification sorts by.
func evaluateHeatCandidate(provinces []Province, heatSource, latitudes []float64, oceanCount, peakCount int, counts [5]int, config ClimateConfig, warmth, cooling, polarTolerance, peakTolerance float64, scratch *heatScratch) heatCalibration {
	values := scratch.values
	adjustHeatValues(values, provinces, heatSource, latitudes, warmth, cooling)
	oceanPolar, peakCold := heatBandPrefixCounts(provinces, values, scratch, counts[0], counts[0]+counts[1])
	polarFraction := float64(oceanPolar) / float64(oceanCount)
	peakError := 0.0
	if peakCount > 0 {
		peakFraction := float64(peakCold) / float64(peakCount)
		peakError = math.Abs(peakFraction - config.PeakChill)
	}
	return heatCalibration{
		warmthScale:     warmth,
		coolingStrength: cooling,
		error:           math.Abs(polarFraction-config.PolarIce) + peakError,
		polarReached:    math.Abs(polarFraction-config.PolarIce) <= polarTolerance,
		peakReached:     peakError <= peakTolerance,
	}
}

// heatBandPrefixCounts partitions scratch.order so that its first polarCount
// entries are the provinces classifyClimatePopulationWithCounts would place
// in band 0 and its first coldCount entries those it would place in bands 0
// and 1, then counts the ocean provinces in the first prefix and the
// peak-slice provinces in the second. Both counts must be at least 1 and
// coldCount at most len(provinces), which climatePopulationCounts guarantees
// whenever it reports the population classifiable.
func heatBandPrefixCounts(provinces []Province, values []float64, scratch *heatScratch, polarCount, coldCount int) (oceanPolar, peakCold int) {
	order := scratch.order
	for provinceID := range order {
		order[provinceID] = provinceID
	}
	selectHeatOrder(provinces, values, order, polarCount-1)
	selectHeatOrder(provinces, values, order[polarCount:], coldCount-polarCount-1)
	for _, provinceID := range order[:polarCount] {
		if provinces[provinceID].IslandID == NoIslandID {
			oceanPolar++
		}
	}
	for _, provinceID := range order[:coldCount] {
		if scratch.inPeakSlice[provinceID] {
			peakCold++
		}
	}
	return oceanPolar, peakCold
}

// heatOrderLess is the total order the heat classification sorts by:
// ascending value, ties by island and then province ID. No two provinces
// compare equal, so every selection prefix is uniquely determined.
func heatOrderLess(provinces []Province, values []float64, first, second int) bool {
	if values[first] != values[second] {
		return values[first] < values[second]
	}
	return climateTieLess(&provinces[first], &provinces[second])
}

// selectHeatOrder rearranges order so that order[k] is the element that
// would sit at position k if order were sorted by heatOrderLess, every
// element before it orders lower, and every element after it orders higher.
// It is a quickselect with median-of-three pivots; the choice of pivot only
// affects speed, never the result, because the order is strict.
func selectHeatOrder(provinces []Province, values []float64, order []int, k int) {
	low, high := 0, len(order)-1
	for low < high {
		middle := low + (high-low)/2
		// Median of three, moved to the end as the pivot.
		if heatOrderLess(provinces, values, order[middle], order[low]) {
			order[middle], order[low] = order[low], order[middle]
		}
		if heatOrderLess(provinces, values, order[high], order[low]) {
			order[high], order[low] = order[low], order[high]
		}
		if heatOrderLess(provinces, values, order[middle], order[high]) {
			order[middle], order[high] = order[high], order[middle]
		}
		pivot := order[high]
		store := low
		for index := low; index < high; index++ {
			if heatOrderLess(provinces, values, order[index], pivot) {
				order[store], order[index] = order[index], order[store]
				store++
			}
		}
		order[store], order[high] = order[high], order[store]
		switch {
		case k < store:
			high = store - 1
		case k > store:
			low = store + 1
		default:
			return
		}
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
	adjustHeatValues(values, provinces, heatSource, latitudes, warmth, cooling)
	return values
}

// adjustHeatValues is adjustedHeatValues writing into a caller-owned slice.
func adjustHeatValues(values []float64, provinces []Province, heatSource, latitudes []float64, warmth, cooling float64) {
	for provinceID := range provinces {
		province := &provinces[provinceID]
		// Explicit conversions round each product separately so arm64
		// cannot fuse them into multiply-adds and diverge from amd64.
		heat := float64((1-warmth)*heatSource[provinceID]) + float64(warmth*(1-latitudes[provinceID]))
		if province.IslandID != NoIslandID {
			heat *= 1 - float64(cooling*max(0, province.Elevation))
		}
		values[provinceID] = heat
	}
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
		return climateTieLess(&provinces[first], &provinces[second])
	})
	byLatitude := append([]int(nil), land...)
	sort.Slice(byLatitude, func(i, j int) bool {
		first, second := byLatitude[i], byLatitude[j]
		if latitudes[first] != latitudes[second] {
			return latitudes[first] < latitudes[second]
		}
		return climateTieLess(&provinces[first], &provinces[second])
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
		return heatOrderLess(provinces, values, first, second)
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

func climateTieLess(first, second *Province) bool {
	if first.IslandID != second.IslandID {
		return first.IslandID < second.IslandID
	}
	return first.ID < second.ID
}

func clamp01(value float64) float64 {
	return min(1, max(0, value))
}
