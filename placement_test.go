package wgvc

import (
	"math"
	"reflect"
	"testing"
)

func TestPlanIslandsProducesSeparatedProportionalFootprints(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 20, IslandCount: 4}
	allocations := []int{8, 6, 3, 3}
	layout, err := planIslands(config, allocations)
	if err != nil {
		t.Fatalf("planIslands() error = %v", err)
	}
	if got, want := len(layout.islands), len(allocations); got != want {
		t.Fatalf("island count = %d, want %d", got, want)
	}

	for islandIndex, plan := range layout.islands {
		if plan.id != IslandID(islandIndex) {
			t.Errorf("plan %d ID = %d", islandIndex, plan.id)
		}
		if got, want := len(plan.provinceCenters), allocations[islandIndex]; got != want {
			t.Errorf("island %d seed count = %d, want %d", plan.id, got, want)
		}
		width := plan.footprint.max.X - plan.footprint.min.X
		height := plan.footprint.max.Y - plan.footprint.min.Y
		if !near(width, height) {
			t.Errorf("island %d footprint is not square: %g x %g", plan.id, width, height)
		}
		if got, want := width*height, float64(allocations[islandIndex]); !near(got, want) {
			t.Errorf("island %d footprint area = %g, want %g", plan.id, got, want)
		}
		assertFootprintInsideBounds(t, plan.footprint, layout.bounds)
		for seedIndex, seed := range plan.provinceCenters {
			worldSeed := plan.footprint.pointAt(seed)
			if !strictlyInside(worldSeed, plan.footprint) {
				t.Errorf("island %d seed %d maps outside footprint: %+v", plan.id, seedIndex, worldSeed)
			}
		}
	}

	for first := range layout.islands {
		for second := first + 1; second < len(layout.islands); second++ {
			if gap := rectangleGap(layout.islands[first].footprint, layout.islands[second].footprint); gap < islandWaterGap-1e-12 {
				t.Errorf("islands %d and %d gap = %g, want at least %g", first, second, gap, islandWaterGap)
			}
		}
	}
}

func TestPlanIslandsIsDeterministicAndSeedSensitive(t *testing.T) {
	allocations := []int{8, 6, 3, 3}
	config := Config{WorldSeed: 1234, ProvinceCount: 20, IslandCount: 4}
	first, err := planIslands(config, allocations)
	if err != nil {
		t.Fatalf("first planIslands() error = %v", err)
	}
	second, err := planIslands(config, allocations)
	if err != nil {
		t.Fatalf("second planIslands() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("identical inputs produced different layouts")
	}

	config.WorldSeed++
	different, err := planIslands(config, allocations)
	if err != nil {
		t.Fatalf("seed-changed planIslands() error = %v", err)
	}
	if reflect.DeepEqual(first, different) {
		t.Fatal("different world seeds produced identical layouts")
	}
}

func TestPlanIslandsRejectsInvalidAllocation(t *testing.T) {
	config := Config{ProvinceCount: 5, IslandCount: 2}
	for _, allocations := range [][]int{{5}, {5, 0}, {3, 1}} {
		if _, err := planIslands(config, allocations); err == nil {
			t.Errorf("planIslands(%v) returned no error", allocations)
		}
	}
}

func assertFootprintInsideBounds(t *testing.T, footprint, bounds rectangle) {
	t.Helper()
	if footprint.min.X-bounds.min.X < islandWorldMargin-1e-12 ||
		footprint.min.Y-bounds.min.Y < islandWorldMargin-1e-12 ||
		bounds.max.X-footprint.max.X < islandWorldMargin-1e-12 ||
		bounds.max.Y-footprint.max.Y < islandWorldMargin-1e-12 {
		t.Errorf("footprint %+v lacks margin inside bounds %+v", footprint, bounds)
	}
}

func strictlyInside(point Point, bounds rectangle) bool {
	return point.X > bounds.min.X && point.X < bounds.max.X &&
		point.Y > bounds.min.Y && point.Y < bounds.max.Y
}

func rectangleGap(first, second rectangle) float64 {
	dx := math.Max(math.Max(first.min.X-second.max.X, second.min.X-first.max.X), 0)
	dy := math.Max(math.Max(first.min.Y-second.max.Y, second.min.Y-first.max.Y), 0)
	return math.Hypot(dx, dy)
}
