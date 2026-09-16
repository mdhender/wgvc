package wgvc

import "math/rand/v2"

const (
	candidateSiteInset = 0.25
)

func planCandidateSites(landCount int, random *rand.Rand) (candidateSitePlan, error) {
	columns, rows, err := candidateGridDimensions(landCount)
	if err != nil {
		return candidateSitePlan{}, err
	}
	return candidateSitePlan{
		columns: columns,
		rows:    rows,
		sites:   generateCandidateSites(columns, rows, random),
	}, nil
}

// generateCandidateSites places one site in the inset center half of every
// grid slot. Sites are therefore strictly inside the unit square and any two
// sites are separated by at least half the narrower slot dimension.
func generateCandidateSites(columns, rows int, random *rand.Rand) []Point {
	sites := make([]Point, 0, columns*rows)
	for row := range rows {
		for column := range columns {
			sites = append(sites, Point{
				X: (float64(column) + candidateSiteInset + (1-2*candidateSiteInset)*random.Float64()) / float64(columns),
				Y: (float64(row) + candidateSiteInset + (1-2*candidateSiteInset)*random.Float64()) / float64(rows),
			})
		}
	}
	return sites
}
