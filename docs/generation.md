# Generation contracts

`Generate` accepts a world seed, province count, island count, and an aspect
ratio expressed as a named alias or width:height value. Counts must
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
- `Province.EdgeIDs` lists the boundary edges in ring order: `EdgeIDs[i]` is
  the edge from `CornerIDs[i]` to `CornerIDs[(i+1) % n]`. The ring starts at
  the province's lowest corner ID, so the numbering is deterministic.
- `Province.Area` is the polygon area and `Edge.Length` is the distance between
  the edge's corners, both in world units. The mesh is scaled so the land
  provinces have a mean area of exactly 1; coarse ocean cells near the world
  boundary can exceed 1. These fields are derived from the final geometry and
  consume no randomness.
- Land provinces identify their island. Water provinces use `NoIslandID`.
- Water provinces with no water path to the world boundary belong to a basin
  (`World.Basins`); every other province uses `NoBasinID`. Basins never change
  whether a province is land or water.
- An interior edge's incident provinces are the authoritative undirected
  geometric adjacency. An edge is never a game route.

## JSON export

`go run ./cmd/generate -format json -output world` writes `world.json`. The
document has `schema_version: 4`, generator identity, generation configuration
and effective result metadata, world-coordinate bounds, and the canonical `islands`,
`basins`, `provinces`, `corners`, and `edges` collections. It uses the same IDs
and references described above. Water provinces retain `island_id: -1`, and
provinces outside any basin have `basin_id: -1`; terrain and elevation, heat, and
moisture bands use descriptive strings. The seed is a hexadecimal string to
preserve the complete unsigned 64-bit value.

`generation.generator.version` is the semantic version of the generator without
build metadata (for example `0.7.3-alpha`), so exports from different builds of
one release compare equal. `generation.generator.build` is the VCS commit the
binary was built from, suffixed `-dirty` for an uncommitted working tree, and is
omitted when the build recorded no VCS information (as under `go run`). Schema
version 2 added the `generator` object; schema version 1 documents lack it.
Schema version 3 added the `basins` collection (`id`, `province_ids`,
`surface_elevation`, `depth`) and each province's `basin_id`. Schema version 4
added each province's `edge_ids` (ring order, parallel to `corner_ids`) and
`area`, and each edge's `length`, all in world units.

JSON uses Cartesian generation coordinates rather than the transformed pixel
coordinates used by image renderers. Use a comma-separated format such as
`-format svg,json` to export world data with an image, or `-format all` for SVG,
PNG, and JSON. The existing `-format both` remains an SVG-and-PNG alias.

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

Across CPU architectures, island membership, topology, every ID, and every band
and terrain classification are identical. Continuous values (coordinates,
elevation, heat, moisture) can differ in their lowest-order bits, because Go
permits fused multiply-add on arm64 but not amd64. Mesh corners that the Voronoi
backend clips to the world rectangle are snapped exactly onto its sides, so this
rounding cannot reorder the canonical corner IDs.

See [Single-mesh island generation](blob-islands.md) for visual fixtures and
regression coverage.
