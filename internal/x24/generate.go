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
		edgeValues, landEligible, edgeDistances := edgeField(mesh, config.EdgeBarrierWidth, config.EdgeRamp)
		attractants, skips := makeAttractants(mesh, edgeDistances, config.EdgeRamp, config.AttractantRamp, config.AttractantJitter, random)
		desirability := desirabilityField(mesh, edgeValues, landEligible, attractants, config.AttractantRamp)
		state := newGrowthState(mesh, desirability, landEligible, config.ControlPenalty)
		if state.seedAndGrow(config.IslandCount, config.ProvinceCount, config.SoftmaxTemperature, random) {
			result := state.result(attractants, skips, config.IslandCount)
			result.RoundsAttempted = round + 1
			result.FinalOcean = ocean
			return result, nil
		}
	}
	return Result{}, fmt.Errorf("%w after %d rounds", ErrStarved, config.MaxRounds)
}

func makeAttractants(cells []singlemesh.Cell, edgeDistances []int, edgeRamp, attractantRamp []float64, jitter float64, random *rand.Rand) ([]Attractant, []AttractantSkip) {
	const regions = 3
	clearance := edgeRampReach(edgeRamp) + attractantRampReach(attractantRamp)
	attractants := make([]Attractant, 0, regions*regions)
	skips := make([]AttractantSkip, 0)
	for regionY := 0; regionY < regions; regionY++ {
		for regionX := 0; regionX < regions; regionX++ {
			center := Point{X: (float64(regionX) + 0.5) / regions, Y: (float64(regionY) + 0.5) / regions}
			maximumJitter := jitter / (2 * regions)
			target := Point{
				X: center.X + (2*random.Float64()-1)*maximumJitter,
				Y: center.Y + (2*random.Float64()-1)*maximumJitter,
			}
			bestCell, bestDistance := -1, math.Inf(1)
			for cellID, cell := range cells {
				cellRegionX := min(int(cell.Site.X*regions), regions-1)
				cellRegionY := min(int(cell.Site.Y*regions), regions-1)
				if cellRegionX != regionX || cellRegionY != regionY || edgeDistances[cellID] <= clearance {
					continue
				}
				dx, dy := cell.Site.X-target.X, cell.Site.Y-target.Y
				distance := dx*dx + dy*dy
				if distance < bestDistance {
					bestCell, bestDistance = cellID, distance
				}
			}
			if bestCell == -1 {
				skips = append(skips, AttractantSkip{
					RegionX: regionX,
					RegionY: regionY,
					Reason:  fmt.Sprintf("no cell is more than %d hops from the edge barrier", clearance),
				})
				continue
			}
			attractants = append(attractants, Attractant{
				CellID:  bestCell,
				Point:   cells[bestCell].Site,
				RegionX: regionX,
				RegionY: regionY,
			})
		}
	}
	return attractants, skips
}

func edgeRampReach(ramp []float64) int {
	return lastNonzeroIndex(ramp) + 1
}

func attractantRampReach(ramp []float64) int {
	return max(lastNonzeroIndex(ramp), 0)
}

func lastNonzeroIndex(ramp []float64) int {
	last := -1
	for i, value := range ramp {
		if value != 0 {
			last = i
		}
	}
	return last
}

func desirabilityField(cells []singlemesh.Cell, edgeValues []float64, landEligible []bool, attractants []Attractant, attractantRamp []float64) []float64 {
	values := append([]float64(nil), edgeValues...)
	reach := attractantRampReach(attractantRamp)
	for _, attractant := range attractants {
		distances := make([]int, len(cells))
		for i := range distances {
			distances[i] = -1
		}
		distances[attractant.CellID] = 0
		queue := []int{attractant.CellID}
		for head := 0; head < len(queue); head++ {
			cellID := queue[head]
			distance := distances[cellID]
			if landEligible[cellID] {
				values[cellID] += attractantRamp[distance]
			}
			if distance >= reach {
				continue
			}
			for _, neighbor := range cells[cellID].Neighbors {
				if distances[neighbor] == -1 {
					distances[neighbor] = distance + 1
					queue = append(queue, neighbor)
				}
			}
		}
	}
	for cellID := range values {
		values[cellID] = max(-1, min(1, values[cellID]))
	}
	return values
}

func edgeField(cells []singlemesh.Cell, barrierWidth float64, ramp []float64) ([]float64, []bool, []int) {
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
	return values, landEligible, distances
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

func (s *growthState) result(attractants []Attractant, skips []AttractantSkip, initialIslandCount int) Result {
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
		Attractants:        append([]Attractant(nil), attractants...),
		AttractantSkips:    append([]AttractantSkip(nil), skips...),
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
