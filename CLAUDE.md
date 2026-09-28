# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository rules (from AGENTS.md)

- Assign every new GitHub issue and PR to `mdhender`.
- Alpha workflow: once a request is complete and all relevant tests pass, commit directly to `main` and push without asking. Reference or close the relevant issue in the commit message (e.g. `Closes #30`) when appropriate.
- Bump the patch version in `version.go` in every commit (keep the pre-release label unless told otherwise); after pushing, tag the commit `v<version>` (e.g. `v0.7.1-alpha`) and push the tag.
- Never use `math/rand`; use `math/rand/v2`. Code that uses randomness must accept a seed and build a local source — never draw from a global source.

## Commands

```sh
go test ./...                       # full suite
go test -race ./...
GOARCH=amd64 go test ./...        # cross-architecture check; runs under Rosetta on Apple silicon
go test -run TestName ./            # single test in the root package
go test -run TestName ./cmd/generate

# Render a map (see README for all flags; `-h` lists defaults)
go run ./cmd/generate -seed 0x0123456789abcdef \
  -islands 25 -provinces 3500 -ocean 0.75 \
  -aspect cinematic -attractors 5 -format png -output world
```

Doc SVG galleries are golden outputs produced by tests; regenerate them with an env var:

```sh
WGVC_UPDATE_SINGLE_MESH_GALLERY=1 go test -run TestSingleMeshIslandGallery   # docs/blob-islands.svg
WGVC_UPDATE_TILE_EXPERIMENT=1    go test -run TestCenteredIslandTileGallery  # docs/tile-experiment.svg
WGVC_UPDATE_BLOB_SVG=1           go test -run TestCandidateBlobContactSheet  # docs/blob-selection.svg
WGVC_UPDATE_TERRAIN_LEGEND=1     go test -run TestTerrainLegend ./cmd/generate  # docs/references/terrain-legend.png
```

## Architecture

`wgvc` is a library (root package `wgvc`) that generates deterministic province-based worlds; `cmd/generate` is the CLI that renders them to SVG/PNG/JSON.

### Pipeline (`generate.go`)

`Generate(Config)` → `generateForRender`, which runs these stages in order:

1. **Growth** — `internal/x24.Generate`: builds one Lloyd-relaxed Voronoi point set for the whole world (`internal/singlemesh`, wrapping `internal/voronoi`, a copy of `pzsz/voronoi` with fusion-safe arithmetic), then grows all islands concurrently over it with a desirability field (edge barrier/ramp, optional attractants, softmax choice, rival ramp by hop distance to other islands' land). Touching islands merge. On failure a round is discarded and retried with +3% ocean on a fresh derived stream. Algorithm and defaults: `docs/x24.md`.
2. **Canonical tessellation** — `tessellateRectangle` (`geometry.go`) re-tessellates the x24 sites into the canonical `islandMesh` with shared, world-global corner and edge IDs and CCW rings.
3. **Scaling** — the mesh is uniformly scaled so total land area equals the land province count (a typical land province has area ≈ 1).
4. **Fields** — `assignGeometry` (edge lengths, ring-ordered `Province.EdgeIDs`, areas), `assignExits` (exits numbered clockwise from north with bearings), `assignElevations` (per-corner noise → corner/edge → province mean/band), `assignRelief` (mean absolute elevation difference to same-medium edge neighbors, in `[0,1)`), `assignClimate` (heat/moisture + calibrated bands), `assignBasins` (water components with no path to the world boundary become lake/inland-sea basins), `assignRivers` (priority-flood drainage over the corner graph; river chains along land-land edges), `assignCoastDistances`, `assignSeaZones` (farthest-point seeds, graph-Voronoi zones over ocean), `assignStraits`, `assignNecks` (small vertex cuts of island land graphs), `assignHarbors` (shelter, water edge counts, river lists), then `assignTerrain` (deterministic classification, consumes no randomness) and `assignFeatures` (named terrain regions and archipelagos, read from terrain). Only elevation and climate consume randomness. Terrain never changes geometry, topology, or land/water membership; only feature identification runs after it.

`Generate` exposes only a small `Config` and uses `x24.DefaultConfig()` for everything else. The CLI instead builds a full `x24.Config` from flags and calls `GenerateForRenderWithClimate`, which is why tuning flags do not appear in the public `Config`.

### Invariants worth preserving

- Every `Island`/`Province`/`Corner`/`Edge` ID equals its index in its `World` slice; membership lists use canonical order.
- `Province.IslandID` (or `NoIslandID` for water) is the authority for land vs water — not terrain or elevation. Basins (`Province.BasinID`) are drawn only from growth water and never change membership.
- Edge incidence is undirected geometric adjacency, not a game route.
- Identical `Config` → deeply identical `World`. Each stage draws from its own domain-separated PCG stream derived via SplitMix64 (`random.go`; x24 has its own in `internal/x24/random.go`). Add a new domain constant for any new random stage rather than reusing an existing stream, so existing outputs are not perturbed.
- Worlds are bit-identical across CPU architectures (`TestGenerateMatchesRecordedHash` guards this with recorded hashes). Go fuses `x*y + z` into one rounding on arm64 but not amd64, so wrap every product that feeds an addition or subtraction in an explicit `float64(...)` conversion, and use `internal/fmath` instead of `math.Hypot`, `math.Atan2`, or `math.Exp`, whose standard-library versions differ by architecture. After an intentional generator change, rerun the hash test with `WGVC_PRINT_WORLD_HASH=1` on both architectures and record the shared value.
- `docs/generation.md` documents the public contracts (coordinates, topology, JSON schema); keep it and README in sync with behavior changes.

### Legacy code

`allocation.go`, `placement.go`, `blob_selection.go`, `seeds.go`, `retained_mesh.go`, and the multi-mesh parts of `geometry.go` belong to an earlier per-island tile pipeline. They are reached only from tests (and the doc galleries above), not from `Generate`, apart from shared helpers such as `transformMesh`/`uniformTransform` and the tessellation code.

### CLI (`cmd/generate`)

`main.go` parses flags into `x24.Config` + `ClimateConfig`; `render.go` draws SVG/PNG (via `fogleman/gg`) with image dimensions derived from land count, effective ocean fraction, and aspect ratio; `json.go` writes the schema-versioned world export in Cartesian world coordinates (no pixel coordinates). Aspect-ratio parsing and aliases live in `internal/aspectratio`.
