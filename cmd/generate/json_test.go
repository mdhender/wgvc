package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mdhender/wgvc"
)

func TestRunJSONContainsCanonicalWorldData(t *testing.T) {
	base := filepath.Join(t.TempDir(), "map")
	err := run([]string{
		"-seed", "0xffffffffffffffff",
		"-provinces", "30",
		"-islands", "3",
		"-ocean", "0.5",
		"-aspect", "16:9",
		"-format", "json",
		"-output", base,
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	data, err := os.ReadFile(base + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var document jsonWorld
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if document.SchemaVersion != jsonSchemaVersion {
		t.Errorf("schema version = %d, want %d", document.SchemaVersion, jsonSchemaVersion)
	}
	if document.Generation.Config.Seed != "0xffffffffffffffff" {
		t.Errorf("seed = %q, want full-width hexadecimal seed", document.Generation.Config.Seed)
	}
	if document.Generation.Config.AspectRatio != "16:9" {
		t.Errorf("aspect ratio = %q, want 16:9", document.Generation.Config.AspectRatio)
	}
	if document.Generation.Result.LandProvinceCount != 30 {
		t.Errorf("land province count = %d, want 30", document.Generation.Result.LandProvinceCount)
	}
	if document.Generation.Result.ProvinceCount != len(document.Provinces) {
		t.Errorf("result province count = %d, JSON contains %d", document.Generation.Result.ProvinceCount, len(document.Provinces))
	}
	if document.Bounds.Minimum.X >= document.Bounds.Maximum.X || document.Bounds.Minimum.Y >= document.Bounds.Maximum.Y {
		t.Errorf("bounds = %+v, want positive width and height", document.Bounds)
	}

	foundWater := false
	elevationBands := map[string]bool{"deep-water": true, "shallow-water": true, "lowland": true, "upland": true, "highland": true, "mountain": true}
	heatBands := map[string]bool{"polar": true, "cold": true, "temperate": true, "warm": true, "hot": true}
	moistureBands := map[string]bool{"arid": true, "dry": true, "moderate": true, "humid": true, "saturated": true}
	for index, island := range document.Islands {
		if island.ID != wgvc.IslandID(index) {
			t.Errorf("island %d ID = %d", index, island.ID)
		}
		for _, provinceID := range island.ProvinceIDs {
			if provinceID < 0 || int(provinceID) >= len(document.Provinces) {
				t.Errorf("island %d references unknown province %d", island.ID, provinceID)
			}
		}
	}
	for index, province := range document.Provinces {
		if province.ID != wgvc.ProvinceID(index) {
			t.Errorf("province %d ID = %d", index, province.ID)
		}
		if province.IslandID == wgvc.NoIslandID {
			foundWater = true
		}
		if !province.Terrain.Valid() {
			t.Errorf("province %d has invalid terrain %q", province.ID, province.Terrain)
		}
		if !elevationBands[province.ElevationBand] || !heatBands[province.HeatBand] || !moistureBands[province.MoistureBand] {
			t.Errorf("province %d has invalid classification names: %+v", province.ID, province)
		}
		for _, cornerID := range province.CornerIDs {
			if cornerID < 0 || int(cornerID) >= len(document.Corners) {
				t.Errorf("province %d references unknown corner %d", province.ID, cornerID)
			}
		}
	}
	if !foundWater {
		t.Fatal("JSON contains no water province with island_id -1")
	}
	for index, corner := range document.Corners {
		if corner.ID != wgvc.CornerID(index) {
			t.Errorf("corner %d ID = %d", index, corner.ID)
		}
	}
	for index, edge := range document.Edges {
		if edge.ID != wgvc.EdgeID(index) {
			t.Errorf("edge %d ID = %d", index, edge.ID)
		}
		for _, cornerID := range edge.CornerIDs {
			if cornerID < 0 || int(cornerID) >= len(document.Corners) {
				t.Errorf("edge %d references unknown corner %d", edge.ID, cornerID)
			}
		}
		for _, provinceID := range edge.ProvinceIDs {
			if provinceID < 0 || int(provinceID) >= len(document.Provinces) {
				t.Errorf("edge %d references unknown province %d", edge.ID, provinceID)
			}
		}
	}
}

func TestRunJSONIsDeterministic(t *testing.T) {
	directory := t.TempDir()
	firstBase := filepath.Join(directory, "first")
	secondBase := filepath.Join(directory, "second")
	commonArgs := []string{
		"-seed", "42",
		"-provinces", "30",
		"-islands", "3",
		"-format", "json",
	}
	if err := run(append(append([]string{}, commonArgs...), "-output", firstBase), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("first run() error = %v", err)
	}
	if err := run(append(append([]string{}, commonArgs...), "-output", secondBase), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("second run() error = %v", err)
	}
	first, err := os.ReadFile(firstBase + ".json")
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(secondBase + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("repeated JSON exports differ")
	}
	if len(first) == 0 || first[len(first)-1] != '\n' {
		t.Fatal("JSON export does not end with a newline")
	}
}
