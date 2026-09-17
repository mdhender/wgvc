# Generation contracts

`Generate` accepts a world seed, province count, island count, and named
width:height aspect ratio. Counts must
satisfy `ProvinceCount >= IslandCount >= 1`. It returns exactly the requested
number of land provinces. `IslandCount` specifies initial seeds; growth can
merge them, so the returned canonical island count is between one and the
requested count. Water can contain interior components such as lakes.

## Coordinates and topology

- Coordinates are Cartesian.
- Polygon corner rings run counterclockwise and do not repeat the closing
  vertex.
- Every ID is the zero-based position of the object in its canonical world
  collection. Membership lists use that same canonical order.
- A province center is its Voronoi generating point, not its polygon centroid.
- Land provinces identify their island. Water provinces use `NoIslandID`.
- An interior edge's incident provinces are the authoritative undirected
  geometric adjacency. An edge is never a game route.

## Single-mesh generation

Generation begins with one deterministic point set over a fixed-area rectangle.
Its width and height come from the configured aspect ratio; `1:1` is the
zero-value default. The point count is derived from the requested land count
and an initial 68% ocean
fraction. Two Lloyd relaxation passes even the spacing without introducing a
grid, and one Voronoi diagram supplies every eventual land and water cell.

Cells entering the outer permanent barrier are ineligible for land. A graph-hop
edge ramp raises desirability from strongly negative near that barrier to
neutral in the interior. The default configuration has no attractants;
calibration can add up to nine jittered regional attractants where the
configured edge and attractant ramps leave enough clearance.

Each island receives one uniformly selected seed. Claims give the island
one-hop control over neighboring cells, making those cells less desirable to
rivals while preserving their original value for the controller. Islands are
drawn from a deterministic deck and choose frontier cells through a softmax
weighted by the visible desirability. When a claim directly connects rival
land, all connected rivals merge immediately. Growth stops after exactly the
requested number of land cells has been claimed. No water-connectivity filter
is applied, so interior lakes are valid.

If a round cannot seed the islands and claim the requested land, the complete
attempt is discarded.
The next deterministic round adds three percentage points of ocean, up to 95%,
and creates a fresh point set. Production allows ten rounds. A round is a
separate derived random stream, so retries remain reproducible rather than
depending on mutable global state.

After growth, the mesh is uniformly scaled so total land area equals the land
province count. A typical land province therefore has area one in world
coordinates. Canonicalization assigns shared world-global corner and edge IDs,
and terrain is assigned only after geometry and topology are final.

## Determinism

Randomness uses local `math/rand/v2` PCG generators. The world seed and round
number derive each growth stream through SplitMix64; terrain uses an independent
domain-separated stream. No package-global random source is used. Identical
configurations produce deeply identical worlds for a fixed generator version.

See [Single-mesh island generation](blob-islands.md) for visual fixtures and
regression coverage.
