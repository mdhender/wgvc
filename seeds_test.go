package wgvc

import (
	"math"
	"reflect"
	"strconv"
	"testing"
)

func TestGenerateProvinceSeedsFixtures(t *testing.T) {
	for _, count := range []int{1, 2, 7, 10} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			seeds := generateProvinceSeeds(count, provinceRandom(42, 0))
			if got := len(seeds); got != count {
				t.Fatalf("seed count = %d, want %d", got, count)
			}
			for index, seed := range seeds {
				if !(seed.X > 0 && seed.X < 1 && seed.Y > 0 && seed.Y < 1) {
					t.Errorf("seed %d = %+v, want point strictly inside unit square", index, seed)
				}
			}
			minimumSeparation := 2 * seedLeafInset / float64(count)
			for first := range seeds {
				for second := first + 1; second < len(seeds); second++ {
					distance := math.Hypot(seeds[first].X-seeds[second].X, seeds[first].Y-seeds[second].Y)
					if distance < minimumSeparation-1e-12 {
						t.Errorf("seeds %d and %d separated by %g, want at least %g", first, second, distance, minimumSeparation)
					}
				}
			}
		})
	}
}

func TestProvinceSeedStreamsAreDeterministicAndIndependent(t *testing.T) {
	want := generateProvinceSeeds(7, provinceRandom(99, 1))
	firstIsland := provinceRandom(99, 0)
	for range 1000 {
		firstIsland.Uint64()
	}
	got := generateProvinceSeeds(7, provinceRandom(99, 1))
	if !reflect.DeepEqual(got, want) {
		t.Fatal("consuming island 0 randomness changed island 1 seeds")
	}
	placement := placementRandom(99)
	for range 1000 {
		placement.Uint64()
	}
	got = generateProvinceSeeds(7, provinceRandom(99, 1))
	if !reflect.DeepEqual(got, want) {
		t.Fatal("consuming placement randomness changed province seeds")
	}

	repeated := generateProvinceSeeds(7, provinceRandom(99, 1))
	if !reflect.DeepEqual(repeated, want) {
		t.Fatal("reconstructed stream produced different seeds")
	}
}
