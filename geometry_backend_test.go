package wgvc

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"
)

// backendTolerance matches the selected backend's absolute epsilon after all
// sites and the clipping rectangle have been normalized to the unit square.
const backendTolerance = geometryTolerance

func TestGeometryBackendOneSiteUsesClippingSquare(t *testing.T) {
	geometry := mustComputeBackendGeometry(t, []Point{{X: 0.37, Y: 0.61}})
	assertValidPartition(t, geometry, 1)
	if got := polygonArea(geometry.cells[0].ring); !near(got, 1) {
		t.Fatalf("single cell area = %g, want 1", got)
	}
	if got := interiorAdjacencies(geometry); len(got) != 0 {
		t.Fatalf("single cell adjacencies = %v, want none", got)
	}
}

func TestGeometryBackendTwoSites(t *testing.T) {
	tests := []struct {
		name  string
		sites []Point
	}{
		{name: "horizontal", sites: []Point{{X: 0.25, Y: 0.5}, {X: 0.75, Y: 0.5}}},
		{name: "vertical", sites: []Point{{X: 0.5, Y: 0.25}, {X: 0.5, Y: 0.75}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			geometry := mustComputeBackendGeometry(t, test.sites)
			assertValidPartition(t, geometry, len(test.sites))
			assertAdjacencies(t, geometry, [][2]int{{0, 1}})
			for i, cell := range geometry.cells {
				if got := polygonArea(cell.ring); !near(got, 0.5) {
					t.Errorf("cell %d area = %g, want 0.5", i, got)
				}
			}
		})
	}
}

func TestGeometryBackendCollinearAndNearCollinearSites(t *testing.T) {
	tests := []struct {
		name  string
		sites []Point
		want  [][2]int
	}{
		{
			name:  "horizontal collinear",
			sites: []Point{{X: 0.1, Y: 0.5}, {X: 0.35, Y: 0.5}, {X: 0.65, Y: 0.5}, {X: 0.9, Y: 0.5}},
			want:  [][2]int{{0, 1}, {1, 2}, {2, 3}},
		},
		{
			name:  "vertical collinear",
			sites: []Point{{X: 0.5, Y: 0.9}, {X: 0.5, Y: 0.35}, {X: 0.5, Y: 0.65}, {X: 0.5, Y: 0.1}},
			want:  [][2]int{{0, 2}, {1, 2}, {1, 3}},
		},
		{
			name:  "near collinear",
			sites: []Point{{X: 0.1, Y: 0.5}, {X: 0.35, Y: 0.50000001}, {X: 0.65, Y: 0.49999999}, {X: 0.9, Y: 0.5}},
			want:  [][2]int{{0, 1}, {1, 2}, {2, 3}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			geometry := mustComputeBackendGeometry(t, test.sites)
			assertValidPartition(t, geometry, len(test.sites))
			assertAdjacencies(t, geometry, test.want)
		})
	}
}

func TestGeometryBackendCocircularPointContactIsNotAdjacency(t *testing.T) {
	sites := []Point{
		{X: 0.25, Y: 0.25},
		{X: 0.75, Y: 0.25},
		{X: 0.75, Y: 0.75},
		{X: 0.25, Y: 0.75},
	}
	geometry := mustComputeBackendGeometry(t, sites)
	assertValidPartition(t, geometry, len(sites))
	assertAdjacencies(t, geometry, [][2]int{{0, 1}, {0, 3}, {1, 2}, {2, 3}})
}

func TestGeometryBackendEqualAxisGrid(t *testing.T) {
	const side = 4
	sites := make([]Point, 0, side*side)
	for y := range side {
		for x := range side {
			sites = append(sites, Point{
				X: (float64(x) + 0.5) / side,
				Y: (float64(y) + 0.5) / side,
			})
		}
	}
	geometry := mustComputeBackendGeometry(t, sites)
	assertValidPartition(t, geometry, len(sites))
}

func TestGeometryBackendPreservesCallerInputAndOrder(t *testing.T) {
	sites := []Point{
		{X: 0.78, Y: 0.72},
		{X: 0.19, Y: 0.23},
		{X: 0.81, Y: 0.18},
		{X: 0.22, Y: 0.79},
		{X: 0.51, Y: 0.46},
	}
	wantSites := append([]Point(nil), sites...)
	geometry := mustComputeBackendGeometry(t, sites)
	if !reflect.DeepEqual(sites, wantSites) {
		t.Fatalf("caller sites changed from %v to %v", wantSites, sites)
	}
	for i, cell := range geometry.cells {
		if cell.siteIndex != i {
			t.Fatalf("cell %d maps to caller site %d", i, cell.siteIndex)
		}
	}
	assertValidPartition(t, geometry, len(sites))
}

func TestGeometryBackendIsRepeatable(t *testing.T) {
	sites := []Point{
		{X: 0.13, Y: 0.17},
		{X: 0.42, Y: 0.11},
		{X: 0.84, Y: 0.23},
		{X: 0.27, Y: 0.58},
		{X: 0.61, Y: 0.49},
		{X: 0.88, Y: 0.76},
		{X: 0.38, Y: 0.91},
	}
	first := mustComputeBackendGeometry(t, sites)
	second := mustComputeBackendGeometry(t, sites)
	assertEquivalentGeometry(t, first, second)
}

func TestGeometryBackendIrregularCorpus(t *testing.T) {
	corpus := [][]Point{
		{{X: 0.18, Y: 0.21}, {X: 0.79, Y: 0.27}, {X: 0.47, Y: 0.83}},
		{{X: 0.12, Y: 0.15}, {X: 0.44, Y: 0.19}, {X: 0.82, Y: 0.14}, {X: 0.23, Y: 0.74}, {X: 0.72, Y: 0.68}},
		{{X: 0.08, Y: 0.31}, {X: 0.29, Y: 0.08}, {X: 0.53, Y: 0.24}, {X: 0.87, Y: 0.12}, {X: 0.16, Y: 0.66}, {X: 0.48, Y: 0.57}, {X: 0.79, Y: 0.49}, {X: 0.68, Y: 0.88}},
	}
	for i, sites := range corpus {
		t.Run(fmt.Sprintf("set %d", i), func(t *testing.T) {
			geometry := mustComputeBackendGeometry(t, sites)
			assertValidPartition(t, geometry, len(sites))
		})
	}
}

func TestGeometryBackendRejectsInvalidSites(t *testing.T) {
	tests := [][]Point{
		nil,
		{{X: 0.2, Y: 0.3}, {X: 0.2, Y: 0.3}},
		{{X: math.NaN(), Y: 0.5}},
		{{X: 0, Y: 0.5}},
		{{X: 1, Y: 0.5}},
		{{X: 1.1, Y: 0.5}},
	}
	for _, sites := range tests {
		if _, err := computeBackendGeometry(sites); err == nil {
			t.Errorf("computeBackendGeometry(%v) returned no error", sites)
		}
	}
}

func mustComputeBackendGeometry(t *testing.T, sites []Point) backendGeometry {
	t.Helper()
	geometry, err := computeBackendGeometry(sites)
	if err != nil {
		t.Fatalf("computeBackendGeometry() error = %v", err)
	}
	return geometry
}

func assertValidPartition(t *testing.T, geometry backendGeometry, siteCount int) {
	t.Helper()
	if len(geometry.cells) != siteCount {
		t.Fatalf("cell count = %d, want %d", len(geometry.cells), siteCount)
	}

	totalArea := 0.0
	for cellIndex, cell := range geometry.cells {
		if cell.siteIndex != cellIndex {
			t.Errorf("cell %d has site index %d", cellIndex, cell.siteIndex)
		}
		if len(cell.ring) < 3 {
			t.Fatalf("cell %d has only %d corners", cellIndex, len(cell.ring))
		}
		area := polygonArea(cell.ring)
		if area <= backendTolerance {
			t.Errorf("cell %d has non-positive area %g", cellIndex, area)
		}
		totalArea += area
		for _, point := range cell.ring {
			if !finitePoint(point) {
				t.Errorf("cell %d contains non-finite point %+v", cellIndex, point)
			}
			if point.X < -backendTolerance || point.X > 1+backendTolerance || point.Y < -backendTolerance || point.Y > 1+backendTolerance {
				t.Errorf("cell %d point lies outside unit square: %+v", cellIndex, point)
			}
		}
	}
	if !near(totalArea, 1) {
		t.Errorf("total cell area = %.17g, want 1", totalArea)
	}

	for edgeIndex, edge := range geometry.edges {
		if !finitePoint(edge.ends[0]) || !finitePoint(edge.ends[1]) {
			t.Errorf("edge %d has non-finite endpoint: %+v", edgeIndex, edge.ends)
		}
		if pointDistance(edge.ends[0], edge.ends[1]) <= backendTolerance {
			t.Errorf("edge %d is degenerate: %+v", edgeIndex, edge.ends)
		}
		if len(edge.siteIndexes) < 1 || len(edge.siteIndexes) > 2 {
			t.Errorf("edge %d has invalid incidence %v", edgeIndex, edge.siteIndexes)
		}
		for _, siteIndex := range edge.siteIndexes {
			if siteIndex < 0 || siteIndex >= siteCount {
				t.Errorf("edge %d has invalid site index %d", edgeIndex, siteIndex)
				continue
			}
			if !ringHasEdge(geometry.cells[siteIndex].ring, edge.ends) {
				t.Errorf("edge %d is missing from incident cell %d", edgeIndex, siteIndex)
			}
		}
	}
}

func assertAdjacencies(t *testing.T, geometry backendGeometry, want [][2]int) {
	t.Helper()
	got := interiorAdjacencies(geometry)
	sort.Slice(want, func(i, j int) bool {
		if want[i][0] != want[j][0] {
			return want[i][0] < want[j][0]
		}
		return want[i][1] < want[j][1]
	})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interior adjacencies = %v, want %v", got, want)
	}
}

func interiorAdjacencies(geometry backendGeometry) [][2]int {
	adjacencies := make([][2]int, 0)
	for _, edge := range geometry.edges {
		if len(edge.siteIndexes) == 2 && pointDistance(edge.ends[0], edge.ends[1]) > backendTolerance {
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

func assertEquivalentGeometry(t *testing.T, first, second backendGeometry) {
	t.Helper()
	if len(first.cells) != len(second.cells) || len(first.edges) != len(second.edges) {
		t.Fatalf("geometry sizes differ: cells %d/%d, edges %d/%d", len(first.cells), len(second.cells), len(first.edges), len(second.edges))
	}
	for i := range first.cells {
		if first.cells[i].siteIndex != second.cells[i].siteIndex || !equivalentRings(first.cells[i].ring, second.cells[i].ring) {
			t.Errorf("cell %d differs between calls", i)
		}
	}
	if !reflect.DeepEqual(interiorAdjacencies(first), interiorAdjacencies(second)) {
		t.Errorf("adjacency differs between calls")
	}
}

func equivalentRings(first, second []Point) bool {
	if len(first) != len(second) {
		return false
	}
	for offset := range second {
		matches := true
		for i := range first {
			if !pointsNear(first[i], second[(i+offset)%len(second)]) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func ringHasEdge(ring []Point, ends [2]Point) bool {
	for i, start := range ring {
		end := ring[(i+1)%len(ring)]
		if pointsNear(start, ends[0]) && pointsNear(end, ends[1]) || pointsNear(start, ends[1]) && pointsNear(end, ends[0]) {
			return true
		}
	}
	return false
}

func polygonArea(ring []Point) float64 {
	return math.Abs(signedArea(ring))
}
