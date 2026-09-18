# Single-mesh island generation

`Generate` builds irregular Voronoi islands from one point set shared by the
whole world. Callers provide only `WorldSeed`, `ProvinceCount`, and
`IslandCount`. The land count is exact; the island count is an initial seed
count and can decrease through mergers. Growth-assigned water provinces use
`NoIslandID`; terrain is a later classification, and interior water is valid.

## Stage order

For every successful generation:

1. Validate `ProvinceCount >= IslandCount >= 1`.
2. Derive a deterministic random stream for the first generation round.
3. Scatter sites across the configured fixed-area rectangle and apply two Lloyd relaxation passes.
4. Build one Voronoi adjacency graph for the complete world.
5. Mark a permanent ocean barrier, propagate the configured edge ramp, and add
   jittered regional attractants wherever their edge clearance can be met.
6. Plant one uniformly selected seed per initial island and apply the same
   one-hop control rule used by every later claim.
7. Draw islands randomly and select frontier cells with a desirability-weighted
   softmax until exactly the requested land count has been claimed. Merge every
   rival island directly connected by a claim.
8. Retry from a fresh deterministic mesh with more ocean if a round cannot
   finish.
9. Scale the complete mesh so total land area equals the requested province
   count, canonicalize shared corners and edges, and assign physical axes and
   final terrain.

Every public province remains finite, convex, counterclockwise, positive-area,
and center-containing. Every island is one authoritative shared-edge land
component. Islands never share land adjacency or touch the world boundary, the
world bounds have no uncovered gaps, and average land province area is one
world area unit. Water need not be one connected component.

## Determinism and regression coverage

Growth and terrain use locally constructed `math/rand/v2` PCG streams derived
from the world seed in separate domains. Identical configurations produce
deeply identical worlds for a fixed generator version; a later version may
intentionally update fixtures and output.

The exhaustive small-world matrix covers every valid province/island count
through 12 across three seeds. Fixed medium, asymmetric multi-island, and large
fixtures lock exact corner, edge, coastline, allocation, and normalized
coastline signatures. Tests also check connected islands, canonical topology
without orphans, complete bounds coverage, area scaling, terrain, determinism,
and concurrent race safety. Coastline regressions select the largest loop as an
island's outer silhouette while still validating every lake loop.

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
