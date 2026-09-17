package wgvc

import "math"

const (
	// Provinces have a typical area of one square world unit, so this spans
	// roughly four typical province widths.
	terrainWavelength = 4.0

	terrainPlainsThreshold = 0.4
	terrainHillsThreshold  = 0.6
)

type valueNoise struct {
	seed uint64
}

func newTerrainNoise(worldSeed uint64) valueNoise {
	return valueNoise{seed: terrainRandom(worldSeed).Uint64()}
}

// sample evaluates seeded two-dimensional value noise. Quintic interpolation
// makes values and their first derivatives continuous at lattice boundaries.
func (noise valueNoise) sample(point Point) float64 {
	x := point.X / terrainWavelength
	y := point.Y / terrainWavelength
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

func assignTerrain(world *World, worldSeed uint64) {
	noise := newTerrainNoise(worldSeed)
	cornerValues := make([]float64, len(world.Corners))
	for cornerID, corner := range world.Corners {
		cornerValues[cornerID] = noise.sample(corner.Point)
	}
	for provinceID := range world.Provinces {
		if world.Provinces[provinceID].Terrain == TerrainWater {
			continue
		}
		world.Provinces[provinceID].Terrain = terrainFromCorners(world.Provinces[provinceID], cornerValues)
	}
}

func assignEdgeElevations(world *World) {
	for edgeID := range world.Edges {
		edge := &world.Edges[edgeID]
		edge.Elevation = 0
		if len(edge.ProvinceIDs) != 2 {
			continue
		}
		first := world.Provinces[edge.ProvinceIDs[0]]
		second := world.Provinces[edge.ProvinceIDs[1]]
		if first.Terrain != TerrainWater && second.Terrain != TerrainWater {
			edge.Elevation = 0.1
		}
	}
}

func terrainFromCorners(province Province, cornerValues []float64) Terrain {
	total := 0.0
	for _, cornerID := range province.CornerIDs {
		total += cornerValues[cornerID]
	}
	return classifyTerrain(total / float64(len(province.CornerIDs)))
}

func classifyTerrain(elevation float64) Terrain {
	switch {
	case elevation < terrainPlainsThreshold:
		return TerrainPlains
	case elevation < terrainHillsThreshold:
		return TerrainHills
	default:
		return TerrainMountains
	}
}
