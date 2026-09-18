package wgvc

import "math"

const (
	// Provinces have a typical area of one square world unit, so this spans
	// roughly four typical province widths.
	elevationWavelength = 4.0

	elevationWaterMargin = 0.1
	elevationLandMargin  = 0.1
	elevationLowlandMax  = 0.2
	elevationUplandMax   = 0.4
	elevationHighlandMax = 0.6

	terrainOceanMax          = -0.1
	terrainCoastMax          = 0.04
	terrainWetlandReliefMax  = 0.34
	terrainMountainReliefMin = 0.42
	terrainHillReliefMin     = 0.38
	terrainBadlandsReliefMin = 0.55
)

var terrainCover = [5][5]Terrain{
	{TerrainTundra, TerrainTundra, TerrainTundra, TerrainTundra, TerrainTundra},
	{TerrainTundra, TerrainSteppe, TerrainBorealForest, TerrainBorealForest, TerrainBorealForest},
	{TerrainScrubland, TerrainGrassland, TerrainPlains, TerrainTemperateForest, TerrainTemperateForest},
	{TerrainDesert, TerrainScrubland, TerrainSavanna, TerrainTemperateForest, TerrainTemperateForest},
	{TerrainDesert, TerrainScrubland, TerrainSavanna, TerrainRainforest, TerrainRainforest},
}

type valueNoise struct {
	seed uint64
}

func newElevationNoise(worldSeed uint64) valueNoise {
	return valueNoise{seed: elevationRandom(worldSeed).Uint64()}
}

// sample evaluates seeded two-dimensional value noise. Quintic interpolation
// makes values and their first derivatives continuous at lattice boundaries.
func (noise valueNoise) sample(point Point) float64 {
	x := point.X / elevationWavelength
	y := point.Y / elevationWavelength
	x0 := int64(math.Floor(x))
	y0 := int64(math.Floor(y))
	tx := smoothQuintic(x - float64(x0))
	ty := smoothQuintic(y - float64(y0))

	bottom := interpolate(noise.latticeValue(x0, y0), noise.latticeValue(x0+1, y0), tx)
	top := interpolate(noise.latticeValue(x0, y0+1), noise.latticeValue(x0+1, y0+1), tx)
	return interpolate(bottom, top, ty)
}

func (noise valueNoise) latticeValue(x, y int64) float64 {
	state := noise.seed ^ uint64(x)*0x9e3779b97f4a7c15 ^ uint64(y)*0xd1b54a32d192ed03
	value := nextSplitMix64(&state)
	return float64(value>>11) / (1 << 53)
}

func smoothQuintic(value float64) float64 {
	return value * value * value * (value*(value*6-15) + 10)
}

func interpolate(first, second, fraction float64) float64 {
	return first + fraction*(second-first)
}

func assignElevations(world *World, worldSeed uint64) {
	noise := newElevationNoise(worldSeed)
	cornerValues := make([]float64, len(world.Corners))
	for cornerID, corner := range world.Corners {
		cornerValues[cornerID] = noise.sample(corner.Point)
	}
	assignEdgeElevations(world, cornerValues)
	assignProvinceElevations(world)
}

func assignEdgeElevations(world *World, cornerValues []float64) {
	for edgeID := range world.Edges {
		edge := &world.Edges[edgeID]
		edge.Elevation = 0
		if len(edge.ProvinceIDs) != 2 {
			continue
		}
		firstLand := world.Provinces[edge.ProvinceIDs[0]].IslandID != NoIslandID
		secondLand := world.Provinces[edge.ProvinceIDs[1]].IslandID != NoIslandID
		if firstLand != secondLand {
			continue
		}
		edgeNoise := (cornerValues[edge.CornerIDs[0]] + cornerValues[edge.CornerIDs[1]]) / 2
		if firstLand {
			edge.Elevation = elevationLandMargin + (1-elevationLandMargin)*edgeNoise
		} else {
			edge.Elevation = -1 + (1-elevationWaterMargin)*edgeNoise
		}
	}
}

func assignProvinceElevations(world *World) {
	totals := make([]float64, len(world.Provinces))
	counts := make([]int, len(world.Provinces))
	for _, edge := range world.Edges {
		for _, provinceID := range edge.ProvinceIDs {
			totals[provinceID] += edge.Elevation
			counts[provinceID]++
		}
	}
	for provinceID := range world.Provinces {
		province := &world.Provinces[provinceID]
		province.Elevation = totals[provinceID] / float64(counts[provinceID])
		province.ElevationBand = classifyElevation(province.IslandID != NoIslandID, province.Elevation)
	}
}

func classifyElevation(land bool, elevation float64) ElevationBand {
	if !land {
		if elevation < -0.5 {
			return ElevationBandDeepWater
		}
		return ElevationBandShallowWater
	}
	switch {
	case elevation < elevationLowlandMax:
		return ElevationBandLowland
	case elevation < elevationUplandMax:
		return ElevationBandUpland
	case elevation < elevationHighlandMax:
		return ElevationBandHighland
	default:
		return ElevationBandMountain
	}
}

func assignTerrain(world *World) {
	adjacentLand := make([]bool, len(world.Provinces))
	adjacentWater := make([]bool, len(world.Provinces))
	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) != 2 {
			continue
		}
		first, second := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
		firstLand := world.Provinces[first].IslandID != NoIslandID
		secondLand := world.Provinces[second].IslandID != NoIslandID
		if firstLand == secondLand {
			continue
		}
		if firstLand {
			adjacentWater[first] = true
			adjacentLand[second] = true
		} else {
			adjacentLand[first] = true
			adjacentWater[second] = true
		}
	}
	for provinceID := range world.Provinces {
		world.Provinces[provinceID].Terrain = classifyTerrain(world.Provinces[provinceID], adjacentLand[provinceID], adjacentWater[provinceID])
	}
}

func classifyTerrain(province Province, adjacentLand, adjacentWater bool) Terrain {
	if province.IslandID == NoIslandID {
		switch {
		case adjacentLand:
			return TerrainCoastalWater
		case province.ElevationBand == ElevationBandDeepWater:
			return TerrainDeepOcean
		case province.Elevation <= terrainOceanMax:
			return TerrainOcean
		default:
			return TerrainShallowSea
		}
	}

	if province.HeatBand == HeatBandPolar && province.MoistureBand > MoistureBandArid {
		return TerrainGlacialIce
	}
	if province.ElevationBand == ElevationBandMountain {
		if province.HeatBand <= HeatBandCold {
			return TerrainAlpine
		}
		return TerrainMountain
	}
	if province.ElevationBand == ElevationBandHighland {
		if province.Relief >= terrainMountainReliefMin {
			return TerrainMountain
		}
		return TerrainHills
	}
	if province.ElevationBand == ElevationBandLowland &&
		province.Relief <= terrainWetlandReliefMax &&
		province.MoistureBand >= MoistureBandHumid {
		switch province.HeatBand {
		case HeatBandPolar, HeatBandCold:
			return TerrainBog
		case HeatBandTemperate:
			return TerrainMarsh
		default:
			return TerrainSwamp
		}
	}
	if province.ElevationBand == ElevationBandLowland && province.Elevation <= terrainCoastMax && adjacentWater {
		return TerrainCoast
	}
	if province.MoistureBand <= MoistureBandDry && province.Relief >= terrainBadlandsReliefMin {
		return TerrainBadlands
	}
	if province.ElevationBand == ElevationBandUpland && province.Relief >= terrainHillReliefMin {
		return TerrainHills
	}
	if province.HeatBand < HeatBandPolar || province.HeatBand > HeatBandHot ||
		province.MoistureBand < MoistureBandArid || province.MoistureBand > MoistureBandSaturated {
		return TerrainPlains
	}
	return terrainCover[province.HeatBand][province.MoistureBand]
}
