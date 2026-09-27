package wgvc

import (
	"reflect"
	"testing"
)

// chainWorld builds a straight chain of provinces joined by edges, with the
// given island membership (NoIslandID for water), and the islands they form.
func chainWorld(memberships []IslandID) World {
	var world World
	for id, islandID := range memberships {
		world.Provinces = append(world.Provinces, Province{ID: ProvinceID(id), IslandID: islandID, BasinID: NoBasinID})
		if islandID != NoIslandID {
			for int(islandID) >= len(world.Islands) {
				world.Islands = append(world.Islands, Island{ID: IslandID(len(world.Islands))})
			}
			world.Islands[islandID].ProvinceIDs = append(world.Islands[islandID].ProvinceIDs, ProvinceID(id))
		}
		if id > 0 {
			world.Edges = append(world.Edges, Edge{ID: EdgeID(id - 1), ProvinceIDs: []ProvinceID{ProvinceID(id - 1), ProvinceID(id)}})
		}
	}
	return world
}

func TestStraitsJoinIslandsThroughAtMostThreeWaterProvinces(t *testing.T) {
	const w = NoIslandID
	for _, test := range []struct {
		name string
		row  []IslandID
		want []Strait
	}{
		{"one water province", []IslandID{0, w, 1}, []Strait{{ID: 0, IslandIDs: [2]IslandID{0, 1}, Width: 1, ProvinceIDs: []ProvinceID{1}, Shores: [2][]ProvinceID{{0}, {2}}}}},
		{"three water provinces", []IslandID{0, 0, w, w, w, 1}, []Strait{{ID: 0, IslandIDs: [2]IslandID{0, 1}, Width: 3, ProvinceIDs: []ProvinceID{2, 3, 4}, Shores: [2][]ProvinceID{{1}, {5}}}}},
		{"four water provinces is no strait", []IslandID{0, w, w, w, w, 1}, nil},
		{"same island is no strait", []IslandID{0, w, 0}, nil},
	} {
		world := chainWorld(test.row)
		assignStraits(&world)
		if !reflect.DeepEqual(world.Straits, test.want) {
			t.Errorf("%s: straits = %+v, want %+v", test.name, world.Straits, test.want)
		}
	}
}

// TestNecksAreSmallCutsBetweenLargeRegions builds one island of two
// ten-province rings joined by a single province, beside one water province
// so coast distances exist, and expects that province as a width-1 neck.
func TestNecksAreSmallCutsBetweenLargeRegions(t *testing.T) {
	var world World
	world.Islands = []Island{{ID: 0}}
	addLand := func() ProvinceID {
		id := ProvinceID(len(world.Provinces))
		world.Provinces = append(world.Provinces, Province{ID: id, IslandID: 0, BasinID: NoBasinID})
		world.Islands[0].ProvinceIDs = append(world.Islands[0].ProvinceIDs, id)
		return id
	}
	join := func(a, b ProvinceID) {
		world.Edges = append(world.Edges, Edge{ID: EdgeID(len(world.Edges)), ProvinceIDs: []ProvinceID{min(a, b), max(a, b)}})
	}
	ring := func() []ProvinceID {
		var ids []ProvinceID
		for range 10 {
			ids = append(ids, addLand())
		}
		for i := range ids {
			join(ids[i], ids[(i+1)%len(ids)])
		}
		return ids
	}
	first := ring()
	bridge := addLand()
	second := ring()
	join(first[0], bridge)
	join(bridge, second[0])
	water := ProvinceID(len(world.Provinces))
	world.Provinces = append(world.Provinces, Province{ID: water, IslandID: NoIslandID, BasinID: NoBasinID})
	join(bridge, water)
	assignCoastDistances(&world)
	assignNecks(&world)
	want := []Neck{{ID: 0, IslandID: 0, Width: 1, ProvinceIDs: []ProvinceID{bridge}, Ends: [2][]ProvinceID{{first[0]}, {second[0]}}, EndSizes: [2]int{10, 10}}}
	if !reflect.DeepEqual(world.Necks, want) {
		t.Errorf("necks = %+v, want %+v", world.Necks, want)
	}

	// Shrink one ring below the minimum region and the neck disappears.
	small := chainWorld([]IslandID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, NoIslandID})
	assignCoastDistances(&small)
	assignNecks(&small)
	if len(small.Necks) != 0 {
		t.Errorf("chain of 12 has necks %+v, want none (no two regions of %d)", small.Necks, neckMinimumRegion)
	}
}

func TestSeaZonesCoverOceanInBoundedConnectedZones(t *testing.T) {
	world, err := Generate(Config{WorldSeed: 42, ProvinceCount: 1_500, IslandCount: 15})
	if err != nil {
		t.Fatal(err)
	}
	ocean := 0
	for _, province := range world.Provinces {
		if province.IslandID == NoIslandID && province.BasinID == NoBasinID {
			ocean++
		}
	}
	covered := 0
	for _, zone := range world.SeaZones {
		covered += len(zone.ProvinceIDs)
	}
	if covered != ocean || len(world.SeaZones) < ocean/(2*seaZoneTargetSize) {
		t.Errorf("%d zones cover %d of %d ocean provinces", len(world.SeaZones), covered, ocean)
	}
	assertValidSeas(t, world)
}
