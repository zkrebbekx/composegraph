// Package composegraph renders a docker-compose file's or a Kubernetes
// manifest's dependency graph as a Mermaid flowchart, then hands that off
// to go-mermaid for SVG/PNG rendering — no headless browser, no Node, no
// Graphviz.
//
//	svg, err := composegraph.Render(src)
//
// The input format is detected automatically. For a compose file, nodes
// are services; edges come from `depends_on` (labeled with any non-default
// condition) and `volumes_from`; services sharing a `networks:` entry are
// grouped into a subgraph. For a Kubernetes manifest, nodes are
// Deployments/StatefulSets/DaemonSets, Services, Ingresses, ConfigMaps,
// and Secrets; edges come from ownerReferences, Service selector → workload
// label matching, Ingress backend → Service name matching, and workload →
// ConfigMap/Secret references; resources sharing a namespace are grouped
// into a subgraph.
package composegraph

import (
	"fmt"

	mermaid "github.com/zkrebbekx/go-mermaid"
)

// ToMermaid parses a docker-compose or Kubernetes manifest YAML document
// and returns the equivalent Mermaid flowchart source.
func ToMermaid(src []byte) (string, error) {
	g, _, err := ParseGraph(src)
	if err != nil {
		return "", err
	}
	return toMermaid(g), nil
}

// ParseGraph parses a docker-compose or Kubernetes manifest YAML document
// into its dependency [Graph] and reports which format was detected
// ("compose" or "k8s"). Most callers want [ToMermaid] or [Render]; this is
// for callers that want the structured graph itself — e.g. to report node/
// edge counts, or build a different renderer entirely.
func ParseGraph(src []byte) (*Graph, string, error) {
	format, err := detectFormat(src)
	if err != nil {
		return nil, "", err
	}
	switch format {
	case "compose":
		cf, err := parseCompose(src)
		if err != nil {
			return nil, "", fmt.Errorf("composegraph: %w", err)
		}
		return buildComposeGraph(cf), format, nil
	default: // "k8s"
		resources, err := parseK8sResources(src)
		if err != nil {
			return nil, "", fmt.Errorf("composegraph: %w", err)
		}
		return buildK8sGraph(resources), format, nil
	}
}

// Render parses a docker-compose or Kubernetes manifest YAML document and
// renders its dependency graph straight to SVG via go-mermaid. opts are
// passed through to [mermaid.Render] (theme, padding, spacing, ...).
func Render(src []byte, opts ...mermaid.Option) ([]byte, error) {
	mmd, err := ToMermaid(src)
	if err != nil {
		return nil, err
	}
	return mermaid.Render(mmd, opts...)
}

// ToMermaidMerged merges two or more docker-compose documents — the
// docker-compose.yml + docker-compose.override.yml pattern — and returns
// the Mermaid flowchart source for the combined graph. See
// [mergeComposeFiles] in compose.go for exactly what "merge" means here.
//
// Kubernetes manifests don't need this: [ToMermaid] already reads a
// `---`-separated multi-document stream directly, so concatenating files
// is enough.
func ToMermaidMerged(srcs [][]byte) (string, error) {
	cf, err := mergeComposeFiles(srcs)
	if err != nil {
		return "", fmt.Errorf("composegraph: %w", err)
	}
	return toMermaid(buildComposeGraph(cf)), nil
}

// RenderMerged is like [ToMermaidMerged] but renders straight to SVG.
func RenderMerged(srcs [][]byte, opts ...mermaid.Option) ([]byte, error) {
	mmd, err := ToMermaidMerged(srcs)
	if err != nil {
		return nil, err
	}
	return mermaid.Render(mmd, opts...)
}
