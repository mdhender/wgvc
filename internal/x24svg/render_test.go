package x24svg

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mdhender/wgvc/internal/x24"
)

func TestRenderIncludesEveryCellAndDiagnostics(t *testing.T) {
	config := x24.DefaultConfig()
	config.ProvinceCount = 30
	config.IslandCount = 3
	result, err := x24.Generate(config)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := Render(result, 640, 480)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	svg := string(data)
	if got := strings.Count(svg, "<polygon "); got != len(result.Cells) {
		t.Errorf("polygon count = %d, want %d", got, len(result.Cells))
	}
	if got := strings.Count(svg, "class=\"attractant\""); got != len(result.Attractants) {
		t.Errorf("attractant marker count = %d, want %d", got, len(result.Attractants))
	}
	if got := strings.Count(svg, "data-land-eligible=\"false\""); got != barrierCount(result) || got == 0 {
		t.Errorf("rendered barrier count = %d, want %d nonzero", got, barrierCount(result))
	}
	for _, text := range []string{
		"30 land",
		fmt.Sprintf("3→%d islands", len(result.Islands)),
		fmt.Sprintf("merges=%d", result.MergeCount),
		fmt.Sprintf("attr=%d/9", len(result.Attractants)),
		fmt.Sprintf("%d barrier", barrierCount(result)),
		fmt.Sprintf("%.0f%% ocean", result.FinalOcean*100),
	} {
		if !strings.Contains(svg, text) {
			t.Errorf("SVG does not contain %q", text)
		}
	}
}
