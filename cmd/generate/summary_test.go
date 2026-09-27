package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mdhender/wgvc"
)

func TestRunSummaryListsEveryValueInOrderWithTotals(t *testing.T) {
	args := []string{"-seed", "0x0123456789abcdef", "-provinces", "100", "-islands", "3", "-format", "json", "-summary"}
	var first, second bytes.Buffer
	if err := run(append(args, "-output", filepath.Join(t.TempDir(), "a")), &first, io.Discard); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if err := run(append(args, "-output", filepath.Join(t.TempDir(), "a")), &second, io.Discard); err != nil {
		t.Fatalf("second run() error = %v", err)
	}
	_, firstSummary, _ := strings.Cut(first.String(), "\n\n")
	_, secondSummary, _ := strings.Cut(second.String(), "\n\n")
	if firstSummary == "" || firstSummary != secondSummary {
		t.Fatalf("summary is empty or not deterministic:\n%s\n---\n%s", firstSummary, secondSummary)
	}

	sections := parseSummary(t, firstSummary)
	want := []struct {
		title string
		names []string
	}{
		{"elevation band", elevationBandNames(t)},
		{"heat band", heatBandNames(t)},
		{"moisture band", moistureBandNames(t)},
		{"terrain", terrainNames()},
		{"river class", riverClassNames()},
	}
	if len(sections) != len(want) {
		t.Fatalf("summary has %d sections, want %d:\n%s", len(sections), len(want), firstSummary)
	}
	provinceCount := -1
	for i, section := range sections {
		if section.title != want[i].title {
			t.Errorf("section %d title = %q, want %q", i, section.title, want[i].title)
		}
		if got := len(section.rows) - 1; got != len(want[i].names) {
			t.Fatalf("section %q has %d value rows, want %d", section.title, got, len(want[i].names))
		}
		sum := 0
		for j, name := range want[i].names {
			if section.rows[j].name != name {
				t.Errorf("section %q row %d = %q, want %q", section.title, j, section.rows[j].name, name)
			}
			sum += section.rows[j].count
		}
		total := section.rows[len(section.rows)-1]
		if total.name != "total" || total.count != sum {
			t.Errorf("section %q total row = %+v, want total %d", section.title, total, sum)
		}
		if section.title == "river class" {
			continue // counts rivers, not provinces
		}
		if provinceCount == -1 {
			provinceCount = sum
		} else if sum != provinceCount {
			t.Errorf("section %q totals %d, want %d", section.title, sum, provinceCount)
		}
	}
	for _, row := range sections[3].rows {
		if row.name == string(wgvc.TerrainVolcano) && row.count != 0 {
			t.Errorf("reserved terrain %q count = %d, want 0", row.name, row.count)
		}
	}
}

func TestRunWithoutSummaryPrintsOnlyStatusLine(t *testing.T) {
	var stdout bytes.Buffer
	args := []string{"-provinces", "100", "-islands", "3", "-format", "json", "-output", filepath.Join(t.TempDir(), "a")}
	if err := run(args, &stdout, io.Discard); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if lines := strings.Count(stdout.String(), "\n"); lines != 1 {
		t.Fatalf("stdout has %d lines, want 1:\n%s", lines, stdout.String())
	}
}

func TestBuildSummaryCountsAbsentValuesAsZero(t *testing.T) {
	world := wgvc.World{
		Provinces: []wgvc.Province{
			{ID: 0, Terrain: wgvc.TerrainLake, ElevationBand: wgvc.ElevationBandShallowWater, HeatBand: wgvc.HeatBandCold, MoistureBand: wgvc.MoistureBandHumid},
			{ID: 1, Terrain: wgvc.TerrainLake, ElevationBand: wgvc.ElevationBandShallowWater, HeatBand: wgvc.HeatBandCold, MoistureBand: wgvc.MoistureBandHumid},
		},
		Rivers: []wgvc.River{{ID: 0, Class: wgvc.RiverClassRiver}, {ID: 1, Class: wgvc.RiverClassRiver}},
	}
	sections, err := buildSummary(world)
	if err != nil {
		t.Fatalf("buildSummary() error = %v", err)
	}
	for _, section := range sections {
		nonZero := 0
		for _, row := range section.rows {
			switch row.count {
			case 0:
			case 2:
				nonZero++
			default:
				t.Errorf("section %q row %q count = %d, want 0 or 2", section.title, row.name, row.count)
			}
		}
		if nonZero != 1 {
			t.Errorf("section %q has %d non-zero rows, want 1", section.title, nonZero)
		}
	}
	var out bytes.Buffer
	if err := writeSummary(&out, sections); err != nil {
		t.Fatalf("writeSummary() error = %v", err)
	}
	if !strings.Contains(out.String(), "  volcanic-highland  0\n") {
		t.Fatalf("summary omits a zero-count terrain row:\n%s", out.String())
	}
}

func TestBuildSummaryRejectsUndeclaredValues(t *testing.T) {
	world := wgvc.World{Provinces: []wgvc.Province{{Terrain: "unknown"}}}
	if _, err := buildSummary(world); err == nil {
		t.Fatal("buildSummary() accepted an undeclared terrain")
	}
}

// The ordered band lists must cover every band the JSON export can name, so a
// band added to the enum and its name switch cannot be missing from the summary.
func TestBandListsCoverEveryNamedBand(t *testing.T) {
	if _, err := elevationBandName(wgvc.ElevationBand(len(wgvc.ElevationBands()))); err == nil {
		t.Error("ElevationBands() is missing a named elevation band")
	}
	if _, err := heatBandName(wgvc.HeatBand(len(wgvc.HeatBands()))); err == nil {
		t.Error("HeatBands() is missing a named heat band")
	}
	if _, err := moistureBandName(wgvc.MoistureBand(len(wgvc.MoistureBands()))); err == nil {
		t.Error("MoistureBands() is missing a named moisture band")
	}
}

func parseSummary(t *testing.T, text string) []summarySection {
	t.Helper()
	var sections []summarySection
	for _, block := range strings.Split(strings.TrimSuffix(text, "\n"), "\n\n") {
		lines := strings.Split(block, "\n")
		section := summarySection{title: lines[0]}
		for _, line := range lines[1:] {
			fields := strings.Fields(line)
			if len(fields) != 2 || !strings.HasPrefix(line, "  ") {
				t.Fatalf("malformed summary row %q", line)
			}
			count, err := strconv.Atoi(fields[1])
			if err != nil {
				t.Fatalf("summary row %q count: %v", line, err)
			}
			section.rows = append(section.rows, summaryRow{name: fields[0], count: count})
		}
		sections = append(sections, section)
	}
	return sections
}

func elevationBandNames(t *testing.T) []string {
	return bandNames(t, wgvc.ElevationBands(), elevationBandName)
}

func heatBandNames(t *testing.T) []string {
	return bandNames(t, wgvc.HeatBands(), heatBandName)
}

func moistureBandNames(t *testing.T) []string {
	return bandNames(t, wgvc.MoistureBands(), moistureBandName)
}

func bandNames[V any](t *testing.T, values []V, name func(V) (string, error)) []string {
	t.Helper()
	names := make([]string, len(values))
	for i, value := range values {
		label, err := name(value)
		if err != nil {
			t.Fatalf("band %v: %v", value, err)
		}
		names[i] = label
	}
	return names
}

func terrainNames() []string {
	var names []string
	for _, terrain := range wgvc.Terrains() {
		names = append(names, string(terrain))
	}
	return names
}

func riverClassNames() []string {
	var names []string
	for _, class := range wgvc.RiverClasses() {
		names = append(names, string(class))
	}
	return names
}
