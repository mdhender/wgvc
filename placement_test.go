package wgvc

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestPlanIslandsBuildsShapeInputsBeforePlacement(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 20, IslandCount: 4}
	allocations := []int{8, 6, 3, 3}
	plans, err := planIslands(config, allocations)
	if err != nil {
		t.Fatalf("planIslands() error = %v", err)
	}
	if got, want := len(plans), len(allocations); got != want {
		t.Fatalf("island count = %d, want %d", got, want)
	}

	for islandIndex, plan := range plans {
		if plan.id != IslandID(islandIndex) {
			t.Errorf("plan %d ID = %d", islandIndex, plan.id)
		}
		if got, want := plan.landProvinceCount, allocations[islandIndex]; got != want {
			t.Errorf("island %d requested land count = %d, want %d", plan.id, got, want)
		}
		if got := len(plan.candidates.sites); got <= plan.landProvinceCount {
			t.Errorf("island %d candidate count = %d, want more than requested land count %d", plan.id, got, plan.landProvinceCount)
		}
		if got, want := len(plan.provinceCenters), allocations[islandIndex]; got != want {
			t.Errorf("island %d temporary seed count = %d, want %d", plan.id, got, want)
		}
	}
}

func TestPlaceIslandsScalesLandAndSeparatesCandidateEnvelopes(t *testing.T) {
	config := Config{WorldSeed: 42, ProvinceCount: 20, IslandCount: 4}
	allocations := []int{8, 6, 3, 3}
	plans, meshes := retainedPlacementFixture(t, config, allocations)
	originalMeshes := cloneTessellation(meshes)

	layout, err := placeIslands(config, plans, meshes)
	if err != nil {
		t.Fatalf("placeIslands() error = %v", err)
	}
	if !reflect.DeepEqual(meshes, originalMeshes) {
		t.Fatal("placeIslands() mutated normalized meshes")
	}
	columns := int(math.Ceil(math.Sqrt(float64(len(allocations)))))
	rows := (len(allocations) + columns - 1) / columns
	scales := make([]float64, len(allocations))
	columnWidths := make([]float64, columns)
	rowHeights := make([]float64, rows)
	for islandIndex, allocation := range allocations {
		scales[islandIndex] = math.Sqrt(float64(allocation) / meshArea(meshes.islands[islandIndex]))
		column := islandIndex % columns
		row := islandIndex / columns
		columnWidths[column] = math.Max(columnWidths[column], scales[islandIndex])
		rowHeights[row] = math.Max(rowHeights[row], scales[islandIndex])
	}
	wantColumnCenters := expectedSlotCenters(columnWidths)
	wantRowCenters := expectedSlotCenters(rowHeights)
	random := placementRandom(config.WorldSeed)
	for islandIndex, placed := range layout.islands {
		if placed.id != IslandID(islandIndex) || placed.landProvinceCount != allocations[islandIndex] {
			t.Errorf("placed island %d identity/allocation = %d/%d, want %d/%d", islandIndex, placed.id, placed.landProvinceCount, islandIndex, allocations[islandIndex])
		}
		normalized := meshes.islands[islandIndex]
		wantScale := scales[islandIndex]
		if !near(placed.transform.scale, wantScale) {
			t.Errorf("island %d scale = %.17g, want %.17g", placed.id, placed.transform.scale, wantScale)
		}
		width := placed.envelope.max.X - placed.envelope.min.X
		height := placed.envelope.max.Y - placed.envelope.min.Y
		if !near(width, wantScale) || !near(height, wantScale) {
			t.Errorf("island %d envelope = %g x %g, want candidate dimensions %g x %g", placed.id, width, height, wantScale, wantScale)
		}
		if width <= math.Sqrt(float64(allocations[islandIndex])) {
			t.Errorf("island %d envelope side = %g, want larger than square land footprint %g", placed.id, width, math.Sqrt(float64(allocations[islandIndex])))
		}
		if got, want := meshArea(placed.mesh), float64(allocations[islandIndex]); math.Abs(got-want) > geometryTolerance*want {
			t.Errorf("island %d world land area = %.17g, want %g", placed.id, got, want)
		}
		center := Point{
			X: (placed.envelope.min.X + placed.envelope.max.X) / 2,
			Y: (placed.envelope.min.Y + placed.envelope.max.Y) / 2,
		}
		wantCenter := Point{
			X: wantColumnCenters[islandIndex%columns] + signedJitter(random.Float64()),
			Y: wantRowCenters[islandIndex/columns] + signedJitter(random.Float64()),
		}
		if !near(center.X, wantCenter.X) || !near(center.Y, wantCenter.Y) {
			t.Errorf("island %d envelope center = %+v, want actual-dimension slot center %+v", placed.id, center, wantCenter)
		}
		assertFootprintInsideBounds(t, placed.envelope, layout.bounds)

		for cornerID, corner := range normalized.corners {
			if got, want := placed.mesh.corners[cornerID], placed.transform.point(corner); got != want {
				t.Errorf("island %d corner %d = %+v, want uniform transform %+v", placed.id, cornerID, got, want)
			}
		}
		for cellID, cell := range normalized.cells {
			if got, want := placed.mesh.cells[cellID].center, placed.transform.point(cell.center); got != want {
				t.Errorf("island %d center %d = %+v, want uniform transform %+v", placed.id, cellID, got, want)
			}
			assertPointInConvexPolygon(t, cellID, placed.mesh.cells[cellID].center, meshCellPoints(placed.mesh, cellID))
		}
	}

	for first := range layout.islands {
		for second := first + 1; second < len(layout.islands); second++ {
			if gap := rectangleGap(layout.islands[first].envelope, layout.islands[second].envelope); gap < islandWaterGap-1e-12 {
				t.Errorf("island envelopes %d and %d gap = %g, want at least %g", first, second, gap, islandWaterGap)
			}
		}
	}
}

func TestPlanAndPlaceIslandsAreDeterministicAndSeedSensitive(t *testing.T) {
	allocations := []int{8, 6, 3, 3}
	config := Config{WorldSeed: 1234, ProvinceCount: 20, IslandCount: 4}
	plans, meshes := retainedPlacementFixture(t, config, allocations)
	first, err := placeIslands(config, plans, meshes)
	if err != nil {
		t.Fatalf("first placeIslands() error = %v", err)
	}
	second, err := placeIslands(config, plans, meshes)
	if err != nil {
		t.Fatalf("second placeIslands() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("identical inputs produced different layouts")
	}

	differentConfig := config
	differentConfig.WorldSeed++
	differentPlans, err := planIslands(differentConfig, allocations)
	if err != nil {
		t.Fatalf("seed-changed planIslands() error = %v", err)
	}
	if reflect.DeepEqual(plans, differentPlans) {
		t.Fatal("different world seeds produced identical shape plans")
	}
	differentLayout, err := placeIslands(differentConfig, plans, meshes)
	if err != nil {
		t.Fatalf("seed-changed placeIslands() error = %v", err)
	}
	if reflect.DeepEqual(first, differentLayout) {
		t.Fatal("different world seeds produced identical placement")
	}
}

func TestPlaceIslandsRejectsInvalidRetainedLandAreaWithIslandContext(t *testing.T) {
	config := Config{ProvinceCount: 1, IslandCount: 1}
	plans := []islandPlan{{id: 0, landProvinceCount: 1}}
	valid := mustTessellateIsland(t, 0, []Point{{X: 0.5, Y: 0.5}})

	for _, test := range []struct {
		name   string
		change func(*islandMesh)
	}{
		{name: "non-finite", change: func(mesh *islandMesh) { mesh.corners[0].X = math.NaN() }},
		{name: "zero", change: func(mesh *islandMesh) {
			for cornerID := range mesh.corners {
				mesh.corners[cornerID] = Point{}
			}
		}},
		{name: "negative", change: func(mesh *islandMesh) {
			for cellID := range mesh.cells {
				reverseInts(mesh.cells[cellID].cornerIDs)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			broken := cloneIslandMesh(valid)
			test.change(&broken)
			_, err := placeIslands(config, plans, tessellation{islands: []islandMesh{broken}})
			if err == nil || !strings.Contains(err.Error(), "island 0 retained land area") {
				t.Fatalf("placeIslands() error = %v, want retained-area error with island context", err)
			}
		})
	}
}

func TestPlanAndPlaceIslandsRejectInvalidContracts(t *testing.T) {
	config := Config{ProvinceCount: 5, IslandCount: 2}
	for _, allocations := range [][]int{{5}, {5, 0}, {3, 1}} {
		if _, err := planIslands(config, allocations); err == nil {
			t.Errorf("planIslands(%v) returned no error", allocations)
		}
	}

	plans := []islandPlan{{id: 0, landProvinceCount: 3}, {id: 1, landProvinceCount: 2}}
	if _, err := placeIslands(config, plans, tessellation{}); err == nil {
		t.Error("placeIslands() accepted a missing mesh collection")
	}
}

func retainedPlacementFixture(t *testing.T, config Config, allocations []int) ([]islandPlan, tessellation) {
	t.Helper()
	plans, err := planIslands(config, allocations)
	if err != nil {
		t.Fatalf("planIslands() error = %v", err)
	}
	meshes := tessellation{islands: make([]islandMesh, len(plans))}
	for islandIndex, plan := range plans {
		candidate, err := tessellateIsland(plan.id, plan.candidates.sites)
		if err != nil {
			t.Fatalf("island %d tessellate candidates: %v", plan.id, err)
		}
		selected, err := selectCandidateCells(plan.landProvinceCount, plan.candidates, candidate, plan.shape)
		if err != nil {
			t.Fatalf("island %d select candidates: %v", plan.id, err)
		}
		meshes.islands[islandIndex], err = extractRetainedMesh(candidate, selected)
		if err != nil {
			t.Fatalf("island %d extract retained mesh: %v", plan.id, err)
		}
	}
	return plans, meshes
}

func cloneTessellation(source tessellation) tessellation {
	clone := tessellation{islands: make([]islandMesh, len(source.islands))}
	for islandIndex, mesh := range source.islands {
		clone.islands[islandIndex] = cloneIslandMesh(mesh)
	}
	return clone
}

func reverseInts(values []int) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func expectedSlotCenters(dimensions []float64) []float64 {
	centers := make([]float64, len(dimensions))
	centers[0] = dimensions[0] / 2
	for index := 1; index < len(dimensions); index++ {
		spacing := islandWaterGap + 2*islandJitter
		centers[index] = centers[index-1] + dimensions[index-1]/2 + spacing + dimensions[index]/2
	}
	return centers
}

func assertFootprintInsideBounds(t *testing.T, footprint, bounds rectangle) {
	t.Helper()
	if footprint.min.X-bounds.min.X < islandWorldMargin-1e-12 ||
		footprint.min.Y-bounds.min.Y < islandWorldMargin-1e-12 ||
		bounds.max.X-footprint.max.X < islandWorldMargin-1e-12 ||
		bounds.max.Y-footprint.max.Y < islandWorldMargin-1e-12 {
		t.Errorf("envelope %+v lacks margin inside bounds %+v", footprint, bounds)
	}
}

func rectangleGap(first, second rectangle) float64 {
	dx := math.Max(math.Max(first.min.X-second.max.X, second.min.X-first.max.X), 0)
	dy := math.Max(math.Max(first.min.Y-second.max.Y, second.min.Y-first.max.Y), 0)
	return math.Hypot(dx, dy)
}
