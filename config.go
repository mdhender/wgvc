package wgvc

import "fmt"

// Config controls deterministic world generation.
type Config struct {
	// WorldSeed controls all deterministic random streams.
	WorldSeed uint64
	// ProvinceCount is the number of land provinces. Generate also returns the
	// surrounding water provinces used to shape the islands.
	ProvinceCount int
	// IslandCount is the number of connected land components.
	IslandCount int
}

func (c Config) validate() error {
	if c.IslandCount < 1 {
		return fmt.Errorf("island count must be at least 1: %d", c.IslandCount)
	}
	if c.ProvinceCount < c.IslandCount {
		return fmt.Errorf("province count must be at least island count: provinces=%d islands=%d", c.ProvinceCount, c.IslandCount)
	}
	return nil
}
