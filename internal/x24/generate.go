package x24

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/mdhender/wgvc/internal/singlemesh"
)

const (
	oceanEscalation = 0.03
	maximumOcean    = 0.95
	boundaryEpsilon = 1e-9
)

func Generate(config Config) (Result, error) {
	if err := config.validate(); err != nil {
		return Result{}, err
	}
	for round := 0; round < config.MaxRounds; round++ {
		ocean := math.Min(config.OceanPercentage+float64(round)*oceanEscalation, maximumOcean)
		cellCount := int(math.Ceil(float64(config.ProvinceCount) / (1 - ocean)))
		random := roundRandom(config.WorldSeed, round)
		mesh, err := singlemesh.Build(cellCount, config.Relaxations, random)
		if err != nil {
			return Result{}, fmt.Errorf("round %d mesh: %w", round+1, err)
		}
		attractants := makeAttractants(config.AttractantCount, random)
		edgeValues, landEligible := edgeField(mesh, config.EdgeBarrierWidth, config.EdgeRamp)
		desirability := desirabilityField(mesh, edgeValues, landEligible, attractants, config.AttractantRadius)
		state := newGrowthState(mesh, desirability, landEligible, config.ControlPenalty)
		if state.seedAndGrow(config.IslandCount, config.ProvinceCount, config.SoftmaxTemperature, random) {
			result := state.result(attractants, config.IslandCount)
			result.RoundsAttempted = round + 1
			result.FinalOcean = ocean
			return result, nil
		}
	}
	return Result{}, fmt.Errorf("%w after %d rounds", ErrStarved, config.MaxRounds)
}

func makeAttractants(count int, random *rand.Rand) []Point {
	points := make([]Point, count)
	for i := range points {
		points[i] = Point{X: random.Float64(), Y: random.Float64()}
	}
	return points
}

func desirabilityField(cells []singlemesh.Cell, edgeValues []float64, landEligible []bool, attractants []Point, attractantRadius float64) []float64 {
	values := append([]float64(nil), edgeValues...)
	for cellID, cell := range cells {
		if !landEligible[cellID] {
			continue
		}
		for _, attractant := range attractants {
			distance := math.Hypot(cell.Site.X-attractant.X, cell.Site.Y-attractant.Y)
			values[cellID] += math.Max(0, 1-distance/attractantRadius)
		}
		values[cellID] = max(-1, min(1, values[cellID]))
	}
	return values
}

func edgeField(cells []singlemesh.Cell, barrierWidth float64, ramp []float64) ([]float64, []bool) {
	values := make([]float64, len(cells))
	landEligible := make([]bool, len(cells))
	distances := make([]int, len(cells))
	queue := make([]int, 0)
	for cellID, cell := range cells {
		distances[cellID] = -1
		landEligible[cellID] = true
		if cellInBarrier(cell, barrierWidth) {
			values[cellID] = -1
			landEligible[cellID] = false
			distances[cellID] = 0
			queue = append(queue, cellID)
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
	for cellID, distance := range distances {
		if distance <= 0 {
			continue
		}
		step := distance - 1
		if step < len(ramp) {
			values[cellID] = ramp[step]
		}
	}
	return values, landEligible
}

func cellInBarrier(cell singlemesh.Cell, width float64) bool {
	for _, point := range cell.Corners {
		if point.X <= width+boundaryEpsilon || point.X >= 1-width-boundaryEpsilon ||
			point.Y <= width+boundaryEpsilon || point.Y >= 1-width-boundaryEpsilon {
			return true
		}
	}
	return false
}

type islandState struct {
	seedID  int
	cellIDs []int
	active  bool
}

type growthState struct {
	cells          []singlemesh.Cell
	desirability   []float64
	landEligible   []bool
	controlPenalty float64
	owners         []int
	controllers    []int
	islands        []islandState
	frontiers      []randomSet
	mergeCount     int
}

func newGrowthState(cells []singlemesh.Cell, desirability []float64, landEligible []bool, controlPenalty float64) *growthState {
	owners := make([]int, len(cells))
	controllers := make([]int, len(cells))
	for i := range cells {
		owners[i] = Water
		controllers[i] = Water
	}
	return &growthState{
		cells:          cells,
		desirability:   desirability,
		landEligible:   landEligible,
		controlPenalty: controlPenalty,
		owners:         owners,
		controllers:    controllers,
	}
}

func (s *growthState) seedAndGrow(islandCount, provinceCount int, temperature float64, random *rand.Rand) bool {
	seedOrder := random.Perm(islandCount)
	s.islands = make([]islandState, islandCount)
	s.frontiers = make([]randomSet, islandCount)
	for _, islandID := range seedOrder {
		unclaimed := make([]int, 0, len(s.cells))
		for cellID, owner := range s.owners {
			if owner == Water && s.landEligible[cellID] {
				unclaimed = append(unclaimed, cellID)
			}
		}
		if len(unclaimed) == 0 {
			return false
		}
		seedID := unclaimed[random.IntN(len(unclaimed))]
		s.islands[islandID] = islandState{seedID: seedID, active: true}
		s.claim(seedID, islandID)
	}

	deck := randomSet{}
	for islandID := range s.islands {
		if s.islands[islandID].active {
			deck.add(islandID)
		}
	}
	remaining := provinceCount - islandCount
	for remaining > 0 && deck.len() > 0 {
		islandID := deck.random(random)
		cellID, ok := s.weightedFrontier(islandID, temperature, random)
		if !ok {
			deck.remove(islandID)
			continue
		}
		loser := s.claim(cellID, islandID)
		if loser != Water {
			deck.remove(loser)
		}
		remaining--
	}
	return remaining == 0
}

func (s *growthState) weightedFrontier(islandID int, temperature float64, random *rand.Rand) (int, bool) {
	frontier := s.frontiers[islandID].values
	if len(frontier) == 0 {
		return 0, false
	}
	weights := make([]float64, len(frontier))
	maximum := math.Inf(-1)
	for i, cellID := range frontier {
		value := s.visibleValue(cellID, islandID)
		maximum = max(maximum, value)
		weights[i] = value
	}
	total := 0.0
	for i, value := range weights {
		weights[i] = math.Exp((value - maximum) / temperature)
		total += weights[i]
	}
	draw := random.Float64() * total
	for i, weight := range weights {
		draw -= weight
		if draw < 0 {
			return frontier[i], true
		}
	}
	return frontier[len(frontier)-1], true
}

func (s *growthState) visibleValue(cellID, islandID int) float64 {
	controller := s.controllers[cellID]
	if controller != Water && controller != islandID {
		return s.controlPenalty
	}
	return s.desirability[cellID]
}

// claim returns the absorbed island ID, or Water when no merger occurred.
func (s *growthState) claim(cellID, islandID int) int {
	controller := s.controllers[cellID]
	merge := controller != Water && controller != islandID && s.adjacentToIsland(cellID, controller)

	s.owners[cellID] = islandID
	s.controllers[cellID] = Water
	s.islands[islandID].cellIDs = append(s.islands[islandID].cellIDs, cellID)
	for i := range s.frontiers {
		s.frontiers[i].remove(cellID)
	}
	for _, neighbor := range s.cells[cellID].Neighbors {
		if s.owners[neighbor] != Water || !s.landEligible[neighbor] {
			continue
		}
		s.frontiers[islandID].add(neighbor)
		if s.controllers[neighbor] == Water {
			s.controllers[neighbor] = islandID
		}
	}
	if !merge {
		return Water
	}
	s.absorb(islandID, controller)
	return controller
}

func (s *growthState) adjacentToIsland(cellID, islandID int) bool {
	for _, neighbor := range s.cells[cellID].Neighbors {
		if s.owners[neighbor] == islandID {
			return true
		}
	}
	return false
}

func (s *growthState) absorb(winner, loser int) {
	for cellID := range s.cells {
		if s.owners[cellID] == loser {
			s.owners[cellID] = winner
		}
		if s.controllers[cellID] == loser {
			s.controllers[cellID] = winner
		}
	}
	s.islands[winner].cellIDs = append(s.islands[winner].cellIDs, s.islands[loser].cellIDs...)
	s.islands[loser].cellIDs = nil
	s.islands[loser].active = false
	for _, cellID := range s.frontiers[loser].values {
		s.frontiers[winner].add(cellID)
	}
	s.frontiers[loser] = randomSet{}
	s.mergeCount++
}

func (s *growthState) result(attractants []Point, initialIslandCount int) Result {
	canonicalIDs := make([]int, len(s.islands))
	for i := range canonicalIDs {
		canonicalIDs[i] = Water
	}
	islands := make([]Island, 0, len(s.islands)-s.mergeCount)
	for oldID, island := range s.islands {
		if !island.active {
			continue
		}
		canonicalIDs[oldID] = len(islands)
		cellIDs := append([]int(nil), island.cellIDs...)
		sort.Ints(cellIDs)
		islands = append(islands, Island{ID: len(islands), SeedID: island.seedID, CellIDs: cellIDs})
	}
	cells := make([]Cell, len(s.cells))
	for cellID, cell := range s.cells {
		owner, controller := s.owners[cellID], s.controllers[cellID]
		if owner != Water {
			owner = canonicalIDs[owner]
		}
		if controller != Water {
			controller = canonicalIDs[controller]
		}
		cells[cellID] = Cell{
			ID:           cell.ID,
			Site:         cell.Site,
			Corners:      cell.Corners,
			Neighbors:    cell.Neighbors,
			LandEligible: s.landEligible[cellID],
			Desirability: s.desirability[cellID],
			IslandID:     owner,
			ControllerID: controller,
		}
	}
	return Result{
		Cells:              cells,
		Islands:            islands,
		Attractants:        append([]Point(nil), attractants...),
		InitialIslandCount: initialIslandCount,
		MergeCount:         s.mergeCount,
	}
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

func (s *randomSet) len() int {
	return len(s.values)
}
