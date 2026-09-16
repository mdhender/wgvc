package wgvc

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCandidateBlobCorpus(t *testing.T) {
	counts := []int{1, 2, 3, 7, 17, 53, 128, 256}
	seeds := []uint64{0, 1, 42, 0xdeadbeef}
	for _, count := range counts {
		for _, seed := range seeds {
			t.Run(fmt.Sprintf("n%d/s%d", count, seed), func(t *testing.T) {
				blob, err := selectCandidateBlob(count, seed)
				if err != nil {
					t.Fatalf("selectCandidateBlob() error = %v", err)
				}
				assertValidCandidateBlob(t, blob, count)
			})
		}
	}
}

func TestSelectCandidateCellsCorpus(t *testing.T) {
	counts := []int{1, 2, 3, 7, 17, 53, 128, 256}
	seeds := []uint64{0, 1, 42, 0xdeadbeef}
	for _, count := range counts {
		for _, seed := range seeds {
			t.Run(fmt.Sprintf("n%d/s%d", count, seed), func(t *testing.T) {
				candidates, mesh, shape := candidateSelectionFixture(t, count, seed)
				originalCandidates := cloneCandidateSitePlan(candidates)
				originalMesh := cloneIslandMesh(mesh)

				selected, err := selectCandidateCells(count, candidates, mesh, shape)
				if err != nil {
					t.Fatalf("selectCandidateCells() error = %v", err)
				}
				if !reflect.DeepEqual(candidates, originalCandidates) {
					t.Fatal("selection mutated candidate inputs")
				}
				if !reflect.DeepEqual(mesh, originalMesh) {
					t.Fatal("selection mutated candidate mesh")
				}
				if len(selected) != count {
					t.Fatalf("selected %d candidates, want %d", len(selected), count)
				}

				land := make([]bool, len(mesh.cells))
				for selectionIndex, candidateID := range selected {
					if selectionIndex > 0 && selected[selectionIndex-1] >= candidateID {
						t.Fatalf("selected IDs are not in canonical original-candidate order: %v", selected)
					}
					land[candidateID] = true
				}
				neighbors, frame, err := candidateMeshTopology(mesh)
				if err != nil {
					t.Fatalf("candidateMeshTopology() error = %v", err)
				}
				for _, candidateID := range selected {
					if frame[candidateID] {
						t.Errorf("selected candidate %d touches the clipping frame", candidateID)
					}
				}
				assertSelectionConnected(t, land, neighbors, count)
				assertWaterReachesFrame(t, land, frame, neighbors)

				repeated, err := selectCandidateCells(count, candidates, mesh, shape)
				if err != nil {
					t.Fatalf("repeated selectCandidateCells() error = %v", err)
				}
				if !reflect.DeepEqual(repeated, selected) {
					t.Fatalf("repeated selection = %v, want %v", repeated, selected)
				}
			})
		}
	}
}

func TestSelectCandidateCellsUsesCentralRootAndCandidateTieBreak(t *testing.T) {
	sites := []Point{
		{X: 0.25, Y: 0.25},
		{X: 0.75, Y: 0.25},
		{X: 0.25, Y: 0.75},
		{X: 0.75, Y: 0.75},
	}
	frame := make([]bool, len(sites))
	if got, want := nearestInteriorCandidate(sites, frame), 0; got != want {
		t.Fatalf("equal-distance root = %d, want candidate-index tie winner %d", got, want)
	}

	frontier := []int{9, 4, 7, 2}
	sortBlobCandidates(frontier, make([]float64, 10))
	if want := []int{2, 4, 7, 9}; !reflect.DeepEqual(frontier, want) {
		t.Fatalf("equal-score frontier = %v, want %v", frontier, want)
	}
}

func TestCandidateMeshTopologyUsesOnlyPositiveLengthSharedEdges(t *testing.T) {
	mesh := islandMesh{
		cells:   make([]meshCell, 2),
		corners: []Point{{X: 0, Y: 0}, {X: 2 * geometryTolerance, Y: 0}, {X: 0, Y: 1}},
		edges: []meshEdge{
			{cornerIDs: [2]int{0, 1}, siteIndexes: []int{0}},
			{cornerIDs: [2]int{0, 2}, siteIndexes: []int{1}},
		},
	}
	neighbors, _, err := candidateMeshTopology(mesh)
	if err != nil {
		t.Fatalf("corner-contact topology error = %v", err)
	}
	if len(neighbors[0]) != 0 || len(neighbors[1]) != 0 {
		t.Fatalf("corner-only contact created adjacency: %v", neighbors)
	}

	mesh.edges = append(mesh.edges, meshEdge{
		cornerIDs:   [2]int{0, 1},
		siteIndexes: []int{0, 1},
	})
	neighbors, _, err = candidateMeshTopology(mesh)
	if err != nil {
		t.Fatalf("positive-length narrow connection error = %v", err)
	}
	if want := [][]int{{1}, {0}}; !reflect.DeepEqual(neighbors, want) {
		t.Fatalf("narrow shared-edge neighbors = %v, want %v", neighbors, want)
	}
}

func TestBlobGrowthCandidatesExcludeGapThatCouldBecomeLake(t *testing.T) {
	const columns, rows = 5, 5
	intervals := make([]occupiedInterval, rows)
	intervals[1] = occupiedInterval{left: 1, right: 3, set: true}
	intervals[2] = occupiedInterval{left: 1, right: 1, set: true}
	intervals[3] = occupiedInterval{left: 1, right: 3, set: true}
	land := make([]bool, columns*rows)
	for _, candidateID := range []int{6, 7, 8, 11, 16, 17, 18} {
		land[candidateID] = true
	}

	frontier := blobGrowthCandidates(columns, rows, intervals, land)
	if containsInt(frontier, 13) {
		t.Fatalf("frontier %v includes candidate 13, which skips the row gap at candidate 12", frontier)
	}
	if !containsInt(frontier, 12) {
		t.Fatalf("frontier %v omits interval-extending candidate 12", frontier)
	}
}

func TestSelectCandidateCellsRejectsMalformedMeshAndInsufficientCapacity(t *testing.T) {
	candidates, mesh, shape := candidateSelectionFixture(t, 7, 42)

	t.Run("malformed incidence", func(t *testing.T) {
		broken := cloneIslandMesh(mesh)
		broken.edges[0].siteIndexes = nil
		if _, err := selectCandidateCells(7, candidates, broken, shape); err == nil || !strings.Contains(err.Error(), "edge 0") {
			t.Fatalf("selectCandidateCells() error = %v, want contextual edge-incidence error", err)
		}
	})

	t.Run("non-positive edge", func(t *testing.T) {
		broken := cloneIslandMesh(mesh)
		broken.edges[0].cornerIDs[1] = broken.edges[0].cornerIDs[0]
		if _, err := selectCandidateCells(7, candidates, broken, shape); err == nil || !strings.Contains(err.Error(), "non-positive length") {
			t.Fatalf("selectCandidateCells() error = %v, want edge-length error", err)
		}
	})

	t.Run("capacity", func(t *testing.T) {
		capacity := (candidates.columns - 2) * (candidates.rows - 2)
		if _, err := selectCandidateCells(capacity+1, candidates, mesh, shape); err == nil || !strings.Contains(err.Error(), "insufficient eligible candidate capacity") {
			t.Fatalf("selectCandidateCells() error = %v, want capacity error", err)
		}
	})
}

func candidateSelectionFixture(t *testing.T, landCount int, seed uint64) (candidateSitePlan, islandMesh, blobShape) {
	t.Helper()
	candidates, err := planCandidateSites(landCount, candidateRandom(seed, 0))
	if err != nil {
		t.Fatalf("planCandidateSites() error = %v", err)
	}
	mesh, err := tessellateIsland(0, candidates.sites)
	if err != nil {
		t.Fatalf("tessellateIsland() error = %v", err)
	}
	return candidates, mesh, generateBlobShape(blobShapeRandom(seed, 0))
}

func cloneCandidateSitePlan(source candidateSitePlan) candidateSitePlan {
	clone := source
	clone.sites = append([]Point(nil), source.sites...)
	return clone
}

func cloneIslandMesh(source islandMesh) islandMesh {
	clone := source
	clone.corners = append([]Point(nil), source.corners...)
	clone.cells = make([]meshCell, len(source.cells))
	for cellID, cell := range source.cells {
		clone.cells[cellID] = cell
		clone.cells[cellID].cornerIDs = append([]int(nil), cell.cornerIDs...)
	}
	clone.edges = make([]meshEdge, len(source.edges))
	for edgeID, edge := range source.edges {
		clone.edges[edgeID] = edge
		clone.edges[edgeID].siteIndexes = append([]int(nil), edge.siteIndexes...)
	}
	return clone
}

func TestCandidateBlobIsDeterministicIncludingScoreTies(t *testing.T) {
	first, err := selectCandidateBlob(53, 42)
	if err != nil {
		t.Fatalf("first selectCandidateBlob() error = %v", err)
	}
	second, err := selectCandidateBlob(53, 42)
	if err != nil {
		t.Fatalf("second selectCandidateBlob() error = %v", err)
	}
	if !reflect.DeepEqual(first.order, second.order) || !reflect.DeepEqual(first.land, second.land) {
		t.Fatal("selection differs for identical count and seed")
	}

	// Equal synthetic scores exercise the explicit candidate-index tie-break
	// independently of floating-point score coincidences.
	values := []int{9, 4, 7, 2}
	sortBlobCandidates(values, make([]float64, 10))
	if want := []int{2, 4, 7, 9}; !reflect.DeepEqual(values, want) {
		t.Fatalf("equal-score order = %v, want %v", values, want)
	}
}

func TestCandidateBlobCapacityPolicy(t *testing.T) {
	for count := 1; count <= 10_000; count++ {
		columns, rows, err := candidateGridDimensions(count)
		if err != nil {
			t.Fatalf("candidateGridDimensions(%d) error = %v", count, err)
		}
		side := columns - 2
		if capacity := side * side; capacity < 2*count {
			t.Fatalf("candidateGridDimensions(%d) capacity = %d, want at least %d", count, capacity, 2*count)
		}
		if side > 1 && (side-1)*(side-1) >= 2*count {
			t.Fatalf("candidateGridDimensions(%d) interior side = %d is not minimal", count, side)
		}
		if columns != rows {
			t.Fatalf("candidateGridDimensions(%d) = %dx%d, want square", count, columns, rows)
		}
	}
}

func TestCandidateGridDimensionsRejectOverflow(t *testing.T) {
	maximumInt := int(^uint(0) >> 1)
	for _, count := range []int{maximumInt/2 + 1, maximumInt / 2} {
		if _, _, err := candidateGridDimensions(count); err == nil || !strings.Contains(err.Error(), "capacity") {
			t.Errorf("candidateGridDimensions(%d) error = %v, want capacity error", count, err)
		}
	}
}

func TestBlobShapeConsumesFixedRandomPrefix(t *testing.T) {
	random := &countingFloat64Source{values: []float64{0.25, 0.75, 0.5}}
	shape := generateBlobShape(random)
	if got, want := random.consumed, 2; got != want {
		t.Fatalf("shape random values consumed = %d, want %d", got, want)
	}
	if got, want := shape.phase3, 0.5*math.Pi; got != want {
		t.Errorf("phase3 = %g, want %g", got, want)
	}
	if got, want := shape.phase5, 1.5*math.Pi; got != want {
		t.Errorf("phase5 = %g, want %g", got, want)
	}
}

type countingFloat64Source struct {
	values   []float64
	consumed int
}

func (source *countingFloat64Source) Float64() float64 {
	value := source.values[source.consumed]
	source.consumed++
	return value
}

func TestCandidateBlobRejectsInvalidCount(t *testing.T) {
	for _, count := range []int{-1, 0} {
		if _, err := selectCandidateBlob(count, 0); err == nil || !strings.Contains(err.Error(), "land count") {
			t.Errorf("selectCandidateBlob(%d) error = %v, want useful land-count error", count, err)
		}
	}
}

func TestCandidateBlobContactSheet(t *testing.T) {
	got := renderBlobContactSheet(t)
	path := filepath.Join("docs", "blob-selection.svg")
	if os.Getenv("WGVC_UPDATE_BLOB_SVG") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read retained contact sheet %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("retained contact sheet is stale; reproduce with WGVC_UPDATE_BLOB_SVG=1 go test -run TestCandidateBlobContactSheet")
	}
}

func assertValidCandidateBlob(t *testing.T, blob candidateBlob, wantLand int) {
	t.Helper()
	if len(blob.sites) != blob.columns*blob.rows || len(blob.geometry.cells) != len(blob.sites) {
		t.Fatalf("candidate dimensions are inconsistent: %dx%d, %d sites, %d cells", blob.columns, blob.rows, len(blob.sites), len(blob.geometry.cells))
	}
	landCount := 0
	for index, selected := range blob.land {
		if !selected {
			continue
		}
		landCount++
		if blob.frame[index] {
			t.Errorf("selected cell %d touches the clipping frame", index)
		}
	}
	if landCount != wantLand || len(blob.order) != wantLand {
		t.Fatalf("selected %d cells in %d order entries, want %d", landCount, len(blob.order), wantLand)
	}

	neighbors := candidateNeighbors(blob.geometry)
	assertSelectionConnected(t, blob.land, neighbors, wantLand)
	assertWaterReachesFrame(t, blob.land, blob.frame, neighbors)
}

func assertSelectionConnected(t *testing.T, selected []bool, neighbors [][]int, want int) {
	t.Helper()
	root := -1
	for index, value := range selected {
		if value {
			root = index
			break
		}
	}
	seen := make([]bool, len(selected))
	seen[root] = true
	queue := []int{root}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbor := range neighbors[current] {
			if selected[neighbor] && !seen[neighbor] {
				seen[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}
	count := 0
	for _, value := range seen {
		if value {
			count++
		}
	}
	if count != want {
		t.Fatalf("shared-edge land component contains %d cells, want %d", count, want)
	}
}

func assertWaterReachesFrame(t *testing.T, land, frame []bool, neighbors [][]int) {
	t.Helper()
	seen := make([]bool, len(land))
	queue := make([]int, 0)
	for index := range land {
		if frame[index] && !land[index] {
			seen[index] = true
			queue = append(queue, index)
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbor := range neighbors[current] {
			if !land[neighbor] && !seen[neighbor] {
				seen[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}
	for index := range land {
		if !land[index] && !seen[index] {
			t.Errorf("omitted cell %d cannot reach frame water", index)
		}
	}
}

func renderBlobContactSheet(t *testing.T) []byte {
	t.Helper()
	cases := []struct {
		count int
		seed  uint64
	}{{1, 0}, {2, 1}, {3, 42}, {17, 1}, {53, 42}, {128, 0xdeadbeef}}

	var svg strings.Builder
	svg.WriteString("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"960\" height=\"710\" viewBox=\"0 0 960 710\">\n")
	svg.WriteString("<rect width=\"960\" height=\"710\" fill=\"#f7f5ef\"/>\n")
	svg.WriteString("<style>text{font-family:ui-monospace,monospace;fill:#17212b}.title{font-size:20px;font-weight:700}.label{font-size:13px}.cell{stroke:#536878;stroke-width:.65}.site{fill:#17212b}</style>\n")
	svg.WriteString("<text class=\"title\" x=\"24\" y=\"30\">Deterministic retained-candidate blob feasibility — issue #9</text>\n")
	for caseIndex, fixture := range cases {
		blob, err := selectCandidateBlob(fixture.count, fixture.seed)
		if err != nil {
			t.Fatalf("contact sheet fixture %+v: %v", fixture, err)
		}
		panelX := 24 + float64(caseIndex%3)*312
		panelY := 52 + float64(caseIndex/3)*300
		const size = 256.0
		fmt.Fprintf(&svg, "<g transform=\"translate(%.0f %.0f)\">\n", panelX, panelY)
		fmt.Fprintf(&svg, "<text class=\"label\" x=\"0\" y=\"14\">seed=%d land=%d candidates=%d</text>\n", fixture.seed, fixture.count, len(blob.sites))
		for index, cell := range blob.geometry.cells {
			fill := "#d9e8ee"
			if blob.frame[index] {
				fill = "#7ca9bd"
			} else if blob.land[index] {
				fill = "#6aa66f"
			}
			fmt.Fprintf(&svg, "<polygon class=\"cell\" fill=\"%s\" points=\"", fill)
			for _, point := range cell.ring {
				fmt.Fprintf(&svg, "%.3f,%.3f ", point.X*size, 24+point.Y*size)
			}
			svg.WriteString("\"/>\n")
		}
		for _, site := range blob.sites {
			fmt.Fprintf(&svg, "<circle class=\"site\" cx=\"%.3f\" cy=\"%.3f\" r=\"0.8\"/>\n", site.X*size, 24+site.Y*size)
		}
		fmt.Fprintf(&svg, "<rect x=\"0\" y=\"24\" width=\"%.0f\" height=\"%.0f\" fill=\"none\" stroke=\"#17212b\" stroke-width=\"2\"/>\n", size, size)
		svg.WriteString("</g>\n")
	}
	svg.WriteString("<text class=\"label\" x=\"24\" y=\"680\">green = selected land · pale blue = omitted water · dark blue = clipping-frame water</text>\n")
	svg.WriteString("<text class=\"label\" x=\"24\" y=\"700\">reproduce: WGVC_UPDATE_BLOB_SVG=1 go test -run TestCandidateBlobContactSheet</text>\n")
	svg.WriteString("</svg>\n")
	return []byte(svg.String())
}
