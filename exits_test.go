package wgvc

import "testing"

func TestOutwardBearingOfCounterclockwiseSquare(t *testing.T) {
	// A unit square traversed counterclockwise with +Y north: the bottom
	// edge faces south, the right edge east, the top north, the left west.
	corners := []Point{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	want := []float64{180, 90, 0, 270}
	for i := range corners {
		if got := outwardBearing(corners[i], corners[(i+1)%4]); got != want[i] {
			t.Errorf("segment %d bearing = %g, want %g", i, got, want[i])
		}
	}
}

func TestCompassForSectors(t *testing.T) {
	for _, test := range []struct {
		bearing float64
		want    Compass
	}{
		{0, CompassNorth}, {22.4, CompassNorth}, {22.5, CompassNortheast}, {45, CompassNortheast},
		{90, CompassEast}, {135, CompassSoutheast}, {180, CompassSouth}, {225, CompassSouthwest},
		{270, CompassWest}, {315, CompassNorthwest}, {337.4, CompassNorthwest}, {337.5, CompassNorth}, {359.9, CompassNorth},
	} {
		if got := compassFor(test.bearing); got != test.want {
			t.Errorf("compassFor(%g) = %q, want %q", test.bearing, got, test.want)
		}
	}
}

func TestExitsNumberClockwiseFromNorth(t *testing.T) {
	world, err := Generate(Config{WorldSeed: 42, ProvinceCount: 97, IslandCount: 7})
	if err != nil {
		t.Fatal(err)
	}
	boundaryExits, interiorExits := 0, 0
	for _, province := range world.Provinces {
		for _, exit := range province.Exits {
			if exit.NeighborID == NoProvinceID {
				boundaryExits++
				if province.IslandID != NoIslandID {
					t.Errorf("land province %d has boundary exit %d", province.ID, exit.Number)
				}
			} else {
				interiorExits++
			}
		}
		assertValidExits(t, world, province)
	}
	if boundaryExits == 0 || interiorExits == 0 {
		t.Errorf("exits: %d boundary, %d interior, want both", boundaryExits, interiorExits)
	}
}
