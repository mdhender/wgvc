package wgvc

import "math/rand/v2"

// Each generation concern has a fixed domain. Domain-separated SplitMix64
// derivation gives every stage a reproducible local PCG stream, so consuming
// values in one stage cannot perturb another stage.
const (
	placementDomain uint64 = 0x706c6163656d656e // "placemen"
	candidateDomain uint64 = 0x63616e6469646174 // "candidat"
	blobShapeDomain uint64 = 0x626c6f6273686170 // "blobshap"
	elevationDomain uint64 = 0x7465727261696e00 // legacy "terrain\x00" value
	heatDomain      uint64 = 0x6865617400000000 // "heat\x00\x00\x00\x00"
	moistureDomain  uint64 = 0x6d6f697374757265 // "moisture"
	splitMixGamma   uint64 = 0x9e3779b97f4a7c15
	sequenceStride  uint64 = 0xd1b54a32d192ed03
)

func placementRandom(worldSeed uint64) *rand.Rand {
	return derivedRandom(worldSeed, placementDomain, 0)
}

func candidateRandom(worldSeed uint64, islandID IslandID) *rand.Rand {
	return derivedRandom(worldSeed, candidateDomain, uint64(islandID))
}

func blobShapeRandom(worldSeed uint64, islandID IslandID) *rand.Rand {
	return derivedRandom(worldSeed, blobShapeDomain, uint64(islandID))
}

func elevationRandom(worldSeed uint64) *rand.Rand {
	return derivedRandom(worldSeed, elevationDomain, 0)
}

func heatRandom(worldSeed uint64) *rand.Rand {
	return derivedRandom(worldSeed, heatDomain, 0)
}

func moistureRandom(worldSeed uint64) *rand.Rand {
	return derivedRandom(worldSeed, moistureDomain, 0)
}

func derivedRandom(worldSeed, domain, sequence uint64) *rand.Rand {
	state := worldSeed ^ domain
	state += sequence * sequenceStride
	first := nextSplitMix64(&state)
	second := nextSplitMix64(&state)
	return rand.New(rand.NewPCG(first, second))
}

func nextSplitMix64(state *uint64) uint64 {
	*state += splitMixGamma
	value := *state
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}
