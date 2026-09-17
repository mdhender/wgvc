# Single-mesh island generation

`Generate` builds irregular Voronoi islands from one point set shared by the
whole world. Callers provide only `WorldSeed`, `ProvinceCount`, and
`IslandCount`, and receive exactly those land and island counts plus one
continuous ocean. Water provinces use `TerrainWater` and `NoIslandID`.

## Stage order

For every successful generation:

1. Validate `ProvinceCount >= IslandCount >= 1`.
2. Derive a deterministic random stream for the first generation round.
3. Scatter sites across the unit square and apply two Lloyd relaxation passes.
4. Build one Voronoi adjacency graph for the complete world.
5. Plant one legal seed cell per island at least three graph hops from the map
   boundary and other islands, with reduced spacing only when a tiny valid
   configuration cannot satisfy the preferred distances.
6. Draw islands and legal frontier cells randomly, growing all islands until
   exactly the requested land count has been claimed. Reject claims that would
   join different islands or disconnect the remaining ocean.
7. Retry from a fresh deterministic mesh with more ocean if a round cannot
   finish.
8. Scale the complete mesh so total land area equals the requested province
   count, canonicalize shared corners and edges, and assign land terrain.

Every public province remains finite, convex, counterclockwise, positive-area,
and center-containing. Every island is one authoritative shared-edge land
component. Islands never share land adjacency or touch the world boundary, the
ocean is connected, the world bounds have no uncovered gaps, and average land
province area is one world area unit.

## Determinism and regression coverage

Growth and terrain use locally constructed `math/rand/v2` PCG streams derived
from the world seed in separate domains. Identical configurations produce
deeply identical worlds for a fixed generator version; a later version may
intentionally update fixtures and output.

The exhaustive small-world matrix covers every valid province/island count
through 12 across three seeds. Fixed medium, asymmetric multi-island, and large
fixtures lock exact corner, edge, coastline, allocation, and normalized
coastline signatures. Tests also check connected land and water, canonical
topology without orphans, complete bounds coverage, area scaling, terrain,
determinism, and concurrent race safety.

![Deterministic tiny, medium, large, and multi-island fixtures](blob-islands.svg)

Regenerate the gallery with:

```sh
WGVC_UPDATE_SINGLE_MESH_GALLERY=1 go test -run TestSingleMeshIslandGallery
```

Run all structural and race checks with:

```sh
go test ./...
go test -race ./...
```
