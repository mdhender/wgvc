package x23

import (
	"math"
	"reflect"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	got := DefaultConfig()
	want := Config{
		WorldSeed:         0x0123456789abcdef,
		ProvinceCount:     1_500,
		IslandCount:       15,
		OceanPercentage:   0.68,
		MinEdgeDistance:   3,
		MinIslandDistance: 3,
		MaxRounds:         10,
		Relaxations:       2,
	}
	if got != want {
		t.Fatalf("DefaultConfig() = %+v, want %+v", got, want)
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

func TestGenerateRetriesFromFreshDeterministicRounds(t *testing.T) {
	config := DefaultConfig()
	config.WorldSeed = 1
	config.ProvinceCount = 200
	config.OceanPercentage = 0.50

	result, err := Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	assertValidResult(t, result, config)
	if result.RoundsAttempted <= 1 {
		t.Fatalf("rounds attempted = %d, want a retry fixture", result.RoundsAttempted)
	}
	wantOcean := math.Min(config.OceanPercentage+float64(result.RoundsAttempted-1)*oceanEscalation, maximumOcean)
	if result.FinalOcean != wantOcean {
		t.Errorf("final ocean = %g, want %g", result.FinalOcean, wantOcean)
	}
}

func TestGenerateRejectsInvalidConfig(t *testing.T) {
	valid := DefaultConfig()
	tests := []Config{
		{},
		withConfig(valid, func(c *Config) { c.IslandCount = 0 }),
		withConfig(valid, func(c *Config) { c.ProvinceCount = c.IslandCount - 1 }),
		withConfig(valid, func(c *Config) { c.OceanPercentage = 1 }),
		withConfig(valid, func(c *Config) { c.OceanPercentage = math.NaN() }),
		withConfig(valid, func(c *Config) { c.MinEdgeDistance = -1 }),
		withConfig(valid, func(c *Config) { c.MinIslandDistance = 0 }),
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
	if len(result.Islands) != config.IslandCount {
		t.Fatalf("island count = %d, want %d", len(result.Islands), config.IslandCount)
	}
	wantCells := int(math.Ceil(float64(config.ProvinceCount) / (1 - result.FinalOcean)))
	if len(result.Cells) != wantCells {
		t.Fatalf("cell count = %d, want %d", len(result.Cells), wantCells)
	}

	boundaryDistance := distancesFromBoundary(result.Cells)
	memberships := make([]int, len(result.Cells))
	landCount := 0
	for islandID, island := range result.Islands {
		if island.ID != islandID {
			t.Errorf("island at index %d has ID %d", islandID, island.ID)
		}
		if len(island.CellIDs) == 0 {
			t.Fatalf("island %d has no land", islandID)
		}
		if result.Cells[island.SeedID].IslandID != islandID {
			t.Errorf("island %d seed %d owner = %d", islandID, island.SeedID, result.Cells[island.SeedID].IslandID)
		}
		assertConnectedIsland(t, result, island)
		for _, cellID := range island.CellIDs {
			if cellID < 0 || cellID >= len(result.Cells) {
				t.Fatalf("island %d has invalid cell %d", islandID, cellID)
			}
			memberships[cellID]++
			landCount++
			if result.Cells[cellID].IslandID != islandID {
				t.Errorf("island %d contains cell %d owned by %d", islandID, cellID, result.Cells[cellID].IslandID)
			}
			if boundaryDistance[cellID] < config.MinEdgeDistance {
				t.Errorf("island %d cell %d boundary distance = %d, want at least %d", islandID, cellID, boundaryDistance[cellID], config.MinEdgeDistance)
			}
			assertNoOtherIslandWithin(t, result, cellID, islandID, config.MinIslandDistance)
		}
	}
	if landCount != config.ProvinceCount {
		t.Errorf("land count = %d, want %d", landCount, config.ProvinceCount)
	}
	assertConnectedWater(t, result)

	for cellID, cell := range result.Cells {
		if cell.ID != cellID {
			t.Errorf("cell at index %d has ID %d", cellID, cell.ID)
		}
		if len(cell.Corners) < 3 || signedArea(cell.Corners) <= 0 {
			t.Errorf("cell %d has invalid polygon", cellID)
		}
		wantMemberships := 1
		if cell.IslandID == Water {
			wantMemberships = 0
		}
		if memberships[cellID] != wantMemberships {
			t.Errorf("cell %d memberships = %d, want %d", cellID, memberships[cellID], wantMemberships)
		}
		for _, neighbor := range cell.Neighbors {
			if neighbor < 0 || neighbor >= len(result.Cells) || !contains(result.Cells[neighbor].Neighbors, cellID) {
				t.Errorf("cell %d has invalid or asymmetric neighbor %d", cellID, neighbor)
			}
		}
	}
}

func assertConnectedWater(t *testing.T, result Result) {
	t.Helper()
	first := -1
	want := 0
	for cellID, cell := range result.Cells {
		if cell.IslandID != Water {
			continue
		}
		want++
		if first == -1 {
			first = cellID
		}
	}
	seen := map[int]bool{first: true}
	queue := []int{first}
	for head := 0; head < len(queue); head++ {
		for _, neighbor := range result.Cells[queue[head]].Neighbors {
			if result.Cells[neighbor].IslandID == Water && !seen[neighbor] {
				seen[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}
	if len(seen) != want {
		t.Errorf("water component has %d cells, want %d", len(seen), want)
	}
}

func assertConnectedIsland(t *testing.T, result Result, island Island) {
	t.Helper()
	seen := map[int]bool{island.CellIDs[0]: true}
	queue := []int{island.CellIDs[0]}
	for head := 0; head < len(queue); head++ {
		for _, neighbor := range result.Cells[queue[head]].Neighbors {
			if !seen[neighbor] && result.Cells[neighbor].IslandID == island.ID {
				seen[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}
	if len(seen) != len(island.CellIDs) {
		t.Errorf("island %d has %d connected cells, want %d", island.ID, len(seen), len(island.CellIDs))
	}
}

func assertNoOtherIslandWithin(t *testing.T, result Result, cellID, islandID, minimumDistance int) {
	t.Helper()
	seen := map[int]bool{cellID: true}
	level := []int{cellID}
	for distance := 1; distance < minimumDistance; distance++ {
		next := make([]int, 0)
		for _, current := range level {
			for _, neighbor := range result.Cells[current].Neighbors {
				if seen[neighbor] {
					continue
				}
				seen[neighbor] = true
				if owner := result.Cells[neighbor].IslandID; owner != Water && owner != islandID {
					t.Errorf("island %d cell %d is %d hops from island %d, want at least %d", islandID, cellID, distance, owner, minimumDistance)
					return
				}
				next = append(next, neighbor)
			}
		}
		level = next
	}
}

func contains(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
