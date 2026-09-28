package wgvc

import (
	"container/heap"
	"sort"
)

// River class thresholds on discharge, in moisture-weighted province-area
// units: a province of mean area and mean moisture contributes about half a
// unit, so a stream drains roughly eight provinces and a river thirty. The
// thresholds are absolute because a province is a fixed real size, so a
// 1,500-province world has a few dozen rivers and no major river, while the
// 10,000-province maps TNYC uses have about a dozen major rivers reaching
// the ocean, several dozen more counting major tributaries and lake
// outflows, and a few hundred rivers in all.
const (
	riverStreamDischarge = 4.0
	riverRiverDischarge  = 16.0
	riverMajorDischarge  = 64.0
)

// ClassifyDischarge is the river class for a discharge, or "" below the
// stream threshold.
func ClassifyDischarge(discharge float64) RiverClass {
	switch {
	case discharge >= riverMajorDischarge:
		return RiverClassMajorRiver
	case discharge >= riverRiverDischarge:
		return RiverClassRiver
	case discharge >= riverStreamDischarge:
		return RiverClassStream
	}
	return ""
}

// assignRivers routes runoff downhill over the corner graph and traces the
// chains that carry enough of it to be rivers.
//
// Every corner touching ocean is an outlet at its own elevation. A
// priority-flood from the outlets visits the remaining corners in ascending
// order of filled elevation, max(own elevation, elevation of the corner it
// was reached from), so each corner gets one downstream neighbor and a
// monotone path to the sea; depressions fill to their spill level instead of
// trapping flow. Heap ties fall to the lower corner ID. A corner's downstream
// neighbor is chosen when the corner is visited, among the neighbors already
// visited: the lowest filled elevation wins; on a tie an ocean outlet beats
// any other corner, then the steeper descent (filled drop over edge length),
// then the shorter edge, then the lower corner ID. Every corner touching
// water sits at elevation 0, so without the outlet preference a lake shore
// could claim a coastal corner from the sea beside it. The filled surface is
// not exported. The first corner of a lake or inland sea the flood reaches is
// the basin's spill corner; every other corner of the basin drains to it
// through the water, so inflowing rivers end at the shore and the basin's
// water leaves at the spill corner alone. Without that, every shore corner
// sits at the spill level with the land around it and picks its own way
// out, and the lake sources several rivers (issue #69).
//
// Each land province spreads Area * Moisture evenly over its corners as
// runoff, which accumulates downstream. An edge between two land provinces
// whose flow reaches riverStreamDischarge is a river edge. Chains are traced
// upstream from each edge that is not the main stem into its downstream
// corner: the main stem is the inflowing river edge with the largest
// discharge, so tributaries end at confluences and every river edge belongs
// to exactly one river.
//
// The pass reads corners, edges, membership, basins, area, and moisture, and
// consumes no randomness.
func assignRivers(world *World) {
	world.Rivers = nil
	for edgeID := range world.Edges {
		world.Edges[edgeID].RiverID = NoRiverID
		world.Edges[edgeID].Discharge = 0
	}
	cornerCount := len(world.Corners)
	if cornerCount == 0 {
		return
	}

	links := make([][]link, cornerCount)
	landEdge := make([]bool, len(world.Edges))
	for _, edge := range world.Edges {
		links[edge.CornerIDs[0]] = append(links[edge.CornerIDs[0]], link{edge.CornerIDs[1], edge.ID})
		links[edge.CornerIDs[1]] = append(links[edge.CornerIDs[1]], link{edge.CornerIDs[0], edge.ID})
		landEdge[edge.ID] = len(edge.ProvinceIDs) == 2 &&
			world.Provinces[edge.ProvinceIDs[0]].IslandID != NoIslandID &&
			world.Provinces[edge.ProvinceIDs[1]].IslandID != NoIslandID
	}

	outlet := make([]bool, cornerCount)
	basinOf := make([]BasinID, cornerCount)
	runoff := make([]float64, cornerCount)
	for cornerID := range basinOf {
		basinOf[cornerID] = NoBasinID
	}
	for _, province := range world.Provinces {
		water := province.IslandID == NoIslandID
		share := 0.0
		if !water {
			share = province.Area * province.Moisture / float64(len(province.CornerIDs))
		}
		for _, cornerID := range province.CornerIDs {
			runoff[cornerID] += share
			if water && province.BasinID == NoBasinID {
				outlet[cornerID] = true
			}
			if water && province.BasinID != NoBasinID {
				basinOf[cornerID] = province.BasinID
			}
		}
	}

	// Priority-flood from the outlets.
	filled := make([]float64, cornerCount)
	downstream := make([]CornerID, cornerCount)
	downstreamEdge := make([]EdgeID, cornerCount)
	visited := make([]bool, cornerCount)
	order := make([]CornerID, 0, cornerCount)
	queue := &cornerHeap{}
	for cornerID := range world.Corners {
		downstream[cornerID] = -1
		downstreamEdge[cornerID] = -1
		if outlet[cornerID] {
			filled[cornerID] = world.Corners[cornerID].Elevation
			visited[cornerID] = true
			heap.Push(queue, cornerHeapItem{corner: CornerID(cornerID), filled: filled[cornerID]})
		}
	}
	basinCorners := make(map[BasinID][]CornerID)
	for cornerID, basinID := range basinOf {
		if basinID != NoBasinID {
			basinCorners[basinID] = append(basinCorners[basinID], CornerID(cornerID))
		}
	}
	spill := make(map[BasinID]CornerID, len(basinCorners))
	resolved := make([]bool, cornerCount)
	for queue.Len() > 0 {
		item := heap.Pop(queue).(cornerHeapItem)
		order = append(order, item.corner)
		resolved[item.corner] = true
		if basinID := basinOf[item.corner]; basinID != NoBasinID && !outlet[item.corner] {
			if spillCorner, spilled := spill[basinID]; spilled {
				// The basin's water leaves at its spill corner, so every
				// other corner of the basin drains there through the water
				// rather than choosing its own way onto the land.
				downstream[item.corner] = spillCorner
			} else {
				// The first corner of a basin reached is its spill corner.
				// The rest of the basin sits at the same filled level and
				// is queued now so inflowing land drains to the shore.
				spill[basinID] = item.corner
				for _, cornerID := range basinCorners[basinID] {
					if !visited[cornerID] {
						visited[cornerID] = true
						filled[cornerID] = max(world.Corners[cornerID].Elevation, item.filled)
						heap.Push(queue, cornerHeapItem{corner: cornerID, filled: filled[cornerID]})
					}
				}
			}
		}
		if !outlet[item.corner] && downstream[item.corner] < 0 {
			// The neighbor that first reached this corner has the lowest
			// filled elevation of any neighbor, so the choice below is only
			// among neighbors tied with it.
			for _, candidate := range links[item.corner] {
				if !resolved[candidate.corner] {
					continue
				}
				if current := downstream[item.corner]; current < 0 ||
					betterDownstream(world, item.corner, candidate, link{current, downstreamEdge[item.corner]}, filled, outlet) {
					downstream[item.corner] = candidate.corner
					downstreamEdge[item.corner] = candidate.edge
				}
			}
		}
		for _, next := range links[item.corner] {
			if visited[next.corner] {
				continue
			}
			visited[next.corner] = true
			filled[next.corner] = max(world.Corners[next.corner].Elevation, filled[item.corner])
			heap.Push(queue, cornerHeapItem{corner: next.corner, filled: filled[next.corner]})
		}
	}

	// Accumulate discharge from the last-visited corner back to the outlets.
	discharge := append([]float64(nil), runoff...)
	for index := len(order) - 1; index >= 0; index-- {
		cornerID := order[index]
		if downstream[cornerID] >= 0 {
			discharge[downstream[cornerID]] += discharge[cornerID]
			if downstreamEdge[cornerID] >= 0 {
				world.Edges[downstreamEdge[cornerID]].Discharge = discharge[cornerID]
			}
		}
	}

	// A river edge is a land edge carrying at least a stream. mainStem[c] is
	// the upstream corner whose river edge into c carries the most flow.
	riverEdge := make([]bool, len(world.Edges))
	mainStem := make([]CornerID, cornerCount)
	for cornerID := range mainStem {
		mainStem[cornerID] = -1
	}
	for _, cornerID := range order {
		edgeID := downstreamEdge[cornerID]
		if edgeID < 0 || !landEdge[edgeID] || discharge[cornerID] < riverStreamDischarge {
			continue
		}
		riverEdge[edgeID] = true
		into := downstream[cornerID]
		if current := mainStem[into]; current < 0 || discharge[cornerID] > discharge[current] ||
			(discharge[cornerID] == discharge[current] && cornerID < current) {
			mainStem[into] = cornerID
		}
	}

	// Trace each river upstream from its last edge.
	type traced struct {
		corners []CornerID
		edges   []EdgeID
		mouth   RiverEnd
		into    CornerID
	}
	var rivers []traced
	for _, cornerID := range order {
		edgeID := downstreamEdge[cornerID]
		if edgeID < 0 || !riverEdge[edgeID] {
			continue
		}
		into := downstream[cornerID]
		var mouth RiverEnd
		switch {
		case outlet[into]:
			mouth = RiverEnd{Kind: RiverEndOcean, BasinID: NoBasinID, RiverID: NoRiverID}
		case basinOf[into] != NoBasinID:
			mouth = RiverEnd{Kind: RiverEndBasin, BasinID: basinOf[into], RiverID: NoRiverID}
		case mainStem[into] == cornerID:
			continue // the chain continues through into
		default:
			mouth = RiverEnd{Kind: RiverEndRiver, BasinID: NoBasinID, RiverID: NoRiverID}
		}
		river := traced{corners: []CornerID{into}, mouth: mouth, into: into}
		for current := cornerID; current >= 0; current = mainStem[current] {
			river.corners = append(river.corners, current)
			river.edges = append(river.edges, downstreamEdge[current])
		}
		reverseCornerIDs(river.corners)
		reverseEdgeIDs(river.edges)
		rivers = append(rivers, river)
	}
	sort.Slice(rivers, func(i, j int) bool {
		first, second := rivers[i], rivers[j]
		firstFlow := world.Edges[first.edges[len(first.edges)-1]].Discharge
		secondFlow := world.Edges[second.edges[len(second.edges)-1]].Discharge
		if firstFlow != secondFlow {
			return firstFlow > secondFlow
		}
		if first.into != second.into {
			return first.into < second.into
		}
		return first.corners[0] < second.corners[0]
	})

	riverAtCorner := make(map[CornerID]RiverID, len(rivers))
	world.Rivers = make([]River, len(rivers))
	for index, river := range rivers {
		riverID := RiverID(index)
		last := river.edges[len(river.edges)-1]
		source := RiverEnd{Kind: RiverEndSpring, BasinID: NoBasinID, RiverID: NoRiverID}
		if basinID := basinOf[river.corners[0]]; basinID != NoBasinID {
			source = RiverEnd{Kind: RiverEndBasin, BasinID: basinID, RiverID: NoRiverID}
		}
		world.Rivers[index] = River{
			ID:        riverID,
			CornerIDs: river.corners,
			EdgeIDs:   river.edges,
			Class:     ClassifyDischarge(world.Edges[last].Discharge),
			Discharge: world.Edges[last].Discharge,
			Source:    source,
			Mouth:     river.mouth,
		}
		for _, edgeID := range river.edges {
			world.Edges[edgeID].RiverID = riverID
		}
		// Every corner but the mouth owns its downstream edge, which is in
		// this river; the mouth corner's downstream edge belongs to the
		// river a tributary joins.
		for _, cornerID := range river.corners[:len(river.corners)-1] {
			riverAtCorner[cornerID] = riverID
		}
	}
	for index := range world.Rivers {
		river := &world.Rivers[index]
		if river.Mouth.Kind == RiverEndRiver {
			river.Mouth.RiverID = riverAtCorner[river.CornerIDs[len(river.CornerIDs)-1]]
		}
	}
}

// link is one edge leaving a corner, seen from that corner.
type link struct {
	corner CornerID
	edge   EdgeID
}

// betterDownstream reports whether candidate is a better downstream neighbor
// for corner than current: lower filled elevation first, then an ocean
// outlet over any other corner, then the steeper descent, then the shorter
// edge, then the lower corner ID.
func betterDownstream(world *World, corner CornerID, candidate, current link, filled []float64, outlet []bool) bool {
	if filled[candidate.corner] != filled[current.corner] {
		return filled[candidate.corner] < filled[current.corner]
	}
	if outlet[candidate.corner] != outlet[current.corner] {
		return outlet[candidate.corner]
	}
	drop := filled[corner] - filled[candidate.corner]
	candidateLength := world.Edges[candidate.edge].Length
	currentLength := world.Edges[current.edge].Length
	if candidateLength > 0 && currentLength > 0 {
		candidateGradient := drop / candidateLength
		currentGradient := drop / currentLength
		if candidateGradient != currentGradient {
			return candidateGradient > currentGradient
		}
	}
	if candidateLength != currentLength {
		return candidateLength < currentLength
	}
	return candidate.corner < current.corner
}

type cornerHeapItem struct {
	corner CornerID
	filled float64
}

type cornerHeap []cornerHeapItem

func (h cornerHeap) Len() int { return len(h) }
func (h cornerHeap) Less(i, j int) bool {
	if h[i].filled != h[j].filled {
		return h[i].filled < h[j].filled
	}
	return h[i].corner < h[j].corner
}
func (h cornerHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *cornerHeap) Push(x any)   { *h = append(*h, x.(cornerHeapItem)) }
func (h *cornerHeap) Pop() any {
	old := *h
	item := old[len(old)-1]
	*h = old[:len(old)-1]
	return item
}

func reverseCornerIDs(values []CornerID) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseEdgeIDs(values []EdgeID) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
