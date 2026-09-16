package wgvc

import (
	"math"
	"reflect"
	"testing"
)

func TestCandidateSitePlansHaveBoundedCapacityAndSeparation(t *testing.T) {
	counts := []int{1, 2, 3, 7, 53, 128}
	seeds := []uint64{0, 1, 42, 0xdeadbeef}
	for _, count := range counts {
		for _, seed := range seeds {
			plan, err := planCandidateSites(count, candidateRandom(seed, 0))
			if err != nil {
				t.Fatalf("planCandidateSites(%d, seed %d) error = %v", count, seed, err)
			}
			if got, want := len(plan.sites), plan.columns*plan.rows; got != want {
				t.Fatalf("candidate count = %d, want %d", got, want)
			}
			if capacity := (plan.columns - 2) * (plan.rows - 2); capacity < 2*count {
				t.Fatalf("interior capacity = %d, want at least %d", capacity, 2*count)
			}

			minimumSeparation := 2 * candidateSiteInset / float64(max(plan.columns, plan.rows))
			seen := make(map[Point]int, len(plan.sites))
			for index, site := range plan.sites {
				if !finitePoint(site) || site.X <= 0 || site.X >= 1 || site.Y <= 0 || site.Y >= 1 {
					t.Errorf("candidate %d = %+v, want finite point strictly inside unit square", index, site)
				}
				if previous, exists := seen[site]; exists {
					t.Errorf("candidates %d and %d are duplicates", previous, index)
				}
				seen[site] = index
				for previous := range index {
					distance := math.Hypot(site.X-plan.sites[previous].X, site.Y-plan.sites[previous].Y)
					if distance < minimumSeparation-1e-12 {
						t.Errorf("candidates %d and %d separated by %g, want at least %g", previous, index, distance, minimumSeparation)
					}
				}
			}

			geometry, err := computeBackendGeometry(plan.sites)
			if err != nil {
				t.Fatalf("computeBackendGeometry() error = %v", err)
			}
			if _, err := canonicalizeGeometry(0, plan.sites, geometry); err != nil {
				t.Fatalf("canonicalizeGeometry() error = %v", err)
			}
			neighbors := candidateNeighbors(geometry)
			if err := validateCandidateGrid(plan.columns, plan.rows, neighbors); err != nil {
				t.Fatal(err)
			}
			frame := candidateFrameCells(geometry)
			for index := range plan.sites {
				row, column := index/plan.columns, index%plan.columns
				wantFrame := row == 0 || row == plan.rows-1 || column == 0 || column == plan.columns-1
				if frame[index] != wantFrame {
					t.Errorf("candidate %d frame contact = %t, want %t", index, frame[index], wantFrame)
				}
			}
		}
	}
}

func TestCandidateSitePlansAreDeterministicAndSeedSensitive(t *testing.T) {
	want, err := planCandidateSites(53, candidateRandom(99, 2))
	if err != nil {
		t.Fatalf("planCandidateSites() error = %v", err)
	}
	repeated, err := planCandidateSites(53, candidateRandom(99, 2))
	if err != nil {
		t.Fatalf("repeated planCandidateSites() error = %v", err)
	}
	if !reflect.DeepEqual(repeated, want) {
		t.Fatal("identical candidate inputs produced different plans")
	}
	different, err := planCandidateSites(53, candidateRandom(100, 2))
	if err != nil {
		t.Fatalf("seed-changed planCandidateSites() error = %v", err)
	}
	if reflect.DeepEqual(different.sites, want.sites) {
		t.Fatal("different world seeds produced identical candidate sites")
	}
}
