package x24

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/mdhender/wgvc/internal/singlemesh"
)

func TestDefaultConfig(t *testing.T) {
	got := DefaultConfig()
	if got.WorldSeed != 0x0123456789abcdef || got.ProvinceCount != 1_500 || got.IslandCount != 15 || got.OceanPercentage != 0.68 {
		t.Fatalf("DefaultConfig() required values = %+v", got)
	}
	if got.EdgeBarrierWidth != 0.02 || !reflect.DeepEqual(got.EdgeRamp, []float64{-1, -0.65, -0.40, -0.22, -0.10, -0.04, 0}) || got.ControlPenalty != -0.82 {
		t.Fatalf("DefaultConfig() issue #25 values = %+v", got)
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

func TestClaimMergesOnlyWhenControlledCellTouchesControllerLand(t *testing.T) {
	cells := lineCells(4)
	state := newGrowthState(cells, []float64{0, 0, 0, 0}, allEligible(4), -0.82)
	state.islands = []islandState{{seedID: 0, active: true}, {seedID: 1, active: true}}
	state.frontiers = make([]randomSet, 2)

	if loser := state.claim(0, 0); loser != Water {
		t.Fatalf("first claim absorbed island %d", loser)
	}
	if loser := state.claim(1, 1); loser != 0 {
		t.Fatalf("adjacent controlled claim absorbed %d, want 0", loser)
	}
	if state.islands[0].active || state.owners[0] != 1 || state.controllers[1] != Water {
		t.Fatalf("merge did not transfer loser state: islands=%+v owners=%v controllers=%v", state.islands, state.owners, state.controllers)
	}

	state = newGrowthState(cells, []float64{0, 0, 0, 0}, allEligible(4), -0.82)
	state.islands = []islandState{{seedID: 0, active: true}, {seedID: 2, active: true}}
	state.frontiers = make([]randomSet, 2)
	state.claim(0, 0)
	state.controllers[2] = 0 // Simulate a future multi-hop control ripple.
	if loser := state.claim(2, 1); loser != Water {
		t.Fatalf("non-adjacent controlled claim absorbed island %d", loser)
	}
	if !state.islands[0].active || state.mergeCount != 0 {
		t.Fatalf("non-adjacent control caused a merge: islands=%+v merges=%d", state.islands, state.mergeCount)
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
	values, eligible := edgeField(cells, 0.01, []float64{-1, -0.5, 0})
	if !reflect.DeepEqual(values, []float64{-1, -1, -0.5, 0, 0}) {
		t.Fatalf("edge values = %v, want [-1 -1 -0.5 0 0]", values)
	}
	if !reflect.DeepEqual(eligible, []bool{false, true, true, true, true}) {
		t.Fatalf("land eligibility = %v", eligible)
	}

	cells[1].Site = Point{X: 0.2, Y: 0.4}
	withAttractants := desirabilityField(cells, values, eligible, []Point{{X: 0.2, Y: 0.4}, {X: 0.2, Y: 0.4}}, 0.2)
	if withAttractants[0] != -1 || withAttractants[1] != 1 {
		t.Fatalf("attractants changed barrier or failed to clip: %v", withAttractants)
	}
}

func TestEdgeFieldKeepsHarsherValueFromMultiplePaths(t *testing.T) {
	cells := []singlemesh.Cell{
		{ID: 0, Corners: []Point{{X: 0, Y: 0.2}}, Neighbors: []int{3}},
		{ID: 1, Corners: []Point{{X: 0, Y: 0.8}}, Neighbors: []int{2}},
		{ID: 2, Corners: []Point{{X: 0.3, Y: 0.8}}, Neighbors: []int{1, 3}},
		{ID: 3, Corners: []Point{{X: 0.3, Y: 0.3}}, Neighbors: []int{0, 2}},
	}
	values, _ := edgeField(cells, 0.01, []float64{-1, -0.4, 0})
	if values[3] != -1 {
		t.Fatalf("cell reached in one and two hops has value %g, want harsher one-hop value -1", values[3])
	}
}

func TestGenerateRejectsInvalidConfig(t *testing.T) {
	valid := DefaultConfig()
	tests := []Config{
		{},
		withConfig(valid, func(c *Config) { c.IslandCount = 0 }),
		withConfig(valid, func(c *Config) { c.ProvinceCount = c.IslandCount - 1 }),
		withConfig(valid, func(c *Config) { c.OceanPercentage = math.NaN() }),
		withConfig(valid, func(c *Config) { c.EdgeBarrierWidth = -0.01 }),
		withConfig(valid, func(c *Config) { c.EdgeRamp = nil }),
		withConfig(valid, func(c *Config) { c.EdgeRamp = []float64{-1, -0.5} }),
		withConfig(valid, func(c *Config) { c.EdgeRamp = []float64{-1, -0.4, -0.6, 0} }),
		withConfig(valid, func(c *Config) { c.AttractantCount = -1 }),
		withConfig(valid, func(c *Config) { c.AttractantRadius = 0 }),
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
	if len(result.Attractants) != config.AttractantCount {
		t.Errorf("attractants = %d, want %d", len(result.Attractants), config.AttractantCount)
	}
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
