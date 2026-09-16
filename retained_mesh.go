package wgvc

import (
	"fmt"
	"math"
	"sort"
)

// extractRetainedMesh filters a complete canonical candidate mesh down to the
// selected land cells. It only remaps existing topology; coordinates and cell
// centers are copied exactly from the candidate mesh.
func extractRetainedMesh(candidate islandMesh, selectedCandidateIDs []int) (islandMesh, error) {
	if err := validateCompleteCandidateMesh(candidate); err != nil {
		return islandMesh{}, fmt.Errorf("validate complete candidate mesh: %w", err)
	}
	if len(selectedCandidateIDs) == 0 {
		return islandMesh{}, fmt.Errorf("at least one retained candidate is required")
	}

	selected := append([]int(nil), selectedCandidateIDs...)
	sort.Ints(selected)
	candidateToCell := make([]int, len(candidate.cells))
	for candidateID := range candidateToCell {
		candidateToCell[candidateID] = -1
	}
	for cellID, candidateID := range selected {
		if candidateID < 0 || candidateID >= len(candidate.cells) {
			return islandMesh{}, fmt.Errorf("selected candidate %d is outside [0, %d)", candidateID, len(candidate.cells))
		}
		if cellID > 0 && selected[cellID-1] == candidateID {
			return islandMesh{}, fmt.Errorf("selected candidate %d is repeated", candidateID)
		}
		candidateToCell[candidateID] = cellID
	}

	retainedEdges := make([]meshEdge, 0, len(candidate.edges))
	usedCorners := make([]bool, len(candidate.corners))
	for _, edge := range candidate.edges {
		incidence := make([]int, 0, len(edge.siteIndexes))
		for _, candidateID := range edge.siteIndexes {
			if cellID := candidateToCell[candidateID]; cellID != -1 {
				incidence = append(incidence, cellID)
			}
		}
		if len(incidence) == 0 {
			continue
		}
		retainedEdges = append(retainedEdges, meshEdge{
			cornerIDs:   edge.cornerIDs,
			siteIndexes: incidence,
		})
		usedCorners[edge.cornerIDs[0]] = true
		usedCorners[edge.cornerIDs[1]] = true
	}
	for _, candidateID := range selected {
		for _, cornerID := range candidate.cells[candidateID].cornerIDs {
			usedCorners[cornerID] = true
		}
	}

	cornerRemap := make([]int, len(candidate.corners))
	for cornerID := range cornerRemap {
		cornerRemap[cornerID] = -1
	}
	corners := make([]Point, 0, len(candidate.corners))
	for candidateCornerID, used := range usedCorners {
		if !used {
			continue
		}
		cornerRemap[candidateCornerID] = len(corners)
		corners = append(corners, candidate.corners[candidateCornerID])
	}

	for edgeID := range retainedEdges {
		retainedEdges[edgeID].cornerIDs = [2]int{
			cornerRemap[retainedEdges[edgeID].cornerIDs[0]],
			cornerRemap[retainedEdges[edgeID].cornerIDs[1]],
		}
	}
	cells := make([]meshCell, len(selected))
	for cellID, candidateID := range selected {
		candidateCell := candidate.cells[candidateID]
		ring := make([]int, len(candidateCell.cornerIDs))
		for ringIndex, candidateCornerID := range candidateCell.cornerIDs {
			ring[ringIndex] = cornerRemap[candidateCornerID]
		}
		cells[cellID] = meshCell{
			siteIndex: cellID,
			center:    candidateCell.center,
			cornerIDs: ring,
		}
	}

	retained := islandMesh{
		islandID: candidate.islandID,
		cells:    cells,
		corners:  corners,
		edges:    retainedEdges,
	}
	if err := validateRetainedLandMesh(retained); err != nil {
		return islandMesh{}, fmt.Errorf("validate retained land mesh: %w", err)
	}
	return retained, nil
}

func validateCompleteCandidateMesh(mesh islandMesh) error {
	return validateCanonicalMesh(mesh, true)
}

func validateRetainedLandMesh(mesh islandMesh) error {
	return validateCanonicalMesh(mesh, false)
}

// validateCanonicalMesh shares topology and polygon validation while keeping
// complete-candidate coverage separate from retained-land validation.
func validateCanonicalMesh(mesh islandMesh, requireUnitCoverage bool) error {
	if len(mesh.cells) == 0 {
		return fmt.Errorf("mesh has no cells")
	}
	if len(mesh.corners) < 3 {
		return fmt.Errorf("mesh has only %d corners", len(mesh.corners))
	}

	cornerInRing := make([]bool, len(mesh.corners))
	cornerInEdge := make([]bool, len(mesh.corners))
	for cornerID, point := range mesh.corners {
		if !finitePoint(point) || !pointInUnitSquare(point) {
			return fmt.Errorf("corner %d is outside the finite unit square: %+v", cornerID, point)
		}
		if cornerID > 0 && !pointLess(mesh.corners[cornerID-1], point) {
			return fmt.Errorf("corners %d and %d are not in strict canonical order", cornerID-1, cornerID)
		}
	}

	edgeByCorners := make(map[[2]int]int, len(mesh.edges))
	for edgeID, edge := range mesh.edges {
		first, second := edge.cornerIDs[0], edge.cornerIDs[1]
		if first < 0 || second >= len(mesh.corners) || first >= second {
			return fmt.Errorf("edge %d has invalid canonical corners %v", edgeID, edge.cornerIDs)
		}
		if edgeID > 0 && !edgePairLess(mesh.edges[edgeID-1].cornerIDs, edge.cornerIDs) {
			return fmt.Errorf("edges %d and %d are not in strict canonical order", edgeID-1, edgeID)
		}
		if pointDistance(mesh.corners[first], mesh.corners[second]) <= geometryTolerance {
			return fmt.Errorf("edge %d has non-positive length within tolerance", edgeID)
		}
		if len(edge.siteIndexes) < 1 || len(edge.siteIndexes) > 2 {
			return fmt.Errorf("edge %d has %d incident cells", edgeID, len(edge.siteIndexes))
		}
		for incidenceIndex, cellID := range edge.siteIndexes {
			if cellID < 0 || cellID >= len(mesh.cells) {
				return fmt.Errorf("edge %d refers to unknown cell %d", edgeID, cellID)
			}
			if incidenceIndex > 0 && edge.siteIndexes[incidenceIndex-1] >= cellID {
				return fmt.Errorf("edge %d has non-canonical incidence %v", edgeID, edge.siteIndexes)
			}
		}
		cornerInEdge[first] = true
		cornerInEdge[second] = true
		edgeByCorners[edge.cornerIDs] = edgeID
	}

	totalArea := 0.0
	edgeTraversals := make([][][2]int, len(mesh.edges))
	for cellID, cell := range mesh.cells {
		if cell.siteIndex != cellID {
			return fmt.Errorf("cell %d has site index %d", cellID, cell.siteIndex)
		}
		if !finitePoint(cell.center) || !pointInUnitSquare(cell.center) {
			return fmt.Errorf("cell %d has center outside the finite unit square: %+v", cellID, cell.center)
		}
		if len(cell.cornerIDs) < 3 {
			return fmt.Errorf("cell %d has only %d corners", cellID, len(cell.cornerIDs))
		}
		if cell.cornerIDs[0] != minimumInt(cell.cornerIDs) {
			return fmt.Errorf("cell %d does not start at its lowest corner ID", cellID)
		}

		ring := make([]Point, len(cell.cornerIDs))
		seenCorners := make(map[int]struct{}, len(cell.cornerIDs))
		for ringIndex, cornerID := range cell.cornerIDs {
			if cornerID < 0 || cornerID >= len(mesh.corners) {
				return fmt.Errorf("cell %d refers to unknown corner %d", cellID, cornerID)
			}
			if _, exists := seenCorners[cornerID]; exists {
				return fmt.Errorf("cell %d repeats corner %d", cellID, cornerID)
			}
			seenCorners[cornerID] = struct{}{}
			cornerInRing[cornerID] = true
			ring[ringIndex] = mesh.corners[cornerID]

			next := cell.cornerIDs[(ringIndex+1)%len(cell.cornerIDs)]
			edgeID, exists := edgeByCorners[orderedPair(cornerID, next)]
			if !exists {
				return fmt.Errorf("cell %d segment %d->%d has no edge", cellID, cornerID, next)
			}
			if !containsInt(mesh.edges[edgeID].siteIndexes, cellID) {
				return fmt.Errorf("cell %d segment edge %d has inconsistent incidence", cellID, edgeID)
			}
			edgeTraversals[edgeID] = append(edgeTraversals[edgeID], [2]int{cornerID, next})
		}

		area := signedArea(ring)
		if area <= geometryTolerance {
			return fmt.Errorf("cell %d has non-positive area %g", cellID, area)
		}
		totalArea += area
		for ringIndex, point := range ring {
			next := ring[(ringIndex+1)%len(ring)]
			after := ring[(ringIndex+2)%len(ring)]
			if pointCross(point, next, after) < -geometryTolerance {
				return fmt.Errorf("cell %d is not convex and counterclockwise at corner %d", cellID, ringIndex)
			}
			if pointCross(point, next, cell.center) < -geometryTolerance {
				return fmt.Errorf("cell %d does not contain its generating center", cellID)
			}
		}
	}

	if requireUnitCoverage && math.Abs(totalArea-1) > geometryTolerance {
		return fmt.Errorf("cell areas sum to %.17g, want 1", totalArea)
	}
	for edgeID, edge := range mesh.edges {
		traversals := edgeTraversals[edgeID]
		if len(traversals) != len(edge.siteIndexes) {
			return fmt.Errorf("edge %d has %d polygon traversals for %d incident cells", edgeID, len(traversals), len(edge.siteIndexes))
		}
		if len(traversals) == 2 && (traversals[0][0] != traversals[1][1] || traversals[0][1] != traversals[1][0]) {
			return fmt.Errorf("edge %d is not traversed in opposite directions", edgeID)
		}
	}
	for cornerID := range mesh.corners {
		if !cornerInRing[cornerID] || !cornerInEdge[cornerID] {
			return fmt.Errorf("corner %d is orphaned", cornerID)
		}
	}
	return nil
}

func pointCross(first, second, third Point) float64 {
	return (second.X-first.X)*(third.Y-first.Y) - (second.Y-first.Y)*(third.X-first.X)
}

func edgePairLess(first, second [2]int) bool {
	if first[0] != second[0] {
		return first[0] < second[0]
	}
	return first[1] < second[1]
}

func minimumInt(values []int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}
