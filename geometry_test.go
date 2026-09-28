package wgvc

import (
	"math"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestTessellateUnitSquareOneSite(t *testing.T) {
	mesh := mustTessellateUnitSquare(t, 3, []Point{{X: 0.37, Y: 0.61}})
	assertValidMesh(t, mesh)

	if got, want := len(mesh.corners), 4; got != want {
		t.Fatalf("corner count = %d, want %d", got, want)
	}
	if got, want := len(mesh.edges), 4; got != want {
		t.Fatalf("edge count = %d, want %d", got, want)
	}
	if got, want := len(mesh.cells), 1; got != want {
		t.Fatalf("cell count = %d, want %d", got, want)
	}
	for edgeIndex, edge := range mesh.edges {
		if !reflect.DeepEqual(edge.siteIndexes, []int{0}) {
			t.Errorf("edge %d incidence = %v, want [0]", edgeIndex, edge.siteIndexes)
		}
	}
}

func TestTessellateUnitSquareTwoSitesHaveOneInteriorAdjacency(t *testing.T) {
	mesh := mustTessellateUnitSquare(t, 0, []Point{{X: 0.25, Y: 0.5}, {X: 0.75, Y: 0.5}})
	assertValidMesh(t, mesh)

	if got, want := meshAdjacencies(mesh), [][2]int{{0, 1}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("adjacencies = %v, want %v", got, want)
	}
}

func TestTessellateUnitSquareFourSymmetricSitesFormCycle(t *testing.T) {
	sites := []Point{
		{X: 0.25, Y: 0.25},
		{X: 0.75, Y: 0.25},
		{X: 0.75, Y: 0.75},
		{X: 0.25, Y: 0.75},
	}
	mesh := mustTessellateUnitSquare(t, 0, sites)
	assertValidMesh(t, mesh)

	want := [][2]int{{0, 1}, {0, 3}, {1, 2}, {2, 3}}
	if got := meshAdjacencies(mesh); !reflect.DeepEqual(got, want) {
		t.Fatalf("adjacencies = %v, want four-cycle %v", got, want)
	}
}

func TestTessellateUnitSquareCanonicalPartitionCorpus(t *testing.T) {
	corpus := [][]Point{
		{{X: 0.1, Y: 0.5}, {X: 0.35, Y: 0.5}, {X: 0.65, Y: 0.5}, {X: 0.9, Y: 0.5}},
		{{X: 0.13, Y: 0.17}, {X: 0.42, Y: 0.11}, {X: 0.84, Y: 0.23}, {X: 0.27, Y: 0.58}, {X: 0.61, Y: 0.49}, {X: 0.88, Y: 0.76}, {X: 0.38, Y: 0.91}},
		jitteredGridSites(3, 3, 1),
		jitteredGridSites(6, 6, 8675309),
	}

	for corpusIndex, sites := range corpus {
		mesh := mustTessellateUnitSquare(t, IslandID(corpusIndex), sites)
		assertValidMesh(t, mesh)

		repeated := mustTessellateUnitSquare(t, IslandID(corpusIndex), sites)
		if !reflect.DeepEqual(mesh, repeated) {
			t.Fatalf("corpus %d is not canonical across repeated calls", corpusIndex)
		}
	}
}

// jitteredGridSites places one site per cell of a columns×rows grid over the
// unit square, each offset from its cell center by up to a quarter cell.
func jitteredGridSites(columns, rows int, seed uint64) []Point {
	random := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	sites := make([]Point, 0, columns*rows)
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			sites = append(sites, Point{
				X: (float64(column) + 0.5 + 0.5*(random.Float64()-0.5)) / float64(columns),
				Y: (float64(row) + 0.5 + 0.5*(random.Float64()-0.5)) / float64(rows),
			})
		}
	}
	return sites
}

func TestTessellateUnitSquareRejectsMalformedSitesWithContext(t *testing.T) {
	tests := []struct {
		name  string
		sites []Point
		want  string
	}{
		{name: "empty", want: "at least one site"},
		{name: "duplicate", sites: []Point{{X: 0.2, Y: 0.3}, {X: 0.2, Y: 0.3}}, want: "sites 0 and 1 are duplicates"},
		{name: "non-finite", sites: []Point{{X: math.NaN(), Y: 0.5}}, want: "site 0"},
		{name: "boundary", sites: []Point{{X: 0, Y: 0.5}}, want: "site 0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := tessellateRectangle(7, test.sites, 1, 1)
			if err == nil {
				t.Fatal("tessellateRectangle() returned no error")
			}
			if !strings.Contains(err.Error(), "island 7") || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want island and site context containing %q", err, test.want)
			}
		})
	}
}

func mustTessellateUnitSquare(t *testing.T, islandID IslandID, sites []Point) islandMesh {
	t.Helper()
	wantSites := append([]Point(nil), sites...)
	mesh, err := tessellateRectangle(islandID, sites, 1, 1)
	if err != nil {
		t.Fatalf("tessellateRectangle() error = %v", err)
	}
	if !reflect.DeepEqual(sites, wantSites) {
		t.Fatalf("tessellateRectangle() mutated sites: got %v, want %v", sites, wantSites)
	}
	return mesh
}

func assertValidMesh(t *testing.T, mesh islandMesh) {
	t.Helper()
	if len(mesh.cells) == 0 {
		t.Fatal("mesh has no cells")
	}
	if len(mesh.corners) < 4 {
		t.Fatalf("mesh has only %d corners", len(mesh.corners))
	}

	for cornerID, point := range mesh.corners {
		if !finitePoint(point) {
			t.Errorf("corner %d is non-finite: %+v", cornerID, point)
		}
		if point.X < -geometryTolerance || point.X > 1+geometryTolerance || point.Y < -geometryTolerance || point.Y > 1+geometryTolerance {
			t.Errorf("corner %d lies outside unit square: %+v", cornerID, point)
		}
		if cornerID > 0 && !pointLess(mesh.corners[cornerID-1], point) {
			t.Errorf("corners %d and %d are not in strict canonical order", cornerID-1, cornerID)
		}
	}

	edgeIndexes := make(map[[2]int]int, len(mesh.edges))
	for edgeID, edge := range mesh.edges {
		if edge.cornerIDs[0] < 0 || edge.cornerIDs[1] >= len(mesh.corners) || edge.cornerIDs[0] >= edge.cornerIDs[1] {
			t.Errorf("edge %d has invalid canonical corners %v", edgeID, edge.cornerIDs)
			continue
		}
		if edgeID > 0 && !edgePairLess(mesh.edges[edgeID-1].cornerIDs, edge.cornerIDs) {
			t.Errorf("edges %d and %d are not in strict canonical order", edgeID-1, edgeID)
		}
		if got := pointDistance(mesh.corners[edge.cornerIDs[0]], mesh.corners[edge.cornerIDs[1]]); got <= geometryTolerance {
			t.Errorf("edge %d length = %g", edgeID, got)
		}
		if len(edge.siteIndexes) != 1 && len(edge.siteIndexes) != 2 {
			t.Errorf("edge %d incidence = %v", edgeID, edge.siteIndexes)
		}
		for i, siteIndex := range edge.siteIndexes {
			if siteIndex < 0 || siteIndex >= len(mesh.cells) {
				t.Errorf("edge %d has invalid site %d", edgeID, siteIndex)
			}
			if i > 0 && edge.siteIndexes[i-1] >= siteIndex {
				t.Errorf("edge %d incidence is not canonical: %v", edgeID, edge.siteIndexes)
			}
		}
		edgeIndexes[edge.cornerIDs] = edgeID
	}

	totalArea := 0.0
	edgeTraversals := make(map[int][][2]int, len(mesh.edges))
	for cellIndex, cell := range mesh.cells {
		if cell.siteIndex != cellIndex {
			t.Errorf("cell %d has site index %d", cellIndex, cell.siteIndex)
		}
		if !finitePoint(cell.center) {
			t.Errorf("cell %d center is non-finite", cellIndex)
		}
		if len(cell.cornerIDs) < 3 {
			t.Fatalf("cell %d has only %d corners", cellIndex, len(cell.cornerIDs))
		}
		if cell.cornerIDs[0] != minimumInt(cell.cornerIDs) {
			t.Errorf("cell %d does not start at its lowest corner ID: %v", cellIndex, cell.cornerIDs)
		}

		ring := make([]Point, len(cell.cornerIDs))
		for ringIndex, cornerID := range cell.cornerIDs {
			if cornerID < 0 || cornerID >= len(mesh.corners) {
				t.Fatalf("cell %d has invalid corner ID %d", cellIndex, cornerID)
			}
			ring[ringIndex] = mesh.corners[cornerID]
			next := cell.cornerIDs[(ringIndex+1)%len(cell.cornerIDs)]
			edgeID, ok := edgeIndexes[orderedPair(cornerID, next)]
			if !ok {
				t.Errorf("cell %d segment %d->%d has no edge", cellIndex, cornerID, next)
				continue
			}
			if !containsInt(mesh.edges[edgeID].siteIndexes, cellIndex) {
				t.Errorf("cell %d segment edge %d lacks incidence", cellIndex, edgeID)
			}
			edgeTraversals[edgeID] = append(edgeTraversals[edgeID], [2]int{cornerID, next})
		}

		area := signedArea(ring)
		if area <= geometryTolerance {
			t.Errorf("cell %d signed area = %g, want positive", cellIndex, area)
		}
		totalArea += area
		assertConvexCCW(t, cellIndex, ring)
		assertPointInConvexPolygon(t, cellIndex, cell.center, ring)
		assertNearestSiteInequalities(t, cellIndex, ring, mesh.cells)
	}
	if math.Abs(totalArea-1) > geometryTolerance {
		t.Errorf("cell areas sum to %.17g, want 1", totalArea)
	}

	for edgeID, edge := range mesh.edges {
		traversals := edgeTraversals[edgeID]
		if len(traversals) != len(edge.siteIndexes) {
			t.Errorf("edge %d has %d polygon traversals, want %d", edgeID, len(traversals), len(edge.siteIndexes))
		}
		if len(traversals) == 2 && (traversals[0][0] != traversals[1][1] || traversals[0][1] != traversals[1][0]) {
			t.Errorf("interior edge %d traversals are not opposite: %v", edgeID, traversals)
		}
	}
	assertMeshConnected(t, mesh)
}

func assertConvexCCW(t *testing.T, cellIndex int, ring []Point) {
	t.Helper()
	for i, point := range ring {
		next := ring[(i+1)%len(ring)]
		after := ring[(i+2)%len(ring)]
		if cross(point, next, after) < -geometryTolerance {
			t.Errorf("cell %d is not convex and counterclockwise at vertex %d", cellIndex, i)
		}
	}
}

func assertPointInConvexPolygon(t *testing.T, cellIndex int, point Point, ring []Point) {
	t.Helper()
	for i, start := range ring {
		if cross(start, ring[(i+1)%len(ring)], point) < -geometryTolerance {
			t.Errorf("cell %d does not contain its generating site %+v", cellIndex, point)
			return
		}
	}
}

func assertNearestSiteInequalities(t *testing.T, cellIndex int, ring []Point, cells []meshCell) {
	t.Helper()
	center := cells[cellIndex].center
	for cornerIndex, corner := range ring {
		ownDistance := squaredDistance(corner, center)
		for otherIndex, other := range cells {
			if ownDistance-squaredDistance(corner, other.center) > 4*geometryTolerance {
				t.Errorf("cell %d corner %d is closer to site %d", cellIndex, cornerIndex, otherIndex)
			}
		}
	}
}

func assertMeshConnected(t *testing.T, mesh islandMesh) {
	t.Helper()
	seen := make([]bool, len(mesh.cells))
	seen[0] = true
	queue := []int{0}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range mesh.edges {
			if len(edge.siteIndexes) != 2 {
				continue
			}
			var neighbor int
			switch current {
			case edge.siteIndexes[0]:
				neighbor = edge.siteIndexes[1]
			case edge.siteIndexes[1]:
				neighbor = edge.siteIndexes[0]
			default:
				continue
			}
			if !seen[neighbor] {
				seen[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}
	for siteIndex, reached := range seen {
		if !reached {
			t.Errorf("cell adjacency graph does not reach site %d", siteIndex)
		}
	}
}

func meshAdjacencies(mesh islandMesh) [][2]int {
	adjacencies := make([][2]int, 0)
	for _, edge := range mesh.edges {
		if len(edge.siteIndexes) == 2 {
			adjacencies = append(adjacencies, [2]int{edge.siteIndexes[0], edge.siteIndexes[1]})
		}
	}
	sort.Slice(adjacencies, func(i, j int) bool {
		if adjacencies[i][0] != adjacencies[j][0] {
			return adjacencies[i][0] < adjacencies[j][0]
		}
		return adjacencies[i][1] < adjacencies[j][1]
	})
	return adjacencies
}

func cross(first, second, third Point) float64 {
	return (second.X-first.X)*(third.Y-first.Y) - (second.Y-first.Y)*(third.X-first.X)
}

func squaredDistance(first, second Point) float64 {
	dx := first.X - second.X
	dy := first.Y - second.Y
	return dx*dx + dy*dy
}

// Corner IDs follow exact coordinate order, so a border corner a few ulps off
// its side can sort differently depending on whether the platform fused a
// multiply-add while clipping (arm64 does, amd64 does not).
func TestTessellateRectangleSnapsBorderCornersExactly(t *testing.T) {
	const width, height = 13.7, 9.1
	for seed := uint64(1); seed <= 8; seed++ {
		source := rand.New(rand.NewPCG(seed, 0xfedcba9876543210))
		sites := make([]Point, 400)
		for i := range sites {
			sites[i] = Point{X: width * (0.001 + 0.998*source.Float64()), Y: height * (0.001 + 0.998*source.Float64())}
		}
		mesh, err := tessellateRectangle(NoIslandID, sites, width, height)
		if err != nil {
			t.Fatalf("seed %d: tessellateRectangle() error = %v", seed, err)
		}

		borderCorners := 0
		for cornerID, point := range mesh.corners {
			for _, coordinate := range []struct {
				value, limit float64
			}{{point.X, width}, {point.Y, height}} {
				for _, side := range []float64{0, coordinate.limit} {
					if near(coordinate.value, side) {
						borderCorners++
						if coordinate.value != side {
							t.Errorf("seed %d: corner %d coordinate = %.17g, want exactly %g", seed, cornerID, coordinate.value, side)
						}
					}
				}
			}
		}
		if borderCorners == 0 {
			t.Fatalf("seed %d: no corners on the rectangle border", seed)
		}
	}
}
