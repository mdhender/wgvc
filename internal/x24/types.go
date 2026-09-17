// Package x24 implements the desirability-field single-mesh growth experiment
// described by issue #24.
package x24

import (
	"errors"
	"fmt"
	"math"

	"github.com/mdhender/wgvc/internal/singlemesh"
)

const Water = -1

var ErrStarved = errors.New("all islands starved before claiming the requested provinces")

type Point = singlemesh.Point

type Config struct {
	WorldSeed          uint64
	ProvinceCount      int
	IslandCount        int
	OceanPercentage    float64
	EdgeRampDistance   int
	AttractantCount    int
	AttractantRadius   float64
	SoftmaxTemperature float64
	ControlPenalty     float64
	MaxRounds          int
	Relaxations        int
}

func DefaultConfig() Config {
	return Config{
		WorldSeed:          0x0123456789abcdef,
		ProvinceCount:      1_500,
		IslandCount:        15,
		OceanPercentage:    0.68,
		EdgeRampDistance:   4,
		AttractantCount:    12,
		AttractantRadius:   0.18,
		SoftmaxTemperature: 0.20,
		ControlPenalty:     -0.95,
		MaxRounds:          10,
		Relaxations:        2,
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
	if c.EdgeRampDistance < 1 {
		return fmt.Errorf("edge ramp distance must be at least 1: %d", c.EdgeRampDistance)
	}
	if c.AttractantCount < 0 {
		return fmt.Errorf("attractant count must not be negative: %d", c.AttractantCount)
	}
	if c.AttractantCount > 0 && !(c.AttractantRadius > 0) {
		return fmt.Errorf("attractant radius must be positive: %g", c.AttractantRadius)
	}
	if !(c.SoftmaxTemperature > 0) || math.IsInf(c.SoftmaxTemperature, 0) {
		return fmt.Errorf("softmax temperature must be finite and positive: %g", c.SoftmaxTemperature)
	}
	if math.IsNaN(c.ControlPenalty) || c.ControlPenalty < -1 || c.ControlPenalty > 1 {
		return fmt.Errorf("control penalty must be in [-1, 1]: %g", c.ControlPenalty)
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
	ID           int
	Site         Point
	Corners      []Point
	Neighbors    []int
	Desirability float64
	IslandID     int
	ControllerID int
}

type Island struct {
	ID      int
	SeedID  int
	CellIDs []int
}

type Result struct {
	Cells              []Cell
	Islands            []Island
	Attractants        []Point
	InitialIslandCount int
	MergeCount         int
	RoundsAttempted    int
	FinalOcean         float64
}
