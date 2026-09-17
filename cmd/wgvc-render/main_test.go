package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesGrowthSVGWithAllOptions(t *testing.T) {
	output := filepath.Join(t.TempDir(), "map.svg")
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-seed", "0x0123456789abcdef",
		"-provinces", "100",
		"-islands", "3",
		"-aspect", "16:9",
		"-ocean", "0.5",
		"-attractors", "5",
		"-edge-barrier", "0.03",
		"-edge-ramp=-1,0",
		"-attractant-ramp=1,0",
		"-attractant-jitter", "0.5",
		"-temperature", "0.3",
		"-control-penalty", "-0.7",
		"-rounds", "5",
		"-relaxations", "1",
		"-output", output,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v; stderr = %s", err, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.HasPrefix(string(data), "<svg ") {
		t.Fatalf("output does not start with SVG element: %.40q", data)
	}
	if !strings.Contains(string(data), `class="attractant"`) {
		t.Fatal("output does not contain attractants")
	}
	if !strings.Contains(stdout.String(), "100 land, 3→") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRenderDimensionsComeFromProvinceCountOceanAndAspect(t *testing.T) {
	tests := []struct {
		name                  string
		provinces             int
		ocean                 float64
		aspect                string
		wantWidth, wantHeight int
	}{
		{name: "square", provinces: 100, ocean: 0.75, aspect: "1:1", wantWidth: 360, wantHeight: 404},
		{name: "widescreen", provinces: 100, ocean: 0.75, aspect: "16:9", wantWidth: 467, wantHeight: 324},
		{name: "more land needs fewer cells", provinces: 100, ocean: 0.5, aspect: "1:1", wantWidth: 267, wantHeight: 311},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			width, height, err := renderDimensions(test.provinces, test.ocean, test.aspect)
			if err != nil {
				t.Fatalf("renderDimensions() error = %v", err)
			}
			if width != test.wantWidth || height != test.wantHeight {
				t.Errorf("renderDimensions() = %d×%d, want %d×%d", width, height, test.wantWidth, test.wantHeight)
			}
		})
	}
}

func TestRunSVGUsesDerivedDimensions(t *testing.T) {
	output := filepath.Join(t.TempDir(), "map.svg")
	if err := run([]string{"-provinces", "30", "-islands", "3", "-ocean", "0.5", "-aspect", "16:9", "-output", output}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var width, height int
	if _, err := fmt.Sscanf(string(data), `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"`, &width, &height); err != nil {
		t.Fatalf("parse SVG dimensions: %v", err)
	}
	wantWidth, wantHeight, err := renderDimensions(30, 0.5, "16:9")
	if err != nil {
		t.Fatal(err)
	}
	if width != wantWidth || height != wantHeight {
		t.Errorf("SVG dimensions = %d×%d, want %d×%d", width, height, wantWidth, wantHeight)
	}
}

func TestRunLogsSkippedAttractantRegions(t *testing.T) {
	output := filepath.Join(t.TempDir(), "map.svg")
	var stdout, stderr bytes.Buffer
	longRamp := strings.Repeat("1,", 20) + "0"
	err := run([]string{
		"-provinces", "30",
		"-islands", "3",
		"-attractors", "9",
		"-attractant-ramp=" + longRamp,
		"-output", output,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v; stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "skipped attractant region") || !strings.Contains(stderr.String(), "hops from the edge barrier") {
		t.Errorf("stderr = %q, want skipped-region reason", stderr.String())
	}
}

func TestRunRejectsUnsupportedAttractorCount(t *testing.T) {
	err := run([]string{"-attractors", "7"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "attractant count must be one of") {
		t.Fatalf("run() error = %v, want unsupported attractor count", err)
	}
}
