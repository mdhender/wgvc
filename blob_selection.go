package wgvc

import (
	"fmt"
	"math"
	"sort"
)

const blobShapeDomain uint64 = 0x626c6f6273686170 // "blobshap"

// candidateBlob is a feasibility prototype for retained-cell islands. It is
// deliberately not wired into Generate until the remaining #8 stages can
// preserve and compact the selected geometry.
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

// selectCandidateBlob tessellates a bounded rectangular candidate grid and
// selects exactly landCount interior cells. The one-cell outer ring is always
// water. Interior capacity is at least twice the request, so no retries or
// quota changes are needed.
func selectCandidateBlob(landCount int, seed uint64) (candidateBlob, error) {
	if landCount < 1 {
		return candidateBlob{}, fmt.Errorf("land count must be at least 1: %d", landCount)
	}

	interiorSide, err := blobInteriorSide(landCount)
	if err != nil {
		return candidateBlob{}, err
	}
	columns := interiorSide + 2
	rows := columns
	if columns > int(^uint(0)>>1)/rows {
		return candidateBlob{}, fmt.Errorf("land count %d exceeds candidate indexing capacity", landCount)
	}

	sites := regularCandidateSites(columns, rows)
	geometry, err := computeBackendGeometry(sites)
	if err != nil {
		return candidateBlob{}, fmt.Errorf("tessellate %d blob candidates: %w", len(sites), err)
	}
	// Run complete candidate-mesh validation rather than validating only the
	// cells that will become land.
	if _, err := canonicalizeGeometry(0, sites, geometry); err != nil {
		return candidateBlob{}, fmt.Errorf("validate complete candidate mesh: %w", err)
	}

	frame := candidateFrameCells(geometry)
	neighbors := candidateNeighbors(geometry)
	if err := validateCandidateGrid(columns, rows, neighbors); err != nil {
		return candidateBlob{}, err
	}
	for index := range sites {
		row, column := index/columns, index%columns
		wantFrame := row == 0 || row == rows-1 || column == 0 || column == columns-1
		if frame[index] != wantFrame {
			return candidateBlob{}, fmt.Errorf("candidate %d frame contact = %t, want %t", index, frame[index], wantFrame)
		}
	}

	random := derivedRandom(seed, blobShapeDomain, 0)
	phase3 := random.Float64() * 2 * math.Pi
	phase5 := random.Float64() * 2 * math.Pi
	scores := make([]float64, len(sites))
	for index, site := range sites {
		dx, dy := site.X-0.5, site.Y-0.5
		angle := math.Atan2(dy, dx)
		// The positive bounded multiplier changes the coastline without
		// overwhelming the low-frequency radial shape.
		wave := 1 + 0.20*math.Sin(3*angle+phase3) + 0.10*math.Sin(5*angle+phase5)
		scores[index] = math.Hypot(dx, dy) * wave
	}

	root := nearestInteriorCandidate(sites, frame)
	land := make([]bool, len(sites))
	land[root] = true
	order := []int{root}
	intervals := make([]occupiedInterval, rows)
	rootRow, rootColumn := root/columns, root%columns
	intervals[rootRow] = occupiedInterval{left: rootColumn, right: rootColumn, set: true}

	for len(order) < landCount {
		frontier := blobGrowthCandidates(columns, rows, intervals, land)
		sortBlobCandidates(frontier, scores)

		chosen := -1
		for _, candidate := range frontier {
			if hasSelectedNeighbor(candidate, neighbors, land) {
				chosen = candidate
				break
			}
		}
		if chosen == -1 {
			return candidateBlob{}, fmt.Errorf("blob selection stalled after %d of %d land cells", len(order), landCount)
		}

		land[chosen] = true
		order = append(order, chosen)
		row, column := chosen/columns, chosen%columns
		if !intervals[row].set {
			intervals[row] = occupiedInterval{left: column, right: column, set: true}
		} else {
			intervals[row].left = min(intervals[row].left, column)
			intervals[row].right = max(intervals[row].right, column)
		}
	}

	return candidateBlob{
		columns: columns, rows: rows, sites: sites, geometry: geometry,
		land: land, frame: frame, order: order,
	}, nil
}

func blobInteriorSide(landCount int) (int, error) {
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
