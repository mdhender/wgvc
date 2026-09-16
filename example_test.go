package wgvc_test

import (
	"fmt"

	"github.com/mdhender/wgvc"
)

func ExampleGenerate() {
	world, err := wgvc.Generate(wgvc.Config{
		WorldSeed:     42,
		ProvinceCount: 20,
		IslandCount:   4,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(len(world.Islands), len(world.Provinces), world.Provinces[0].Terrain)
	// Output: 4 20 plains
}
