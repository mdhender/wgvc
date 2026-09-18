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

	land, water := 0, 0
	for _, province := range world.Provinces {
		if province.IslandID == wgvc.NoIslandID {
			water++
		} else {
			land++
		}
	}
	fmt.Println(len(world.Islands), land, water)
	// Output: 1 20 43
}
