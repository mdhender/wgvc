// Package x24 implements the desirability-field single-mesh growth algorithm
// described by issues #24 through #26.
package x24

import (
	"errors"
	"fmt"
	"math"

	"github.com/mdhender/wgvc/internal/aspectratio"
	"github.com/mdhender/wgvc/internal/singlemesh"
)

const Water = -1

var ErrStarved = errors.New("all islands starved before claiming the requested provinces")

type Point = singlemesh.Point

type Config struct {
	WorldSeed          uint64
	ProvinceCount      int
	IslandCount        int
	AspectRatio        string
	OceanPercentage    float64
	EdgeBarrierWidth   float64
	EdgeRamp           []float64
	AttractantCount    int
	AttractantRamp     []float64
	AttractantJitter   float64
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
		AspectRatio:        aspectratio.Default,
		OceanPercentage:    0.68,
		EdgeBarrierWidth:   0.02,
		EdgeRamp:           []float64{-1, -0.65, -0.40, -0.22, -0.10, -0.04, 0},
		AttractantCount:    0,
		AttractantRamp:     []float64{1, 0.78, 0.58, 0.42, 0.29, 0.18, 0.10, 0.04, 0.01, 0},
		AttractantJitter:   0.65,
		SoftmaxTemperature: 0.20,
		ControlPenalty:     -0.82,
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
	if _, _, err := aspectratio.Dimensions(c.AspectRatio); err != nil {
		return err
	}
	if !(c.OceanPercentage >= 0 && c.OceanPercentage <= maximumOcean) {
		return fmt.Errorf("ocean percentage must be in [0, %.2f]: %g", maximumOcean, c.OceanPercentage)
	}
	if math.IsNaN(c.EdgeBarrierWidth) || math.IsInf(c.EdgeBarrierWidth, 0) || c.EdgeBarrierWidth < 0 || c.EdgeBarrierWidth >= 0.5 {
		return fmt.Errorf("edge barrier width must be finite and in [0, 0.5): %g", c.EdgeBarrierWidth)
	}
	if len(c.EdgeRamp) == 0 {
		return fmt.Errorf("edge ramp must not be empty")
	}
	for i, value := range c.EdgeRamp {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < -1 || value > 0 {
			return fmt.Errorf("edge ramp value %d must be finite and in [-1, 0]: %g", i, value)
		}
		if i > 0 && value < c.EdgeRamp[i-1] {
			return fmt.Errorf("edge ramp must be nondecreasing: value %d is %g after %g", i, value, c.EdgeRamp[i-1])
		}
	}
	if c.EdgeRamp[len(c.EdgeRamp)-1] != 0 {
		return fmt.Errorf("edge ramp must end at 0: %g", c.EdgeRamp[len(c.EdgeRamp)-1])
	}
	if !validAttractantCount(c.AttractantCount) {
		return fmt.Errorf("attractant count must be one of 0, 1, 2, 3, 4, 5, 6, or 9: %d", c.AttractantCount)
	}
	if len(c.AttractantRamp) == 0 {
		return fmt.Errorf("attractant ramp must not be empty")
	}
	if !(c.AttractantRamp[0] > 0) {
		return fmt.Errorf("attractant ramp must start above 0: %g", c.AttractantRamp[0])
	}
	for i, value := range c.AttractantRamp {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return fmt.Errorf("attractant ramp value %d must be finite and in [0, 1]: %g", i, value)
		}
		if i > 0 && value > c.AttractantRamp[i-1] {
			return fmt.Errorf("attractant ramp must be nonincreasing: value %d is %g after %g", i, value, c.AttractantRamp[i-1])
		}
	}
	if c.AttractantRamp[len(c.AttractantRamp)-1] != 0 {
		return fmt.Errorf("attractant ramp must end at 0: %g", c.AttractantRamp[len(c.AttractantRamp)-1])
	}
	if math.IsNaN(c.AttractantJitter) || math.IsInf(c.AttractantJitter, 0) || c.AttractantJitter < 0 || c.AttractantJitter > 1 {
		return fmt.Errorf("attractant jitter must be finite and in [0, 1]: %g", c.AttractantJitter)
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
	LandEligible bool
	Desirability float64
	IslandID     int
	ControllerID int
}

type Island struct {
	ID      int
	SeedID  int
	CellIDs []int
}

type Attractant struct {
	CellID  int
	Point   Point
	RegionX int
	RegionY int
}

type AttractantSkip struct {
	RegionX int
	RegionY int
	Reason  string
}

type Result struct {
	Cells              []Cell
	Islands            []Island
	Attractants        []Attractant
	AttractantSkips    []AttractantSkip
	InitialIslandCount int
	MergeCount         int
	RoundsAttempted    int
	FinalOcean         float64
}
