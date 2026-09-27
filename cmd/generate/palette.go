package main

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/mdhender/wgvc"
)

// mapLayer selects the province field that colors a rendered map. A layer
// changes only the polygon fill; borders, coastlines, and the background are
// the same for every layer.
type mapLayer string

const (
	layerTerrain   mapLayer = "terrain"
	layerElevation mapLayer = "elevation"
	layerRelief    mapLayer = "relief"
	layerHeat      mapLayer = "heat"
	layerMoisture  mapLayer = "moisture"
	layerClimate   mapLayer = "climate"
)

var mapLayers = [...]mapLayer{layerTerrain, layerElevation, layerRelief, layerHeat, layerMoisture, layerClimate}

func parseLayer(value string) (mapLayer, error) {
	for _, layer := range mapLayers {
		if value == string(layer) {
			return layer, nil
		}
	}
	names := make([]string, len(mapLayers))
	for i, layer := range mapLayers {
		names[i] = string(layer)
	}
	return "", fmt.Errorf("unsupported layer %q: use %s", value, strings.Join(names, ", "))
}

// provinceFill returns the hex fill color for a province under a layer.
func provinceFill(layer mapLayer, province wgvc.Province) (string, error) {
	switch layer {
	case layerTerrain:
		return terrainColor(province.Terrain)
	case layerElevation:
		if err := checkFinite("elevation", province.Elevation); err != nil {
			return "", err
		}
		// Pick the side of the coastline step by membership, not by value, so
		// a land province at exactly sea level still reads as land.
		t := (province.Elevation + 1) / 2
		if province.IslandID == wgvc.NoIslandID {
			t = min(t, elevationRampBreak)
		} else {
			t = max(t, elevationRampLand)
		}
		return hexColor(ElevationRamp.At(t)), nil
	case layerRelief:
		// Relief is nominally in [0, 1) but a mean of neighbor differences
		// rarely passes 0.3, so the layer spans a fixed display range and
		// saturates above it rather than stretching per world.
		return rampFill(UnitRamp, "relief", province.Relief/reliefDisplayMax)
	case layerHeat:
		return rampFill(TemperatureRamp, "heat", province.Heat)
	case layerMoisture:
		return rampFill(MoistureRamp, "moisture", province.Moisture)
	case layerClimate:
		return climateColor(province.HeatBand, province.MoistureBand)
	}
	return "", fmt.Errorf("unsupported layer %q", layer)
}

func rampFill(ramp Ramp, field string, value float64) (string, error) {
	if err := checkFinite(field, value); err != nil {
		return "", err
	}
	return hexColor(ramp.At(value)), nil
}

func checkFinite(field string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s %v is not finite", field, value)
	}
	return nil
}

func climateColor(heat wgvc.HeatBand, moisture wgvc.MoistureBand) (string, error) {
	if heat < 0 || int(heat) >= len(climateColors) || moisture < 0 || int(moisture) >= len(climateColors[heat]) {
		return "", fmt.Errorf("unsupported climate: heat band %d, moisture band %d", heat, moisture)
	}
	return hexColor(climateColors[heat][moisture]), nil
}

func hexColor(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// RampStop is one anchor of a diagnostic ramp: a position in [0, 1] and the
// color there.
type RampStop struct {
	At    float64
	Color color.RGBA
}

// Ramp maps a normalized scalar to a color by interpolating between its stops.
// Stops ascend by At, the first is at 0 and the last at 1.
type Ramp []RampStop

// At returns the color at t, clamped to [0, 1]. Interpolation is linear in
// sRGB, which is not perceptually even but is enough for a diagnostic ramp.
// Callers reject NaN before calling.
func (r Ramp) At(t float64) color.RGBA {
	t = min(max(t, 0), 1)
	for i := 1; i < len(r); i++ {
		if t > r[i].At {
			continue
		}
		lo, hi := r[i-1], r[i]
		span := hi.At - lo.At
		if span <= 0 {
			return hi.Color
		}
		u := (t - lo.At) / span
		return color.RGBA{
			R: lerpChannel(lo.Color.R, hi.Color.R, u),
			G: lerpChannel(lo.Color.G, hi.Color.G, u),
			B: lerpChannel(lo.Color.B, hi.Color.B, u),
			A: 0xff,
		}
	}
	return r[len(r)-1].Color
}

func lerpChannel(a, b uint8, t float64) uint8 {
	v := float64(a) + (float64(b)-float64(a))*t
	return uint8(min(max(math.Round(v), 0), 255))
}

const (
	// elevationRampBreak is sea level on ElevationRamp: the last water stop.
	elevationRampBreak = 0.5
	// elevationRampLand is the first land stop, just past the break.
	elevationRampLand = 0.5001
)

// ElevationRamp colors elevation fed as (e+1)/2, so sea level sits at 0.5. The
// stops either side of the midpoint are a step rather than a blend, so the
// coastline shows as a hard line. Below the break runs deep ocean to shelf;
// above it, coastal green through upland brown to snow.
var ElevationRamp = Ramp{
	{At: 0.00, Color: color.RGBA{R: 0x04, G: 0x14, B: 0x33, A: 0xff}},
	{At: 0.35, Color: color.RGBA{R: 0x1d, G: 0x52, B: 0x8c, A: 0xff}},
	{At: elevationRampBreak, Color: color.RGBA{R: 0x74, G: 0xb3, B: 0xd4, A: 0xff}},
	{At: elevationRampLand, Color: color.RGBA{R: 0x3f, G: 0x6f, B: 0x3a, A: 0xff}},
	{At: 0.65, Color: color.RGBA{R: 0x8f, G: 0x9c, B: 0x4a, A: 0xff}},
	{At: 0.82, Color: color.RGBA{R: 0x9c, G: 0x6f, B: 0x40, A: 0xff}},
	{At: 1.00, Color: color.RGBA{R: 0xf4, G: 0xf4, B: 0xf0, A: 0xff}},
}

// reliefDisplayMax is the relief value that reaches the pale end of the relief
// layer. Measured across seeds, sizes, and aspect ratios, the 99th percentile
// of land relief sits between 0.23 and 0.31 and the maximum near 0.55, so 0.4
// keeps nearly every province on the ramp while the terrain thresholds (0.12
// wetland cap, 0.14 hills, 0.16 mountains, 0.20 badlands) fall at 30% to 50%
// of it, around the ramp's first color break, where flat and rugged separate.
const reliefDisplayMax = 0.4

// UnitRamp is a sequential ramp for a field in [0, 1] with no zero crossing,
// such as relief over its display range.
var UnitRamp = Ramp{
	{At: 0.00, Color: color.RGBA{R: 0x10, G: 0x14, B: 0x20, A: 0xff}},
	{At: 0.35, Color: color.RGBA{R: 0x39, G: 0x5c, B: 0x7a, A: 0xff}},
	{At: 0.70, Color: color.RGBA{R: 0xc0, G: 0x9a, B: 0x54, A: 0xff}},
	{At: 1.00, Color: color.RGBA{R: 0xfb, G: 0xf3, B: 0xdc, A: 0xff}},
}

// TemperatureRamp colors heat in [0, 1], cold blue through a near-neutral
// midpoint to hot red.
var TemperatureRamp = Ramp{
	{At: 0.00, Color: color.RGBA{R: 0x16, G: 0x2c, B: 0x63, A: 0xff}},
	{At: 0.25, Color: color.RGBA{R: 0x4d, G: 0x8f, B: 0xc4, A: 0xff}},
	{At: 0.50, Color: color.RGBA{R: 0xf0, G: 0xea, B: 0xdc, A: 0xff}},
	{At: 0.75, Color: color.RGBA{R: 0xdd, G: 0x8b, B: 0x3a, A: 0xff}},
	{At: 1.00, Color: color.RGBA{R: 0x8c, G: 0x1f, B: 0x1a, A: 0xff}},
}

// MoistureRamp colors moisture in [0, 1], desert ochre through neutral to deep
// green. It deliberately differs from TemperatureRamp so the two independent
// axes are not read as one quantity.
var MoistureRamp = Ramp{
	{At: 0.00, Color: color.RGBA{R: 0x8a, G: 0x67, B: 0x24, A: 0xff}},
	{At: 0.25, Color: color.RGBA{R: 0xd2, G: 0xb1, B: 0x6a, A: 0xff}},
	{At: 0.50, Color: color.RGBA{R: 0xef, G: 0xec, B: 0xe0, A: 0xff}},
	{At: 0.75, Color: color.RGBA{R: 0x5a, G: 0x9e, B: 0x86, A: 0xff}},
	{At: 1.00, Color: color.RGBA{R: 0x10, G: 0x3f, B: 0x45, A: 0xff}},
}

// climateColors is indexed [HeatBand][MoistureBand]: polar to hot down, arid
// to saturated across. It is one table rather than two multiplied ramps
// because the axes are independent; lightness varies across and hue down, so a
// reader can tell which axis a difference is on.
var climateColors = [5][5]color.RGBA{
	{ // polar
		{R: 0xc9, G: 0xd4, B: 0xe0, A: 0xff}, {R: 0xb3, G: 0xc4, B: 0xd8, A: 0xff},
		{R: 0x9a, G: 0xb2, B: 0xcd, A: 0xff}, {R: 0x7e, G: 0x9c, B: 0xc0, A: 0xff},
		{R: 0x62, G: 0x88, B: 0xb4, A: 0xff},
	},
	{ // cold
		{R: 0xcb, G: 0xd3, B: 0xc9, A: 0xff}, {R: 0xae, G: 0xc3, B: 0xb6, A: 0xff},
		{R: 0x8f, G: 0xb2, B: 0xa2, A: 0xff}, {R: 0x6f, G: 0xa0, B: 0x8e, A: 0xff},
		{R: 0x4f, G: 0x8e, B: 0x7b, A: 0xff},
	},
	{ // temperate
		{R: 0xde, G: 0xd9, B: 0xb4, A: 0xff}, {R: 0xc8, G: 0xcf, B: 0x93, A: 0xff},
		{R: 0xa9, G: 0xc1, B: 0x76, A: 0xff}, {R: 0x83, G: 0xb2, B: 0x5c, A: 0xff},
		{R: 0x5d, G: 0xa2, B: 0x44, A: 0xff},
	},
	{ // warm
		{R: 0xe6, G: 0xcf, B: 0x94, A: 0xff}, {R: 0xd6, G: 0xc4, B: 0x73, A: 0xff},
		{R: 0xbd, G: 0xb9, B: 0x5a, A: 0xff}, {R: 0x94, G: 0xad, B: 0x4d, A: 0xff},
		{R: 0x6a, G: 0x9f, B: 0x40, A: 0xff},
	},
	{ // hot
		{R: 0xe8, G: 0xbb, B: 0x6e, A: 0xff}, {R: 0xdb, G: 0xa8, B: 0x5a, A: 0xff},
		{R: 0xc7, G: 0x9a, B: 0x4c, A: 0xff}, {R: 0x8f, G: 0x9c, B: 0x3c, A: 0xff},
		{R: 0x4f, G: 0x8f, B: 0x33, A: 0xff},
	},
}
