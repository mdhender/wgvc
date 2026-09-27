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
	// Ocean is the ocean fraction the figure is drawn for, or 0 to leave the
	// default. The generate command uses it when -ocean is not given.
	Ocean float64
	Sites []Point
	// Weights is optional and parallel to Sites. A positive weight is the
	// site's share of growth draws relative to the others (a missing or
	// zero entry counts as 1). A negative weight makes the site a repulsor:
	// it seeds no island and no attractant, and pushes every frontier away
	// with the repulsor ramp scaled by the weight's magnitude.
	Weights []float64
	// Kin lists site indexes whose islands form one landmass: they ignore
	// each other's rival ramp, merge when they touch, and the merged
	// landmass keeps every member's share of growth. Sites not listed grow
	// their own island. Repulsor sites must not be listed.
	Kin [][]int
}

// weight returns the site's weight, defaulting to 1.
func (c Constellation) weight(site int) float64 {
	if site < len(c.Weights) && c.Weights[site] != 0 {
		return c.Weights[site]
	}
	return 1
}

// starSites lists the sites that seed islands, in order.
func (c Constellation) starSites() []int {
	stars := make([]int, 0, len(c.Sites))
	for site := range c.Sites {
		if c.weight(site) > 0 {
			stars = append(stars, site)
		}
	}
	return stars
}

// repulsorSites lists the sites with negative weight, in order.
func (c Constellation) repulsorSites() []int {
	repulsors := make([]int, 0)
	for site := range c.Sites {
		if c.weight(site) < 0 {
			repulsors = append(repulsors, site)
		}
	}
	return repulsors
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
	// Subaru is not a constellation but an archipelago in the shape of
	// Japan: a twelve-star mainland arcing from southwest to northeast, a
	// four-star northern island beyond a strait at its northeast end, a
	// two-star southwestern island, one island south of the mainland's
	// western half, and two half-weight islands off the western edge. Three
	// repulsors hold open an inland sea between the mainland and the
	// southern island, a bay on the mainland's south coast, and the strait
	// to the northern island. Sites are ordered so the mainland is seeded
	// first when fewer islands are asked for than stars.
	"subaru": {
		Name:        "subaru",
		AspectRatio: "5:2",
		Sites: []Point{
			{X: -0.59, Y: -0.25}, // mainland, west end
			{X: -0.47, Y: -0.24},
			{X: -0.36, Y: -0.23},
			{X: -0.24, Y: -0.20},
			{X: -0.13, Y: -0.17},
			{X: -0.01, Y: -0.14},
			{X: 0.10, Y: -0.11},
			{X: 0.22, Y: -0.06}, // the bend
			{X: 0.32, Y: 0.00},
			{X: 0.41, Y: 0.08},
			{X: 0.49, Y: 0.16},
			{X: 0.56, Y: 0.24}, // mainland, northeast end
			{X: 0.72, Y: 0.15}, // northern island
			{X: 0.86, Y: 0.08},
			{X: 0.84, Y: 0.20},
			{X: 1.00, Y: 0.15},
			{X: -0.77, Y: -0.07}, // southwestern island
			{X: -0.72, Y: -0.20},
			{X: -0.33, Y: -0.38}, // island south of the mainland
			{X: -0.92, Y: 0.13},  // small western islands
			{X: -1.00, Y: -0.16},
			{X: -0.35, Y: -0.30}, // repulsor: inland sea
			{X: 0.14, Y: -0.19},  // repulsor: bay on the south coast
			{X: 0.64, Y: 0.20},   // repulsor: strait to the northern island
		},
		Weights: []float64{
			1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
			1, 1, 1, 1,
			1, 1,
			1,
			0.5, 0.5,
			-0.8, -0.6, -0.4,
		},
		Kin: [][]int{
			{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			{12, 13, 14, 15},
			{16, 17},
		},
	},
	// Draco winds from a five-star head in the west, down through the neck to
	// Altais and Tyl, north through chi, Aldhibah, Athebyne, and theta to
	// Edasich, then east through Thuban and kappa to Giausar at the tail.
	// The head is drawn 2.2 times its sky size and closed with a fifth site
	// so its ring can hold an inland sea around a repulsor; the neck and the
	// rising body are spread apart so they stay two arms; sites are filled in
	// every 0.16 of the frame so the kin chain stays joined at 86% ocean; and
	// the tail tapers by weight. The whole dragon is one kin group.
	"draco": {
		Name:        "draco",
		AspectRatio: "16:9",
		Ocean:       0.86,
		Sites: []Point{
			{X: -1.00, Y: 0.09},  // Eltanin
			{X: -0.92, Y: 0.19},  // between Eltanin and Rastaban
			{X: -0.83, Y: 0.29},  // between Eltanin and Rastaban
			{X: -0.75, Y: 0.39},  // Rastaban
			{X: -0.65, Y: 0.32},  // between Rastaban and Kuma
			{X: -0.55, Y: 0.24},  // Kuma
			{X: -0.55, Y: 0.13},  // between Kuma and Grumium
			{X: -0.55, Y: 0.03},  // between Kuma and Grumium
			{X: -0.55, Y: -0.08}, // Grumium
			{X: -0.67, Y: -0.04}, // between Grumium and south of the head
			{X: -0.78, Y: 0.00},  // south of the head
			{X: -0.89, Y: 0.04},  // between south of the head and Eltanin
			{X: -0.51, Y: -0.26}, // Grumium's neck
			{X: -0.48, Y: -0.35}, // between Grumium's neck and Altais
			{X: -0.46, Y: -0.44}, // Altais
			{X: -0.42, Y: -0.51}, // between Altais and Tyl
			{X: -0.38, Y: -0.58}, // Tyl
			{X: -0.28, Y: -0.51}, // between Tyl and chi Draconis
			{X: -0.18, Y: -0.44}, // between Tyl and chi Draconis
			{X: -0.08, Y: -0.38}, // between Tyl and chi Draconis
			{X: 0.02, Y: -0.31},  // chi Draconis
			{X: -0.01, Y: -0.19}, // between chi Draconis and Aldhibah
			{X: -0.05, Y: -0.07}, // between chi Draconis and Aldhibah
			{X: -0.08, Y: 0.05},  // Aldhibah
			{X: -0.07, Y: 0.18},  // between Aldhibah and Athebyne
			{X: -0.06, Y: 0.32},  // Athebyne
			{X: -0.05, Y: 0.41},  // between Athebyne and theta Draconis
			{X: -0.04, Y: 0.49},  // theta Draconis
			{X: 0.03, Y: 0.53},   // between theta Draconis and Edasich
			{X: 0.10, Y: 0.58},   // Edasich
			{X: 0.22, Y: 0.55},   // between Edasich and Thuban
			{X: 0.33, Y: 0.51},   // between Edasich and Thuban
			{X: 0.45, Y: 0.48},   // Thuban
			{X: 0.56, Y: 0.40},   // between Thuban and kappa Draconis
			{X: 0.68, Y: 0.32},   // between Thuban and kappa Draconis
			{X: 0.79, Y: 0.24},   // kappa Draconis
			{X: 0.90, Y: 0.20},   // between kappa Draconis and Giausar
			{X: 1.00, Y: 0.16},   // Giausar
			{X: -0.73, Y: 0.14},  // repulsor: inland sea in the head
		},
		Weights: []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, -1},
		Kin:     [][]int{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37}},
	},
	// Aster is Draco with a chain of islands for a body: the same figure,
	// but only the twelve half-weight head sites are kin, ringing a repulsor
	// that leaves a bay or an inland sea, while every body site grows its
	// own island held apart from its neighbors by the rival ramp. Body sites
	// are spaced every 0.30 of the frame so the beads stay distinct.
	"aster": {
		Name:        "aster",
		AspectRatio: "16:9",
		Ocean:       0.78,
		Sites: []Point{
			{X: -1.00, Y: 0.09},  // Eltanin
			{X: -0.92, Y: 0.19},  // between Eltanin and Rastaban
			{X: -0.83, Y: 0.29},  // between Eltanin and Rastaban
			{X: -0.75, Y: 0.39},  // Rastaban
			{X: -0.65, Y: 0.32},  // between Rastaban and Kuma
			{X: -0.55, Y: 0.24},  // Kuma
			{X: -0.55, Y: 0.13},  // between Kuma and Grumium
			{X: -0.55, Y: 0.03},  // between Kuma and Grumium
			{X: -0.55, Y: -0.08}, // Grumium
			{X: -0.67, Y: -0.04}, // between Grumium and south of the head
			{X: -0.78, Y: 0.00},  // south of the head
			{X: -0.89, Y: 0.04},  // between south of the head and Eltanin
			{X: -0.51, Y: -0.26}, // Grumium's neck
			{X: -0.46, Y: -0.44}, // Altais
			{X: -0.38, Y: -0.58}, // Tyl
			{X: -0.18, Y: -0.44}, // between Tyl and chi Draconis
			{X: 0.02, Y: -0.31},  // chi Draconis
			{X: -0.03, Y: -0.13}, // between chi Draconis and Aldhibah
			{X: -0.08, Y: 0.05},  // Aldhibah
			{X: -0.06, Y: 0.32},  // Athebyne
			{X: -0.04, Y: 0.49},  // theta Draconis
			{X: 0.10, Y: 0.58},   // Edasich
			{X: 0.28, Y: 0.53},   // between Edasich and Thuban
			{X: 0.45, Y: 0.48},   // Thuban
			{X: 0.62, Y: 0.36},   // between Thuban and kappa Draconis
			{X: 0.79, Y: 0.24},   // kappa Draconis
			{X: 1.00, Y: 0.16},   // Giausar
			{X: -0.73, Y: 0.14},  // repulsor: bay or inland sea in the head
		},
		Weights: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0.7, 0.7, 0.7, 0.7, -1},
		Kin:     [][]int{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
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
