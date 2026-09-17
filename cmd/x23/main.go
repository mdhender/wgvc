package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mdhender/wgvc/internal/x23"
	"github.com/mdhender/wgvc/internal/x23svg"
)

type options struct {
	config x23.Config
	width  int
	height int
	output string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "x23: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	options := options{config: x23.DefaultConfig(), width: 1200, height: 900, output: "x23.svg"}
	flags := flag.NewFlagSet("x23", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Uint64Var(&options.config.WorldSeed, "seed", options.config.WorldSeed, "world seed (decimal or 0x-prefixed hexadecimal)")
	flags.IntVar(&options.config.ProvinceCount, "provinces", options.config.ProvinceCount, "number of land provinces")
	flags.IntVar(&options.config.IslandCount, "islands", options.config.IslandCount, "number of islands")
	flags.Float64Var(&options.config.OceanPercentage, "ocean", options.config.OceanPercentage, "initial ocean fraction")
	flags.IntVar(&options.config.MinEdgeDistance, "edge-hops", options.config.MinEdgeDistance, "minimum land distance from the map edge")
	flags.IntVar(&options.config.MinIslandDistance, "island-hops", options.config.MinIslandDistance, "minimum distance between islands")
	flags.IntVar(&options.config.MaxRounds, "rounds", options.config.MaxRounds, "maximum generation rounds")
	flags.IntVar(&options.config.Relaxations, "relaxations", options.config.Relaxations, "Lloyd relaxation passes per round")
	flags.IntVar(&options.width, "width", options.width, "SVG width in pixels")
	flags.IntVar(&options.height, "height", options.height, "SVG height in pixels")
	flags.StringVar(&options.output, "output", options.output, "output SVG path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	result, err := x23.Generate(options.config)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	svg, err := x23svg.Render(result, options.width, options.height)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(options.output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(options.output, svg, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", options.output, err)
	}
	fmt.Fprintf(stdout, "wrote %s: %d cells, %d land, %d islands, %.0f%% ocean, %d round(s)\n", options.output, len(result.Cells), options.config.ProvinceCount, len(result.Islands), result.FinalOcean*100, result.RoundsAttempted)
	return nil
}
