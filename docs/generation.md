# Generation contracts

`Generate` accepts a world seed, province count, and island count. Counts must
satisfy `ProvinceCount >= IslandCount >= 1`. It creates the exact requested
number of canonical islands and provinces and allocates at least one province
to every island. Island planning creates private candidate sites and shape
parameters. Geometry tessellates the complete candidate maps, selects exact
connected blobs, and extracts only retained land. Placement scales retained
land area and separates the complete candidate envelopes. `Generate` then
assigns world-global IDs and terrain and returns complete province centers and
polygon rings.

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
with fixed placement, per-island candidate-site, per-island blob-shape, and
terrain domains and expanded into PCG's two seeds with SplitMix64. Recreating
a stream recreates its sequence, and consuming one stage's stream cannot alter
another stage's sequence. No package-global random source is used.

## Island planning

Island planning remains private input to later geometry stages. An island
allocated `n` land provinces receives the smallest square interior candidate
grid with capacity at least `2n`, surrounded by a one-cell frame. One site is
sampled from the inset center half of each grid slot. The complete map is
tessellated in the unit square before land selection, so private ocean cells
constrain the final coastline.

A deterministic radial score with independent three-fold and five-fold shape
phases grows exactly `n` edge-connected interior cells from the center. Frame
cells are never eligible. Row-convex growth ensures every omitted cell remains
connected to clipping-frame water, so the generator introduces no lakes. The
selected cells are extracted without retessellation: omitted cells and unused
topology are removed, references are compacted, and land/ocean boundaries
become one-incidence coastline edges.

Placement measures each retained mesh and applies a uniform scale so its
world-space land area equals `n`. It lays out the complete candidate envelopes,
not merely the tighter land bounds, in row-major slots with a one-unit water
gap and bounded positional jitter. Consequently private ocean still determines
separation even though it is absent from `World`. Island IDs retain allocation
order and are not spatially resorted.

## World assembly

Islands are assembled in island ID order, and each island's provinces retain
their selected original-candidate order. Corners and edges retain canonical
mesh order within each island. Local province and corner references are offset
into their public world collections, so every public ID equals its collection
index. Uniform positive scale and translation preserve polygon orientation,
Voronoi generating centers, and shared topology. No edge is shared across
islands. Terrain is assigned only after this geometry and topology are final.

See [Blob-island generation](blob-islands.md) for the complete stage order,
visual fixtures, and resolution limits.
