// Command composegraph renders a docker-compose file's or a Kubernetes
// manifest's dependency graph to SVG (or Mermaid source, or PNG). The
// input format is detected automatically.
//
// Usage:
//
//	composegraph [flags] [input ...]
//
// With no input file (or "-"), source is read from stdin and output is
// written to stdout (or -o). With multiple compose input files, each
// FILE.yml is rendered to FILE.svg (or .mmd/.png) by default — pass
// -merge to instead merge them (docker-compose.yml +
// docker-compose.override.yml) into one graph.
//
//	composegraph docker-compose.yml > graph.svg
//	cat deployment.yaml service.yaml ingress.yaml | composegraph > graph.svg
//	composegraph -format mmd -o graph.mmd docker-compose.yml
//	composegraph a/docker-compose.yml b/docker-compose.yml         # writes a.svg, b.svg
//	composegraph -merge -o graph.svg docker-compose.yml docker-compose.override.yml
//	composegraph -format png -scale 2 -o graph.png docker-compose.yml
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zkrebbekx/composegraph"
	mermaid "github.com/zkrebbekx/go-mermaid"
	"github.com/zkrebbekx/go-mermaid/raster"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "composegraph:", err)
		os.Exit(1)
	}
}

func run() error {
	format := flag.String("format", "svg", "output format: svg, mmd, png")
	out := flag.String("o", "", "output file (single-input/-merge mode only; default stdout)")
	theme := flag.String("theme", "default", "color theme: default, dark, neutral, forest, base")
	scale := flag.Float64("scale", 1, "PNG scale factor")
	merge := flag.Bool("merge", false, "merge multiple compose files (docker-compose.yml + docker-compose.override.yml) into one graph, instead of batch mode")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println("composegraph", version)
		return nil
	}

	opts := []mermaid.Option{mermaid.WithTheme(mermaid.Theme(*theme))}

	args := flag.Args()
	if *merge {
		if len(args) < 2 {
			return fmt.Errorf("-merge requires at least 2 input files")
		}
		return renderMerged(args, *format, *out, opts, *scale)
	}
	if len(args) > 1 {
		if *out != "" {
			return fmt.Errorf("-o cannot be used with multiple input files")
		}
		return renderBatch(args, *format, opts, *scale)
	}

	src, err := readInput(firstArg(args))
	if err != nil {
		return err
	}
	data, err := renderBytes(src, *format, opts, *scale)
	if err != nil {
		return err
	}
	return writeOutput(*out, data)
}

func writeOutput(out string, data []byte) error {
	if out == "" || out == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(out, data, 0o644)
}

func renderMerged(files []string, format, out string, opts []mermaid.Option, scale float64) error {
	srcs := make([][]byte, len(files))
	for i, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		srcs[i] = src
	}
	var data []byte
	switch format {
	case "mmd":
		mmd, err := composegraph.ToMermaidMerged(srcs)
		if err != nil {
			return err
		}
		data = []byte(mmd)
	case "png":
		mmd, err := composegraph.ToMermaidMerged(srcs)
		if err != nil {
			return err
		}
		data, err = raster.PNG(mmd, scale, opts...)
		if err != nil {
			return err
		}
	case "svg":
		var err error
		data, err = composegraph.RenderMerged(srcs, opts...)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown -format %q (want svg, mmd, or png)", format)
	}
	return writeOutput(out, data)
}

// renderBytes produces output in the requested format for one compose file.
func renderBytes(src []byte, format string, opts []mermaid.Option, scale float64) ([]byte, error) {
	switch format {
	case "mmd":
		mmd, err := composegraph.ToMermaid(src)
		return []byte(mmd), err
	case "png":
		mmd, err := composegraph.ToMermaid(src)
		if err != nil {
			return nil, err
		}
		return raster.PNG(mmd, scale, opts...)
	case "svg":
		return composegraph.Render(src, opts...)
	default:
		return nil, fmt.Errorf("unknown -format %q (want svg, mmd, or png)", format)
	}
}

// renderBatch renders each input file to a sibling output file, with the
// extension matching format.
func renderBatch(files []string, format string, opts []mermaid.Option, scale float64) error {
	ext, err := extFor(format)
	if err != nil {
		return err
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		data, err := renderBytes(src, format, opts, scale)
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		dst := strings.TrimSuffix(f, filepath.Ext(f)) + ext
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "wrote", dst)
	}
	return nil
}

func extFor(format string) (string, error) {
	switch format {
	case "svg":
		return ".svg", nil
	case "mmd":
		return ".mmd", nil
	case "png":
		return ".png", nil
	default:
		return "", fmt.Errorf("unknown -format %q (want svg, mmd, or png)", format)
	}
}

func readInput(path string) ([]byte, error) {
	if path == "" || path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func usage() {
	fmt.Fprintf(os.Stderr, `composegraph %s - render a docker-compose dependency graph (pure Go)

Usage:
  composegraph [flags] [input ...]

Flags:
`, version)
	flag.PrintDefaults()
}
