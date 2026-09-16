package wgvc

import (
	"fmt"
	"math"
	"sort"
)

// candidateBlob retains the #9 visual fixture around the production selection
// stage. It is deliberately not wired into Generate until the remaining #8
// stages can preserve and compact the selected geometry.
type candidateBlob struct {
	columns  int
	rows     int
	sites    []Point
	geometry backendGeometry
	land     []bool
	frame    []bool
	order    []int
}

type occupiedInterval struct {
	left  int
	right int
	set   bool
}

// selectCandidateBlob tessellates the regular #9 fixture and delegates its
// selection to selectCandidateCells.
func selectCandidateBlob(landCount int, seed uint64) (candidateBlob, error) {
	if landCount < 1 {
		return candidateBlob{}, fmt.Errorf("land count must be at least 1: %d", landCount)
	}

	columns, rows, err := candidateGridDimensions(landCount)
	if err != nil {
		return candidateBlob{}, err
	}

	sites := regularCandidateSites(columns, rows)
	geometry, err := computeBackendGeometry(sites)
	if err != nil {
		return candidateBlob{}, fmt.Errorf("tessellate %d blob candidates: %w", len(sites), err)
	}
	shape := generateBlobShape(blobShapeRandom(seed, 0))
	mesh, err := canonicalizeGeometry(0, sites, geometry)
	if err != nil {
		return candidateBlob{}, fmt.Errorf("validate complete candidate mesh: %w", err)
	}
	order, err := selectCandidateCells(landCount, candidateSitePlan{
		columns: columns,
		rows:    rows,
		sites:   sites,
	}, mesh, shape)
	if err != nil {
		return candidateBlob{}, err
	}
	frame := candidateFrameCells(geometry)
	land := make([]bool, len(sites))
	for _, candidateID := range order {
		land[candidateID] = true
	}

	return candidateBlob{
		columns: columns, rows: rows, sites: sites, geometry: geometry,
		land: land, frame: frame, order: order,
	}, nil
}

// selectCandidateCells chooses exactly landCount cells from a canonical
// complete candidate mesh. The result is in ascending original candidate
// order, independently of the order in which the blob grows.
func selectCandidateCells(landCount int, candidates candidateSitePlan, mesh islandMesh, shape blobShape) ([]int, error) {
	if landCount < 1 {
		return nil, fmt.Errorf("land count must be at least 1: %d", landCount)
	}
	if candidates.columns < 3 || candidates.rows < 3 ||
		candidates.columns > int(^uint(0)>>1)/candidates.rows {
		return nil, fmt.Errorf("invalid candidate grid dimensions: %dx%d", candidates.columns, candidates.rows)
	}
	candidateCount := candidates.columns * candidates.rows
	if len(candidates.sites) != candidateCount {
		return nil, fmt.Errorf("candidate site count = %d, want %d for %dx%d grid", len(candidates.sites), candidateCount, candidates.columns, candidates.rows)
	}
	if len(mesh.cells) != candidateCount {
		return nil, fmt.Errorf("candidate mesh cell count = %d, want %d", len(mesh.cells), candidateCount)
	}
	for candidateID, cell := range mesh.cells {
		if cell.siteIndex != candidateID {
			return nil, fmt.Errorf("candidate mesh cell %d refers to original candidate %d", candidateID, cell.siteIndex)
		}
		if cell.center != candidates.sites[candidateID] {
			return nil, fmt.Errorf("candidate mesh cell %d center does not match its original site", candidateID)
		}
	}

	neighbors, frame, err := candidateMeshTopology(mesh)
	if err != nil {
		return nil, err
	}
	if err := validateCandidateGrid(candidates.columns, candidates.rows, neighbors); err != nil {
		return nil, fmt.Errorf("validate candidate mesh: %w", err)
	}
	eligibleCount := 0
	for candidateID := range mesh.cells {
		row, column := candidateID/candidates.columns, candidateID%candidates.columns
		wantFrame := row == 0 || row == candidates.rows-1 || column == 0 || column == candidates.columns-1
		if frame[candidateID] != wantFrame {
			return nil, fmt.Errorf("candidate %d frame contact = %t, want %t", candidateID, frame[candidateID], wantFrame)
		}
		if !frame[candidateID] {
			eligibleCount++
		}
	}
	if landCount > eligibleCount {
		return nil, fmt.Errorf("insufficient eligible candidate capacity: need %d land cells, have %d", landCount, eligibleCount)
	}

	scores := make([]float64, candidateCount)
	for candidateID, cell := range mesh.cells {
		scores[candidateID] = shape.score(cell.center)
		if math.IsNaN(scores[candidateID]) || math.IsInf(scores[candidateID], 0) {
			return nil, fmt.Errorf("candidate %d has non-finite blob score", candidateID)
		}
	}

	root := nearestInteriorCandidate(candidates.sites, frame)
	selected := make([]bool, candidateCount)
	selected[root] = true
	growthOrder := []int{root}
	intervals := make([]occupiedInterval, candidates.rows)
	rootRow, rootColumn := root/candidates.columns, root%candidates.columns
	intervals[rootRow] = occupiedInterval{left: rootColumn, right: rootColumn, set: true}

	for len(growthOrder) < landCount {
		frontier := blobGrowthCandidates(candidates.columns, candidates.rows, intervals, selected)
		sortBlobCandidates(frontier, scores)

		chosen := -1
		for _, candidateID := range frontier {
			if hasSelectedNeighbor(candidateID, neighbors, selected) {
				chosen = candidateID
				break
			}
		}
		if chosen == -1 {
			return nil, fmt.Errorf("blob selection stalled after %d of %d land cells", len(growthOrder), landCount)
		}

		selected[chosen] = true
		growthOrder = append(growthOrder, chosen)
		row, column := chosen/candidates.columns, chosen%candidates.columns
		if !intervals[row].set {
			intervals[row] = occupiedInterval{left: column, right: column, set: true}
		} else {
			intervals[row].left = min(intervals[row].left, column)
			intervals[row].right = max(intervals[row].right, column)
		}
	}

	selectedIDs := make([]int, 0, landCount)
	for candidateID, isSelected := range selected {
		if isSelected {
			selectedIDs = append(selectedIDs, candidateID)
		}
	}
	return selectedIDs, nil
}

func candidateMeshTopology(mesh islandMesh) ([][]int, []bool, error) {
	neighbors := make([][]int, len(mesh.cells))
	frame := make([]bool, len(mesh.cells))
	for edgeID, edge := range mesh.edges {
		firstCorner, secondCorner := edge.cornerIDs[0], edge.cornerIDs[1]
		if firstCorner < 0 || firstCorner >= len(mesh.corners) || secondCorner < 0 || secondCorner >= len(mesh.corners) {
			return nil, nil, fmt.Errorf("candidate mesh edge %d refers to unknown corner %v", edgeID, edge.cornerIDs)
		}
		if !finitePoint(mesh.corners[firstCorner]) || !finitePoint(mesh.corners[secondCorner]) {
			return nil, nil, fmt.Errorf("candidate mesh edge %d has a non-finite endpoint", edgeID)
		}
		if pointDistance(mesh.corners[firstCorner], mesh.corners[secondCorner]) <= geometryTolerance {
			return nil, nil, fmt.Errorf("candidate mesh edge %d has non-positive length within tolerance", edgeID)
		}
		if len(edge.siteIndexes) < 1 || len(edge.siteIndexes) > 2 {
			return nil, nil, fmt.Errorf("candidate mesh edge %d has %d incident candidates", edgeID, len(edge.siteIndexes))
		}
		for _, candidateID := range edge.siteIndexes {
			if candidateID < 0 || candidateID >= len(mesh.cells) {
				return nil, nil, fmt.Errorf("candidate mesh edge %d refers to unknown candidate %d", edgeID, candidateID)
			}
		}
		if len(edge.siteIndexes) == 1 {
			frame[edge.siteIndexes[0]] = true
			continue
		}
		first, second := edge.siteIndexes[0], edge.siteIndexes[1]
		if first >= second {
			return nil, nil, fmt.Errorf("candidate mesh edge %d has non-canonical incidence %v", edgeID, edge.siteIndexes)
		}
		neighbors[first] = append(neighbors[first], second)
		neighbors[second] = append(neighbors[second], first)
	}
	for candidateID := range neighbors {
		sort.Ints(neighbors[candidateID])
		for neighborIndex := 1; neighborIndex < len(neighbors[candidateID]); neighborIndex++ {
			if neighbors[candidateID][neighborIndex-1] == neighbors[candidateID][neighborIndex] {
				return nil, nil, fmt.Errorf("candidate mesh repeats adjacency between %d and %d", candidateID, neighbors[candidateID][neighborIndex])
			}
		}
	}
	return neighbors, frame, nil
}

func candidateGridDimensions(landCount int) (int, int, error) {
	interiorSide, err := blobInteriorSide(landCount)
	if err != nil {
		return 0, 0, err
	}
	columns := interiorSide + 2
	rows := columns
	if columns > int(^uint(0)>>1)/rows {
		return 0, 0, fmt.Errorf("land count %d exceeds candidate indexing capacity", landCount)
	}
	return columns, rows, nil
}

func blobInteriorSide(landCount int) (int, error) {
	if landCount < 1 {
		return 0, fmt.Errorf("land count must be at least 1: %d", landCount)
	}
	if landCount > int(^uint(0)>>1)/2 {
		return 0, fmt.Errorf("land count %d exceeds candidate capacity", landCount)
	}
	wanted := 2 * landCount
	side := int(math.Ceil(math.Sqrt(float64(wanted))))
	for side > 1 && !integerSquareLess(side-1, wanted) {
		side--
	}
	for integerSquareLess(side, wanted) {
		side++
	}
	if side > int(^uint(0)>>1)-2 {
		return 0, fmt.Errorf("land count %d exceeds candidate dimensions", landCount)
	}
	return side, nil
}

func generateBlobShape(random interface{ Float64() float64 }) blobShape {
	return blobShape{
		phase3: random.Float64() * 2 * math.Pi,
		phase5: random.Float64() * 2 * math.Pi,
	}
}

func (shape blobShape) score(site Point) float64 {
	dx, dy := site.X-0.5, site.Y-0.5
	angle := math.Atan2(dy, dx)
	// The positive bounded multiplier changes the coastline without
	// overwhelming the low-frequency radial shape.
	wave := 1 + 0.20*math.Sin(3*angle+shape.phase3) + 0.10*math.Sin(5*angle+shape.phase5)
	return math.Hypot(dx, dy) * wave
}

func integerSquareLess(value, target int) bool {
	quotient := target / value
	return value < quotient || value == quotient && target%value != 0
}

func regularCandidateSites(columns, rows int) []Point {
	sites := make([]Point, 0, columns*rows)
	for row := range rows {
		for column := range columns {
			sites = append(sites, Point{
				X: (float64(column) + 0.5) / float64(columns),
				Y: (float64(row) + 0.5) / float64(rows),
			})
		}
	}
	return sites
}

func candidateNeighbors(geometry backendGeometry) [][]int {
	neighbors := make([][]int, len(geometry.cells))
	for _, edge := range geometry.edges {
		if len(edge.siteIndexes) != 2 || pointDistance(edge.ends[0], edge.ends[1]) <= geometryTolerance {
			continue
		}
		first, second := edge.siteIndexes[0], edge.siteIndexes[1]
		neighbors[first] = append(neighbors[first], second)
		neighbors[second] = append(neighbors[second], first)
	}
	for index := range neighbors {
		sort.Ints(neighbors[index])
	}
	return neighbors
}

func candidateFrameCells(geometry backendGeometry) []bool {
	frame := make([]bool, len(geometry.cells))
	for _, edge := range geometry.edges {
		if len(edge.siteIndexes) == 1 {
			frame[edge.siteIndexes[0]] = true
		}
	}
	return frame
}

func validateCandidateGrid(columns, rows int, neighbors [][]int) error {
	for row := range rows {
		for column := range columns {
			index := row*columns + column
			for _, adjacent := range []struct {
				valid bool
				index int
			}{
				{column > 0, index - 1},
				{column+1 < columns, index + 1},
				{row > 0, index - columns},
				{row+1 < rows, index + columns},
			} {
				if adjacent.valid && !containsInt(neighbors[index], adjacent.index) {
					return fmt.Errorf("candidate grid cells %d and %d do not share a positive-length edge", index, adjacent.index)
				}
			}
		}
	}
	return nil
}

func nearestInteriorCandidate(sites []Point, frame []bool) int {
	nearest := -1
	nearestDistance := math.Inf(1)
	for index, site := range sites {
		if frame[index] {
			continue
		}
		distance := math.Hypot(site.X-0.5, site.Y-0.5)
		if distance < nearestDistance {
			nearest, nearestDistance = index, distance
		}
	}
	return nearest
}

func blobGrowthCandidates(columns, rows int, intervals []occupiedInterval, land []bool) []int {
	candidates := make([]int, 0)
	seen := make([]bool, len(land))
	add := func(row, column int) {
		index := row*columns + column
		if row > 0 && row < rows-1 && column > 0 && column < columns-1 && !land[index] && !seen[index] {
			seen[index] = true
			candidates = append(candidates, index)
		}
	}
	for row, interval := range intervals {
		if !interval.set {
			continue
		}
		add(row, interval.left-1)
		add(row, interval.right+1)
		for column := interval.left; column <= interval.right; column++ {
			if row > 1 && !intervals[row-1].set {
				add(row-1, column)
			}
			if row+2 < rows && !intervals[row+1].set {
				add(row+1, column)
			}
		}
	}
	return candidates
}

func hasSelectedNeighbor(index int, neighbors [][]int, selected []bool) bool {
	for _, neighbor := range neighbors[index] {
		if selected[neighbor] {
			return true
		}
	}
	return false
}

func sortBlobCandidates(candidates []int, scores []float64) {
	sort.Slice(candidates, func(i, j int) bool {
		if scores[candidates[i]] != scores[candidates[j]] {
			return scores[candidates[i]] < scores[candidates[j]]
		}
		return candidates[i] < candidates[j]
	})
}
