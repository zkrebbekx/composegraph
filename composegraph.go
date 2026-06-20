// Package composegraph renders a docker-compose file's service dependency
// graph as a Mermaid flowchart, then hands that off to go-mermaid for
// SVG/PNG rendering — no headless browser, no Node, no Graphviz.
//
//	svg, err := composegraph.Render(composeYAML)
//
// Nodes are services; edges come from `depends_on` (labeled with any
// non-default condition) and `volumes_from`. Services sharing a `networks:`
// entry are grouped into a subgraph, so the diagram clusters visually by
// network.
package composegraph

import (
	"fmt"

	mermaid "github.com/zkrebbekx/go-mermaid"
)

// ToMermaid parses a docker-compose YAML document and returns the
// equivalent Mermaid flowchart source.
func ToMermaid(src []byte) (string, error) {
	cf, err := parseCompose(src)
	if err != nil {
		return "", fmt.Errorf("composegraph: %w", err)
	}
	return toMermaid(buildGraph(cf)), nil
}

// Render parses a docker-compose YAML document and renders its dependency
// graph straight to SVG via go-mermaid. opts are passed through to
// [mermaid.Render] (theme, padding, spacing, ...).
func Render(src []byte, opts ...mermaid.Option) ([]byte, error) {
	mmd, err := ToMermaid(src)
	if err != nil {
		return nil, err
	}
	return mermaid.Render(mmd, opts...)
}
