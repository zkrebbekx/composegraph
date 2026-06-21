//go:build js && wasm

// Command wasm is the browser entry point for the composegraph playground.
// It exposes composegraph's Render/ParseGraph to JavaScript, plus an
// in-wasm micro-benchmark so the page can show real render latency
// without JS↔wasm boundary noise.
package main

import (
	"encoding/base64"
	"time"

	"syscall/js"

	"github.com/zkrebbekx/composegraph"
)

func main() {
	js.Global().Set("composegraphRender", js.FuncOf(render))
	js.Global().Set("composegraphBench", js.FuncOf(bench))
	js.Global().Set("composegraphReady", js.ValueOf(true))
	// Notify the page that wasm is live.
	if cb := js.Global().Get("onComposegraphReady"); cb.Type() == js.TypeFunction {
		cb.Invoke()
	}
	select {} // keep the Go runtime alive
}

// render(src) -> { ok, error?, format, nodeCount, edgeCount, svgDataURL, mermaid, nodes, edges }
func render(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return map[string]any{"ok": false, "error": "no input"}
	}
	src := []byte(args[0].String())

	g, format, err := composegraph.ParseGraph(src)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	mmd, err := composegraph.ToMermaid(src)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	svg, err := composegraph.Render(src)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}

	nodes := make([]any, len(g.Nodes))
	for i, n := range g.Nodes {
		nodes[i] = map[string]any{"name": n.Name, "group": n.Group, "shape": n.Shape}
	}
	edges := make([]any, len(g.Edges))
	for i, e := range g.Edges {
		edges[i] = map[string]any{"from": e.From, "to": e.To, "label": e.Label}
	}

	return map[string]any{
		"ok":         true,
		"format":     format,
		"nodeCount":  len(g.Nodes),
		"edgeCount":  len(g.Edges),
		"svgDataURL": "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(svg),
		"mermaid":    mmd,
		"nodes":      nodes,
		"edges":      edges,
	}
}

// bench(src, iters) -> nanoseconds per Render (0 on error).
func bench(_ js.Value, args []js.Value) any {
	src := []byte(args[0].String())
	iters := 200
	if len(args) > 1 {
		iters = args[1].Int()
	}
	if _, err := composegraph.Render(src); err != nil {
		return 0.0
	}
	start := time.Now()
	for i := 0; i < iters; i++ {
		_, _ = composegraph.Render(src)
	}
	return float64(time.Since(start).Nanoseconds()) / float64(iters)
}
