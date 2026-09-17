# Generation contracts

`Generate` accepts a world seed, province count, and island count. Counts must
satisfy `ProvinceCount >= IslandCount >= 1`. It returns exactly the requested
number of canonical islands and land provinces, with at least one land province
per island, plus a connected world-level ocean.

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

Generation begins with one deterministic point set over the unit square. The
point count is derived from the requested land count and an initial 68% ocean
fraction. Two Lloyd relaxation passes even the spacing without introducing a
grid, and one Voronoi diagram supplies every eventual land and water cell.

Each island receives one seed cell at least three graph hops from the map
boundary and other islands. Islands then grow concurrently: an island is drawn
from a deterministic random deck and claims a random legal frontier cell. A
claim cannot violate that spacing or disconnect the remaining water graph.
Growth stops after exactly the requested number of land cells has been claimed.
Tiny valid configurations that cannot satisfy three-hop spacing retry with one
edge hop and two island hops.

If a round cannot place or grow every island, the complete attempt is discarded.
The next deterministic round adds three percentage points of ocean, up to 95%,
and creates a fresh point set. Production allows twenty rounds. A round is a
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
