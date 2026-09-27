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
- `Province.Exits` numbers those same edges from 1, clockwise from north
  (reverse ring order). Exit 1 is the edge whose outward bearing is smallest,
  the first exit clockwise from due north. Each exit records its `EdgeID`, the
  `NeighborID` across the edge (`NoProvinceID` on the world boundary), the
  `Bearing` of the edge's outward normal in degrees clockwise from north in
  `[0, 360)`, and an 8-point `Compass` label rounded from the bearing. The
  bearing is the direction contract; compass labels are informative and two
  exits can share one. Bearings increase strictly with exit number. The
  numbering is geometric and never expresses passability. The world does not
  wrap; boundary exits lead nowhere. See
  [About exits, bearings, and compass points](explanations/bearing-not-compass.md).
- `Corner.Elevation` is the seeded elevation noise at the corner on the edge
  scale: `[0.1, 1)` where every incident province is land, `(-1, -0.1]` where
  every one is water, and exactly `0` on a coastline. An edge's elevation is
  the mean of its corners'.
- `World.Rivers` are chains of edges between land provinces, listed from
  source to mouth: `CornerIDs` in flow order and `EdgeIDs[i]` joining
  `CornerIDs[i]` to `CornerIDs[i+1]`. Rivers never run along or across water.
  Each edge names its river in `Edge.RiverID` (`NoRiverID` otherwise) and
  carries `Edge.Discharge`, the flow along it in moisture-weighted
  province-area units; discharge is set on every edge drainage uses, river or
  not. A river's `Discharge` is its last edge's, and its `Class` follows from
  it: stream, river, or major river, of which the last two are navigable.
  `Source.Kind` is `spring` or `basin` (the outflow of a lake or inland sea,
  with `BasinID`); `Mouth.Kind` is `ocean`, `basin` (with `BasinID`), or
  `river` (a confluence, with the `RiverID` joined). At a confluence the
  larger branch continues and the smaller ends, so every river edge belongs
  to exactly one river. Rivers are ordered by descending discharge.
- Flow direction is the river's corner order. Consecutive river edges pivot
  around a shared corner, so the bank provinces taken in flow order are
  pairwise adjacent; how boats use that is a game rule outside this library.
- Drainage is derived from elevation, moisture, and IDs alone and consumes no
  randomness. Every corner on ocean is an outlet; a priority-flood from the
  outlets gives every other corner one downstream neighbor along a monotone
  path to the sea, filling depressions to their spill level internally. Lakes
  and inland seas are ordinary nodes, so rivers flowing in end at the shore
  and the basin's water leaves at its spill corner. Runoff is each land
  province's `Area * Moisture` spread over its corners.
- `Province.CoastDistance` is the number of hops through the province's own
  medium (land through land, water through water) to the nearest edge shared
  with the other medium, so coastal land and coastal water are `0`. A lake
  shore counts as coast for the land beside it and for the lake; the world
  boundary does not.
- `World.SeaZones` partition the ocean provinces, water outside any basin,
  into contiguous regions of about 200 provinces (never more than 400) for
  naming seas. Seeds come from farthest-point sampling over water adjacency
  and each province joins its nearest seed, ties to the lower seed, so zones
  never cross land. `Province.SeaZoneID` is `NoSeaZoneID` for land and basin
  water. Zones are ordered by lowest member and record their seed as
  `CenterProvinceID`.
- `World.Straits` are narrow water crossings between two islands: the water
  provinces on a path of at most 3 water provinces joining them, grouped by
  water adjacency, with the two `IslandIDs`, `Width` as the shortest such path,
  and `Shores`, each island's land provinces adjacent to the strait. A water
  province can lie in more than one strait. Straits are ordered by island pair
  and then lowest province. Whether any appear depends on how closely the
  growth stage lets islands approach one another.
- `World.Necks` are narrow isthmuses: minimal cuts of at most 3 adjacent land
  provinces whose removal splits their island into regions of which the two
  largest each have at least 10 provinces. Cuts that share or touch a province
  merge into one neck. `Ends` lists, largest region first, the land provinces
  of each region adjacent to the neck, and `EndSizes` the regions' sizes with
  the whole neck removed. Necks are ordered by island and then lowest
  province.
- Coast distance, sea zones, straits, and necks are derived from topology and
  IDs alone and consume no randomness.
- `World.Features` are geographic units a consumer can name: connected
  regions of land whose terrains share one family, of at least 5 provinces,
  each on one island (`mountain-range`: mountain, alpine, volcano,
  volcanic-highland; `hill-country`: hills, badlands; `plateau`; `forest`:
  boreal, temperate, rainforest; `desert`: desert, scrubland; `wetland`:
  marsh, swamp, bog; `ice-field`: glacial ice), and `archipelago` groups of
  two or more islands joined by water paths of at most 6 water provinces,
  whose provinces are all their land. Features are ordered by kind, in
  `FeatureKinds` order, then lowest province ID. Seas, straits, and rivers
  are their own collections. Feature identification runs after terrain and
  consumes no randomness.
- Siting inputs on land provinces, zero on water: `OceanEdges` and
  `BasinEdges` count edges onto ocean and onto lakes or inland seas;
  `Shelter` in `[0, 1]` is the mean, over the water provinces it touches, of
  the fraction of that water province's neighbors that are land, so a bay
  scores high and a promontory low; `RiverIDs` are the rivers along its
  edges, `RiverMouthIDs` those ending on ocean or a basin at one of its
  corners, and `ConfluenceIDs` the tributaries joining another river at one
  of its corners. Both bank provinces of a river's last edge list its end.
  These are measures, not placements; where settlements go is a game rule.
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
document has `schema_version: 9`, generator identity, generation configuration
and effective result metadata, world-coordinate bounds, and the canonical `islands`,
`basins`, `provinces`, `corners`, `edges`, `rivers`, `sea_zones`, `straits`,
`necks`, and `features` collections. It uses the same IDs
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
`area`, and each edge's `length`, all in world units. Schema version 5 added
each province's `exits` (`number`, `edge_id`, `neighbor_id`, `bearing`,
`compass`), ordered by `number`. Schema version 6 added each corner's
`elevation`, each edge's `river_id` and `discharge`, and the `rivers`
collection (`id`, `class`, `discharge`, `corner_ids`, `edge_ids`, and `source`
and `mouth` objects with `kind`, `basin_id`, and `river_id`). Schema version 7
added each province's `coast_distance` and `sea_zone_id`, and the `sea_zones`
(`id`, `center_province_id`, `province_ids`), `straits` (`id`, `island_ids`,
`width`, `province_ids`, `shores`), and `necks` (`id`, `island_id`, `width`,
`province_ids`, `ends`, `end_sizes`) collections. Schema version 8 added each
province's `ocean_edges`, `basin_edges`, `shelter`, `river_ids`,
`river_mouth_ids`, and `confluence_ids`. Schema version 9 added the `plateau`
terrain and the `features` collection (`id`, `kind`, `island_ids`,
`province_ids`).

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
