package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mdhender/wgvc/internal/x24"
	"github.com/mdhender/wgvc/internal/x24svg"
)

type options struct {
	config x24.Config
	width  int
	height int
	output string
}

type rampFlag struct {
	values *[]float64
}

func (f rampFlag) String() string {
	if f.values == nil {
		return ""
	}
	parts := make([]string, len(*f.values))
	for i, value := range *f.values {
		parts[i] = strconv.FormatFloat(value, 'g', -1, 64)
	}
	return strings.Join(parts, ",")
}

func (f rampFlag) Set(input string) error {
	parts := strings.Split(input, ",")
	values := make([]float64, len(parts))
	for i, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return fmt.Errorf("parse edge ramp value %q: %w", part, err)
		}
		values[i] = value
	}
	*f.values = values
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "x24: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	options := options{config: x24.DefaultConfig(), width: 1200, height: 900, output: "x24.svg"}
	flags := flag.NewFlagSet("x24", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Uint64Var(&options.config.WorldSeed, "seed", options.config.WorldSeed, "world seed (decimal or 0x-prefixed hexadecimal)")
	flags.IntVar(&options.config.ProvinceCount, "provinces", options.config.ProvinceCount, "number of land provinces")
	flags.IntVar(&options.config.IslandCount, "islands", options.config.IslandCount, "number of initial islands")
	flags.Float64Var(&options.config.OceanPercentage, "ocean", options.config.OceanPercentage, "initial ocean fraction")
	flags.Float64Var(&options.config.EdgeBarrierWidth, "edge-barrier", options.config.EdgeBarrierWidth, "permanent ocean strip width in unit-map coordinates")
	flags.Var(rampFlag{values: &options.config.EdgeRamp}, "edge-ramp", "comma-separated desirability values by hop past the barrier")
	flags.IntVar(&options.config.AttractantCount, "attractors", options.config.AttractantCount, "number of attractors: 0, 1, 2, 3, 4, 5, 6, or 9")
	flags.Var(rampFlag{values: &options.config.AttractantRamp}, "attractant-ramp", "comma-separated attractant values by hop from its source")
	flags.Float64Var(&options.config.AttractantJitter, "attractant-jitter", options.config.AttractantJitter, "maximum placement jitter as a fraction of half a region")
	flags.Float64Var(&options.config.SoftmaxTemperature, "temperature", options.config.SoftmaxTemperature, "softmax temperature for frontier selection")
	flags.Float64Var(&options.config.ControlPenalty, "control-penalty", options.config.ControlPenalty, "value rivals see for a controlled cell")
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

	result, err := x24.Generate(options.config)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	for _, skip := range result.AttractantSkips {
		fmt.Fprintf(stderr, "x24: skipped attractant region (%d,%d): %s\n", skip.RegionX, skip.RegionY, skip.Reason)
	}
	svg, err := x24svg.Render(result, options.width, options.height)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(options.output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(options.output, svg, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", options.output, err)
	}
	fmt.Fprintf(stdout, "wrote %s: %d cells, %d land, %d→%d islands, merges=%d, %.0f%% ocean, %d round(s)\n",
		options.output, len(result.Cells), options.config.ProvinceCount, result.InitialIslandCount,
		len(result.Islands), result.MergeCount, result.FinalOcean*100, result.RoundsAttempted)
	return nil
}
