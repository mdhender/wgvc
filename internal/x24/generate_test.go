package x24

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/mdhender/wgvc/internal/aspectratio"
	"github.com/mdhender/wgvc/internal/singlemesh"
)

func TestDefaultConfig(t *testing.T) {
	got := DefaultConfig()
	if got.WorldSeed != 0x0123456789abcdef || got.ProvinceCount != 1_500 || got.IslandCount != 15 || got.AspectRatio != "1:1" || got.OceanPercentage != 0.68 {
		t.Fatalf("DefaultConfig() required values = %+v", got)
	}
	if got.EdgeBarrierWidth != 0.02 || !reflect.DeepEqual(got.EdgeRamp, []float64{-1, -0.65, -0.40, -0.22, -0.10, -0.04, 0}) || got.ControlPenalty != -0.82 {
		t.Fatalf("DefaultConfig() issue #25 values = %+v", got)
	}
	if got.AttractantCount != 0 || !reflect.DeepEqual(got.AttractantRamp, []float64{1, 0.78, 0.58, 0.42, 0.29, 0.18, 0.10, 0.04, 0.01, 0}) || got.AttractantJitter != 0.65 {
		t.Fatalf("DefaultConfig() issue #26 values = %+v", got)
	}
	if err := got.validate(); err != nil {
		t.Fatalf("DefaultConfig() is invalid: %v", err)
	}
}

func TestGenerateDefaultIsDeterministicAndValid(t *testing.T) {
	config := DefaultConfig()
	first, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	second, err := Generate(config)
	if err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("identical configurations produced different results")
	}
	assertValidResult(t, first, config)
	if first.RoundsAttempted != 1 || first.FinalOcean != config.OceanPercentage {
		t.Errorf("default converged in %d rounds at ocean %g, want one round at %g", first.RoundsAttempted, first.FinalOcean, config.OceanPercentage)
	}
}

func TestClaimMergesOnlyWithDirectlyAdjacentLand(t *testing.T) {
	cells := lineCells(4)
	state := newGrowthState(cells, []float64{0, 0, 0, 0}, allEligible(4), -0.82)
	state.islands = []islandState{{seedID: 0, active: true}, {seedID: 1, active: true}}
	state.frontiers = make([]randomSet, 2)

	if losers := state.claim(0, 0); len(losers) != 0 {
		t.Fatalf("first claim absorbed islands %v", losers)
	}
	if losers := state.claim(1, 1); !reflect.DeepEqual(losers, []int{0}) {
		t.Fatalf("adjacent controlled claim absorbed %v, want [0]", losers)
	}
	if state.islands[0].active || state.owners[0] != 1 || state.controllers[1] != Water {
		t.Fatalf("merge did not transfer loser state: islands=%+v owners=%v controllers=%v", state.islands, state.owners, state.controllers)
	}

	state = newGrowthState(cells, []float64{0, 0, 0, 0}, allEligible(4), -0.82)
	state.islands = []islandState{{seedID: 0, active: true}, {seedID: 2, active: true}}
	state.frontiers = make([]randomSet, 2)
	state.claim(0, 0)
	state.controllers[2] = 0 // Simulate a future multi-hop control ripple.
	if losers := state.claim(2, 1); len(losers) != 0 {
		t.Fatalf("non-adjacent controlled claim absorbed islands %v", losers)
	}
	if !state.islands[0].active || state.mergeCount != 0 {
		t.Fatalf("non-adjacent control caused a merge: islands=%+v merges=%d", state.islands, state.mergeCount)
	}
}

func TestClaimAbsorbsEveryAdjacentIslandRegardlessOfController(t *testing.T) {
	cells := []singlemesh.Cell{
		{ID: 0, Neighbors: []int{2}},
		{ID: 1, Neighbors: []int{2}},
		{ID: 2, Neighbors: []int{0, 1}},
	}
	state := newGrowthState(cells, []float64{0, 0, 0}, allEligible(3), -0.82)
	state.islands = []islandState{{seedID: 0, active: true}, {seedID: 1, active: true}, {seedID: 2, active: true}}
	state.frontiers = make([]randomSet, 3)
	state.claim(0, 0)
	state.claim(1, 1)
	state.controllers[2] = 0

	if losers := state.claim(2, 2); !reflect.DeepEqual(losers, []int{0, 1}) {
		t.Fatalf("claim absorbed islands %v, want [0 1]", losers)
	}
	if state.islands[0].active || state.islands[1].active || state.mergeCount != 2 {
		t.Fatalf("adjacent islands were not absorbed: islands=%+v merges=%d", state.islands, state.mergeCount)
	}
}

func TestVisibleValueUsesHonestFieldForController(t *testing.T) {
	state := newGrowthState(lineCells(2), []float64{0.75, 0.50}, allEligible(2), -0.82)
	state.controllers[1] = 0
	if got := state.visibleValue(1, 0); got != 0.50 {
		t.Errorf("controller sees %g, want honest field 0.5", got)
	}
	if got := state.visibleValue(1, 1); got != -0.82 {
		t.Errorf("rival sees %g, want control penalty -0.82", got)
	}
}

func TestEdgeFieldCreatesBarrierAndConfiguredRamp(t *testing.T) {
	cells := []singlemesh.Cell{
		{ID: 0, Corners: []Point{{X: 0, Y: 0.4}}, Neighbors: []int{1}},
		{ID: 1, Corners: []Point{{X: 0.2, Y: 0.4}}, Neighbors: []int{0, 2}},
		{ID: 2, Corners: []Point{{X: 0.4, Y: 0.4}}, Neighbors: []int{1, 3}},
		{ID: 3, Corners: []Point{{X: 0.6, Y: 0.4}}, Neighbors: []int{2, 4}},
		{ID: 4, Corners: []Point{{X: 0.8, Y: 0.4}}, Neighbors: []int{3}},
	}
	values, eligible, _ := edgeField(cells, singlemesh.Bounds{Width: 1, Height: 1}, 0.01, []float64{-1, -0.5, 0})
	if !reflect.DeepEqual(values, []float64{-1, -1, -0.5, 0, 0}) {
		t.Fatalf("edge values = %v, want [-1 -1 -0.5 0 0]", values)
	}
	if !reflect.DeepEqual(eligible, []bool{false, true, true, true, true}) {
		t.Fatalf("land eligibility = %v", eligible)
	}
}

func TestEdgeFieldKeepsHarsherValueFromMultiplePaths(t *testing.T) {
	cells := []singlemesh.Cell{
		{ID: 0, Corners: []Point{{X: 0, Y: 0.2}}, Neighbors: []int{3}},
		{ID: 1, Corners: []Point{{X: 0, Y: 0.8}}, Neighbors: []int{2}},
		{ID: 2, Corners: []Point{{X: 0.3, Y: 0.8}}, Neighbors: []int{1, 3}},
		{ID: 3, Corners: []Point{{X: 0.3, Y: 0.3}}, Neighbors: []int{0, 2}},
	}
	values, _, _ := edgeField(cells, singlemesh.Bounds{Width: 1, Height: 1}, 0.01, []float64{-1, -0.4, 0})
	if values[3] != -1 {
		t.Fatalf("cell reached in one and two hops has value %g, want harsher one-hop value -1", values[3])
	}
}

func TestMakeAttractantsUsesDiePipRegions(t *testing.T) {
	cells := regionalCells()
	edgeDistances := make([]int, len(cells))
	for i := range edgeDistances {
		edgeDistances[i] = 20
	}
	tests := []struct {
		count   int
		regions []int
	}{
		{0, nil},
		{1, []int{4}},
		{2, []int{0, 8}},
		{3, []int{0, 4, 8}},
		{4, []int{0, 2, 6, 8}},
		{5, []int{0, 2, 4, 6, 8}},
		{6, []int{0, 2, 3, 5, 6, 8}},
		{9, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
	}
	for _, test := range tests {
		attractants, skips := makeAttractants(cells, singlemesh.Bounds{Width: 1, Height: 1}, edgeDistances, []float64{-1, 0}, []float64{1, 0}, test.count, 0.65, roundRandom(1, 0))
		if len(attractants) != len(test.regions) || len(skips) != 0 {
			t.Fatalf("count %d: attractants=%d skips=%+v, want %d and none", test.count, len(attractants), skips, len(test.regions))
		}
		for i, attractant := range attractants {
			regionID := test.regions[i]
			wantX, wantY := regionID%3, regionID/3
			if attractant.RegionX != wantX || attractant.RegionY != wantY || attractant.CellID != regionID || attractant.Point != cells[regionID].Site {
				t.Errorf("count %d attractant %d = %+v, want region (%d,%d) at cell %d", test.count, i, attractant, wantX, wantY, regionID)
			}
		}
	}
}

func TestGenerateRectangleUsesFixedAreaBoundsAndPointBudget(t *testing.T) {
	const ocean = 0.5
	for _, aspect := range []string{"16:9", "9:16"} {
		config := DefaultConfig()
		config.WorldSeed = 42
		config.ProvinceCount = 40
		config.IslandCount = 4
		config.AspectRatio = aspect
		config.OceanPercentage = ocean
		config.MaxRounds = 1
		result, err := Generate(config)
		if err != nil {
			t.Fatalf("Generate(%s) error = %v", aspect, err)
		}
		if got, want := len(result.Cells), int(math.Ceil(float64(config.ProvinceCount)/(1-ocean))); got != want {
			t.Errorf("Generate(%s) created %d cells, want unchanged budget %d", aspect, got, want)
		}
		width, height, _ := aspectratio.Dimensions(aspect)
		for _, cell := range result.Cells {
			if !(cell.Site.X > 0 && cell.Site.X < width && cell.Site.Y > 0 && cell.Site.Y < height) {
				t.Errorf("Generate(%s) site is outside rectangle %gx%g: %+v", aspect, width, height, cell.Site)
			}
		}
	}
}

func TestMakeAttractantsSkipsRegionWithoutClearance(t *testing.T) {
	cells := regionalCells()
	edgeDistances := make([]int, len(cells))
	for i := range edgeDistances {
		edgeDistances[i] = 20
	}
	edgeRamp := []float64{-1, -0.5, 0}
	attractantRamp := []float64{1, 0.5, 0}
	edgeDistances[0] = edgeRampReach(edgeRamp) + attractantRampReach(attractantRamp)
	attractants, skips := makeAttractants(cells, singlemesh.Bounds{Width: 1, Height: 1}, edgeDistances, edgeRamp, attractantRamp, 9, 0.65, roundRandom(1, 0))
	if len(attractants) != 8 || len(skips) != 1 || skips[0].RegionX != 0 || skips[0].RegionY != 0 || skips[0].Reason == "" {
		t.Fatalf("attractants=%d skips=%+v, want region (0,0) skipped with a reason", len(attractants), skips)
	}
}

func TestOverlappingAttractantsAddAndClipAtOne(t *testing.T) {
	cells := lineCells(3)
	attractants := []Attractant{{CellID: 0}, {CellID: 2}}
	values := desirabilityField(cells, []float64{0, 0, 0}, allEligible(3), attractants, []float64{0.7, 0.6, 0})
	if !reflect.DeepEqual(values, []float64{0.7, 1, 0.7}) {
		t.Fatalf("desirability = %v, want [0.7 1 0.7]", values)
	}
}

func TestGenerateRejectsInvalidConfig(t *testing.T) {
	valid := DefaultConfig()
	tests := []Config{
		{},
		withConfig(valid, func(c *Config) { c.IslandCount = 0 }),
		withConfig(valid, func(c *Config) { c.ProvinceCount = c.IslandCount - 1 }),
		withConfig(valid, func(c *Config) { c.AspectRatio = "16/9" }),
		withConfig(valid, func(c *Config) { c.OceanPercentage = math.NaN() }),
		withConfig(valid, func(c *Config) { c.EdgeBarrierWidth = -0.01 }),
		withConfig(valid, func(c *Config) { c.EdgeRamp = nil }),
		withConfig(valid, func(c *Config) { c.EdgeRamp = []float64{-1, -0.5} }),
		withConfig(valid, func(c *Config) { c.EdgeRamp = []float64{-1, -0.4, -0.6, 0} }),
		withConfig(valid, func(c *Config) { c.AttractantCount = 7 }),
		withConfig(valid, func(c *Config) { c.AttractantRamp = nil }),
		withConfig(valid, func(c *Config) { c.AttractantRamp = []float64{0} }),
		withConfig(valid, func(c *Config) { c.AttractantRamp = []float64{1, 0.5} }),
		withConfig(valid, func(c *Config) { c.AttractantRamp = []float64{0.5, 0.7, 0} }),
		withConfig(valid, func(c *Config) { c.AttractantJitter = 1.01 }),
		withConfig(valid, func(c *Config) { c.SoftmaxTemperature = 0 }),
		withConfig(valid, func(c *Config) { c.ControlPenalty = -1.01 }),
		withConfig(valid, func(c *Config) { c.MaxRounds = 0 }),
		withConfig(valid, func(c *Config) { c.Relaxations = -1 }),
	}
	for _, config := range tests {
		if result, err := Generate(config); err == nil || !reflect.DeepEqual(result, Result{}) {
			t.Errorf("Generate(%+v) = (%+v, %v), want empty result and error", config, result, err)
		}
	}
}

func TestGenerateReportsStarvationWhenBarrierLeavesTooFewCells(t *testing.T) {
	config := DefaultConfig()
	config.WorldSeed = 1
	config.ProvinceCount = 10
	config.IslandCount = 1
	config.OceanPercentage = 0
	config.EdgeBarrierWidth = 0.10
	config.MaxRounds = 1
	config.Relaxations = 0
	if result, err := Generate(config); !errors.Is(err, ErrStarved) || !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("Generate() = (%+v, %v), want empty result and ErrStarved", result, err)
	}
}

func withConfig(config Config, change func(*Config)) Config {
	change(&config)
	return config
}

func assertValidResult(t *testing.T, result Result, config Config) {
	t.Helper()
	if result.InitialIslandCount != config.IslandCount || result.MergeCount != config.IslandCount-len(result.Islands) {
		t.Errorf("islands=%d initial=%d merges=%d", len(result.Islands), result.InitialIslandCount, result.MergeCount)
	}
	if len(result.Attractants)+len(result.AttractantSkips) != config.AttractantCount {
		t.Errorf("attractants=%d skips=%d, want %d regions", len(result.Attractants), len(result.AttractantSkips), config.AttractantCount)
	}
	assertAttractantCoverage(t, result, config)
	landCount := 0
	barrierCount := 0
	memberships := make([]int, len(result.Cells))
	for islandID, island := range result.Islands {
		if island.ID != islandID {
			t.Errorf("island at index %d has ID %d", islandID, island.ID)
		}
		if len(island.CellIDs) == 0 {
			t.Fatalf("island %d has no cells", islandID)
		}
		seen := map[int]bool{island.CellIDs[0]: true}
		queue := []int{island.CellIDs[0]}
		for head := 0; head < len(queue); head++ {
			for _, neighbor := range result.Cells[queue[head]].Neighbors {
				if !seen[neighbor] && result.Cells[neighbor].IslandID == islandID {
					seen[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
		if len(seen) != len(island.CellIDs) {
			t.Errorf("island %d has %d connected cells, want %d", islandID, len(seen), len(island.CellIDs))
		}
		for _, cellID := range island.CellIDs {
			landCount++
			memberships[cellID]++
			if result.Cells[cellID].IslandID != islandID {
				t.Errorf("island %d includes cell %d owned by %d", islandID, cellID, result.Cells[cellID].IslandID)
			}
		}
	}
	if landCount != config.ProvinceCount {
		t.Errorf("land count = %d, want %d", landCount, config.ProvinceCount)
	}
	for cellID, cell := range result.Cells {
		if cell.ID != cellID || cell.Desirability < -1 || cell.Desirability > 1 {
			t.Errorf("invalid cell %d: %+v", cellID, cell)
		}
		wantMemberships := 0
		if cell.IslandID != Water {
			wantMemberships = 1
		}
		if memberships[cellID] != wantMemberships {
			t.Errorf("cell %d memberships = %d, want %d", cellID, memberships[cellID], wantMemberships)
		}
		if !cell.LandEligible {
			barrierCount++
			if cell.IslandID != Water || cell.ControllerID != Water {
				t.Errorf("barrier cell %d has owner %d or controller %d", cellID, cell.IslandID, cell.ControllerID)
			}
		}
		if cell.ControllerID >= len(result.Islands) {
			t.Errorf("cell %d has invalid controller %d", cellID, cell.ControllerID)
		}
	}
	if barrierCount == 0 {
		t.Error("result has no permanent ocean barrier cells")
	}
}

func lineCells(count int) []singlemesh.Cell {
	cells := make([]singlemesh.Cell, count)
	for i := range cells {
		cells[i].ID = i
		if i > 0 {
			cells[i].Neighbors = append(cells[i].Neighbors, i-1)
		}
		if i+1 < count {
			cells[i].Neighbors = append(cells[i].Neighbors, i+1)
		}
	}
	return cells
}

func allEligible(count int) []bool {
	values := make([]bool, count)
	for i := range values {
		values[i] = true
	}
	return values
}

func regionalCells() []singlemesh.Cell {
	cells := make([]singlemesh.Cell, 0, 9)
	for regionY := 0; regionY < 3; regionY++ {
		for regionX := 0; regionX < 3; regionX++ {
			cells = append(cells, singlemesh.Cell{
				ID: len(cells),
				Site: Point{
					X: (float64(regionX) + 0.5) / 3,
					Y: (float64(regionY) + 0.5) / 3,
				},
			})
		}
	}
	return cells
}

func assertAttractantCoverage(t *testing.T, result Result, config Config) {
	t.Helper()
	distances := make([]int, len(result.Cells))
	queue := make([]int, 0)
	for cellID, cell := range result.Cells {
		distances[cellID] = -1
		if !cell.LandEligible {
			distances[cellID] = 0
			queue = append(queue, cellID)
		}
	}
	for head := 0; head < len(queue); head++ {
		for _, neighbor := range result.Cells[queue[head]].Neighbors {
			if distances[neighbor] == -1 {
				distances[neighbor] = distances[queue[head]] + 1
				queue = append(queue, neighbor)
			}
		}
	}
	clearance := edgeRampReach(config.EdgeRamp) + attractantRampReach(config.AttractantRamp)
	regions := make(map[[2]int]bool, config.AttractantCount)
	for _, attractant := range result.Attractants {
		region := [2]int{attractant.RegionX, attractant.RegionY}
		if regions[region] {
			t.Errorf("region %v has multiple attractants", region)
		}
		regions[region] = true
		if attractant.CellID < 0 || attractant.CellID >= len(result.Cells) || result.Cells[attractant.CellID].Site != attractant.Point {
			t.Errorf("invalid attractant: %+v", attractant)
			continue
		}
		if distances[attractant.CellID] <= clearance {
			t.Errorf("attractant in region %v is %d hops from edge, want more than %d", region, distances[attractant.CellID], clearance)
		}
		cell := result.Cells[attractant.CellID]
		if min(int(cell.Site.X*3), 2) != attractant.RegionX || min(int(cell.Site.Y*3), 2) != attractant.RegionY {
			t.Errorf("attractant %+v is outside its region", attractant)
		}
	}
	for _, skip := range result.AttractantSkips {
		region := [2]int{skip.RegionX, skip.RegionY}
		if regions[region] || skip.Reason == "" {
			t.Errorf("invalid attractant skip: %+v", skip)
		}
		regions[region] = true
	}
	if len(regions) != config.AttractantCount {
		t.Errorf("attractant diagnostics cover %d regions, want %d", len(regions), config.AttractantCount)
	}
}
