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

func TestRunOutputFormats(t *testing.T) {
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
			args := append([]string{"-seed", "42", "-provinces", "8", "-islands", "2", "-width", "240", "-height", "180", "-output", base}, test.formatArg...)
			if err := run(args, io.Discard); err != nil {
				t.Fatalf("run() error = %v", err)
			}
			assertOutputExists(t, base+".svg", test.wantSVG)
			assertOutputExists(t, base+".png", test.wantPNG)
		})
	}
}

func TestDefaultAndExplicitSVGMatch(t *testing.T) {
	directory := t.TempDir()
	common := []string{"-seed", "7", "-provinces", "4", "-islands", "4", "-width", "180", "-height", "180"}
	if err := run(append(common, "-output", filepath.Join(directory, "default")), io.Discard); err != nil {
		t.Fatalf("default run() error = %v", err)
	}
	if err := run(append(common, "-format", "svg", "-output", filepath.Join(directory, "explicit")), io.Discard); err != nil {
		t.Fatalf("explicit run() error = %v", err)
	}
	defaultSVG, err := os.ReadFile(filepath.Join(directory, "default.svg"))
	if err != nil {
		t.Fatal(err)
	}
	explicitSVG, err := os.ReadFile(filepath.Join(directory, "explicit.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(defaultSVG, explicitSVG) {
		t.Fatal("default and explicit SVG output differ")
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

func TestSVGUsesInlineCellBorders(t *testing.T) {
	world, err := wgvc.Generate(wgvc.Config{WorldSeed: 1, ProvinceCount: 3, IslandCount: 1})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	scene, err := buildScene(world, 240, 180)
	if err != nil {
		t.Fatalf("buildScene() error = %v", err)
	}
	data, err := renderSVG(scene)
	if err != nil {
		t.Fatalf("renderSVG() error = %v", err)
	}
	if count := strings.Count(string(data), `stroke="`+cellBorderColor+`"`); count != len(world.Provinces) {
		t.Fatalf("SVG cell-border count = %d, want %d", count, len(world.Provinces))
	}
}

func TestRunRejectsUnknownFormat(t *testing.T) {
	err := run([]string{"-format", "jpeg"}, io.Discard)
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
