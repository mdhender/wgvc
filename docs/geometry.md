# Geometry backend decision

The geometry spike selects `github.com/pzsz/voronoi` at commit
`4314be88c79fb3d64a0f2095b0f14182b4522808`, pinned by Go pseudo-version
`v0.0.0-20130609164533-4314be88c79f`. The package implements Fortune's
algorithm, clips its output to an axis-aligned rectangle, closes cells along
that rectangle, and exposes each edge's left and right cells. It is licensed
under the MIT License, which is compatible with this repository's MIT
license.

The retained harness in `geometry_backend_test.go` proves the backend behavior
needed by a later production wrapper. It covers horizontal and vertical
two-site cases, horizontal and vertical collinear sites, a near-collinear
case, four cocircular sites, an equal-axis grid, repeatability, and several
irregular separated point sets. It verifies finite closed counterclockwise
rings, one cell per distinct site, complete unit-square area, nondegenerate
edges, and consistent edge-to-cell incidence.

## Wrapper contract

The production wrapper should keep backend types private and apply the same
rules demonstrated by the harness:

- Normalize each island's clipping rectangle and finite, distinct sites to the
  open unit square before invoking the backend. Sites on the clipping boundary
  are outside the supported contract.
- Copy sites into a fresh backend slice. `ComputeDiagram` sorts its input slice
  in place, so passing caller-owned storage would mutate it.
- Retain an exact coordinate-to-caller-index map before the call. The backend
  copies site coordinates unchanged into each cell but returns cells in its
  internal processing order. The map restores canonical caller order.
- Reject duplicate sites before the call and reject output unless its cell
  count equals the distinct input count. The backend otherwise silently drops
  exact duplicate coordinates.
- Special-case one site as the entire clipping square. The backend returns the
  site but does not create its four boundary edges.
- Read each polygon from its ordered halfedges, verify closure, and reverse the
  ring when necessary to produce the package's counterclockwise convention.
- Build adjacency only from nondegenerate backend edges having both `LeftCell`
  and `RightCell`. Border edges have one incident cell. A shared vertex alone
  is therefore not adjacency; the cocircular fixture proves that the two
  diagonal pairs are not neighbors.

## Numerical policy

Geometry is evaluated only after normalization to `[0,1]²`. Comparisons use an
absolute tolerance of `1e-9`, matching the backend's clipping and cell-closing
epsilon at that scale. Coordinates are never compared exactly except when
mapping unchanged input sites and rejecting exact duplicates. Output is
invalid if it contains a non-finite coordinate, an edge of length at most
`1e-9`, an open ring, a non-positive-area cell, a point outside the clipping
square by more than the tolerance, or total cell area differing from one by
more than the tolerance.

The site planner remains responsible for producing separated sites. This
decision does not define a policy for nearly coincident sites.

## Limitations and rationale

The selected repository has no tagged release and has not been maintained
since 2013. Upstream reports include a nil-pointer panic for a regular grid
whose sites include the clipping boundary. Reproducing that fixture confirmed
the failure; requiring sites in the open unit square avoids that unsupported
case, and centered equal-axis grids pass the retained harness. The wrapper
must continue treating malformed output as an error rather than repairing it.

Despite those limitations, this backend is preferred for the bounded alpha
contract because it directly supplies rectangular closure and authoritative
shared-edge incidence. The evaluated fixtures pass without a fork, and no
second candidate was needed. The dependency should be replaced rather than
forked if later production inputs violate this documented contract.
