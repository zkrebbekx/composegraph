package composegraph

import "sort"

// EdgeKind distinguishes the compose relationship an [Edge] represents.
type EdgeKind int

const (
	// DependsOn is a `depends_on` relationship.
	DependsOn EdgeKind = iota
	// VolumesFrom is a `volumes_from` relationship.
	VolumesFrom
)

// Node is one service in the compose file.
type Node struct {
	// Name is the service name (the map key under `services:`).
	Name string
	// Group is the subgraph this node is clustered under (its first network,
	// alphabetically, or "default" if it declares none). Empty when the
	// compose file uses no `networks:` keys at all, so no grouping applies.
	Group string
	// HasHealthCheck reports whether the service declares a healthcheck.
	HasHealthCheck bool
}

// Edge is a directed relationship between two services.
type Edge struct {
	From, To string
	Kind     EdgeKind
	// Label is shown on the edge; empty for a plain dependency.
	Label string
}

// Graph is the dependency graph extracted from a compose file. Nodes and
// Edges are sorted for deterministic output.
type Graph struct {
	Nodes []Node
	Edges []Edge
}

// buildGraph converts a parsed compose file into a [Graph].
func buildGraph(cf *composeFile) *Graph {
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
		node := Node{Name: name, HasHealthCheck: svc.HealthCheck != nil}
		if anyExplicitNetworks {
			node.Group = firstNetwork(svc.Networks)
		}
		g.Nodes = append(g.Nodes, node)

		for _, dep := range sortedKeys(svc.DependsOn) {
			g.Edges = append(g.Edges, Edge{
				From:  name,
				To:    dep,
				Kind:  DependsOn,
				Label: conditionLabel(svc.DependsOn[dep].Condition),
			})
		}

		volTargets := append([]string(nil), svc.VolumesFrom...)
		sort.Strings(volTargets)
		for _, target := range volTargets {
			g.Edges = append(g.Edges, Edge{From: name, To: target, Kind: VolumesFrom, Label: "volumes"})
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
