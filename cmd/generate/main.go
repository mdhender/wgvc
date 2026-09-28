package main

import (
	"bufio"
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
	pretty           bool
	scale            float64
	layer            string
	region           string
	selectIDs        string
	radius           int
	outputBase       string
	showVersion      bool
	summary          bool
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
			return fmt.Errorf("parse ramp value %q: %w", part, err)
		}
		values[i] = value
	}
	*f.values = values
	return nil
}

// attractorsFlag sets either the regional attractant count or a named
// constellation, which replaces the count.
type attractorsFlag struct {
	config *x24.Config
}

func (f attractorsFlag) String() string {
	if f.config == nil {
		return ""
	}
	if f.config.Constellation != "" {
		return f.config.Constellation
	}
	return strconv.Itoa(f.config.AttractantCount)
}

func (f attractorsFlag) Set(input string) error {
	input = strings.TrimSpace(input)
	if count, err := strconv.Atoi(input); err == nil {
		f.config.AttractantCount = count
		f.config.Constellation = ""
		return nil
	}
	if _, ok := x24.ConstellationByName(input); !ok {
		return fmt.Errorf("unknown attractor count or constellation %q: want a count or one of %s", input, strings.Join(x24.ConstellationNames(), ", "))
	}
	f.config.Constellation = input
	f.config.AttractantCount = 0
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
		scale:            defaultPixelScale,
		layer:            string(layerTerrain),
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
	flags.Var(attractorsFlag{config: &options.config}, "attractors", "number of attractors (0, 1, 2, 3, 4, 5, 6, or 9) or a constellation name: "+strings.Join(x24.ConstellationNames(), ", "))
	flags.Var(rampFlag{values: &options.config.AttractantRamp}, "attractant-ramp", "comma-separated attractant values by hop from its source")
	flags.Float64Var(&options.config.AttractantJitter, "attractant-jitter", options.config.AttractantJitter, "maximum placement jitter as a fraction of half a region")
	flags.Float64Var(&options.config.SoftmaxTemperature, "temperature", options.config.SoftmaxTemperature, "softmax temperature for frontier selection")
	flags.Var(rampFlag{values: &options.config.RivalRamp}, "rival-ramp", "comma-separated penalties by hop distance to the nearest rival land")
	flags.Var(rampFlag{values: &options.config.RepulsorRamp}, "repulsor-ramp", "comma-separated penalties by hop distance from a constellation repulsor at full strength")
	flags.IntVar(&options.config.MaxRounds, "rounds", options.config.MaxRounds, "maximum generation rounds")
	flags.IntVar(&options.config.Relaxations, "relaxations", options.config.Relaxations, "Lloyd relaxation passes per round")
	flags.Float64Var(&options.polarIcePercent, "polar-ice", options.polarIcePercent, "target percentage of ocean provinces in the polar heat band")
	flags.Float64Var(&options.peakChillPercent, "peak-chill", options.peakChillPercent, "target percentage of warm-region high peaks classified cold or colder")
	flags.StringVar(&options.format, "format", options.format, "output format: svg, png, json, both, all, or a comma-separated combination")
	flags.BoolVar(&options.pretty, "pretty", false, "indent the JSON export for reading; the default is compact")
	flags.Float64Var(&options.scale, "scale", options.scale, "pixel scale for SVG and PNG: multiplies the derived image size, margin, and every stroke width")
	flags.StringVar(&options.layer, "layer", options.layer, "field that colors SVG and PNG provinces: terrain, elevation, relief, heat, moisture, or climate")
	flags.StringVar(&options.region, "region", "", "render only provinces centered in this world-coordinate box: minx,miny,maxx,maxy")
	flags.StringVar(&options.selectIDs, "select", "", "render only these comma-separated province IDs")
	flags.IntVar(&options.radius, "radius", 0, "grow the -region or -select selection by this many province hops")
	flags.StringVar(&options.outputBase, "output", options.outputBase, "output path without an extension")
	flags.BoolVar(&options.showVersion, "version", false, "print the generator version and exit")
	flags.BoolVar(&options.summary, "summary", false, "print province counts per elevation band, heat band, moisture band, and terrain")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if constellation, ok := x24.ConstellationByName(options.config.Constellation); ok {
		aspectSet, oceanSet := false, false
		flags.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "aspect":
				aspectSet = true
			case "ocean":
				oceanSet = true
			}
		})
		if !aspectSet && constellation.AspectRatio != "" {
			options.config.AspectRatio = constellation.AspectRatio
		}
		if !oceanSet && constellation.Ocean > 0 {
			options.config.OceanPercentage = constellation.Ocean
		}
	}
	if options.showVersion {
		fmt.Fprintln(stdout, wgvc.Version())
		return nil
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
	layer, err := parseLayer(options.layer)
	if err != nil {
		return err
	}
	selection, err := parseSelection(options.region, options.selectIDs, options.radius)
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
	for _, fallback := range result.SeedFallbacks {
		fmt.Fprintf(stderr, "generate: island %d seeded on cell %d instead of star cell %d: %s\n", fallback.IslandID, fallback.SeedID, fallback.CellID, fallback.Reason)
	}
	var width, height int
	var scene renderScene
	if formats.svg || formats.png {
		width, height, err = renderDimensions(options.config.ProvinceCount, result.FinalOcean, options.config.AspectRatio, options.scale)
		if err != nil {
			return fmt.Errorf("derive render dimensions: %w", err)
		}
		selected, err := selection.apply(world)
		if err != nil {
			return err
		}
		scene, err = buildScene(world, width, height, options.scale, layer, selected)
		if err != nil {
			return fmt.Errorf("build render scene: %w", err)
		}
		width, height = scene.width, scene.height
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
		err := writeOutputStream(options.outputBase+".json", func(output io.Writer) error {
			return writeJSON(output, world, options.config, climateConfig, result, options.pretty)
		})
		if err != nil {
			return fmt.Errorf("render JSON: %w", err)
		}
	}
	dimensions := ""
	if formats.svg || formats.png {
		dimensions = fmt.Sprintf(" (%d×%d)", width, height)
	}
	fmt.Fprintf(stdout, "wrote %s%s: %d cells, %d land, %d→%d islands, merges=%d, %.0f%% ocean, %d round(s)\n",
		options.outputBase, dimensions, len(result.Cells), options.config.ProvinceCount, result.InitialIslandCount,
		len(result.Islands), result.MergeCount, result.FinalOcean*100, result.RoundsAttempted)
	if options.summary {
		sections, err := buildSummary(world)
		if err != nil {
			return fmt.Errorf("build summary: %w", err)
		}
		fmt.Fprintln(stdout)
		if err := writeSummary(stdout, sections); err != nil {
			return fmt.Errorf("write summary: %w", err)
		}
	}
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

// renderDimensions derives the image size from the land province count, the
// effective ocean fraction, and the aspect ratio, then multiplies it by the
// pixel scale so the whole map, margin included, is drawn at that resolution.
func renderDimensions(provinceCount int, oceanPercentage float64, aspect string, scale float64) (int, int, error) {
	unitWidth, unitHeight, err := aspectratio.Dimensions(aspect)
	if err != nil {
		return 0, 0, err
	}
	if !(scale > 0) || math.IsInf(scale, 1) {
		return 0, 0, fmt.Errorf("pixel scale %g must be a positive finite number", scale)
	}
	cellCount := math.Ceil(float64(provinceCount) / (1 - oceanPercentage))
	linearScale := pixelsPerCell * math.Sqrt(cellCount)
	width := max(minimumImageSize, int(math.Ceil((unitWidth*linearScale+2*imageMargin)*scale)))
	height := max(minimumImageSize, int(math.Ceil((unitHeight*linearScale+2*imageMargin)*scale)))
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

// writeOutputStream creates path and hands write a buffered writer, so a
// large export streams to disk instead of being assembled in memory first.
func writeOutputStream(path string, write func(io.Writer) error) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory for %s: %w", path, err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", path, closeErr)
		}
	}()
	buffered := bufio.NewWriterSize(file, 1<<20)
	if err := write(buffered); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := buffered.Flush(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
