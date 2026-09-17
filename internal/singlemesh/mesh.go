// Package singlemesh builds the relaxed rectangular Voronoi mesh shared by
// single-mesh growth experiments.
package singlemesh

import (
	"fmt"
	"math"
	"sort"

	voronoi "github.com/pzsz/voronoi"
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

func Build(count, relaxations int, bounds Bounds, random interface{ Float64() float64 }) ([]Cell, error) {
	const siteInset = 1e-12
	if !validBounds(bounds) {
		return nil, fmt.Errorf("bounds must be finite and positive: %+v", bounds)
	}
	points := make([]Point, count)
	for i := range points {
		points[i] = Point{
			X: siteInset + (bounds.Width-2*siteInset)*random.Float64(),
			Y: siteInset + (bounds.Height-2*siteInset)*random.Float64(),
		}
	}
	for range relaxations {
		diagram, indexes, err := computeDiagram(points, bounds)
		if err != nil {
			return nil, err
		}
		relaxed := make([]Point, len(points))
		for _, cell := range diagram.Cells {
			index := indexes[Point{X: cell.Site.X, Y: cell.Site.Y}]
			ring, err := cellRing(cell)
			if err != nil {
				return nil, fmt.Errorf("relax site %d: %w", index, err)
			}
			relaxed[index] = polygonCentroid(ring)
		}
		points = relaxed
	}

	diagram, indexes, err := computeDiagram(points, bounds)
	if err != nil {
		return nil, err
	}
	cells := make([]Cell, len(points))
	cellIndexes := make(map[*voronoi.Cell]int, len(points))
	for _, cell := range diagram.Cells {
		index := indexes[Point{X: cell.Site.X, Y: cell.Site.Y}]
		ring, err := cellRing(cell)
		if err != nil {
			return nil, fmt.Errorf("site %d: %w", index, err)
		}
		cells[index] = Cell{ID: index, Site: points[index], Corners: ring}
		cellIndexes[cell] = index
	}
	for edgeIndex, edge := range diagram.Edges {
		if edge.LeftCell == nil {
			return nil, fmt.Errorf("edge %d has no incident cell", edgeIndex)
		}
		if edge.RightCell == nil {
			continue
		}
		left, leftOK := cellIndexes[edge.LeftCell]
		right, rightOK := cellIndexes[edge.RightCell]
		if !leftOK || !rightOK {
			return nil, fmt.Errorf("edge %d refers to an unknown cell", edgeIndex)
		}
		cells[left].Neighbors = append(cells[left].Neighbors, right)
		cells[right].Neighbors = append(cells[right].Neighbors, left)
	}
	for i := range cells {
		sort.Ints(cells[i].Neighbors)
	}
	return cells, nil
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
		return nil, nil, fmt.Errorf("Voronoi backend returned %d cells for %d sites", len(diagram.Cells), len(points))
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

func polygonCentroid(ring []Point) Point {
	twiceArea, x, y := 0.0, 0.0, 0.0
	for i, current := range ring {
		next := ring[(i+1)%len(ring)]
		cross := current.X*next.Y - next.X*current.Y
		twiceArea += cross
		x += (current.X + next.X) * cross
		y += (current.Y + next.Y) * cross
	}
	return Point{X: x / (3 * twiceArea), Y: y / (3 * twiceArea)}
}

func signedArea(ring []Point) float64 {
	twiceArea := 0.0
	for i, current := range ring {
		next := ring[(i+1)%len(ring)]
		twiceArea += current.X*next.Y - next.X*current.Y
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
