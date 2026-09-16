package wgvc

import (
	"fmt"
	"math/big"
	"sort"
)

// These function contracts are deliberately package-private. They define the
// boundaries between generation stages without making the strategies part of
// the public API.
type allocationStage func(provinceCount, islandCount int) ([]int, error)

type islandPlanningStage func(Config, []int) (islandLayout, error)

type tessellationStage func([]islandPlan) (tessellation, error)

type islandPlan struct {
	id                IslandID
	landProvinceCount int
	candidates        candidateSitePlan
	shape             blobShape
	footprint         rectangle
	// provinceCenters drives the pre-blob tessellation until #14 integrates
	// retained candidate cells into Generate.
	provinceCenters []Point
}

type candidateSitePlan struct {
	columns int
	rows    int
	sites   []Point
}

type blobShape struct {
	phase3 float64
	phase5 float64
}

type islandLayout struct {
	islands []islandPlan
	bounds  rectangle
}

type tessellation struct {
	islands []islandMesh
}

// allocateProvinces reserves one province per island, then apportions the
// remainder using descending Fibonacci weights and the largest-remainder
// method. Equal remainders favor the lower island ID.
func allocateProvinces(provinceCount, islandCount int) ([]int, error) {
	if islandCount < 1 {
		return nil, fmt.Errorf("island count must be at least 1: %d", islandCount)
	}
	if provinceCount < islandCount {
		return nil, fmt.Errorf("province count must be at least island count: provinces=%d islands=%d", provinceCount, islandCount)
	}

	weights := descendingFibonacci(islandCount)
	totalWeight := new(big.Int)
	for _, weight := range weights {
		totalWeight.Add(totalWeight, weight)
	}

	allocations := make([]int, islandCount)
	remainders := make([]*big.Int, islandCount)
	remaining := provinceCount - islandCount
	allocated := 0
	remainingBig := new(big.Int).SetUint64(uint64(remaining))
	for i, weight := range weights {
		product := new(big.Int).Mul(remainingBig, weight)
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(product, totalWeight, remainder)
		share := int(quotient.Uint64())
		allocations[i] = 1 + share
		allocated += share
		remainders[i] = remainder
	}

	order := make([]int, islandCount)
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		comparison := remainders[order[i]].Cmp(remainders[order[j]])
		if comparison == 0 {
			return order[i] < order[j]
		}
		return comparison > 0
	})
	for i := 0; i < remaining-allocated; i++ {
		allocations[order[i]]++
	}

	return allocations, nil
}

func descendingFibonacci(count int) []*big.Int {
	ascending := make([]*big.Int, count)
	for i := range ascending {
		switch i {
		case 0, 1:
			ascending[i] = big.NewInt(1)
		default:
			ascending[i] = new(big.Int).Add(ascending[i-1], ascending[i-2])
		}
	}

	descending := make([]*big.Int, count)
	for i := range ascending {
		descending[i] = new(big.Int).Set(ascending[count-1-i])
	}
	return descending
}
