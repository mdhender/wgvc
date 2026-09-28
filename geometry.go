package wgvc

import (
	"fmt"
	"github.com/mdhender/wgvc/internal/fmath"
	"math"
	"sort"

	voronoi "github.com/mdhender/wgvc/internal/voronoi"
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

// islandMesh is the canonical tessellation of one point set: corners in
// strict coordinate order, edges in strict corner-pair order, and cells whose
// rings are counterclockwise and start at their lowest corner ID. Generate
// builds one for the whole world with islandID set to NoIslandID, then
// assigns the public world-global IDs from it.
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

type uniformTransform struct {
	scale       float64
	translation Point
}

func (transform uniformTransform) point(point Point) Point {
	return Point{
		X: transform.translation.X + transform.scale*point.X,
		Y: transform.translation.Y + transform.scale*point.Y,
	}
}

// transformMesh returns a deep copy of mesh with every corner and cell center
// mapped through transform. Topology is unchanged.
func transformMesh(mesh islandMesh, transform uniformTransform) islandMesh {
	transformed := mesh
	transformed.corners = make([]Point, len(mesh.corners))
	for cornerID, corner := range mesh.corners {
		transformed.corners[cornerID] = transform.point(corner)
	}
	transformed.cells = make([]meshCell, len(mesh.cells))
	for cellID, cell := range mesh.cells {
		transformed.cells[cellID] = cell
		transformed.cells[cellID].center = transform.point(cell.center)
		transformed.cells[cellID].cornerIDs = append([]int(nil), cell.cornerIDs...)
	}
	transformed.edges = make([]meshEdge, len(mesh.edges))
	for edgeID, edge := range mesh.edges {
		transformed.edges[edgeID] = edge
		transformed.edges[edgeID].siteIndexes = append([]int(nil), edge.siteIndexes...)
	}
	return transformed
}

// tessellateRectangle tessellates sites inside the width×height rectangle
// with the origin at one corner. Generate does not call it: it builds the
// canonical mesh from the growth mesh's own diagram through
// canonicalizeInBounds instead. Tests and the doc galleries use it.
func tessellateRectangle(islandID IslandID, sites []Point, width, height float64) (islandMesh, error) {
	geometry, err := computeBackendGeometryInBounds(sites, width, height)
	if err != nil {
		return islandMesh{}, fmt.Errorf("island %d: %w", islandID, err)
	}
	return canonicalizeInBounds(islandID, sites, geometry, width, height)
}

// canonicalizeInBounds canonicalizes backend geometry computed inside the
// width×height rectangle with the origin at one corner. Canonical order is
// decided on the unit square, so the sites and geometry are normalized on
// the way in and the mesh is scaled back on the way out. The geometry is
// consumed: its rings and endpoints are rewritten in place.
func canonicalizeInBounds(islandID IslandID, sites []Point, geometry backendGeometry, width, height float64) (islandMesh, error) {
	normalizedSites := make([]Point, len(sites))
	for i, site := range sites {
		normalizedSites[i] = Point{X: site.X / width, Y: site.Y / height}
	}
	for i := range geometry.cells {
		for j, point := range geometry.cells[i].ring {
			geometry.cells[i].ring[j] = Point{X: point.X / width, Y: point.Y / height}
		}
	}
	for i := range geometry.edges {
		for j, point := range geometry.edges[i].ends {
			geometry.edges[i].ends[j] = Point{X: point.X / width, Y: point.Y / height}
		}
	}
	mesh, err := canonicalizeGeometry(islandID, normalizedSites, geometry)
	if err != nil {
		return islandMesh{}, fmt.Errorf("island %d: %w", islandID, err)
	}
	for i, point := range mesh.corners {
		mesh.corners[i] = Point{X: point.X * width, Y: point.Y * height}
	}
	for i, cell := range mesh.cells {
		mesh.cells[i].center = Point{X: cell.center.X * width, Y: cell.center.Y * height}
	}
	return mesh, nil
}

// computeBackendGeometryInBounds isolates dependency-specific types and
// restores the caller's site order after the backend sorts its copied input.
func computeBackendGeometryInBounds(sites []Point, width, height float64) (backendGeometry, error) {
	if len(sites) == 0 {
		return backendGeometry{}, fmt.Errorf("at least one site is required")
	}
	if !(width > 0) || math.IsNaN(width) || math.IsInf(width, 0) || !(height > 0) || math.IsNaN(height) || math.IsInf(height, 0) {
		return backendGeometry{}, fmt.Errorf("bounds must be finite and positive: %gx%g", width, height)
	}

	siteIndexes := make(map[Point]int, len(sites))
	backendSites := make([]voronoi.Vertex, len(sites))
	for i, site := range sites {
		if !finitePoint(site) || site.X <= 0 || site.X >= width || site.Y <= 0 || site.Y >= height {
			return backendGeometry{}, fmt.Errorf("site %d is outside the open finite bounds %gx%g: %+v", i, width, height, site)
		}
		if previous, exists := siteIndexes[site]; exists {
			return backendGeometry{}, fmt.Errorf("sites %d and %d are duplicates", previous, i)
		}
		siteIndexes[site] = i
		backendSites[i] = voronoi.Vertex{X: site.X, Y: site.Y}
	}

	if len(sites) == 1 {
		return oneSiteGeometry(width, height), nil
	}

	// ComputeDiagram sorts its argument in place. A fresh slice protects both
	// caller order and caller-owned storage.
	diagram := voronoi.ComputeDiagram(backendSites, voronoi.NewBBox(0, width, 0, height), true)
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
			start := backendPointInBounds(halfedge.GetStartpoint(), width, height)
			end := backendPointInBounds(halfedge.GetEndpoint(), width, height)
			next := backendPointInBounds(cell.Halfedges[(i+1)%len(cell.Halfedges)].GetStartpoint(), width, height)
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
				backendPointInBounds(edge.Va.Vertex, width, height),
				backendPointInBounds(edge.Vb.Vertex, width, height),
			},
			siteIndexes: incidence,
		})
	}

	return geometry, nil
}

func oneSiteGeometry(width, height float64) backendGeometry {
	ring := []Point{{X: 0, Y: 0}, {X: width, Y: 0}, {X: width, Y: height}, {X: 0, Y: height}}
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
	if err := validateCanonicalMesh(mesh); err != nil {
		return islandMesh{}, err
	}
	return mesh, nil
}

// validateCanonicalMesh checks the canonical ordering, topology, and polygon
// invariants of a normalized mesh: every corner and center inside the unit
// square, corners and edges in strict order, every cell a convex
// counterclockwise ring that starts at its lowest corner, contains its center,
// and is bounded by consistent edges, interior edges traversed in opposite
// directions, no orphaned corners, and cell areas summing to the unit square.
func validateCanonicalMesh(mesh islandMesh) error {
	if len(mesh.cells) == 0 {
		return fmt.Errorf("mesh has no cells")
	}
	if len(mesh.corners) < 3 {
		return fmt.Errorf("mesh has only %d corners", len(mesh.corners))
	}

	cornerInRing := make([]bool, len(mesh.corners))
	cornerInEdge := make([]bool, len(mesh.corners))
	for cornerID, point := range mesh.corners {
		if !finitePoint(point) || !pointInUnitSquare(point) {
			return fmt.Errorf("corner %d is outside the finite unit square: %+v", cornerID, point)
		}
		if cornerID > 0 && !pointLess(mesh.corners[cornerID-1], point) {
			return fmt.Errorf("corners %d and %d are not in strict canonical order", cornerID-1, cornerID)
		}
	}

	edgeByCorners := make(map[[2]int]int, len(mesh.edges))
	for edgeID, edge := range mesh.edges {
		first, second := edge.cornerIDs[0], edge.cornerIDs[1]
		if first < 0 || second >= len(mesh.corners) || first >= second {
			return fmt.Errorf("edge %d has invalid canonical corners %v", edgeID, edge.cornerIDs)
		}
		if edgeID > 0 && !edgePairLess(mesh.edges[edgeID-1].cornerIDs, edge.cornerIDs) {
			return fmt.Errorf("edges %d and %d are not in strict canonical order", edgeID-1, edgeID)
		}
		if pointDistance(mesh.corners[first], mesh.corners[second]) <= geometryTolerance {
			return fmt.Errorf("edge %d has non-positive length within tolerance", edgeID)
		}
		if len(edge.siteIndexes) < 1 || len(edge.siteIndexes) > 2 {
			return fmt.Errorf("edge %d has %d incident cells", edgeID, len(edge.siteIndexes))
		}
		for incidenceIndex, cellID := range edge.siteIndexes {
			if cellID < 0 || cellID >= len(mesh.cells) {
				return fmt.Errorf("edge %d refers to unknown cell %d", edgeID, cellID)
			}
			if incidenceIndex > 0 && edge.siteIndexes[incidenceIndex-1] >= cellID {
				return fmt.Errorf("edge %d has non-canonical incidence %v", edgeID, edge.siteIndexes)
			}
		}
		cornerInEdge[first] = true
		cornerInEdge[second] = true
		edgeByCorners[edge.cornerIDs] = edgeID
	}

	totalArea := 0.0
	edgeTraversals := make([][][2]int, len(mesh.edges))
	for cellID, cell := range mesh.cells {
		if cell.siteIndex != cellID {
			return fmt.Errorf("cell %d has site index %d", cellID, cell.siteIndex)
		}
		if !finitePoint(cell.center) || !pointInUnitSquare(cell.center) {
			return fmt.Errorf("cell %d has center outside the finite unit square: %+v", cellID, cell.center)
		}
		if len(cell.cornerIDs) < 3 {
			return fmt.Errorf("cell %d has only %d corners", cellID, len(cell.cornerIDs))
		}
		if cell.cornerIDs[0] != minimumInt(cell.cornerIDs) {
			return fmt.Errorf("cell %d does not start at its lowest corner ID", cellID)
		}

		ring := make([]Point, len(cell.cornerIDs))
		seenCorners := make(map[int]struct{}, len(cell.cornerIDs))
		for ringIndex, cornerID := range cell.cornerIDs {
			if cornerID < 0 || cornerID >= len(mesh.corners) {
				return fmt.Errorf("cell %d refers to unknown corner %d", cellID, cornerID)
			}
			if _, exists := seenCorners[cornerID]; exists {
				return fmt.Errorf("cell %d repeats corner %d", cellID, cornerID)
			}
			seenCorners[cornerID] = struct{}{}
			cornerInRing[cornerID] = true
			ring[ringIndex] = mesh.corners[cornerID]

			next := cell.cornerIDs[(ringIndex+1)%len(cell.cornerIDs)]
			edgeID, exists := edgeByCorners[orderedPair(cornerID, next)]
			if !exists {
				return fmt.Errorf("cell %d segment %d->%d has no edge", cellID, cornerID, next)
			}
			if !containsInt(mesh.edges[edgeID].siteIndexes, cellID) {
				return fmt.Errorf("cell %d segment edge %d has inconsistent incidence", cellID, edgeID)
			}
			edgeTraversals[edgeID] = append(edgeTraversals[edgeID], [2]int{cornerID, next})
		}

		area := signedArea(ring)
		if area <= geometryTolerance {
			return fmt.Errorf("cell %d has non-positive area %g", cellID, area)
		}
		totalArea += area
		for ringIndex, point := range ring {
			next := ring[(ringIndex+1)%len(ring)]
			after := ring[(ringIndex+2)%len(ring)]
			if pointCross(point, next, after) < -geometryTolerance {
				return fmt.Errorf("cell %d is not convex and counterclockwise at corner %d", cellID, ringIndex)
			}
			if pointCross(point, next, cell.center) < -geometryTolerance {
				return fmt.Errorf("cell %d does not contain its generating center", cellID)
			}
		}
	}

	if math.Abs(totalArea-1) > geometryTolerance {
		return fmt.Errorf("cell areas sum to %.17g, want 1", totalArea)
	}
	for edgeID, edge := range mesh.edges {
		traversals := edgeTraversals[edgeID]
		if len(traversals) != len(edge.siteIndexes) {
			return fmt.Errorf("edge %d has %d polygon traversals for %d incident cells", edgeID, len(traversals), len(edge.siteIndexes))
		}
		if len(traversals) == 2 && (traversals[0][0] != traversals[1][1] || traversals[0][1] != traversals[1][0]) {
			return fmt.Errorf("edge %d is not traversed in opposite directions", edgeID)
		}
	}
	for cornerID := range mesh.corners {
		if !cornerInRing[cornerID] || !cornerInEdge[cornerID] {
			return fmt.Errorf("corner %d is orphaned", cornerID)
		}
	}
	return nil
}

// pointCross returns twice the signed area of the triangle first, second,
// third: positive when third lies to the left of the directed segment from
// first to second.
func pointCross(first, second, third Point) float64 {
	return (second.X-first.X)*(third.Y-first.Y) - (second.Y-first.Y)*(third.X-first.X)
}

func edgePairLess(first, second [2]int) bool {
	if first[0] != second[0] {
		return first[0] < second[0]
	}
	return first[1] < second[1]
}

func minimumInt(values []int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
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

// backendPointInBounds snaps coordinates within tolerance of a bounding-box
// side exactly onto it. The backend clips edges with Liang-Barsky arithmetic,
// so a clipped vertex lands a few ulps off the side, and the direction of that
// error depends on whether the compiler fused a multiply-add (Go permits this
// on arm64 but not amd64). Corner IDs are assigned by exact coordinate order,
// so without snapping, border corners would be numbered differently on
// different architectures.
func backendPointInBounds(vertex voronoi.Vertex, width, height float64) Point {
	return snapPointToBounds(Point{X: vertex.X, Y: vertex.Y}, width, height)
}

// snapPointToBounds moves a coordinate within tolerance of the rectangle's
// boundary onto it, so border corners compare equal wherever the backend's
// clipping arithmetic put them.
func snapPointToBounds(point Point, width, height float64) Point {
	return Point{X: snapToBounds(point.X, width), Y: snapToBounds(point.Y, height)}
}

func snapToBounds(value, limit float64) float64 {
	switch {
	case near(value, 0):
		return 0
	case near(value, limit):
		return limit
	default:
		return value
	}
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
	return fmath.Hypot(first.X-second.X, first.Y-second.Y)
}

func signedArea(ring []Point) float64 {
	twiceArea := 0.0
	for i, point := range ring {
		next := ring[(i+1)%len(ring)]
		// Explicit conversions keep the products from fusing into a
		// multiply-add on arm64; see backendPointInBounds.
		twiceArea += float64(point.X*next.Y) - float64(next.X*point.Y)
	}
	return twiceArea / 2
}

func reversePoints(points []Point) {
	for left, right := 0, len(points)-1; left < right; left, right = left+1, right-1 {
		points[left], points[right] = points[right], points[left]
	}
}
