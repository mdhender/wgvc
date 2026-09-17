package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mdhender/wgvc"
)

const (
	formatSVG  = "svg"
	formatPNG  = "png"
	formatBoth = "both"
)

type cliOptions struct {
	seed       uint64
	provinces  int
	islands    int
	aspect     string
	width      int
	height     int
	format     string
	outputBase string
}

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(os.Stderr, "wgvc-render: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	options := cliOptions{}
	flags := flag.NewFlagSet("wgvc-render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Uint64Var(&options.seed, "seed", 42, "world generation seed")
	flags.IntVar(&options.provinces, "provinces", 137, "number of land provinces")
	flags.IntVar(&options.islands, "islands", 11, "number of islands")
	flags.StringVar(&options.aspect, "aspect", string(wgvc.AspectRatioSquare), "map aspect ratio in width:height notation")
	flags.IntVar(&options.width, "width", 1200, "output width in pixels")
	flags.IntVar(&options.height, "height", 800, "output height in pixels")
	flags.StringVar(&options.format, "format", formatSVG, "output format: svg, png, or both")
	flags.StringVar(&options.outputBase, "output", "world", "output path without an extension")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	if options.outputBase == "" {
		return fmt.Errorf("output path must not be empty")
	}
	if options.width < minimumImageSize || options.height < minimumImageSize {
		return fmt.Errorf("width and height must each be at least %d pixels", minimumImageSize)
	}
	if options.format != formatSVG && options.format != formatPNG && options.format != formatBoth {
		return fmt.Errorf("unsupported format %q: use svg, png, or both", options.format)
	}

	world, err := wgvc.Generate(wgvc.Config{
		WorldSeed:     options.seed,
		ProvinceCount: options.provinces,
		IslandCount:   options.islands,
		AspectRatio:   wgvc.AspectRatio(options.aspect),
	})
	if err != nil {
		return fmt.Errorf("generate world: %w", err)
	}
	scene, err := buildScene(world, options.width, options.height)
	if err != nil {
		return fmt.Errorf("build render scene: %w", err)
	}

	if options.format == formatSVG || options.format == formatBoth {
		data, err := renderSVG(scene)
		if err != nil {
			return fmt.Errorf("render SVG: %w", err)
		}
		if err := writeOutput(options.outputBase+".svg", data); err != nil {
			return err
		}
	}
	if options.format == formatPNG || options.format == formatBoth {
		data, err := renderPNG(scene)
		if err != nil {
			return fmt.Errorf("render PNG: %w", err)
		}
		if err := writeOutput(options.outputBase+".png", data); err != nil {
			return err
		}
	}
	return nil
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
