package wgvc

// IslandID identifies an island by its index in World.Islands.
type IslandID int

// NoIslandID is the IslandID of every water province.
const NoIslandID IslandID = -1

// BasinID identifies an enclosed water body by its index in World.Basins.
type BasinID int

// NoBasinID is the BasinID of every land province and every ocean province.
const NoBasinID BasinID = -1

// ProvinceID identifies a province by its index in World.Provinces.
type ProvinceID int

// NoProvinceID is the NeighborID of an exit on the world boundary.
const NoProvinceID ProvinceID = -1

// CornerID identifies a polygon corner by its index in World.Corners.
type CornerID int

// EdgeID identifies an edge by its index in World.Edges.
type EdgeID int

// RiverID identifies a river by its index in World.Rivers.
type RiverID int

// NoRiverID is the RiverID of every edge that carries no river.
const NoRiverID RiverID = -1

// Point is a Cartesian coordinate.
type Point struct {
	X float64
	Y float64
}

// Terrain is a province's terrain classification.
type Terrain string

const (
	TerrainDeepOcean        Terrain = "deep-ocean"
	TerrainOcean            Terrain = "ocean"
	TerrainShallowSea       Terrain = "shallow-sea"
	TerrainCoastalWater     Terrain = "coastal-water"
	TerrainInlandSea        Terrain = "inland-sea"
	TerrainLake             Terrain = "lake"
	TerrainGlacialIce       Terrain = "glacial-ice"
	TerrainTundra           Terrain = "tundra"
	TerrainMarsh            Terrain = "marsh"
	TerrainSwamp            Terrain = "swamp"
	TerrainBog              Terrain = "bog"
	TerrainDesert           Terrain = "desert"
	TerrainBadlands         Terrain = "badlands"
	TerrainScrubland        Terrain = "scrubland"
	TerrainPlains           Terrain = "plains"
	TerrainGrassland        Terrain = "grassland"
	TerrainSteppe           Terrain = "steppe"
	TerrainSavanna          Terrain = "savanna"
	TerrainBorealForest     Terrain = "boreal-forest"
	TerrainTemperateForest  Terrain = "temperate-forest"
	TerrainRainforest       Terrain = "rainforest"
	TerrainHills            Terrain = "hills"
	TerrainMountain         Terrain = "mountain"
	TerrainAlpine           Terrain = "alpine"
	TerrainVolcano          Terrain = "volcano"
	TerrainVolcanicHighland Terrain = "volcanic-highland"
	TerrainCoast            Terrain = "coast"
)

var terrains = [...]Terrain{
	TerrainDeepOcean, TerrainOcean, TerrainShallowSea, TerrainCoastalWater,
	TerrainInlandSea, TerrainLake,
	TerrainGlacialIce, TerrainTundra,
	TerrainMarsh, TerrainSwamp, TerrainBog,
	TerrainDesert, TerrainBadlands, TerrainScrubland,
	TerrainPlains, TerrainGrassland, TerrainSteppe, TerrainSavanna,
	TerrainBorealForest, TerrainTemperateForest, TerrainRainforest,
	TerrainHills, TerrainMountain, TerrainAlpine,
	TerrainVolcano, TerrainVolcanicHighland,
	TerrainCoast,
}

// Terrains returns every terrain in stable family order.
func Terrains() []Terrain {
	return append([]Terrain(nil), terrains[:]...)
}

// Valid reports whether terrain is a declared terrain value.
func (terrain Terrain) Valid() bool {
	for _, candidate := range terrains {
		if terrain == candidate {
			return true
		}
	}
	return false
}

// IsWater reports whether terrain represents ocean or inland water.
func (terrain Terrain) IsWater() bool {
	switch terrain {
	case TerrainDeepOcean, TerrainOcean, TerrainShallowSea, TerrainCoastalWater, TerrainInlandSea, TerrainLake:
		return true
	default:
		return false
	}
}

// ElevationBand is an ordered classification of province elevation. Water
// bands sort below land bands so callers can compare bands directly.
type ElevationBand int

const (
	ElevationBandDeepWater ElevationBand = iota
	ElevationBandShallowWater
	ElevationBandLowland
	ElevationBandUpland
	ElevationBandHighland
	ElevationBandMountain
)

var elevationBands = [...]ElevationBand{
	ElevationBandDeepWater, ElevationBandShallowWater,
	ElevationBandLowland, ElevationBandUpland, ElevationBandHighland, ElevationBandMountain,
}

// ElevationBands returns every elevation band from lowest to highest.
func ElevationBands() []ElevationBand {
	return append([]ElevationBand(nil), elevationBands[:]...)
}

// HeatBand is an ordered classification from coldest to warmest.
type HeatBand int

const (
	HeatBandPolar HeatBand = iota
	HeatBandCold
	HeatBandTemperate
	HeatBandWarm
	HeatBandHot
)

var heatBands = [...]HeatBand{HeatBandPolar, HeatBandCold, HeatBandTemperate, HeatBandWarm, HeatBandHot}

// HeatBands returns every heat band from coldest to warmest.
func HeatBands() []HeatBand {
	return append([]HeatBand(nil), heatBands[:]...)
}

// MoistureBand is an ordered classification from driest to wettest.
type MoistureBand int

const (
	MoistureBandArid MoistureBand = iota
	MoistureBandDry
	MoistureBandModerate
	MoistureBandHumid
	MoistureBandSaturated
)

var moistureBands = [...]MoistureBand{
	MoistureBandArid, MoistureBandDry, MoistureBandModerate, MoistureBandHumid, MoistureBandSaturated,
}

// MoistureBands returns every moisture band from driest to wettest.
func MoistureBands() []MoistureBand {
	return append([]MoistureBand(nil), moistureBands[:]...)
}

// World contains canonically ordered world data. Every object's ID equals its
// index in the corresponding collection.
type World struct {
	Islands   []Island
	Basins    []Basin
	Provinces []Province
	Corners   []Corner
	Edges     []Edge
	Rivers    []River
}

// Island groups the land provinces in one connected component. ProvinceIDs is
// in ascending canonical province order; surrounding water provinces are not
// members of an island.
type Island struct {
	ID          IslandID
	ProvinceIDs []ProvinceID
}

// Basin is a connected body of water provinces with no path to the world
// boundary: a lake or inland sea enclosed by land. ProvinceIDs is in ascending
// canonical province order, and basins are ordered by their lowest province
// ID. SurfaceElevation is the lowest elevation among the land provinces
// bordering the basin, the height at which it would spill, in [0, 1). Member
// provinces keep their water elevation as depth below that surface; Depth is
// the greatest such depth, in [0, 1).
type Basin struct {
	ID               BasinID
	ProvinceIDs      []ProvinceID
	SurfaceElevation float64
	Depth            float64
}

// Province is one land or water cell. IslandID identifies the containing
// island for land and is NoIslandID for water. Only land provinces appear in
// Island.ProvinceIDs. BasinID identifies the enclosing basin for water that
// cannot reach the world boundary and is NoBasinID otherwise; it never changes
// whether a province is land or water. Center is the generating point used for its Voronoi
// cell, not the polygon centroid. CornerIDs is a counterclockwise polygon ring
// without a repeated closing corner. EdgeIDs lists the boundary edges in the
// same ring order: EdgeIDs[i] joins CornerIDs[i] to CornerIDs[(i+1) % n].
// Exits lists the same edges numbered clockwise from north, in Number order.
// Area is the polygon area in world units. Elevation is the mean elevation of the
// province's boundary edges, normalized to [-1, 1]. ElevationBand preserves
// the growth-assigned land/water classification even at sea level. Relief is
// the mean absolute elevation difference to neighbors across shared edges,
// counting only land-land or water-water edges, and is 0 for a province with
// no such neighbor. Relief, Heat, and Moisture are normalized to [0, 1]; Heat
// and Moisture retain their ordered classifications.
type Province struct {
	ID            ProvinceID
	IslandID      IslandID
	BasinID       BasinID
	Center        Point
	CornerIDs     []CornerID
	EdgeIDs       []EdgeID
	Exits         []Exit
	Area          float64
	Terrain       Terrain
	Elevation     float64
	ElevationBand ElevationBand
	Relief        float64
	Heat          float64
	HeatBand      HeatBand
	Moisture      float64
	MoistureBand  MoistureBand
}

// Exit is one boundary edge of a province seen as a way out of it. Exits are
// numbered from 1, clockwise from north, so 0 stays free for "stay here".
// Bearing is the outward normal of the edge in degrees clockwise from north,
// in [0, 360); because the edge lies on the perpendicular bisector between the
// two Voronoi centers, it is also the bearing from this province's center to
// the neighbor's. Compass is the nearest 8-point label, for reports only;
// two exits can share one. NeighborID is the province across the edge, or
// NoProvinceID on the world boundary. Exits describe geometry, never
// passability or routes.
type Exit struct {
	Number     int
	EdgeID     EdgeID
	NeighborID ProvinceID
	Bearing    float64
	Compass    Compass
}

// Compass is an 8-point compass label.
type Compass string

const (
	CompassNorth     Compass = "N"
	CompassNortheast Compass = "NE"
	CompassEast      Compass = "E"
	CompassSoutheast Compass = "SE"
	CompassSouth     Compass = "S"
	CompassSouthwest Compass = "SW"
	CompassWest      Compass = "W"
	CompassNorthwest Compass = "NW"
)

// Corner is a vertex shared by province polygons. Elevation is the seeded
// elevation noise at the corner on the same scale as Edge.Elevation: a corner
// whose provinces are all land lies in [0.1, 1), one whose provinces are all
// water in (-1, -0.1], and a corner on a coastline is exactly 0, sea level.
// An edge's elevation is the mean of its two corners.
type Corner struct {
	ID        CornerID
	Point     Point
	Elevation float64
}

// RiverClass is a river's size at its mouth, from its discharge.
type RiverClass string

const (
	RiverClassStream     RiverClass = "stream"
	RiverClassRiver      RiverClass = "river"
	RiverClassMajorRiver RiverClass = "major-river"
)

// RiverClasses returns the river classes in ascending order of size.
func RiverClasses() []RiverClass {
	return []RiverClass{RiverClassStream, RiverClassRiver, RiverClassMajorRiver}
}

// Navigable reports whether boats can use a river of this class: rivers and
// major rivers, not streams.
func (class RiverClass) Navigable() bool {
	return class == RiverClassRiver || class == RiverClassMajorRiver
}

// RiverEndKind says what a river rises from or empties into.
type RiverEndKind string

const (
	// RiverEndSpring is a source with no upstream river.
	RiverEndSpring RiverEndKind = "spring"
	// RiverEndBasin is a lake or inland sea: a source that is its outflow,
	// or a mouth that feeds it.
	RiverEndBasin RiverEndKind = "basin"
	// RiverEndOcean is a mouth on ocean water.
	RiverEndOcean RiverEndKind = "ocean"
	// RiverEndRiver is a mouth at a confluence with a larger river.
	RiverEndRiver RiverEndKind = "river"
)

// RiverEnd is one end of a river. BasinID is set for RiverEndBasin and
// RiverID for RiverEndRiver; each is -1 otherwise.
type RiverEnd struct {
	Kind    RiverEndKind
	BasinID BasinID
	RiverID RiverID
}

// River is a chain of province edges that water flows along, from source to
// mouth. CornerIDs lists the corners in flow order, one more than EdgeIDs;
// EdgeIDs[i] joins CornerIDs[i] to CornerIDs[i+1]. Every edge lies between
// two land provinces, so rivers never cross water. Discharge is the flow on
// the last edge, in moisture-weighted province-area units, and Class is
// derived from it. At a confluence the branch with the larger discharge
// continues and the smaller ends with a RiverEndRiver mouth, so each edge
// belongs to exactly one river. Rivers are ordered by descending discharge.
type River struct {
	ID        RiverID
	CornerIDs []CornerID
	EdgeIDs   []EdgeID
	Class     RiverClass
	Discharge float64
	Source    RiverEnd
	Mouth     RiverEnd
}

// Edge is an undirected geometric boundary, never a route. CornerIDs contains
// its two endpoints. ProvinceIDs contains one province at the outside of the
// world or two provinces inside it. A coastline has one land and one water
// province; incidence is authoritative geometric adjacency. Length is the
// Euclidean distance between the two corners in world units. Elevation is
// normalized to [-1, 1], with zero representing sea level. RiverID names the
// river flowing along the edge, or NoRiverID; Discharge is the water flowing
// along it in moisture-weighted province-area units, and is set on every
// edge drainage uses, river or not.
type Edge struct {
	ID          EdgeID
	CornerIDs   [2]CornerID
	ProvinceIDs []ProvinceID
	Length      float64
	Elevation   float64
	RiverID     RiverID
	Discharge   float64
}
