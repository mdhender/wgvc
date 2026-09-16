package wgvc

// IslandID identifies an island by its index in World.Islands.
type IslandID int

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
	TerrainPlains    Terrain = "plains"
	TerrainHills     Terrain = "hills"
	TerrainMountains Terrain = "mountains"
)

// World contains canonically ordered world data. Every object's ID equals its
// index in the corresponding collection.
type World struct {
	Islands   []Island
	Provinces []Province
	Corners   []Corner
	Edges     []Edge
}

// Island groups the provinces in one connected land component. ProvinceIDs is
// in ascending canonical province order.
type Island struct {
	ID          IslandID
	ProvinceIDs []ProvinceID
}

// Province is one territorial cell. Center is the generating point used for
// its Voronoi cell, not the polygon centroid. CornerIDs is a counterclockwise
// polygon ring and does not repeat its closing corner.
type Province struct {
	ID        ProvinceID
	IslandID  IslandID
	Center    Point
	CornerIDs []CornerID
	Terrain   Terrain
}

// Corner is a vertex shared by province polygons.
type Corner struct {
	ID    CornerID
	Point Point
}

// Edge is an undirected geometric boundary, never a route. CornerIDs contains
// its two endpoints. ProvinceIDs contains one province for a coastline edge or
// two provinces for an interior edge; that incidence is authoritative
// geometric adjacency.
type Edge struct {
	ID          EdgeID
	CornerIDs   [2]CornerID
	ProvinceIDs []ProvinceID
}
