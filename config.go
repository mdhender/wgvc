package wgvc

import (
	"fmt"

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
	return nil
}

func (c Config) aspectDimensions() (width, height float64, err error) {
	return aspectratio.Dimensions(string(c.AspectRatio))
}
