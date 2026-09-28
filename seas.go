package wgvc

import (
	"slices"
	"sort"
)

const (
	// seaZoneTargetSize is the number of ocean provinces per sea zone seed.
	seaZoneTargetSize = 200
	// maxStraitWidth is the longest water path, in provinces, that counts as
	// a strait between two islands.
	maxStraitWidth = 3
	// maxNeckWidth is the longest land crossing, in provinces, that counts
	// as a neck.
	maxNeckWidth = 3
	// neckMinimumRegion is the smallest land region a neck may join.
	neckMinimumRegion = 10
)

// provinceNeighbors is the undirected province adjacency from interior edges,
// each list in ascending order.
func provinceNeighbors(world *World) [][]ProvinceID {
	neighbors := make([][]ProvinceID, len(world.Provinces))
	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) != 2 {
			continue
		}
		first, second := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
		neighbors[first] = append(neighbors[first], second)
		neighbors[second] = append(neighbors[second], first)
	}
	for _, list := range neighbors {
		slices.Sort(list)
	}
	return neighbors
}

// assignCoastDistances runs one multi-source BFS per medium from the
// provinces that share an edge with the other medium. A province that cannot
// reach a coast through its own medium keeps -1, which no generated world
// produces. The pass consumes no randomness.
func assignCoastDistances(world *World) {
	neighbors := provinceNeighbors(world)
	isLand := func(provinceID ProvinceID) bool { return world.Provinces[provinceID].IslandID != NoIslandID }
	var queue []ProvinceID
	for provinceID := range world.Provinces {
		province := &world.Provinces[provinceID]
		province.CoastDistance = -1
		for _, neighborID := range neighbors[provinceID] {
			if isLand(neighborID) != isLand(province.ID) {
				province.CoastDistance = 0
				queue = append(queue, province.ID)
				break
			}
		}
	}
	for next := 0; next < len(queue); next++ {
		provinceID := queue[next]
		for _, neighborID := range neighbors[provinceID] {
			if world.Provinces[neighborID].CoastDistance < 0 && isLand(neighborID) == isLand(provinceID) {
				world.Provinces[neighborID].CoastDistance = world.Provinces[provinceID].CoastDistance + 1
				queue = append(queue, neighborID)
			}
		}
	}
}

// assignSeaZones partitions the ocean provinces into contiguous zones of
// roughly seaZoneTargetSize provinces. Seeds come from farthest-point
// sampling over water adjacency: the lowest ocean province ID first, then
// repeatedly the ocean province farthest from every seed so far (unreached
// before reached, then lower ID), one seed per seaZoneTargetSize provinces
// and at least one per ocean component. A layered multi-source BFS then
// gives each ocean province its nearest seed, ties to the lower seed, which
// keeps every zone connected. A zone that still holds more than twice the
// target is partitioned again within itself. Zones are ordered by lowest
// member ID. The pass consumes no randomness.
func assignSeaZones(world *World) {
	neighbors := provinceNeighbors(world)
	world.SeaZones = nil
	var ocean []ProvinceID
	for provinceID := range world.Provinces {
		province := &world.Provinces[provinceID]
		province.SeaZoneID = NoSeaZoneID
		if province.IslandID == NoIslandID && province.BasinID == NoBasinID {
			ocean = append(ocean, province.ID)
		}
	}
	if len(ocean) == 0 {
		return
	}
	zones := partitionWater(ocean, neighbors, len(world.Provinces))
	sort.Slice(zones, func(i, j int) bool { return zones[i].ProvinceIDs[0] < zones[j].ProvinceIDs[0] })
	world.SeaZones = zones
	for index := range world.SeaZones {
		zone := &world.SeaZones[index]
		zone.ID = SeaZoneID(index)
		for _, provinceID := range zone.ProvinceIDs {
			world.Provinces[provinceID].SeaZoneID = zone.ID
		}
	}
}

// partitionWater splits members into zones by farthest-point seeds and
// nearest-seed assignment, recursing into any zone larger than twice
// seaZoneTargetSize. Zones come back with members ascending and IDs unset.
func partitionWater(members []ProvinceID, neighbors [][]ProvinceID, provinceCount int) []SeaZone {
	const unreached = -1
	inSet := make([]bool, provinceCount)
	for _, provinceID := range members {
		inSet[provinceID] = true
	}
	distance := make([]int, provinceCount)
	for i := range distance {
		distance[i] = unreached
	}
	seedTarget := max(1, (len(members)+seaZoneTargetSize-1)/seaZoneTargetSize)
	var seeds []ProvinceID
	for {
		seed := members[0]
		for _, provinceID := range members[1:] {
			switch {
			case distance[provinceID] == unreached && distance[seed] != unreached:
				seed = provinceID
			case distance[provinceID] != unreached && distance[seed] != unreached && distance[provinceID] > distance[seed]:
				seed = provinceID
			}
		}
		if distance[seed] != unreached && len(seeds) >= seedTarget {
			break
		}
		seeds = append(seeds, seed)
		distance[seed] = 0
		queue := []ProvinceID{seed}
		for next := 0; next < len(queue); next++ {
			provinceID := queue[next]
			for _, neighborID := range neighbors[provinceID] {
				if inSet[neighborID] && (distance[neighborID] == unreached || distance[neighborID] > distance[provinceID]+1) {
					distance[neighborID] = distance[provinceID] + 1
					queue = append(queue, neighborID)
				}
			}
		}
	}

	// Layered assignment: a province at layer d takes the lowest seed among
	// its neighbors at layer d-1, so each zone is connected.
	layer := make([]int, provinceCount)
	zoneOf := make([]int, provinceCount)
	for i := range layer {
		layer[i] = unreached
		zoneOf[i] = -1
	}
	var frontier []ProvinceID
	for index, seed := range seeds {
		layer[seed] = 0
		zoneOf[seed] = index
		frontier = append(frontier, seed)
	}
	for depth := 1; len(frontier) > 0; depth++ {
		var nextFrontier []ProvinceID
		for _, provinceID := range frontier {
			for _, neighborID := range neighbors[provinceID] {
				if inSet[neighborID] && layer[neighborID] == unreached {
					layer[neighborID] = depth
					nextFrontier = append(nextFrontier, neighborID)
				}
			}
		}
		for _, provinceID := range nextFrontier {
			for _, neighborID := range neighbors[provinceID] {
				if inSet[neighborID] && layer[neighborID] == depth-1 && (zoneOf[provinceID] < 0 || zoneOf[neighborID] < zoneOf[provinceID]) {
					zoneOf[provinceID] = zoneOf[neighborID]
				}
			}
		}
		frontier = nextFrontier
	}
	zones := make([]SeaZone, len(seeds))
	for index, seed := range seeds {
		zones[index].CenterProvinceID = seed
	}
	for _, provinceID := range members {
		zone := &zones[zoneOf[provinceID]]
		zone.ProvinceIDs = append(zone.ProvinceIDs, provinceID)
	}
	var result []SeaZone
	for _, zone := range zones {
		if len(seeds) > 1 && len(zone.ProvinceIDs) > 2*seaZoneTargetSize {
			result = append(result, partitionWater(zone.ProvinceIDs, neighbors, provinceCount)...)
		} else {
			result = append(result, zone)
		}
	}
	return result
}

// assignStraits finds narrow water crossings between islands. From each
// island's coastal water a bounded water BFS records, per water province, the
// number of water provinces on the shortest path to that island including
// itself. A water province is narrow for islands A and B when those two
// counts sum to maxStraitWidth + 1. Narrow provinces of one island pair that
// touch by water form a strait, ordered by island pair and then lowest
// province ID. The pass consumes no randomness.
func assignStraits(world *World) {
	neighbors := provinceNeighbors(world)
	world.Straits = nil
	isWater := func(provinceID ProvinceID) bool { return world.Provinces[provinceID].IslandID == NoIslandID }
	type reach struct {
		island   IslandID
		distance int
	}
	reaches := make([][]reach, len(world.Provinces))
	for _, island := range world.Islands {
		distance := make(map[ProvinceID]int)
		var queue []ProvinceID
		for _, landID := range island.ProvinceIDs {
			for _, neighborID := range neighbors[landID] {
				if _, seen := distance[neighborID]; isWater(neighborID) && !seen {
					distance[neighborID] = 1
					queue = append(queue, neighborID)
				}
			}
		}
		slices.Sort(queue)
		for next := 0; next < len(queue); next++ {
			provinceID := queue[next]
			reaches[provinceID] = append(reaches[provinceID], reach{island.ID, distance[provinceID]})
			if distance[provinceID] >= maxStraitWidth {
				continue
			}
			for _, neighborID := range neighbors[provinceID] {
				if _, seen := distance[neighborID]; isWater(neighborID) && !seen {
					distance[neighborID] = distance[provinceID] + 1
					queue = append(queue, neighborID)
				}
			}
		}
	}

	type pair [2]IslandID
	narrow := make(map[pair]map[ProvinceID]int) // width through each narrow province
	for provinceID, list := range reaches {
		for i, first := range list {
			for _, second := range list[i+1:] {
				width := first.distance + second.distance - 1
				if width > maxStraitWidth {
					continue
				}
				key := pair{min(first.island, second.island), max(first.island, second.island)}
				if narrow[key] == nil {
					narrow[key] = make(map[ProvinceID]int)
				}
				narrow[key][ProvinceID(provinceID)] = width
			}
		}
	}
	keys := make([]pair, 0, len(narrow))
	for key := range narrow {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	for _, key := range keys {
		cells := narrow[key]
		starts := make([]ProvinceID, 0, len(cells))
		for provinceID := range cells {
			starts = append(starts, provinceID)
		}
		slices.Sort(starts)
		visited := make(map[ProvinceID]bool, len(cells))
		for _, start := range starts {
			if visited[start] {
				continue
			}
			visited[start] = true
			members := []ProvinceID{start}
			for next := 0; next < len(members); next++ {
				for _, neighborID := range neighbors[members[next]] {
					if _, narrowCell := cells[neighborID]; narrowCell && !visited[neighborID] {
						visited[neighborID] = true
						members = append(members, neighborID)
					}
				}
			}
			slices.Sort(members)
			strait := Strait{ID: StraitID(len(world.Straits)), IslandIDs: key, Width: maxStraitWidth, ProvinceIDs: members}
			for _, memberID := range members {
				strait.Width = min(strait.Width, cells[memberID])
				for _, neighborID := range neighbors[memberID] {
					for side, islandID := range key {
						if world.Provinces[neighborID].IslandID == islandID && !slices.Contains(strait.Shores[side], neighborID) {
							strait.Shores[side] = append(strait.Shores[side], neighborID)
						}
					}
				}
			}
			slices.Sort(strait.Shores[0])
			slices.Sort(strait.Shores[1])
			world.Straits = append(world.Straits, strait)
		}
	}
}

// assignNecks finds narrow isthmuses as small vertex cuts of each island's
// land graph. A width-1 neck is an articulation point; wider necks are paths
// of two or three adjacent land provinces whose ends touch water, tested by
// removal. A cut counts when the two largest regions it leaves each have at
// least neckMinimumRegion provinces and no smaller subset of it is already a
// neck. Cuts that share or touch a province merge into one neck, so a long
// corridor one province wide is reported once; the merged neck's Ends and
// EndSizes describe the regions left when all of it is removed. Necks are
// ordered by island and then lowest province ID. The pass consumes no
// randomness.
func assignNecks(world *World) {
	neighbors := provinceNeighbors(world)
	world.Necks = nil
	scratch := newNeckScratch(len(world.Provinces))
	for _, island := range world.Islands {
		cuts := islandNeckCuts(world, neighbors, island, scratch)
		for _, group := range mergeTouchingCuts(cuts, neighbors) {
			neck, ok := neckForCandidate(world, neighbors, group.members, false, scratch)
			if !ok {
				continue
			}
			neck.ID = NeckID(len(world.Necks))
			neck.Width = group.width
			world.Necks = append(world.Necks, neck)
		}
	}
}

type neckCut struct {
	members []ProvinceID // ascending
	width   int
}

// neckScratch holds per-province work arrays reused across cut tests. A
// stamp equal to the current generation marks a province as touched in the
// current search, so no array needs clearing between searches.
type neckScratch struct {
	generation int
	stamp      []int
	region     []int
	removed    []int
	queue      []ProvinceID
}

func newNeckScratch(provinceCount int) *neckScratch {
	return &neckScratch{
		stamp:   make([]int, provinceCount),
		region:  make([]int, provinceCount),
		removed: make([]int, provinceCount),
	}
}

// islandNeckCuts lists the minimal cuts of width 1 to maxNeckWidth on one
// island, each with members ascending, ordered by lowest member.
func islandNeckCuts(world *World, neighbors [][]ProvinceID, island Island, scratch *neckScratch) []neckCut {
	onIsland := func(provinceID ProvinceID) bool { return world.Provinces[provinceID].IslandID == island.ID }
	var cuts []neckCut
	isCut := map[[maxNeckWidth]ProvinceID]bool{}
	key := func(members []ProvinceID) [maxNeckWidth]ProvinceID {
		var k [maxNeckWidth]ProvinceID
		for i := range k {
			k[i] = -1
		}
		copy(k[:], members)
		return k
	}
	record := func(members []ProvinceID, width int) {
		cuts = append(cuts, neckCut{members: members, width: width})
		isCut[key(members)] = true
	}

	for _, provinceID := range articulationNecks(world, neighbors, island) {
		record([]ProvinceID{provinceID}, 1)
	}
	if maxNeckWidth < 2 {
		return cuts
	}
	coastal := func(provinceID ProvinceID) bool { return world.Provinces[provinceID].CoastDistance == 0 }
	test := func(members []ProvinceID, width int) {
		for _, memberID := range members {
			if isCut[key([]ProvinceID{memberID})] {
				return
			}
		}
		if width == 3 {
			for i := range members {
				for j := i + 1; j < len(members); j++ {
					if isCut[key([]ProvinceID{members[i], members[j]})] {
						return
					}
				}
			}
		}
		if isCut[key(members)] {
			return
		}
		if _, ok := neckForCandidate(world, neighbors, members, true, scratch); ok {
			record(members, width)
		}
	}
	for _, first := range island.ProvinceIDs {
		if !coastal(first) {
			continue
		}
		for _, second := range neighbors[first] {
			if !onIsland(second) {
				continue
			}
			if coastal(second) && second > first {
				test([]ProvinceID{first, second}, 2)
			}
			if maxNeckWidth < 3 {
				continue
			}
			for _, third := range neighbors[second] {
				if third == first || !onIsland(third) || !coastal(third) || third < first {
					continue
				}
				members := []ProvinceID{first, second, third}
				slices.Sort(members)
				test(members, 3)
			}
		}
	}
	sort.Slice(cuts, func(i, j int) bool {
		if cuts[i].members[0] != cuts[j].members[0] {
			return cuts[i].members[0] < cuts[j].members[0]
		}
		return cuts[i].width < cuts[j].width
	})
	return cuts
}

// articulationNecks returns the island's articulation points whose removal
// leaves two regions of at least neckMinimumRegion provinces, ascending.
func articulationNecks(world *World, neighbors [][]ProvinceID, island Island) []ProvinceID {
	onIsland := func(provinceID ProvinceID) bool { return world.Provinces[provinceID].IslandID == island.ID }
	discovered := make([]int, len(world.Provinces))
	low := make([]int, len(world.Provinces))
	subtree := make([]int, len(world.Provinces))
	for i := range discovered {
		discovered[i] = -1
	}
	clock := 0
	var necks []ProvinceID
	total := len(island.ProvinceIDs)
	var visit func(provinceID, parent ProvinceID, root bool)
	visit = func(provinceID, parent ProvinceID, root bool) {
		discovered[provinceID] = clock
		low[provinceID] = clock
		clock++
		subtree[provinceID] = 1
		var split []int
		for _, neighborID := range neighbors[provinceID] {
			if !onIsland(neighborID) || neighborID == parent {
				continue
			}
			if discovered[neighborID] >= 0 {
				low[provinceID] = min(low[provinceID], discovered[neighborID])
				continue
			}
			visit(neighborID, provinceID, false)
			subtree[provinceID] += subtree[neighborID]
			low[provinceID] = min(low[provinceID], low[neighborID])
			if root || low[neighborID] >= discovered[provinceID] {
				split = append(split, subtree[neighborID])
			}
		}
		if len(split) == 0 {
			return
		}
		rest := total - 1
		for _, size := range split {
			rest -= size
		}
		if !root {
			split = append(split, rest)
		}
		if len(split) < 2 {
			return
		}
		sort.Sort(sort.Reverse(sort.IntSlice(split)))
		if split[1] >= neckMinimumRegion {
			necks = append(necks, provinceID)
		}
	}
	if len(island.ProvinceIDs) > 0 {
		visit(island.ProvinceIDs[0], -1, true)
	}
	slices.Sort(necks)
	return necks
}

// mergeTouchingCuts unions cuts that share or neighbor a province.
func mergeTouchingCuts(cuts []neckCut, neighbors [][]ProvinceID) []neckCut {
	owner := map[ProvinceID]int{}
	parent := make([]int, len(cuts))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for index, cut := range cuts {
		for _, memberID := range cut.members {
			candidates := append([]ProvinceID{memberID}, neighbors[memberID]...)
			for _, nearID := range candidates {
				if other, ok := owner[nearID]; ok {
					parent[find(index)] = find(other)
				}
			}
		}
		for _, memberID := range cut.members {
			owner[memberID] = index
		}
	}
	groups := map[int]*neckCut{}
	var order []int
	for index, cut := range cuts {
		root := find(index)
		group, ok := groups[root]
		if !ok {
			group = &neckCut{width: cut.width}
			groups[root] = group
			order = append(order, root)
		}
		group.width = min(group.width, cut.width)
		for _, memberID := range cut.members {
			if !slices.Contains(group.members, memberID) {
				group.members = append(group.members, memberID)
			}
		}
	}
	merged := make([]neckCut, 0, len(order))
	for _, root := range order {
		slices.Sort(groups[root].members)
		merged = append(merged, *groups[root])
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].members[0] < merged[j].members[0] })
	return merged
}

// neckForCandidate removes the candidate from its island and describes the
// regions left. With requireMinimum the two largest regions must each have
// neckMinimumRegion provinces; otherwise two regions suffice. A bounded
// search from the candidate's neighbors runs first, so a candidate whose
// neighbors reconnect nearby is rejected without touching the whole island.
func neckForCandidate(world *World, neighbors [][]ProvinceID, members []ProvinceID, requireMinimum bool, scratch *neckScratch) (Neck, bool) {
	const localReach = 10
	islandID := world.Provinces[members[0]].IslandID
	scratch.generation++
	generation := scratch.generation
	for _, memberID := range members {
		scratch.removed[memberID] = generation
	}
	open := func(provinceID ProvinceID) bool {
		return world.Provinces[provinceID].IslandID == islandID && scratch.removed[provinceID] != generation
	}
	var boundary []ProvinceID
	for _, memberID := range members {
		for _, neighborID := range neighbors[memberID] {
			if open(neighborID) && !slices.Contains(boundary, neighborID) {
				boundary = append(boundary, neighborID)
			}
		}
	}
	if len(boundary) < 2 {
		return Neck{}, false
	}
	slices.Sort(boundary)

	// Bounded search from the first boundary province.
	queue := scratch.queue[:0]
	depth := scratch.region // reused as the depth array for this search
	scratch.stamp[boundary[0]] = generation
	depth[boundary[0]] = 0
	queue = append(queue, boundary[0])
	for next := 0; next < len(queue); next++ {
		provinceID := queue[next]
		if depth[provinceID] >= localReach {
			continue
		}
		for _, neighborID := range neighbors[provinceID] {
			if open(neighborID) && scratch.stamp[neighborID] != generation {
				scratch.stamp[neighborID] = generation
				depth[neighborID] = depth[provinceID] + 1
				queue = append(queue, neighborID)
			}
		}
	}
	scratch.queue = queue
	connected := true
	for _, provinceID := range boundary[1:] {
		if scratch.stamp[provinceID] != generation {
			connected = false
			break
		}
	}
	if connected {
		return Neck{}, false
	}

	// Full region labelling of the island without the candidate.
	scratch.generation++
	generation = scratch.generation
	for _, memberID := range members {
		scratch.removed[memberID] = generation
	}
	region := scratch.region
	var regionSizes []int
	for _, landID := range world.Islands[islandID].ProvinceIDs {
		if scratch.stamp[landID] == generation || !open(landID) {
			continue
		}
		regionIndex := len(regionSizes)
		scratch.stamp[landID] = generation
		region[landID] = regionIndex
		queue = scratch.queue[:0]
		queue = append(queue, landID)
		for next := 0; next < len(queue); next++ {
			for _, neighborID := range neighbors[queue[next]] {
				if open(neighborID) && scratch.stamp[neighborID] != generation {
					scratch.stamp[neighborID] = generation
					region[neighborID] = regionIndex
					queue = append(queue, neighborID)
				}
			}
		}
		scratch.queue = queue
		regionSizes = append(regionSizes, len(queue))
	}
	if len(regionSizes) < 2 {
		return Neck{}, false
	}
	order := make([]int, len(regionSizes))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return regionSizes[order[i]] > regionSizes[order[j]] })
	first, second := order[0], order[1]
	if requireMinimum && regionSizes[second] < neckMinimumRegion {
		return Neck{}, false
	}
	neck := Neck{IslandID: islandID, Width: len(members), ProvinceIDs: members, EndSizes: [2]int{regionSizes[first], regionSizes[second]}}
	for _, memberID := range members {
		for _, neighborID := range neighbors[memberID] {
			if !open(neighborID) {
				continue
			}
			for side, want := range [2]int{first, second} {
				if region[neighborID] == want && !slices.Contains(neck.Ends[side], neighborID) {
					neck.Ends[side] = append(neck.Ends[side], neighborID)
				}
			}
		}
	}
	slices.Sort(neck.Ends[0])
	slices.Sort(neck.Ends[1])
	return neck, true
}
