package composegraph

import (
	"fmt"
	"sort"
	"strings"
)

// toMermaid renders g as Mermaid flowchart source. Services sharing a
// network are grouped into a subgraph; services with no `networks:`
// declared anywhere in the file are left ungrouped.
func toMermaid(g *Graph) string {
	var b strings.Builder
	b.WriteString("graph LR\n")

	ids := nodeIDs(g.Nodes)

	grouped := false
	for _, n := range g.Nodes {
		if n.Group != "" {
			grouped = true
			break
		}
	}

	if grouped {
		for _, group := range sortedGroups(g.Nodes) {
			fmt.Fprintf(&b, "  subgraph net_%s [%s]\n", sanitizeID(group), group)
			for _, n := range g.Nodes {
				if n.Group == group {
					writeNode(&b, "    ", ids[n.Name], n)
				}
			}
			b.WriteString("  end\n")
		}
	} else {
		for _, n := range g.Nodes {
			writeNode(&b, "  ", ids[n.Name], n)
		}
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
	if n.HasHealthCheck {
		fmt.Fprintf(b, "%s%s([%s])\n", indent, id, n.Name)
	} else {
		fmt.Fprintf(b, "%s%s[%s]\n", indent, id, n.Name)
	}
}

func sortedGroups(nodes []Node) []string {
	seen := map[string]bool{}
	var groups []string
	for _, n := range nodes {
		if !seen[n.Group] {
			seen[n.Group] = true
			groups = append(groups, n.Group)
		}
	}
	sort.Strings(groups)
	return groups
}

// nodeIDs assigns each node a Mermaid-safe identifier, deduplicating any
// collisions produced by sanitization.
func nodeIDs(nodes []Node) map[string]string {
	ids := make(map[string]string, len(nodes))
	used := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		id := sanitizeID(n.Name)
		for used[id] {
			id += "_"
		}
		used[id] = true
		ids[n.Name] = id
	}
	return ids
}

// sanitizeID maps a compose service name to a Mermaid node ID: letters,
// digits, and underscores only (compose names may contain '.' and '-',
// neither safe as a bare Mermaid ID).
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
