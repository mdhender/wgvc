# wgvc

`wgvc` generates deterministic, province-first game worlds on one relaxed
Voronoi mesh, with concurrently grown and merging islands, shared polygon
topology, and spatially correlated terrain.

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
`IslandCount` is the number of initial seeds; directly connected islands merge,
so the returned world may contain fewer islands. A world-level ocean mesh covers
the space between and around land and can include interior lakes. Water provinces
have `TerrainWater`, use `NoIslandID`, and do not appear in `Island.ProvinceIDs`.
For a fixed version of this generator, identical `Config` values produce deeply identical `World` values.
Generation stages use independent deterministic random streams, so consuming
randomness in terrain generation cannot perturb island placement or province
tessellation. A newer generator version may intentionally change generated
output for the same configuration.

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

The `wgvc-render` command generates a world and writes an SVG map by default:

```sh
go run ./cmd/wgvc-render -seed 42 -provinces 137 -islands 11 -output world
```

Use `-format png` for `world.png`, or `-format both` to write both
`world.svg` and `world.png`. The `-output` value is a path without an
extension. `-width` and `-height` set both SVG and PNG dimensions in pixels.
Both formats are rendered from the same scene and include terrain fills, thin
cell borders, and a heavier coastline.

The production issue #24-#26 algorithm has a calibration command for its
desirability field, permanent ocean barrier, and optional regional attractants:

```sh
go run ./cmd/x24
```

Attractants are disabled by default. Use `-attractors` with 1 through 6 to
select regions in the familiar die-face pattern, or 9 to select every region
of the 3×3 grid. Zero disables them. For example:

```sh
go run ./cmd/x24 -attractors 5 -output five-attractors.svg
```

The calibration command also accepts `-attractant-ramp` and
`-attractant-jitter` to control the influence and placement of those targets.
Run `go run ./cmd/x24 -h` for the complete set of growth controls.

See [Desirability-field growth](docs/x24.md) for its rules, defaults,
and calibration controls. The public `Generate` API uses those defaults,
including zero attractants; these tuning flags do not expand its `Config`.

## Terrain

Terrain is generated from a seeded two-dimensional value-noise field in world
coordinates. Lattice values are in `[0, 1)` and use quintic interpolation. The
fixed wavelength is four world units, spanning roughly four typical province
widths because a province's typical area is one square world unit.

The field is sampled once per shared corner. Each province receives the mean of
its corner samples and is classified using fixed thresholds:

| Mean value | Terrain |
|---:|---|
| not sampled | `water` |
| `< 0.4` | `plains` |
| `>= 0.4` and `< 0.6` | `hills` |
| `>= 0.6` | `mountains` |

Noise values remain internal. Terrain assignment preserves water and never
removes provinces or changes geometry, island membership, or adjacency.

## Tests

```sh
go test ./...
go test -race ./...
```
