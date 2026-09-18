package wgvc

import (
	"fmt"
	"math"

	"github.com/mdhender/wgvc/internal/aspectratio"
)

// AspectRatio names a map shape using an alias or width:height notation.
type AspectRatio string

const (
	AspectRatioSquare     AspectRatio = "1:1"
	AspectRatioStandard   AspectRatio = "4:3"
	AspectRatioWidescreen AspectRatio = "16:9"
	AspectRatioCinema     AspectRatio = "2.39:1"
	AspectRatioUltrawide  AspectRatio = "21:9"
	AspectRatioPortrait   AspectRatio = "9:16"
)

// Config controls deterministic world generation.
type Config struct {
	// WorldSeed controls all deterministic random streams.
	WorldSeed uint64
	// ProvinceCount is the number of land provinces. Generate also returns the
	// surrounding water provinces used to shape the islands.
	ProvinceCount int
	// IslandCount is the number of islands seeded before growth. Islands may
	// merge, so the generated world can contain fewer connected land components.
	IslandCount int
	// AspectRatio controls the map's shape without changing its area or point
	// budget. The zero value defaults to AspectRatioSquare.
	AspectRatio AspectRatio
	// PolarIce is the desired fraction of ocean provinces in the polar heat
	// band. The zero value uses the default.
	PolarIce float64
	// PeakChill is the desired fraction of warm-region high peaks classified
	// cold or colder. The zero value uses the default.
	PeakChill float64
}

// ClimateConfig controls climate calibration for the renderer's extended
// generation entry point.
type ClimateConfig struct {
	PolarIce  float64
	PeakChill float64
}

const (
	defaultPolarIce  = 0.05
	defaultPeakChill = 0.20
)

// DefaultClimateConfig returns the default climate calibration targets.
func DefaultClimateConfig() ClimateConfig {
	return ClimateConfig{PolarIce: defaultPolarIce, PeakChill: defaultPeakChill}
}

func (c Config) validate() error {
	if c.IslandCount < 1 {
		return fmt.Errorf("island count must be at least 1: %d", c.IslandCount)
	}
	if c.ProvinceCount < c.IslandCount {
		return fmt.Errorf("province count must be at least island count: provinces=%d islands=%d", c.ProvinceCount, c.IslandCount)
	}
	if _, _, err := c.aspectDimensions(); err != nil {
		return err
	}
	if _, err := normalizeClimateConfig(ClimateConfig{PolarIce: c.PolarIce, PeakChill: c.PeakChill}); err != nil {
		return err
	}
	return nil
}

func (c Config) aspectDimensions() (width, height float64, err error) {
	return aspectratio.Dimensions(string(c.AspectRatio))
}

func normalizeClimateConfig(config ClimateConfig) (ClimateConfig, error) {
	if config.PolarIce == 0 {
		config.PolarIce = defaultPolarIce
	}
	if config.PeakChill == 0 {
		config.PeakChill = defaultPeakChill
	}
	if math.IsNaN(config.PolarIce) || math.IsInf(config.PolarIce, 0) || config.PolarIce < 0 || config.PolarIce > 1 {
		return ClimateConfig{}, fmt.Errorf("polar ice must be finite and in [0, 1]: %g", config.PolarIce)
	}
	if math.IsNaN(config.PeakChill) || math.IsInf(config.PeakChill, 0) || config.PeakChill < 0 || config.PeakChill > 1 {
		return ClimateConfig{}, fmt.Errorf("peak chill must be finite and in [0, 1]: %g", config.PeakChill)
	}
	return config, nil
}
