package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mdhender/wgvc"
	"github.com/mdhender/wgvc/internal/aspectratio"
	"github.com/mdhender/wgvc/internal/x24"
)

const (
	formatSVG  = "svg"
	formatPNG  = "png"
	formatJSON = "json"
	formatBoth = "both"
	formatAll  = "all"

	pixelsPerCell = 16
)

type options struct {
	config           x24.Config
	polarIcePercent  float64
	peakChillPercent float64
	format           string
	outputBase       string
}

type outputFormats struct {
	svg  bool
	png  bool
	json bool
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
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	climateDefaults := wgvc.DefaultClimateConfig()
	options := options{
		config:           x24.DefaultConfig(),
		polarIcePercent:  climateDefaults.PolarIce * 100,
		peakChillPercent: climateDefaults.PeakChill * 100,
		format:           formatSVG,
		outputBase:       "world",
	}
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Uint64Var(&options.config.WorldSeed, "seed", options.config.WorldSeed, "world seed (decimal or 0x-prefixed hexadecimal)")
	flags.IntVar(&options.config.ProvinceCount, "provinces", options.config.ProvinceCount, "number of land provinces")
	flags.IntVar(&options.config.IslandCount, "islands", options.config.IslandCount, "number of initial islands")
	flags.StringVar(&options.config.AspectRatio, "aspect", options.config.AspectRatio, "map aspect name or width:height ratio")
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
	flags.Float64Var(&options.polarIcePercent, "polar-ice", options.polarIcePercent, "target percentage of ocean provinces in the polar heat band")
	flags.Float64Var(&options.peakChillPercent, "peak-chill", options.peakChillPercent, "target percentage of warm-region high peaks classified cold or colder")
	flags.StringVar(&options.format, "format", options.format, "output format: svg, png, json, both, all, or a comma-separated combination")
	flags.StringVar(&options.outputBase, "output", options.outputBase, "output path without an extension")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	if options.outputBase == "" {
		return fmt.Errorf("output path must not be empty")
	}
	formats, err := parseOutputFormats(options.format)
	if err != nil {
		return err
	}

	climateConfig := wgvc.ClimateConfig{
		PolarIce:  options.polarIcePercent / 100,
		PeakChill: options.peakChillPercent / 100,
	}
	if climateConfig.PolarIce == 0 {
		climateConfig.PolarIce = climateDefaults.PolarIce
	}
	if climateConfig.PeakChill == 0 {
		climateConfig.PeakChill = climateDefaults.PeakChill
	}
	world, result, err := wgvc.GenerateForRenderWithClimate(options.config, climateConfig)
	if err != nil {
		return fmt.Errorf("generate world: %w", err)
	}
	for _, skip := range result.AttractantSkips {
		fmt.Fprintf(stderr, "generate: skipped attractant region (%d,%d): %s\n", skip.RegionX, skip.RegionY, skip.Reason)
	}
	var width, height int
	var scene renderScene
	if formats.svg || formats.png {
		width, height, err = renderDimensions(options.config.ProvinceCount, result.FinalOcean, options.config.AspectRatio)
		if err != nil {
			return fmt.Errorf("derive render dimensions: %w", err)
		}
		scene, err = buildScene(world, width, height)
		if err != nil {
			return fmt.Errorf("build render scene: %w", err)
		}
	}

	if formats.svg {
		data, err := renderSVG(scene)
		if err != nil {
			return fmt.Errorf("render SVG: %w", err)
		}
		if err := writeOutput(options.outputBase+".svg", data); err != nil {
			return err
		}
	}
	if formats.png {
		data, err := renderPNG(scene)
		if err != nil {
			return fmt.Errorf("render PNG: %w", err)
		}
		if err := writeOutput(options.outputBase+".png", data); err != nil {
			return err
		}
	}
	if formats.json {
		data, err := renderJSON(world, options.config, climateConfig, result)
		if err != nil {
			return fmt.Errorf("render JSON: %w", err)
		}
		if err := writeOutput(options.outputBase+".json", data); err != nil {
			return err
		}
	}
	dimensions := ""
	if formats.svg || formats.png {
		dimensions = fmt.Sprintf(" (%d×%d)", width, height)
	}
	fmt.Fprintf(stdout, "wrote %s%s: %d cells, %d land, %d→%d islands, merges=%d, %.0f%% ocean, %d round(s)\n",
		options.outputBase, dimensions, len(result.Cells), options.config.ProvinceCount, result.InitialIslandCount,
		len(result.Islands), result.MergeCount, result.FinalOcean*100, result.RoundsAttempted)
	return nil
}

func parseOutputFormats(value string) (outputFormats, error) {
	var formats outputFormats
	for _, part := range strings.Split(value, ",") {
		switch strings.TrimSpace(part) {
		case formatSVG:
			formats.svg = true
		case formatPNG:
			formats.png = true
		case formatJSON:
			formats.json = true
		case formatBoth:
			formats.svg = true
			formats.png = true
		case formatAll:
			formats.svg = true
			formats.png = true
			formats.json = true
		default:
			return outputFormats{}, fmt.Errorf("unsupported format %q: use svg, png, json, both, all, or a comma-separated combination", value)
		}
	}
	return formats, nil
}

func renderDimensions(provinceCount int, oceanPercentage float64, aspect string) (int, int, error) {
	unitWidth, unitHeight, err := aspectratio.Dimensions(aspect)
	if err != nil {
		return 0, 0, err
	}
	cellCount := math.Ceil(float64(provinceCount) / (1 - oceanPercentage))
	linearScale := pixelsPerCell * math.Sqrt(cellCount)
	width := max(minimumImageSize, int(math.Ceil(unitWidth*linearScale+2*imageMargin)))
	height := max(minimumImageSize, int(math.Ceil(unitHeight*linearScale+2*imageMargin)))
	return width, height, nil
}

func writeOutput(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
