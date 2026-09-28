package wgvc

import (
	"testing"

	"github.com/mdhender/wgvc/internal/x24"
)

func TestClassifyDischargeThresholds(t *testing.T) {
	for _, test := range []struct {
		discharge float64
		want      RiverClass
	}{
		{0, ""}, {riverStreamDischarge - 1e-9, ""}, {riverStreamDischarge, RiverClassStream},
		{riverRiverDischarge - 1e-9, RiverClassStream}, {riverRiverDischarge, RiverClassRiver},
		{riverMajorDischarge - 1e-9, RiverClassRiver}, {riverMajorDischarge, RiverClassMajorRiver}, {1e6, RiverClassMajorRiver},
	} {
		if got := ClassifyDischarge(test.discharge); got != test.want {
			t.Errorf("ClassifyDischarge(%g) = %q, want %q", test.discharge, got, test.want)
		}
	}
	if RiverClassStream.Navigable() || !RiverClassRiver.Navigable() || !RiverClassMajorRiver.Navigable() {
		t.Error("navigability: want rivers and major rivers only")
	}
}

// TestDefaultWorldHasRiversOfEveryEndKind pins the river statistics a
// default world produces: dozens of rivers, some navigable, with mouths on the
// ocean, in basins, and at confluences, and sources at both springs and basin
// outflows. Every land corner drains, so total edge discharge into the ocean
// equals total land runoff.
func TestDefaultWorldHasRiversOfEveryEndKind(t *testing.T) {
	world, _, err := GenerateForRender(x24.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	assertValidRivers(t, world)
	mouths := map[RiverEndKind]int{}
	sources := map[RiverEndKind]int{}
	navigable := 0
	for _, river := range world.Rivers {
		mouths[river.Mouth.Kind]++
		sources[river.Source.Kind]++
		if river.Class.Navigable() {
			navigable++
		}
	}
	if len(world.Rivers) < 20 || navigable == 0 {
		t.Errorf("default world has %d rivers, %d navigable; want dozens with some navigable", len(world.Rivers), navigable)
	}
	for _, kind := range []RiverEndKind{RiverEndOcean, RiverEndBasin, RiverEndRiver} {
		if mouths[kind] == 0 {
			t.Errorf("no river has a %s mouth", kind)
		}
	}
	for _, kind := range []RiverEndKind{RiverEndSpring, RiverEndBasin} {
		if sources[kind] == 0 {
			t.Errorf("no river has a %s source", kind)
		}
	}

	// Runoff on a coast corner enters the sea where it falls; every other
	// corner's runoff must arrive over an edge into an ocean corner.
	oceanCorner := make([]bool, len(world.Corners))
	for _, province := range world.Provinces {
		if province.IslandID == NoIslandID && province.BasinID == NoBasinID {
			for _, cornerID := range province.CornerIDs {
				oceanCorner[cornerID] = true
			}
		}
	}
	inlandRunoff := 0.0
	for _, province := range world.Provinces {
		if province.IslandID == NoIslandID {
			continue
		}
		share := province.Area * province.Moisture / float64(len(province.CornerIDs))
		for _, cornerID := range province.CornerIDs {
			if !oceanCorner[cornerID] {
				inlandRunoff += share
			}
		}
	}
	intoOcean := 0.0
	for _, edge := range world.Edges {
		if oceanCorner[edge.CornerIDs[0]] != oceanCorner[edge.CornerIDs[1]] {
			intoOcean += edge.Discharge
		}
	}
	if diff := intoOcean - inlandRunoff; diff > 1e-6*inlandRunoff || diff < -1e-6*inlandRunoff {
		t.Errorf("discharge into ocean = %g, want inland runoff %g", intoOcean, inlandRunoff)
	}
}

// TestRiversTakeTheSeaWhenItIsAdjacent checks that a river corner with a land
// edge into the sea drains into the sea rather than continuing inland or into
// a lake. Every shore corner sits at elevation 0, so a lake shore and a sea
// shore tie on filled elevation; the tie must go to the sea.
func TestRiversTakeTheSeaWhenItIsAdjacent(t *testing.T) {
	config := x24.DefaultConfig()
	config.WorldSeed = 0x0123456789abcdef
	config.ProvinceCount = 10000
	config.IslandCount = 21
	config.OceanPercentage = 0.78
	config.Constellation = "subaru"
	config.AspectRatio = "5:2"
	world, _, err := GenerateForRender(config)
	if err != nil {
		t.Fatal(err)
	}
	assertRiversTakeAdjacentSea(t, world)
}

func assertRiversTakeAdjacentSea(t *testing.T, world World) {
	t.Helper()
	oceanCorner := make([]bool, len(world.Corners))
	for _, province := range world.Provinces {
		if province.IslandID == NoIslandID && province.BasinID == NoBasinID {
			for _, cornerID := range province.CornerIDs {
				oceanCorner[cornerID] = true
			}
		}
	}
	seaNeighbor := make([]bool, len(world.Corners))
	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) != 2 || world.Provinces[edge.ProvinceIDs[0]].IslandID == NoIslandID || world.Provinces[edge.ProvinceIDs[1]].IslandID == NoIslandID {
			continue
		}
		first, second := edge.CornerIDs[0], edge.CornerIDs[1]
		if oceanCorner[second] {
			seaNeighbor[first] = true
		}
		if oceanCorner[first] {
			seaNeighbor[second] = true
		}
	}
	failures := 0
	for _, river := range world.Rivers {
		for position, cornerID := range river.CornerIDs[:len(river.CornerIDs)-1] {
			next := river.CornerIDs[position+1]
			if seaNeighbor[cornerID] && !oceanCorner[next] {
				failures++
				if failures <= 5 {
					t.Errorf("river %d corner %d has a land edge into the sea but flows to corner %d", river.ID, cornerID, next)
				}
			}
		}
	}
	if failures > 5 {
		t.Errorf("%d river corners in all skipped an adjacent sea corner", failures)
	}
}
