package x23

import (
	"fmt"
	"math"
	"math/rand/v2"
)

const (
	oceanEscalation = 0.03
	maximumOcean    = 0.95
)

func Generate(config Config) (Result, error) {
	if err := config.validate(); err != nil {
		return Result{}, err
	}
	for round := 0; round < config.MaxRounds; round++ {
		ocean := math.Min(config.OceanPercentage+float64(round)*oceanEscalation, maximumOcean)
		cellCount := int(math.Ceil(float64(config.ProvinceCount) / (1 - ocean)))
		random := roundRandom(config.WorldSeed, round)
		cells, err := buildMesh(cellCount, config.Relaxations, random)
		if err != nil {
			return Result{}, fmt.Errorf("round %d mesh: %w", round+1, err)
		}
		result, ok := grow(config, cells, random)
		if ok {
			result.RoundsAttempted = round + 1
			result.FinalOcean = ocean
			return result, nil
		}
	}
	return Result{}, fmt.Errorf("could not grow %d islands to %d provinces after %d rounds", config.IslandCount, config.ProvinceCount, config.MaxRounds)
}

func grow(config Config, cells []Cell, random *rand.Rand) (Result, bool) {
	boundaryDistance := distancesFromBoundary(cells)
	owners := make([]int, len(cells))
	for i := range owners {
		owners[i] = Water
	}
	frontiers := make([]randomSet, config.IslandCount)
	islands := make([]Island, config.IslandCount)
	seedOrder := random.Perm(config.IslandCount)
	for _, islandID := range seedOrder {
		legal := make([]int, 0, len(cells))
		for cellID := range cells {
			if legalForIsland(cellID, islandID, config.MinEdgeDistance, config.MinIslandDistance, cells, owners, boundaryDistance) {
				legal = append(legal, cellID)
			}
		}
		if len(legal) == 0 {
			return Result{}, false
		}
		seedID := legal[random.IntN(len(legal))]
		islands[islandID] = Island{ID: islandID, SeedID: seedID}
		claim(seedID, islandID, config.MinEdgeDistance, config.MinIslandDistance, cells, owners, boundaryDistance, frontiers, &islands[islandID])
	}

	deck := randomSet{}
	for islandID := range islands {
		deck.add(islandID)
	}
	remaining := config.ProvinceCount - config.IslandCount
	for remaining > 0 && deck.len() > 0 {
		islandID := deck.random(random)
		for {
			cellID, ok := frontiers[islandID].randomOK(random)
			if !ok {
				deck.remove(islandID)
				break
			}
			if !legalForIsland(cellID, islandID, config.MinEdgeDistance, config.MinIslandDistance, cells, owners, boundaryDistance) {
				frontiers[islandID].remove(cellID)
				continue
			}
			claim(cellID, islandID, config.MinEdgeDistance, config.MinIslandDistance, cells, owners, boundaryDistance, frontiers, &islands[islandID])
			remaining--
			break
		}
	}
	if remaining != 0 {
		return Result{}, false
	}
	for i := range cells {
		cells[i].IslandID = owners[i]
	}
	return Result{Cells: cells, Islands: islands}, true
}

func claim(cellID, islandID, minimumEdgeDistance, minimumIslandDistance int, cells []Cell, owners, boundaryDistance []int, frontiers []randomSet, island *Island) {
	owners[cellID] = islandID
	island.CellIDs = append(island.CellIDs, cellID)
	for other := range frontiers {
		frontiers[other].remove(cellID)
	}
	for _, neighbor := range cells[cellID].Neighbors {
		if legalForIsland(neighbor, islandID, minimumEdgeDistance, minimumIslandDistance, cells, owners, boundaryDistance) {
			frontiers[islandID].add(neighbor)
		}
	}
	pruneDistanceBlocked(cellID, islandID, minimumIslandDistance, cells, frontiers)
}

func legalForIsland(cellID, islandID, minimumEdgeDistance, minimumIslandDistance int, cells []Cell, owners, boundaryDistance []int) bool {
	if owners[cellID] != Water || boundaryDistance[cellID] < minimumEdgeDistance {
		return false
	}
	if minimumIslandDistance <= 1 {
		return true
	}
	seen := map[int]bool{cellID: true}
	level := []int{cellID}
	for distance := 1; distance < minimumIslandDistance; distance++ {
		next := make([]int, 0)
		for _, current := range level {
			for _, neighbor := range cells[current].Neighbors {
				if seen[neighbor] {
					continue
				}
				seen[neighbor] = true
				if owners[neighbor] != Water && owners[neighbor] != islandID {
					return false
				}
				next = append(next, neighbor)
			}
		}
		level = next
	}
	return true
}

func pruneDistanceBlocked(cellID, islandID, minimumDistance int, cells []Cell, frontiers []randomSet) {
	if minimumDistance <= 1 {
		return
	}
	seen := map[int]bool{cellID: true}
	level := []int{cellID}
	for distance := 1; distance < minimumDistance; distance++ {
		next := make([]int, 0)
		for _, current := range level {
			for _, neighbor := range cells[current].Neighbors {
				if seen[neighbor] {
					continue
				}
				seen[neighbor] = true
				for other := range frontiers {
					if other != islandID {
						frontiers[other].remove(neighbor)
					}
				}
				next = append(next, neighbor)
			}
		}
		level = next
	}
}

func distancesFromBoundary(cells []Cell) []int {
	distances := make([]int, len(cells))
	queue := make([]int, 0)
	for cellID, cell := range cells {
		distances[cellID] = -1
		for _, point := range cell.Corners {
			if point.X == 0 || point.X == 1 || point.Y == 0 || point.Y == 1 {
				distances[cellID] = 0
				queue = append(queue, cellID)
				break
			}
		}
	}
	for head := 0; head < len(queue); head++ {
		cellID := queue[head]
		for _, neighbor := range cells[cellID].Neighbors {
			if distances[neighbor] != -1 {
				continue
			}
			distances[neighbor] = distances[cellID] + 1
			queue = append(queue, neighbor)
		}
	}
	return distances
}

type randomSet struct {
	values  []int
	indexes map[int]int
}

func (s *randomSet) add(value int) {
	if s.indexes == nil {
		s.indexes = make(map[int]int)
	}
	if _, exists := s.indexes[value]; exists {
		return
	}
	s.indexes[value] = len(s.values)
	s.values = append(s.values, value)
}

func (s *randomSet) remove(value int) {
	index, exists := s.indexes[value]
	if !exists {
		return
	}
	last := len(s.values) - 1
	moved := s.values[last]
	s.values[index] = moved
	s.indexes[moved] = index
	s.values = s.values[:last]
	delete(s.indexes, value)
}

func (s *randomSet) random(random *rand.Rand) int {
	return s.values[random.IntN(len(s.values))]
}

func (s *randomSet) randomOK(random *rand.Rand) (int, bool) {
	if len(s.values) == 0 {
		return 0, false
	}
	return s.random(random), true
}

func (s *randomSet) len() int {
	return len(s.values)
}
