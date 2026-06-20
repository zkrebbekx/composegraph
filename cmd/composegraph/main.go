// Command composegraph renders a docker-compose file's service dependency
// graph to SVG (or Mermaid source, or PNG).
//
// Usage:
//
//	composegraph [flags] [input ...]
//
// With no input file (or "-"), source is read from stdin and output is
// written to stdout (or -o). With multiple input files, each FILE.yml is
// rendered to FILE.svg (or .mmd/.png) and -o is not allowed.
//
//	composegraph docker-compose.yml > graph.svg
//	composegraph -format mmd -o graph.mmd docker-compose.yml
//	composegraph a/docker-compose.yml b/docker-compose.yml   # writes a.svg, b.svg
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
	out := flag.String("o", "", "output file (single-input mode only; default stdout)")
	theme := flag.String("theme", "default", "color theme: default, dark, neutral, forest, base")
	scale := flag.Float64("scale", 1, "PNG scale factor")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println("composegraph", version)
		return nil
	}

	opts := []mermaid.Option{mermaid.WithTheme(mermaid.Theme(*theme))}

	args := flag.Args()
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
	if *out == "" || *out == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(*out, data, 0o644)
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
