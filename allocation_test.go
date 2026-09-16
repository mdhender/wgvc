package wgvc

import (
	"math/big"
	"reflect"
	"testing"
)

func TestAllocateProvincesFixtures(t *testing.T) {
	tests := []struct {
		name      string
		provinces int
		islands   int
		want      []int
	}{
		{name: "single", provinces: 1, islands: 1, want: []int{1}},
		{name: "two islands", provinces: 5, islands: 2, want: []int{3, 2}},
		{name: "four islands", provinces: 20, islands: 4, want: []int{8, 6, 3, 3}},
		{name: "one each", provinces: 5, islands: 5, want: []int{1, 1, 1, 1, 1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := allocateProvinces(test.provinces, test.islands)
			if err != nil {
				t.Fatalf("allocateProvinces() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("allocateProvinces() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAllocateProvincesLargeIslandCount(t *testing.T) {
	const (
		provinceCount = 10_000
		islandCount   = 100
	)
	allocations, err := allocateProvinces(provinceCount, islandCount)
	if err != nil {
		t.Fatalf("allocateProvinces() error = %v", err)
	}

	total := 0
	for id, allocation := range allocations {
		if allocation < 1 {
			t.Fatalf("island %d received %d provinces", id, allocation)
		}
		total += allocation
	}
	if total != provinceCount {
		t.Fatalf("allocation total = %d, want %d", total, provinceCount)
	}

	weights := descendingFibonacci(islandCount)
	if weights[0].Cmp(new(big.Int).SetUint64(^uint64(0))) <= 0 {
		t.Fatalf("largest weight = %s, want a value larger than uint64", weights[0])
	}
}

func TestAllocateProvincesPreservesCountAndMinimum(t *testing.T) {
	for islandCount := 1; islandCount <= 40; islandCount++ {
		for provinceCount := islandCount; provinceCount <= 200; provinceCount++ {
			allocations, err := allocateProvinces(provinceCount, islandCount)
			if err != nil {
				t.Fatalf("allocateProvinces(%d, %d) error = %v", provinceCount, islandCount, err)
			}
			total := 0
			for islandID, allocation := range allocations {
				if allocation < 1 {
					t.Fatalf("allocateProvinces(%d, %d): island %d received %d", provinceCount, islandCount, islandID, allocation)
				}
				total += allocation
			}
			if total != provinceCount {
				t.Fatalf("allocateProvinces(%d, %d) total = %d", provinceCount, islandCount, total)
			}
		}
	}
}

func TestAllocateProvincesRejectsInvalidCounts(t *testing.T) {
	for _, counts := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {-1, 1}, {2, 3}} {
		if _, err := allocateProvinces(counts[0], counts[1]); err == nil {
			t.Errorf("allocateProvinces(%d, %d) returned no error", counts[0], counts[1])
		}
	}
}
