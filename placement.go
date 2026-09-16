package wgvc

import (
	"fmt"
	"math"
)

const (
	islandWaterGap    = 1.0
	islandWorldMargin = 1.0
	islandJitter      = 0.25
)

type rectangle struct {
	min Point
	max Point
}

func (r rectangle) pointAt(normalized Point) Point {
	return Point{
		X: r.min.X + normalized.X*(r.max.X-r.min.X),
		Y: r.min.Y + normalized.Y*(r.max.Y-r.min.Y),
	}
}

func planIslands(config Config, allocations []int) (islandLayout, error) {
	if err := config.validate(); err != nil {
		return islandLayout{}, err
	}
	if len(allocations) != config.IslandCount {
		return islandLayout{}, fmt.Errorf("allocation count = %d, want %d", len(allocations), config.IslandCount)
	}

	maxSide := 0.0
	total := 0
	for islandID, allocation := range allocations {
		if allocation < 1 {
			return islandLayout{}, fmt.Errorf("island %d allocation must be at least 1: %d", islandID, allocation)
		}
		total += allocation
		maxSide = math.Max(maxSide, math.Sqrt(float64(allocation)))
	}
	if total != config.ProvinceCount {
		return islandLayout{}, fmt.Errorf("allocation total = %d, want %d", total, config.ProvinceCount)
	}

	columns := int(math.Ceil(math.Sqrt(float64(len(allocations)))))
	slotSize := maxSide + islandWaterGap + 2*islandJitter
	random := placementRandom(config.WorldSeed)
	plans := make([]islandPlan, len(allocations))
	for islandIndex, allocation := range allocations {
		column := islandIndex % columns
		row := islandIndex / columns
		center := Point{
			X: (float64(column)+0.5)*slotSize + signedJitter(random.Float64()),
			Y: (float64(row)+0.5)*slotSize + signedJitter(random.Float64()),
		}
		side := math.Sqrt(float64(allocation))
		halfSide := side / 2
		plans[islandIndex] = islandPlan{
			id: IslandID(islandIndex),
			footprint: rectangle{
				min: Point{X: center.X - halfSide, Y: center.Y - halfSide},
				max: Point{X: center.X + halfSide, Y: center.Y + halfSide},
			},
			provinceCenters: generateProvinceSeeds(allocation, provinceRandom(config.WorldSeed, IslandID(islandIndex))),
		}
	}

	return islandLayout{
		islands: plans,
		bounds:  boundsAround(plans, islandWorldMargin),
	}, nil
}

func signedJitter(value float64) float64 {
	return (2*value - 1) * islandJitter
}

func boundsAround(plans []islandPlan, margin float64) rectangle {
	bounds := plans[0].footprint
	for _, plan := range plans[1:] {
		bounds.min.X = math.Min(bounds.min.X, plan.footprint.min.X)
		bounds.min.Y = math.Min(bounds.min.Y, plan.footprint.min.Y)
		bounds.max.X = math.Max(bounds.max.X, plan.footprint.max.X)
		bounds.max.Y = math.Max(bounds.max.Y, plan.footprint.max.Y)
	}
	bounds.min.X -= margin
	bounds.min.Y -= margin
	bounds.max.X += margin
	bounds.max.Y += margin
	return bounds
}
