package main

import (
	"bytes"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdhender/wgvc"
)

func TestParseLayerAcceptsEveryLayerAndRejectsUnknown(t *testing.T) {
	for _, layer := range mapLayers {
		got, err := parseLayer(string(layer))
		if err != nil || got != layer {
			t.Errorf("parseLayer(%q) = %q, %v; want %q, nil", layer, got, err, layer)
		}
	}
	_, err := parseLayer("bogus")
	if err == nil {
		t.Fatal("parseLayer accepted an unknown layer")
	}
	for _, layer := range mapLayers {
		if !strings.Contains(err.Error(), string(layer)) {
			t.Errorf("error %q does not list layer %q", err, layer)
		}
	}
}

func TestRunRendersEveryLayer(t *testing.T) {
	for _, layer := range mapLayers {
		t.Run(string(layer), func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "map")
			var stderr bytes.Buffer
			args := []string{"-provinces", "40", "-islands", "2", "-format", "both", "-layer", string(layer), "-output", base}
			if err := run(args, io.Discard, &stderr); err != nil {
				t.Fatalf("run() error = %v; stderr = %s", err, stderr.String())
			}
			assertOutputExists(t, base+".svg", true)
			assertOutputExists(t, base+".png", true)
		})
	}
}

func TestRunRejectsUnknownLayer(t *testing.T) {
	base := filepath.Join(t.TempDir(), "map")
	err := run([]string{"-layer", "bogus", "-output", base}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "unsupported layer") {
		t.Fatalf("run() error = %v, want unsupported layer", err)
	}
	if matches, _ := filepath.Glob(base + ".*"); len(matches) != 0 {
		t.Fatalf("-layer bogus wrote files: %v", matches)
	}
}

func TestLayerChangesOnlyPolygonFill(t *testing.T) {
	world, err := wgvc.Generate(wgvc.Config{WorldSeed: 42, ProvinceCount: 53, IslandCount: 1})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	terrain, err := buildScene(world, 480, 360, layerTerrain)
	if err != nil {
		t.Fatalf("buildScene(terrain) error = %v", err)
	}
	for _, layer := range mapLayers[1:] {
		scene, err := buildScene(world, 480, 360, layer)
		if err != nil {
			t.Fatalf("buildScene(%s) error = %v", layer, err)
		}
		if !reflect.DeepEqual(scene.coastlines, terrain.coastlines) || scene.width != terrain.width || scene.height != terrain.height {
			t.Errorf("layer %s changed the scene outside polygon fills", layer)
		}
		for i := range scene.polygons {
			if !reflect.DeepEqual(scene.polygons[i].points, terrain.polygons[i].points) {
				t.Errorf("layer %s moved polygon %d", layer, i)
			}
		}
	}
}

func TestRampLayersUseEndColorsAtFieldExtremes(t *testing.T) {
	land := wgvc.Province{IslandID: 0}
	water := wgvc.Province{IslandID: wgvc.NoIslandID}
	with := func(p wgvc.Province, set func(*wgvc.Province)) wgvc.Province {
		set(&p)
		return p
	}
	first := func(r Ramp) string { return hexColor(r[0].Color) }
	last := func(r Ramp) string { return hexColor(r[len(r)-1].Color) }
	tests := []struct {
		name     string
		layer    mapLayer
		province wgvc.Province
		want     string
	}{
		{"elevation min", layerElevation, with(water, func(p *wgvc.Province) { p.Elevation = -1 }), first(ElevationRamp)},
		{"elevation max", layerElevation, with(land, func(p *wgvc.Province) { p.Elevation = 1 }), last(ElevationRamp)},
		{"water at sea level", layerElevation, water, hexColor(ElevationRamp[2].Color)},
		{"land at sea level", layerElevation, land, hexColor(ElevationRamp[3].Color)},
		{"relief min", layerRelief, with(land, func(p *wgvc.Province) { p.Relief = 0 }), first(UnitRamp)},
		{"relief display max", layerRelief, with(land, func(p *wgvc.Province) { p.Relief = reliefDisplayMax }), last(UnitRamp)},
		{"relief above display max saturates", layerRelief, with(land, func(p *wgvc.Province) { p.Relief = 1 }), last(UnitRamp)},
		{"relief at half the display range", layerRelief, with(land, func(p *wgvc.Province) { p.Relief = reliefDisplayMax / 2 }), hexColor(UnitRamp.At(0.5))},
		{"heat min", layerHeat, with(land, func(p *wgvc.Province) { p.Heat = 0 }), first(TemperatureRamp)},
		{"heat max", layerHeat, with(land, func(p *wgvc.Province) { p.Heat = 1 }), last(TemperatureRamp)},
		{"moisture min", layerMoisture, with(land, func(p *wgvc.Province) { p.Moisture = 0 }), first(MoistureRamp)},
		{"moisture max", layerMoisture, with(land, func(p *wgvc.Province) { p.Moisture = 1 }), last(MoistureRamp)},
	}
	for _, test := range tests {
		got, err := provinceFill(test.layer, test.province)
		if err != nil {
			t.Errorf("%s: provinceFill() error = %v", test.name, err)
			continue
		}
		if got != test.want {
			t.Errorf("%s: fill = %s, want %s", test.name, got, test.want)
		}
	}
	if ElevationRamp[2].At != elevationRampBreak || ElevationRamp[3].At != elevationRampLand {
		t.Fatal("ElevationRamp stops 2 and 3 are no longer the coastline step")
	}
}

func TestRampLayersRejectNonFiniteValues(t *testing.T) {
	for _, layer := range []mapLayer{layerElevation, layerRelief, layerHeat, layerMoisture} {
		province := wgvc.Province{Elevation: math.NaN(), Relief: math.NaN(), Heat: math.NaN(), Moisture: math.NaN()}
		if _, err := provinceFill(layer, province); err == nil {
			t.Errorf("layer %s accepted NaN", layer)
		}
	}
}

func TestClimateColorsCoverEveryBandPair(t *testing.T) {
	seen := map[string]bool{}
	for _, heat := range wgvc.HeatBands() {
		for _, moisture := range wgvc.MoistureBands() {
			fill, err := climateColor(heat, moisture)
			if err != nil {
				t.Errorf("climateColor(%v, %v) error = %v", heat, moisture, err)
				continue
			}
			if seen[fill] {
				t.Errorf("climateColor(%v, %v) = %s reuses another cell's color", heat, moisture, fill)
			}
			seen[fill] = true
		}
	}
	if _, err := climateColor(wgvc.HeatBand(len(wgvc.HeatBands())), wgvc.MoistureBandArid); err == nil {
		t.Error("climateColor accepted an undeclared heat band")
	}
	if _, err := climateColor(wgvc.HeatBandPolar, wgvc.MoistureBand(-1)); err == nil {
		t.Error("climateColor accepted an undeclared moisture band")
	}
}
