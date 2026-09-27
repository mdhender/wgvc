package main

import (
	"bytes"
	"fmt"
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

	backgroundColor = "#f7f5ef"
	cellBorderColor = "#69787b"
	coastlineColor  = "#24343d"
	riverColor      = "#2a7fd4"
)

// riverWidths is the stroke width of a river edge by the class of the flow on
// that edge, so a river thickens downstream. Streams stay thinner than the
// coastline; major rivers are the heaviest line on the map.
var riverWidths = map[wgvc.RiverClass]float64{
	wgvc.RiverClassStream:     1.5,
	wgvc.RiverClassRiver:      2.5,
	wgvc.RiverClassMajorRiver: 4.0,
}

var terrainColors = map[wgvc.Terrain]string{
	wgvc.TerrainDeepOcean:        "#04142b",
	wgvc.TerrainOcean:            "#0d3a6b",
	wgvc.TerrainShallowSea:       "#2f7fb5",
	wgvc.TerrainCoastalWater:     "#74b3d4",
	wgvc.TerrainInlandSea:        "#1f5e8c",
	wgvc.TerrainLake:             "#3d86b8",
	wgvc.TerrainGlacialIce:       "#eef4f8",
	wgvc.TerrainTundra:           "#9aa79a",
	wgvc.TerrainMarsh:            "#5d7a52",
	wgvc.TerrainSwamp:            "#3f5c3a",
	wgvc.TerrainBog:              "#6b6f4e",
	wgvc.TerrainDesert:           "#d9c07a",
	wgvc.TerrainBadlands:         "#b07a4e",
	wgvc.TerrainScrubland:        "#a89a5e",
	wgvc.TerrainPlains:           "#a7bd72",
	wgvc.TerrainGrassland:        "#8fb45c",
	wgvc.TerrainSteppe:           "#b9b071",
	wgvc.TerrainSavanna:          "#c9b45a",
	wgvc.TerrainBorealForest:     "#2f5741",
	wgvc.TerrainTemperateForest:  "#3f7a3a",
	wgvc.TerrainRainforest:       "#1f5a2c",
	wgvc.TerrainHills:            "#8a8257",
	wgvc.TerrainPlateau:          "#b3a98a",
	wgvc.TerrainMountain:         "#8a8a8a",
	wgvc.TerrainAlpine:           "#c7ccd1",
	wgvc.TerrainVolcano:          "#7a2a24",
	wgvc.TerrainVolcanicHighland: "#5c4038",
	wgvc.TerrainCoast:            "#d8cfa5",
}

type renderPoint struct {
	x float64
	y float64
}

type renderPolygon struct {
	points []renderPoint
	fill   string
}

type renderLine struct {
	start renderPoint
	end   renderPoint
}

type renderRiverSegment struct {
	line  renderLine
	width float64
}

type renderScene struct {
	width      int
	height     int
	polygons   []renderPolygon
	coastlines []renderLine
	rivers     []renderRiverSegment
}

func buildScene(world wgvc.World, width, height int, layer mapLayer) (renderScene, error) {
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
	transform := func(point wgvc.Point) renderPoint {
		return renderPoint{
			x: xOffset + (point.X-minimum.X)*scale,
			y: float64(height) - yOffset - (point.Y-minimum.Y)*scale,
		}
	}

	scene := renderScene{width: width, height: height, polygons: make([]renderPolygon, 0, len(world.Provinces))}
	for _, province := range world.Provinces {
		fill, err := provinceFill(layer, province)
		if err != nil {
			return renderScene{}, fmt.Errorf("province %d: %w", province.ID, err)
		}
		if len(province.CornerIDs) < 3 {
			return renderScene{}, fmt.Errorf("province %d has fewer than three corners", province.ID)
		}
		polygon := renderPolygon{fill: fill, points: make([]renderPoint, len(province.CornerIDs))}
		for index, cornerID := range province.CornerIDs {
			if cornerID < 0 || int(cornerID) >= len(world.Corners) {
				return renderScene{}, fmt.Errorf("province %d references unknown corner %d", province.ID, cornerID)
			}
			polygon.points[index] = transform(world.Corners[cornerID].Point)
		}
		scene.polygons = append(scene.polygons, polygon)
	}

	for _, edge := range world.Edges {
		if len(edge.ProvinceIDs) != 2 {
			continue
		}
		firstProvince, secondProvince := edge.ProvinceIDs[0], edge.ProvinceIDs[1]
		if firstProvince < 0 || int(firstProvince) >= len(world.Provinces) || secondProvince < 0 || int(secondProvince) >= len(world.Provinces) {
			return renderScene{}, fmt.Errorf("edge %d references an unknown province", edge.ID)
		}
		firstWater := world.Provinces[firstProvince].IslandID == wgvc.NoIslandID
		secondWater := world.Provinces[secondProvince].IslandID == wgvc.NoIslandID
		if firstWater == secondWater {
			continue
		}
		firstCorner, secondCorner := edge.CornerIDs[0], edge.CornerIDs[1]
		if firstCorner < 0 || int(firstCorner) >= len(world.Corners) || secondCorner < 0 || int(secondCorner) >= len(world.Corners) {
			return renderScene{}, fmt.Errorf("edge %d references an unknown corner", edge.ID)
		}
		scene.coastlines = append(scene.coastlines, renderLine{
			start: transform(world.Corners[firstCorner].Point),
			end:   transform(world.Corners[secondCorner].Point),
		})
	}
	for _, river := range world.Rivers {
		for _, edgeID := range river.EdgeIDs {
			if edgeID < 0 || int(edgeID) >= len(world.Edges) {
				return renderScene{}, fmt.Errorf("river %d references unknown edge %d", river.ID, edgeID)
			}
			edge := world.Edges[edgeID]
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

func terrainColor(terrain wgvc.Terrain) (string, error) {
	color, ok := terrainColors[terrain]
	if !ok {
		return "", fmt.Errorf("unsupported terrain %q", terrain)
	}
	return color, nil
}

func renderSVG(scene renderScene) ([]byte, error) {
	var svg strings.Builder
	fmt.Fprintf(&svg, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", scene.width, scene.height, scene.width, scene.height)
	fmt.Fprintf(&svg, "<rect width=\"%d\" height=\"%d\" fill=\"%s\"/>\n", scene.width, scene.height, backgroundColor)
	for _, polygon := range scene.polygons {
		fmt.Fprintf(&svg, "<polygon fill=\"%s\" stroke=\"%s\" stroke-width=\"%.1f\" stroke-linejoin=\"round\" points=\"", polygon.fill, cellBorderColor, cellBorderWidth)
		for _, point := range polygon.points {
			fmt.Fprintf(&svg, "%.3f,%.3f ", point.x, point.y)
		}
		svg.WriteString("\"/>\n")
	}
	for _, coastline := range scene.coastlines {
		fmt.Fprintf(&svg, "<line stroke=\"%s\" stroke-width=\"%.1f\" stroke-linecap=\"round\" x1=\"%.3f\" y1=\"%.3f\" x2=\"%.3f\" y2=\"%.3f\"/>\n", coastlineColor, coastlineWidth, coastline.start.x, coastline.start.y, coastline.end.x, coastline.end.y)
	}
	for _, river := range scene.rivers {
		fmt.Fprintf(&svg, "<line class=\"river\" stroke=\"%s\" stroke-width=\"%.1f\" stroke-linecap=\"round\" x1=\"%.3f\" y1=\"%.3f\" x2=\"%.3f\" y2=\"%.3f\"/>\n", riverColor, river.width, river.line.start.x, river.line.start.y, river.line.end.x, river.line.end.y)
	}
	svg.WriteString("</svg>\n")
	return []byte(svg.String()), nil
}

func renderPNG(scene renderScene) ([]byte, error) {
	canvas := gg.NewContext(scene.width, scene.height)
	canvas.SetHexColor(backgroundColor)
	canvas.Clear()
	canvas.SetLineJoin(gg.LineJoinRound)
	for _, polygon := range scene.polygons {
		tracePolygon(canvas, polygon.points)
		canvas.SetHexColor(polygon.fill)
		canvas.FillPreserve()
		canvas.SetHexColor(cellBorderColor)
		canvas.SetLineWidth(cellBorderWidth)
		canvas.Stroke()
	}
	canvas.SetHexColor(coastlineColor)
	canvas.SetLineWidth(coastlineWidth)
	canvas.SetLineCap(gg.LineCapRound)
	for _, coastline := range scene.coastlines {
		canvas.DrawLine(coastline.start.x, coastline.start.y, coastline.end.x, coastline.end.y)
		canvas.Stroke()
	}
	canvas.SetHexColor(riverColor)
	for _, river := range scene.rivers {
		canvas.SetLineWidth(river.width)
		canvas.DrawLine(river.line.start.x, river.line.start.y, river.line.end.x, river.line.end.y)
		canvas.Stroke()
	}

	var output bytes.Buffer
	if err := png.Encode(&output, canvas.Image()); err != nil {
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
