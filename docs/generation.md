# Generation contracts

`Generate` accepts a world seed, province count, and island count. Counts must
satisfy `ProvinceCount >= IslandCount >= 1`. The current generation stage
creates the exact requested number of canonical islands and provinces and
allocates at least one province to every island. Island planning, geometry, and
terrain generation are deliberately deferred; corners, edges, and province
polygon rings therefore remain empty for now.

## Coordinates and topology

- Coordinates are Cartesian.
- Polygon corner rings run counterclockwise and do not repeat the closing
  vertex.
- Every ID is the zero-based position of the object in its canonical world
  collection. Membership lists use that same canonical order.
- A province center is its Voronoi generating point, not its polygon centroid.
- An interior edge's incident provinces are the authoritative undirected
  geometric adjacency. An edge is never a game route.

## Allocation

The allocator reserves one province for every island. It apportions the
remainder in proportion to descending Fibonacci weights, using exact
`math/big.Int` arithmetic. Truncated units are assigned by descending
fractional remainder, with lower island IDs winning ties. This guarantees an
exact total without imposing an arbitrary island-count cap.

## Random streams

Randomness uses local `math/rand/v2` PCG generators. The world seed is combined
with fixed placement, per-island province, and terrain domains and expanded
into PCG's two seeds with SplitMix64. Recreating a stream recreates its
sequence, and consuming one stage's stream cannot alter another stage's
sequence. No package-global random source is used.
