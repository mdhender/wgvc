package x23svg

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mdhender/wgvc/internal/x23"
)

func TestRenderIncludesEveryCellAndMetadata(t *testing.T) {
	config := x23.DefaultConfig()
	config.ProvinceCount = 30
	config.IslandCount = 3
	result, err := x23.Generate(config)
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
	for _, text := range []string{
		"30 land",
		"3 islands",
		fmt.Sprintf("%.0f%% ocean", result.FinalOcean*100),
		fmt.Sprintf("round %d", result.RoundsAttempted),
	} {
		if !strings.Contains(svg, text) {
			t.Errorf("SVG does not contain %q", text)
		}
	}
}
