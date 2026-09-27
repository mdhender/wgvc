# wgvc

`wgvc` generates deterministic, province-first game worlds on one relaxed
Voronoi mesh, with concurrently grown and merging islands, shared polygon
topology, and spatially correlated terrain.

The included `generate` command exposes the complete growth configuration
and writes canonical JSON world data as well as terrain-colored SVG and PNG
maps at automatically derived dimensions.

## Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/mdhender/wgvc"
)

func main() {
	world, err := wgvc.Generate(wgvc.Config{
		WorldSeed:     42,
		ProvinceCount: 100,
		IslandCount:   8,
		AspectRatio:   wgvc.AspectRatioWidescreen,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%d islands, %d total land and water provinces\n",
		len(world.Islands), len(world.Provinces))
	for _, provinceID := range world.Islands[0].ProvinceIDs {
		province := world.Provinces[provinceID]
		fmt.Printf("province %d: %s, %d corners\n",
			province.ID, province.Terrain, len(province.CornerIDs))
	}
}
```

The required input constraint is:

```text
ProvinceCount >= IslandCount >= 1
```

Every successful call returns exactly the requested number of land provinces.
`AspectRatio` accepts width:height values such as `1:1`, `4:3`, `16:9`, and
`2.39:1`. It also accepts the aliases `landscape` (`4:3`), `portrait` (`3:4`),
`widescreen` (`16:9`), and `cinematic` (`2.39:1`). The zero value is `1:1`.
Changing the ratio changes the map bounds but not their area or point budget.
`IslandCount` is the number of initial seeds; directly connected islands merge,
so the returned world may contain fewer islands. A world-level ocean mesh covers
the space between and around land and can include enclosed growth-water
provinces. Water provinces use `NoIslandID` and do not appear in
`Island.ProvinceIDs`; `IslandID` remains the authoritative land/water assignment
independently of terrain vocabulary.
For a fixed version of this generator, identical `Config` values produce deeply identical `World` values.
Generation stages use independent deterministic random streams, so consuming
randomness in elevation or climate generation cannot perturb island placement
or province tessellation. Terrain classification itself consumes no randomness.
A newer generator version may intentionally change generated output for the
same configuration.

## World topology

The returned geometry is indexed:

- Each `Island`, `Province`, `Corner`, and `Edge` has an ID equal to its index
  in the corresponding `World` slice.
- `Island.ProvinceIDs` lists the island's land provinces in canonical order.
- `Province.IslandID` identifies its island for land and is `NoIslandID` for
  water.
- `Province.CornerIDs` is a counterclockwise polygon ring without a repeated
  closing corner. `Province.Center` is its original Voronoi generating point,
  not its polygon centroid.
- `Province.EdgeIDs` lists the boundary edges in the same ring order:
  `EdgeIDs[i]` joins `CornerIDs[i]` to `CornerIDs[(i+1) % n]`.
- `Province.Exits` numbers those edges from 1 clockwise from north, each with
  its neighbor (or `NoProvinceID` on the world boundary), a bearing in degrees
  clockwise from north, and an informative 8-point compass label. See
  [About exits, bearings, and compass points](docs/explanations/bearing-not-compass.md).
- `Province.Area` and `Edge.Length` are in world units, where the mesh is
  scaled so land provinces have a mean area of exactly 1.
- Corners and edges are shared objects. An `Edge` references two corners and
  either one province at the outside of the world or two provinces inside it.
  A coastline edge joins one land province to one water province.

Interior edge incidence is authoritative **undirected geometric adjacency**:
two provinces are adjacent when they share a boundary. It is not a game route.
Future routes may be directed, so `A -> B` and `B -> A` will be separate
decisions rather than consequences of geometric adjacency.

The generator creates and Lloyd-relaxes one point set for the entire world,
then plants one seed per island and grows all islands concurrently across that
mesh using a static desirability field. A permanent ocean barrier keeps land
off the map boundary, while optional regional attractants can bias growth
toward selected parts of the interior. Directly connected islands merge, and
interior lakes remain valid. The coastline can form bays and promontories while
every province remains one convex polygon.

The deterministic [single-mesh island gallery](docs/blob-islands.svg) shows tiny,
medium, large, and multi-island fixtures without terrain colors. See
[Single-mesh island generation](docs/blob-islands.md) for the pipeline, guarantees,
resolution limits, regression fixtures, and reproduction command.

## Render a map

The `generate` command runs the desirability-field generator and writes a
terrain map as SVG by default. The demo parameters generate a cinematic PNG
with 25 initial islands, 3,500 land provinces, 75% water, and five attractors:

```sh
go run ./cmd/generate -seed 0x0123456789abcdef \
  -islands 25 -provinces 3500 -ocean 0.75 \
  -aspect cinematic -attractors 5 -format png -output world
```

### TNYC maps

TNYC uses about 10,000 land provinces on a cinematic map. Island separation is
governed almost entirely by the ocean fraction: at 45% water the 25 initial
islands grow into one continent, while 78% water with 20 initial islands keeps
nine separate landmasses.

```sh
# One continent: 25→1 islands, 18,182 provinces, 3384×1444
go run ./cmd/generate -seed 0x0123456789abcdef \
  -islands 25 -provinces 10000 -ocean 0.45 \
  -aspect cinematic -attractors 5 -format png -output tnyc-continent

# Nine landmasses: 20→9 islands, 45,455 provinces, 5322×2255
go run ./cmd/generate -seed 0x0123456789abcdef \
  -islands 20 -provinces 10000 -ocean 0.78 \
  -aspect cinematic -attractors 5 -format png -output tnyc
```

| Map | Landmasses (provinces each) | Wall time | Max resident memory |
|---|---|---:|---:|
| `tnyc-continent` | 1 (10,000) | 1.3 s | 83 MiB |
| `tnyc` | 9 (1874, 1782, 1574, 974, 776, 765, 761, 754, 740) | 3.4 s | 204 MiB |

Timings are for a built binary writing PNG only, measured with
`/usr/bin/time -l` on an Apple M4 at v0.7.5-alpha. The final landmass count is
seed-specific and sensitive to nearby settings (78% water with 25 or 30 initial
islands gives 11 or 7), so sweep `-ocean` and `-islands` when changing the seed.

Use `-format png` for `world.png`, `-format json` for `world.json`, or
`-format both` to write `world.svg` and `world.png`. Comma-separated values such
as `-format svg,json` select any combination, and `-format all` writes all three
files. The `-output` value is a path without an extension. SVG and PNG dimensions
are derived from the requested land-province count, the effective ocean
percentage, and the aspect ratio. Both image formats use the same terrain fills,
thin cell borders, and heavier coastline.
The `-aspect` flag accepts `landscape`, `portrait`, `widescreen`, and
`cinematic` as aliases for their corresponding numeric ratios.

JSON output contains the canonical indexed world topology in Cartesian world
coordinates; it does not contain pixel coordinates or rendering styles. Its
top-level shape is:

```json
{
  "schema_version": 5,
  "generation": {
    "generator": { "version": "0.7.3-alpha", "build": "9a64246" },
    "config": { "seed": "0x0123456789abcdef" },
    "result": { "province_count": 4688, "land_province_count": 1500 }
  },
  "bounds": { "minimum": { "x": 0, "y": 0 }, "maximum": { "x": 68, "y": 68 } },
  "islands": [],
  "basins": [],
  "provinces": [],
  "corners": [],
  "edges": []
}
```

The abbreviated objects above omit fields and collection entries. IDs equal
their array indexes, polygon rings refer to shared corners, each province also
lists its boundary `edge_ids` in ring order, its numbered `exits`, and its
`area`, edges refer to their
incident provinces and carry their `length`, water provinces use
`island_id: -1`, and provinces outside any basin use `basin_id: -1`. Terrain and climate
bands are descriptive strings. The seed is a hexadecimal string so all 64 bits
survive parsers whose numeric values use IEEE-754 doubles.
`generation.generator` records the generator version (without build metadata)
and, when available, the commit the binary was built from; see
[Generation contracts](docs/generation.md#json-export).

Attractants are disabled by default. Use `-attractors` with 1 through 6 to
select regions in the familiar die-face pattern, or 9 to select every region
of the 3×3 grid. Zero disables them. For example:

```sh
go run ./cmd/generate -attractors 5 -output five-attractors
```

The command exposes all growth controls, including `-ocean`, `-edge-barrier`,
`-edge-ramp`, `-attractant-ramp`, `-attractant-jitter`, `-temperature`,
`-control-penalty`, `-rounds`, and `-relaxations`. Climate calibration is
controlled by `-polar-ice` and `-peak-chill`, both expressed as percentages. Run
`go run ./cmd/generate -h` for their defaults and descriptions. `-version`
prints the generator version and exits; a built binary appends the commit it
was built from as semver build metadata.

`-summary` also prints, after the status line, the number of provinces in each
elevation band, heat band, moisture band, and terrain, followed by each
histogram's total. Every declared value is listed in declaration order, and
values no province has print `0`:

```sh
go run ./cmd/generate -summary -format json
```

`-layer` chooses the province field that colors SVG and PNG maps. JSON output
is unaffected, and cell borders, coastlines, and the background are the same
for every layer:

| Layer | Colors each province by |
|---|---|
| `terrain` | its terrain (the default) |
| `elevation` | elevation, deep ocean through shelf, then a hard step at sea level to coastal green, upland brown, and snow; the side of the step follows land/water membership |
| `relief` | relief, dark (flat) to pale (rugged), over a fixed display range of `[0, 0.4]`; higher values saturate |
| `heat` | heat, blue (cold) through neutral to red (hot) |
| `moisture` | moisture, ochre (dry) through neutral to deep green (wet) |
| `climate` | its heat band × moisture band, one color per cell of the 5×5 table |

Ramps cover a fixed range (`[-1, 1]` for elevation, `[0, 1]` for heat and
moisture) rather than stretching to the values a world happens to contain, so
two maps are comparable. Relief is the exception: it is nominally in `[0, 1)`,
but as a mean of neighbor differences it rarely passes 0.3 (the 99th percentile
of land relief is 0.23 to 0.31 across seeds and sizes), so its layer spans
`[0, 0.4]`. On that scale the terrain thresholds land at 30% (wetland cap
0.12), 35% (hills 0.14), 40% (mountains 0.16), and 50% (badlands 0.20), around
the ramp's first color break, so flat, hilly, and mountainous regions separate
visibly. Unknown layer names are rejected:

```sh
go run ./cmd/generate -layer heat -format png -output heat
```

See [Desirability-field growth](docs/x24.md) for the rules and defaults. The
public `Generate` API uses those defaults, including zero attractants; these
tuning flags do not expand its `Config`.

## Terrain

Terrain is a deterministic final classification derived from island membership,
elevation, relief, heat, moisture, and coastal adjacency. It has no independent
noise field. `Terrains` returns the complete vocabulary in stable family order:
ocean, inland water, frozen, wetland, dry, open land, forest, elevated,
volcanic, and coast. `Terrain.IsWater` recognizes both ocean and inland-water
values.

Classification considers ocean depth first, then glacial ice, elevated terrain,
wetlands, coastal land, rough dry or upland terrain, and finally a heat ×
moisture cover table. Water next to growth-assigned land is coastal water; low,
growth-assigned land next to water can be coast. `volcano` and
`volcanic-highland` are reserved for volcanism (issue #40) and are not currently
emitted.

The [terrain legend](docs/references/terrain-legend.md) shows each terrain's
map color and the exact rule that assigns it.

Water that cannot reach the world boundary through other water forms a basin:
a connected body enclosed by land. Every member of a basin shares one terrain,
`inland-sea` for basins of 10 or more provinces and `lake` otherwise, taking
precedence over coastal water. A basin's `SurfaceElevation` is the lowest
elevation of the land provinces around it, the height at which it would spill;
members keep their water elevation as depth below that surface, and `Depth` is
the deepest. Basins come only from growth-assigned water, so they never change
whether a province is land or water; shorelines are exactly the growth
coastline.

The seeded elevation field is sampled once per shared corner. Land-land edges occupy
`[0.1, 1)`, water-water edges occupy `[-1, -0.1)`, and coastlines and world
boundaries remain exactly at sea level (`0`). A province keeps the mean of its
boundary-edge elevations and an ordered elevation band: deep water, shallow
water, lowland, upland, highland, or mountain. Island membership remains the
authority for land and water, including provinces whose mean elevation is zero.

| Growth assignment | Mean edge elevation | Elevation band |
|---|---:|---|
| water | `< -0.5` | deep water |
| water | `>= -0.5` | shallow water |
| land | `< 0.2` | lowland |
| land | `>= 0.2` and `< 0.4` | upland |
| land | `>= 0.4` and `< 0.6` | highland |
| land | `>= 0.6` | mountain |

Terrain assignment never removes provinces or changes geometry, island
membership, adjacency, or the growth-assigned land/water decision.

Relief measures local roughness rather than height. A province's relief is the
mean absolute elevation difference across its shared edges with same-medium
neighbors (land with land, water with water):
`relief(p) = mean(|p.Elevation - neighbor.Elevation|)`. World-boundary and
coastline edges are ignored, so the sea-level step at the coast never counts.
Because land and water elevations each span a unit-wide range, relief is
naturally in `[0, 1)`; a province with no same-medium neighbor has relief `0`.
The pass consumes no randomness. Typical land relief is about 0.09 (median),
with the roughest tenth above about 0.17.

Terrain reads relief at four thresholds: highland with relief `>= 0.16` is
mountain rather than hills, upland with relief `>= 0.14` is hills, dry land with
relief `>= 0.20` is badlands, and humid lowland is a wetland only when its
relief is `<= 0.12`.

## Climate

Each province stores independent `Heat` and `Moisture` values in `[0,1]` plus
ordered bands. Heat bands are polar, cold, temperate, warm, and hot; moisture
bands are arid, dry, moderate, humid, and saturated. `ElevationBands`
(lowest to highest), `HeatBands`, and `MoistureBands` return the declared bands
in order. Both fields use their own
domain-separated noise stream sampled at shared corners, so climate generation
cannot perturb terrain, elevation, island growth, or geometry.

Heat blends its corner mean with a north-cold/south-warm latitude gradient
(north is +Y, the top of a rendered map), then cools land in proportion to its
positive elevation:

```
heat = ((1 - warmth) * noise + warmth * (1 - latitude)) * (1 - cooling * max(0, elevation))
```

Both factors lie in `[0, 1]`, so heat does too without clamping, and no mass of
provinces piles up at either end. Water receives no elevation cooling.
`Config.PolarIce` and `Config.PeakChill` are the calibration targets, as
fractions, with zero values selecting the defaults: the share of ocean
provinces in the polar band, and the share of high-latitude high-elevation land
that is cold or polar. The generator searches `warmth` in `[0.25, 1]` and
`cooling` in `[0, 1]` for the pair whose fractions come within half a
percentage point (or half a province) of both targets; among such pairs the
strongest gradient wins, so the targets bound the gradient rather than being
met by noise alone. If no pair reaches both targets, the lowest total error
wins. The floor on `warmth` keeps a visible gradient even when a target is
unreachable. Bands use population shares of 5%, 15%, 45%, 25%, and 10%, so a
normal world always receives every band. Only a population too small for the
shares, a world without ocean, or an empty peak slice falls back to the middle
band for every province; continuous values remain available either way.

## Tests

```sh
go test ./...
go test -race ./...
```
