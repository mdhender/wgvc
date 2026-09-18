package wgvc

import (
	"reflect"
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
	first := candidateRandom(42, 7)
	second := candidateRandom(42, 7)
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

func TestClimateStreamsAreIndependent(t *testing.T) {
	const seed = 8675309
	heat := heatRandom(seed)
	moisture := moistureRandom(seed)
	terrain := terrainRandom(seed)
	heatFirst, moistureFirst, terrainFirst := heat.Uint64(), moisture.Uint64(), terrain.Uint64()
	if heatFirst == moistureFirst || heatFirst == terrainFirst || moistureFirst == terrainFirst {
		t.Fatal("climate and terrain streams unexpectedly started with the same value")
	}

	wantMoisture := moistureRandom(seed).Uint64()
	wantTerrain := terrainRandom(seed).Uint64()
	for range 1000 {
		heat.Uint64()
	}
	if got := moistureRandom(seed).Uint64(); got != wantMoisture {
		t.Errorf("moisture value = %d after heat consumption, want %d", got, wantMoisture)
	}
	if got := terrainRandom(seed).Uint64(); got != wantTerrain {
		t.Errorf("terrain value = %d after climate consumption, want %d", got, wantTerrain)
	}
}

func TestIslandCandidateStreamsAreIndependent(t *testing.T) {
	firstIsland := candidateRandom(42, 0)
	secondIsland := candidateRandom(42, 1)
	if firstIsland.Uint64() == secondIsland.Uint64() {
		t.Fatal("different islands unexpectedly started with the same random value")
	}
}

func TestShapeConsumptionCannotPerturbOtherStreams(t *testing.T) {
	const seed = 99
	wantPlacement := placementRandom(seed).Uint64()
	wantCandidates, err := planCandidateSites(53, candidateRandom(seed, 2))
	if err != nil {
		t.Fatalf("planCandidateSites() error = %v", err)
	}
	wantTerrain := terrainRandom(seed).Uint64()
	wantOtherIsland := blobShapeRandom(seed, 3).Uint64()

	shape := blobShapeRandom(seed, 2)
	for range 1000 {
		shape.Uint64()
	}

	if got := placementRandom(seed).Uint64(); got != wantPlacement {
		t.Errorf("placement value = %d after shape consumption, want %d", got, wantPlacement)
	}
	gotCandidates, err := planCandidateSites(53, candidateRandom(seed, 2))
	if err != nil {
		t.Fatalf("planCandidateSites() after shape consumption error = %v", err)
	}
	if !reflect.DeepEqual(gotCandidates, wantCandidates) {
		t.Error("candidate sites changed after shape consumption")
	}
	if got := terrainRandom(seed).Uint64(); got != wantTerrain {
		t.Errorf("terrain value = %d after shape consumption, want %d", got, wantTerrain)
	}
	if got := blobShapeRandom(seed, 3).Uint64(); got != wantOtherIsland {
		t.Errorf("island 3 shape value = %d after island 2 shape consumption, want %d", got, wantOtherIsland)
	}
}

func TestCandidateAndShapeDomainsAndIslandSequencesDiffer(t *testing.T) {
	const seed = 42
	candidate := candidateRandom(seed, 0).Uint64()
	shape := blobShapeRandom(seed, 0).Uint64()
	otherShape := blobShapeRandom(seed, 1).Uint64()
	if candidate == shape {
		t.Fatal("candidate and shape domains unexpectedly started with the same value")
	}
	if shape == otherShape {
		t.Fatal("different island shape sequences unexpectedly started with the same value")
	}
}

func TestShapeParametersDoNotDependOnAllocation(t *testing.T) {
	const seed = 42
	want := generateBlobShape(blobShapeRandom(seed, 3))
	for _, allocation := range []int{1, 2, 53, 128} {
		if _, err := planCandidateSites(allocation, candidateRandom(seed, 3)); err != nil {
			t.Fatalf("planCandidateSites(%d) error = %v", allocation, err)
		}
		if got := generateBlobShape(blobShapeRandom(seed, 3)); got != want {
			t.Errorf("shape after planning %d candidates = %+v, want %+v", allocation, got, want)
		}
	}
}
