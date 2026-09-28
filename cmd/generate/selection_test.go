package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdhender/wgvc"
)

func TestParseSelectionValidatesFlags(t *testing.T) {
	for _, test := range []struct {
		region string
		ids    string
		radius int
		wantOK bool
	}{
		{"", "", 0, true},
		{"0,0,10,10", "", 0, true},
		{"", "1,2,3", 2, true},
		{"0,0,10", "", 0, false},
		{"10,0,0,10", "", 0, false},
		{"a,b,c,d", "", 0, false},
		{"", "x", 0, false},
		{"", "-1", 0, false},
		{"", "", 1, false},
		{"", "1", -1, false},
	} {
		_, err := parseSelection(test.region, test.ids, test.radius)
		if (err == nil) != test.wantOK {
			t.Errorf("parseSelection(%q, %q, %d) error = %v, want ok=%t", test.region, test.ids, test.radius, err, test.wantOK)
		}
	}
}

func TestSelectionAppliesRegionIDsAndRadius(t *testing.T) {
	world, err := wgvc.Generate(wgvc.Config{WorldSeed: 42, ProvinceCount: 97, IslandCount: 7})
	if err != nil {
		t.Fatal(err)
	}
	whole, err := selection{}.apply(world)
	if err != nil || whole != nil {
		t.Fatalf("empty selection = %v, %v; want nil, nil", whole, err)
	}
	if _, err := (selection{ids: []wgvc.ProvinceID{wgvc.ProvinceID(len(world.Provinces))}}).apply(world); err == nil {
		t.Error("out-of-range province ID was accepted")
	}
	center := world.Provinces[0].Center
	sel, err := parseSelection(fmt.Sprintf("%g,%g,%g,%g", center.X-1e-9, center.Y-1e-9, center.X+1e-9, center.Y+1e-9), "5", 1)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := sel.apply(world)
	if err != nil {
		t.Fatal(err)
	}
	want := map[wgvc.ProvinceID]bool{0: true, 5: true}
	for _, id := range []wgvc.ProvinceID{0, 5} {
		for _, exit := range world.Provinces[id].Exits {
			if exit.NeighborID != wgvc.NoProvinceID {
				want[exit.NeighborID] = true
			}
		}
	}
	for id, in := range selected {
		if in != want[wgvc.ProvinceID(id)] {
			t.Errorf("province %d selected = %t, want %t", id, in, want[wgvc.ProvinceID(id)])
		}
	}
}

// TestPartialMapKeepsFullMapScaleAndFogsTheRest checks that a partial map is
// a crop of the full map: every selected polygon appears at the full map's
// coordinates shifted by one integer offset, unselected polygons inside the
// crop are fog, and coastlines and rivers touching no selected province are
// dropped.
func TestPartialMapKeepsFullMapScaleAndFogsTheRest(t *testing.T) {
	world, err := wgvc.Generate(wgvc.Config{WorldSeed: 42, ProvinceCount: 97, IslandCount: 7})
	if err != nil {
		t.Fatal(err)
	}
	width, height, err := renderDimensions(97, 0.68, "")
	if err != nil {
		t.Fatal(err)
	}
	full, err := buildScene(world, width, height, layerTerrain, nil)
	if err != nil {
		t.Fatal(err)
	}
	selected := make([]bool, len(world.Provinces))
	selected[world.Islands[0].ProvinceIDs[0]] = true
	for _, exit := range world.Provinces[world.Islands[0].ProvinceIDs[0]].Exits {
		if exit.NeighborID != wgvc.NoProvinceID {
			selected[exit.NeighborID] = true
		}
	}
	partial, err := buildScene(world, width, height, layerTerrain, selected)
	if err != nil {
		t.Fatal(err)
	}
	if partial.width >= full.width || partial.height >= full.height || partial.width < 1 || partial.height < 1 {
		t.Fatalf("partial map is %d×%d, full map is %d×%d", partial.width, partial.height, full.width, full.height)
	}
	fullByFirstPoint := map[[2]float64]renderPolygon{}
	for _, polygon := range full.polygons {
		fullByFirstPoint[[2]float64{polygon.points[0].x, polygon.points[0].y}] = polygon
	}
	var offsetX, offsetY float64
	seenOffset := false
	fog, filled := 0, 0
	for _, polygon := range partial.polygons {
		if polygon.fill == fogColor {
			fog++
			continue
		}
		filled++
		matched := false
		for key, fullPolygon := range fullByFirstPoint {
			dx, dy := key[0]-polygon.points[0].x, key[1]-polygon.points[0].y
			if len(fullPolygon.points) != len(polygon.points) || fullPolygon.fill != polygon.fill {
				continue
			}
			ok := true
			for i := range polygon.points {
				if fullPolygon.points[i].x-polygon.points[i].x != dx || fullPolygon.points[i].y-polygon.points[i].y != dy {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			matched = true
			if !seenOffset {
				offsetX, offsetY, seenOffset = dx, dy, true
			} else if dx != offsetX || dy != offsetY {
				t.Errorf("polygon offset (%g, %g) differs from (%g, %g): scale or grid changed", dx, dy, offsetX, offsetY)
			}
			break
		}
		if !matched {
			t.Errorf("partial polygon with fill %s has no full-map counterpart at the same scale", hexColor(polygon.fill))
		}
	}
	selectedCount := 0
	for _, in := range selected {
		if in {
			selectedCount++
		}
	}
	if filled != selectedCount || fog == 0 {
		t.Errorf("partial map has %d filled and %d fog polygons, want %d filled and some fog", filled, fog, selectedCount)
	}
	if offsetX != float64(int(offsetX)) || offsetY != float64(int(offsetY)) {
		t.Errorf("crop offset (%g, %g) is not on the full map's pixel grid", offsetX, offsetY)
	}
	if len(partial.coastlines) >= len(full.coastlines) || len(partial.rivers) > len(full.rivers) {
		t.Errorf("partial map keeps %d coastlines and %d rivers of %d and %d", len(partial.coastlines), len(partial.rivers), len(full.coastlines), len(full.rivers))
	}
}

func TestRunRendersPartialMap(t *testing.T) {
	base := filepath.Join(t.TempDir(), "part")
	var stdout bytes.Buffer
	if err := run([]string{"-provinces", "60", "-islands", "3", "-select", "0", "-radius", "2", "-format", "svg", "-output", base}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	data, err := os.ReadFile(base + ".svg")
	if err != nil {
		t.Fatal(err)
	}
	var width, height int
	if _, err := fmt.Sscanf(string(data), `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"`, &width, &height); err != nil {
		t.Fatalf("parse SVG dimensions: %v", err)
	}
	fullWidth, fullHeight, err := renderDimensions(60, 0.68, "")
	if err != nil {
		t.Fatal(err)
	}
	fogHex := hexColor(fogColor)
	if width >= fullWidth || height >= fullHeight || !strings.Contains(string(data), `fill="`+fogHex+`"`) {
		t.Errorf("partial SVG is %d×%d (full %d×%d) and fog present = %t", width, height, fullWidth, fullHeight, strings.Contains(string(data), fogHex))
	}
	if !strings.Contains(stdout.String(), fmt.Sprintf("(%d×%d)", width, height)) {
		t.Errorf("status line %q does not report the partial dimensions", stdout.String())
	}
	if err := run([]string{"-provinces", "60", "-islands", "3", "-select", "999", "-output", base}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Error("run() accepted a province ID outside the world")
	}
}
