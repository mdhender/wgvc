package wgvc

import (
	"fmt"
	"math"
	"sort"

	voronoi "github.com/pzsz/voronoi"
)

// geometryTolerance is the absolute tolerance used after normalization to
// the unit square. It matches the backend's clipping and closing epsilon.
const geometryTolerance = 1e-9

type backendGeometry struct {
	cells []backendCell
	edges []backendEdge
}

type backendCell struct {
	siteIndex int
	ring      []Point
}

type backendEdge struct {
	ends        [2]Point
	siteIndexes []int
}

// Mesh IDs are local to one island. World-space conversion and assignment of
// public, world-global IDs belong to the later generation integration stage.
type islandMesh struct {
	islandID IslandID
	cells    []meshCell
	corners  []Point
	edges    []meshEdge
}

type meshCell struct {
	siteIndex int
	center    Point
	cornerIDs []int
}

type meshEdge struct {
	cornerIDs   [2]int
	siteIndexes []int
}

func tessellateIslands(plans []islandPlan) (tessellation, error) {
	if len(plans) == 0 {
		return tessellation{}, fmt.Errorf("tessellation requires at least one island")
	}

	result := tessellation{islands: make([]islandMesh, len(plans))}
	for islandIndex, plan := range plans {
		if plan.id != IslandID(islandIndex) {
			return tessellation{}, fmt.Errorf("island %d has non-canonical ID %d", islandIndex, plan.id)
		}
		mesh, err := tessellateIsland(plan.id, plan.provinceCenters)
		if err != nil {
			return tessellation{}, err
		}
		result.islands[islandIndex] = mesh
	}
	return result, nil
}

func tessellateIsland(islandID IslandID, sites []Point) (islandMesh, error) {
	geometry, err := computeBackendGeometry(sites)
	if err != nil {
		return islandMesh{}, fmt.Errorf("island %d: %w", islandID, err)
	}

	mesh, err := canonicalizeGeometry(islandID, sites, geometry)
	if err != nil {
		return islandMesh{}, fmt.Errorf("island %d: %w", islandID, err)
	}
	return mesh, nil
}

// computeBackendGeometry isolates dependency-specific types and restores the
// caller's site order after the backend sorts its copied input.
func computeBackendGeometry(sites []Point) (backendGeometry, error) {
	if len(sites) == 0 {
		return backendGeometry{}, fmt.Errorf("at least one site is required")
	}

	siteIndexes := make(map[Point]int, len(sites))
	backendSites := make([]voronoi.Vertex, len(sites))
	for i, site := range sites {
		if !finitePoint(site) || site.X <= 0 || site.X >= 1 || site.Y <= 0 || site.Y >= 1 {
			return backendGeometry{}, fmt.Errorf("site %d is outside the open finite unit square: %+v", i, site)
		}
		if previous, exists := siteIndexes[site]; exists {
			return backendGeometry{}, fmt.Errorf("sites %d and %d are duplicates", previous, i)
		}
		siteIndexes[site] = i
		backendSites[i] = voronoi.Vertex{X: site.X, Y: site.Y}
	}

	if len(sites) == 1 {
		return oneSiteGeometry(), nil
	}

	// ComputeDiagram sorts its argument in place. A fresh slice protects both
	// caller order and caller-owned storage.
	diagram := voronoi.ComputeDiagram(backendSites, voronoi.NewBBox(0, 1, 0, 1), true)
	if len(diagram.Cells) != len(sites) {
		return backendGeometry{}, fmt.Errorf("backend returned %d cells for %d distinct sites", len(diagram.Cells), len(sites))
	}

	geometry := backendGeometry{cells: make([]backendCell, len(sites))}
	seen := make([]bool, len(sites))
	cellIndexes := make(map[*voronoi.Cell]int, len(diagram.Cells))
	for _, cell := range diagram.Cells {
		index, ok := siteIndexes[Point{X: cell.Site.X, Y: cell.Site.Y}]
		if !ok {
			return backendGeometry{}, fmt.Errorf("backend returned an unknown site: %+v", cell.Site)
		}
		if seen[index] {
			return backendGeometry{}, fmt.Errorf("backend returned site %d more than once", index)
		}
		if len(cell.Halfedges) < 3 {
			return backendGeometry{}, fmt.Errorf("backend returned only %d edges for site %d", len(cell.Halfedges), index)
		}

		ring := make([]Point, len(cell.Halfedges))
		for i, halfedge := range cell.Halfedges {
			start := backendPoint(halfedge.GetStartpoint())
			end := backendPoint(halfedge.GetEndpoint())
			next := backendPoint(cell.Halfedges[(i+1)%len(cell.Halfedges)].GetStartpoint())
			if !pointsNear(end, next) {
				return backendGeometry{}, fmt.Errorf("site %d has an open ring between %+v and %+v", index, end, next)
			}
			ring[i] = start
		}
		if signedArea(ring) < 0 {
			reversePoints(ring)
		}

		geometry.cells[index] = backendCell{siteIndex: index, ring: ring}
		cellIndexes[cell] = index
		seen[index] = true
	}

	geometry.edges = make([]backendEdge, 0, len(diagram.Edges))
	for edgeIndex, edge := range diagram.Edges {
		if edge.LeftCell == nil {
			return backendGeometry{}, fmt.Errorf("backend edge %d has no incident cell", edgeIndex)
		}
		leftIndex, ok := cellIndexes[edge.LeftCell]
		if !ok {
			return backendGeometry{}, fmt.Errorf("backend edge %d refers to an unknown left cell", edgeIndex)
		}
		incidence := []int{leftIndex}
		if edge.RightCell != nil {
			rightIndex, ok := cellIndexes[edge.RightCell]
			if !ok {
				return backendGeometry{}, fmt.Errorf("backend edge %d refers to an unknown right cell", edgeIndex)
			}
			incidence = append(incidence, rightIndex)
			sort.Ints(incidence)
		}
		geometry.edges = append(geometry.edges, backendEdge{
			ends: [2]Point{
				backendPoint(edge.Va.Vertex),
				backendPoint(edge.Vb.Vertex),
			},
			siteIndexes: incidence,
		})
	}

	return geometry, nil
}

func oneSiteGeometry() backendGeometry {
	ring := []Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}
	edges := make([]backendEdge, len(ring))
	for i := range ring {
		edges[i] = backendEdge{
			ends:        [2]Point{ring[i], ring[(i+1)%len(ring)]},
			siteIndexes: []int{0},
		}
	}
	return backendGeometry{
		cells: []backendCell{{siteIndex: 0, ring: ring}},
		edges: edges,
	}
}

func canonicalizeGeometry(islandID IslandID, sites []Point, geometry backendGeometry) (islandMesh, error) {
	if len(geometry.cells) != len(sites) {
		return islandMesh{}, fmt.Errorf("cell count = %d, want %d", len(geometry.cells), len(sites))
	}

	cornerCandidates := make([]Point, 0, 2*len(geometry.edges))
	for edgeIndex, edge := range geometry.edges {
		if !finitePoint(edge.ends[0]) || !finitePoint(edge.ends[1]) {
			return islandMesh{}, fmt.Errorf("edge %d has a non-finite endpoint", edgeIndex)
		}
		if !pointInUnitSquare(edge.ends[0]) || !pointInUnitSquare(edge.ends[1]) {
			return islandMesh{}, fmt.Errorf("edge %d has an endpoint outside the unit square", edgeIndex)
		}
		if pointDistance(edge.ends[0], edge.ends[1]) <= geometryTolerance {
			return islandMesh{}, fmt.Errorf("edge %d has non-positive length within tolerance", edgeIndex)
		}
		if len(edge.siteIndexes) < 1 || len(edge.siteIndexes) > 2 {
			return islandMesh{}, fmt.Errorf("edge %d has %d incident sites", edgeIndex, len(edge.siteIndexes))
		}
		cornerCandidates = append(cornerCandidates, edge.ends[0], edge.ends[1])
	}

	sort.Slice(cornerCandidates, func(i, j int) bool { return pointLess(cornerCandidates[i], cornerCandidates[j]) })
	corners := make([]Point, 0, len(cornerCandidates))
	cornerIDs := make(map[Point]int, len(cornerCandidates))
	for _, point := range cornerCandidates {
		if _, exists := cornerIDs[point]; exists {
			continue
		}
		cornerID := -1
		for candidateID := len(corners) - 1; candidateID >= 0 && point.X-corners[candidateID].X <= geometryTolerance; candidateID-- {
			if pointsNear(point, corners[candidateID]) {
				cornerID = candidateID
				break
			}
		}
		if cornerID == -1 {
			cornerID = len(corners)
			corners = append(corners, point)
		}
		cornerIDs[point] = cornerID
	}

	edges := make([]meshEdge, len(geometry.edges))
	for edgeIndex, edge := range geometry.edges {
		first, firstOK := cornerIDs[edge.ends[0]]
		second, secondOK := cornerIDs[edge.ends[1]]
		if !firstOK || !secondOK {
			return islandMesh{}, fmt.Errorf("edge %d endpoint is missing from canonical corners", edgeIndex)
		}
		if first > second {
			first, second = second, first
		}
		incidence := append([]int(nil), edge.siteIndexes...)
		sort.Ints(incidence)
		for _, siteIndex := range incidence {
			if siteIndex < 0 || siteIndex >= len(sites) {
				return islandMesh{}, fmt.Errorf("edge %d refers to unknown site %d", edgeIndex, siteIndex)
			}
		}
		if len(incidence) == 2 && incidence[0] == incidence[1] {
			return islandMesh{}, fmt.Errorf("edge %d repeats incident site %d", edgeIndex, incidence[0])
		}
		edges[edgeIndex] = meshEdge{cornerIDs: [2]int{first, second}, siteIndexes: incidence}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].cornerIDs[0] != edges[j].cornerIDs[0] {
			return edges[i].cornerIDs[0] < edges[j].cornerIDs[0]
		}
		return edges[i].cornerIDs[1] < edges[j].cornerIDs[1]
	})
	for i := 1; i < len(edges); i++ {
		if edges[i-1].cornerIDs == edges[i].cornerIDs {
			return islandMesh{}, fmt.Errorf("canonical edges %d and %d have the same endpoints", i-1, i)
		}
	}

	edgeByCorners := make(map[[2]int]int, len(edges))
	for edgeID, edge := range edges {
		edgeByCorners[edge.cornerIDs] = edgeID
	}

	cells := make([]meshCell, len(geometry.cells))
	totalArea := 0.0
	edgeTraversals := make([][][2]int, len(edges))
	for cellIndex, cell := range geometry.cells {
		if cell.siteIndex != cellIndex {
			return islandMesh{}, fmt.Errorf("cell %d refers to site %d", cellIndex, cell.siteIndex)
		}
		if len(cell.ring) < 3 {
			return islandMesh{}, fmt.Errorf("site %d cell has only %d corners", cellIndex, len(cell.ring))
		}
		if area := signedArea(cell.ring); area <= geometryTolerance {
			return islandMesh{}, fmt.Errorf("site %d cell has non-positive area %g", cellIndex, area)
		} else {
			totalArea += area
		}

		ring := make([]int, len(cell.ring))
		for ringIndex, point := range cell.ring {
			if !finitePoint(point) || !pointInUnitSquare(point) {
				return islandMesh{}, fmt.Errorf("site %d has an invalid polygon corner %+v", cellIndex, point)
			}
			cornerID, ok := cornerIDs[point]
			if !ok {
				return islandMesh{}, fmt.Errorf("site %d polygon corner %+v is not an edge endpoint", cellIndex, point)
			}
			ring[ringIndex] = cornerID
		}
		rotateIntsToMinimum(ring)
		for ringIndex, first := range ring {
			second := ring[(ringIndex+1)%len(ring)]
			key := orderedPair(first, second)
			edgeID, ok := edgeByCorners[key]
			if !ok {
				return islandMesh{}, fmt.Errorf("site %d polygon segment %v has no retained edge", cellIndex, key)
			}
			if !containsInt(edges[edgeID].siteIndexes, cellIndex) {
				return islandMesh{}, fmt.Errorf("site %d polygon segment %v has inconsistent incidence", cellIndex, key)
			}
			edgeTraversals[edgeID] = append(edgeTraversals[edgeID], [2]int{first, second})
		}
		cells[cellIndex] = meshCell{siteIndex: cellIndex, center: sites[cellIndex], cornerIDs: ring}
	}
	if math.Abs(totalArea-1) > geometryTolerance {
		return islandMesh{}, fmt.Errorf("cell areas sum to %.17g, want 1", totalArea)
	}
	for edgeID, edge := range edges {
		traversals := edgeTraversals[edgeID]
		if len(traversals) != len(edge.siteIndexes) {
			return islandMesh{}, fmt.Errorf("edge %d has %d polygon traversals for %d incident sites", edgeID, len(traversals), len(edge.siteIndexes))
		}
		if len(traversals) == 2 && (traversals[0][0] != traversals[1][1] || traversals[0][1] != traversals[1][0]) {
			return islandMesh{}, fmt.Errorf("edge %d is not traversed in opposite directions", edgeID)
		}
	}

	mesh := islandMesh{islandID: islandID, cells: cells, corners: corners, edges: edges}
	if err := validateCompleteCandidateMesh(mesh); err != nil {
		return islandMesh{}, err
	}
	return mesh, nil
}

func orderedPair(first, second int) [2]int {
	if first > second {
		first, second = second, first
	}
	return [2]int{first, second}
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func pointLess(first, second Point) bool {
	if first.X != second.X {
		return first.X < second.X
	}
	return first.Y < second.Y
}

func pointInUnitSquare(point Point) bool {
	return point.X >= -geometryTolerance && point.X <= 1+geometryTolerance &&
		point.Y >= -geometryTolerance && point.Y <= 1+geometryTolerance
}

func rotateIntsToMinimum(values []int) {
	minimum := 0
	for i := 1; i < len(values); i++ {
		if values[i] < values[minimum] {
			minimum = i
		}
	}
	if minimum == 0 {
		return
	}
	rotated := append(append(make([]int, 0, len(values)), values[minimum:]...), values[:minimum]...)
	copy(values, rotated)
}

func backendPoint(vertex voronoi.Vertex) Point {
	return Point{X: vertex.X, Y: vertex.Y}
}

func finitePoint(point Point) bool {
	return !math.IsNaN(point.X) && !math.IsNaN(point.Y) && !math.IsInf(point.X, 0) && !math.IsInf(point.Y, 0)
}

func pointsNear(first, second Point) bool {
	return near(first.X, second.X) && near(first.Y, second.Y)
}

func near(first, second float64) bool {
	return math.Abs(first-second) <= geometryTolerance
}

func pointDistance(first, second Point) float64 {
	return math.Hypot(first.X-second.X, first.Y-second.Y)
}

func signedArea(ring []Point) float64 {
	twiceArea := 0.0
	for i, point := range ring {
		next := ring[(i+1)%len(ring)]
		twiceArea += point.X*next.Y - next.X*point.Y
	}
	return twiceArea / 2
}

func reversePoints(points []Point) {
	for left, right := 0, len(points)-1; left < right; left, right = left+1, right-1 {
		points[left], points[right] = points[right], points[left]
	}
}
