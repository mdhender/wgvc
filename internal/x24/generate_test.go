package x24

import (
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
	state := newGrowthState(cells, []float64{0, 0, 0, 0}, -0.95)
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

	state = newGrowthState(cells, []float64{0, 0, 0, 0}, -0.95)
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
	state := newGrowthState(lineCells(2), []float64{0.75, 0.50}, -0.95)
	state.controllers[1] = 0
	if got := state.visibleValue(1, 0); got != 0.50 {
		t.Errorf("controller sees %g, want honest field 0.5", got)
	}
	if got := state.visibleValue(1, 1); got != -0.95 {
		t.Errorf("rival sees %g, want control penalty -0.95", got)
	}
}

func TestDesirabilityFieldRampsFromBoundaryAndClipsAttractants(t *testing.T) {
	cells := []singlemesh.Cell{
		{ID: 0, Site: Point{X: 0.05, Y: 0.5}, Corners: []Point{{X: 0, Y: 0}, {X: 0.1, Y: 0}, {X: 0.1, Y: 1}, {X: 0, Y: 1}}, Neighbors: []int{1}},
		{ID: 1, Site: Point{X: 0.5, Y: 0.5}, Corners: []Point{{X: 0.1, Y: 0.1}, {X: 0.9, Y: 0.1}, {X: 0.9, Y: 0.9}, {X: 0.1, Y: 0.9}}, Neighbors: []int{0, 2}},
		{ID: 2, Site: Point{X: 0.95, Y: 0.5}, Corners: []Point{{X: 0.9, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0.9, Y: 1}}, Neighbors: []int{1}},
	}
	without := desirabilityField(cells, nil, 2, 0.2)
	if without[0] != -1 || without[1] != -0.5 || without[2] != -1 {
		t.Fatalf("edge ramp = %v, want [-1 -0.5 -1]", without)
	}
	with := desirabilityField(cells, []Point{{X: 0.5, Y: 0.5}, {X: 0.5, Y: 0.5}}, 2, 0.2)
	if with[1] != 1 {
		t.Fatalf("stacked attractants produced %g, want clipped value 1", with[1])
	}
}

func TestGenerateRejectsInvalidConfig(t *testing.T) {
	valid := DefaultConfig()
	tests := []Config{
		{},
		withConfig(valid, func(c *Config) { c.IslandCount = 0 }),
		withConfig(valid, func(c *Config) { c.ProvinceCount = c.IslandCount - 1 }),
		withConfig(valid, func(c *Config) { c.OceanPercentage = math.NaN() }),
		withConfig(valid, func(c *Config) { c.EdgeRampDistance = 0 }),
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
		if cell.ControllerID >= len(result.Islands) {
			t.Errorf("cell %d has invalid controller %d", cellID, cell.ControllerID)
		}
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
