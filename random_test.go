package wgvc

import "testing"

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
	first := provinceRandom(42, 7)
	second := provinceRandom(42, 7)
	for i := 0; i < 20; i++ {
		if got, want := first.Uint64(), second.Uint64(); got != want {
			t.Fatalf("value %d = %d, want %d", i, got, want)
		}
	}
}

func TestTerrainConsumptionDoesNotChangePlacement(t *testing.T) {
	const seed = 8675309
	want := placementRandom(seed)
	got := placementRandom(seed)
	terrain := terrainRandom(seed)

	for range 1000 {
		terrain.Uint64()
	}
	for i := 0; i < 20; i++ {
		if gotValue, wantValue := got.Uint64(), want.Uint64(); gotValue != wantValue {
			t.Fatalf("placement value %d = %d after terrain consumption, want %d", i, gotValue, wantValue)
		}
	}
}

func TestIslandProvinceStreamsAreIndependent(t *testing.T) {
	firstIsland := provinceRandom(42, 0)
	secondIsland := provinceRandom(42, 1)
	if firstIsland.Uint64() == secondIsland.Uint64() {
		t.Fatal("different islands unexpectedly started with the same random value")
	}
}
