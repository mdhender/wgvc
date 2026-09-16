# Generation contracts

`Generate` accepts a world seed, province count, and island count. Counts must
satisfy `ProvinceCount >= IslandCount >= 1`. The current generation stage
creates the exact requested number of canonical islands and provinces and
allocates at least one province to every island. Private island planning now
provides separated footprints and normalized province centers to the later
geometry stage. Private geometry now turns those centers into canonical
unit-square Voronoi meshes with shared corners, shared edges, and authoritative
adjacency. Integration into `Generate`, world-space conversion, global ID
assignment, and terrain generation remain deferred; public centers, corners,
edges, and polygon rings therefore remain empty for now.

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

## Island planning

Island planning is currently private input to the later geometry stage. An
island allocated `n` provinces receives a square world-space footprint with
side length `sqrt(n)`, so its area is exactly proportional to its allocation.
Footprints occupy row-major slots in a near-square regular grid. The slot pitch
includes a one-unit water gap plus enough slack for independently sampled
positional jitter of at most 0.25 units per axis. Consequently every pair of
footprints has at least a one-unit water gap. Island IDs retain allocation
order and are not spatially resorted. World bounds are the complete footprint
extent expanded by a one-unit water margin on every side.

Each island's province centers remain normalized to the open unit square for
the geometry backend. The square is recursively split along its longest axis;
requested counts are divided as evenly as possible and split area is exactly
proportional to those counts. Every resulting leaf therefore has area `1/n`.
A center is sampled from the middle half of each leaf along both axes, using
the island's independent random stream. This inset makes centers distinct and
gives every pair a separation of at least `0.5/n` in normalized coordinates,
without rejection sampling or retries.
