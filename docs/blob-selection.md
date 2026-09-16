# Retained-candidate blob feasibility decision

Issue #9 is a **go** for the retained-candidate architecture proposed by #8.
The private prototype in `blob_selection.go` does not change `Generate`; it
demonstrates bounded candidate construction and exact selection against the
pinned rectangular Voronoi backend. The retained contact sheet is
[`blob-selection.svg`](blob-selection.svg).

## Construction and capacity

For a request of `n >= 1` land cells, the prototype chooses the smallest
interior side `s` satisfying `s² >= 2n`. It places a centered normalized site
in each cell of an `(s+2) × (s+2)` regular grid. Thus there are at least `2n`
eligible interior candidates plus a one-cell ring reserved for exterior water.
The candidate count is fixed before tessellation and is `O(n)` (specifically
`(ceil(sqrt(2n))+2)²`), with no rejection sampling or retry loop.

The complete candidate tessellation is canonicalized and validated before
selection, including unit-square coverage. Frame cells are identified from
one-sided backend edges, not inferred only from site positions. The prototype
also verifies that each horizontal and vertical grid neighbor has a
positive-length shared backend edge; an unsupported backend result fails with
context instead of reducing the quota or repairing geometry.

## Selection and termination

The deterministic root is the interior site nearest the unit-square center,
with candidate index resolving equal distances. Selected cells in each grid
row are maintained as one interval. A legal next step either extends an
occupied interval by one cell or starts the adjacent row at a column that
overlaps an occupied interval. Legal candidates are ranked by a bounded
low-frequency radial score:

`radius × (1 + 0.20 sin(3 angle + phase₃) + 0.10 sin(5 angle + phase₅))`

The phases come from a seed-derived local PCG stream. Candidate index breaks
score ties.

This construction always progresses until every interior cell is selected:
if an occupied row is not full, one of its interval ends can extend; otherwise,
if any interior row remains empty, a row adjacent to the contiguous occupied
row range can start in an overlapping column. Every addition therefore shares
a positive-length edge with land. Row convexity also prevents lakes: every
omitted interior cell can move horizontally through omitted cells to the
one-cell frame ring. These invariants establish termination and exterior-water
connectivity independently of the fixed-seed corpus.

The production selection boundary accepts the canonical complete candidate
mesh, its original row-major candidate plan, the requested land quota, and the
already-derived shape parameters. It validates edge incidence and grid
connectivity without mutating those inputs. Only a two-cell, positive-length
canonical mesh edge creates adjacency; sharing a corner does not. Every cell
incident to a one-cell clipping-frame edge is ineligible.

Root choice minimizes distance from the original candidate site to `(0.5,
0.5)`, breaking equal-distance ties by original candidate index. Frontier
ranking uses the score above, again breaking ties by original candidate index.
Growth order is private: the stage returns exactly the requested IDs sorted in
ascending original-candidate order for deterministic downstream compaction.

## Evidence and limits

The corpus covers counts `1`, `2`, `3`, prime and non-square counts, tens of
cells, and allocations of `128` and `256` cells across four fixed seeds. Tests
check exact distinct count, shared-edge land connectivity, no selected frame
cell, exterior reachability of every omitted cell, deterministic order,
explicit tie-breaking, complete mesh validation, and the bounded capacity
formula through 10,000 requested cells.

The contact sheet shows the frame, all candidate cells, retained land, and
omitted water for representative fixtures. Reproduce it with:

```sh
WGVC_UPDATE_BLOB_SVG=1 go test -run TestCandidateBlobContactSheet
```

This is a feasibility shape, not final visual tuning. Regular candidate sites
make the conservative row-convex proof direct and produce visibly non-square
silhouettes at useful sizes, but later #8 stages may perturb sites while
retaining and validating the required grid-neighbor edges. Production work
must still extract and compact retained topology, scale land area, place
islands, and integrate the result without retessellating selected sites.
