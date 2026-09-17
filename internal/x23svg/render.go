// Package x23svg renders single-mesh growth calibration results without
// coupling the generator to a particular command.
package x23svg

import (
	"fmt"
	"strings"

	"github.com/mdhender/wgvc/internal/x23"
)

var islandColors = []string{
	"#d58a62", "#d3ad57", "#9eb65d", "#60aa78", "#55a9a4",
	"#5b9fc6", "#798fd0", "#9a83c7", "#bc7eb1", "#cf7f8d",
	"#a99a62", "#79ad64", "#58a98f", "#6599c9", "#a383c3",
}

func Render(result x23.Result, width, height int) ([]byte, error) {
	if width < 64 || height < 64 {
		return nil, fmt.Errorf("width and height must each be at least 64 pixels")
	}
	if len(result.Cells) == 0 {
		return nil, fmt.Errorf("result has no cells")
	}

	const margin = 20.0
	mapHeight := float64(height) - 44
	scale := min((float64(width) - 2*margin), mapHeight-2*margin)
	xOffset := (float64(width) - scale) / 2
	yOffset := 44 + (mapHeight-scale)/2
	transform := func(point x23.Point) (float64, float64) {
		return xOffset + point.X*scale, yOffset + (1-point.Y)*scale
	}

	var svg strings.Builder
	fmt.Fprintf(&svg, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", width, height, width, height)
	svg.WriteString("<rect width=\"100%\" height=\"100%\" fill=\"#f7f5ef\"/>\n")
	fmt.Fprintf(&svg, "<text x=\"20\" y=\"27\" font-family=\"ui-monospace,monospace\" font-size=\"15\" font-weight=\"700\" fill=\"#17212b\">issue #23 · %d land · %d islands · %.0f%% ocean · round %d</text>\n", landCount(result), len(result.Islands), result.FinalOcean*100, result.RoundsAttempted)
	for _, cell := range result.Cells {
		fill := "#c6e3ec"
		if cell.IslandID != x23.Water {
			fill = islandColors[cell.IslandID%len(islandColors)]
		}
		fmt.Fprintf(&svg, "<polygon fill=\"%s\" stroke=\"#60777e\" stroke-width=\"0.7\" stroke-linejoin=\"round\" points=\"", fill)
		for _, point := range cell.Corners {
			x, y := transform(point)
			fmt.Fprintf(&svg, "%.2f,%.2f ", x, y)
		}
		svg.WriteString("\"/>\n")
	}
	for _, cell := range result.Cells {
		if cell.IslandID == x23.Water {
			continue
		}
		for _, neighborID := range cell.Neighbors {
			if result.Cells[neighborID].IslandID != x23.Water {
				continue
			}
			shared := sharedCorners(cell.Corners, result.Cells[neighborID].Corners)
			if len(shared) != 2 {
				continue
			}
			x1, y1 := transform(shared[0])
			x2, y2 := transform(shared[1])
			fmt.Fprintf(&svg, "<line stroke=\"#24343d\" stroke-width=\"1.35\" stroke-linecap=\"round\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\"/>\n", x1, y1, x2, y2)
		}
	}
	svg.WriteString("</svg>\n")
	return []byte(svg.String()), nil
}

func landCount(result x23.Result) int {
	count := 0
	for _, island := range result.Islands {
		count += len(island.CellIDs)
	}
	return count
}

func sharedCorners(first, second []x23.Point) []x23.Point {
	shared := make([]x23.Point, 0, 2)
	for _, a := range first {
		for _, b := range second {
			if a == b {
				shared = append(shared, a)
				break
			}
		}
	}
	return shared
}
