package wgvc

import (
	"fmt"
	"math"
)

const (
	islandWaterGap    = 1.0
	islandWorldMargin = 1.0
	islandJitter      = 0.25
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

	columns := int(math.Ceil(math.Sqrt(float64(len(plans)))))
	rows := (len(plans) + columns - 1) / columns
	scales := make([]float64, len(plans))
	columnWidths := make([]float64, columns)
	rowHeights := make([]float64, rows)
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
		column := islandIndex % columns
		row := islandIndex / columns
		columnWidths[column] = math.Max(columnWidths[column], scale)
		rowHeights[row] = math.Max(rowHeights[row], scale)
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

	columnCenters := slotCenters(columnWidths)
	rowCenters := slotCenters(rowHeights)
	random := placementRandom(config.WorldSeed)
	placed := make([]placedIsland, len(plans))
	for islandIndex, plan := range plans {
		scale := scales[islandIndex]
		center := Point{
			X: columnCenters[islandIndex%columns] + signedJitter(random.Float64()),
			Y: rowCenters[islandIndex/columns] + signedJitter(random.Float64()),
		}
		halfScale := scale / 2
		envelope := rectangle{
			min: Point{X: center.X - halfScale, Y: center.Y - halfScale},
			max: Point{X: center.X + halfScale, Y: center.Y + halfScale},
		}
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

func slotCenters(dimensions []float64) []float64 {
	centers := make([]float64, len(dimensions))
	if len(dimensions) > 0 {
		centers[0] = dimensions[0] / 2
		for index := 1; index < len(dimensions); index++ {
			centers[index] = centers[index-1] + dimensions[index-1]/2 + islandWaterGap + 2*islandJitter + dimensions[index]/2
		}
	}
	return centers
}

func signedJitter(value float64) float64 {
	return (2*value - 1) * islandJitter
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
