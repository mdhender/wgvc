package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdhender/wgvc"
)

func TestRunOutputFormatsWithGrowthOptions(t *testing.T) {
	tests := []struct {
		name      string
		formatArg []string
		wantSVG   bool
		wantPNG   bool
	}{
		{name: "default is SVG", wantSVG: true},
		{name: "explicit SVG", formatArg: []string{"-format", "svg"}, wantSVG: true},
		{name: "PNG", formatArg: []string{"-format", "png"}, wantPNG: true},
		{name: "both", formatArg: []string{"-format", "both"}, wantSVG: true, wantPNG: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "map")
			args := append([]string{
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
				"-output", base,
			}, test.formatArg...)
			var stdout, stderr bytes.Buffer
			if err := run(args, &stdout, &stderr); err != nil {
				t.Fatalf("run() error = %v; stderr = %s", err, stderr.String())
			}
			assertOutputExists(t, base+".svg", test.wantSVG)
			assertOutputExists(t, base+".png", test.wantPNG)
			if !strings.Contains(stdout.String(), "200 cells, 100 land") {
				t.Errorf("stdout = %q, want configured 50%% ocean cell budget", stdout.String())
			}
		})
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
		{name: "square", provinces: 100, ocean: 0.75, aspect: "1:1", wantWidth: 368, wantHeight: 368},
		{name: "widescreen", provinces: 100, ocean: 0.75, aspect: "16:9", wantWidth: 475, wantHeight: 288},
		{name: "more land needs fewer cells", provinces: 100, ocean: 0.5, aspect: "1:1", wantWidth: 275, wantHeight: 275},
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

func TestRunSVGUsesDerivedDimensionsAndTerrainColors(t *testing.T) {
	base := filepath.Join(t.TempDir(), "map")
	if err := run([]string{"-provinces", "30", "-islands", "3", "-ocean", "0.5", "-aspect", "16:9", "-output", base}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
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
	wantWidth, wantHeight, err := renderDimensions(30, 0.5, "16:9")
	if err != nil {
		t.Fatal(err)
	}
	if width != wantWidth || height != wantHeight {
		t.Errorf("SVG dimensions = %d×%d, want %d×%d", width, height, wantWidth, wantHeight)
	}
	terrainFills := strings.Count(string(data), `fill="`+plainsColor+`"`) +
		strings.Count(string(data), `fill="`+hillsColor+`"`) +
		strings.Count(string(data), `fill="`+mountainsColor+`"`)
	if terrainFills != 30 {
		t.Errorf("SVG terrain-filled land cells = %d, want 30", terrainFills)
	}
}

func TestRenderedPNGIsDeterministicAndKeepsCellBorders(t *testing.T) {
	world, err := wgvc.Generate(wgvc.Config{WorldSeed: 42, ProvinceCount: 53, IslandCount: 1})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	scene, err := buildScene(world, 480, 360)
	if err != nil {
		t.Fatalf("buildScene() error = %v", err)
	}
	first, err := renderPNG(scene)
	if err != nil {
		t.Fatalf("renderPNG() error = %v", err)
	}
	second, err := renderPNG(scene)
	if err != nil {
		t.Fatalf("second renderPNG() error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("repeated PNG renders differ")
	}
	image, err := png.Decode(bytes.NewReader(first))
	if err != nil {
		t.Fatalf("decode rendered PNG: %v", err)
	}
	if got, want := image.Bounds().Dx(), 480; got != want {
		t.Errorf("PNG width = %d, want %d", got, want)
	}
	if got, want := image.Bounds().Dy(), 360; got != want {
		t.Errorf("PNG height = %d, want %d", got, want)
	}

	border := parseHexColor(t, cellBorderColor)
	borderPixels := 0
	for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
		for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
			pixel := color.NRGBAModel.Convert(image.At(x, y)).(color.NRGBA)
			if colorsNear(pixel, border, 8) {
				borderPixels++
			}
		}
	}
	if borderPixels < 100 {
		t.Fatalf("PNG contains only %d cell-border pixels, want at least 100", borderPixels)
	}
}

func TestRunLogsSkippedAttractantRegions(t *testing.T) {
	base := filepath.Join(t.TempDir(), "map")
	var stdout, stderr bytes.Buffer
	longRamp := strings.Repeat("1,", 20) + "0"
	err := run([]string{
		"-provinces", "30",
		"-islands", "3",
		"-attractors", "9",
		"-attractant-ramp=" + longRamp,
		"-output", base,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v; stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "skipped attractant region") || !strings.Contains(stderr.String(), "hops from the edge barrier") {
		t.Errorf("stderr = %q, want skipped-region reason", stderr.String())
	}
}

func TestRunRejectsUnknownFormat(t *testing.T) {
	err := run([]string{"-format", "jpeg"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("run() error = %v, want unsupported format", err)
	}
}

func assertOutputExists(t *testing.T, path string, want bool) {
	t.Helper()
	_, err := os.Stat(path)
	if want && err != nil {
		t.Errorf("expected output %s: %v", path, err)
	}
	if !want && !os.IsNotExist(err) {
		t.Errorf("unexpected output %s", path)
	}
}

func parseHexColor(t *testing.T, value string) color.NRGBA {
	t.Helper()
	var result color.NRGBA
	if _, err := fmt.Sscanf(value, "#%02x%02x%02x", &result.R, &result.G, &result.B); err != nil {
		t.Fatalf("parse color %q: %v", value, err)
	}
	result.A = 255
	return result
}

func colorsNear(first, second color.NRGBA, tolerance uint8) bool {
	return channelNear(first.R, second.R, tolerance) && channelNear(first.G, second.G, tolerance) && channelNear(first.B, second.B, tolerance)
}

func channelNear(first, second, tolerance uint8) bool {
	difference := int(first) - int(second)
	if difference < 0 {
		difference = -difference
	}
	return difference <= int(tolerance)
}
