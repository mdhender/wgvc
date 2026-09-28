package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"strings"

	"github.com/fogleman/gg"
	"github.com/mdhender/wgvc"
)

const (
	minimumImageSize = 64
	imageMargin      = 24.0
	cellBorderWidth  = 1.0
	coastlineWidth   = 2.5
)

// Map colors are held as color values and formatted as hex only where the
// SVG needs text, so the PNG path never parses a color string.
var (
	backgroundColor = rgb(0xf7, 0xf5, 0xef)
	cellBorderColor = rgb(0x69, 0x78, 0x7b)
	coastlineColor  = rgb(0x24, 0x34, 0x3d)
	riverColor      = rgb(0x2a, 0x7f, 0xd4)
	// fogColor fills provinces outside a partial map's selection.
	fogColor = rgb(0xdc, 0xd8, 0xcf)
)

func rgb(r, g, b uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: 0xff}
}

// riverWidths is the stroke width of a river edge by the class of the flow on
// that edge, so a river thickens downstream. Streams stay thinner than the
// coastline; major rivers are the heaviest line on the map.
var riverWidths = map[wgvc.RiverClass]float64{
	wgvc.RiverClassStream:     1.5,
	wgvc.RiverClassRiver:      2.5,
	wgvc.RiverClassMajorRiver: 4.0,
}

var terrainColors = map[wgvc.Terrain]color.RGBA{
	wgvc.TerrainDeepOcean:        rgb(0x04, 0x14, 0x2b),
	wgvc.TerrainOcean:            rgb(0x0d, 0x3a, 0x6b),
	wgvc.TerrainShallowSea:       rgb(0x2f, 0x7f, 0xb5),
	wgvc.TerrainCoastalWater:     rgb(0x74, 0xb3, 0xd4),
	wgvc.TerrainInlandSea:        rgb(0x1f, 0x5e, 0x8c),
	wgvc.TerrainLake:             rgb(0x3d, 0x86, 0xb8),
	wgvc.TerrainGlacialIce:       rgb(0xee, 0xf4, 0xf8),
	wgvc.TerrainTundra:           rgb(0x9a, 0xa7, 0x9a),
	wgvc.TerrainMarsh:            rgb(0x5d, 0x7a, 0x52),
	wgvc.TerrainSwamp:            rgb(0x3f, 0x5c, 0x3a),
	wgvc.TerrainBog:              rgb(0x6b, 0x6f, 0x4e),
	wgvc.TerrainDesert:           rgb(0xd9, 0xc0, 0x7a),
	wgvc.TerrainBadlands:         rgb(0xb0, 0x7a, 0x4e),
	wgvc.TerrainScrubland:        rgb(0xa8, 0x9a, 0x5e),
	wgvc.TerrainPlains:           rgb(0xa7, 0xbd, 0x72),
	wgvc.TerrainGrassland:        rgb(0x8f, 0xb4, 0x5c),
	wgvc.TerrainSteppe:           rgb(0xb9, 0xb0, 0x71),
	wgvc.TerrainSavanna:          rgb(0xc9, 0xb4, 0x5a),
	wgvc.TerrainBorealForest:     rgb(0x2f, 0x57, 0x41),
	wgvc.TerrainTemperateForest:  rgb(0x3f, 0x7a, 0x3a),
	wgvc.TerrainRainforest:       rgb(0x1f, 0x5a, 0x2c),
	wgvc.TerrainHills:            rgb(0x8a, 0x82, 0x57),
	wgvc.TerrainPlateau:          rgb(0xb3, 0xa9, 0x8a),
	wgvc.TerrainMountain:         rgb(0x8a, 0x8a, 0x8a),
	wgvc.TerrainAlpine:           rgb(0xc7, 0xcc, 0xd1),
	wgvc.TerrainVolcano:          rgb(0x7a, 0x2a, 0x24),
	wgvc.TerrainVolcanicHighland: rgb(0x5c, 0x40, 0x38),
	wgvc.TerrainCoast:            rgb(0xd8, 0xcf, 0xa5),
}

type renderPoint struct {
	x float64
	y float64
}

type renderPolygon struct {
	points []renderPoint
	fill   color.RGBA
}

type renderLine struct {
	start renderPoint
	end   renderPoint
}

type renderRiverSegment struct {
	line  renderLine
	width float64
}

// renderScene is the resolved drawing: polygons carry their fill, borders
// list every edge of a drawn polygon exactly once so the raster path strokes
// each shared border a single time, and coastlines and rivers overlay them.
type renderScene struct {
	width      int
	height     int
	polygons   []renderPolygon
	borders    []renderLine
	coastlines []renderLine
	rivers     []renderRiverSegment
}

// buildScene lays out the whole world at the given image size. With a
// non-nil selected slice it instead produces a partial map: the image is
// cropped to the selected provinces plus the usual margin, at exactly the
// full map's scale and on its pixel grid, so partial maps of one world line
// up with each other and with the full map. Unselected provinces that fall
// inside the crop are drawn as fog with borders but no fill, coastline, or
// river.
func buildScene(world wgvc.World, width, height int, layer mapLayer, selected []bool) (renderScene, error) {
	if width < minimumImageSize || height < minimumImageSize {
		return renderScene{}, fmt.Errorf("width and height must each be at least %d pixels", minimumImageSize)
	}
	if len(world.Corners) == 0 || len(world.Provinces) == 0 {
		return renderScene{}, fmt.Errorf("world has no renderable geometry")
	}

	minimum := world.Corners[0].Point
	maximum := world.Corners[0].Point
	for _, corner := range world.Corners[1:] {
		minimum.X = math.Min(minimum.X, corner.Point.X)
		minimum.Y = math.Min(minimum.Y, corner.Point.Y)
		maximum.X = math.Max(maximum.X, corner.Point.X)
		maximum.Y = math.Max(maximum.Y, corner.Point.Y)
	}
	worldWidth := maximum.X - minimum.X
	worldHeight := maximum.Y - minimum.Y
	if worldWidth <= 0 || worldHeight <= 0 || math.IsNaN(worldWidth) || math.IsNaN(worldHeight) {
		return renderScene{}, fmt.Errorf("world bounds are not finite and positive")
	}
	scale := math.Min(
		(float64(width)-2*imageMargin)/worldWidth,
		(float64(height)-2*imageMargin)/worldHeight,
	)
	xOffset := (float64(width) - worldWidth*scale) / 2
	yOffset := (float64(height) - worldHeight*scale) / 2
	fullHeight := float64(height)
	fullTransform := func(point wgvc.Point) renderPoint {
		return renderPoint{
			x: xOffset + (point.X-minimum.X)*scale,
			y: fullHeight - yOffset - (point.Y-minimum.Y)*scale,
		}
	}
	transform := fullTransform
	isSelected := func(provinceID wgvc.ProvinceID) bool { return selected == nil || selected[provinceID] }
	if selected != nil {
		if len(selected) != len(world.Provinces) {
			return renderScene{}, fmt.Errorf("selection has %d entries for %d provinces", len(selected), len(world.Provinces))
		}
		cropMinX, cropMinY := math.Inf(1), math.Inf(1)
		cropMaxX, cropMaxY := math.Inf(-1), math.Inf(-1)
		for _, province := range world.Provinces {
			if !selected[province.ID] {
				continue
			}
			for _, cornerID := range province.CornerIDs {
				point := fullTransform(world.Corners[cornerID].Point)
				cropMinX, cropMinY = math.Min(cropMinX, point.x), math.Min(cropMinY, point.y)
				cropMaxX, cropMaxY = math.Max(cropMaxX, point.x), math.Max(cropMaxY, point.y)
			}
		}
		if math.IsInf(cropMinX, 1) {
			return renderScene{}, fmt.Errorf("selection contains no province")
		}
		originX := math.Floor(math.Max(0, cropMinX-imageMargin))
		originY := math.Floor(math.Max(0, cropMinY-imageMargin))
		width = int(math.Ceil(math.Min(float64(width), cropMaxX+imageMargin))) - int(originX)
		height = int(math.Ceil(math.Min(float64(height), cropMaxY+imageMargin))) - int(originY)
		transform = func(point wgvc.Point) renderPoint {
			full := fullTransform(point)
			return renderPoint{x: full.x - originX, y: full.y - originY}
		}
	}

	scene := renderScene{width: width, height: height, polygons: make([]renderPolygon, 0, len(world.Provinces))}
	drawn := make([]bool, len(world.Provinces))
	for _, province := range world.Provinces {
		if len(province.CornerIDs) < 3 {
			return renderScene{}, fmt.Errorf("province %d has fewer than three corners", province.ID)
		}
		polygon := renderPolygon{points: make([]renderPoint, len(province.CornerIDs))}
		inside := false
		for index, cornerID := range province.CornerIDs {
			if cornerID < 0 || int(cornerID) >= len(world.Corners) {
				return renderScene{}, fmt.Errorf("province %d references unknown corner %d", province.ID, cornerID)
			}
			polygon.points[index] = transform(world.Corners[cornerID].Point)
			if p := polygon.points[index]; p.x >= 0 && p.x <= float64(width) && p.y >= 0 && p.y <= float64(height) {
				inside = true
			}
		}
		if !isSelected(province.ID) {
			if !inside {
				continue
			}
			polygon.fill = fogColor
			scene.polygons = append(scene.polygons, polygon)
			drawn[province.ID] = true
			continue
		}
		fill, err := provinceFill(layer, province)
		if err != nil {
			return renderScene{}, fmt.Errorf("province %d: %w", province.ID, err)
		}
		polygon.fill = fill
		scene.polygons = append(scene.polygons, polygon)
		drawn[province.ID] = true
	}

	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) == 0 || len(edge.ProvinceIDs) > 2 {
			return renderScene{}, fmt.Errorf("edge %d has %d incident provinces", edge.ID, len(edge.ProvinceIDs))
		}
		bordersDrawn := false
		for _, provinceID := range edge.ProvinceIDs {
			if provinceID < 0 || int(provinceID) >= len(world.Provinces) {
				return renderScene{}, fmt.Errorf("edge %d references an unknown province", edge.ID)
			}
			bordersDrawn = bordersDrawn || drawn[provinceID]
		}
		if !bordersDrawn {
			continue
		}
		firstCorner, secondCorner := edge.CornerIDs[0], edge.CornerIDs[1]
		if firstCorner < 0 || int(firstCorner) >= len(world.Corners) || secondCorner < 0 || int(secondCorner) >= len(world.Corners) {
			return renderScene{}, fmt.Errorf("edge %d references an unknown corner", edge.ID)
		}
		line := renderLine{
			start: transform(world.Corners[firstCorner].Point),
			end:   transform(world.Corners[secondCorner].Point),
		}
		scene.borders = append(scene.borders, line)
		if len(edge.ProvinceIDs) != 2 {
			continue
		}
		firstProvince, secondProvince := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
		if !isSelected(firstProvince) && !isSelected(secondProvince) {
			continue
		}
		firstWater := world.Provinces[firstProvince].IslandID == wgvc.NoIslandID
		secondWater := world.Provinces[secondProvince].IslandID == wgvc.NoIslandID
		if firstWater == secondWater {
			continue
		}
		scene.coastlines = append(scene.coastlines, line)
	}
	for _, river := range world.Rivers {
		for _, edgeID := range river.EdgeIDs {
			if edgeID < 0 || int(edgeID) >= len(world.Edges) {
				return renderScene{}, fmt.Errorf("river %d references unknown edge %d", river.ID, edgeID)
			}
			edge := world.Edges[edgeID]
			if !isSelected(edge.ProvinceIDs[0]) && !isSelected(edge.ProvinceIDs[1]) {
				continue
			}
			width, ok := riverWidths[wgvc.ClassifyDischarge(edge.Discharge)]
			if !ok {
				return renderScene{}, fmt.Errorf("river %d edge %d has discharge %g below the stream threshold", river.ID, edgeID, edge.Discharge)
			}
			scene.rivers = append(scene.rivers, renderRiverSegment{
				width: width,
				line: renderLine{
					start: transform(world.Corners[edge.CornerIDs[0]].Point),
					end:   transform(world.Corners[edge.CornerIDs[1]].Point),
				},
			})
		}
	}
	return scene, nil
}

func terrainColor(terrain wgvc.Terrain) (color.RGBA, error) {
	fill, ok := terrainColors[terrain]
	if !ok {
		return color.RGBA{}, fmt.Errorf("unsupported terrain %q", terrain)
	}
	return fill, nil
}

func renderSVG(scene renderScene) ([]byte, error) {
	var svg strings.Builder
	fmt.Fprintf(&svg, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", scene.width, scene.height, scene.width, scene.height)
	fmt.Fprintf(&svg, "<rect width=\"%d\" height=\"%d\" fill=\"%s\"/>\n", scene.width, scene.height, hexColor(backgroundColor))
	borderHex := hexColor(cellBorderColor)
	for _, polygon := range scene.polygons {
		fmt.Fprintf(&svg, "<polygon fill=\"%s\" stroke=\"%s\" stroke-width=\"%.1f\" stroke-linejoin=\"round\" points=\"", hexColor(polygon.fill), borderHex, cellBorderWidth)
		for _, point := range polygon.points {
			fmt.Fprintf(&svg, "%.3f,%.3f ", point.x, point.y)
		}
		svg.WriteString("\"/>\n")
	}
	for _, coastline := range scene.coastlines {
		fmt.Fprintf(&svg, "<line stroke=\"%s\" stroke-width=\"%.1f\" stroke-linecap=\"round\" x1=\"%.3f\" y1=\"%.3f\" x2=\"%.3f\" y2=\"%.3f\"/>\n", hexColor(coastlineColor), coastlineWidth, coastline.start.x, coastline.start.y, coastline.end.x, coastline.end.y)
	}
	for _, river := range scene.rivers {
		fmt.Fprintf(&svg, "<line class=\"river\" stroke=\"%s\" stroke-width=\"%.1f\" stroke-linecap=\"round\" x1=\"%.3f\" y1=\"%.3f\" x2=\"%.3f\" y2=\"%.3f\"/>\n", hexColor(riverColor), river.width, river.line.start.x, river.line.start.y, river.line.end.x, river.line.end.y)
	}
	svg.WriteString("</svg>\n")
	return []byte(svg.String()), nil
}

// renderPNG rasterises the scene. Polygons are filled without a stroke and
// every border is then stroked once from scene.borders, so a border shared by
// two provinces is composited a single time and reads the same weight as a
// border on the map edge. The encoder favours speed: the map is flat colour,
// so the fastest deflate level costs a few percent in size.
func renderPNG(scene renderScene) ([]byte, error) {
	canvas := gg.NewContext(scene.width, scene.height)
	canvas.SetColor(backgroundColor)
	canvas.Clear()
	canvas.SetLineJoin(gg.LineJoinRound)
	canvas.SetLineCap(gg.LineCapRound)
	for _, polygon := range scene.polygons {
		tracePolygon(canvas, polygon.points)
		canvas.SetColor(polygon.fill)
		canvas.Fill()
	}
	canvas.SetColor(cellBorderColor)
	canvas.SetLineWidth(cellBorderWidth)
	for _, border := range scene.borders {
		canvas.DrawLine(border.start.x, border.start.y, border.end.x, border.end.y)
		canvas.Stroke()
	}
	canvas.SetColor(coastlineColor)
	canvas.SetLineWidth(coastlineWidth)
	for _, coastline := range scene.coastlines {
		canvas.DrawLine(coastline.start.x, coastline.start.y, coastline.end.x, coastline.end.y)
		canvas.Stroke()
	}
	canvas.SetColor(riverColor)
	for _, river := range scene.rivers {
		canvas.SetLineWidth(river.width)
		canvas.DrawLine(river.line.start.x, river.line.start.y, river.line.end.x, river.line.end.y)
		canvas.Stroke()
	}

	var output bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&output, canvas.Image()); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func tracePolygon(canvas *gg.Context, points []renderPoint) {
	canvas.MoveTo(points[0].x, points[0].y)
	for _, point := range points[1:] {
		canvas.LineTo(point.x, point.y)
	}
	canvas.ClosePath()
}
