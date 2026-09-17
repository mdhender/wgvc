// Package x23 implements the single-mesh island-growth algorithm introduced by
// issue #23. It is internal so calibration controls and diagnostics do not
// expand the public wgvc API.
package x23

import (
	"errors"
	"fmt"
)

const Water = -1

// ErrUnsatisfiable reports that every configured generation round exhausted
// its legal growth frontier.
var ErrUnsatisfiable = errors.New("growth constraints are unsatisfiable")

type Point struct {
	X float64
	Y float64
}

type Config struct {
	WorldSeed         uint64
	ProvinceCount     int
	IslandCount       int
	OceanPercentage   float64
	MinEdgeDistance   int
	MinIslandDistance int
	MaxRounds         int
	Relaxations       int
}

func DefaultConfig() Config {
	return Config{
		WorldSeed:         0x0123456789abcdef,
		ProvinceCount:     1_500,
		IslandCount:       15,
		OceanPercentage:   0.68,
		MinEdgeDistance:   3,
		MinIslandDistance: 3,
		MaxRounds:         10,
		Relaxations:       2,
	}
}

func (c Config) validate() error {
	if c.IslandCount < 1 {
		return fmt.Errorf("island count must be at least 1: %d", c.IslandCount)
	}
	if c.ProvinceCount < c.IslandCount {
		return fmt.Errorf("province count must be at least island count: provinces=%d islands=%d", c.ProvinceCount, c.IslandCount)
	}
	if !(c.OceanPercentage >= 0 && c.OceanPercentage <= maximumOcean) {
		return fmt.Errorf("ocean percentage must be in [0, %.2f]: %g", maximumOcean, c.OceanPercentage)
	}
	if c.MinEdgeDistance < 0 {
		return fmt.Errorf("minimum edge distance must not be negative: %d", c.MinEdgeDistance)
	}
	if c.MinIslandDistance < 1 {
		return fmt.Errorf("minimum island distance must be at least 1: %d", c.MinIslandDistance)
	}
	if c.MaxRounds < 1 {
		return fmt.Errorf("maximum rounds must be at least 1: %d", c.MaxRounds)
	}
	if c.Relaxations < 0 {
		return fmt.Errorf("relaxations must not be negative: %d", c.Relaxations)
	}
	return nil
}

type Cell struct {
	ID        int
	Site      Point
	Corners   []Point
	Neighbors []int
	IslandID  int
}

type Island struct {
	ID      int
	SeedID  int
	CellIDs []int
}

type Result struct {
	Cells           []Cell
	Islands         []Island
	RoundsAttempted int
	FinalOcean      float64
}
