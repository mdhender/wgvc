package main

import (
	"fmt"
	"io"

	"github.com/mdhender/wgvc"
)

// summaryRow is one histogram row: a band or terrain name and its province count.
type summaryRow struct {
	name  string
	count int
}

// summarySection is one histogram in declared enum order.
type summarySection struct {
	title string
	rows  []summaryRow
}

// buildSummary counts provinces per elevation band, heat band, moisture band,
// and terrain. Every declared value gets a row, including values with a count
// of zero, and rows follow the library's ordered lists.
func buildSummary(world wgvc.World) ([]summarySection, error) {
	elevation, err := countRows(wgvc.ElevationBands(), elevationBandName, world.Provinces, func(p wgvc.Province) wgvc.ElevationBand { return p.ElevationBand })
	if err != nil {
		return nil, err
	}
	heat, err := countRows(wgvc.HeatBands(), heatBandName, world.Provinces, func(p wgvc.Province) wgvc.HeatBand { return p.HeatBand })
	if err != nil {
		return nil, err
	}
	moisture, err := countRows(wgvc.MoistureBands(), moistureBandName, world.Provinces, func(p wgvc.Province) wgvc.MoistureBand { return p.MoistureBand })
	if err != nil {
		return nil, err
	}
	terrainName := func(terrain wgvc.Terrain) (string, error) { return string(terrain), nil }
	terrain, err := countRows(wgvc.Terrains(), terrainName, world.Provinces, func(p wgvc.Province) wgvc.Terrain { return p.Terrain })
	if err != nil {
		return nil, err
	}
	return []summarySection{
		{title: "elevation band", rows: elevation},
		{title: "heat band", rows: heat},
		{title: "moisture band", rows: moisture},
		{title: "terrain", rows: terrain},
	}, nil
}

func countRows[V comparable](values []V, name func(V) (string, error), provinces []wgvc.Province, value func(wgvc.Province) V) ([]summaryRow, error) {
	index := make(map[V]int, len(values))
	rows := make([]summaryRow, len(values))
	for i, v := range values {
		label, err := name(v)
		if err != nil {
			return nil, err
		}
		index[v] = i
		rows[i].name = label
	}
	for _, province := range provinces {
		i, ok := index[value(province)]
		if !ok {
			return nil, fmt.Errorf("province %d has undeclared value %v", province.ID, value(province))
		}
		rows[i].count++
	}
	return rows, nil
}

// writeSummary prints each histogram as aligned plain text followed by its total.
func writeSummary(w io.Writer, sections []summarySection) error {
	nameWidth, countWidth := len("total"), 1
	for _, section := range sections {
		total := 0
		for _, row := range section.rows {
			nameWidth = max(nameWidth, len(row.name))
			total += row.count
		}
		countWidth = max(countWidth, len(fmt.Sprint(total)))
	}
	for i, section := range sections {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, section.title); err != nil {
			return err
		}
		total := 0
		for _, row := range section.rows {
			total += row.count
			if _, err := fmt.Fprintf(w, "  %-*s  %*d\n", nameWidth, row.name, countWidth, row.count); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "  %-*s  %*d\n", nameWidth, "total", countWidth, total); err != nil {
			return err
		}
	}
	return nil
}
