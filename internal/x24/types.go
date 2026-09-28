// Package x24 implements the desirability-field single-mesh growth algorithm
// described by issues #24 through #26 and the rival ramp from issue #42.
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
	WorldSeed        uint64
	ProvinceCount    int
	IslandCount      int
	AspectRatio      string
	OceanPercentage  float64
	EdgeBarrierWidth float64
	EdgeRamp         []float64
	AttractantCount  int
	// Constellation names an attractant arrangement that replaces the
	// regional placement selected by AttractantCount, which must then be 0.
	// It is placed without jitter and consumes no randomness.
	Constellation      string
	AttractantRamp     []float64
	AttractantJitter   float64
	SoftmaxTemperature float64
	RivalRamp          []float64
	// RepulsorRamp is the desirability a repulsor of full strength subtracts
	// by hop distance from its cell; weaker repulsors scale it.
	RepulsorRamp []float64
	MaxRounds    int
	Relaxations  int
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
		RivalRamp:          []float64{-1, -1, -0.5, 0},
		RepulsorRamp:       []float64{-1, -0.95, -0.90, -0.84, -0.78, -0.71, -0.64, -0.56, -0.48, -0.40, -0.33, -0.26, -0.20, -0.15, -0.10, -0.06, -0.03, -0.02, -0.01, 0},
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
	if c.Constellation != "" {
		if _, ok := ConstellationByName(c.Constellation); !ok {
			return fmt.Errorf("unknown constellation %q: want one of %v", c.Constellation, ConstellationNames())
		}
		if c.AttractantCount != 0 {
			return fmt.Errorf("attractant count must be 0 when a constellation is set: %d with %q", c.AttractantCount, c.Constellation)
		}
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
	if err := validateRivalRamp(c.RivalRamp); err != nil {
		return err
	}
	if err := validatePenaltyRamp("repulsor", c.RepulsorRamp); err != nil {
		return err
	}
	if c.MaxRounds < 1 {
		return fmt.Errorf("maximum rounds must be at least 1: %d", c.MaxRounds)
	}
	if c.Relaxations < 0 {
		return fmt.Errorf("relaxations must not be negative: %d", c.Relaxations)
	}
	return nil
}

// validateRivalRamp accepts a nondecreasing ramp of penalties in [-1, 0] that
// ends at 0, indexed by hop distance to the nearest rival land minus one.
func validateRivalRamp(ramp []float64) error {
	return validatePenaltyRamp("rival", ramp)
}

func validatePenaltyRamp(name string, ramp []float64) error {
	if len(ramp) == 0 {
		return fmt.Errorf("%s ramp must not be empty", name)
	}
	for i, value := range ramp {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < -1 || value > 0 {
			return fmt.Errorf("%s ramp value %d must be finite and in [-1, 0]: %g", name, i, value)
		}
		if i > 0 && value < ramp[i-1] {
			return fmt.Errorf("%s ramp must be nondecreasing: value %d is %g after %g", name, i, value, ramp[i-1])
		}
	}
	if ramp[len(ramp)-1] != 0 {
		return fmt.Errorf("%s ramp must end at 0: %g", name, ramp[len(ramp)-1])
	}
	return nil
}

// Cell is one Voronoi cell of the growth mesh. Generate reads Site and
// IslandID; Corners and Neighbors alias the mesh slices and are exposed for
// inspection and tests only, since the caller re-tessellates the sites to
// build the canonical world mesh.
type Cell struct {
	ID           int
	Site         Point
	Corners      []Point
	Neighbors    []int
	LandEligible bool
	Desirability float64
	IslandID     int
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

// Repulsor is a placed repulsor site: its cell, position, and strength in
// (0, 1], which scales the repulsor ramp.
type Repulsor struct {
	CellID   int
	Point    Point
	Strength float64
}

// AttractantSkip records a regional attractant or constellation site that was
// not placed: no cell had the edge clearance, or every eligible cell near a
// constellation site already hosted an earlier site. The region is the 3×3
// grid cell holding the site's target.
type AttractantSkip struct {
	RegionX int
	RegionY int
	Reason  string
}

// SeedFallback records an island whose pinned seed cell (a constellation
// star) could not be used because the cell was already claimed or is not
// eligible for land, so the island drew a uniform seed instead. IslandID is
// the island's initial index, which is also its star's index among the
// placed attractants.
type SeedFallback struct {
	IslandID int
	CellID   int // the pinned cell
	SeedID   int // the cell seeded instead
	Reason   string
}

type Result struct {
	Cells              []Cell
	Islands            []Island
	Attractants        []Attractant
	Repulsors          []Repulsor
	AttractantSkips    []AttractantSkip
	SeedFallbacks      []SeedFallback
	InitialIslandCount int
	MergeCount         int
	RoundsAttempted    int
	FinalOcean         float64
}
