# wgvc

`wgvc` generates deterministic, province-first game worlds composed of
Fibonacci-weighted islands, clipped Voronoi provinces, shared polygon topology,
and spatially correlated terrain.

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

	fmt.Printf("%d islands, %d provinces\n", len(world.Islands), len(world.Provinces))
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

Every successful call returns exactly the requested number of provinces and
islands, with at least one province per island. For a fixed version of this
generator, identical `Config` values produce deeply identical `World` values.
Generation stages use independent deterministic random streams, so consuming
randomness in terrain generation cannot perturb island placement or province
tessellation. A newer generator version may intentionally change generated
output for the same configuration.

## World topology

The returned geometry is indexed:

- Each `Island`, `Province`, `Corner`, and `Edge` has an ID equal to its index
  in the corresponding `World` slice.
- `Island.ProvinceIDs` lists the island's provinces in canonical order.
- `Province.CornerIDs` is a counterclockwise polygon ring without a repeated
  closing corner. `Province.Center` is its original Voronoi generating point,
  not its polygon centroid.
- Corners and edges are shared objects. An `Edge` references two corners and
  either one province at a coastline or two provinces at an interior boundary.

Interior edge incidence is authoritative **undirected geometric adjacency**:
two provinces are adjacent when they share a boundary. It is not a game route.
Future routes may be directed, so `A -> B` and `B -> A` will be separate
decisions rather than consequences of geometric adjacency.

Each island is an irregular blob extracted from a larger private Voronoi map.
Frame-touching and unselected cells remain private ocean; only the exact
requested connected land cells are returned. The retained coastline can form
bays and promontories while every province remains one convex polygon. A
one-province island is therefore still one convex cell, and coastline detail
increases with province count rather than adding rendering-only noise.

The deterministic [blob-island gallery](docs/blob-islands.svg) shows tiny,
medium, large, and multi-island fixtures without terrain colors. See
[Blob-island generation](docs/blob-islands.md) for the pipeline, guarantees,
resolution limits, regression fixtures, and reproduction command.

## Terrain

Terrain is generated from a seeded two-dimensional value-noise field in world
coordinates. Lattice values are in `[0, 1)` and use quintic interpolation. The
fixed wavelength is four world units, spanning roughly four typical province
widths because a province's typical area is one square world unit.

The field is sampled once per shared corner. Each province receives the mean of
its corner samples and is classified using fixed thresholds:

| Mean value | Terrain |
|---:|---|
| `< 0.4` | `plains` |
| `>= 0.4` and `< 0.6` | `hills` |
| `>= 0.6` | `mountains` |

Noise values remain internal. Terrain assignment never creates water, removes
provinces, or changes geometry, island membership, or adjacency.

## Tests

```sh
go test ./...
go test -race ./...
```
