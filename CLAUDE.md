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
```

## Architecture

`wgvc` is a library (root package `wgvc`) that generates deterministic province-based worlds; `cmd/generate` is the CLI that renders them to SVG/PNG/JSON.

### Pipeline (`generate.go`)

`Generate(Config)` → `generateForRender`, which runs these stages in order:

1. **Growth** — `internal/x24.Generate`: builds one Lloyd-relaxed Voronoi point set for the whole world (`internal/singlemesh`, wrapping `pzsz/voronoi`), then grows all islands concurrently over it with a desirability field (edge barrier/ramp, optional attractants, softmax choice, control penalty). Touching islands merge. On failure a round is discarded and retried with +3% ocean on a fresh derived stream. Algorithm and defaults: `docs/x24.md`.
2. **Canonical tessellation** — `tessellateRectangle` (`geometry.go`) re-tessellates the x24 sites into the canonical `islandMesh` with shared, world-global corner and edge IDs and CCW rings.
3. **Scaling** — the mesh is uniformly scaled so total land area equals the land province count (a typical land province has area ≈ 1).
4. **Fields** — `assignElevations` (per-corner noise → edge → province mean/band), `assignRelief` (mean absolute elevation difference to same-medium edge neighbors, in `[0,1)`), `assignClimate` (heat/moisture + calibrated bands), then `assignTerrain` (deterministic classification, consumes no randomness). Terrain is always assigned last and never changes geometry, topology, or land/water membership.

`Generate` exposes only a small `Config` and uses `x24.DefaultConfig()` for everything else. The CLI instead builds a full `x24.Config` from flags and calls `GenerateForRenderWithClimate`, which is why tuning flags do not appear in the public `Config`.

### Invariants worth preserving

- Every `Island`/`Province`/`Corner`/`Edge` ID equals its index in its `World` slice; membership lists use canonical order.
- `Province.IslandID` (or `NoIslandID` for water) is the authority for land vs water — not terrain or elevation.
- Edge incidence is undirected geometric adjacency, not a game route.
- Identical `Config` → deeply identical `World`. Each stage draws from its own domain-separated PCG stream derived via SplitMix64 (`random.go`; x24 has its own in `internal/x24/random.go`). Add a new domain constant for any new random stage rather than reusing an existing stream, so existing outputs are not perturbed.
- `docs/generation.md` documents the public contracts (coordinates, topology, JSON schema); keep it and README in sync with behavior changes.

### Legacy code

`allocation.go`, `placement.go`, `blob_selection.go`, `seeds.go`, `retained_mesh.go`, and the multi-mesh parts of `geometry.go` belong to an earlier per-island tile pipeline. They are reached only from tests (and the doc galleries above), not from `Generate`, apart from shared helpers such as `transformMesh`/`uniformTransform` and the tessellation code.

### CLI (`cmd/generate`)

`main.go` parses flags into `x24.Config` + `ClimateConfig`; `render.go` draws SVG/PNG (via `fogleman/gg`) with image dimensions derived from land count, effective ocean fraction, and aspect ratio; `json.go` writes the schema-versioned world export in Cartesian world coordinates (no pixel coordinates). Aspect-ratio parsing and aliases live in `internal/aspectratio`.
