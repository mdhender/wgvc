package x24

import (
	"slices"
)

// Constellation is a named arrangement of attractant sites. Sites are in a
// centered frame with x and y in [-1, 1], oriented with y up (north) and the
// longer extent spanning the full range. Placement scales the frame uniformly
// to fit the map, so the shape keeps its proportions on any aspect ratio.
type Constellation struct {
	Name string
	// AspectRatio is the map shape the figure is drawn for. The generate
	// command uses it when -aspect is not given.
	AspectRatio string
	Sites       []Point
}

// constellations maps a -attractors name to its sites. Positions come from
// the stars' right ascension and declination projected onto the celestial
// pole, rotated so the figure's long axis lies west to east.
var constellations = map[string]Constellation{
	// Ursa Minor, the Little Dipper: the handle runs from Polaris at the west
	// end through Yildun and epsilon to zeta, and the bowl is zeta, eta,
	// Pherkad, and Kochab.
	"ursa-minor": {
		Name:        "ursa-minor",
		AspectRatio: "2:1",
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
	// Cygnus, the Northern Cross: the body runs north to south from Deneb
	// through Sadr and eta to Albireo, and the wings run from zeta through
	// Gienah, Sadr, and delta to iota. Kappa is omitted because it sits too
	// close to iota for two islands.
	"cygnus": {
		Name:        "cygnus",
		AspectRatio: "3:2",
		Sites: []Point{
			{X: -0.03, Y: 0.76},  // Deneb
			{X: -0.04, Y: 0.33},  // Sadr
			{X: 0.01, Y: -0.16},  // eta Cygni
			{X: -0.03, Y: -0.76}, // Albireo
			{X: 0.56, Y: 0.28},   // delta Cygni
			{X: 1.00, Y: 0.51},   // iota Cygni
			{X: -0.56, Y: 0.19},  // Gienah
			{X: -1.00, Y: 0.21},  // zeta Cygni
		},
	},
	// Virgo: a sprawling figure along the ecliptic. One arm runs from
	// Zavijava in the east through Zaniah, Porrima, and Auva up to
	// Vindemiatrix; the body drops from Porrima through theta to Spica and
	// back up through Heze to tau; the tail runs from Heze through Syrma to
	// mu in the west.
	"virgo": {
		Name:        "virgo",
		AspectRatio: "2:1",
		Sites: []Point{
			{X: 1.00, Y: 0.09},   // Zavijava
			{X: 0.66, Y: -0.03},  // Zaniah
			{X: 0.41, Y: -0.06},  // Porrima
			{X: 0.25, Y: 0.16},   // Auva
			{X: 0.17, Y: 0.51},   // Vindemiatrix
			{X: 0.08, Y: -0.25},  // theta Virginis
			{X: -0.10, Y: -0.51}, // Spica
			{X: -0.21, Y: -0.02}, // Heze
			{X: -0.52, Y: 0.08},  // tau Virginis
			{X: -0.69, Y: -0.27}, // Syrma
			{X: -1.00, Y: -0.26}, // mu Virginis
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
