package x24

import (
	"slices"
)

// Constellation is a named arrangement of attractant sites. Sites are in a
// centered frame with x and y in [-1, 1], oriented with y up (north) and the
// longer extent spanning the full range. Placement scales the frame uniformly
// to fit the map, so the shape keeps its proportions on any aspect ratio.
type Constellation struct {
	Name  string
	Sites []Point
}

// constellations maps a -attractors name to its sites. Positions come from
// the stars' right ascension and declination projected onto the celestial
// pole, rotated so the figure's long axis lies west to east.
var constellations = map[string]Constellation{
	// Ursa Minor, the Little Dipper: the handle runs from Polaris at the west
	// end through Yildun and epsilon to zeta, and the bowl is zeta, eta,
	// Pherkad, and Kochab.
	"ursa-minor": {
		Name: "ursa-minor",
		Sites: []Point{
			{X: -1.00, Y: -0.01}, // Polaris
			{X: -0.61, Y: 0.16},  // Yildun
			{X: -0.13, Y: 0.26},  // epsilon Ursae Minoris
			{X: 0.37, Y: 0.06},   // zeta Ursae Minoris
			{X: 0.55, Y: 0.30},   // eta Ursae Minoris
			{X: 1.00, Y: -0.09},  // Pherkad
			{X: 0.73, Y: -0.30},  // Kochab
		},
	},
}

// ConstellationByName returns the named constellation.
func ConstellationByName(name string) (Constellation, bool) {
	constellation, ok := constellations[name]
	return constellation, ok
}

// ConstellationNames lists the known constellation names in sorted order.
func ConstellationNames() []string {
	names := make([]string, 0, len(constellations))
	for name := range constellations {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
