package composegraph

import (
	"fmt"
	"sort"
	"strings"
)

// toMermaid renders g as Mermaid flowchart source. Nodes sharing a Group
// are clustered into a subgraph; ungrouped nodes are emitted at the top
// level alongside those blocks.
func toMermaid(g *Graph) string {
	var b strings.Builder
	b.WriteString("graph LR\n")

	ids := nodeIDs(g.Nodes)

	for _, n := range g.Nodes {
		if n.Group == "" {
			writeNode(&b, "  ", ids[n.Key], n)
		}
	}
	for _, group := range sortedGroups(g.Nodes) {
		fmt.Fprintf(&b, "  subgraph net_%s [%s]\n", sanitizeID(group), group)
		for _, n := range g.Nodes {
			if n.Group == group {
				writeNode(&b, "    ", ids[n.Key], n)
			}
		}
		b.WriteString("  end\n")
	}

	for _, e := range g.Edges {
		from, ok := ids[e.From]
		if !ok {
			from = sanitizeID(e.From)
		}
		to, ok := ids[e.To]
		if !ok {
			to = sanitizeID(e.To)
		}
		if e.Label != "" {
			fmt.Fprintf(&b, "  %s -->|%s| %s\n", from, e.Label, to)
		} else {
			fmt.Fprintf(&b, "  %s --> %s\n", from, to)
		}
	}

	return b.String()
}

func writeNode(b *strings.Builder, indent, id string, n Node) {
	switch n.Shape {
	case "stadium":
		fmt.Fprintf(b, "%s%s([%s])\n", indent, id, n.Name)
	case "rounded":
		fmt.Fprintf(b, "%s%s(%s)\n", indent, id, n.Name)
	case "hexagon":
		fmt.Fprintf(b, "%s%s{{%s}}\n", indent, id, n.Name)
	case "cylinder":
		fmt.Fprintf(b, "%s%s[(%s)]\n", indent, id, n.Name)
	default:
		fmt.Fprintf(b, "%s%s[%s]\n", indent, id, n.Name)
	}
}

func sortedGroups(nodes []Node) []string {
	seen := map[string]bool{}
	var groups []string
	for _, n := range nodes {
		if n.Group != "" && !seen[n.Group] {
			seen[n.Group] = true
			groups = append(groups, n.Group)
		}
	}
	sort.Strings(groups)
	return groups
}

// nodeIDs assigns each node a short, sequential Mermaid-safe identifier,
// keyed by Node.Key. Sequential IDs sidestep sanitizing a node's own key
// (which for Kubernetes resources is a composite like "ns/Kind/name") into
// the diagram source; the original name survives as the node's label.
func nodeIDs(nodes []Node) map[string]string {
	ids := make(map[string]string, len(nodes))
	for i, n := range nodes {
		ids[n.Key] = fmt.Sprintf("n%d", i+1)
	}
	return ids
}

// sanitizeID maps an arbitrary key (used for group/subgraph IDs, and as a
// fallback for an edge endpoint with no matching node) to a Mermaid-safe
// identifier: letters, digits, and underscores only.
func sanitizeID(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	id := b.String()
	if id == "" || (id[0] >= '0' && id[0] <= '9') {
		id = "n_" + id
	}
	return id
}
