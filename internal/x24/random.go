package x24

import "math/rand/v2"

const (
	roundDomain    uint64 = 0x783234726f756e64 // "x24round"
	splitMixGamma  uint64 = 0x9e3779b97f4a7c15
	sequenceStride uint64 = 0xd1b54a32d192ed03
)

func roundRandom(worldSeed uint64, round int) *rand.Rand {
	state := worldSeed ^ roundDomain
	state += uint64(round) * sequenceStride
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
