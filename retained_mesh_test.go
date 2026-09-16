package wgvc

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestExtractRetainedMeshFiltersAndCompactsTopology(t *testing.T) {
	candidate := mustTessellateIsland(t, 7, regularCandidateSites(2, 2))
	originalCandidate := cloneIslandMesh(candidate)
	selected := []int{1, 0}
	originalSelection := append([]int(nil), selected...)

	retained, err := extractRetainedMesh(candidate, selected)
	if err != nil {
		t.Fatalf("extractRetainedMesh() error = %v", err)
	}
	if !reflect.DeepEqual(candidate, originalCandidate) {
		t.Fatal("extraction mutated the candidate mesh")
	}
	if !reflect.DeepEqual(selected, originalSelection) {
		t.Fatal("extraction mutated the caller-owned selection")
	}
	if retained.islandID != candidate.islandID {
		t.Errorf("retained island ID = %d, want %d", retained.islandID, candidate.islandID)
	}
	if got, want := len(retained.cells), 2; got != want {
		t.Fatalf("retained cell count = %d, want %d", got, want)
	}
	for cellID, candidateID := range []int{0, 1} {
		if retained.cells[cellID].siteIndex != cellID {
			t.Errorf("retained cell %d site index = %d", cellID, retained.cells[cellID].siteIndex)
		}
		if retained.cells[cellID].center != candidate.cells[candidateID].center {
			t.Errorf("retained cell %d center = %+v, want exact candidate center %+v", cellID, retained.cells[cellID].center, candidate.cells[candidateID].center)
		}
		gotRing := meshCellPoints(retained, cellID)
		wantRing := meshCellPoints(candidate, candidateID)
		if !reflect.DeepEqual(gotRing, wantRing) {
			t.Errorf("retained cell %d ring coordinates changed: got %v, want %v", cellID, gotRing, wantRing)
		}
	}

	wantEdges := retainedEdgeCoordinates(candidate, map[int]int{0: 0, 1: 1})
	gotEdges := retainedEdgeCoordinates(retained, map[int]int{0: 0, 1: 1})
	if !reflect.DeepEqual(gotEdges, wantEdges) {
		t.Errorf("retained edge geometry/incidence changed:\n got %v\nwant %v", gotEdges, wantEdges)
	}
	incidenceCounts := [3]int{}
	for _, edge := range candidate.edges {
		retainedIncidence := 0
		for _, candidateID := range edge.siteIndexes {
			if candidateID == 0 || candidateID == 1 {
				retainedIncidence++
			}
		}
		incidenceCounts[retainedIncidence]++
	}
	for incidence := range incidenceCounts {
		if incidenceCounts[incidence] == 0 {
			t.Errorf("fixture has no edge with %d retained incident cells", incidence)
		}
	}
	if got, want := len(retained.edges), incidenceCounts[1]+incidenceCounts[2]; got != want {
		t.Errorf("retained edge count = %d, want %d", got, want)
	}

	if err := validateRetainedLandMesh(retained); err != nil {
		t.Fatalf("validateRetainedLandMesh() error = %v", err)
	}
	if err := validateCompleteCandidateMesh(retained); err == nil || !strings.Contains(err.Error(), "want 1") {
		t.Fatalf("validateCompleteCandidateMesh(retained) error = %v, want unit-coverage error", err)
	}
	area := meshArea(retained)
	if math.Abs(area-0.5) > geometryTolerance {
		t.Errorf("retained land area = %.17g, want 0.5 without retessellation", area)
	}

	repeated, err := extractRetainedMesh(candidate, selected)
	if err != nil {
		t.Fatalf("repeated extractRetainedMesh() error = %v", err)
	}
	if !reflect.DeepEqual(repeated, retained) {
		t.Fatal("repeated extraction returned a different mesh")
	}
}

func TestExtractRetainedMeshGeneratedCorpus(t *testing.T) {
	for _, count := range []int{1, 7, 53} {
		for _, seed := range []uint64{0, 42} {
			t.Run(fmt.Sprintf("land%d/seed%d", count, seed), func(t *testing.T) {
				candidates, candidate, shape := candidateSelectionFixture(t, count, seed)
				selected, err := selectCandidateCells(count, candidates, candidate, shape)
				if err != nil {
					t.Fatalf("selectCandidateCells() error = %v", err)
				}
				retained, err := extractRetainedMesh(candidate, selected)
				if err != nil {
					t.Fatalf("extractRetainedMesh() error = %v", err)
				}
				if len(retained.cells) != count {
					t.Fatalf("retained cell count = %d, want %d", len(retained.cells), count)
				}
				for cellID, candidateID := range selected {
					if retained.cells[cellID].center != candidate.cells[candidateID].center {
						t.Errorf("retained cell %d did not preserve candidate %d center", cellID, candidateID)
					}
				}
				if err := validateRetainedLandMesh(retained); err != nil {
					t.Fatalf("validateRetainedLandMesh() error = %v", err)
				}
			})
		}
	}
}

func TestExtractRetainedMeshRejectsInvalidSelectionAndCandidateMesh(t *testing.T) {
	candidate := mustTessellateIsland(t, 0, regularCandidateSites(2, 2))
	for _, test := range []struct {
		name      string
		selection []int
		want      string
	}{
		{name: "empty", selection: nil, want: "at least one"},
		{name: "negative", selection: []int{-1}, want: "outside"},
		{name: "past end", selection: []int{len(candidate.cells)}, want: "outside"},
		{name: "duplicate", selection: []int{1, 1}, want: "repeated"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := extractRetainedMesh(candidate, test.selection); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("extractRetainedMesh() error = %v, want error containing %q", err, test.want)
			}
		})
	}

	broken := cloneIslandMesh(candidate)
	broken.edges = broken.edges[1:]
	if _, err := extractRetainedMesh(broken, []int{0}); err == nil || !strings.Contains(err.Error(), "complete candidate mesh") {
		t.Fatalf("extractRetainedMesh(broken) error = %v, want complete-mesh validation error", err)
	}
}

type coordinateEdge struct {
	ends     [2]Point
	incident []int
}

func retainedEdgeCoordinates(mesh islandMesh, retainedCandidateToCell map[int]int) []coordinateEdge {
	edges := make([]coordinateEdge, 0, len(mesh.edges))
	for _, edge := range mesh.edges {
		incident := make([]int, 0, len(edge.siteIndexes))
		for _, candidateID := range edge.siteIndexes {
			if cellID, retained := retainedCandidateToCell[candidateID]; retained {
				incident = append(incident, cellID)
			}
		}
		if len(incident) == 0 {
			continue
		}
		edges = append(edges, coordinateEdge{
			ends: [2]Point{
				mesh.corners[edge.cornerIDs[0]],
				mesh.corners[edge.cornerIDs[1]],
			},
			incident: incident,
		})
	}
	return edges
}

func meshCellPoints(mesh islandMesh, cellID int) []Point {
	points := make([]Point, len(mesh.cells[cellID].cornerIDs))
	for ringIndex, cornerID := range mesh.cells[cellID].cornerIDs {
		points[ringIndex] = mesh.corners[cornerID]
	}
	return points
}

func meshArea(mesh islandMesh) float64 {
	area := 0.0
	for cellID := range mesh.cells {
		area += signedArea(meshCellPoints(mesh, cellID))
	}
	return area
}
