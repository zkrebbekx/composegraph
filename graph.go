package composegraph

import "sort"

// Node is one resource in the dependency graph.
type Node struct {
	// Key uniquely identifies the node (used for edge endpoints and as the
	// basis for its Mermaid ID); Name is what's displayed.
	Key, Name string
	// Group is the subgraph this node is clustered under (a compose
	// network, or a Kubernetes namespace). Empty if ungrouped.
	Group string
	// Shape selects the Mermaid shape: "" (rectangle), "stadium", "rounded",
	// "hexagon", or "cylinder".
	Shape string
}

// Edge is a directed relationship between two nodes.
type Edge struct {
	From, To string
	// Label is shown on the edge; empty for a plain relationship.
	Label string
}

// Graph is the dependency graph extracted from an input file. Nodes and
// Edges are in deterministic order.
type Graph struct {
	Nodes []Node
	Edges []Edge
}

// buildComposeGraph converts a parsed compose file into a [Graph].
func buildComposeGraph(cf *composeFile) *Graph {
	names := make([]string, 0, len(cf.Services))
	for name := range cf.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	anyExplicitNetworks := false
	for _, name := range names {
		if len(cf.Services[name].Networks) > 0 {
			anyExplicitNetworks = true
			break
		}
	}

	g := &Graph{}
	for _, name := range names {
		svc := cf.Services[name]
		node := Node{Key: name, Name: name}
		if svc.HealthCheck != nil {
			node.Shape = "stadium"
		}
		if anyExplicitNetworks {
			node.Group = firstNetwork(svc.Networks)
		}
		g.Nodes = append(g.Nodes, node)

		for _, dep := range sortedKeys(svc.DependsOn) {
			g.Edges = append(g.Edges, Edge{
				From:  name,
				To:    dep,
				Label: conditionLabel(svc.DependsOn[dep].Condition),
			})
		}

		volTargets := append([]string(nil), svc.VolumesFrom...)
		sort.Strings(volTargets)
		for _, target := range volTargets {
			g.Edges = append(g.Edges, Edge{From: name, To: target, Label: "volumes"})
		}
	}
	return g
}

func firstNetwork(networks []string) string {
	if len(networks) == 0 {
		return "default"
	}
	sorted := append([]string(nil), networks...)
	sort.Strings(sorted)
	return sorted[0]
}

// conditionLabel maps a depends_on condition to an edge label. The default
// condition (service_started, or unset) carries no extra information, so it
// renders as a plain edge.
func conditionLabel(condition string) string {
	switch condition {
	case "", "service_started":
		return ""
	default:
		const prefix = "service_"
		if len(condition) > len(prefix) && condition[:len(prefix)] == prefix {
			return condition[len(prefix):]
		}
		return condition
	}
}

func sortedKeys(m dependsOn) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
