// Package singlemesh builds the relaxed rectangular Voronoi mesh shared by
// single-mesh growth experiments.
package singlemesh

import (
	"fmt"
	"math"
	"sort"

	voronoi "github.com/mdhender/wgvc/internal/voronoi"
)

type Point struct {
	X float64
	Y float64
}

type Bounds struct {
	Width  float64
	Height float64
}

type Cell struct {
	ID        int
	Site      Point
	Corners   []Point
	Neighbors []int
}

// Edge is one segment of the final diagram: its endpoints, clipped to the
// bounds, and the one or two cells it separates in ascending order. A
// boundary edge has one cell.
type Edge struct {
	Ends    [2]Point
	CellIDs []int
}

// Mesh is the relaxed Voronoi diagram of one point set. Edges carries the
// diagram's segments so a caller can build its own mesh from them without
// computing the diagram again.
type Mesh struct {
	Cells []Cell
	Edges []Edge
}

func Build(count, relaxations int, bounds Bounds, random interface{ Float64() float64 }) (Mesh, error) {
	const siteInset = 1e-12
	if !validBounds(bounds) {
		return Mesh{}, fmt.Errorf("bounds must be finite and positive: %+v", bounds)
	}
	points := make([]Point, count)
	for i := range points {
		points[i] = Point{
			X: siteInset + float64((bounds.Width-2*siteInset)*random.Float64()),
			Y: siteInset + float64((bounds.Height-2*siteInset)*random.Float64()),
		}
	}
	for range relaxations {
		diagram, indexes, err := computeDiagram(points, bounds)
		if err != nil {
			return Mesh{}, err
		}
		relaxed := make([]Point, len(points))
		for _, cell := range diagram.Cells {
			index := indexes[Point{X: cell.Site.X, Y: cell.Site.Y}]
			ring, err := cellRing(cell)
			if err != nil {
				return Mesh{}, fmt.Errorf("relax site %d: %w", index, err)
			}
			relaxed[index] = polygonCentroid(ring)
		}
		points = relaxed
	}

	diagram, indexes, err := computeDiagram(points, bounds)
	if err != nil {
		return Mesh{}, err
	}
	cells := make([]Cell, len(points))
	cellIndexes := make(map[*voronoi.Cell]int, len(points))
	for _, cell := range diagram.Cells {
		index := indexes[Point{X: cell.Site.X, Y: cell.Site.Y}]
		ring, err := cellRing(cell)
		if err != nil {
			return Mesh{}, fmt.Errorf("site %d: %w", index, err)
		}
		cells[index] = Cell{ID: index, Site: points[index], Corners: ring}
		cellIndexes[cell] = index
	}
	edges := make([]Edge, 0, len(diagram.Edges))
	for edgeIndex, edge := range diagram.Edges {
		if edge.LeftCell == nil {
			return Mesh{}, fmt.Errorf("edge %d has no incident cell", edgeIndex)
		}
		left, leftOK := cellIndexes[edge.LeftCell]
		if !leftOK {
			return Mesh{}, fmt.Errorf("edge %d refers to an unknown cell", edgeIndex)
		}
		cellIDs := []int{left}
		if edge.RightCell != nil {
			right, rightOK := cellIndexes[edge.RightCell]
			if !rightOK {
				return Mesh{}, fmt.Errorf("edge %d refers to an unknown cell", edgeIndex)
			}
			cellIDs = append(cellIDs, right)
			sort.Ints(cellIDs)
			cells[left].Neighbors = append(cells[left].Neighbors, right)
			cells[right].Neighbors = append(cells[right].Neighbors, left)
		}
		edges = append(edges, Edge{
			Ends:    [2]Point{{X: edge.Va.X, Y: edge.Va.Y}, {X: edge.Vb.X, Y: edge.Vb.Y}},
			CellIDs: cellIDs,
		})
	}
	for i := range cells {
		sort.Ints(cells[i].Neighbors)
	}
	return Mesh{Cells: cells, Edges: edges}, nil
}

// ComputeDiagram sorts its input, so it always receives a disposable copy.
func computeDiagram(points []Point, bounds Bounds) (*voronoi.Diagram, map[Point]int, error) {
	backend := make([]voronoi.Vertex, len(points))
	indexes := make(map[Point]int, len(points))
	for i, point := range points {
		if !finitePoint(point) || point.X <= 0 || point.X >= bounds.Width || point.Y <= 0 || point.Y >= bounds.Height {
			return nil, nil, fmt.Errorf("site %d is outside the open bounds %+v: %+v", i, bounds, point)
		}
		if previous, exists := indexes[point]; exists {
			return nil, nil, fmt.Errorf("sites %d and %d are duplicates", previous, i)
		}
		indexes[point] = i
		backend[i] = voronoi.Vertex{X: point.X, Y: point.Y}
	}
	diagram := voronoi.ComputeDiagram(backend, voronoi.NewBBox(0, bounds.Width, 0, bounds.Height), true)
	if len(diagram.Cells) != len(points) {
		return nil, nil, fmt.Errorf("voronoi backend returned %d cells for %d sites", len(diagram.Cells), len(points))
	}
	return diagram, indexes, nil
}

func cellRing(cell *voronoi.Cell) ([]Point, error) {
	if len(cell.Halfedges) < 3 {
		return nil, fmt.Errorf("cell has only %d edges", len(cell.Halfedges))
	}
	ring := make([]Point, len(cell.Halfedges))
	for i, halfedge := range cell.Halfedges {
		vertex := halfedge.GetStartpoint()
		ring[i] = Point{X: vertex.X, Y: vertex.Y}
	}
	if signedArea(ring) < 0 {
		for left, right := 0, len(ring)-1; left < right; left, right = left+1, right-1 {
			ring[left], ring[right] = ring[right], ring[left]
		}
	}
	return ring, nil
}

// polygonCentroid and signedArea wrap each product in an explicit float64
// conversion. The Go spec lets a compiler fuse x*y + z into one rounding, and
// the arm64 backend does while amd64 does not; the conversion forces the
// product to round on its own so both architectures relax to identical sites.
func polygonCentroid(ring []Point) Point {
	twiceArea, x, y := 0.0, 0.0, 0.0
	for i, current := range ring {
		next := ring[(i+1)%len(ring)]
		cross := float64(current.X*next.Y) - float64(next.X*current.Y)
		twiceArea += cross
		x += float64((current.X + next.X) * cross)
		y += float64((current.Y + next.Y) * cross)
	}
	return Point{X: x / (3 * twiceArea), Y: y / (3 * twiceArea)}
}

func signedArea(ring []Point) float64 {
	twiceArea := 0.0
	for i, current := range ring {
		next := ring[(i+1)%len(ring)]
		twiceArea += float64(current.X*next.Y) - float64(next.X*current.Y)
	}
	return twiceArea / 2
}

func finitePoint(point Point) bool {
	return !math.IsNaN(point.X) && !math.IsInf(point.X, 0) && !math.IsNaN(point.Y) && !math.IsInf(point.Y, 0)
}

func validBounds(bounds Bounds) bool {
	return bounds.Width > 2e-12 && bounds.Height > 2e-12 &&
		!math.IsNaN(bounds.Width) && !math.IsInf(bounds.Width, 0) &&
		!math.IsNaN(bounds.Height) && !math.IsInf(bounds.Height, 0)
}
