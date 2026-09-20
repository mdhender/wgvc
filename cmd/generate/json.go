package main

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/mdhender/wgvc"
	"github.com/mdhender/wgvc/internal/x24"
)

const jsonSchemaVersion = 1

type jsonWorld struct {
	SchemaVersion int            `json:"schema_version"`
	Generation    jsonGeneration `json:"generation"`
	Bounds        jsonBounds     `json:"bounds"`
	Islands       []jsonIsland   `json:"islands"`
	Provinces     []jsonProvince `json:"provinces"`
	Corners       []jsonCorner   `json:"corners"`
	Edges         []jsonEdge     `json:"edges"`
}

type jsonGeneration struct {
	Config jsonGenerationConfig `json:"config"`
	Result jsonGenerationResult `json:"result"`
}

type jsonGenerationConfig struct {
	Seed               string    `json:"seed"`
	ProvinceCount      int       `json:"province_count"`
	IslandCount        int       `json:"island_count"`
	AspectRatio        string    `json:"aspect_ratio"`
	OceanFraction      float64   `json:"ocean_fraction"`
	EdgeBarrierWidth   float64   `json:"edge_barrier_width"`
	EdgeRamp           []float64 `json:"edge_ramp"`
	AttractantCount    int       `json:"attractant_count"`
	AttractantRamp     []float64 `json:"attractant_ramp"`
	AttractantJitter   float64   `json:"attractant_jitter"`
	SoftmaxTemperature float64   `json:"softmax_temperature"`
	ControlPenalty     float64   `json:"control_penalty"`
	MaxRounds          int       `json:"max_rounds"`
	Relaxations        int       `json:"relaxations"`
	PolarIceFraction   float64   `json:"polar_ice_fraction"`
	PeakChillFraction  float64   `json:"peak_chill_fraction"`
}

type jsonGenerationResult struct {
	ProvinceCount      int     `json:"province_count"`
	LandProvinceCount  int     `json:"land_province_count"`
	InitialIslandCount int     `json:"initial_island_count"`
	IslandCount        int     `json:"island_count"`
	MergeCount         int     `json:"merge_count"`
	RoundsAttempted    int     `json:"rounds_attempted"`
	OceanFraction      float64 `json:"ocean_fraction"`
}

type jsonBounds struct {
	Minimum jsonPoint `json:"minimum"`
	Maximum jsonPoint `json:"maximum"`
}

type jsonPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type jsonIsland struct {
	ID          wgvc.IslandID     `json:"id"`
	ProvinceIDs []wgvc.ProvinceID `json:"province_ids"`
}

type jsonProvince struct {
	ID            wgvc.ProvinceID `json:"id"`
	IslandID      wgvc.IslandID   `json:"island_id"`
	Center        jsonPoint       `json:"center"`
	CornerIDs     []wgvc.CornerID `json:"corner_ids"`
	Terrain       wgvc.Terrain    `json:"terrain"`
	Elevation     float64         `json:"elevation"`
	ElevationBand string          `json:"elevation_band"`
	Relief        float64         `json:"relief"`
	Heat          float64         `json:"heat"`
	HeatBand      string          `json:"heat_band"`
	Moisture      float64         `json:"moisture"`
	MoistureBand  string          `json:"moisture_band"`
}

type jsonCorner struct {
	ID    wgvc.CornerID `json:"id"`
	Point jsonPoint     `json:"point"`
}

type jsonEdge struct {
	ID          wgvc.EdgeID       `json:"id"`
	CornerIDs   [2]wgvc.CornerID  `json:"corner_ids"`
	ProvinceIDs []wgvc.ProvinceID `json:"province_ids"`
	Elevation   float64           `json:"elevation"`
}

func renderJSON(world wgvc.World, config x24.Config, climateConfig wgvc.ClimateConfig, result x24.Result) ([]byte, error) {
	bounds, err := worldBounds(world)
	if err != nil {
		return nil, err
	}
	landProvinceCount := 0
	for _, province := range world.Provinces {
		if province.IslandID != wgvc.NoIslandID {
			landProvinceCount++
		}
	}
	document := jsonWorld{
		SchemaVersion: jsonSchemaVersion,
		Generation: jsonGeneration{
			Config: jsonGenerationConfig{
				Seed:               fmt.Sprintf("0x%016x", config.WorldSeed),
				ProvinceCount:      config.ProvinceCount,
				IslandCount:        config.IslandCount,
				AspectRatio:        config.AspectRatio,
				OceanFraction:      config.OceanPercentage,
				EdgeBarrierWidth:   config.EdgeBarrierWidth,
				EdgeRamp:           config.EdgeRamp,
				AttractantCount:    config.AttractantCount,
				AttractantRamp:     config.AttractantRamp,
				AttractantJitter:   config.AttractantJitter,
				SoftmaxTemperature: config.SoftmaxTemperature,
				ControlPenalty:     config.ControlPenalty,
				MaxRounds:          config.MaxRounds,
				Relaxations:        config.Relaxations,
				PolarIceFraction:   climateConfig.PolarIce,
				PeakChillFraction:  climateConfig.PeakChill,
			},
			Result: jsonGenerationResult{
				ProvinceCount:      len(world.Provinces),
				LandProvinceCount:  landProvinceCount,
				InitialIslandCount: result.InitialIslandCount,
				IslandCount:        len(world.Islands),
				MergeCount:         result.MergeCount,
				RoundsAttempted:    result.RoundsAttempted,
				OceanFraction:      result.FinalOcean,
			},
		},
		Bounds:    bounds,
		Islands:   make([]jsonIsland, len(world.Islands)),
		Provinces: make([]jsonProvince, len(world.Provinces)),
		Corners:   make([]jsonCorner, len(world.Corners)),
		Edges:     make([]jsonEdge, len(world.Edges)),
	}
	for index, island := range world.Islands {
		document.Islands[index] = jsonIsland{ID: island.ID, ProvinceIDs: island.ProvinceIDs}
	}
	for index, province := range world.Provinces {
		elevationBand, err := elevationBandName(province.ElevationBand)
		if err != nil {
			return nil, fmt.Errorf("province %d: %w", province.ID, err)
		}
		heatBand, err := heatBandName(province.HeatBand)
		if err != nil {
			return nil, fmt.Errorf("province %d: %w", province.ID, err)
		}
		moistureBand, err := moistureBandName(province.MoistureBand)
		if err != nil {
			return nil, fmt.Errorf("province %d: %w", province.ID, err)
		}
		document.Provinces[index] = jsonProvince{
			ID:            province.ID,
			IslandID:      province.IslandID,
			Center:        toJSONPoint(province.Center),
			CornerIDs:     province.CornerIDs,
			Terrain:       province.Terrain,
			Elevation:     province.Elevation,
			ElevationBand: elevationBand,
			Relief:        province.Relief,
			Heat:          province.Heat,
			HeatBand:      heatBand,
			Moisture:      province.Moisture,
			MoistureBand:  moistureBand,
		}
	}
	for index, corner := range world.Corners {
		document.Corners[index] = jsonCorner{ID: corner.ID, Point: toJSONPoint(corner.Point)}
	}
	for index, edge := range world.Edges {
		document.Edges[index] = jsonEdge{
			ID:          edge.ID,
			CornerIDs:   edge.CornerIDs,
			ProvinceIDs: edge.ProvinceIDs,
			Elevation:   edge.Elevation,
		}
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func worldBounds(world wgvc.World) (jsonBounds, error) {
	if len(world.Corners) == 0 {
		return jsonBounds{}, fmt.Errorf("world has no corners")
	}
	minimum := world.Corners[0].Point
	maximum := minimum
	for _, corner := range world.Corners[1:] {
		minimum.X = math.Min(minimum.X, corner.Point.X)
		minimum.Y = math.Min(minimum.Y, corner.Point.Y)
		maximum.X = math.Max(maximum.X, corner.Point.X)
		maximum.Y = math.Max(maximum.Y, corner.Point.Y)
	}
	return jsonBounds{Minimum: toJSONPoint(minimum), Maximum: toJSONPoint(maximum)}, nil
}

func toJSONPoint(point wgvc.Point) jsonPoint {
	return jsonPoint{X: point.X, Y: point.Y}
}

func elevationBandName(band wgvc.ElevationBand) (string, error) {
	switch band {
	case wgvc.ElevationBandDeepWater:
		return "deep-water", nil
	case wgvc.ElevationBandShallowWater:
		return "shallow-water", nil
	case wgvc.ElevationBandLowland:
		return "lowland", nil
	case wgvc.ElevationBandUpland:
		return "upland", nil
	case wgvc.ElevationBandHighland:
		return "highland", nil
	case wgvc.ElevationBandMountain:
		return "mountain", nil
	default:
		return "", fmt.Errorf("unsupported elevation band %d", band)
	}
}

func heatBandName(band wgvc.HeatBand) (string, error) {
	switch band {
	case wgvc.HeatBandPolar:
		return "polar", nil
	case wgvc.HeatBandCold:
		return "cold", nil
	case wgvc.HeatBandTemperate:
		return "temperate", nil
	case wgvc.HeatBandWarm:
		return "warm", nil
	case wgvc.HeatBandHot:
		return "hot", nil
	default:
		return "", fmt.Errorf("unsupported heat band %d", band)
	}
}

func moistureBandName(band wgvc.MoistureBand) (string, error) {
	switch band {
	case wgvc.MoistureBandArid:
		return "arid", nil
	case wgvc.MoistureBandDry:
		return "dry", nil
	case wgvc.MoistureBandModerate:
		return "moderate", nil
	case wgvc.MoistureBandHumid:
		return "humid", nil
	case wgvc.MoistureBandSaturated:
		return "saturated", nil
	default:
		return "", fmt.Errorf("unsupported moisture band %d", band)
	}
}
