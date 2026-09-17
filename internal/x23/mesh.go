package x23

import "github.com/mdhender/wgvc/internal/singlemesh"

func buildMesh(count, relaxations int, random interface{ Float64() float64 }) ([]Cell, error) {
	mesh, err := singlemesh.Build(count, relaxations, random)
	if err != nil {
		return nil, err
	}
	cells := make([]Cell, len(mesh))
	for i, cell := range mesh {
		cells[i] = Cell{
			ID:        cell.ID,
			Site:      cell.Site,
			Corners:   cell.Corners,
			Neighbors: cell.Neighbors,
			IslandID:  Water,
		}
	}
	return cells, nil
}

func signedArea(ring []Point) float64 {
	twiceArea := 0.0
	for i, current := range ring {
		next := ring[(i+1)%len(ring)]
		twiceArea += current.X*next.Y - next.X*current.Y
	}
	return twiceArea / 2
}
