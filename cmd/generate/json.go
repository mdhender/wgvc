package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"

	"github.com/mdhender/wgvc"
	"github.com/mdhender/wgvc/internal/x24"
)

const jsonSchemaVersion = 11

type jsonWorld struct {
	SchemaVersion int            `json:"schema_version"`
	Generation    jsonGeneration `json:"generation"`
	Bounds        jsonBounds     `json:"bounds"`
	// MinimumPassableEdgeLength is wgvc.MinimumPassableEdgeLength: edges
	// shorter than it are exported like any other but are not borders a
	// player may cross.
	MinimumPassableEdgeLength float64        `json:"minimum_passable_edge_length"`
	Islands                   []jsonIsland   `json:"islands"`
	Basins                    []jsonBasin    `json:"basins"`
	Provinces                 []jsonProvince `json:"provinces"`
	Corners                   []jsonCorner   `json:"corners"`
	Edges                     []jsonEdge     `json:"edges"`
	Rivers                    []jsonRiver    `json:"rivers"`
	SeaZones                  []jsonSeaZone  `json:"sea_zones"`
	Straits                   []jsonStrait   `json:"straits"`
	Necks                     []jsonNeck     `json:"necks"`
	Features                  []jsonFeature  `json:"features"`
}

type jsonFeature struct {
	ID          wgvc.FeatureID    `json:"id"`
	Kind        wgvc.FeatureKind  `json:"kind"`
	IslandIDs   []wgvc.IslandID   `json:"island_ids"`
	ProvinceIDs []wgvc.ProvinceID `json:"province_ids"`
}

type jsonSeaZone struct {
	ID               wgvc.SeaZoneID    `json:"id"`
	CenterProvinceID wgvc.ProvinceID   `json:"center_province_id"`
	ProvinceIDs      []wgvc.ProvinceID `json:"province_ids"`
}

type jsonStrait struct {
	ID          wgvc.StraitID        `json:"id"`
	IslandIDs   [2]wgvc.IslandID     `json:"island_ids"`
	Width       int                  `json:"width"`
	ProvinceIDs []wgvc.ProvinceID    `json:"province_ids"`
	Shores      [2][]wgvc.ProvinceID `json:"shores"`
}

type jsonNeck struct {
	ID          wgvc.NeckID          `json:"id"`
	IslandID    wgvc.IslandID        `json:"island_id"`
	Width       int                  `json:"width"`
	ProvinceIDs []wgvc.ProvinceID    `json:"province_ids"`
	Ends        [2][]wgvc.ProvinceID `json:"ends"`
	EndSizes    [2]int               `json:"end_sizes"`
}

type jsonGeneration struct {
	Generator jsonGenerator        `json:"generator"`
	Config    jsonGenerationConfig `json:"config"`
	Result    jsonGenerationResult `json:"result"`
}

// jsonGenerator identifies the code that produced a world. Version omits
// semver build metadata so exports from different builds of one release
// compare equal; Build carries the VCS commit when the binary recorded one.
type jsonGenerator struct {
	Version string `json:"version"`
	Build   string `json:"build,omitempty"`
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
	Constellation      string    `json:"attractant_constellation,omitempty"`
	AttractantRamp     []float64 `json:"attractant_ramp"`
	AttractantJitter   float64   `json:"attractant_jitter"`
	SoftmaxTemperature float64   `json:"softmax_temperature"`
	RivalRamp          []float64 `json:"rival_ramp"`
	RepulsorRamp       []float64 `json:"repulsor_ramp"`
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
	// AttractantProvinceIDs lists the provinces that hosted attractant
	// sources, in placement order; a cell's ID is its province's ID.
	AttractantProvinceIDs []int `json:"attractant_province_ids"`
	// RepulsorProvinceIDs lists the provinces that hosted constellation
	// repulsors, in placement order.
	RepulsorProvinceIDs []int `json:"repulsor_province_ids"`
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

type jsonBasin struct {
	ID               wgvc.BasinID      `json:"id"`
	ProvinceIDs      []wgvc.ProvinceID `json:"province_ids"`
	SurfaceElevation float64           `json:"surface_elevation"`
	Depth            float64           `json:"depth"`
}

type jsonProvince struct {
	ID            wgvc.ProvinceID `json:"id"`
	IslandID      wgvc.IslandID   `json:"island_id"`
	BasinID       wgvc.BasinID    `json:"basin_id"`
	Center        jsonPoint       `json:"center"`
	CornerIDs     []wgvc.CornerID `json:"corner_ids"`
	EdgeIDs       []wgvc.EdgeID   `json:"edge_ids"`
	Exits         []jsonExit      `json:"exits"`
	Area          float64         `json:"area"`
	CoastDistance int             `json:"coast_distance"`
	SeaZoneID     wgvc.SeaZoneID  `json:"sea_zone_id"`
	OceanEdges    int             `json:"ocean_edges"`
	BasinEdges    int             `json:"basin_edges"`
	Shelter       float64         `json:"shelter"`
	RiverIDs      []wgvc.RiverID  `json:"river_ids"`
	RiverMouthIDs []wgvc.RiverID  `json:"river_mouth_ids"`
	ConfluenceIDs []wgvc.RiverID  `json:"confluence_ids"`
	Terrain       wgvc.Terrain    `json:"terrain"`
	Elevation     float64         `json:"elevation"`
	ElevationBand string          `json:"elevation_band"`
	Relief        float64         `json:"relief"`
	Heat          float64         `json:"heat"`
	HeatBand      string          `json:"heat_band"`
	Moisture      float64         `json:"moisture"`
	MoistureBand  string          `json:"moisture_band"`
}

type jsonExit struct {
	Number     int             `json:"number"`
	EdgeID     wgvc.EdgeID     `json:"edge_id"`
	NeighborID wgvc.ProvinceID `json:"neighbor_id"`
	Bearing    float64         `json:"bearing"`
	Compass    wgvc.Compass    `json:"compass"`
}

type jsonCorner struct {
	ID        wgvc.CornerID `json:"id"`
	Point     jsonPoint     `json:"point"`
	Elevation float64       `json:"elevation"`
}

type jsonRiver struct {
	ID        wgvc.RiverID    `json:"id"`
	Class     wgvc.RiverClass `json:"class"`
	Discharge float64         `json:"discharge"`
	CornerIDs []wgvc.CornerID `json:"corner_ids"`
	EdgeIDs   []wgvc.EdgeID   `json:"edge_ids"`
	Source    jsonRiverEnd    `json:"source"`
	Mouth     jsonRiverEnd    `json:"mouth"`
}

type jsonRiverEnd struct {
	Kind    wgvc.RiverEndKind `json:"kind"`
	BasinID wgvc.BasinID      `json:"basin_id"`
	RiverID wgvc.RiverID      `json:"river_id"`
}

type jsonEdge struct {
	ID          wgvc.EdgeID       `json:"id"`
	CornerIDs   [2]wgvc.CornerID  `json:"corner_ids"`
	ProvinceIDs []wgvc.ProvinceID `json:"province_ids"`
	Length      float64           `json:"length"`
	Elevation   float64           `json:"elevation"`
	RiverID     wgvc.RiverID      `json:"river_id"`
	Discharge   float64           `json:"discharge"`
}

// jsonIndent is the indentation of a -pretty export.
const jsonIndent = "  "

// writeJSON streams the world export to output as one compact JSON document
// followed by a newline, or indented with jsonIndent when pretty is set. The
// two forms decode to the same document; compact is the default because the
// indentation roughly doubles the file and the encoding time.
func writeJSON(output io.Writer, world wgvc.World, config x24.Config, climateConfig wgvc.ClimateConfig, result x24.Result, pretty bool) error {
	bounds, err := worldBounds(world)
	if err != nil {
		return err
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
			Generator: jsonGenerator{
				Version: wgvc.Version().Short(),
				Build:   wgvc.Version().Build,
			},
			Config: jsonGenerationConfig{
				Seed:               fmt.Sprintf("0x%016x", config.WorldSeed),
				ProvinceCount:      config.ProvinceCount,
				IslandCount:        config.IslandCount,
				AspectRatio:        config.AspectRatio,
				OceanFraction:      config.OceanPercentage,
				EdgeBarrierWidth:   config.EdgeBarrierWidth,
				EdgeRamp:           config.EdgeRamp,
				AttractantCount:    config.AttractantCount,
				Constellation:      config.Constellation,
				AttractantRamp:     config.AttractantRamp,
				AttractantJitter:   config.AttractantJitter,
				SoftmaxTemperature: config.SoftmaxTemperature,
				RivalRamp:          config.RivalRamp,
				RepulsorRamp:       config.RepulsorRamp,
				MaxRounds:          config.MaxRounds,
				Relaxations:        config.Relaxations,
				PolarIceFraction:   climateConfig.PolarIce,
				PeakChillFraction:  climateConfig.PeakChill,
			},
			Result: jsonGenerationResult{
				ProvinceCount:         len(world.Provinces),
				LandProvinceCount:     landProvinceCount,
				InitialIslandCount:    result.InitialIslandCount,
				IslandCount:           len(world.Islands),
				MergeCount:            result.MergeCount,
				RoundsAttempted:       result.RoundsAttempted,
				OceanFraction:         result.FinalOcean,
				AttractantProvinceIDs: attractantProvinceIDs(result),
				RepulsorProvinceIDs:   repulsorProvinceIDs(result),
			},
		},
		Bounds:                    bounds,
		MinimumPassableEdgeLength: wgvc.MinimumPassableEdgeLength,
		Islands:                   make([]jsonIsland, len(world.Islands)),
		Basins:                    make([]jsonBasin, len(world.Basins)),
		Provinces:                 make([]jsonProvince, len(world.Provinces)),
		Corners:                   make([]jsonCorner, len(world.Corners)),
		Edges:                     make([]jsonEdge, len(world.Edges)),
		Rivers:                    make([]jsonRiver, len(world.Rivers)),
		SeaZones:                  make([]jsonSeaZone, len(world.SeaZones)),
		Straits:                   make([]jsonStrait, len(world.Straits)),
		Necks:                     make([]jsonNeck, len(world.Necks)),
		Features:                  make([]jsonFeature, len(world.Features)),
	}
	for index, island := range world.Islands {
		document.Islands[index] = jsonIsland{ID: island.ID, ProvinceIDs: island.ProvinceIDs}
	}
	for index, basin := range world.Basins {
		document.Basins[index] = jsonBasin{
			ID:               basin.ID,
			ProvinceIDs:      basin.ProvinceIDs,
			SurfaceElevation: basin.SurfaceElevation,
			Depth:            basin.Depth,
		}
	}
	for index, province := range world.Provinces {
		elevationBand, err := elevationBandName(province.ElevationBand)
		if err != nil {
			return fmt.Errorf("province %d: %w", province.ID, err)
		}
		heatBand, err := heatBandName(province.HeatBand)
		if err != nil {
			return fmt.Errorf("province %d: %w", province.ID, err)
		}
		moistureBand, err := moistureBandName(province.MoistureBand)
		if err != nil {
			return fmt.Errorf("province %d: %w", province.ID, err)
		}
		exits := make([]jsonExit, len(province.Exits))
		for exitIndex, exit := range province.Exits {
			exits[exitIndex] = jsonExit{
				Number:     exit.Number,
				EdgeID:     exit.EdgeID,
				NeighborID: exit.NeighborID,
				Bearing:    exit.Bearing,
				Compass:    exit.Compass,
			}
		}
		document.Provinces[index] = jsonProvince{
			ID:            province.ID,
			IslandID:      province.IslandID,
			BasinID:       province.BasinID,
			Center:        toJSONPoint(province.Center),
			CornerIDs:     province.CornerIDs,
			EdgeIDs:       province.EdgeIDs,
			Exits:         exits,
			Area:          province.Area,
			CoastDistance: province.CoastDistance,
			SeaZoneID:     province.SeaZoneID,
			OceanEdges:    province.OceanEdges,
			BasinEdges:    province.BasinEdges,
			Shelter:       province.Shelter,
			RiverIDs:      emptyIfNil(province.RiverIDs),
			RiverMouthIDs: emptyIfNil(province.RiverMouthIDs),
			ConfluenceIDs: emptyIfNil(province.ConfluenceIDs),
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
		document.Corners[index] = jsonCorner{ID: corner.ID, Point: toJSONPoint(corner.Point), Elevation: corner.Elevation}
	}
	for index, edge := range world.Edges {
		document.Edges[index] = jsonEdge{
			ID:          edge.ID,
			CornerIDs:   edge.CornerIDs,
			ProvinceIDs: edge.ProvinceIDs,
			Length:      edge.Length,
			Elevation:   edge.Elevation,
			RiverID:     edge.RiverID,
			Discharge:   edge.Discharge,
		}
	}
	for index, river := range world.Rivers {
		document.Rivers[index] = jsonRiver{
			ID:        river.ID,
			Class:     river.Class,
			Discharge: river.Discharge,
			CornerIDs: river.CornerIDs,
			EdgeIDs:   river.EdgeIDs,
			Source:    jsonRiverEnd{Kind: river.Source.Kind, BasinID: river.Source.BasinID, RiverID: river.Source.RiverID},
			Mouth:     jsonRiverEnd{Kind: river.Mouth.Kind, BasinID: river.Mouth.BasinID, RiverID: river.Mouth.RiverID},
		}
	}
	for index, zone := range world.SeaZones {
		document.SeaZones[index] = jsonSeaZone{ID: zone.ID, CenterProvinceID: zone.CenterProvinceID, ProvinceIDs: zone.ProvinceIDs}
	}
	for index, strait := range world.Straits {
		document.Straits[index] = jsonStrait{ID: strait.ID, IslandIDs: strait.IslandIDs, Width: strait.Width, ProvinceIDs: strait.ProvinceIDs, Shores: strait.Shores}
	}
	for index, neck := range world.Necks {
		document.Necks[index] = jsonNeck{ID: neck.ID, IslandID: neck.IslandID, Width: neck.Width, ProvinceIDs: neck.ProvinceIDs, Ends: neck.Ends, EndSizes: neck.EndSizes}
	}
	for index, feature := range world.Features {
		document.Features[index] = jsonFeature{ID: feature.ID, Kind: feature.Kind, IslandIDs: feature.IslandIDs, ProvinceIDs: feature.ProvinceIDs}
	}
	encoder := json.NewEncoder(output)
	if pretty {
		encoder.SetIndent("", jsonIndent)
	}
	return encoder.Encode(document)
}

// emptyIfNil keeps list fields as [] rather than null in the export.
func emptyIfNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
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

func attractantProvinceIDs(result x24.Result) []int {
	ids := make([]int, len(result.Attractants))
	for i, attractant := range result.Attractants {
		ids[i] = attractant.CellID
	}
	return ids
}

func repulsorProvinceIDs(result x24.Result) []int {
	ids := make([]int, len(result.Repulsors))
	for i, repulsor := range result.Repulsors {
		ids[i] = repulsor.CellID
	}
	return ids
}
