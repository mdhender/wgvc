package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/mdhender/wgvc"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	legendColumns     = 3
	legendColumnWidth = 300
	legendRowHeight   = 44
	legendMargin      = 24
	legendTitleHeight = 40
	legendSwatchWidth = 56
	legendSwatchInset = 8
	legendFontSize    = 16
	legendTitleSize   = 20
)

func TestTerrainLegend(t *testing.T) {
	got := renderTerrainLegend(t)
	path := filepath.Join("..", "..", "docs", "references", "terrain-legend.png")
	if os.Getenv("WGVC_UPDATE_TERRAIN_LEGEND") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read retained legend %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("retained legend is stale; reproduce with WGVC_UPDATE_TERRAIN_LEGEND=1 go test -run TestTerrainLegend ./cmd/generate")
	}
}

// renderTerrainLegend draws every terrain in declared order, column-major, as a
// swatch in its map fill color followed by its name and hex value.
func renderTerrainLegend(t *testing.T) []byte {
	t.Helper()
	terrains := wgvc.Terrains()
	rows := (len(terrains) + legendColumns - 1) / legendColumns
	width := 2*legendMargin + legendColumns*legendColumnWidth
	height := 2*legendMargin + legendTitleHeight + rows*legendRowHeight

	font, err := truetype.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse font: %v", err)
	}
	canvas := gg.NewContext(width, height)
	canvas.SetHexColor(backgroundColor)
	canvas.Clear()

	canvas.SetFontFace(truetype.NewFace(font, &truetype.Options{Size: legendTitleSize}))
	canvas.SetHexColor(coastlineColor)
	canvas.DrawStringAnchored("wgvc terrain colors", legendMargin, legendMargin+legendTitleHeight/2, 0, 0.35)

	canvas.SetFontFace(truetype.NewFace(font, &truetype.Options{Size: legendFontSize}))
	for i, terrain := range terrains {
		fill, err := terrainColor(terrain)
		if err != nil {
			t.Fatalf("terrain %q: %v", terrain, err)
		}
		x := float64(legendMargin + (i/rows)*legendColumnWidth)
		y := float64(legendMargin + legendTitleHeight + (i%rows)*legendRowHeight)
		canvas.DrawRectangle(x, y+legendSwatchInset, legendSwatchWidth, legendRowHeight-2*legendSwatchInset)
		canvas.SetHexColor(fill)
		canvas.FillPreserve()
		canvas.SetHexColor(cellBorderColor)
		canvas.SetLineWidth(cellBorderWidth)
		canvas.Stroke()

		canvas.SetHexColor(coastlineColor)
		canvas.DrawStringAnchored(string(terrain), x+legendSwatchWidth+12, y+legendRowHeight/2, 0, 0.35)
		canvas.SetHexColor(cellBorderColor)
		canvas.DrawStringAnchored(fill, x+legendColumnWidth-legendMargin, y+legendRowHeight/2, 1, 0.35)
	}

	var png bytes.Buffer
	if err := canvas.EncodePNG(&png); err != nil {
		t.Fatalf("encode legend: %v", err)
	}
	return png.Bytes()
}
