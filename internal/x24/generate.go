package x24

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/mdhender/wgvc/internal/aspectratio"
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
	width, height, _ := aspectratio.Dimensions(config.AspectRatio)
	bounds := singlemesh.Bounds{Width: width, Height: height}
	for round := 0; round < config.MaxRounds; round++ {
		ocean := math.Min(config.OceanPercentage+float64(round)*oceanEscalation, maximumOcean)
		cellCount := int(math.Ceil(float64(config.ProvinceCount) / (1 - ocean)))
		random := roundRandom(config.WorldSeed, round)
		mesh, err := singlemesh.Build(cellCount, config.Relaxations, bounds, random)
		if err != nil {
			return Result{}, fmt.Errorf("round %d mesh: %w", round+1, err)
		}
		edgeValues, landEligible, edgeDistances := edgeField(mesh, bounds, config.EdgeBarrierWidth, config.EdgeRamp)
		var attractants []Attractant
		var skips []AttractantSkip
		if constellation, ok := ConstellationByName(config.Constellation); ok {
			attractants, skips = makeConstellationAttractants(mesh, bounds, edgeDistances, config.EdgeRamp, config.AttractantRamp, constellation)
		} else {
			attractants, skips = makeAttractants(mesh, bounds, edgeDistances, config.EdgeRamp, config.AttractantRamp, config.AttractantCount, config.AttractantJitter, random)
		}
		desirability := desirabilityField(mesh, edgeValues, landEligible, attractants, config.AttractantRamp)
		state := newGrowthState(mesh, desirability, landEligible, config.RivalRamp)
		if state.seedAndGrow(config.IslandCount, config.ProvinceCount, config.SoftmaxTemperature, random) {
			result := state.result(attractants, skips, config.IslandCount)
			result.RoundsAttempted = round + 1
			result.FinalOcean = ocean
			return result, nil
		}
	}
	return Result{}, fmt.Errorf("%w after %d rounds", ErrStarved, config.MaxRounds)
}

func makeAttractants(cells []singlemesh.Cell, bounds singlemesh.Bounds, edgeDistances []int, edgeRamp, attractantRamp []float64, count int, jitter float64, random *rand.Rand) ([]Attractant, []AttractantSkip) {
	const regions = 3
	clearance := edgeRampReach(edgeRamp) + attractantRampReach(attractantRamp)
	regionIDs := attractantRegionIDs(count)
	attractants := make([]Attractant, 0, len(regionIDs))
	skips := make([]AttractantSkip, 0)
	for _, regionID := range regionIDs {
		regionX, regionY := regionID%regions, regionID/regions
		regionWidth, regionHeight := bounds.Width/regions, bounds.Height/regions
		center := Point{X: (float64(regionX) + 0.5) * regionWidth, Y: (float64(regionY) + 0.5) * regionHeight}
		target := Point{
			X: center.X + (2*random.Float64()-1)*jitter*regionWidth/2,
			Y: center.Y + (2*random.Float64()-1)*jitter*regionHeight/2,
		}
		bestCell, bestDistance := -1, math.Inf(1)
		for cellID, cell := range cells {
			cellRegionX := min(int(cell.Site.X/bounds.Width*regions), regions-1)
			cellRegionY := min(int(cell.Site.Y/bounds.Height*regions), regions-1)
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
	return attractants, skips
}

// Constellation sites fill this much of the map's width and height, whichever
// binds first, so the figure keeps its proportions and clears the edge ramp.
const (
	constellationWidthFill  = 0.76
	constellationHeightFill = 0.70
)

// makeConstellationAttractants scales the constellation's centered frame to
// fit the map and places each site on the nearest cell that clears the edge
// barrier by the combined ramp reach. Sites keep the constellation's order.
// A site is skipped only when no cell on the map has that clearance.
func makeConstellationAttractants(cells []singlemesh.Cell, bounds singlemesh.Bounds, edgeDistances []int, edgeRamp, attractantRamp []float64, constellation Constellation) ([]Attractant, []AttractantSkip) {
	const regions = 3
	clearance := edgeRampReach(edgeRamp) + attractantRampReach(attractantRamp)
	maxX, maxY := 0.0, 0.0
	for _, site := range constellation.Sites {
		maxX = max(maxX, math.Abs(site.X))
		maxY = max(maxY, math.Abs(site.Y))
	}
	scale := math.Inf(1)
	if maxX > 0 {
		scale = min(scale, constellationWidthFill*bounds.Width/2/maxX)
	}
	if maxY > 0 {
		scale = min(scale, constellationHeightFill*bounds.Height/2/maxY)
	}
	if math.IsInf(scale, 1) {
		scale = 0
	}
	attractants := make([]Attractant, 0, len(constellation.Sites))
	skips := make([]AttractantSkip, 0)
	for _, site := range constellation.Sites {
		target := Point{X: bounds.Width/2 + site.X*scale, Y: bounds.Height/2 + site.Y*scale}
		regionX := min(int(target.X/bounds.Width*regions), regions-1)
		regionY := min(int(target.Y/bounds.Height*regions), regions-1)
		bestCell, bestDistance := -1, math.Inf(1)
		for cellID, cell := range cells {
			if edgeDistances[cellID] <= clearance {
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
	return attractants, skips
}

func validAttractantCount(count int) bool {
	switch count {
	case 0, 1, 2, 3, 4, 5, 6, 9:
		return true
	default:
		return false
	}
}

func attractantRegionIDs(count int) []int {
	switch count {
	case 0:
		return nil
	case 1:
		return []int{4}
	case 2:
		return []int{0, 8}
	case 3:
		return []int{0, 4, 8}
	case 4:
		return []int{0, 2, 6, 8}
	case 5:
		return []int{0, 2, 4, 6, 8}
	case 6:
		return []int{0, 2, 3, 5, 6, 8}
	case 9:
		return []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	default:
		return nil
	}
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

func edgeField(cells []singlemesh.Cell, bounds singlemesh.Bounds, barrierWidth float64, ramp []float64) ([]float64, []bool, []int) {
	values := make([]float64, len(cells))
	landEligible := make([]bool, len(cells))
	distances := make([]int, len(cells))
	queue := make([]int, 0)
	for cellID, cell := range cells {
		distances[cellID] = -1
		landEligible[cellID] = true
		if cellInBarrier(cell, bounds, barrierWidth) {
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

func cellInBarrier(cell singlemesh.Cell, bounds singlemesh.Bounds, width float64) bool {
	for _, point := range cell.Corners {
		if point.X <= width+boundaryEpsilon || point.X >= bounds.Width-width-boundaryEpsilon ||
			point.Y <= width+boundaryEpsilon || point.Y >= bounds.Height-width-boundaryEpsilon {
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
	cells        []singlemesh.Cell
	desirability []float64
	landEligible []bool
	owners       []int
	rivals       rivalField
	islands      []islandState
	frontiers    []randomSet
	mergeCount   int
}

func newGrowthState(cells []singlemesh.Cell, desirability []float64, landEligible []bool, rivalRamp []float64) *growthState {
	owners := make([]int, len(cells))
	for i := range cells {
		owners[i] = Water
	}
	return &growthState{
		cells:        cells,
		desirability: desirability,
		landEligible: landEligible,
		owners:       owners,
		rivals:       newRivalField(cells, rivalRamp),
	}
}

// noRival marks a cell with no island land within the rival ramp's reach.
const noRival = math.MaxInt

// rivalField tracks, for every cell, the hop distance to the nearest land of
// each island, compressed to the nearest island and the nearest land of any
// other island. That is enough to answer "how far is this cell from land that
// is not mine?" for any island in constant time. Distances beyond the ramp's
// reach are not tracked because they carry no penalty.
type rivalField struct {
	cells      []singlemesh.Cell
	ramp       []float64
	reach      int
	nearest    []int // island whose land is nearest, or Water
	nearestHop []int // hops to that land, or noRival
	otherHop   []int // hops to the nearest land of any island other than nearest, or noRival
	visited    []int // BFS stamp per cell
	hops       []int // BFS depth per cell, valid when visited matches stamp
	stamp      int
	queue      []int
}

func newRivalField(cells []singlemesh.Cell, ramp []float64) rivalField {
	field := rivalField{
		cells:      cells,
		ramp:       ramp,
		reach:      lastNonzeroIndex(ramp) + 1,
		nearest:    make([]int, len(cells)),
		nearestHop: make([]int, len(cells)),
		otherHop:   make([]int, len(cells)),
		visited:    make([]int, len(cells)),
		hops:       make([]int, len(cells)),
	}
	field.reset()
	return field
}

func (f *rivalField) reset() {
	for i := range f.cells {
		f.nearest[i] = Water
		f.nearestHop[i] = noRival
		f.otherHop[i] = noRival
	}
}

// penalty returns the ramp value islandID sees on cellID, by hop distance to
// the nearest land it does not own, and false when that land is beyond the
// ramp's reach.
func (f *rivalField) penalty(cellID, islandID int) (float64, bool) {
	hop := f.nearestHop[cellID]
	if f.nearest[cellID] == islandID {
		hop = f.otherHop[cellID]
	}
	if hop < 1 || hop > f.reach {
		return 0, false
	}
	return f.ramp[hop-1], true
}

// claim records that islandID now owns cellID and propagates the new land's
// distance to every cell within the ramp's reach.
func (f *rivalField) claim(cellID, islandID int) {
	f.stamp++
	f.queue = append(f.queue[:0], cellID)
	f.visited[cellID] = f.stamp
	f.hops[cellID] = 0
	for head := 0; head < len(f.queue); head++ {
		current := f.queue[head]
		hop := f.hops[current]
		f.record(current, islandID, hop)
		if hop >= f.reach {
			continue
		}
		for _, neighbor := range f.cells[current].Neighbors {
			if f.visited[neighbor] == f.stamp {
				continue
			}
			f.visited[neighbor] = f.stamp
			f.hops[neighbor] = hop + 1
			f.queue = append(f.queue, neighbor)
		}
	}
}

func (f *rivalField) record(cellID, islandID, hop int) {
	switch {
	case f.nearest[cellID] == Water:
		f.nearest[cellID], f.nearestHop[cellID] = islandID, hop
	case f.nearest[cellID] == islandID:
		f.nearestHop[cellID] = min(f.nearestHop[cellID], hop)
	case hop < f.nearestHop[cellID]:
		f.otherHop[cellID] = f.nearestHop[cellID]
		f.nearest[cellID], f.nearestHop[cellID] = islandID, hop
	default:
		f.otherHop[cellID] = min(f.otherHop[cellID], hop)
	}
}

// rebuild recomputes every distance from the current owners. Merges change
// island identity, which the two-entry compression cannot relabel in place.
func (f *rivalField) rebuild(owners []int) {
	f.reset()
	for cellID, owner := range owners {
		if owner != Water {
			f.claim(cellID, owner)
		}
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
		for _, loser := range s.claim(cellID, islandID) {
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

// visibleValue is the static desirability, or the rival ramp value for the
// cell's hop distance to the nearest land islandID does not own, whichever is
// harsher. Cells beyond the ramp's reach keep the static field, so an
// attractant cannot cancel the penalty on a cell whose claim would merge.
func (s *growthState) visibleValue(cellID, islandID int) float64 {
	value := s.desirability[cellID]
	if penalty, ok := s.rivals.penalty(cellID, islandID); ok {
		value = min(value, penalty)
	}
	return value
}

// claim returns every island absorbed because its claimed land directly
// touches cellID. The rival ramp affects desirability, but never decides
// connectivity.
func (s *growthState) claim(cellID, islandID int) []int {
	losers := make([]int, 0)
	seen := make(map[int]bool)
	for _, neighbor := range s.cells[cellID].Neighbors {
		owner := s.owners[neighbor]
		if owner != Water && owner != islandID && !seen[owner] {
			seen[owner] = true
			losers = append(losers, owner)
		}
	}

	s.owners[cellID] = islandID
	s.rivals.claim(cellID, islandID)
	s.islands[islandID].cellIDs = append(s.islands[islandID].cellIDs, cellID)
	for i := range s.frontiers {
		s.frontiers[i].remove(cellID)
	}
	for _, neighbor := range s.cells[cellID].Neighbors {
		if s.owners[neighbor] != Water || !s.landEligible[neighbor] {
			continue
		}
		s.frontiers[islandID].add(neighbor)
	}
	for _, loser := range losers {
		s.absorb(islandID, loser)
	}
	if len(losers) > 0 {
		s.rivals.rebuild(s.owners)
	}
	return losers
}

func (s *growthState) absorb(winner, loser int) {
	for cellID := range s.cells {
		if s.owners[cellID] == loser {
			s.owners[cellID] = winner
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
		owner := s.owners[cellID]
		if owner != Water {
			owner = canonicalIDs[owner]
		}
		cells[cellID] = Cell{
			ID:           cell.ID,
			Site:         cell.Site,
			Corners:      cell.Corners,
			Neighbors:    cell.Neighbors,
			LandEligible: s.landEligible[cellID],
			Desirability: s.desirability[cellID],
			IslandID:     owner,
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
