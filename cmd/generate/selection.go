package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mdhender/wgvc"
)

// selection describes which provinces a partial map shows. A zero selection
// means the whole world.
type selection struct {
	hasRegion              bool
	minX, minY, maxX, maxY float64
	ids                    []wgvc.ProvinceID
	radius                 int
}

// parseSelection reads the -region, -select, and -radius flags. Region is
// minx,miny,maxx,maxy in world coordinates and selects provinces whose
// center lies inside it; select lists province IDs; radius grows the union
// of both by that many hops and needs at least one of them.
func parseSelection(region, ids string, radius int) (selection, error) {
	var result selection
	if region != "" {
		parts := strings.Split(region, ",")
		if len(parts) != 4 {
			return selection{}, fmt.Errorf("region %q must be minx,miny,maxx,maxy", region)
		}
		values := make([]float64, 4)
		for i, part := range parts {
			value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
			if err != nil {
				return selection{}, fmt.Errorf("parse region value %q: %w", part, err)
			}
			values[i] = value
		}
		result.hasRegion = true
		result.minX, result.minY, result.maxX, result.maxY = values[0], values[1], values[2], values[3]
		if !(result.minX < result.maxX) || !(result.minY < result.maxY) {
			return selection{}, fmt.Errorf("region %q must have positive width and height", region)
		}
	}
	if ids != "" {
		for _, part := range strings.Split(ids, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || id < 0 {
				return selection{}, fmt.Errorf("province ID %q must be a non-negative integer", part)
			}
			result.ids = append(result.ids, wgvc.ProvinceID(id))
		}
	}
	if radius < 0 {
		return selection{}, fmt.Errorf("radius must not be negative: %d", radius)
	}
	if radius > 0 && !result.hasRegion && len(result.ids) == 0 {
		return selection{}, fmt.Errorf("-radius needs -region or -select")
	}
	result.radius = radius
	return result, nil
}

// apply resolves the selection against a world: nil for the whole world,
// otherwise one flag per province.
func (s selection) apply(world wgvc.World) ([]bool, error) {
	if !s.hasRegion && len(s.ids) == 0 {
		return nil, nil
	}
	selected := make([]bool, len(world.Provinces))
	for _, id := range s.ids {
		if int(id) >= len(world.Provinces) {
			return nil, fmt.Errorf("province %d does not exist; the world has %d provinces", id, len(world.Provinces))
		}
		selected[id] = true
	}
	if s.hasRegion {
		for _, province := range world.Provinces {
			c := province.Center
			if c.X >= s.minX && c.X <= s.maxX && c.Y >= s.minY && c.Y <= s.maxY {
				selected[province.ID] = true
			}
		}
	}
	var frontier []wgvc.ProvinceID
	for id, in := range selected {
		if in {
			frontier = append(frontier, wgvc.ProvinceID(id))
		}
	}
	if len(frontier) == 0 {
		return nil, fmt.Errorf("selection contains no province")
	}
	for range s.radius {
		var next []wgvc.ProvinceID
		for _, id := range frontier {
			for _, exit := range world.Provinces[id].Exits {
				if exit.NeighborID != wgvc.NoProvinceID && !selected[exit.NeighborID] {
					selected[exit.NeighborID] = true
					next = append(next, exit.NeighborID)
				}
			}
		}
		frontier = next
	}
	return selected, nil
}
