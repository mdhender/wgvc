package wgvc

import (
	"fmt"
	"math"
)

const (
	islandWaterGap    = 1.0
	islandWorldMargin = 1.0
	placementAttempts = 10_000
	placementRetries  = 20
)

type rectangle struct {
	min Point
	max Point
}

type uniformTransform struct {
	scale       float64
	translation Point
}

func (transform uniformTransform) point(point Point) Point {
	return Point{
		X: transform.translation.X + transform.scale*point.X,
		Y: transform.translation.Y + transform.scale*point.Y,
	}
}

// planIslands constructs deterministic shape inputs without assigning world
// coordinates. Placement must wait until the resulting land meshes are known.
func planIslands(config Config, allocations []int) ([]islandPlan, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if len(allocations) != config.IslandCount {
		return nil, fmt.Errorf("allocation count = %d, want %d", len(allocations), config.IslandCount)
	}

	total := 0
	plans := make([]islandPlan, len(allocations))
	for islandIndex, allocation := range allocations {
		if allocation < 1 {
			return nil, fmt.Errorf("island %d allocation must be at least 1: %d", islandIndex, allocation)
		}
		total += allocation
		islandID := IslandID(islandIndex)
		candidates, err := planCandidateSites(allocation, candidateRandom(config.WorldSeed, islandID))
		if err != nil {
			return nil, fmt.Errorf("island %d candidate sites: %w", islandID, err)
		}
		plans[islandIndex] = islandPlan{
			id:                islandID,
			landProvinceCount: allocation,
			candidates:        candidates,
			shape:             generateBlobShape(blobShapeRandom(config.WorldSeed, islandID)),
		}
	}
	if total != config.ProvinceCount {
		return nil, fmt.Errorf("allocation total = %d, want %d", total, config.ProvinceCount)
	}
	return plans, nil
}

// placeIslands scales each normalized land mesh so its land area equals its
// requested province count, then places its full candidate envelope. Omitted
// ocean therefore still contributes to spacing even though it is not public.
func placeIslands(config Config, plans []islandPlan, meshes tessellation) (islandLayout, error) {
	if err := config.validate(); err != nil {
		return islandLayout{}, err
	}
	if len(plans) != config.IslandCount {
		return islandLayout{}, fmt.Errorf("island plan count = %d, want %d", len(plans), config.IslandCount)
	}
	if len(meshes.islands) != len(plans) {
		return islandLayout{}, fmt.Errorf("island mesh count = %d, want %d", len(meshes.islands), len(plans))
	}

	scales := make([]float64, len(plans))
	total := 0
	for islandIndex, plan := range plans {
		if plan.id != IslandID(islandIndex) {
			return islandLayout{}, fmt.Errorf("island %d has non-canonical plan ID %d", islandIndex, plan.id)
		}
		if plan.landProvinceCount < 1 {
			return islandLayout{}, fmt.Errorf("island %d allocation must be at least 1: %d", plan.id, plan.landProvinceCount)
		}
		total += plan.landProvinceCount
		mesh := meshes.islands[islandIndex]
		if mesh.islandID != plan.id {
			return islandLayout{}, fmt.Errorf("island %d mesh has ID %d", plan.id, mesh.islandID)
		}
		if len(mesh.cells) != plan.landProvinceCount {
			return islandLayout{}, fmt.Errorf("island %d retained cell count = %d, want %d", plan.id, len(mesh.cells), plan.landProvinceCount)
		}
		area, err := retainedLandArea(mesh)
		if err != nil {
			return islandLayout{}, fmt.Errorf("island %d retained land area: %w", plan.id, err)
		}
		if err := validateRetainedLandMesh(mesh); err != nil {
			return islandLayout{}, fmt.Errorf("island %d retained land mesh: %w", plan.id, err)
		}
		scale := math.Sqrt(float64(plan.landProvinceCount) / area)
		if math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
			return islandLayout{}, fmt.Errorf("island %d retained land scale is invalid: %g", plan.id, scale)
		}
		scales[islandIndex] = scale
	}
	if total != config.ProvinceCount {
		return islandLayout{}, fmt.Errorf("allocation total = %d, want %d", total, config.ProvinceCount)
	}
	if len(meshes.candidates) != len(plans) || len(meshes.landCandidateIDs) != len(plans) {
		return islandLayout{}, fmt.Errorf("candidate mesh count = %d/%d, want %d", len(meshes.candidates), len(meshes.landCandidateIDs), len(plans))
	}
	for islandIndex, plan := range plans {
		if err := validateCompleteCandidateMesh(meshes.candidates[islandIndex]); err != nil {
			return islandLayout{}, fmt.Errorf("island %d candidate mesh: %w", plan.id, err)
		}
		if len(meshes.landCandidateIDs[islandIndex]) != plan.landProvinceCount {
			return islandLayout{}, fmt.Errorf("island %d selected candidate count = %d, want %d", plan.id, len(meshes.landCandidateIDs[islandIndex]), plan.landProvinceCount)
		}
	}

	envelopes, err := scatterIslandEnvelopes(config.WorldSeed, scales)
	if err != nil {
		return islandLayout{}, err
	}
	placed := make([]placedIsland, len(plans))
	for islandIndex, plan := range plans {
		scale := scales[islandIndex]
		envelope := envelopes[islandIndex]
		transform := uniformTransform{scale: scale, translation: envelope.min}
		placed[islandIndex] = placedIsland{
			id:                plan.id,
			landProvinceCount: plan.landProvinceCount,
			mesh:              transformMesh(meshes.islands[islandIndex], transform),
			candidateMesh:     transformMesh(meshes.candidates[islandIndex], transform),
			landCandidateIDs:  append([]int(nil), meshes.landCandidateIDs[islandIndex]...),
			envelope:          envelope,
			transform:         transform,
		}
	}

	return islandLayout{islands: placed, bounds: boundsAround(placed, islandWorldMargin)}, nil
}

func retainedLandArea(mesh islandMesh) (float64, error) {
	area := 0.0
	for cellID, cell := range mesh.cells {
		if len(cell.cornerIDs) < 3 {
			return 0, fmt.Errorf("cell %d has only %d corners", cellID, len(cell.cornerIDs))
		}
		ring := make([]Point, len(cell.cornerIDs))
		for ringIndex, cornerID := range cell.cornerIDs {
			if cornerID < 0 || cornerID >= len(mesh.corners) {
				return 0, fmt.Errorf("cell %d refers to unknown corner %d", cellID, cornerID)
			}
			ring[ringIndex] = mesh.corners[cornerID]
		}
		area += signedArea(ring)
	}
	if math.IsNaN(area) || math.IsInf(area, 0) || area <= 0 {
		return 0, fmt.Errorf("must be finite and positive: %g", area)
	}
	return area, nil
}

func transformMesh(mesh islandMesh, transform uniformTransform) islandMesh {
	transformed := mesh
	transformed.corners = make([]Point, len(mesh.corners))
	for cornerID, corner := range mesh.corners {
		transformed.corners[cornerID] = transform.point(corner)
	}
	transformed.cells = make([]meshCell, len(mesh.cells))
	for cellID, cell := range mesh.cells {
		transformed.cells[cellID] = cell
		transformed.cells[cellID].center = transform.point(cell.center)
		transformed.cells[cellID].cornerIDs = append([]int(nil), cell.cornerIDs...)
	}
	transformed.edges = make([]meshEdge, len(mesh.edges))
	for edgeID, edge := range mesh.edges {
		transformed.edges[edgeID] = edge
		transformed.edges[edgeID].siteIndexes = append([]int(nil), edge.siteIndexes...)
	}
	return transformed
}

// scatterIslandEnvelopes uses deterministic rejection packing in an expanding
// square. Packing the complete candidate envelopes, rather than only the land
// bounds, keeps the private water sites separated enough to preserve each
// coastline when all candidates are tessellated together.
func scatterIslandEnvelopes(worldSeed uint64, scales []float64) ([]rectangle, error) {
	maxSpan, paddedArea := 0.0, 0.0
	for _, scale := range scales {
		maxSpan = math.Max(maxSpan, scale+2*islandWorldMargin)
		paddedArea += (scale + islandWaterGap) * (scale + islandWaterGap)
	}

	side := math.Max(maxSpan, 1.5*math.Sqrt(paddedArea)+2*islandWorldMargin)
	for expansion := 0; expansion < placementRetries; expansion++ {
		random := placementRandom(worldSeed)
		half := side / 2
		envelopes := make([]rectangle, 0, len(scales))
		for _, scale := range scales {
			limit := half - islandWorldMargin - scale/2
			accepted := false
			for attempt := 0; attempt < placementAttempts; attempt++ {
				center := Point{
					X: (2*random.Float64() - 1) * limit,
					Y: (2*random.Float64() - 1) * limit,
				}
				halfScale := scale / 2
				envelope := rectangle{
					min: Point{X: center.X - halfScale, Y: center.Y - halfScale},
					max: Point{X: center.X + halfScale, Y: center.Y + halfScale},
				}
				if overlapsEnvelopes(envelope, envelopes) {
					continue
				}
				envelopes = append(envelopes, envelope)
				accepted = true
				break
			}
			if !accepted {
				break
			}
		}
		if len(envelopes) == len(scales) {
			return envelopes, nil
		}
		side *= 1.2
	}
	return nil, fmt.Errorf("could not scatter %d island envelopes after %d expansions", len(scales), placementRetries)
}

func overlapsEnvelopes(envelope rectangle, placed []rectangle) bool {
	for _, other := range placed {
		if rectangleGap(envelope, other) < islandWaterGap {
			return true
		}
	}
	return false
}

func rectangleGap(first, second rectangle) float64 {
	dx := math.Max(math.Max(first.min.X-second.max.X, second.min.X-first.max.X), 0)
	dy := math.Max(math.Max(first.min.Y-second.max.Y, second.min.Y-first.max.Y), 0)
	return math.Hypot(dx, dy)
}

func boundsAround(islands []placedIsland, margin float64) rectangle {
	bounds := islands[0].envelope
	for _, island := range islands[1:] {
		bounds.min.X = math.Min(bounds.min.X, island.envelope.min.X)
		bounds.min.Y = math.Min(bounds.min.Y, island.envelope.min.Y)
		bounds.max.X = math.Max(bounds.max.X, island.envelope.max.X)
		bounds.max.Y = math.Max(bounds.max.Y, island.envelope.max.Y)
	}
	bounds.min.X -= margin
	bounds.min.Y -= margin
	bounds.max.X += margin
	bounds.max.Y += margin
	return bounds
}
