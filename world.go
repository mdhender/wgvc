package wgvc

// IslandID identifies an island by its index in World.Islands.
type IslandID int

// NoIslandID is the IslandID of every water province.
const NoIslandID IslandID = -1

// ProvinceID identifies a province by its index in World.Provinces.
type ProvinceID int

// CornerID identifies a polygon corner by its index in World.Corners.
type CornerID int

// EdgeID identifies an edge by its index in World.Edges.
type EdgeID int

// Point is a Cartesian coordinate.
type Point struct {
	X float64
	Y float64
}

// Terrain is a province's terrain classification.
type Terrain string

const (
	TerrainWater     Terrain = "water"
	TerrainPlains    Terrain = "plains"
	TerrainHills     Terrain = "hills"
	TerrainMountains Terrain = "mountains"
)

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

// World contains canonically ordered world data. Every object's ID equals its
// index in the corresponding collection.
type World struct {
	Islands   []Island
	Provinces []Province
	Corners   []Corner
	Edges     []Edge
}

// Island groups the land provinces in one connected component. ProvinceIDs is
// in ascending canonical province order; surrounding water provinces are not
// members of an island.
type Island struct {
	ID          IslandID
	ProvinceIDs []ProvinceID
}

// Province is one land or water cell. IslandID identifies the containing
// island for land and is NoIslandID for water. Only land provinces appear in
// Island.ProvinceIDs. Center is the generating point used for its Voronoi
// cell, not the polygon centroid. CornerIDs is a counterclockwise polygon ring
// without a repeated closing corner. Elevation is the mean elevation of the
// province's boundary edges, normalized to [-1, 1]. ElevationBand preserves
// the growth-assigned land/water classification even at sea level.
type Province struct {
	ID            ProvinceID
	IslandID      IslandID
	Center        Point
	CornerIDs     []CornerID
	Terrain       Terrain
	Elevation     float64
	ElevationBand ElevationBand
}

// Corner is a vertex shared by province polygons.
type Corner struct {
	ID    CornerID
	Point Point
}

// Edge is an undirected geometric boundary, never a route. CornerIDs contains
// its two endpoints. ProvinceIDs contains one province at the outside of the
// world or two provinces inside it. A coastline has one land and one water
// province; incidence is authoritative geometric adjacency. Elevation is
// normalized to [-1, 1], with zero representing sea level.
type Edge struct {
	ID          EdgeID
	CornerIDs   [2]CornerID
	ProvinceIDs []ProvinceID
	Elevation   float64
}
