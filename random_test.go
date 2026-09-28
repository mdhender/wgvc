package wgvc

import (
	"testing"
)

func TestSplitMix64ReferenceSequence(t *testing.T) {
	state := uint64(0)
	want := []uint64{0xe220a8397b1dcdaf, 0x6e789e6aa1b965f4}
	for i, wantValue := range want {
		if got := nextSplitMix64(&state); got != wantValue {
			t.Fatalf("value %d = %#x, want %#x", i, got, wantValue)
		}
	}
}

func TestRandomStreamCanBeReconstructed(t *testing.T) {
	first := elevationRandom(42)
	second := elevationRandom(42)
	for i := 0; i < 20; i++ {
		if got, want := first.Uint64(), second.Uint64(); got != want {
			t.Fatalf("value %d = %d, want %d", i, got, want)
		}
	}
}

func TestDerivedStreamsDifferBySeedDomainAndSequence(t *testing.T) {
	const seed = 42
	bySeed := elevationRandom(seed + 1).Uint64()
	byDomain := derivedRandom(seed, elevationDomain^1, 0).Uint64()
	bySequence := derivedRandom(seed, elevationDomain, 1).Uint64()
	base := elevationRandom(seed).Uint64()
	if base == bySeed || base == byDomain || base == bySequence {
		t.Fatalf("streams unexpectedly started with the same value: base %d, seed %d, domain %d, sequence %d", base, bySeed, byDomain, bySequence)
	}
}

func TestClimateStreamsAreIndependent(t *testing.T) {
	const seed = 8675309
	heat := heatRandom(seed)
	moisture := moistureRandom(seed)
	elevation := elevationRandom(seed)
	heatFirst, moistureFirst, elevationFirst := heat.Uint64(), moisture.Uint64(), elevation.Uint64()
	if heatFirst == moistureFirst || heatFirst == elevationFirst || moistureFirst == elevationFirst {
		t.Fatal("climate and elevation streams unexpectedly started with the same value")
	}

	wantMoisture := moistureRandom(seed).Uint64()
	wantElevation := elevationRandom(seed).Uint64()
	for range 1000 {
		heat.Uint64()
	}
	if got := moistureRandom(seed).Uint64(); got != wantMoisture {
		t.Errorf("moisture value = %d after heat consumption, want %d", got, wantMoisture)
	}
	if got := elevationRandom(seed).Uint64(); got != wantElevation {
		t.Errorf("elevation value = %d after climate consumption, want %d", got, wantElevation)
	}
}
